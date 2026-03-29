package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/example/business-automation/backend/google-forms/internal/models"
	"github.com/example/business-automation/backend/google-forms/internal/storage"
)

var scopes = []string{
	"https://www.googleapis.com/auth/forms.body",
	"https://www.googleapis.com/auth/forms.responses.readonly",
	"https://www.googleapis.com/auth/drive.file",
}

type Service struct {
	cfg   *oauth2.Config
	store storage.Store
}

func NewService(clientID, clientSecret, redirectURI string, store storage.Store) *Service {
	return &Service{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURI,
			Scopes:       scopes,
			Endpoint:     google.Endpoint,
		},
		store: store,
	}
}

func (s *Service) AuthURL(orgID string) string {
	return s.cfg.AuthCodeURL(orgID, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

func (s *Service) Exchange(ctx context.Context, code, orgID string) error {
	tok, err := s.cfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}
	m := &models.OAuthToken{
		OrgID:        orgID,
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
	stored, err := s.store.GetToken(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, fmt.Errorf("no google connection for org %s", orgID)
	}

	tok := &oauth2.Token{
		AccessToken:  stored.AccessToken,
		RefreshToken: stored.RefreshToken,
		TokenType:    stored.TokenType,
		Expiry:       stored.Expiry,
	}

	fresh, err := s.cfg.TokenSource(ctx, tok).Token()
	if err != nil {
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
	stored, err := s.store.GetToken(ctx, orgID)
	if err != nil {
		return err
	}
	if stored != nil {
		resp, err := http.PostForm("https://oauth2.googleapis.com/revoke",
			url.Values{"token": {stored.AccessToken}})
		if err == nil {
			resp.Body.Close()
		}
	}
	return s.store.DeleteToken(ctx, orgID)
}

func RegisterHandlers(mux *http.ServeMux, svc *Service, store storage.Store) {
	mux.HandleFunc("/auth/google/connect", func(w http.ResponseWriter, r *http.Request) {
		orgID := r.URL.Query().Get("org_id")
		if orgID == "" {
			writeError(w, http.StatusBadRequest, "org_id required")
			return
		}
		http.Redirect(w, r, svc.AuthURL(orgID), http.StatusTemporaryRedirect)
	})

	mux.HandleFunc("/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		orgID := r.URL.Query().Get("state")
		if code == "" || orgID == "" {
			writeError(w, http.StatusBadRequest, "missing code or state")
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
		if orgID == "" {
			writeError(w, http.StatusBadRequest, "org_id required")
			return
		}
		if err := svc.Disconnect(r.Context(), orgID); err != nil {
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
		tok, err := store.GetToken(r.Context(), orgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if tok == nil {
			writeJSON(w, http.StatusOK, map[string]bool{"connected": false})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"connected":    true,
			"connected_at": tok.ConnectedAt,
			"scopes":       tok.Scopes,
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
