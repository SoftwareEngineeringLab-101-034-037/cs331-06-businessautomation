package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/example/business-automation/backend/google-forms/internal/models"
	"github.com/example/business-automation/backend/google-forms/internal/storage"
)

var scopes = []string{
	"https://www.googleapis.com/auth/forms.body",
	"https://www.googleapis.com/auth/forms.responses.readonly",
	"https://www.googleapis.com/auth/drive.metadata.readonly",
	"https://www.googleapis.com/auth/drive.file",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

const providerGoogleForms = "google_forms"

type Service struct {
	cfg           *oauth2.Config
	store         storage.Store
	configured    bool
	missingFields []string
	stateMu       sync.Mutex
	authStates    map[string]pendingAuthState
}

type pendingAuthState struct {
	OrgID     string
	ExpiresAt time.Time
}

const authStateTTL = 10 * time.Minute

var ErrOAuthNotConfigured = errors.New("google oauth is not configured on server")
var ErrOAuthReconnectRequired = errors.New("google oauth reconnect required")

func NewService(clientID, clientSecret, redirectURI string, store storage.Store) *Service {
	missing := make([]string, 0, 3)
	if strings.TrimSpace(clientID) == "" {
		missing = append(missing, "GOOGLE_CLIENT_ID")
	}
	if strings.TrimSpace(clientSecret) == "" {
		missing = append(missing, "GOOGLE_CLIENT_SECRET")
	}
	if strings.TrimSpace(redirectURI) == "" {
		missing = append(missing, "GOOGLE_REDIRECT_URI")
	}

	configured := len(missing) == 0

	return &Service{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURI,
			Scopes:       scopes,
			Endpoint:     google.Endpoint,
		},
		store:         store,
		configured:    configured,
		missingFields: missing,
		authStates:    make(map[string]pendingAuthState),
	}
}

func (s *Service) IsConfigured() bool {
	return s.configured
}

func (s *Service) MissingFields() []string {
	out := make([]string, len(s.missingFields))
	copy(out, s.missingFields)
	return out
}

func IsNotConfiguredError(err error) bool {
	return errors.Is(err, ErrOAuthNotConfigured)
}

func IsReconnectRequiredError(err error) bool {
	return errors.Is(err, ErrOAuthReconnectRequired)
}

func (s *Service) AuthURL(orgID string) string {
	if !s.IsConfigured() {
		return ""
	}
	state, err := s.GenerateState(orgID)
	if err != nil {
		log.Printf("warn: failed to generate oauth state for org %s: %v", orgID, err)
		return ""
	}
	return s.cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

func (s *Service) Exchange(ctx context.Context, code, orgID string) error {
	if !s.IsConfigured() {
		return ErrOAuthNotConfigured
	}
	tok, err := s.cfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	client := s.cfg.Client(ctx, tok)
	accountID, accountEmail, accountName := s.fetchGoogleAccountDetails(ctx, client)
	if accountID == "" {
		if accountEmail != "" {
			accountID = strings.ToLower(accountEmail)
		} else {
			accountID = fmt.Sprintf("account-%d", time.Now().UnixNano())
		}
	}
	if accountName == "" {
		accountName = accountEmail
	}

	if tok.RefreshToken == "" {
		existing, getErr := s.store.GetTokenByAccount(ctx, orgID, providerGoogleForms, accountID)
		if getErr == nil && existing != nil {
			tok.RefreshToken = existing.RefreshToken
		}
	}

	isPrimary := false
	if existing, getErr := s.store.GetTokenByAccount(ctx, orgID, providerGoogleForms, accountID); getErr == nil && existing != nil && existing.IsPrimary {
		isPrimary = true
	} else {
		tokens, listErr := s.store.ListTokens(ctx, orgID, providerGoogleForms)
		if listErr == nil {
			if len(tokens) == 0 {
				isPrimary = true
			} else {
				hasPrimary := false
				for _, token := range tokens {
					if token.IsPrimary {
						hasPrimary = true
						break
					}
				}
				isPrimary = !hasPrimary
			}
		}
	}

	m := &models.OAuthToken{
		Provider:     providerGoogleForms,
		OrgID:        orgID,
		AccountID:    accountID,
		AccountEmail: accountEmail,
		AccountName:  accountName,
		IsPrimary:    isPrimary,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		Expiry:       tok.Expiry,
		Scopes:       scopes,
		ConnectedAt:  time.Now(),
	}
	return s.store.SaveToken(ctx, m)
}

// GetClient returns an HTTP client authenticated for the given org.
// It auto-refreshes the access token and persists the new token if refreshed.
func (s *Service) GetClient(ctx context.Context, orgID string) (*http.Client, error) {
	if !s.IsConfigured() {
		return nil, ErrOAuthNotConfigured
	}

	stored, err := s.store.GetToken(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, fmt.Errorf("no google connection for org %s", orgID)
	}
	if ok, missing := hasRequiredScopes(stored.Scopes, scopes); !ok {
		return nil, fmt.Errorf("%w: missing scopes %s", ErrOAuthReconnectRequired, strings.Join(missing, ", "))
	}

	tok := &oauth2.Token{
		AccessToken:  stored.AccessToken,
		RefreshToken: stored.RefreshToken,
		TokenType:    stored.TokenType,
		Expiry:       stored.Expiry,
	}

	fresh, err := s.cfg.TokenSource(ctx, tok).Token()
	if err != nil {
		if isRefreshReauthError(err) {
			if delErr := s.store.DeleteTokenByAccount(ctx, orgID, stored.Provider, stored.AccountID); delErr != nil {
				log.Printf("warn: delete invalid token for org %s: %v", orgID, delErr)
			}
			return nil, fmt.Errorf("%w: %v", ErrOAuthReconnectRequired, err)
		}
		return nil, fmt.Errorf("token refresh: %w", err)
	}

	if fresh.AccessToken != stored.AccessToken {
		stored.AccessToken = fresh.AccessToken
		stored.Expiry = fresh.Expiry
		if err := s.store.SaveToken(ctx, stored); err != nil {
			log.Printf("warn: persist refreshed token for org %s: %v", orgID, err)
		}
	}

	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(fresh)), nil
}

func (s *Service) Disconnect(ctx context.Context, orgID string) error {
	if !s.IsConfigured() {
		return nil
	}

	tokens, err := s.store.ListTokens(ctx, orgID, providerGoogleForms)
	if err != nil {
		return err
	}
	for _, stored := range tokens {
		resp, err := http.PostForm("https://oauth2.googleapis.com/revoke",
			url.Values{"token": {stored.AccessToken}})
		if err == nil {
			resp.Body.Close()
		}
	}
	return s.store.DeleteToken(ctx, orgID)
}

func (s *Service) ListConnections(ctx context.Context, orgID string) ([]*models.OAuthToken, error) {
	if !s.IsConfigured() {
		return []*models.OAuthToken{}, nil
	}
	return s.store.ListTokens(ctx, orgID, providerGoogleForms)
}

func (s *Service) DisconnectAccount(ctx context.Context, orgID, accountID string) error {
	if !s.IsConfigured() {
		return nil
	}
	if strings.TrimSpace(accountID) == "" {
		return fmt.Errorf("account_id required")
	}

	tok, err := s.store.GetTokenByAccount(ctx, orgID, providerGoogleForms, accountID)
	if err != nil {
		return err
	}
	if tok == nil {
		return nil
	}

	resp, revokeErr := http.PostForm("https://oauth2.googleapis.com/revoke",
		url.Values{"token": {tok.AccessToken}})
	if revokeErr == nil {
		resp.Body.Close()
	}

	if err := s.store.DeleteTokenByAccount(ctx, orgID, providerGoogleForms, accountID); err != nil {
		return err
	}

	if tok.IsPrimary {
		nextTokens, listErr := s.store.ListTokens(ctx, orgID, providerGoogleForms)
		if listErr == nil && len(nextTokens) > 0 {
			nextTokens[0].IsPrimary = true
			if saveErr := s.store.SaveToken(ctx, nextTokens[0]); saveErr != nil {
				log.Printf("warn: failed to persist replacement primary token for org %s account %s: %v", orgID, nextTokens[0].AccountID, saveErr)
			}
		}
	}

	return nil
}

func RegisterHandlers(mux *http.ServeMux, svc *Service, store storage.Store) {
	mux.HandleFunc("/auth/google/connect", func(w http.ResponseWriter, r *http.Request) {
		if !svc.IsConfigured() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"error":          ErrOAuthNotConfigured.Error(),
				"configured":     false,
				"missing_fields": svc.MissingFields(),
				"action":         "Set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URI in service environment, then restart.",
			})
			return
		}

		orgID := r.URL.Query().Get("org_id")
		if orgID == "" {
			writeError(w, http.StatusBadRequest, "org_id required")
			return
		}
		authURL := svc.AuthURL(orgID)
		if authURL == "" {
			writeError(w, http.StatusInternalServerError, "failed to initialize oauth authorization")
			return
		}
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	})

	mux.HandleFunc("/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		if !svc.IsConfigured() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"error":          ErrOAuthNotConfigured.Error(),
				"configured":     false,
				"missing_fields": svc.MissingFields(),
				"action":         "Set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URI in service environment, then restart.",
			})
			return
		}

		code := r.URL.Query().Get("code")
		stateToken := r.URL.Query().Get("state")
		if code == "" || stateToken == "" {
			writeError(w, http.StatusBadRequest, "missing code or state")
			return
		}
		orgID, err := svc.ValidateAndConsumeState(stateToken)
		if err != nil {
			writeError(w, http.StatusForbidden, "invalid or expired oauth state")
			return
		}
		if err := svc.Exchange(r.Context(), code, orgID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "connected", "org_id": orgID})
	})

	mux.HandleFunc("/auth/google/disconnect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		orgID := r.URL.Query().Get("org_id")
		accountID := r.URL.Query().Get("account_id")
		if orgID == "" {
			writeError(w, http.StatusBadRequest, "org_id required")
			return
		}
		var err error
		if accountID != "" {
			err = svc.DisconnectAccount(r.Context(), orgID, accountID)
		} else {
			err = svc.Disconnect(r.Context(), orgID)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/auth/google/status", func(w http.ResponseWriter, r *http.Request) {
		orgID := r.URL.Query().Get("org_id")
		if orgID == "" {
			writeError(w, http.StatusBadRequest, "org_id required")
			return
		}
		if !svc.IsConfigured() {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"configured":     false,
				"connected":      false,
				"missing_fields": svc.MissingFields(),
				"message":        "Google Forms integration needs admin setup before org admins can connect accounts.",
			})
			return
		}

		tok, err := store.GetToken(r.Context(), orgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if tok == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"configured": true, "connected": false})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"configured":    true,
			"connected":     true,
			"connected_at":  tok.ConnectedAt,
			"scopes":        tok.Scopes,
			"account_id":    tok.AccountID,
			"account_email": tok.AccountEmail,
			"account_name":  tok.AccountName,
		})
	})
}

func (s *Service) fetchGoogleAccountDetails(ctx context.Context, client *http.Client) (string, string, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return "", "", ""
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("warn: fetch google user info failed: %v", err)
		return "", "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("warn: fetch google user info returned %d", resp.StatusCode)
		return "", "", ""
	}

	var user struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		log.Printf("warn: decode google user info failed: %v", err)
		return "", "", ""
	}

	return user.Sub, user.Email, user.Name
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func hasRequiredScopes(granted, required []string) (bool, []string) {
	if len(required) == 0 {
		return true, nil
	}
	grantedSet := make(map[string]struct{}, len(granted))
	for _, scope := range granted {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}
		grantedSet[trimmed] = struct{}{}
	}
	missing := make([]string, 0)
	for _, scope := range required {
		if _, ok := grantedSet[scope]; !ok {
			missing = append(missing, scope)
		}
	}
	return len(missing) == 0, missing
}

func (s *Service) GenerateState(orgID string) (string, error) {
	if strings.TrimSpace(orgID) == "" {
		return "", fmt.Errorf("org_id required")
	}

	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate state bytes: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(raw)

	now := time.Now()
	s.stateMu.Lock()
	for key, pending := range s.authStates {
		if pending.ExpiresAt.Before(now) {
			delete(s.authStates, key)
		}
	}
	s.authStates[state] = pendingAuthState{OrgID: orgID, ExpiresAt: now.Add(authStateTTL)}
	s.stateMu.Unlock()

	return state, nil
}

func (s *Service) ValidateAndConsumeState(state string) (string, error) {
	trimmed := strings.TrimSpace(state)
	if trimmed == "" {
		return "", fmt.Errorf("state is required")
	}

	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	pending, ok := s.authStates[trimmed]
	if !ok {
		return "", fmt.Errorf("state not found")
	}
	delete(s.authStates, trimmed)

	if time.Now().After(pending.ExpiresAt) {
		return "", fmt.Errorf("state expired")
	}
	if strings.TrimSpace(pending.OrgID) == "" {
		return "", fmt.Errorf("state missing org")
	}
	return pending.OrgID, nil
}

// isRefreshReauthError uses string matching because golang.org/x/oauth2 does
// not expose structured error types for Google OAuth token refresh responses.
// It intentionally detects unauthorized_client, invalid_grant, and
// invalid_client to trigger reconnect flows.
func isRefreshReauthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unauthorized_client") ||
		strings.Contains(msg, "invalid_grant") ||
		strings.Contains(msg, "invalid_client")
}
