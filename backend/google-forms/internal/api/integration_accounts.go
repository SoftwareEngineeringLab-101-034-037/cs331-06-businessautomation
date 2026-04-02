package api

import (
	"net/http"
	"strings"
)

func (s *Server) handleIntegrationAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org_id required")
		return
	}

	service := strings.TrimSpace(r.URL.Query().Get("service"))
	if service == "" {
		service = "google_forms"
	}
	if service != "google_forms" {
		writeError(w, http.StatusBadRequest, "unsupported service")
		return
	}

	accounts, err := s.oauthSvc.ListConnections(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service": service,
		"count":   len(accounts),
		"items":   accounts,
	})
}

func (s *Server) handleIntegrationAccountByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	accountID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/integration/accounts/"))
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "account_id required")
		return
	}

	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org_id required")
		return
	}

	if err := s.oauthSvc.DisconnectAccount(r.Context(), orgID, accountID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
