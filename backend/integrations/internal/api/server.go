package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/config"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/integrations"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/oauth"
	providergmail "github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/providers/gmail"
	gmailhttp "github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/providers/gmail/httpapi"
	providergoogleforms "github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/providers/googleforms"
	googleformshttp "github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/providers/googleforms/httpapi"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/storage"
)

type Server struct {
	cfg        *config.Config
	store      storage.Store
	oauthSvc   *oauth.Service
	providers  *integrations.Registry
	defaultID  string
	gfHTTP     *googleformshttp.Handler
	gmailHTTP  *gmailhttp.Handler
	handlerMu  sync.Mutex
	mux        *http.ServeMux
	httpClient *http.Client
}

func NewServer(cfg *config.Config, store storage.Store, oauthSvc *oauth.Service, providers *integrations.Registry, defaultProviderID string) http.Handler {
	s := &Server{
		cfg:        cfg,
		store:      store,
		oauthSvc:   oauthSvc,
		providers:  providers,
		defaultID:  strings.TrimSpace(defaultProviderID),
		mux:        http.NewServeMux(),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	if s.providers == nil {
		s.providers = integrations.NewRegistry()
	}
	if provider, ok := s.providerByService(providergoogleforms.ProviderID); ok {
		s.gfHTTP = googleformshttp.NewHandler(store, provider, authorizedOrgIDFromContext)
	}
	if provider, ok := s.providerByService(providergmail.ProviderID); ok {
		s.gmailHTTP = gmailhttp.NewHandler(store, provider, authorizedOrgIDFromContext)
	}
	s.registerRoutes()
	return s.mux
}

func (s *Server) providerByService(service string) (integrations.Provider, bool) {
	if s.providers == nil {
		return nil, false
	}
	return s.providers.GetOrDefault(normalizeServiceID(service), normalizeServiceID(s.defaultID))
}

func normalizeServiceID(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return ""
	}
	return strings.ReplaceAll(trimmed, "-", "_")
}

func (s *Server) googleFormsHandler() *googleformshttp.Handler {
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()

	if s.gfHTTP != nil {
		return s.gfHTTP
	}
	provider, ok := s.providerByService(providergoogleforms.ProviderID)
	if !ok {
		return nil
	}
	s.gfHTTP = googleformshttp.NewHandler(s.store, provider, authorizedOrgIDFromContext)
	return s.gfHTTP
}

func (s *Server) gmailHandler() *gmailhttp.Handler {
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()

	if s.gmailHTTP != nil {
		return s.gmailHTTP
	}
	provider, ok := s.providerByService(providergmail.ProviderID)
	if !ok {
		return nil
	}
	s.gmailHTTP = gmailhttp.NewHandler(s.store, provider, authorizedOrgIDFromContext)
	return s.gmailHTTP
}

func (s *Server) registerRoutes() {
	oauth.RegisterHandlers(s.mux, s.oauthSvc, s.store, s.cfg.CORSAllowedOrigins)

	s.mux.HandleFunc("/forms", s.withOrgAuthorization(s.handleForms))
	s.mux.HandleFunc("/forms/", s.withOrgAuthorization(s.handleFormByPath))
	s.mux.HandleFunc("/watches", s.withOrgAuthorization(s.handleWatches))
	s.mux.HandleFunc("/watches/", s.withOrgAuthorization(s.handleWatchByID))
	s.mux.HandleFunc("/integration/status", s.handleIntegrationStatus)
	s.mux.HandleFunc("/integration/accounts", s.withOrgAuthorization(s.handleIntegrationAccounts))
	s.mux.HandleFunc("/integration/accounts/", s.withOrgAuthorization(s.handleIntegrationAccountByID))
	s.mux.HandleFunc("/integrations/providers", s.handleProviders)
	s.mux.HandleFunc("/integrations/", s.handleIntegrationProviderRoutes)
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api.writeJSON encode failed: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
