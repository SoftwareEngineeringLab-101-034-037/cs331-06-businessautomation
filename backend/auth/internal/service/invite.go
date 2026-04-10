package service

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
)

const clerkBaseURL = "https://api.clerk.com/v1"

var (
	ErrDuplicateInvite                   = errors.New("duplicate invitation")
	ErrAccountExists                     = errors.New("account already exists")
	ErrNotFound                          = errors.New("not found")
	ClerkRevokeOrgInvitationsByEmailFunc = revokeClerkOrgInvitationsByEmail
)

// InviteInput holds the parameters for creating a single employee invitation.
type InviteInput struct {
	OrgID        string
	Email        string
	FirstName    string
	LastName     string
	DepartmentID string
	Role         string
	Roles        []string
	JobTitle     string
	InvitedBy    string
}

type InviteResult struct {
	Invitation models.EmployeeInvitation `json:"invitation"`
}

func (s *EmployeeService) InviteAndNotify(input InviteInput) (*InviteResult, error) {
	var existingUser models.User
	err := s.db.Where(
		"organization_id = ? AND lower(email) = lower(?)",
		input.OrgID, input.Email,
	).First(&existingUser).Error
	if err == nil {
		return nil, fmt.Errorf("%w: employee account already exists for %s", ErrAccountExists, input.Email)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("user lookup failed for %s in org %s: %w", input.Email, input.OrgID, err)
	}

	var existing models.EmployeeInvitation
	err = s.db.Where(
		"email = ? AND organization_id = ? AND status = ?",
		input.Email, input.OrgID, "pending",
	).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: a pending invitation already exists for %s", ErrDuplicateInvite, input.Email)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database lookup failed for %s in org %s: %w", input.Email, input.OrgID, err)
	}

	deptID, err := s.resolveDepartmentID(input.OrgID, input.DepartmentID)
	if err != nil {
		return nil, fmt.Errorf("department lookup failed: %w", err)
	}

	// Generate a secure invite token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate invite token: %w", err)
	}
	tokenHash := sha256.Sum256(tokenBytes)
	hashedToken := hex.EncodeToString(tokenHash[:])

	invitation := models.EmployeeInvitation{
		OrganizationID: input.OrgID,
		DepartmentID:   deptID,
		Email:          input.Email,
		FirstName:      input.FirstName,
		LastName:       input.LastName,
		RoleName:       input.Role,
		RoleNames:      mustMarshalJSONStringArray(uniqueStrings(append(input.Roles, input.Role))),
		JobTitle:       input.JobTitle,
		Token:          hashedToken,
		Status:         "pending",
		InvitedBy:      input.InvitedBy,
		ExpiresAt:      time.Now().Add(7 * 24 * time.Hour), // 7-day expiry
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.db.Create(&invitation).Error; err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Send the invitation email via Clerk's organization invitation API
	if err := s.sendClerkOrgInvitation(input.OrgID, input.Email); err != nil {
		log.Printf("Warning: Clerk invitation email failed for %s: %v (local invitation still created)", input.Email, err)
	} else {
		log.Printf("Clerk invitation email sent to %s for org %s", input.Email, input.OrgID)
	}

	return &InviteResult{Invitation: invitation}, nil
}

// sendClerkOrgInvitation calls Clerk's API to create an organization invitation,
// which triggers Clerk to send the invitation email automatically.
func (s *EmployeeService) sendClerkOrgInvitation(orgID, email string) error {
	if s.clerkSecretKey == "" {
		return fmt.Errorf("clerk secret key not configured, skipping email")
	}

	url := fmt.Sprintf("%s/organizations/%s/invitations", clerkBaseURL, orgID)

	body, _ := json.Marshal(map[string]string{
		"email_address": email,
		"role":          "org:member",
	})

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.clerkSecretKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("clerk API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clerk API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

type clerkOrgInvitation struct {
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
	Status       string `json:"status"`
}

type clerkInvitationListPage struct {
	Data       []clerkOrgInvitation `json:"data"`
	TotalCount int                  `json:"totalCount"`
}

func revokeClerkOrgInvitationsByEmail(clerkSecretKey, orgID, email string) error {
	if clerkSecretKey == "" {
		return fmt.Errorf("clerk secret key not configured")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	const pageLimit = 100
	invites := make([]clerkOrgInvitation, 0)
	for offset := 0; ; offset += pageLimit {
		listURL := fmt.Sprintf("%s/organizations/%s/invitations?limit=%d&offset=%d", clerkBaseURL, orgID, pageLimit, offset)
		listReq, err := http.NewRequest(http.MethodGet, listURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create Clerk list-invitations request: %w", err)
		}
		listReq.Header.Set("Authorization", "Bearer "+clerkSecretKey)

		listResp, err := client.Do(listReq)
		if err != nil {
			return fmt.Errorf("clerk list invitations request failed: %w", err)
		}
		payload, err := io.ReadAll(listResp.Body)
		listResp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read clerk invitations response: %w", err)
		}
		if listResp.StatusCode >= 300 {
			return fmt.Errorf("clerk list invitations returned status %d: %s", listResp.StatusCode, string(payload))
		}

		pageInvites, totalCount, err := decodeClerkInvitationPage(payload)
		if err != nil {
			return err
		}
		invites = append(invites, pageInvites...)

		if len(pageInvites) == 0 {
			break
		}
		if totalCount > 0 && offset+pageLimit >= totalCount {
			break
		}
		if totalCount <= 0 && len(pageInvites) < pageLimit {
			break
		}
	}

	revokedCount := 0
	for _, invitation := range invites {
		if !strings.EqualFold(strings.TrimSpace(invitation.EmailAddress), strings.TrimSpace(email)) {
			continue
		}
		if invitation.Status != "pending" {
			continue
		}

		if err := revokeSingleClerkInvitation(client, clerkSecretKey, orgID, invitation.ID); err != nil {
			return err
		}
		revokedCount++
	}

	if revokedCount == 0 {
		return fmt.Errorf("no pending Clerk invitation found for %s in org %s", email, orgID)
	}

	return nil
}

func decodeClerkInvitationPage(payload []byte) ([]clerkOrgInvitation, int, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, 0, nil
	}
	if trimmed[0] == '[' {
		var invites []clerkOrgInvitation
		if err := json.Unmarshal(trimmed, &invites); err != nil {
			return nil, 0, fmt.Errorf("failed to decode clerk invitations payload: %w", err)
		}
		return invites, 0, nil
	}

	var wrapped clerkInvitationListPage
	if err := json.Unmarshal(trimmed, &wrapped); err != nil {
		return nil, 0, fmt.Errorf("failed to decode clerk invitations payload: %w", err)
	}
	return wrapped.Data, wrapped.TotalCount, nil
}

func revokeSingleClerkInvitation(client *http.Client, clerkSecretKey, orgID, invitationID string) error {
	revokeURL := fmt.Sprintf("%s/organizations/%s/invitations/%s/revoke", clerkBaseURL, orgID, invitationID)
	req, err := http.NewRequest(http.MethodPost, revokeURL, bytes.NewBufferString("{}"))
	if err != nil {
		return fmt.Errorf("failed to create Clerk revoke request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+clerkSecretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("clerk revoke invitation request failed: %w", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("clerk revoke invitation %s returned status %d: %s", invitationID, resp.StatusCode, string(respBody))
}

// AcceptInvitationByEmail finds a pending invitation matching the email and
// org, marks it as accepted, and assigns the department/role to the user.
func (s *EmployeeService) AcceptInvitationByEmail(email, orgID, userID string) error {
	var invitation models.EmployeeInvitation
	err := s.db.Where(
		"email = ? AND organization_id = ? AND status = ?",
		email, orgID, "pending",
	).First(&invitation).Error
	if err != nil {
		return fmt.Errorf("no pending invitation found for %s in org %s", email, orgID)
	}

	if invitation.IsExpired() {
		if err := s.db.Model(&invitation).Update("status", "expired").Error; err != nil {
			return fmt.Errorf("failed to mark invitation as expired: %w", err)
		}
		return fmt.Errorf("invitation for %s has expired", email)
	}

	now := time.Now()

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Mark invitation as accepted
		if err := tx.Model(&invitation).Updates(map[string]interface{}{
			"status":           "accepted",
			"accepted_at":      now,
			"accepted_user_id": userID,
			"updated_at":       now,
		}).Error; err != nil {
			return fmt.Errorf("failed to accept invitation: %w", err)
		}

		// Build user updates
		userUpdates := map[string]interface{}{
			"organization_id": invitation.OrganizationID,
			"department_id":   invitation.DepartmentID,
			"updated_at":      now,
		}
		if trimmedFirstName := strings.TrimSpace(invitation.FirstName); trimmedFirstName != "" {
			userUpdates["first_name"] = trimmedFirstName
		}
		if trimmedLastName := strings.TrimSpace(invitation.LastName); trimmedLastName != "" {
			userUpdates["last_name"] = trimmedLastName
		}
		if invitation.JobTitle != "" {
			userUpdates["job_title"] = invitation.JobTitle
		}

		result := tx.Model(&models.User{}).Where("id = ?", userID).Updates(userUpdates)
		if result.Error != nil {
			return fmt.Errorf("failed to update user after invitation acceptance: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("user %s not found, rolling back invitation acceptance", userID)
		}

		roleNames := invitedRoleNames(invitation)
		if err := s.AssignRoleNamesToUser(tx, orgID, userID, invitation.InvitedBy, roleNames); err != nil {
			return err
		}

		log.Printf("Invitation accepted: user %s joined org %s, assigned to department %s",
			userID, orgID, invitation.DepartmentID)
		return nil
	})
}

// resolveDepartmentID tries to find a department by name within the org first,
// then falls back to treating the input as a direct department ID.
func (s *EmployeeService) resolveDepartmentID(orgID, nameOrID string) (string, error) {
	// Try by name first
	var dept models.Department
	err := s.db.Where("name = ? AND organization_id = ?", nameOrID, orgID).First(&dept).Error
	if err == nil {
		return dept.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("DB error looking up department %q in org %s: %w", nameOrID, orgID, err)
	}

	// Fall back to treating as a direct ID
	err = s.db.Where("id = ? AND organization_id = ?", nameOrID, orgID).First(&dept).Error
	if err == nil {
		return dept.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("DB error looking up department %q in org %s: %w", nameOrID, orgID, err)
	}

	return "", fmt.Errorf("%w: department %q not found in organization %s", ErrNotFound, nameOrID, orgID)
}

// AcceptInvitationByID finds a pending invitation by its ID, verifies that the
// accepting user's email matches, and then accepts it (marks as accepted, assigns
// department/role to the user).
func (s *EmployeeService) AcceptInvitationByID(invitationID, orgID, userID string) error {
	// Look up the user to get their email
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Look up the invitation
	var invitation models.EmployeeInvitation
	err := s.db.Where(
		"id = ? AND organization_id = ? AND status = ?",
		invitationID, orgID, "pending",
	).First(&invitation).Error
	if err != nil {
		return fmt.Errorf("%w: invitation not found or already processed", ErrNotFound)
	}

	// Verify the invitation is for this user
	if invitation.Email != user.Email {
		return fmt.Errorf("invitation email does not match user email")
	}

	// Check expiry
	if invitation.IsExpired() {
		if err := s.db.Model(&invitation).Update("status", "expired").Error; err != nil {
			return fmt.Errorf("failed to mark invitation as expired: %w", err)
		}
		return fmt.Errorf("%w: invitation has expired", ErrNotFound)
	}

	// Delegate to existing acceptance logic
	return s.AcceptInvitationByEmail(user.Email, orgID, userID)
}

func mustMarshalJSONStringArray(values []string) []byte {
	if len(values) == 0 {
		return []byte("[]")
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return []byte("[]")
	}
	return encoded
}

func invitedRoleNames(invitation models.EmployeeInvitation) []string {
	var roleNames []string
	if len(invitation.RoleNames) > 0 {
		if err := json.Unmarshal(invitation.RoleNames, &roleNames); err != nil {
			log.Printf("failed to decode invitation role tags for invitation %s: %v", invitation.ID, err)
		}
	}
	if invitation.RoleName != "" {
		roleNames = append(roleNames, invitation.RoleName)
	}
	return uniqueStrings(roleNames)
}
