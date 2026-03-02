package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testIssuer = "https://test.clerk.accounts.dev"

// testRSAKey is generated once per test binary and used to sign/verify test JWTs.
var testRSAKey *rsa.PrivateKey

func init() {
	var err error
	testRSAKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("failed to generate test RSA key: " + err.Error())
	}
}

// testKeyFunc returns the RSA public key for any token — used as jwt.Keyfunc in tests.
func testKeyFunc(token *jwt.Token) (interface{}, error) {
	return &testRSAKey.PublicKey, nil
}

func TestClerkAuthMiddlewareMissingAuthHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", ClerkAuthMiddleware(testKeyFunc, testIssuer), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestClerkAuthMiddlewareInvalidBearerPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", ClerkAuthMiddleware(testKeyFunc, testIssuer), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Token abc")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestClerkAuthMiddlewareInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", ClerkAuthMiddleware(testKeyFunc, testIssuer), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer this-is-not-a-jwt")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestClerkAuthMiddlewareMissingUserIDInToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", ClerkAuthMiddleware(testKeyFunc, testIssuer), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	token := signedTestJWT(t, jwt.MapClaims{
		"iss": testIssuer,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestClerkAuthMiddlewareSetsUserIDOnSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", ClerkAuthMiddleware(testKeyFunc, testIssuer), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": GetUserID(c),
		})
	})

	token := signedTestJWT(t, jwt.MapClaims{
		"sub": "user_123",
		"iss": testIssuer,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response json: %v", err)
	}
	if body["user_id"] != "user_123" {
		t.Fatalf("expected user_id=user_123, got %q", body["user_id"])
	}
}

func TestOrgAdminOnlyRequiresAuthentication(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t, true, true)
	database.DB = db

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/orgs/:orgId/admin", OrgAdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/org_1/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAdminOnlyRequiresOrgID(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t, true, true)
	database.DB = db

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, "user_1")
		c.Next()
	})
	r.GET("/admin", OrgAdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAdminOnlyReturnsInternalErrorWhenMembershipLookupFails(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t, false, true)
	database.DB = db

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, "user_1")
		c.Next()
	})
	r.GET("/orgs/:orgId/admin", OrgAdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/org_1/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAdminOnlyAllowsAdminMembership(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t, true, true)
	database.DB = db

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO organization_memberships (id, user_id, organization_id, clerk_role, joined_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "mem_1", "user_admin", "org_1", "org:admin", now, now, now).Error; err != nil {
		t.Fatalf("failed seeding membership: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, "user_admin")
		c.Next()
	})
	r.GET("/orgs/:orgId/admin", OrgAdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": GetUserID(c),
			"org_id":  GetOrgID(c),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/org_1/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAdminOnlyFallsBackToOrganizationAdmin(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t, true, true)
	database.DB = db

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO organizations (id, name, slug, org_admin_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "org_1", "Org One", "org-one", "user_owner", true, now, now).Error; err != nil {
		t.Fatalf("failed seeding organization: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, "user_owner")
		c.Next()
	})
	r.GET("/orgs/:orgId/admin", OrgAdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"org_id": GetOrgID(c),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/org_1/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAdminOnlyForbiddenWhenNotAdmin(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t, true, true)
	database.DB = db

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO organization_memberships (id, user_id, organization_id, clerk_role, joined_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "mem_2", "user_member", "org_1", "org:member", now, now, now).Error; err != nil {
		t.Fatalf("failed seeding membership: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, "user_member")
		c.Next()
	})
	r.GET("/orgs/:orgId/admin", OrgAdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/org_1/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestOrgAdminOnlyReturnsInternalErrorWhenFallbackLookupFails(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t, true, false)
	database.DB = db

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, "user_1")
		c.Next()
	})
	r.GET("/orgs/:orgId/admin", OrgAdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/org_1/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d; body=%s", w.Code, w.Body.String())
	}
}

func setupMiddlewareTestDB(t *testing.T, withMembershipsTable, withOrganizationsTable bool) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if withMembershipsTable {
		if err := db.Exec(`
			CREATE TABLE organization_memberships (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				organization_id TEXT NOT NULL,
				clerk_role TEXT NOT NULL,
				local_role_id TEXT,
				joined_at DATETIME,
				created_at DATETIME,
				updated_at DATETIME
			)
		`).Error; err != nil {
			t.Fatalf("failed creating organization_memberships table: %v", err)
		}
	}

	if withOrganizationsTable {
		if err := db.Exec(`
			CREATE TABLE organizations (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				slug TEXT UNIQUE,
				image_url TEXT,
				org_admin_id TEXT,
				is_active BOOLEAN DEFAULT 1,
				created_at DATETIME,
				updated_at DATETIME
			)
		`).Error; err != nil {
			t.Fatalf("failed creating organizations table: %v", err)
		}
	}

	return db
}

func restoreDB(t *testing.T) {
	t.Helper()
	previous := database.DB
	t.Cleanup(func() {
		database.DB = previous
	})
}

func signedTestJWT(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(testRSAKey)
	if err != nil {
		t.Fatalf("failed signing test jwt: %v", err)
	}
	return signed
}
