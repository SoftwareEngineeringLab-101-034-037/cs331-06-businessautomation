package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/business-automation/backend/integrations/internal/config"
)

func TestWithOrgAuthorizationRequiresOrgID(t *testing.T) {
	s := &Server{cfg: &config.Config{}, httpClient: &http.Client{}}

	h := s.withOrgAuthorization(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/watches", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWithOrgAuthorizationRejectsMissingAuthorizationHeader(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer authServer.Close()

	s := &Server{
		cfg:        &config.Config{AuthServiceURL: authServer.URL},
		httpClient: authServer.Client(),
	}

	h := s.withOrgAuthorization(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/watches?org_id=org_1", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestWithOrgAuthorizationAllowsAuthorizedOrg(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer authServer.Close()

	s := &Server{
		cfg:        &config.Config{AuthServiceURL: authServer.URL},
		httpClient: authServer.Client(),
	}

	h := s.withOrgAuthorization(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/watches?org_id=org_1", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestWithOrgAuthorizationOrIntegrationKeyAcceptsValidKey(t *testing.T) {
	s := &Server{
		cfg:        &config.Config{WorkflowServiceKey: "secret-key"},
		httpClient: &http.Client{},
	}

	h := s.withOrgAuthorizationOrIntegrationKey(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/watches?org_id=org_1", nil)
	req.Header.Set("X-Integration-Key", "secret-key")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestWithOrgAuthorizationOrIntegrationKeyRejectsInvalidKey(t *testing.T) {
	s := &Server{
		cfg:        &config.Config{WorkflowServiceKey: "secret-key"},
		httpClient: &http.Client{},
	}

	h := s.withOrgAuthorizationOrIntegrationKey(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/watches?org_id=org_1", nil)
	req.Header.Set("X-Integration-Key", "wrong")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
