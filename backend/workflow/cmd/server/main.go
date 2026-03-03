package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	keyfunc "github.com/MicahParks/keyfunc/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/example/business-automation/backend/workflow/internal/config"
	"github.com/example/business-automation/backend/workflow/internal/connectors"
	"github.com/example/business-automation/backend/workflow/internal/executor"
	"github.com/example/business-automation/backend/workflow/internal/handler"
	"github.com/example/business-automation/backend/workflow/internal/middleware"
	"github.com/example/business-automation/backend/workflow/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var store storage.Store
	if cfg.MongoURI != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if ms, connErr := storage.NewMongoStore(ctx, cfg.MongoURI); connErr == nil {
			log.Printf("Connected to MongoDB: %s", cfg.MongoURI)
			store = ms
		} else {
			log.Printf("MongoDB connection failed: %v — falling back to in-memory store", connErr)
		}
	}
	if store == nil {
		log.Println("Using in-memory store")
		store = storage.NewMemoryStore()
	}

	email := connectors.NewMockEmail()
	exec := executor.NewExecutor(store, email)

	workflowHandler := handler.NewWorkflowHandler(store)
	instanceHandler := handler.NewInstanceHandler(store, exec)
	taskHandler := handler.NewTaskHandler(store)

	// ── Clerk JWT auth (optional — disabled if CLERK_ISSUER_URL is not set) ───
	var authMW gin.HandlerFunc
	if cfg.ClerkIssuerURL != "" {
		jwksURL := strings.TrimRight(cfg.ClerkIssuerURL, "/") + "/.well-known/jwks.json"
		jwks, jwksErr := keyfunc.Get(jwksURL, keyfunc.Options{RefreshInterval: time.Hour})
		if jwksErr != nil {
			log.Fatalf("Failed to fetch JWKS from %s: %v", jwksURL, jwksErr)
		}
		defer jwks.EndBackground()
		authMW = middleware.ClerkAuthMiddleware(jwks.Keyfunc, cfg.ClerkIssuerURL)
		log.Printf("Clerk JWT auth enabled (issuer: %s)", cfg.ClerkIssuerURL)
	} else {
		log.Println("[WARN] CLERK_ISSUER_URL not set — JWT auth is disabled")
		authMW = func(c *gin.Context) { c.Next() }
	}

	// ── Gin router ────────────────────────────────────────────────────────────
	gin.SetMode(gin.DebugMode)
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// ── Routes ────────────────────────────────────────────────────────────────

	// Public
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Protected
	api := r.Group("/api")
	api.Use(authMW)
	{
		// Workflow CRUD
		api.GET("/workflows", workflowHandler.List)
		api.POST("/workflows", workflowHandler.Create)
		api.GET("/workflows/:id", workflowHandler.Get)
		api.PUT("/workflows/:id", workflowHandler.Update)
		api.DELETE("/workflows/:id", workflowHandler.Delete)

		// Instance management
		api.POST("/instances", instanceHandler.Start)
		api.GET("/instances/:id", instanceHandler.Get)

		// Task management
		api.GET("/tasks", taskHandler.List)
		api.PUT("/tasks/:id/:action", taskHandler.Action)
	}

	// ── Start server ──────────────────────────────────────────────────────────
	addr := ":" + cfg.Port
	log.Printf("Workflow service running on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
