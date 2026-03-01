package connectors

import "log"

// WebhookConnector calls external HTTP endpoints.
type WebhookConnector interface {
	Post(url string, payload map[string]interface{}) error
}

// MockWebhook logs webhook calls instead of making them.
type MockWebhook struct{}

func NewMockWebhook() *MockWebhook { return &MockWebhook{} }

func (m *MockWebhook) Post(url string, payload map[string]interface{}) error {
	log.Printf("[mock-webhook] POST %s payload=%v", url, payload)
	return nil
}
