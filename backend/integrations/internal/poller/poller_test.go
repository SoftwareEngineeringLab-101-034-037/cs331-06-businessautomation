package poller

import (
	"encoding/json"
	"testing"

	"github.com/example/business-automation/backend/integrations/internal/googleapi"
	"github.com/example/business-automation/backend/integrations/internal/models"
)

func TestNewPollerDefaultTriggerPath(t *testing.T) {
	p := New(nil, nil, "http://workflow", "", "", 10)
	if p.triggerPath == "" {
		t.Fatalf("expected default trigger path")
	}
}

func TestMapFieldsIncludesMappedAndEmail(t *testing.T) {
	p := New(nil, nil, "http://workflow", "", "", 10)
	resp := googleapi.FormResponse{
		RespondentEmail: "u@example.com",
		Answers: map[string]googleapi.Answer{
			"q1":       {TextAnswers: &googleapi.TextAnswers{Answers: []googleapi.TextAnswer{{Value: "v1"}}}},
			"q_upload": {FileUploadAnswers: &googleapi.FileUploadAnswers{Answers: []googleapi.FileUploadAnswer{{FileID: "abc123"}}}},
		},
	}
	data := p.mapFields(resp, map[string]string{"q1": "name", "q_upload": "resume_links", "_respondent_email": "email"})
	if data["name"] != "v1" || data["email"] != "u@example.com" {
		t.Fatalf("unexpected mapped data: %#v", data)
	}
	var entries []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal([]byte(data["resume_links"]), &entries); err != nil {
		t.Fatalf("expected JSON-encoded upload links, got %q: %v", data["resume_links"], err)
	}
	if len(entries) != 1 || entries[0].Name != "Drive file abc123" || entries[0].URL != "https://drive.google.com/file/d/abc123/view" {
		t.Fatalf("unexpected structured upload links: %#v", entries)
	}
}

func TestReconnectAndAutoDisableHeuristics(t *testing.T) {
	p := New(nil, nil, "http://workflow", "", "", 10)
	if !p.isReconnectError(errString("unauthorized_client")) {
		t.Fatalf("expected reconnect error")
	}
	if !p.shouldAutoDisableTestWatch(&models.FormWatch{OrgID: "test-org"}, errString("no google connection for org")) {
		t.Fatalf("expected auto-disable for test org")
	}
	if p.shouldAutoDisableTestWatch(&models.FormWatch{OrgID: "org_1"}, errString("no google connection for org")) {
		t.Fatalf("did not expect auto-disable for non-test org")
	}
}

func TestPauseMapHelpers(t *testing.T) {
	p := New(nil, nil, "http://workflow", "", "", 10)
	if !p.markWatchPaused("a") {
		t.Fatalf("first pause mark should return true")
	}
	if p.markWatchPaused("a") {
		t.Fatalf("second pause mark should return false")
	}
	p.clearWatchPaused("a")
	if !p.markWatchPaused("a") {
		t.Fatalf("pause mark should be true after clear")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
