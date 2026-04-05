package poller

import "testing"

func TestNewGmailDefaults(t *testing.T) {
	p := NewGmail(nil, nil, "http://workflow/", "", "", 0)
	if p.triggerPath == "" {
		t.Fatalf("expected default gmail trigger path")
	}
	if p.workflowURL != "http://workflow" {
		t.Fatalf("expected trimmed workflow URL, got %q", p.workflowURL)
	}
	if p.interval <= 0 {
		t.Fatalf("expected positive interval")
	}
}
