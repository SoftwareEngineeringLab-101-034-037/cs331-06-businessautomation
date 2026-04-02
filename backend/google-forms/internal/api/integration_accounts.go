package api

import (
	"net/http"
	"net/url"
	"strings"
)

func (s *Server) handleIntegrationAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	orgID := authorizedOrgIDFromContext(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
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

	rawAccountID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/integration/accounts/"))
	decodedAccountID, err := url.PathUnescape(rawAccountID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account_id path")
		return
	}
	accountID := strings.TrimSpace(decodedAccountID)
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "account_id required")
		return
	}

	orgID := authorizedOrgIDFromContext(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	if err := s.oauthSvc.DisconnectAccount(r.Context(), orgID, accountID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
