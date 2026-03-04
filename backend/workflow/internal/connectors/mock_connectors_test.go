package connectors

import "testing"

func TestMockEmail(t *testing.T) {
	c := NewMockEmail()
	if c == nil {
		t.Fatalf("expected non-nil connector")
	}
	if err := c.Send("to@example.com", "subject", "body"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
}

func TestMockWebhook(t *testing.T) {
	c := NewMockWebhook()
	if c == nil {
		t.Fatalf("expected non-nil connector")
	}
	if err := c.Post("https://example.com", map[string]interface{}{"k": "v"}); err != nil {
		t.Fatalf("Post returned error: %v", err)
	}
}

func TestMockForm(t *testing.T) {
	c := NewMockForm()
	formID, err := c.CreateForm("Title", []string{"a", "b"})
	if err != nil {
		t.Fatalf("CreateForm returned error: %v", err)
	}
	if formID != "mock-form-001" {
		t.Fatalf("unexpected form ID: %s", formID)
	}

	submission, err := c.GetSubmission(formID)
	if err != nil {
		t.Fatalf("GetSubmission returned error: %v", err)
	}
	if submission["department"] != "Engineering" {
		t.Fatalf("unexpected submission: %+v", submission)
	}
}

func TestMockPayment(t *testing.T) {
	c := NewMockPayment()
	txnID, err := c.ProcessPayment(100.25, "USD", "acct-1")
	if err != nil {
		t.Fatalf("ProcessPayment returned error: %v", err)
	}
	if txnID != "txn-mock-001" {
		t.Fatalf("unexpected txn ID: %s", txnID)
	}
}
