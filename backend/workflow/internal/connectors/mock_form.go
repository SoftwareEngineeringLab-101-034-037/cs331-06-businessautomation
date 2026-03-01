package connectors

import "log"

// FormConnector integrates with form services (Google Forms, etc.)
type FormConnector interface {
	CreateForm(title string, fields []string) (formID string, err error)
	GetSubmission(formID string) (map[string]interface{}, error)
}

// MockForm logs form operations instead of calling real APIs.
type MockForm struct{}

func NewMockForm() *MockForm { return &MockForm{} }

func (m *MockForm) CreateForm(title string, fields []string) (string, error) {
	id := "mock-form-001"
	log.Printf("[mock-form] created form=%s title=%q fields=%v", id, title, fields)
	return id, nil
}

func (m *MockForm) GetSubmission(formID string) (map[string]interface{}, error) {
	log.Printf("[mock-form] fetching submission for form=%s", formID)
	return map[string]interface{}{
		"employee":   "john.doe@company.com",
		"amount":     250.00,
		"department": "Engineering",
		"receipt":    "receipt_001.pdf",
	}, nil
}
