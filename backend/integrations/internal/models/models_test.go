package models

import (
	"encoding/json"
	"strings"
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
	if strings.Contains(js, "secret-access") || strings.Contains(js, "secret-refresh") {
		t.Fatalf("sensitive tokens leaked in JSON: %s", js)
	}
}

func TestFormWatchAndGmailWatchFieldRoundTrip(t *testing.T) {
	fw := FormWatch{OrgID: "org_1", FormID: "f1", WorkflowID: "wf1", Active: true}
	gw := GmailWatch{OrgID: "org_1", WorkflowID: "wf2", Query: "in:inbox", Active: true}

	fwJSON, err := json.Marshal(fw)
	if err != nil {
		t.Fatalf("marshal form watch failed: %v", err)
	}
	var fwDecoded FormWatch
	if err := json.Unmarshal(fwJSON, &fwDecoded); err != nil {
		t.Fatalf("unmarshal form watch failed: %v", err)
	}
	if fwDecoded.OrgID != fw.OrgID || fwDecoded.FormID != fw.FormID || fwDecoded.WorkflowID != fw.WorkflowID || fwDecoded.Active != fw.Active {
		t.Fatalf("form watch round-trip mismatch: got=%+v want=%+v", fwDecoded, fw)
	}

	gwJSON, err := json.Marshal(gw)
	if err != nil {
		t.Fatalf("marshal gmail watch failed: %v", err)
	}
	var gwDecoded GmailWatch
	if err := json.Unmarshal(gwJSON, &gwDecoded); err != nil {
		t.Fatalf("unmarshal gmail watch failed: %v", err)
	}
	if gwDecoded.OrgID != gw.OrgID || gwDecoded.WorkflowID != gw.WorkflowID || gwDecoded.Query != gw.Query || gwDecoded.Active != gw.Active {
		t.Fatalf("gmail watch round-trip mismatch: got=%+v want=%+v", gwDecoded, gw)
	}
}
