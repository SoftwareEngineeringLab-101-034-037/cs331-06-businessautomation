package executor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type httpRoleDirectory struct {
	baseURL string
	client  *http.Client
}

type roleSummaryResponse struct {
	Name    string `json:"name"`
	Members []struct {
		ID string `json:"id"`
	} `json:"members"`
}

func NewHTTPRoleDirectory(baseURL string) RoleMemberDirectory {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return nil
	}
	return &httpRoleDirectory{
		baseURL: strings.TrimRight(trimmed, "/"),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (d *httpRoleDirectory) ListMemberIDs(orgID, roleName string) ([]string, error) {
	if strings.TrimSpace(orgID) == "" || strings.TrimSpace(roleName) == "" {
		return nil, nil
	}

	endpoint := fmt.Sprintf("%s/api/orgs/%s/roles", d.baseURL, url.PathEscape(orgID))
	resp, err := d.client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("role directory request failed: status=%d", resp.StatusCode)
	}

	var roles []roleSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return nil, err
	}

	targetRole := strings.ToLower(strings.TrimSpace(roleName))
	for _, role := range roles {
		if strings.ToLower(strings.TrimSpace(role.Name)) != targetRole {
			continue
		}
		result := make([]string, 0, len(role.Members))
		for _, member := range role.Members {
			if strings.TrimSpace(member.ID) == "" {
				continue
			}
			result = append(result, member.ID)
		}
		return result, nil
	}

	return nil, nil
}
