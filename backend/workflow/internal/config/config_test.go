package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingRequiredEnvVars(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if err := os.Unsetenv("MONGO_URI"); err != nil {
		t.Fatalf("unset env failed: %v", err)
	}
	if err := os.Unsetenv("CLERK_ISSUER_URL"); err != nil {
		t.Fatalf("unset env failed: %v", err)
	}
	if err := os.Unsetenv("PORT"); err != nil {
		t.Fatalf("unset env failed: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Fatalf("expected missing env vars error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "MONGO_URI") || !strings.Contains(msg, "CLERK_ISSUER_URL") {
		t.Fatalf("unexpected error message: %s", msg)
	}
}

func TestLoadSuccessAndDefaultPort(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	t.Setenv("MONGO_URI", "  mongodb://localhost:27017  ")
	t.Setenv("CLERK_ISSUER_URL", "  https://issuer.example.com  ")
	if err := os.Unsetenv("PORT"); err != nil {
		t.Fatalf("unset env failed: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.MongoURI != "mongodb://localhost:27017" {
		t.Fatalf("unexpected mongo uri: %q", cfg.MongoURI)
	}
	if cfg.ClerkIssuerURL != "https://issuer.example.com" {
		t.Fatalf("unexpected issuer: %q", cfg.ClerkIssuerURL)
	}
	if cfg.Port != "8085" {
		t.Fatalf("expected default port 8085, got %q", cfg.Port)
	}
	if cfg.AuthServiceURL != "http://localhost:8080" {
		t.Fatalf("expected default auth service URL, got %q", cfg.AuthServiceURL)
	}
}

func TestLoadReadsDotEnv(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	envPath := filepath.Join(tmp, ".env")
	content := "MONGO_URI=mongodb://env\nCLERK_ISSUER_URL=https://issuer.env\nAUTH_SERVICE_URL=http://localhost:8082\nPORT=9999\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write .env failed: %v", err)
	}

	if err := os.Unsetenv("MONGO_URI"); err != nil {
		t.Fatalf("unset env failed: %v", err)
	}
	if err := os.Unsetenv("CLERK_ISSUER_URL"); err != nil {
		t.Fatalf("unset env failed: %v", err)
	}
	if err := os.Unsetenv("PORT"); err != nil {
		t.Fatalf("unset env failed: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.MongoURI != "mongodb://env" || cfg.ClerkIssuerURL != "https://issuer.env" || cfg.AuthServiceURL != "http://localhost:8082" || cfg.Port != "9999" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("TEST_KEY", "value")
	if got := getEnv("TEST_KEY", "default"); got != "value" {
		t.Fatalf("expected value, got %q", got)
	}
	if got := getEnv("MISSING_KEY", "default"); got != "default" {
		t.Fatalf("expected default, got %q", got)
	}
}
