package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the auth service
type Config struct {
	ClerkSecretKey     string
	ClerkWebhookSecret string
	DatabaseURL        string
	Port               string
}

func Load() (*Config, error) {
	envPaths := []string{
		".env",
		"../../.env",
		"../../../.env",
	}

	for _, path := range envPaths { //godotenv just loads the variables into the OS when it finds an .env file does not return the actual env vars
		err := godotenv.Load(path)
		if err == nil {
			break
		}
	}

	cfg := &Config{
		ClerkSecretKey:     strings.TrimSpace(getEnv("CLERK_SECRET_KEY", "")),
		ClerkWebhookSecret: strings.TrimSpace(getEnv("CLERK_WEBHOOK_SECRET", "")),
		DatabaseURL:        strings.TrimSpace(getEnv("DATABASE_URL", "")),
		Port:               strings.TrimSpace(getEnv("PORT", "8080")),
	}

	//If any required env vars are missing we return an error
	var missing []string
	if cfg.ClerkSecretKey == "" {
		missing = append(missing, "CLERK_SECRET_KEY")
	}
	if cfg.ClerkWebhookSecret == "" {
		missing = append(missing, "CLERK_WEBHOOK_SECRET")
	}
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.Port == "" {
		cfg.Port = "8080" // Default port
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}
