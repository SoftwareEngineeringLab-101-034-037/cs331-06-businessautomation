package connectors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxIntegrationErrorBodyBytes = 16 * 1024

// IntegrationsGmailConnector sends workflow emails through the integrations
// service Gmail endpoint.
type IntegrationsGmailConnector struct {
	baseURL        string
	integrationKey string
	httpClient     *http.Client
}

func NewIntegrationsGmailConnector(baseURL, integrationKey string) *IntegrationsGmailConnector {
	resolvedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if resolvedBaseURL == "" {
		resolvedBaseURL = "http://localhost:8086"
	}
	return &IntegrationsGmailConnector{
		baseURL:        resolvedBaseURL,
		integrationKey: strings.TrimSpace(integrationKey),
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *IntegrationsGmailConnector) Send(to, subject, body string) error {
	return fmt.Errorf("org-aware send required for integrations gmail connector")
}

func (c *IntegrationsGmailConnector) SendForOrg(orgID, to, cc, subject, body, fromName, fromAccountID string) error {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return fmt.Errorf("org_id is required")
	}

	toRecipients := parseRecipients(to)
	if len(toRecipients) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	payload := map[string]interface{}{
		"to":        toRecipients,
		"cc":        parseRecipients(cc),
		"subject":   strings.TrimSpace(subject),
		"body_text": body,
	}
	trimmedFromName := strings.TrimSpace(fromName)
	if trimmedFromName != "" {
		payload["from_name"] = trimmedFromName
	}
	if strings.TrimSpace(fromAccountID) != "" {
		payload["from_account_id"] = strings.TrimSpace(fromAccountID)
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode gmail send payload: %w", err)
	}

	endpoint := c.baseURL + "/integrations/gmail/send?org_id=" + url.QueryEscape(orgID)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build gmail send request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.integrationKey != "" {
		req.Header.Set("X-Integration-Key", c.integrationKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call integrations gmail send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited := io.LimitReader(resp.Body, maxIntegrationErrorBodyBytes+1)
		rawBody, _ := io.ReadAll(limited)
		truncated := len(rawBody) > maxIntegrationErrorBodyBytes
		if truncated {
			rawBody = rawBody[:maxIntegrationErrorBodyBytes]
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		trimmed := strings.TrimSpace(string(rawBody))
		if truncated && trimmed != "" {
			trimmed += "... (truncated)"
		}
		if message := parseIntegrationErrorMessage(trimmed); message != "" {
			return fmt.Errorf("integrations gmail send failed status=%d error=%s", resp.StatusCode, message)
		}
		return fmt.Errorf("integrations gmail send failed status=%d body=%s", resp.StatusCode, trimmed)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("read integrations gmail send response: %w", err)
	}
	return nil
}

func parseIntegrationErrorMessage(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Error)
}

func parseRecipients(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	})
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
