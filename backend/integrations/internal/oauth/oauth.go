package oauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/models"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/storage"
)

var identityScopes = []string{
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

var providerScopes = map[string][]string{
	providerGoogleForms: {
		"https://www.googleapis.com/auth/forms.body",
		"https://www.googleapis.com/auth/forms.responses.readonly",
		"https://www.googleapis.com/auth/drive.metadata.readonly",
		"https://www.googleapis.com/auth/drive.file",
	},
	providerGmail: {
		"https://www.googleapis.com/auth/gmail.readonly",
		"https://www.googleapis.com/auth/gmail.send",
	},
}

const providerGoogleForms = "google_forms"
const providerGmail = "gmail"

type Service struct {
	cfg             *oauth2.Config
	store           storage.Store
	configured      bool
	missingFields   []string
	stateSigningKey []byte
}

type signedAuthState struct {
	OrgID     string `json:"org_id"`
	UserID    string `json:"user_id"`
	Provider  string `json:"provider,omitempty"`
	ExpiresAt int64  `json:"exp"`
	Nonce     string `json:"nonce"`
}

const authStateTTL = 10 * time.Minute
const oauthActorCookieName = "gf_oauth_actor"

var ErrOAuthNotConfigured = errors.New("google oauth is not configured on server")
var ErrOAuthReconnectRequired = errors.New("google oauth reconnect required")
var ErrOAuthNotConnected = errors.New("google oauth not connected")
var ErrOAuthAccountNotFound = errors.New("google oauth account not found")
var ErrOAuthInvalidProvider = errors.New("invalid oauth provider")

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
			Scopes:       requiredScopesForProvider(providerGoogleForms),
			Endpoint:     google.Endpoint,
		},
		store:           store,
		configured:      configured,
		missingFields:   missing,
		stateSigningKey: buildStateSigningKey(clientSecret),
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

func IsNotConnectedError(err error) bool {
	return errors.Is(err, ErrOAuthNotConnected) || errors.Is(err, ErrOAuthAccountNotFound)
}

func IsAccountNotFoundError(err error) bool {
	return errors.Is(err, ErrOAuthAccountNotFound)
}

func requiredScopesForProvider(provider string) []string {
	resolved := normalizeProviderID(provider)
	out := make([]string, 0, len(identityScopes)+4)
	out = append(out, identityScopes...)
	if specific, ok := providerScopes[resolved]; ok {
		out = append(out, specific...)
	}
	return out
}

func normalizeProviderID(provider string) string {
	resolved := strings.TrimSpace(strings.ToLower(provider))
	resolved = strings.ReplaceAll(resolved, "-", "_")
	switch resolved {
	case providerGoogleForms, "googleforms", "forms":
		return providerGoogleForms
	case providerGmail:
		return providerGmail
	default:
		return ""
	}
}

func (s *Service) AuthURL(orgID, userID string) string {
	return s.AuthURLForProvider(orgID, userID, providerGoogleForms)
}

func (s *Service) AuthURLForProvider(orgID, userID, provider string) string {
	if !s.IsConfigured() {
		return ""
	}
	provider = normalizeProviderID(provider)
	if provider == "" {
		return ""
	}
	state, err := s.GenerateState(orgID, userID, provider)
	if err != nil {
		log.Printf("warn: failed to generate oauth state for org %s: %v", orgID, err)
		return ""
	}

	providerCfg := *s.cfg
	providerCfg.Scopes = requiredScopesForProvider(provider)
	return providerCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

func (s *Service) Exchange(ctx context.Context, code, orgID string) error {
	return s.ExchangeForProvider(ctx, code, orgID, providerGoogleForms)
}

func (s *Service) ExchangeForProvider(ctx context.Context, code, orgID, provider string) error {
	if !s.IsConfigured() {
		return ErrOAuthNotConfigured
	}
	provider = normalizeProviderID(provider)
	if provider == "" {
		return ErrOAuthInvalidProvider
	}
	tok, err := s.cfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	grantedScopes := parseTokenScopes(tok)
	if ok, missing := hasRequiredScopes(grantedScopes, requiredScopesForProvider(provider)); !ok {
		return fmt.Errorf("missing required granted scopes %s", strings.Join(missing, ", "))
	}

	client := s.cfg.Client(ctx, tok)
	accountID, accountEmail, accountName := s.fetchGoogleAccountDetails(ctx, client)
	if accountID == "" {
		if accountEmail != "" {
			accountID = strings.ToLower(accountEmail)
		} else {
			log.Printf("oauth.exchange missing stable account identifier org_id=%q: google user info returned empty sub and email", orgID)
			return fmt.Errorf("missing stable account identifier from google user info")
		}
	}
	if accountName == "" {
		accountName = accountEmail
	}

	if tok.RefreshToken == "" {
		existing, getErr := s.store.GetTokenByAccount(ctx, orgID, provider, accountID)
		if getErr == nil && existing != nil {
			tok.RefreshToken = existing.RefreshToken
		}
	}

	isPrimary := false
	if existing, getErr := s.store.GetTokenByAccount(ctx, orgID, provider, accountID); getErr == nil && existing != nil && existing.IsPrimary {
		isPrimary = true
	} else {
		tokens, listErr := s.store.ListTokens(ctx, orgID, provider)
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
		Provider:     provider,
		OrgID:        orgID,
		AccountID:    accountID,
		AccountEmail: accountEmail,
		AccountName:  accountName,
		IsPrimary:    isPrimary,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		Expiry:       tok.Expiry,
		Scopes:       grantedScopes,
		ConnectedAt:  time.Now(),
	}
	return s.store.SaveToken(ctx, m)
}

// GetClient returns an HTTP client authenticated for the given org.
// It auto-refreshes the access token and persists the new token if refreshed.
func (s *Service) GetClient(ctx context.Context, orgID string) (*http.Client, error) {
	return s.GetClientForProvider(ctx, orgID, providerGoogleForms)
}

func (s *Service) GetClientForProvider(ctx context.Context, orgID, provider string) (*http.Client, error) {
	return s.GetClientForProviderAndAccount(ctx, orgID, provider, "")
}

func (s *Service) GetClientForProviderAndAccount(ctx context.Context, orgID, provider, accountHint string) (*http.Client, error) {
	if !s.IsConfigured() {
		return nil, ErrOAuthNotConfigured
	}
	provider = normalizeProviderID(provider)
	if provider == "" {
		return nil, ErrOAuthInvalidProvider
	}

	stored, err := s.selectStoredToken(ctx, orgID, provider, accountHint)
	if err != nil {
		return nil, err
	}

	if ok, missing := hasRequiredScopes(stored.Scopes, requiredScopesForProvider(provider)); !ok {
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

func (s *Service) selectStoredToken(ctx context.Context, orgID, provider, accountHint string) (*models.OAuthToken, error) {
	tokens, err := s.store.ListTokens(ctx, orgID, provider)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, ErrOAuthNotConnected
	}

	hint := strings.ToLower(strings.TrimSpace(accountHint))
	if hint == "primary" || hint == "default" {
		hint = ""
	}
	if hint != "" {
		for _, tok := range tokens {
			if tok == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(tok.AccountID), hint) || strings.EqualFold(strings.TrimSpace(tok.AccountEmail), hint) {
				return tok, nil
			}
		}
		return nil, ErrOAuthAccountNotFound
	}

	for _, tok := range tokens {
		if tok != nil && tok.IsPrimary {
			return tok, nil
		}
	}

	for _, tok := range tokens {
		if tok != nil {
			return tok, nil
		}
	}

	return nil, ErrOAuthNotConnected
}

func (s *Service) Disconnect(ctx context.Context, orgID string) error {
	return s.DisconnectForProvider(ctx, orgID, providerGoogleForms)
}

func (s *Service) DisconnectForProvider(ctx context.Context, orgID, provider string) error {
	if !s.IsConfigured() {
		return nil
	}
	provider = normalizeProviderID(provider)
	if provider == "" {
		return ErrOAuthInvalidProvider
	}

	tokens, err := s.store.ListTokens(ctx, orgID, provider)
	if err != nil {
		return err
	}
	for _, stored := range tokens {
		for _, token := range []string{stored.AccessToken, stored.RefreshToken} {
			if strings.TrimSpace(token) == "" {
				continue
			}
			if err := revokeToken(ctx, token); err != nil {
				log.Printf("warn: revoke token failed org_id=%q account_id=%q: %v", orgID, stored.AccountID, err)
			}
		}
	}

	for _, stored := range tokens {
		if stored == nil {
			continue
		}
		if err := s.store.DeleteTokenByAccount(ctx, orgID, provider, stored.AccountID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListConnections(ctx context.Context, orgID string) ([]*models.OAuthToken, error) {
	return s.ListConnectionsForProvider(ctx, orgID, providerGoogleForms)
}

func (s *Service) ListConnectionsForProvider(ctx context.Context, orgID, provider string) ([]*models.OAuthToken, error) {
	if !s.IsConfigured() {
		return []*models.OAuthToken{}, nil
	}
	provider = normalizeProviderID(provider)
	if provider == "" {
		return nil, ErrOAuthInvalidProvider
	}
	return s.store.ListTokens(ctx, orgID, provider)
}

func (s *Service) DisconnectAccount(ctx context.Context, orgID, accountID string) error {
	return s.DisconnectAccountForProvider(ctx, orgID, providerGoogleForms, accountID)
}

func (s *Service) DisconnectAccountForProvider(ctx context.Context, orgID, provider, accountID string) error {
	if !s.IsConfigured() {
		return nil
	}
	provider = normalizeProviderID(provider)
	if provider == "" {
		return ErrOAuthInvalidProvider
	}
	if strings.TrimSpace(accountID) == "" {
		return fmt.Errorf("account_id required")
	}

	tok, err := s.store.GetTokenByAccount(ctx, orgID, provider, accountID)
	if err != nil {
		return err
	}
	if tok == nil {
		return nil
	}

	for _, token := range []string{tok.AccessToken, tok.RefreshToken} {
		if strings.TrimSpace(token) == "" {
			continue
		}
		if revokeErr := revokeToken(ctx, token); revokeErr != nil {
			log.Printf("warn: revoke token failed org_id=%q account_id=%q: %v", orgID, accountID, revokeErr)
		}
	}

	if err := s.store.DeleteTokenByAccount(ctx, orgID, provider, accountID); err != nil {
		return err
	}

	if tok.IsPrimary {
		nextTokens, listErr := s.store.ListTokens(ctx, orgID, provider)
		if listErr == nil && len(nextTokens) > 0 {
			nextTokens[0].IsPrimary = true
			if saveErr := s.store.SaveToken(ctx, nextTokens[0]); saveErr != nil {
				log.Printf("warn: failed to persist replacement primary token for org %s account %s: %v", orgID, nextTokens[0].AccountID, saveErr)
			}
		}
	}

	return nil
}

func RegisterHandlers(mux *http.ServeMux, svc *Service, store storage.Store, trustedFrontendOrigins []string) {
	authorizeOrg := func(r *http.Request, orgID string) (int, string) {
		return authorizeOrgAccess(r, orgID)
	}

	resolveProviderOrDefault := func(rawService string) (string, bool) {
		rawService = strings.TrimSpace(rawService)
		if rawService == "" {
			return providerGoogleForms, false
		}
		provider := normalizeProviderID(rawService)
		if provider == "" {
			return "", true
		}
		return provider, false
	}

	mux.HandleFunc("/auth/google/connect-url", func(w http.ResponseWriter, r *http.Request) {
		cookieSecure := r.TLS != nil
		if !svc.IsConfigured() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"error":          ErrOAuthNotConfigured.Error(),
				"configured":     false,
				"missing_fields": svc.MissingFields(),
				"action":         "Set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URI in service environment, then restart.",
			})
			return
		}

		orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
		provider, invalidProvider := resolveProviderOrDefault(r.URL.Query().Get("service"))
		if invalidProvider {
			writeError(w, http.StatusBadRequest, "unsupported service")
			return
		}
		if orgID == "" {
			writeError(w, http.StatusBadRequest, "org_id required")
			return
		}
		status, msg := authorizeOrg(r, orgID)
		if status != 0 {
			writeError(w, status, msg)
			return
		}

		userID, err := extractBearerUserID(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid authorization token")
			return
		}

		authURL := svc.AuthURLForProvider(orgID, userID, provider)
		if authURL == "" {
			writeError(w, http.StatusInternalServerError, "failed to initialize oauth authorization")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     oauthActorCookieName,
			Value:    userID,
			Path:     "/",
			HttpOnly: true,
			Secure:   cookieSecure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(authStateTTL.Seconds()),
		})

		writeJSON(w, http.StatusOK, map[string]string{"auth_url": authURL, "service": provider})
	})

	mux.HandleFunc("/auth/google/connect", func(w http.ResponseWriter, r *http.Request) {
		cookieSecure := r.TLS != nil
		if !svc.IsConfigured() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"error":          ErrOAuthNotConfigured.Error(),
				"configured":     false,
				"missing_fields": svc.MissingFields(),
				"action":         "Set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URI in service environment, then restart.",
			})
			return
		}

		orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
		provider, invalidProvider := resolveProviderOrDefault(r.URL.Query().Get("service"))
		if invalidProvider {
			writeError(w, http.StatusBadRequest, "unsupported service")
			return
		}
		if orgID == "" {
			writeError(w, http.StatusBadRequest, "org_id required")
			return
		}
		status, msg := authorizeOrg(r, orgID)
		if status != 0 {
			writeError(w, status, msg)
			return
		}

		userID, err := extractBearerUserID(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid authorization token")
			return
		}

		authURL := svc.AuthURLForProvider(orgID, userID, provider)
		if authURL == "" {
			writeError(w, http.StatusInternalServerError, "failed to initialize oauth authorization")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     oauthActorCookieName,
			Value:    userID,
			Path:     "/",
			HttpOnly: true,
			Secure:   cookieSecure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(authStateTTL.Seconds()),
		})
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	})

	mux.HandleFunc("/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		cookieSecure := r.TLS != nil
		wantsJSON := wantsJSONResponse(r)
		frontendOrigin := resolveTrustedFrontendOrigin(r, trustedFrontendOrigins)
		writeCallbackError := func(status int, publicMessage, internalMessage, orgID, provider string) {
			if strings.TrimSpace(internalMessage) != "" {
				log.Printf("oauth.callback error status=%d org_id=%q provider=%q detail=%s", status, orgID, provider, internalMessage)
			}
			if wantsJSON {
				message := publicMessage
				if strings.TrimSpace(internalMessage) != "" {
					message = internalMessage
				}
				writeError(w, status, message)
				return
			}
			writeOAuthCallbackPage(w, status, frontendOrigin, map[string]interface{}{
				"type":    "integration_oauth_result",
				"status":  "error",
				"org_id":  orgID,
				"service": provider,
				"error":   publicMessage,
				"message": "Connection failed. You can close this window and return to the app.",
			})
		}
		if !svc.IsConfigured() {
			if wantsJSON {
				writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
					"error":          ErrOAuthNotConfigured.Error(),
					"configured":     false,
					"missing_fields": svc.MissingFields(),
					"action":         "Set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URI in service environment, then restart.",
				})
				return
			}
			writeOAuthCallbackPage(w, http.StatusServiceUnavailable, frontendOrigin, map[string]interface{}{
				"type":       "integration_oauth_result",
				"status":     "error",
				"error":      ErrOAuthNotConfigured.Error(),
				"message":    "Google OAuth is not configured on the integrations service.",
				"configured": false,
			})
			return
		}

		code := r.URL.Query().Get("code")
		stateToken := r.URL.Query().Get("state")
		if code == "" || stateToken == "" {
			writeCallbackError(http.StatusBadRequest, "missing code or state", "", "", "")
			return
		}
		orgID, stateUserID, provider, err := svc.ValidateAndConsumeState(stateToken)
		if err != nil {
			writeCallbackError(http.StatusForbidden, "invalid or expired oauth state", err.Error(), "", "")
			return
		}
		actorCookie, cookieErr := r.Cookie(oauthActorCookieName)
		if cookieErr != nil || strings.TrimSpace(actorCookie.Value) == "" || strings.TrimSpace(actorCookie.Value) != stateUserID {
			writeCallbackError(http.StatusForbidden, "oauth callback actor mismatch", "", orgID, provider)
			return
		}
		if authHeader := strings.TrimSpace(r.Header.Get("Authorization")); authHeader != "" {
			currentUserID, userErr := extractBearerUserID(authHeader)
			if userErr != nil || currentUserID != stateUserID {
				writeCallbackError(http.StatusForbidden, "oauth callback actor mismatch", "", orgID, provider)
				return
			}
		}

		http.SetCookie(w, &http.Cookie{Name: oauthActorCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: cookieSecure, SameSite: http.SameSiteLaxMode})
		if err := svc.ExchangeForProvider(r.Context(), code, orgID, provider); err != nil {
			log.Printf("oauth.callback exchange failed org_id=%q provider=%q: %v", orgID, provider, err)
			writeCallbackError(http.StatusInternalServerError, "OAuth exchange failed, please try again", err.Error(), orgID, provider)
			return
		}

		if wantsJSON {
			writeJSON(w, http.StatusOK, map[string]string{"status": "connected", "org_id": orgID, "service": provider})
			return
		}

		writeOAuthCallbackPage(w, http.StatusOK, frontendOrigin, map[string]interface{}{
			"type":    "integration_oauth_result",
			"status":  "connected",
			"org_id":  orgID,
			"service": provider,
			"message": "Account connected. Returning to the app...",
		})
	})

	mux.HandleFunc("/auth/google/disconnect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
		accountID := r.URL.Query().Get("account_id")
		provider, invalidProvider := resolveProviderOrDefault(r.URL.Query().Get("service"))
		if invalidProvider {
			writeError(w, http.StatusBadRequest, "unsupported service")
			return
		}
		if orgID == "" {
			writeError(w, http.StatusBadRequest, "org_id required")
			return
		}
		status, msg := authorizeOrg(r, orgID)
		if status != 0 {
			writeError(w, status, msg)
			return
		}
		var err error
		if accountID != "" {
			err = svc.DisconnectAccountForProvider(r.Context(), orgID, provider, accountID)
		} else {
			err = svc.DisconnectForProvider(r.Context(), orgID, provider)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/auth/google/status", func(w http.ResponseWriter, r *http.Request) {
		orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
		provider, invalidProvider := resolveProviderOrDefault(r.URL.Query().Get("service"))
		if invalidProvider {
			writeError(w, http.StatusBadRequest, "unsupported service")
			return
		}
		if orgID == "" {
			writeError(w, http.StatusBadRequest, "org_id required")
			return
		}
		status, msg := authorizeOrg(r, orgID)
		if status != 0 {
			writeError(w, status, msg)
			return
		}
		if !svc.IsConfigured() {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"service":        provider,
				"configured":     false,
				"connected":      false,
				"missing_fields": svc.MissingFields(),
				"message":        "Google integration needs admin setup before org admins can connect accounts.",
			})
			return
		}

		accounts, err := svc.ListConnectionsForProvider(r.Context(), orgID, provider)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if len(accounts) == 0 {
			writeJSON(w, http.StatusOK, map[string]interface{}{"service": provider, "configured": true, "connected": false})
			return
		}

		primary := accounts[0]
		for _, account := range accounts {
			if account != nil && account.IsPrimary {
				primary = account
				break
			}
		}

		connected := true
		if _, clientErr := svc.GetClientForProvider(r.Context(), orgID, provider); clientErr != nil {
			connected = false
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"service":            provider,
			"configured":         true,
			"connected":          connected,
			"connected_at":       primary.ConnectedAt,
			"scopes":             primary.Scopes,
			"account_id":         primary.AccountID,
			"account_email":      primary.AccountEmail,
			"account_name":       primary.AccountName,
			"connected_accounts": len(accounts),
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
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("oauth.writeJSON encode failed: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func wantsJSONResponse(r *http.Request) bool {
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	if accept == "" {
		return false
	}
	if strings.Contains(accept, "text/html") {
		return false
	}
	return strings.Contains(accept, "application/json")
}

func resolveTrustedFrontendOrigin(r *http.Request, trustedOrigins []string) string {
	candidate := strings.TrimSpace(r.URL.Query().Get("frontend_origin"))
	if isTrustedOrigin(candidate, trustedOrigins) {
		return candidate
	}

	headerOrigin := strings.TrimSpace(r.Header.Get("Origin"))
	if isTrustedOrigin(headerOrigin, trustedOrigins) {
		return headerOrigin
	}

	if len(trustedOrigins) == 1 {
		fallback := strings.TrimSpace(trustedOrigins[0])
		if fallback != "" {
			return fallback
		}
	}

	return ""
}

func isTrustedOrigin(origin string, trustedOrigins []string) bool {
	trimmedOrigin := strings.TrimSpace(origin)
	if trimmedOrigin == "" || len(trustedOrigins) == 0 {
		return false
	}
	for _, allowed := range trustedOrigins {
		if strings.EqualFold(strings.TrimSpace(allowed), trimmedOrigin) {
			return true
		}
	}
	return false
}

func writeOAuthCallbackPage(w http.ResponseWriter, status int, frontendOrigin string, payload map[string]interface{}) {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		payloadJSON = []byte(`{"type":"integration_oauth_result","status":"error","error":"failed to render oauth callback response","message":"Connection result is available. You can close this window."}`)
	}
	frontendOriginJSON, err := json.Marshal(strings.TrimSpace(frontendOrigin))
	if err != nil {
		frontendOriginJSON = []byte(`""`)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<title>Integration OAuth</title>
	<style>
		body { font-family: Segoe UI, Arial, sans-serif; margin: 0; padding: 24px; background: #f8fafc; color: #0f172a; }
		.card { max-width: 560px; margin: 48px auto; background: #ffffff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 20px; box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06); }
		h1 { margin: 0 0 8px; font-size: 20px; }
		p { margin: 0; color: #334155; }
		.hint { margin-top: 12px; font-size: 14px; color: #64748b; }
		button { margin-top: 16px; border: 1px solid #cbd5e1; border-radius: 8px; background: #ffffff; padding: 8px 12px; cursor: pointer; }
	</style>
</head>
<body>
	<div class="card">
		<h1 id="oauth-title">Finishing connection...</h1>
		<p id="oauth-message">Please wait while we return you to the app.</p>
		<p class="hint">If this window does not close automatically, you can close it manually.</p>
		<button type="button" onclick="window.close()">Close window</button>
	</div>
	<script>
		(function () {
			const payload = %s;
			const frontendOrigin = %s;
			const ok = payload && payload.status === "connected";
			const titleEl = document.getElementById("oauth-title");
			const messageEl = document.getElementById("oauth-message");
			if (titleEl) {
				titleEl.textContent = ok ? "Connection complete" : "Connection failed";
			}
			if (messageEl) {
				messageEl.textContent = payload && payload.message
					? payload.message
					: (ok ? "Account connected. Returning to the app..." : "Connection failed. You can close this window and return to the app.");
			}
			try {
				if (frontendOrigin && window.opener && !window.opener.closed) {
					window.opener.postMessage(payload, frontendOrigin);
					window.close();
				}
			} catch (err) {
				// No-op: keep fallback UI visible.
			}
		})();
	</script>
</body>
</html>`, string(payloadJSON), string(frontendOriginJSON))
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

func (s *Service) GenerateState(orgID, userID string, provider ...string) (string, error) {
	if strings.TrimSpace(orgID) == "" {
		return "", fmt.Errorf("org_id required")
	}
	if strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("user_id required")
	}

	resolvedProvider := providerGoogleForms
	if len(provider) > 0 {
		resolvedProvider = normalizeProviderID(provider[0])
		if resolvedProvider == "" {
			return "", ErrOAuthInvalidProvider
		}
	}

	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate state bytes: %w", err)
	}

	statePayload := signedAuthState{
		OrgID:     strings.TrimSpace(orgID),
		UserID:    strings.TrimSpace(userID),
		Provider:  resolvedProvider,
		ExpiresAt: time.Now().Add(authStateTTL).Unix(),
		Nonce:     base64.RawURLEncoding.EncodeToString(raw),
	}
	payloadJSON, err := json.Marshal(statePayload)
	if err != nil {
		return "", fmt.Errorf("marshal oauth state: %w", err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signature := s.signStatePayload(encodedPayload)
	return encodedPayload + "." + signature, nil
}

func (s *Service) ValidateAndConsumeState(state string) (string, string, string, error) {
	trimmed := strings.TrimSpace(state)
	if trimmed == "" {
		return "", "", "", fmt.Errorf("state is required")
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("state format invalid")
	}
	encodedPayload := parts[0]
	signature := parts[1]
	if !hmac.Equal([]byte(signature), []byte(s.signStatePayload(encodedPayload))) {
		return "", "", "", fmt.Errorf("state signature invalid")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return "", "", "", fmt.Errorf("state payload decode failed")
	}
	var pending signedAuthState
	if err := json.Unmarshal(payloadBytes, &pending); err != nil {
		return "", "", "", fmt.Errorf("state payload parse failed")
	}

	if time.Now().Unix() > pending.ExpiresAt {
		return "", "", "", fmt.Errorf("state expired")
	}
	if strings.TrimSpace(pending.OrgID) == "" {
		return "", "", "", fmt.Errorf("state missing org")
	}
	if strings.TrimSpace(pending.UserID) == "" {
		return "", "", "", fmt.Errorf("state missing user")
	}
	provider := normalizeProviderID(pending.Provider)
	if provider == "" {
		return "", "", "", ErrOAuthInvalidProvider
	}
	return pending.OrgID, pending.UserID, provider, nil
}

func buildStateSigningKey(clientSecret string) []byte {
	secret := strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_STATE_SECRET"))
	if secret == "" {
		secret = clientSecret
	}
	if secret == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func (s *Service) signStatePayload(encodedPayload string) string {
	if len(s.stateSigningKey) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, s.stateSigningKey)
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func revokeToken(ctx context.Context, token string) error {
	form := url.Values{"token": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/revoke", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send revoke request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("revoke request failed with status %d", resp.StatusCode)
	}
	return nil
}

func parseTokenScopes(tok *oauth2.Token) []string {
	raw := tok.Extra("scope")
	out := make([]string, 0)
	seen := make(map[string]struct{})
	appendScope := func(v string) {
		s := strings.TrimSpace(v)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	switch v := raw.(type) {
	case string:
		for _, part := range strings.Fields(v) {
			appendScope(part)
		}
	case []string:
		for _, part := range v {
			appendScope(part)
		}
	case []interface{}:
		for _, part := range v {
			appendScope(fmt.Sprint(part))
		}
	}
	return out
}

func authorizeOrgAccess(r *http.Request, orgID string) (int, string) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return http.StatusUnauthorized, "missing or invalid authorization header"
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("AUTH_SERVICE_URL")), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	verifyURL := fmt.Sprintf("%s/api/orgs/%s/roles", baseURL, url.PathEscape(orgID))
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, verifyURL, nil)
	if err != nil {
		return http.StatusInternalServerError, "failed to build auth verification request"
	}
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
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

func extractBearerUserID(authHeader string) (string, error) {
	header := strings.TrimSpace(authHeader)
	if header == "" {
		return "", fmt.Errorf("missing authorization header")
	}
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return "", fmt.Errorf("invalid authorization header")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid authorization header")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", fmt.Errorf("missing bearer token")
	}
	jwtParts := strings.Split(token, ".")
	if len(jwtParts) != 3 {
		return "", fmt.Errorf("invalid jwt format")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(jwtParts[1])
	if err != nil {
		return "", fmt.Errorf("decode jwt payload: %w", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", fmt.Errorf("parse jwt payload: %w", err)
	}
	sub, _ := claims["sub"].(string)
	if strings.TrimSpace(sub) == "" {
		return "", fmt.Errorf("missing sub claim")
	}
	return strings.TrimSpace(sub), nil
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
