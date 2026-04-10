package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/config"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/integrations"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/models"
)

type apiMockProvider struct{}

func (apiMockProvider) ID() string              { return "gmail" }
func (apiMockProvider) DisplayName() string     { return "Gmail" }
func (apiMockProvider) IsConfigured() bool      { return true }
func (apiMockProvider) MissingFields() []string { return nil }
func (apiMockProvider) GetClient(context.Context, string) (*http.Client, error) {
	return &http.Client{}, nil
}
func (apiMockProvider) ListConnections(context.Context, string) ([]*models.OAuthToken, error) {
	return []*models.OAuthToken{{OrgID: "org_1", AccountID: "a1", IsPrimary: true}}, nil
}
func (apiMockProvider) Disconnect(context.Context, string) error                { return nil }
func (apiMockProvider) DisconnectAccount(context.Context, string, string) error { return nil }
func (apiMockProvider) IsNotConfiguredError(error) bool                         { return false }
func (apiMockProvider) IsReconnectRequiredError(error) bool                     { return false }
func (apiMockProvider) IsNotConnectedError(error) bool                          { return false }

func TestNormalizeServiceID(t *testing.T) {
	if got := normalizeServiceID(" Google-Forms "); got != "google_forms" {
		t.Fatalf("unexpected normalized service: %q", got)
	}
}

func TestWithServiceQueryAddsService(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/integrations/providers", nil)
	r2 := withServiceQuery(req, "gmail")
	if got := r2.URL.Query().Get("service"); got != "gmail" {
		t.Fatalf("expected service query, got %q", got)
	}
}

func TestHandleProvidersMethodNotAllowed(t *testing.T) {
	s := &Server{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/integrations/providers", nil)
	s.handleProviders(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleProvidersListsRegisteredProviders(t *testing.T) {
	reg := integrations.NewRegistry()
	reg.Register(apiMockProvider{})
	s := &Server{providers: reg}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/integrations/providers", nil)
	s.handleProviders(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["count"].(float64) != 1 {
		t.Fatalf("expected count=1, got %#v", body["count"])
	}
}

func TestHandleIntegrationAccountsUnauthorizedWithoutContext(t *testing.T) {
	reg := integrations.NewRegistry()
	reg.Register(apiMockProvider{})
	s := &Server{providers: reg}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/integration/accounts?service=gmail", nil)
	s.handleIntegrationAccounts(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleIntegrationAccountsHappyPath(t *testing.T) {
	reg := integrations.NewRegistry()
	reg.Register(apiMockProvider{})
	s := &Server{providers: reg}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/integration/accounts?service=gmail", nil)
	req = req.WithContext(context.WithValue(req.Context(), authorizedOrgIDKey, "org_1"))
	s.handleIntegrationAccounts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleIntegrationProviderRoutesUnsupportedService(t *testing.T) {
	s := &Server{cfg: &config.Config{}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/integrations/not-real/status", nil)
	s.handleIntegrationProviderRoutes(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestFormAndGmailWrappersReturnServiceUnavailableWhenMissingHandlers(t *testing.T) {
	s := &Server{}
	for _, tc := range []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
		path string
	}{
		{"forms", s.handleForms, "/forms"},
		{"formByPath", s.handleFormByPath, "/forms/abc"},
		{"watches", s.handleWatches, "/watches"},
		{"watchByID", s.handleWatchByID, "/watches/abc"},
		{"gmailSend", s.handleGmailSend, "/integrations/gmail/send"},
		{"gmailMessages", s.handleGmailMessages, "/integrations/gmail/messages"},
		{"gmailWatches", s.handleGmailWatches, "/integrations/gmail/watches"},
		{"gmailWatchByID", s.handleGmailWatchByID, "/integrations/gmail/watches/1"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		tc.fn(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s expected 503, got %d", tc.name, w.Code)
		}
	}
}
