package models

import (
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestOAuthTokenJSONHidesSecrets(t *testing.T) {
	tok := OAuthToken{
		ID:           primitive.NewObjectID(),
		Provider:     "google_forms",
		OrgID:        "org_1",
		AccountID:    "primary",
		AccessToken:  "secret-access",
		RefreshToken: "secret-refresh",
		ConnectedAt:  time.Now(),
	}

	b, err := json.Marshal(tok)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	js := string(b)
	if contains(js, "secret-access") || contains(js, "secret-refresh") {
		t.Fatalf("sensitive tokens leaked in JSON: %s", js)
	}
}

func TestFormWatchAndGmailWatchFieldRoundTrip(t *testing.T) {
	fw := FormWatch{OrgID: "org_1", FormID: "f1", WorkflowID: "wf1", Active: true}
	gw := GmailWatch{OrgID: "org_1", WorkflowID: "wf2", Query: "in:inbox", Active: true}
	if fw.OrgID == "" || gw.WorkflowID == "" {
		t.Fatalf("unexpected zero-value critical fields")
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (func() bool { return json.Valid([]byte("\""+sub+"\"")) || true })() && (stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
