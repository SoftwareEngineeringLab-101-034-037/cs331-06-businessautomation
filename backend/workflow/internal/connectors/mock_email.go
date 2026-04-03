package connectors

import "log"

// EmailConnector sends emails.
type EmailConnector interface {
	Send(to, subject, body string) error
}

// OrgEmailConnector supports org-aware sending for connectors backed by
// integration services that need org-level context.
type OrgEmailConnector interface {
	SendForOrg(orgID, to, cc, subject, body, fromName, fromAccountID string) error
}

// MockEmail logs emails instead of sending them.
type MockEmail struct{}

func NewMockEmail() *MockEmail { return &MockEmail{} }

func (m *MockEmail) Send(to, subject, body string) error {
	log.Printf("[mock-email] to=%s subject=%q body_len=%d", to, subject, len(body))
	return nil
}
