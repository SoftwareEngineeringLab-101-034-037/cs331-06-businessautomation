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

	"github.com/example/business-automation/backend/google-forms/internal/api"
	"github.com/example/business-automation/backend/google-forms/internal/config"
	"github.com/example/business-automation/backend/google-forms/internal/oauth"
	"github.com/example/business-automation/backend/google-forms/internal/poller"
	"github.com/example/business-automation/backend/google-forms/internal/storage"
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
	handler := api.NewServer(cfg, store, oauthSvc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if oauthSvc.IsConfigured() {
		formPoller := poller.New(store, oauthSvc, strings.TrimRight(cfg.WorkflowEngineURL, "/"), cfg.WorkflowServiceKey, cfg.PollIntervalSeconds)
		go formPoller.Start(ctx)
	} else {
		log.Printf("Google OAuth is not configured yet. Set %v and restart to enable Forms integration.", oauthSvc.MissingFields())
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withCORS(handler),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Google Forms service running on http://localhost:%s", cfg.Port)
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
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Integration-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
