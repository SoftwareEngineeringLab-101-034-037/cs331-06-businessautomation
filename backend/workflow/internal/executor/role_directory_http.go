package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type httpRoleDirectory struct {
	baseURL   string
	authToken string
	client    *http.Client
}

type roleSummaryResponse struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	CreatedByUserID string    `json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
	Members         []struct {
		ID        string `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
	} `json:"members"`
}

var (
	ErrInvalidArgs  = errors.New("invalid role directory arguments")
	ErrRoleNotFound = errors.New("role not found")
	ErrNoMembers    = errors.New("role has no members")
)

func NewHTTPRoleDirectory(baseURL, authToken string) (RoleMemberDirectory, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return nil, fmt.Errorf("empty baseURL for HTTP role directory")
	}
	return &httpRoleDirectory{
		baseURL:   strings.TrimRight(trimmed, "/"),
		authToken: strings.TrimSpace(authToken),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

func (d *httpRoleDirectory) ListMemberIDs(orgID, roleName string) ([]string, error) {
	return d.listMemberIDs(orgID, roleName, "")
}

func (d *httpRoleDirectory) ListMemberIDsWithAuth(orgID, roleName, authHeader string) ([]string, error) {
	return d.listMemberIDs(orgID, roleName, authHeader)
}

func (d *httpRoleDirectory) listMemberIDs(orgID, roleName, authHeader string) ([]string, error) {
	if strings.TrimSpace(orgID) == "" || strings.TrimSpace(roleName) == "" {
		log.Printf("role_directory_http.ListMemberIDs invalid args org_id=%q role=%q", orgID, roleName)
		return nil, ErrInvalidArgs
	}

	endpoint := fmt.Sprintf("%s/api/orgs/%s/roles", d.baseURL, url.PathEscape(orgID))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	switch {
	case strings.TrimSpace(authHeader) != "":
		req.Header.Set("Authorization", strings.TrimSpace(authHeader))
	case d.authToken != "":
		req.Header.Set("Authorization", "Bearer "+d.authToken)
	}

	resp, err := d.client.Do(req)
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
		if len(result) == 0 {
			log.Printf("role_directory_http.ListMemberIDs no members org_id=%q role=%q", orgID, roleName)
			return []string{}, nil
		}
		return result, nil
	}

	log.Printf("role_directory_http.ListMemberIDs role not found org_id=%q role=%q", orgID, roleName)
	return nil, ErrRoleNotFound
}
