package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	clerkSecretKeyEnv     = "CLERK_SECRET_KEY"
	clerkWebhookSecretEnv = "CLERK_WEBHOOK_SECRET"
	databaseURLEnv        = "DATABASE_URL"
	portEnv               = "PORT"
)

func TestGetEnv(t *testing.T) {
	unsetEnv(t, "TEST_CONFIG_VALUE")
	if got := getEnv("TEST_CONFIG_VALUE", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback value, got %q", got)
	}

	t.Setenv("TEST_CONFIG_VALUE", "configured")
	if got := getEnv("TEST_CONFIG_VALUE", "fallback"); got != "configured" {
		t.Fatalf("expected configured value, got %q", got)
	}
}

func TestLoadReturnsErrorWhenRequiredVarsMissing(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}
	chdir(t, workDir)

	unsetEnv(t, clerkSecretKeyEnv)
	unsetEnv(t, clerkWebhookSecretEnv)
	unsetEnv(t, databaseURLEnv)
	unsetEnv(t, portEnv)

	cfg, err := Load()
	if err == nil {
		t.Fatalf("expected error for missing env vars, got cfg: %+v", cfg)
	}
	if !strings.Contains(err.Error(), clerkSecretKeyEnv) {
		t.Fatalf("expected missing %s in error, got %v", clerkSecretKeyEnv, err)
	}
	if !strings.Contains(err.Error(), clerkWebhookSecretEnv) {
		t.Fatalf("expected missing %s in error, got %v", clerkWebhookSecretEnv, err)
	}
	if !strings.Contains(err.Error(), databaseURLEnv) {
		t.Fatalf("expected missing %s in error, got %v", databaseURLEnv, err)
	}
}

func TestLoadTrimsValuesAndDefaultsPortAfterTrim(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}
	chdir(t, workDir)

	t.Setenv(clerkSecretKeyEnv, "  sk_test  ")
	t.Setenv(clerkWebhookSecretEnv, "  whsec_test  ")
	t.Setenv(databaseURLEnv, "  postgres://example  ")
	t.Setenv(portEnv, "   ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected config to load, got error: %v", err)
	}

	if cfg.ClerkSecretKey != "sk_test" {
		t.Fatalf("expected trimmed %s, got %q", clerkSecretKeyEnv, cfg.ClerkSecretKey)
	}
	if cfg.ClerkWebhookSecret != "whsec_test" {
		t.Fatalf("expected trimmed %s, got %q", clerkWebhookSecretEnv, cfg.ClerkWebhookSecret)
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("expected trimmed %s, got %q", databaseURLEnv, cfg.DatabaseURL)
	}
	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %q", cfg.Port)
	}
}

func TestLoadFallsBackToParentEnvPath(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}
	chdir(t, workDir)

	unsetEnv(t, clerkSecretKeyEnv)
	unsetEnv(t, clerkWebhookSecretEnv)
	unsetEnv(t, databaseURLEnv)
	unsetEnv(t, portEnv)

	parentEnv := strings.Join([]string{
		clerkSecretKeyEnv + "=sk_parent",
		clerkWebhookSecretEnv + "=whsec_parent",
		databaseURLEnv + "=postgres://parent",
		portEnv + "=9001",
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(parentEnv), 0o644); err != nil {
		t.Fatalf("failed to write parent .env: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected config to load from ../../.env, got error: %v", err)
	}
	if cfg.ClerkSecretKey != "sk_parent" {
		t.Fatalf("expected parent %s, got %q", clerkSecretKeyEnv, cfg.ClerkSecretKey)
	}
	if cfg.Port != "9001" {
		t.Fatalf("expected parent %s value, got %q", portEnv, cfg.Port)
	}
}

func TestLoadStopsAfterFirstEnvFile(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}
	chdir(t, workDir)

	unsetEnv(t, clerkSecretKeyEnv)
	unsetEnv(t, clerkWebhookSecretEnv)
	unsetEnv(t, databaseURLEnv)
	unsetEnv(t, portEnv)

	firstEnv := strings.Join([]string{
		clerkWebhookSecretEnv + "=whsec_first",
		databaseURLEnv + "=postgres://first",
		portEnv + "=8082",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workDir, ".env"), []byte(firstEnv), 0o644); err != nil {
		t.Fatalf("failed to write first .env: %v", err)
	}

	secondEnv := clerkSecretKeyEnv + "=sk_from_second\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(secondEnv), 0o644); err != nil {
		t.Fatalf("failed to write second .env: %v", err)
	}

	cfg, err := Load()
	if err == nil {
		t.Fatalf("expected missing env error, got cfg: %+v", cfg)
	}
	if !strings.Contains(err.Error(), clerkSecretKeyEnv) {
		t.Fatalf("expected missing %s because loader should stop at first .env, got %v", clerkSecretKeyEnv, err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	})
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	previousValue, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		var err error
		if existed {
			err = os.Setenv(key, previousValue)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("failed to restore %s: %v", key, err)
		}
	})
}
