package poller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/example/business-automation/backend/google-forms/internal/googleapi"
	"github.com/example/business-automation/backend/google-forms/internal/models"
	"github.com/example/business-automation/backend/google-forms/internal/oauth"
	"github.com/example/business-automation/backend/google-forms/internal/storage"
)

type Poller struct {
	store       storage.Store
	oauthSvc    *oauth.Service
	workflowURL string
	interval    time.Duration
	stopCh      chan struct{}
}

func New(store storage.Store, oauthSvc *oauth.Service, workflowURL string, intervalSeconds int) *Poller {
	return &Poller{
		store:       store,
		oauthSvc:    oauthSvc,
		workflowURL: workflowURL,
		interval:    time.Duration(intervalSeconds) * time.Second,
		stopCh:      make(chan struct{}),
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
	watches, err := p.store.ListActiveWatches(ctx)
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
		return fmt.Errorf("get oauth client: %w", err)
	}

	responses, err := googleapi.ListResponses(client, watch.FormID, watch.LastResponseTS)
	if err != nil {
		return fmt.Errorf("list responses: %w", err)
	}

	if len(responses) == 0 {
		return p.updatePollTime(ctx, watch)
	}

	latestTS := watch.LastResponseTS
	for _, resp := range responses {
		data := p.mapFields(resp, watch.FieldMapping)
		data["_response_id"] = resp.ResponseID
		data["_form_id"] = watch.FormID
		data["_submitted_at"] = resp.LastSubmittedTime

		if err := p.triggerWorkflow(watch.WorkflowID, data); err != nil {
			log.Printf("poller: trigger workflow %s for response %s: %v",
				watch.WorkflowID, resp.ResponseID, err)
		}

		if resp.LastSubmittedTime > latestTS {
			latestTS = resp.LastSubmittedTime
		}
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

func (p *Poller) triggerWorkflow(workflowID string, data map[string]string) error {
	payload, err := json.Marshal(map[string]interface{}{
		"workflow_id": workflowID,
		"data":        data,
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(
		p.workflowURL+"/instances",
		"application/json",
		bytes.NewReader(payload),
	)
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
