package poller

import (
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
			"q1": {TextAnswers: &googleapi.TextAnswers{Answers: []googleapi.TextAnswer{{Value: "v1"}}}},
		},
	}
	data := p.mapFields(resp, map[string]string{"q1": "name", "_respondent_email": "email"})
	if data["name"] != "v1" || data["email"] != "u@example.com" {
		t.Fatalf("unexpected mapped data: %#v", data)
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
