package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	keyfunc "github.com/MicahParks/keyfunc/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/cleanup"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/config"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/database"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/handler"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/middleware"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/service"
)

func main() {
	runMigrations := flag.Bool("migrate", false, "run database migrations on startup")
	flag.Parse()

	//Load env vars
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to Supabase
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations by default so workflow-role membership support is always available.
	if *runMigrations {
		if err := database.Migrate(); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
	} else {
		log.Println("Skipping database migrations (-migrate=false)")
	}

	cleanup.Start(cleanup.DefaultConfig())

	// Initialize JWKS for JWT verification against Clerk's public keys
	jwksURL := strings.TrimRight(cfg.ClerkIssuerURL, "/") + "/.well-known/jwks.json"
	jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{
		RefreshInterval: time.Hour,
	})
	if err != nil {
		log.Fatalf("Failed to fetch JWKS from %s: %v", jwksURL, err)
	}
	defer jwks.EndBackground()

	employeeService := service.NewEmployeeService(database.DB, cfg.ClerkSecretKey)
	employeeHandler := handler.NewEmployeeHandler(employeeService)

	webhookHandler := handler.NewWebhookHandler(cfg.ClerkWebhookSecret, employeeService)
	gin.SetMode(gin.DebugMode)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/api/webhooks/clerk", webhookHandler.Handle)

	api := r.Group("/api")
	api.Use(middleware.ClerkAuthMiddleware(jwks.Keyfunc, cfg.ClerkIssuerURL))
	{
		// Routes accessible by any authenticated user
		api.POST("/orgs/:orgId/invitations/:invitationId/accept", employeeHandler.AcceptInvitation)
		memberOrgAPI := api.Group("/orgs/:orgId")
		memberOrgAPI.Use(middleware.OrgMemberOnly())
		{
			memberOrgAPI.GET("/me/profile", employeeHandler.GetMyProfile)
			// Workflow callers that depend on role lookup must forward the original user
			// Authorization header into StartInstance()/downstream execution. This route
			// requires a user JWT, so non-user-initiated workflows need a separate
			// service-token-capable endpoint instead of calling /roles without user context.
			memberOrgAPI.GET("/roles", employeeHandler.ListRoles)
		}

		// Routes restricted to org admins
		orgApi := api.Group("/orgs/:orgId")
		orgApi.Use(middleware.OrgAdminOnly())
		{
			orgApi.POST("/departments", employeeHandler.CreateDepartment)
			orgApi.GET("/departments", employeeHandler.ListDepartments)
			orgApi.GET("/departments/:deptID", employeeHandler.GetDepartment)
			orgApi.PUT("/departments/:deptID", employeeHandler.UpdateDepartment)
			orgApi.DELETE("/departments/:deptID", employeeHandler.DeleteDepartment)
			orgApi.POST("/roles", employeeHandler.CreateRole)
			orgApi.PUT("/roles/:roleID", employeeHandler.UpdateRole)
			orgApi.DELETE("/roles/:roleID", employeeHandler.DeleteRole)
			orgApi.POST("/employees/invite", employeeHandler.InviteSingle)
			orgApi.POST("/employees/invite/bulk", employeeHandler.InviteBulk)
			orgApi.GET("/employees", employeeHandler.ListEmployees)
			orgApi.DELETE("/employees/:employeeId", employeeHandler.DeleteEmployee)
			orgApi.GET("/invitations", employeeHandler.ListInvitations)
			orgApi.DELETE("/invitations/:invitationId", employeeHandler.RevokeInvitation)
		}
	}

	addr := ":" + cfg.Port
	log.Printf("Auth service running on http://localhost%s", addr)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
