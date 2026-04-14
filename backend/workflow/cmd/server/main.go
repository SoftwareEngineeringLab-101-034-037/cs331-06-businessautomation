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

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/config"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/connectors"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/executor"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/handler"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/middleware"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if strings.TrimSpace(cfg.IntegrationKey) == "" {
		log.Printf("WARNING: WORKFLOW_INTEGRATION_KEY is empty; integration ingress is disabled")
		log.Fatalf("WORKFLOW_INTEGRATION_KEY must be set")
	}

	// Connect to MongoDB (required) with retries for transient Atlas topology issues.
	var store *storage.MongoStore
	var lastMongoErr error
	for attempt := 1; attempt <= 6; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		store, err = storage.NewMongoStore(ctx, cfg.MongoURI)
		cancel()
		if err == nil {
			lastMongoErr = nil
			break
		}
		lastMongoErr = err
		log.Printf("Mongo connect attempt %d/6 failed: %v", attempt, err)
		if attempt < 6 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}

	if lastMongoErr != nil {
		log.Fatalf("Failed to connect to MongoDB after retries: %v", lastMongoErr)
	}

	if store == nil {
		log.Fatalf("Failed to connect to MongoDB: unknown error")
	}
	log.Printf("Connected to MongoDB succesfully")

	email := connectors.NewIntegrationsGmailConnector(cfg.IntegrationsURL, cfg.IntegrationKey)
	roleDirectory, err := executor.NewHTTPRoleDirectory(cfg.AuthServiceURL, cfg.AuthServiceToken)
	if err != nil {
		log.Fatalf("Failed to configure role directory: %v", err)
	}
	assigneeSelector := executor.NewBalancedRoleAssigneeSelector(roleDirectory, store)
	exec := executor.NewExecutor(store, email, assigneeSelector)

	workflowHandler := handler.NewWorkflowHandler(store)
	instanceHandler := handler.NewInstanceHandler(store, exec, cfg.IntegrationKey)
	taskHandler := handler.NewTaskHandler(store, exec)
	analyticsHandler := handler.NewAnalyticsHandler(store)

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
	r.POST("/integrations/google-forms/events", instanceHandler.StartFromGoogleForms)
	r.POST("/integrations/gmail/events", instanceHandler.StartFromGmail)

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
			orgApi.POST("/instances/:id/restart", instanceHandler.Restart)
			orgApi.GET("/instances", instanceHandler.List)
			orgApi.GET("/instances/:id", instanceHandler.Get)

			// Task management
			orgApi.GET("/tasks", taskHandler.List)
			orgApi.PUT("/tasks/:id/:action", taskHandler.Action)
			orgApi.GET("/tasks/:id/escalation-candidates", taskHandler.EscalationCandidates)

			// Analytics
			orgApi.GET("/analytics", analyticsHandler.Get)
		}
	}

	addr := ":" + cfg.Port
	log.Printf("Workflow service running on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
