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

	// Connect to MongoDB (required)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := storage.NewMongoStore(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	log.Printf("Connected to MongoDB succesfully")

	email := connectors.NewMockEmail()
	roleDirectory, err := executor.NewHTTPRoleDirectory(cfg.AuthServiceURL, cfg.AuthServiceToken)
	if err != nil {
		log.Fatalf("Failed to configure role directory: %v", err)
	}
	assigneeSelector := executor.NewRandomRoleAssigneeSelector(roleDirectory)
	exec := executor.NewExecutor(store, email, assigneeSelector)

	workflowHandler := handler.NewWorkflowHandler(store)
	instanceHandler := handler.NewInstanceHandler(store, exec)
	taskHandler := handler.NewTaskHandler(store, exec)

	// ── Clerk JWT auth ────────────────────────────────────────────────────────
	jwksURL := strings.TrimRight(cfg.ClerkIssuerURL, "/") + "/.well-known/jwks.json"
	jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{RefreshInterval: time.Hour})
	if err != nil {
		log.Fatalf("Failed to fetch JWKS from %s: %v", jwksURL, err)
	}
	defer jwks.EndBackground()
	authMW := middleware.ClerkAuthMiddleware(jwks.Keyfunc, cfg.ClerkIssuerURL)
	log.Printf("Clerk JWT auth enabled (issuer: %s)", cfg.ClerkIssuerURL)

	// ── Gin router ────────────────────────────────────────────────────────────
	gin.SetMode(gin.DebugMode)
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300 * time.Second,
	}))

	// Public
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Protected
	api := r.Group("/api")
	api.Use(authMW)
	{
		orgApi := api.Group("/orgs/:orgId")
		orgApi.Use(middleware.RequireOrgMatch())
		{
			// Workflow CRUD
			orgApi.GET("/workflows", workflowHandler.List)
			orgApi.POST("/workflows", workflowHandler.Create)
			orgApi.GET("/workflows/:id", workflowHandler.Get)
			orgApi.PUT("/workflows/:id", workflowHandler.Update)
			orgApi.DELETE("/workflows/:id", workflowHandler.Delete)

			// Instance management
			orgApi.POST("/instances", instanceHandler.Start)
			orgApi.GET("/instances", instanceHandler.List)
			orgApi.GET("/instances/:id", instanceHandler.Get)

			// Task management
			orgApi.GET("/tasks", taskHandler.List)
			orgApi.PUT("/tasks/:id/:action", taskHandler.Action)
		}
	}

	addr := ":" + cfg.Port
	log.Printf("Workflow service running on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
