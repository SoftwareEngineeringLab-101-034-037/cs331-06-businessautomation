package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestClerkAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key generation failed: %v", err)
	}
	publicKey := &privateKey.PublicKey

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	}

	issuer := "https://issuer.example.com"

	makeToken := func(claims jwt.MapClaims, key *rsa.PrivateKey) string {
		t.Helper()
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		s, signErr := token.SignedString(key)
		if signErr != nil {
			t.Fatalf("sign token failed: %v", signErr)
		}
		return s
	}

	r := gin.New()
	r.Use(ClerkAuthMiddleware(keyFunc, issuer))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user": GetUserID(c),
			"org":  GetOrgID(c),
		})
	})

	t.Run("missing auth header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer not-a-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("missing sub claim", func(t *testing.T) {
		token := makeToken(jwt.MapClaims{
			"iss":    issuer,
			"exp":    time.Now().Add(time.Hour).Unix(),
			"org_id": "org-1",
		}, privateKey)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		token := makeToken(jwt.MapClaims{
			"iss":    issuer,
			"exp":    time.Now().Add(time.Hour).Unix(),
			"sub":    "user-1",
			"org_id": "org-1",
		}, privateKey)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if body := rec.Body.String(); body == "" || body == "{}" {
			t.Fatalf("expected non-empty json body")
		}
	})

	t.Run("wrong issuer", func(t *testing.T) {
		token := makeToken(jwt.MapClaims{
			"iss":    "https://wrong.example.com",
			"exp":    time.Now().Add(time.Hour).Unix(),
			"sub":    "user-1",
			"org_id": "org-1",
		}, privateKey)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}
