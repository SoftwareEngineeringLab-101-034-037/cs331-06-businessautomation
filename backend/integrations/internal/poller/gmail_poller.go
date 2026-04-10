package poller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/googleapi"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/models"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/oauth"
	providergmail "github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/providers/gmail"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/storage"
)

type GmailPoller struct {
	store       storage.Store
	oauthSvc    *oauth.Service
	workflowURL string
	triggerPath string
	workflowKey string
	interval    time.Duration
}

func NewGmail(store storage.Store, oauthSvc *oauth.Service, workflowURL, triggerPath, workflowKey string, intervalSeconds int) *GmailPoller {
	resolvedTriggerPath := strings.TrimSpace(triggerPath)
	if resolvedTriggerPath == "" {
		resolvedTriggerPath = providergmail.TriggerEventPath
	}
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	return &GmailPoller{
		store:       store,
		oauthSvc:    oauthSvc,
		workflowURL: strings.TrimRight(strings.TrimSpace(workflowURL), "/"),
		triggerPath: resolvedTriggerPath,
		workflowKey: strings.TrimSpace(workflowKey),
		interval:    time.Duration(intervalSeconds) * time.Second,
	}
}

func (p *GmailPoller) Start(ctx context.Context) {
	log.Printf("gmail-poller: starting, interval=%s", p.interval)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.tick(ctx)
	for {
		select {
		case <-ticker.C:
			p.tick(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (p *GmailPoller) tick(ctx context.Context) {
	watches, err := p.store.ListActiveGmailWatches(ctx)
	if err != nil {
		log.Printf("gmail-poller: list watches: %v", err)
		return
	}
	for _, watch := range watches {
		if watch == nil {
			continue
		}
		if err := p.processWatch(ctx, watch); err != nil {
			log.Printf("gmail-poller: watch %s failed: %v", watch.ID.Hex(), err)
		}
	}
}

func (p *GmailPoller) processWatch(ctx context.Context, watch *models.GmailWatch) error {
	client, err := p.oauthSvc.GetClientForProviderAndAccount(ctx, watch.OrgID, providergmail.ProviderID, strings.TrimSpace(watch.AccountID))
	if err != nil {
		if oauth.IsNotConnectedError(err) || oauth.IsReconnectRequiredError(err) {
			log.Printf("gmail-poller: watch %s paused for org %s until Google reconnect", watch.ID.Hex(), watch.OrgID)
			return nil
		}
		return fmt.Errorf("get oauth client: %w", err)
	}

	messages, err := googleapi.ListMessages(client, watch.Query, watch.LastMessageInternalTS, 50)
	if err != nil {
		return fmt.Errorf("list gmail messages: %w", err)
	}
	if len(messages) == 0 {
		watch.LastPolledAt = time.Now()
		return p.store.UpdateGmailWatch(ctx, watch)
	}

	latestTS := watch.LastMessageInternalTS
	triggered := 0
	for _, message := range messages {
		data := map[string]interface{}{
			"_message_id":         message.ID,
			"_thread_id":          message.ThreadID,
			"_subject":            message.Subject,
			"_from":               message.From,
			"_to":                 message.To,
			"_date":               message.Date,
			"_snippet":            message.Snippet,
			"_label_ids":          message.LabelIDs,
			"_internal_date_ms":   message.InternalDate,
			"_internal_date_unix": message.InternalDate / 1000,
		}
		if err := p.triggerWorkflow(watch.OrgID, watch.WorkflowID, data); err != nil {
			log.Printf("gmail-poller: trigger workflow %s for message %s failed (will retry): %v", watch.WorkflowID, message.ID, err)
			break
		}
		triggered++
		if message.InternalDate > latestTS {
			latestTS = message.InternalDate
		}
	}

	if triggered == 0 {
		watch.LastPolledAt = time.Now()
		return p.store.UpdateGmailWatch(ctx, watch)
	}

	watch.LastMessageInternalTS = latestTS
	watch.LastPolledAt = time.Now()
	return p.store.UpdateGmailWatch(ctx, watch)
}

func (p *GmailPoller) triggerWorkflow(orgID, workflowID string, data map[string]interface{}) error {
	payload, err := json.Marshal(map[string]interface{}{
		"org_id":      orgID,
		"workflow_id": workflowID,
		"data":        data,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, p.workflowURL+p.triggerPath, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.workflowKey != "" {
		req.Header.Set("X-Integration-Key", p.workflowKey)
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("post instance: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("workflow engine returned %d", resp.StatusCode)
	}
	return nil
}
