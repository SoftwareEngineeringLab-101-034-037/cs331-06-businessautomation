package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/example/business-automation/backend/integrations/internal/api"
	"github.com/example/business-automation/backend/integrations/internal/config"
	"github.com/example/business-automation/backend/integrations/internal/integrations"
	"github.com/example/business-automation/backend/integrations/internal/oauth"
	"github.com/example/business-automation/backend/integrations/internal/poller"
	providergoogleforms "github.com/example/business-automation/backend/integrations/internal/providers/googleforms"
	"github.com/example/business-automation/backend/integrations/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	store, err := storage.NewMongo(cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("connect mongo: %v", err)
	}
	defer store.Close()

	oauthSvc := oauth.NewService(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURI, store)
	providers := integrations.NewRegistry()
	gfProvider := providergoogleforms.NewProvider(oauthSvc)
	providers.Register(gfProvider)
	handler := api.NewServer(cfg, store, oauthSvc, providers, providergoogleforms.ProviderID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if oauthSvc.IsConfigured() {
		triggerPath := providergoogleforms.TriggerEventPath
		if triggerSource, ok := gfProvider.(integrations.TriggerSource); ok {
			candidate := strings.TrimSpace(triggerSource.TriggerEventPath())
			if candidate != "" {
				triggerPath = candidate
			}
		}
		formPoller := poller.New(store, oauthSvc, strings.TrimRight(cfg.WorkflowEngineURL, "/"), triggerPath, cfg.WorkflowServiceKey, cfg.PollIntervalSeconds)
		go formPoller.Start(ctx)
	} else {
		log.Printf("Google OAuth is not configured yet. Set %v and restart to enable Forms integration.", oauthSvc.MissingFields())
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withRequestLogging(withCORS(handler)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Integrations service running on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Integration-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lw, r)
		log.Printf("http %s %s -> %d (%s)", r.Method, r.URL.RequestURI(), lw.status, time.Since(start).Round(time.Millisecond))
	})
}
