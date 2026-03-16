package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/database"
	"github.com/gin-gonic/gin"
	svix "github.com/svix/svix-webhooks/go"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestHandleReturnsUnauthorizedForInvalidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewWebhookHandler(testWebhookSecret(), nil)

	body := mustMarshal(t, map[string]interface{}{
		"type":   "ignored.event",
		"data":   map[string]interface{}{},
		"object": "event",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/clerk", bytes.NewReader(body))
	// Intentionally invalid signature headers.
	req.Header.Set("svix-id", "msg_bad")
	req.Header.Set("svix-timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("svix-signature", "v1,invalid")
	c.Request = req

	handler.Handle(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleReturnsOKForValidSignedUnknownEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewWebhookHandler(testWebhookSecret(), nil)

	body := mustMarshal(t, map[string]interface{}{
		"type":   "some.unhandled.event",
		"data":   map[string]interface{}{},
		"object": "event",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/clerk", bytes.NewReader(body))
	addValidSvixHeaders(t, req, testWebhookSecret(), body)
	c.Request = req

	handler.Handle(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"received":true`) {
		t.Fatalf("expected response body to confirm receipt, got %s", w.Body.String())
	}
}

func TestHandleReturnsInternalServerErrorWhenHandlerFails(t *testing.T) {
	restoreDB(t)
	db := setupWebhookTestDB(t)
	database.DB = db

	// Pre-insert same user ID to force duplicate PK failure on user.created.
	if err := db.Exec(`
		INSERT INTO users (id, email, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, "user_dup", "dup@example.com", true, time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("failed to seed duplicate user: %v", err)
	}

	gin.SetMode(gin.TestMode)
	handler := NewWebhookHandler(testWebhookSecret(), nil)
	body := mustMarshal(t, map[string]interface{}{
		"type": "user.created",
		"data": map[string]interface{}{
			"id": "user_dup",
			"email_addresses": []map[string]interface{}{
				{"email_address": "dup@example.com"},
			},
			"first_name": "Dup",
			"last_name":  "User",
			"image_url":  "https://example.com/avatar.png",
			"created_at": time.Now().UnixMilli(),
			"updated_at": time.Now().UnixMilli(),
		},
		"object": "event",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/clerk", bytes.NewReader(body))
	addValidSvixHeaders(t, req, testWebhookSecret(), body)
	c.Request = req

	handler.Handle(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleUserCreatedCreatesRow(t *testing.T) {
	restoreDB(t)
	db := setupWebhookTestDB(t)
	database.DB = db

	handler := NewWebhookHandler("", nil)
	data := mustMarshal(t, map[string]interface{}{
		"id": "user_1",
		"email_addresses": []map[string]interface{}{
			{"email_address": "user1@example.com"},
		},
		"first_name": "Jane",
		"last_name":  "Doe",
		"image_url":  "https://example.com/jane.png",
		"created_at": time.Now().UnixMilli(),
		"updated_at": time.Now().UnixMilli(),
	})

	if err := handler.handleUserCreated(data); err != nil {
		t.Fatalf("expected no error creating user, got %v", err)
	}

	var count int64
	if err := db.Table("users").Where("id = ?", "user_1").Count(&count).Error; err != nil {
		t.Fatalf("failed counting users: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 user row, got %d", count)
	}
}

func TestHandleUserUpdatedUpdatesExistingRow(t *testing.T) {
	restoreDB(t)
	db := setupWebhookTestDB(t)
	database.DB = db

	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, avatar_url, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_2", "before@example.com", "Before", "Name", "before.png", true, time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}

	handler := NewWebhookHandler("", nil)
	data := mustMarshal(t, map[string]interface{}{
		"id": "user_2",
		"email_addresses": []map[string]interface{}{
			{"email_address": "after@example.com"},
		},
		"first_name": "After",
		"last_name":  "Name",
		"image_url":  "after.png",
		"updated_at": time.Now().UnixMilli(),
	})

	if err := handler.handleUserUpdated(data); err != nil {
		t.Fatalf("expected no error updating user, got %v", err)
	}

	var row struct {
		Email     string
		FirstName string `gorm:"column:first_name"`
	}
	if err := db.Table("users").Select("email, first_name").Where("id = ?", "user_2").Take(&row).Error; err != nil {
		t.Fatalf("failed reading updated user: %v", err)
	}
	if row.Email != "after@example.com" || row.FirstName != "After" {
		t.Fatalf("unexpected updated values: %+v", row)
	}
}

func TestHandleUserDeletedMarksInactive(t *testing.T) {
	restoreDB(t)
	db := setupWebhookTestDB(t)
	database.DB = db

	if err := db.Exec(`
		INSERT INTO users (id, email, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, "user_3", "user3@example.com", true, time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}

	handler := NewWebhookHandler("", nil)
	data := mustMarshal(t, map[string]interface{}{"id": "user_3"})
	if err := handler.handleUserDeleted(data); err != nil {
		t.Fatalf("expected no error deleting user, got %v", err)
	}

	var isActive bool
	if err := db.Table("users").Select("is_active").Where("id = ?", "user_3").Scan(&isActive).Error; err != nil {
		t.Fatalf("failed reading user state: %v", err)
	}
	if isActive {
		t.Fatal("expected user to be inactive after soft delete")
	}
}

func TestHandleOrganizationCreatedCreatesOrgAndSettings(t *testing.T) {
	restoreDB(t)
	db := setupWebhookTestDB(t)
	database.DB = db

	if err := db.Exec(`
		INSERT INTO users (id, email, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, "user_admin", "admin@example.com", true, time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("failed seeding org creator user: %v", err)
	}

	handler := NewWebhookHandler("", nil)
	data := mustMarshal(t, map[string]interface{}{
		"id":         "org_1",
		"name":       "Org One",
		"slug":       "org-one",
		"image_url":  "https://example.com/org.png",
		"created_at": time.Now().UnixMilli(),
		"updated_at": time.Now().UnixMilli(),
		"created_by": "user_admin",
	})

	if err := handler.handleOrganizationCreated(data); err != nil {
		t.Fatalf("expected no error creating org, got %v", err)
	}

	var orgCount int64
	if err := db.Table("organizations").Where("id = ?", "org_1").Count(&orgCount).Error; err != nil {
		t.Fatalf("failed counting organizations: %v", err)
	}
	if orgCount != 1 {
		t.Fatalf("expected 1 organization row, got %d", orgCount)
	}

	var settingsCount int64
	if err := db.Table("organization_settings").Where("organization_id = ?", "org_1").Count(&settingsCount).Error; err != nil {
		t.Fatalf("failed counting organization settings: %v", err)
	}
	if settingsCount != 1 {
		t.Fatalf("expected 1 organization_settings row, got %d", settingsCount)
	}

	var dept struct {
		ID              string
		Name            string
		CreatedByUserID string `gorm:"column:created_by_user_id"`
	}
	if err := db.Table("departments").Select("id, name, created_by_user_id").Where("organization_id = ?", "org_1").Take(&dept).Error; err != nil {
		t.Fatalf("expected admin department row, got error: %v", err)
	}
	if dept.Name != "Admin" {
		t.Fatalf("expected default admin department, got %q", dept.Name)
	}
	if dept.CreatedByUserID != "user_admin" {
		t.Fatalf("expected department created_by_user_id user_admin, got %q", dept.CreatedByUserID)
	}

	var creator struct {
		OrganizationID *string `gorm:"column:organization_id"`
		DepartmentID   *string `gorm:"column:department_id"`
		IsAdmin        bool    `gorm:"column:is_admin"`
	}
	if err := db.Table("users").Select("organization_id, department_id, is_admin").Where("id = ?", "user_admin").Take(&creator).Error; err != nil {
		t.Fatalf("failed reading creator row: %v", err)
	}
	if creator.OrganizationID == nil || *creator.OrganizationID != "org_1" {
		t.Fatalf("expected creator organization_id org_1, got %+v", creator.OrganizationID)
	}
	if creator.DepartmentID == nil || *creator.DepartmentID != dept.ID {
		t.Fatalf("expected creator department_id %s, got %+v", dept.ID, creator.DepartmentID)
	}
	if !creator.IsAdmin {
		t.Fatal("expected creator to be marked admin")
	}
}

func TestHandleOrganizationCreatedRollsBackWhenSettingsInsertFails(t *testing.T) {
	restoreDB(t)
	db := setupWebhookTestDB(t)
	database.DB = db

	// Pre-create settings with the same organization_id to force unique violation.
	if err := db.Exec(`
		INSERT INTO organization_settings (id, organization_id, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, "settings_seed", "org_tx_rollback", time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("failed seeding organization settings: %v", err)
	}

	handler := NewWebhookHandler("", nil)
	data := mustMarshal(t, map[string]interface{}{
		"id":         "org_tx_rollback",
		"name":       "Rollback Org",
		"slug":       "rollback-org",
		"image_url":  "https://example.com/org2.png",
		"created_at": time.Now().UnixMilli(),
		"updated_at": time.Now().UnixMilli(),
	})

	err := handler.handleOrganizationCreated(data)
	if err == nil {
		t.Fatal("expected organization create to fail due settings unique conflict")
	}

	var orgCount int64
	if err := db.Table("organizations").Where("id = ?", "org_tx_rollback").Count(&orgCount).Error; err != nil {
		t.Fatalf("failed counting organizations: %v", err)
	}
	if orgCount != 0 {
		t.Fatalf("expected organization insert rollback, found %d rows", orgCount)
	}
}

func TestHandleOrganizationDeletedMarksInactive(t *testing.T) {
	restoreDB(t)
	db := setupWebhookTestDB(t)
	database.DB = db

	if err := db.Exec(`
		INSERT INTO organizations (id, name, slug, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "org_2", "Org Two", "org-two", true, time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("failed seeding organization: %v", err)
	}

	handler := NewWebhookHandler("", nil)
	data := mustMarshal(t, map[string]interface{}{"id": "org_2"})
	if err := handler.handleOrganizationDeleted(data); err != nil {
		t.Fatalf("expected no error deleting organization, got %v", err)
	}

	var isActive bool
	if err := db.Table("organizations").Select("is_active").Where("id = ?", "org_2").Scan(&isActive).Error; err != nil {
		t.Fatalf("failed reading organization state: %v", err)
	}
	if isActive {
		t.Fatal("expected organization to be inactive after soft delete")
	}
}

func TestHandleMembershipCreatedWhenUserMissing(t *testing.T) {
	restoreDB(t)
	db := setupWebhookTestDB(t)
	database.DB = db

	handler := NewWebhookHandler("", nil)
	data := mustMarshal(t, map[string]interface{}{
		"id":         "mem_1",
		"role":       "org:member",
		"created_at": time.Now().UnixMilli(),
		"updated_at": time.Now().UnixMilli(),
		"organization": map[string]interface{}{
			"id": "org_1",
		},
		"public_user_data": map[string]interface{}{
			"user_id": "missing_user",
		},
	})

	if err := handler.handleMembershipCreated(data); err != nil {
		t.Fatalf("expected no error handling membership created for missing user, got %v", err)
	}
}

func TestHandleMembershipCreatedUpdatesUserOrgAndAdmin(t *testing.T) {
	restoreDB(t)
	db := setupWebhookTestDB(t)
	database.DB = db

	if err := db.Exec(`
		INSERT INTO users (id, email, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, "user_4", "user4@example.com", true, time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}

	handler := NewWebhookHandler("", nil)
	data := mustMarshal(t, map[string]interface{}{
		"id":         "mem_2",
		"role":       "org:admin",
		"created_at": time.Now().UnixMilli(),
		"updated_at": time.Now().UnixMilli(),
		"organization": map[string]interface{}{
			"id": "org_1",
		},
		"public_user_data": map[string]interface{}{
			"user_id": "user_4",
		},
	})

	if err := handler.handleMembershipCreated(data); err != nil {
		t.Fatalf("expected no error handling membership created, got %v", err)
	}

	var row struct {
		OrganizationID string `gorm:"column:organization_id"`
		DepartmentID   string `gorm:"column:department_id"`
		IsAdmin        bool   `gorm:"column:is_admin"`
	}
	if err := db.Table("users").Select("organization_id, department_id, is_admin").Where("id = ?", "user_4").Take(&row).Error; err != nil {
		t.Fatalf("failed reading user row: %v", err)
	}
	if row.OrganizationID != "org_1" {
		t.Fatalf("expected organization_id=org_1, got %q", row.OrganizationID)
	}
	if row.DepartmentID == "" {
		t.Fatal("expected admin membership to assign default admin department")
	}
	if !row.IsAdmin {
		t.Fatal("expected is_admin=true for org:admin role")
	}
}

func TestHandleMembershipCreatedReturnsErrorForInvalidPayload(t *testing.T) {
	restoreDB(t)
	db := setupWebhookTestDB(t)
	database.DB = db

	handler := NewWebhookHandler("", nil)
	if err := handler.handleMembershipCreated(json.RawMessage(`{"id":`)); err == nil {
		t.Fatal("expected invalid membership payload to return error")
	}
}

func TestGetString(t *testing.T) {
	if got := getString(map[string]interface{}{"created_by": "user_123"}, "created_by"); got != "user_123" {
		t.Fatalf("expected user_123, got %q", got)
	}
	if got := getString(map[string]interface{}{"created_by": 123}, "created_by"); got != "" {
		t.Fatalf("expected empty string for non-string value, got %q", got)
	}
	if got := getString(map[string]interface{}{}, "created_by"); got != "" {
		t.Fatalf("expected empty string for missing key, got %q", got)
	}
}

func setupWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	schema := []string{
		`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			first_name TEXT,
			last_name TEXT,
			avatar_url TEXT,
			organization_id TEXT,
			department_id TEXT,
			role_id TEXT,
			job_title TEXT,
			is_admin BOOLEAN DEFAULT 0,
			preferences TEXT,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME,
			last_sign_in_at DATETIME
		)
		`,
		`
		CREATE TABLE organizations (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT UNIQUE,
			image_url TEXT,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME
		)
		`,
		`
		CREATE TABLE organization_settings (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			organization_id TEXT NOT NULL UNIQUE,
			domain TEXT,
			industry TEXT,
			size TEXT,
			country TEXT,
			use_case TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
		`,
		`
		CREATE TABLE departments (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			name TEXT NOT NULL,
			organization_id TEXT NOT NULL,
			description TEXT,
			created_by_user_id TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(name, organization_id)
		)
		`,
	}

	for _, stmt := range schema {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("failed creating test schema: %v", err)
		}
	}

	return db
}

func restoreDB(t *testing.T) {
	t.Helper()
	previousDB := database.DB
	t.Cleanup(func() {
		database.DB = previousDB
	})
}

func addValidSvixHeaders(t *testing.T, req *http.Request, secret string, payload []byte) {
	t.Helper()
	wh, err := svix.NewWebhook(secret)
	if err != nil {
		t.Fatalf("failed to initialize svix webhook signer: %v", err)
	}

	msgID := "msg_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	msgTimestamp := time.Now()
	signature, err := wh.Sign(msgID, msgTimestamp, payload)
	if err != nil {
		t.Fatalf("failed to sign payload: %v", err)
	}

	req.Header.Set("svix-id", msgID)
	req.Header.Set("svix-timestamp", strconv.FormatInt(msgTimestamp.Unix(), 10))
	req.Header.Set("svix-signature", signature)
}

func testWebhookSecret() string {
	return "whsec_" + base64.StdEncoding.EncodeToString([]byte("test-secret-key"))
}

func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal json: %v", err)
	}
	return out
}
