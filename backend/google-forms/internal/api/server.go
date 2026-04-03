package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/example/business-automation/backend/google-forms/internal/config"
	"github.com/example/business-automation/backend/google-forms/internal/oauth"
	"github.com/example/business-automation/backend/google-forms/internal/storage"
)

type Server struct {
	cfg        *config.Config
	store      storage.Store
	oauthSvc   *oauth.Service
	mux        *http.ServeMux
	httpClient *http.Client
}

func NewServer(cfg *config.Config, store storage.Store, oauthSvc *oauth.Service) http.Handler {
	s := &Server{
		cfg:        cfg,
		store:      store,
		oauthSvc:   oauthSvc,
		mux:        http.NewServeMux(),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	s.registerRoutes()
	return s.mux
}

func (s *Server) registerRoutes() {
	oauth.RegisterHandlers(s.mux, s.oauthSvc, s.store)

	s.mux.HandleFunc("/forms", s.withOrgAuthorization(s.handleForms))
	s.mux.HandleFunc("/forms/", s.withOrgAuthorization(s.handleFormByPath))
	s.mux.HandleFunc("/watches", s.handleWatches)
	s.mux.HandleFunc("/watches/", s.handleWatchByID)
	s.mux.HandleFunc("/integration/status", s.handleIntegrationStatus)
	s.mux.HandleFunc("/integration/accounts", s.withOrgAuthorization(s.handleIntegrationAccounts))
	s.mux.HandleFunc("/integration/accounts/", s.withOrgAuthorization(s.handleIntegrationAccountByID))
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
