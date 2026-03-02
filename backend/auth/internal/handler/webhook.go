package handler

import (
	"io"
	"log"
	"net/http"
	"time"

	"encoding/json"

	"github.com/gin-gonic/gin"
	svix "github.com/svix/svix-webhooks/go"
	"gorm.io/gorm"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/database"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/service"
)

type WebhookHandler struct {
	webhookSecret   string
	employeeService *service.EmployeeService
}

func NewWebhookHandler(secret string, empSvc *service.EmployeeService) *WebhookHandler {
	return &WebhookHandler{webhookSecret: secret, employeeService: empSvc}
}

// ClerkWebhookEvent represents the structure of a Clerk webhook payload
type ClerkWebhookEvent struct {
	Type   string          `json:"type"`
	Data   json.RawMessage `json:"data"`
	Object string          `json:"object"`
}

// ClerkUser represents user data from Clerk webhook
type ClerkUser struct {
	ID             string `json:"id"`
	EmailAddresses []struct {
		EmailAddress string `json:"email_address"`
	} `json:"email_addresses"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ImageURL     string `json:"image_url"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	LastSignInAt *int64 `json:"last_sign_in_at"`
}

// ClerkOrganization represents organization data from Clerk webhook
type ClerkOrganization struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	ImageURL  string `json:"image_url"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type ClerkMembershipData struct {
	ID           string `json:"id"`
	Role         string `json:"role"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	Organization struct {
		ID string `json:"id"`
	} `json:"organization"`
	PublicUserData struct {
		UserID string `json:"user_id"`
	} `json:"public_user_data"`
}

// Handle processes incoming Clerk webhooks
func (h *WebhookHandler) Handle(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Error reading webhook body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error reading request body"})
		return
	}

	// Verify the webhook signature using Svix. We do this because anyone on the web can send a webhook request to our backendURL. clerk signs its webhooks with a secret signature here we verify the webhook is indeed from clerk.
	wh, err := svix.NewWebhook(h.webhookSecret)
	if err != nil {
		log.Printf("Error creating webhook verifier: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	headers := http.Header{}
	headers.Set("svix-id", c.GetHeader("svix-id"))
	headers.Set("svix-timestamp", c.GetHeader("svix-timestamp"))
	headers.Set("svix-signature", c.GetHeader("svix-signature"))

	err = wh.Verify(body, headers)
	if err != nil {
		log.Printf("Invalid webhook signature: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// Parse the webhook event
	var event ClerkWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Error parsing webhook event: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing event"})
		return
	}

	log.Printf("Received Clerk webhook: %s", event.Type)

	switch event.Type {
	case "user.created":
		if err := h.handleUserCreated(event.Data); err != nil {
			log.Printf("Error handling user created event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling user created event"})
			return
		}
	case "user.updated":
		if err := h.handleUserUpdated(event.Data); err != nil {
			log.Printf("Error handling user updated event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling user updated event"})
			return
		}
	case "user.deleted":
		if err := h.handleUserDeleted(event.Data); err != nil {
			log.Printf("Error handling user deleted event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling user deleted event"})
			return
		}
	case "organization.created":
		if err := h.handleOrganizationCreated(event.Data); err != nil {
			log.Printf("Error handling organization created event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling organization created event"})
			return
		}
	case "organization.deleted":
		if err := h.handleOrganizationDeleted(event.Data); err != nil {
			log.Printf("Error handling organization deleted event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling organization deleted event"})
			return
		}
	case "organizationMembership.created":
		if err := h.handleMembershipCreated(event.Data); err != nil {
			log.Printf("Error handling membership created event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling membership created event"})
			return
		}
	default:
		log.Printf("Unhandled webhook event type: %s", event.Type)
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// This function handles the backend when clerk sends a webhook saying a new user is created
func (h *WebhookHandler) handleUserCreated(data json.RawMessage) error {
	var clerkUser ClerkUser
	if err := json.Unmarshal(data, &clerkUser); err != nil {
		log.Printf("Error parsing user data: %v", err)
		return err
	}

	email := ""
	if len(clerkUser.EmailAddresses) > 0 {
		email = clerkUser.EmailAddresses[0].EmailAddress
	}

	user := models.User{
		ID:        clerkUser.ID,
		Email:     email,
		FirstName: clerkUser.FirstName,
		LastName:  clerkUser.LastName,
		AvatarURL: clerkUser.ImageURL,
		IsActive:  true,
		CreatedAt: time.UnixMilli(clerkUser.CreatedAt),
		UpdatedAt: time.UnixMilli(clerkUser.UpdatedAt),
	}

	err := database.DB.Create(&user).Error
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return err
	}

	log.Printf("User created: %s (%s)", user.ID, user.Email)
	return nil
}

// This function handles the backend when clerk sends a webhook saying a user is updated
func (h *WebhookHandler) handleUserUpdated(data json.RawMessage) error {
	var clerkUser ClerkUser
	if err := json.Unmarshal(data, &clerkUser); err != nil {
		log.Printf("Error parsing user data: %v", err)
		return err
	}

	email := ""
	if len(clerkUser.EmailAddresses) > 0 {
		email = clerkUser.EmailAddresses[0].EmailAddress
	}

	updates := map[string]interface{}{
		"email":      email,
		"first_name": clerkUser.FirstName,
		"last_name":  clerkUser.LastName,
		"avatar_url": clerkUser.ImageURL,
		"updated_at": time.UnixMilli(clerkUser.UpdatedAt),
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", clerkUser.ID).Updates(updates).Error; err != nil {
		log.Printf("Error updating user: %v", err)
		return err
	}

	log.Printf("User updated: %s", clerkUser.ID)
	return nil
}

// This function handles the backend when clerk sends a webhook saying a user is deleted
func (h *WebhookHandler) handleUserDeleted(data json.RawMessage) error {
	var clerkUser struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &clerkUser); err != nil {
		log.Printf("Error parsing user data: %v", err)
		return err
	}

	// Soft delete - set isActive to false
	if err := database.DB.Model(&models.User{}).Where("id = ?", clerkUser.ID).Update("is_active", false).Error; err != nil {
		log.Printf("Error deleting user: %v", err)
		return err
	}

	log.Printf("User deleted (soft): %s", clerkUser.ID)
	return nil
}

// This function handles the backend when clerk sends a webhook saying a new organization is created
func (h *WebhookHandler) handleOrganizationCreated(data json.RawMessage) error {
	var clerkOrg ClerkOrganization
	if err := json.Unmarshal(data, &clerkOrg); err != nil {
		log.Printf("Error parsing organization data: %v", err)
		return err
	}

	// Get the creator's user ID from the webhook - this will be the org admin
	// Clerk sends created_by in the organization webhook
	var rawData map[string]interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		log.Printf("Error parsing raw organization data: %v", err)
		return err
	}

	orgAdminID := getString(rawData, "created_by")

	org := models.Organization{
		ID:        clerkOrg.ID,
		Name:      clerkOrg.Name,
		Slug:      clerkOrg.Slug,
		ImageURL:  clerkOrg.ImageURL,
		IsActive:  true,
		CreatedAt: time.UnixMilli(clerkOrg.CreatedAt),
		UpdatedAt: time.UnixMilli(clerkOrg.UpdatedAt),
	}

	// Only set org_admin_id if the user actually exists in our DB
	// (the user.created webhook may arrive late or fail)
	if orgAdminID != "" {
		var userCount int64
		database.DB.Model(&models.User{}).Where("id = ?", orgAdminID).Count(&userCount)
		if userCount > 0 {
			org.OrgAdminID = &orgAdminID
		} else {
			log.Printf("Admin user %s not yet in DB for org %s — will be linked via membership webhook", orgAdminID, clerkOrg.ID)
		}
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&org).Error; err != nil {
			return err
		}

		settings := models.OrganizationSettings{
			OrganizationID: clerkOrg.ID,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if err := tx.Create(&settings).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Printf("Error creating organization and settings: %v", err)
		return err
	}

	log.Printf("Organization created: %s (%s) with admin: %s", clerkOrg.ID, clerkOrg.Name, orgAdminID)
	return nil
}

func (h *WebhookHandler) handleOrganizationDeleted(data json.RawMessage) error {
	var clerkOrg struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &clerkOrg); err != nil {
		log.Printf("Error parsing organization data: %v", err)
		return err
	}

	// Soft delete
	if err := database.DB.Model(&models.Organization{}).Where("id = ?", clerkOrg.ID).Update("is_active", false).Error; err != nil {
		log.Printf("Error deleting organization: %v", err)
		return err
	}

	log.Printf("Organization deleted (soft): %s", clerkOrg.ID)
	return nil
}

func (h *WebhookHandler) handleMembershipCreated(data json.RawMessage) error {
	var memberData ClerkMembershipData
	if err := json.Unmarshal(data, &memberData); err != nil {
		log.Printf("Error parsing membership data: %v", err)
		return err
	}

	userID := memberData.PublicUserData.UserID
	orgID := memberData.Organization.ID

	// 1. Create OrganizationMembership record in our DB
	membership := models.OrganizationMembership{
		ID:             memberData.ID,
		UserID:         userID,
		OrganizationID: orgID,
		ClerkRole:      memberData.Role,
		JoinedAt:       time.UnixMilli(memberData.CreatedAt),
		CreatedAt:      time.UnixMilli(memberData.CreatedAt),
		UpdatedAt:      time.UnixMilli(memberData.UpdatedAt),
	}
	if err := database.DB.Create(&membership).Error; err != nil {
		log.Printf("Error creating membership record (may already exist): %v", err)
	}

	// 2. Look up user's email to match against pending invitations
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		log.Printf("User %s not found for membership linking: %v", userID, err)
		return nil
	}

	// 3. Try to auto-accept a matching invitation (assigns department + role)
	if h.employeeService != nil {
		if err := h.employeeService.AcceptInvitationByEmail(user.Email, orgID, userID); err != nil {
			log.Printf("Invitation auto-accept note: %v", err)
		}
	}

	log.Printf("Membership created: user %s joined org %s as %s", userID, orgID, memberData.Role)
	return nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
