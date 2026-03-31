package middleware

import (
	"fmt"
	"log"
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
		log.Printf("[AUTH] --> %s %s", c.Request.Method, c.Request.URL.Path)

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			log.Printf("[AUTH] FAIL: missing or malformed Authorization header")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing or invalid authorization header",
			})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		log.Printf("[AUTH] token prefix (first 40 chars): %.40s", tokenString)

		token, err := jwt.Parse(tokenString, keyFunc,
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuer(issuerURL),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !token.Valid {
			log.Printf("[AUTH] FAIL: jwt.Parse error: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Printf("[AUTH] FAIL: claims type assertion failed")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		// Log every claim key present in the token for diagnosis
		claimKeys := make([]string, 0, len(claims))
		for k := range claims {
			claimKeys = append(claimKeys, fmt.Sprintf("%s=%v", k, claims[k]))
		}
		log.Printf("[AUTH] token claims: %s", strings.Join(claimKeys, " | "))

		userID, _ := claims["sub"].(string)
		if userID == "" {
			log.Printf("[AUTH] FAIL: sub claim missing")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing user ID in token"})
			return
		}

		// Clerk v1 tokens carry org_id as a top-level string claim.
		// Clerk v2 tokens (v=2) carry it nested inside the "o" object: {"id":"org_...","rol":"admin",...}
		orgID, _ := claims["org_id"].(string)
		if orgID == "" {
			if o, ok := claims["o"].(map[string]interface{}); ok {
				orgID, _ = o["id"].(string)
			}
		}
		log.Printf("[AUTH] OK  sub=%s org_id=%q (empty means personal session token)", userID, orgID)

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

func GetAuthorizationHeader(c *gin.Context) string {
	return strings.TrimSpace(c.GetHeader("Authorization"))
}

// RequireOrgMatch ensures the :orgId path parameter matches the org_id
// claim from the authenticated JWT. Returns 403 if they differ.
func RequireOrgMatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		pathOrgID := c.Param("orgId")
		tokenOrgID := c.GetString(OrgIDKey)

		log.Printf("[ORG_MATCH] path_org_id=%q  token_org_id=%q", pathOrgID, tokenOrgID)

		if pathOrgID == "" {
			log.Printf("[ORG_MATCH] DENY: path :orgId is empty")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "You do not have access to this organisation"})
			return
		}
		if tokenOrgID == "" {
			log.Printf("[ORG_MATCH] DENY: token has no org_id claim — token is a personal (non-org) session")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "You do not have access to this organisation"})
			return
		}
		if pathOrgID != tokenOrgID {
			log.Printf("[ORG_MATCH] DENY: mismatch path=%q token=%q", pathOrgID, tokenOrgID)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "You do not have access to this organisation"})
			return
		}

		log.Printf("[ORG_MATCH] ALLOW: org_id=%q", pathOrgID)
		c.Next()
	}
}
