package connectors

import "log"

// PaymentConnector processes payment API calls.
type PaymentConnector interface {
	ProcessPayment(amount float64, currency string, recipient string) (txnID string, err error)
}

// MockPayment logs payment operations instead of processing them.
type MockPayment struct{}

func NewMockPayment() *MockPayment { return &MockPayment{} }

func (m *MockPayment) ProcessPayment(amount float64, currency, recipient string) (string, error) {
	txnID := "txn-mock-001"
	log.Printf("[mock-payment] amount=%.2f %s to=%s txn=%s", amount, currency, recipient, txnID)
	return txnID, nil
}
