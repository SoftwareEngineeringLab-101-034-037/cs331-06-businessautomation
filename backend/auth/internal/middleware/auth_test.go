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
	db := setupMiddlewareTestDB(t)
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
	db := setupMiddlewareTestDB(t)
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

func TestOrgAdminOnlyAllowsAdmin(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t)
	database.DB = db

	orgID := "org_1"
	now := time.Now()
	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, is_admin, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_admin", "admin@example.com", "Admin", "User", orgID, true, true, now, now).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
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

func TestOrgAdminOnlyForbiddenWhenNotAdmin(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t)
	database.DB = db

	orgID := "org_1"
	now := time.Now()
	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, is_admin, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_member", "member@example.com", "Member", "User", orgID, false, true, now, now).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
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

func TestOrgAdminOnlyForbiddenWhenDifferentOrg(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t)
	database.DB = db

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, is_admin, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_admin", "admin@example.com", "Admin", "User", "org_2", true, true, now, now).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, "user_admin")
		c.Next()
	})
	r.GET("/orgs/:orgId/admin", OrgAdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// User is admin of org_2, trying to access org_1
	req := httptest.NewRequest(http.MethodGet, "/orgs/org_1/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestOrgMemberOnlyAllowsMember(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t)
	database.DB = db

	orgID := "org_1"
	now := time.Now()
	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, is_admin, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_member", "member@example.com", "Member", "User", orgID, false, true, now, now).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, "user_member")
		c.Next()
	})
	r.GET("/orgs/:orgId/member", OrgMemberOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "org_id": GetOrgID(c)})
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/org_1/member", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestOrgMemberOnlyForbiddenWhenDifferentOrg(t *testing.T) {
	restoreDB(t)
	db := setupMiddlewareTestDB(t)
	database.DB = db

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, is_admin, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_member", "member@example.com", "Member", "User", "org_2", false, true, now, now).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, "user_member")
		c.Next()
	})
	r.GET("/orgs/:orgId/member", OrgMemberOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/org_1/member", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body=%s", w.Code, w.Body.String())
	}
}

func setupMiddlewareTestDB(t *testing.T) *gorm.DB {
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

	if err := db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			first_name TEXT,
			last_name TEXT,
			avatar_url TEXT,
			organization_id TEXT,
			department_id TEXT,
			role_id TEXT,
			job_title TEXT,
			is_admin BOOLEAN DEFAULT 0,
			preferences TEXT,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME,
			last_sign_in_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("failed creating users table: %v", err)
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
