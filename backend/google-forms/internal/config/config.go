package config

import (
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
	WorkflowServiceKey  string
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
		WorkflowServiceKey:  strings.TrimSpace(os.Getenv("WORKFLOW_INTEGRATION_KEY")),
		PollIntervalSeconds: interval,
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
