package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/example/business-automation/backend/integrations/internal/models"
	providergoogleforms "github.com/example/business-automation/backend/integrations/internal/providers/googleforms"
)

func (s *Server) handleIntegrationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org_id required")
		return
	}
	status, msg := s.authorizeOrgAccess(r, orgID)
	if status != 0 {
		writeError(w, status, msg)
		return
	}
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	if service == "" {
		service = providergoogleforms.ProviderID
	}
	provider, ok := s.providerByService(service)
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported service")
		return
	}

	workflowHealthy := false
	workflowErr := ""
	workflowURL := strings.TrimRight(s.cfg.WorkflowEngineURL, "/") + "/health"
	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Get(workflowURL)
	if err != nil {
		log.Printf("integration status workflow health check failed url=%q: %v", workflowURL, err)
		workflowErr = "upstream service error"
	} else {
		_ = resp.Body.Close()
		workflowHealthy = resp.StatusCode >= 200 && resp.StatusCode < 300
		if !workflowHealthy {
			log.Printf("integration status workflow health returned status=%d url=%q", resp.StatusCode, workflowURL)
			workflowErr = "upstream service error"
		}
	}

	out := map[string]interface{}{
		"service":                 provider.ID(),
		"configured":              provider.IsConfigured(),
		"missing_fields":          provider.MissingFields(),
		"workflow_engine":         "redacted",
		"workflow_engine_healthy": workflowHealthy,
		"workflow_engine_error":   workflowErr,
	}

	if provider.IsConfigured() {
		accounts, listErr := provider.ListConnections(r.Context(), orgID)
		if listErr != nil {
			log.Printf("integration status list connections failed org_id=%q: %v", orgID, listErr)
			out["accounts_error"] = "internal processing error"
		} else {
			out["connected_accounts"] = len(accounts)

			var primary *models.OAuthToken
			for _, account := range accounts {
				if account != nil && account.IsPrimary {
					primary = account
					break
				}
			}
			if primary == nil && len(accounts) > 0 {
				primary = accounts[0]
			}

			if primary != nil {
				out["connected"] = true
				out["connected_at"] = primary.ConnectedAt
				out["scopes"] = primary.Scopes
				out["primary_account_id"] = primary.AccountID
				out["primary_account_email"] = primary.AccountEmail
				out["primary_account_name"] = primary.AccountName

				if _, clientErr := provider.GetClient(r.Context(), orgID); clientErr != nil {
					log.Printf("integration status oauth client validation failed service=%q org_id=%q: %v", provider.ID(), orgID, clientErr)
					out["connected"] = false
					out["oauth_error"] = "upstream service error"
					if provider.IsReconnectRequiredError(clientErr) {
						out["reconnect_required"] = true
						out["reconnect_message"] = "Stored integration token is no longer valid. Reconnect from Integrations."
					}
				}
			} else {
				out["connected"] = false
			}
		}
	}

	writeJSON(w, http.StatusOK, out)
}
