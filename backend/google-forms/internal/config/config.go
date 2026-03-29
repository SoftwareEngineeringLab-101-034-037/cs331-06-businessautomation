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
	WorkflowEngineURL   string
	PollIntervalSeconds int
}

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
		MongoDB:             getenv("MONGO_DB", "google_forms_service"),
		GoogleClientID:      strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		GoogleClientSecret:  strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")),
		GoogleRedirectURI:   strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URI")),
		WorkflowEngineURL:   getenv("WORKFLOW_ENGINE_URL", "http://localhost:8085"),
		PollIntervalSeconds: interval,
	}

	var missing []string
	if cfg.GoogleClientID == "" {
		missing = append(missing, "GOOGLE_CLIENT_ID")
	}
	if cfg.GoogleClientSecret == "" {
		missing = append(missing, "GOOGLE_CLIENT_SECRET")
	}
	if cfg.MongoURI == "mongodb://localhost:27017" && os.Getenv("MONGO_URI") == "" {
		// not fatal, just a default — skip
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
