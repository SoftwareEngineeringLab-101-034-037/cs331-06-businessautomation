package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/database"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
)

const (
	UserIDKey = "user_id"
	OrgIDKey  = "org_id"
)

// For production, replace ParseUnverified with full JWKS-based verification.
func ClerkAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing or invalid authorization header",
			})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		token, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		userID, _ := claims["sub"].(string)
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing user ID in token"})
			return
		}

		c.Set(UserIDKey, userID)
		c.Next()
	}
}

// OrgAdminOnly ensures the authenticated user is an admin of the organization
// specified by the :orgId URL parameter. Must be used after ClerkAuthMiddleware.
func OrgAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString(UserIDKey)
		orgID := c.Param("orgId")

		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		if orgID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Organization ID is required"})
			return
		}

		// First try: check organization_memberships table
		var membership models.OrganizationMembership
		err := database.DB.Where(
			"user_id = ? AND organization_id = ?", userID, orgID,
		).First(&membership).Error

		if err == nil && membership.IsOrgAdmin() {
			c.Set(OrgIDKey, orgID)
			c.Next()
			return
		}

		// Fallback: check if user is the org_admin_id on the organizations table
		var org models.Organization
		err = database.DB.Where("id = ? AND org_admin_id = ?", orgID, userID).First(&org).Error

		if err != nil {
			log.Printf("[OrgAdminOnly] denied: user_id=%s org_id=%s (not found in memberships or org_admin)", userID, orgID)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Not a member of this organization"})
			return
		}

		c.Set(OrgIDKey, orgID)
		c.Next()
	}
}

func GetUserID(c *gin.Context) string {
	return c.GetString(UserIDKey)
}

func GetOrgID(c *gin.Context) string {
	return c.GetString(OrgIDKey)
}
