package api

import (
	"net/http"
	"sort"
	"strings"

	providergmail "github.com/example/business-automation/backend/integrations/internal/providers/gmail"
)

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.providers == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"count": 0, "items": []map[string]interface{}{}})
		return
	}

	ids := s.providers.IDs()
	sort.Strings(ids)
	items := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		provider, ok := s.providers.Get(id)
		if !ok || provider == nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"id":             provider.ID(),
			"name":           provider.DisplayName(),
			"configured":     provider.IsConfigured(),
			"missing_fields": provider.MissingFields(),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count": len(items),
		"items": items,
	})
}

func (s *Server) handleIntegrationProviderRoutes(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/integrations/"), "/")
	if trimmed == "" {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	service := normalizeServiceID(parts[0])
	if service == "" {
		writeError(w, http.StatusBadRequest, "service required")
		return
	}
	if _, ok := s.providerByService(service); !ok {
		writeError(w, http.StatusBadRequest, "unsupported service")
		return
	}

	switch parts[1] {
	case "status":
		if len(parts) != 2 {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		s.handleIntegrationStatus(w, withServiceQuery(r, service))
		return
	case "accounts":
		if len(parts) == 2 {
			s.withOrgAuthorization(s.handleIntegrationAccounts)(w, withServiceQuery(r, service))
			return
		}
		if len(parts) == 3 {
			r2 := withServiceQuery(r, service)
			u := *r2.URL
			u.Path = "/integration/accounts/" + parts[2]
			r2.URL = &u
			s.withOrgAuthorization(s.handleIntegrationAccountByID)(w, r2)
			return
		}
		writeError(w, http.StatusNotFound, "route not found")
		return
	case "send":
		if service != providergmail.ProviderID || len(parts) != 2 {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		s.withOrgAuthorizationOrIntegrationKey(s.handleGmailSend)(w, r)
		return
	case "messages":
		if service != providergmail.ProviderID || len(parts) != 2 {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		s.withOrgAuthorization(s.handleGmailMessages)(w, r)
		return
	case "watches":
		if service != providergmail.ProviderID {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		if len(parts) == 2 {
			s.withOrgAuthorization(s.handleGmailWatches)(w, r)
			return
		}
		if len(parts) == 3 {
			s.withOrgAuthorization(s.handleGmailWatchByID)(w, r)
			return
		}
		writeError(w, http.StatusNotFound, "route not found")
		return
	default:
		writeError(w, http.StatusNotFound, "route not found")
		return
	}
}

func withServiceQuery(r *http.Request, service string) *http.Request {
	r2 := r.Clone(r.Context())
	u := *r.URL
	q := u.Query()
	q.Set("service", service)
	u.RawQuery = q.Encode()
	r2.URL = &u
	return r2
}
