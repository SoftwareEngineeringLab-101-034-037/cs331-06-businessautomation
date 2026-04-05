package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	MongoURI            string
	MongoDB             string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleRedirectURI   string
	AuthServiceURL      string
	WorkflowEngineURL   string
	WorkflowServiceKey  string
	CORSAllowedOrigins  []string
	PollIntervalSeconds int
}

const defaultMongoDBName = "google_forms_service"

func Load() (*Config, error) {
	for _, path := range []string{".env", "../../.env", "../../../.env"} {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}

	interval := 60
	if v, err := strconv.Atoi(os.Getenv("POLL_INTERVAL_SECONDS")); err == nil && v > 0 {
		interval = v
	}

	cfg := &Config{
		Port:                getenv("PORT", "8086"),
		MongoURI:            getenv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:             getenv("MONGO_DB", defaultMongoDBName),
		GoogleClientID:      strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		GoogleClientSecret:  strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")),
		GoogleRedirectURI:   strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URI")),
		AuthServiceURL:      getenv("AUTH_SERVICE_URL", "http://localhost:8080"),
		WorkflowEngineURL:   getenv("WORKFLOW_ENGINE_URL", "http://localhost:8085"),
		WorkflowServiceKey:  strings.TrimSpace(os.Getenv("WORKFLOW_INTEGRATION_KEY")),
		CORSAllowedOrigins:  parseCSVEnvWithDefault("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://127.0.0.1:3000"}),
		PollIntervalSeconds: interval,
	}

	missing := make([]string, 0, 3)
	if cfg.GoogleClientID == "" {
		missing = append(missing, "GOOGLE_CLIENT_ID")
	}
	if cfg.GoogleClientSecret == "" {
		missing = append(missing, "GOOGLE_CLIENT_SECRET")
	}
	if cfg.WorkflowServiceKey == "" {
		missing = append(missing, "WORKFLOW_INTEGRATION_KEY")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseCSVEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func parseCSVEnvWithDefault(key string, fallback []string) []string {
	values := parseCSVEnv(key)
	if len(values) > 0 {
		return values
	}
	if len(fallback) == 0 {
		return nil
	}
	out := make([]string, len(fallback))
	copy(out, fallback)
	return out
}
