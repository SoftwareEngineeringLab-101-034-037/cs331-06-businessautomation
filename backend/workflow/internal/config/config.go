package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the workflow service.
type Config struct {
	MongoURI         string
	ClerkIssuerURL   string
	AuthServiceURL   string
	IntegrationsURL  string
	AuthServiceToken string
	IntegrationKey   string
	Port             string
}

// Load reads configuration from environment variables, trying several .env
// file locations the same way the auth service does.
func Load() (*Config, error) {
	envPaths := []string{
		".env",
		"../../.env",
		"../../../.env",
	}

	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}

	cfg := &Config{
		MongoURI:         strings.TrimSpace(getEnv("MONGO_URI", "")),
		ClerkIssuerURL:   strings.TrimSpace(getEnv("CLERK_ISSUER_URL", "")),
		AuthServiceURL:   strings.TrimSpace(getEnv("AUTH_SERVICE_URL", "http://localhost:8080")),
		IntegrationsURL:  strings.TrimSpace(getEnv("INTEGRATIONS_SERVICE_URL", "http://localhost:8086")),
		AuthServiceToken: strings.TrimSpace(getEnv("AUTH_SERVICE_TOKEN", "")),
		IntegrationKey:   strings.TrimSpace(getEnv("WORKFLOW_INTEGRATION_KEY", "")),
		Port:             strings.TrimSpace(getEnv("PORT", "8085")),
	}

	var missing []string
	if cfg.MongoURI == "" {
		missing = append(missing, "MONGO_URI")
	}
	if cfg.ClerkIssuerURL == "" {
		missing = append(missing, "CLERK_ISSUER_URL")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
