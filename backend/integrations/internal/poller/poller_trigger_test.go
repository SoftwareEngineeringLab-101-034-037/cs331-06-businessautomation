package poller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/business-automation/backend/integrations/internal/models"
)

type noopStore struct{}

func (noopStore) ListActiveWatchesByProvider(context.Context, string) ([]*models.FormWatch, error) {
	return nil, nil
}
func (noopStore) UpdateWatch(context.Context, *models.FormWatch) error { return nil }
func (noopStore) ListActiveGmailWatches(context.Context) ([]*models.GmailWatch, error) {
	return nil, nil
}
func (noopStore) UpdateGmailWatch(context.Context, *models.GmailWatch) error { return nil }

func TestPollerTriggerWorkflowPostsToEngine(t *testing.T) {
	var gotPath string
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-Integration-Key")
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New(nil, nil, server.URL, "/trigger", "shared-key", 10)
	if err := p.triggerWorkflow("org_1", "wf_1", map[string]string{"x": "y"}); err != nil {
		t.Fatalf("triggerWorkflow failed: %v", err)
	}
	if gotPath != "/trigger" || gotKey != "shared-key" {
		t.Fatalf("unexpected request path/key: %q %q", gotPath, gotKey)
	}
}

func TestGmailPollerTriggerWorkflowPostsToEngine(t *testing.T) {
	var gotPath string
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-Integration-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewGmail(nil, nil, server.URL, "/gmail-trigger", "gmail-key", 10)
	if err := p.triggerWorkflow("org_1", "wf_1", map[string]interface{}{"id": "1"}); err != nil {
		t.Fatalf("triggerWorkflow failed: %v", err)
	}
	if gotPath != "/gmail-trigger" || gotKey != "gmail-key" {
		t.Fatalf("unexpected request path/key: %q %q", gotPath, gotKey)
	}
}
