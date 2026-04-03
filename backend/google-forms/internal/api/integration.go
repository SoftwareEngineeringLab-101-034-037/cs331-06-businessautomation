package api

import (
	"net/http"
	"strings"

	"github.com/example/business-automation/backend/google-forms/internal/oauth"
)

func (s *Server) handleIntegrationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if orgID != "" {
		status, msg := s.authorizeOrgAccess(r, orgID)
		if status != 0 {
			writeError(w, status, msg)
			return
		}
	}

	workflowHealthy := false
	workflowErr := ""
	workflowURL := strings.TrimRight(s.cfg.WorkflowEngineURL, "/") + "/health"
	client := s.httpClient
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Get(workflowURL)
	if err != nil {
		workflowErr = err.Error()
	} else {
		_ = resp.Body.Close()
		workflowHealthy = resp.StatusCode >= 200 && resp.StatusCode < 300
		if !workflowHealthy {
			workflowErr = "workflow health check returned non-2xx"
		}
	}

	out := map[string]interface{}{
		"service":                 "google_forms",
		"configured":              s.oauthSvc.IsConfigured(),
		"missing_fields":          s.oauthSvc.MissingFields(),
		"workflow_engine_url":     s.cfg.WorkflowEngineURL,
		"workflow_engine_healthy": workflowHealthy,
		"workflow_engine_error":   workflowErr,
	}

	if orgID != "" && s.oauthSvc.IsConfigured() {
		accounts, listErr := s.oauthSvc.ListConnections(r.Context(), orgID)
		if listErr != nil {
			out["accounts_error"] = listErr.Error()
		} else {
			out["connected_accounts"] = len(accounts)
		}

		tok, tokenErr := s.store.GetToken(r.Context(), orgID)
		if tokenErr != nil {
			out["token_lookup_error"] = tokenErr.Error()
		} else if tok != nil {
			out["connected"] = true
			out["connected_at"] = tok.ConnectedAt
			out["scopes"] = tok.Scopes
			out["primary_account_id"] = tok.AccountID
			out["primary_account_email"] = tok.AccountEmail
			out["primary_account_name"] = tok.AccountName

			if _, clientErr := s.oauthSvc.GetClient(r.Context(), orgID); clientErr != nil {
				out["connected"] = false
				out["oauth_error"] = clientErr.Error()
				if oauth.IsReconnectRequiredError(clientErr) {
					out["reconnect_required"] = true
					out["reconnect_message"] = "Stored Google token is no longer valid. Reconnect Google Forms from Integrations."
				}
			}
		} else {
			out["connected"] = false
		}
	}

	writeJSON(w, http.StatusOK, out)
}
