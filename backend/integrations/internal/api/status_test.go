package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/config"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/integrations"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/models"
)

type statusMockProvider struct{}

func (statusMockProvider) ID() string              { return "gmail" }
func (statusMockProvider) DisplayName() string     { return "Gmail" }
func (statusMockProvider) IsConfigured() bool      { return true }
func (statusMockProvider) MissingFields() []string { return nil }
func (statusMockProvider) GetClient(context.Context, string) (*http.Client, error) {
	return &http.Client{}, nil
}
func (statusMockProvider) ListConnections(context.Context, string) ([]*models.OAuthToken, error) {
	return []*models.OAuthToken{{OrgID: "org_1", AccountID: "primary", IsPrimary: true}}, nil
}
func (statusMockProvider) Disconnect(context.Context, string) error                { return nil }
func (statusMockProvider) DisconnectAccount(context.Context, string, string) error { return nil }
func (statusMockProvider) IsNotConfiguredError(error) bool                         { return false }
func (statusMockProvider) IsReconnectRequiredError(error) bool                     { return false }
func (statusMockProvider) IsNotConnectedError(error) bool                          { return false }

func TestHandleIntegrationStatusHappyPath(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer authServer.Close()

	workflowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("unexpected workflow path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer workflowServer.Close()

	s := &Server{
		cfg: &config.Config{AuthServiceURL: authServer.URL, WorkflowEngineURL: workflowServer.URL},
		providers: func() *integrations.Registry {
			r := integrations.NewRegistry()
			r.Register(statusMockProvider{})
			return r
		}(),
		httpClient: workflowServer.Client(),
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/integration/status?org_id=org_1&service=gmail", nil)
	req.Header.Set("Authorization", "Bearer token")
	s.handleIntegrationStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
}
