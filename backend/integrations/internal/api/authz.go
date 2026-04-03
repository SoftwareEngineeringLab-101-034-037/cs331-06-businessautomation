package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type orgContextKey string

const authorizedOrgIDKey orgContextKey = "authorized_org_id"

func (s *Server) withOrgAuthorization(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		ctx := context.WithValue(r.Context(), authorizedOrgIDKey, orgID)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) authorizeOrgAccess(r *http.Request, orgID string) (int, string) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return http.StatusUnauthorized, "missing or invalid authorization header"
	}

	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.AuthServiceURL), "/")
	if baseURL == "" {
		return http.StatusServiceUnavailable, "auth service URL is not configured"
	}

	verifyURL := fmt.Sprintf("%s/api/orgs/%s/roles", baseURL, url.PathEscape(orgID))
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, verifyURL, nil)
	if err != nil {
		return http.StatusInternalServerError, "failed to build auth verification request"
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return http.StatusBadGateway, "failed to verify org access"
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return 0, ""
	case http.StatusUnauthorized:
		return http.StatusUnauthorized, "invalid or expired authorization token"
	case http.StatusForbidden, http.StatusNotFound:
		return http.StatusForbidden, "forbidden for org"
	default:
		return http.StatusBadGateway, "auth service verification failed"
	}
}

func authorizedOrgIDFromContext(ctx context.Context) string {
	orgID, _ := ctx.Value(authorizedOrgIDKey).(string)
	return strings.TrimSpace(orgID)
}
