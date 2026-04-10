package poller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPollerTriggerWorkflowPostsToEngine(t *testing.T) {
	type requestMeta struct {
		path string
		key  string
	}
	reqCh := make(chan requestMeta, 1)
	errCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			errCh <- err
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		reqCh <- requestMeta{path: r.URL.Path, key: r.Header.Get("X-Integration-Key")}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New(nil, nil, server.URL, "/trigger", "shared-key", 10)
	if err := p.triggerWorkflow("org_1", "wf_1", map[string]string{"x": "y"}); err != nil {
		t.Fatalf("triggerWorkflow failed: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("decode body: %v", err)
	case req := <-reqCh:
		if req.path != "/trigger" || req.key != "shared-key" {
			t.Fatalf("unexpected request path/key: %q %q", req.path, req.key)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for trigger request")
	}
}

func TestGmailPollerTriggerWorkflowPostsToEngine(t *testing.T) {
	type requestMeta struct {
		path string
		key  string
	}
	reqCh := make(chan requestMeta, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCh <- requestMeta{path: r.URL.Path, key: r.Header.Get("X-Integration-Key")}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewGmail(nil, nil, server.URL, "/gmail-trigger", "gmail-key", 10)
	if err := p.triggerWorkflow("org_1", "wf_1", map[string]interface{}{"id": "1"}); err != nil {
		t.Fatalf("triggerWorkflow failed: %v", err)
	}

	select {
	case req := <-reqCh:
		if req.path != "/gmail-trigger" || req.key != "gmail-key" {
			t.Fatalf("unexpected request path/key: %q %q", req.path, req.key)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for gmail trigger request")
	}
}
