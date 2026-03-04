package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	UserIDKey = "user_id"
	OrgIDKey  = "org_id"
)

func ClerkAuthMiddleware(keyFunc jwt.Keyfunc, issuerURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing or invalid authorization header",
			})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenString, keyFunc,
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuer(issuerURL),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
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

		// Extract org_id from Clerk's org claim if present
		orgID, _ := claims["org_id"].(string)

		c.Set(UserIDKey, userID)
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

// RequireOrgMatch ensures the :orgId path parameter matches the org_id
// claim from the authenticated JWT. Returns 403 if they differ.
func RequireOrgMatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		pathOrgID := c.Param("orgId")
		tokenOrgID := c.GetString(OrgIDKey)

		if pathOrgID == "" || tokenOrgID == "" || pathOrgID != tokenOrgID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "You do not have access to this organisation",
			})
			return
		}
		c.Next()
	}
}
