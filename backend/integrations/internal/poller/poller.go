package poller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/example/business-automation/backend/integrations/internal/googleapi"
	"github.com/example/business-automation/backend/integrations/internal/models"
	"github.com/example/business-automation/backend/integrations/internal/oauth"
	providergoogleforms "github.com/example/business-automation/backend/integrations/internal/providers/googleforms"
	"github.com/example/business-automation/backend/integrations/internal/storage"
)

type Poller struct {
	store       storage.Store
	oauthSvc    *oauth.Service
	workflowURL string
	triggerPath string
	workflowKey string
	interval    time.Duration
	stopCh      chan struct{}
	authPaused  map[string]bool
	mu          sync.Mutex
}

func New(store storage.Store, oauthSvc *oauth.Service, workflowURL, triggerPath, workflowKey string, intervalSeconds int) *Poller {
	resolvedTriggerPath := strings.TrimSpace(triggerPath)
	if resolvedTriggerPath == "" {
		resolvedTriggerPath = providergoogleforms.TriggerEventPath
	}
	return &Poller{
		store:       store,
		oauthSvc:    oauthSvc,
		workflowURL: workflowURL,
		triggerPath: resolvedTriggerPath,
		workflowKey: workflowKey,
		interval:    time.Duration(intervalSeconds) * time.Second,
		stopCh:      make(chan struct{}),
		authPaused:  make(map[string]bool),
	}
}

func (p *Poller) Start(ctx context.Context) {
	log.Printf("poller: starting, interval=%s", p.interval)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.tick(ctx)

	for {
		select {
		case <-ticker.C:
			p.tick(ctx)
		case <-p.stopCh:
			log.Println("poller: stopped")
			return
		case <-ctx.Done():
			return
		}
	}
}

func (p *Poller) Stop() {
	close(p.stopCh)
}

func (p *Poller) tick(ctx context.Context) {
	watches, err := p.store.ListActiveWatchesByProvider(ctx, providergoogleforms.ProviderID)
	if err != nil {
		log.Printf("poller: list watches: %v", err)
		return
	}
	for _, w := range watches {
		if err := p.processWatch(ctx, w); err != nil {
			log.Printf("poller: watch %s (form %s): %v", w.ID.Hex(), w.FormID, err)
		}
	}
}

func (p *Poller) processWatch(ctx context.Context, watch *models.FormWatch) error {
	client, err := p.oauthSvc.GetClient(ctx, watch.OrgID)
	if err != nil {
		if p.isReconnectError(err) {
			if p.shouldAutoDisableTestWatch(watch, err) {
				watch.Active = false
				if updateErr := p.store.UpdateWatch(ctx, watch); updateErr != nil {
					return fmt.Errorf("disable stale test watch: %w", updateErr)
				}
				if p.markWatchPaused(watch.ID.Hex()) {
					log.Printf("poller: disabled stale test watch %s for org %s (no Google connection)", watch.ID.Hex(), watch.OrgID)
				}
				return nil
			}
			if p.markWatchPaused(watch.ID.Hex()) {
				log.Printf("poller: watch %s paused until Google is reconnected for org %s", watch.ID.Hex(), watch.OrgID)
			}
			return nil
		}
		return fmt.Errorf("get oauth client: %w", err)
	}
	p.clearWatchPaused(watch.ID.Hex())

	responses, err := googleapi.ListResponses(client, watch.FormID, watch.LastResponseTS)
	if err != nil {
		return fmt.Errorf("list responses: %w", err)
	}

	if len(responses) == 0 {
		return p.updatePollTime(ctx, watch)
	}

	// Process oldest first so checkpoint advancement is monotonic and safe to retry.
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].LastSubmittedTime < responses[j].LastSubmittedTime
	})

	latestTS := watch.LastResponseTS
	triggeredCount := 0
	for _, resp := range responses {
		data := p.mapFields(resp, watch.FieldMapping)
		data["_response_id"] = resp.ResponseID
		data["_form_id"] = watch.FormID
		data["_submitted_at"] = resp.LastSubmittedTime

		if err := p.triggerWorkflow(watch.OrgID, watch.WorkflowID, data); err != nil {
			log.Printf("poller: trigger workflow %s for response %s failed (will retry): %v",
				watch.WorkflowID, resp.ResponseID, err)
			break
		}
		triggeredCount++

		if resp.LastSubmittedTime > latestTS {
			latestTS = resp.LastSubmittedTime
		}
	}

	if triggeredCount == 0 {
		// Keep checkpoint unchanged to retry on next poll.
		return p.updatePollTime(ctx, watch)
	}

	watch.LastResponseTS = latestTS
	watch.LastPolledAt = time.Now()
	return p.store.UpdateWatch(ctx, watch)
}

func (p *Poller) mapFields(resp googleapi.FormResponse, mapping map[string]string) map[string]string {
	data := make(map[string]string, len(resp.Answers))
	for questionID, answer := range resp.Answers {
		key := questionID
		if mapped, ok := mapping[questionID]; ok {
			key = mapped
		}
		if answer.TextAnswers != nil && len(answer.TextAnswers.Answers) > 0 {
			data[key] = answer.TextAnswers.Answers[0].Value
		}
	}
	return data
}

func (p *Poller) triggerWorkflow(orgID, workflowID string, data map[string]string) error {
	payload, err := json.Marshal(map[string]interface{}{
		"org_id":      orgID,
		"workflow_id": workflowID,
		"data":        data,
	})
	if err != nil {
		return err
	}

	url := p.workflowURL + p.triggerPath
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.workflowKey != "" {
		req.Header.Set("X-Integration-Key", p.workflowKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post instance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("workflow engine returned %d", resp.StatusCode)
	}
	return nil
}

func (p *Poller) updatePollTime(ctx context.Context, watch *models.FormWatch) error {
	watch.LastPolledAt = time.Now()
	return p.store.UpdateWatch(ctx, watch)
}

func (p *Poller) isReconnectError(err error) bool {
	if oauth.IsReconnectRequiredError(err) {
		return true
	}
	if oauth.IsNotConfiguredError(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no google connection for org") || strings.Contains(msg, "unauthorized_client")
}

func (p *Poller) shouldAutoDisableTestWatch(watch *models.FormWatch, err error) bool {
	if watch == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "no google connection for org") {
		return false
	}
	org := strings.ToLower(strings.TrimSpace(watch.OrgID))
	return org == "test-org" || strings.HasPrefix(org, "test-") || strings.Contains(org, "-test")
}

func (p *Poller) markWatchPaused(watchID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.authPaused[watchID] {
		return false
	}
	p.authPaused[watchID] = true
	return true
}

func (p *Poller) clearWatchPaused(watchID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.authPaused, watchID)
}
