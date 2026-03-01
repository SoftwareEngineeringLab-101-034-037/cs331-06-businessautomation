package connectors

import "log"

// EmailConnector sends emails.
type EmailConnector interface {
Send(to, subject, body string) error
}

// MockEmail logs emails instead of sending them.
type MockEmail struct{}

func NewMockEmail() *MockEmail { return &MockEmail{} }

func (m *MockEmail) Send(to, subject, body string) error {
log.Printf("[mock-email] to=%s subject=%q body=%q", to, subject, body)
return nil
}
