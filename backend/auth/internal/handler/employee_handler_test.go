package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCreateDepartmentBadRequestWhenNameMissing(t *testing.T) {
	h, _ := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/departments", strings.NewReader(`{"description":"team"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestCreateDepartmentCreatedThenConflict(t *testing.T) {
	h, _ := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	body := `{"name":"Engineering","description":"Core team"}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/departments", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body=%s", w1.Code, w1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/departments", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d; body=%s", w2.Code, w2.Body.String())
	}
}

func TestCreateDepartmentAttributesCreatedByUserID(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	body := `{"name":"Finance","description":"Finance team"}`
	req := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/departments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed unmarshalling response: %v", err)
	}
	if resp["created_by_user_id"] != "user_admin" {
		t.Fatalf("expected created_by_user_id user_admin, got %v", resp["created_by_user_id"])
	}

	var stored struct {
		CreatedByUserID *string `gorm:"column:created_by_user_id"`
	}
	if err := db.Table("departments").Select("created_by_user_id").Where("name = ? AND organization_id = ?", "Finance", "org_1").Take(&stored).Error; err != nil {
		t.Fatalf("failed reading stored department: %v", err)
	}
	if stored.CreatedByUserID == nil || *stored.CreatedByUserID != "user_admin" {
		t.Fatalf("expected stored created_by_user_id user_admin, got %v", stored.CreatedByUserID)
	}

	// Duplicate should still return conflict
	req2 := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/departments", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-User-ID", "user_admin")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d; body=%s", w2.Code, w2.Body.String())
	}
}

func TestListDepartmentsReturnsOrgDepartments(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?)
	`,
		"dept_1", "Engineering", "org_1", "Eng", now, now,
		"dept_2", "Support", "org_2", "Support", now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding departments: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/org_1/departments", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var got []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed unmarshalling response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 department for org_1, got %d", len(got))
	}
}

func TestGetOrganizationSettingsReturnsExistingValues(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO organization_settings (id, organization_id, domain, industry, size, country, use_case, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "settings_1", "org_1", "acme.com", "Technology", "51-200", "Bangladesh", "Workflow automation", now, now).Error; err != nil {
		t.Fatalf("failed seeding organization settings: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/org_1/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var got map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed unmarshalling response: %v", err)
	}
	if got["domain"] != "acme.com" {
		t.Fatalf("expected domain acme.com, got %v", got["domain"])
	}
	if got["industry"] != "Technology" {
		t.Fatalf("expected industry Technology, got %v", got["industry"])
	}
}

func TestUpdateOrganizationSettingsUpsertsAndPersists(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	body := `{"domain":" example.org ","industry":"Finance","size":"11-50","country":"Bangladesh","use_case":"Approvals"}`
	req := httptest.NewRequest(http.MethodPut, "/api/orgs/org_1/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var stored struct {
		Domain   string `gorm:"column:domain"`
		Industry string `gorm:"column:industry"`
		Size     string `gorm:"column:size"`
		Country  string `gorm:"column:country"`
		UseCase  string `gorm:"column:use_case"`
	}
	if err := db.Table("organization_settings").
		Select("domain, industry, size, country, use_case").
		Where("organization_id = ?", "org_1").
		Take(&stored).Error; err != nil {
		t.Fatalf("failed reading stored settings: %v", err)
	}

	if stored.Domain != "example.org" {
		t.Fatalf("expected trimmed domain example.org, got %q", stored.Domain)
	}
	if stored.Industry != "Finance" || stored.Size != "11-50" || stored.Country != "Bangladesh" || stored.UseCase != "Approvals" {
		t.Fatalf("unexpected stored settings: %+v", stored)
	}
}

func TestInviteSingleBadRequestOnInvalidBody(t *testing.T) {
	h, _ := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/employees/invite", strings.NewReader(`{"email":"not-an-email"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestInviteSingleCreatedThenDuplicateConflict(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "Eng", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	body := `{"email":"new.user@example.com","first_name":"New","last_name":"User","department":"Engineering","role":"member","job_title":"Analyst"}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/employees/invite", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-User-ID", "user_admin")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body=%s", w1.Code, w1.Body.String())
	}

	var inviteCount int64
	if err := db.Table("employee_invitations").Where("email = ?", "new.user@example.com").Count(&inviteCount).Error; err != nil {
		t.Fatalf("failed counting invitations: %v", err)
	}
	if inviteCount != 1 {
		t.Fatalf("expected 1 invitation row, got %d", inviteCount)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/employees/invite", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-User-ID", "user_admin")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d; body=%s", w2.Code, w2.Body.String())
	}
}

func TestInviteSingleBadRequestWhenDepartmentMissing(t *testing.T) {
	h, _ := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	body := `{"email":"person@example.com","first_name":"A","last_name":"B","department":"Unknown"}`
	req := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/employees/invite", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestListInvitationsReturnsOnlyPending(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "Eng", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO employee_invitations (
			id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
			invited_by, expires_at, created_at, updated_at
		) VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"inv_1", "org_1", "dept_1", "pending@example.com", "Pen", "Ding", "member", "Analyst", "tok_pending", "pending", "admin", now.Add(24*time.Hour), now, now,
		"inv_2", "org_1", "dept_1", "revoked@example.com", "Re", "Voked", "member", "Analyst", "tok_revoked", "revoked", "admin", now.Add(24*time.Hour), now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding invitations: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/org_1/invitations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var got []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed unmarshalling response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 pending invitation, got %d", len(got))
	}
}

func TestRevokeInvitationSuccessThenNotFound(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "Eng", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO employee_invitations (
			id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
			invited_by, expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"inv_1", "org_1", "dept_1", "person@example.com", "Per", "Son", "member", "Analyst", "tok_1", "pending", "admin", now.Add(24*time.Hour), now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding invitation: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodDelete, "/api/orgs/org_1/invitations/inv_1", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w1.Code, w1.Body.String())
	}

	var status string
	if err := db.Table("employee_invitations").Select("status").Where("id = ?", "inv_1").Scan(&status).Error; err != nil {
		t.Fatalf("failed reading invitation status: %v", err)
	}
	if status != "revoked" {
		t.Fatalf("expected invitation status revoked, got %q", status)
	}

	req2 := httptest.NewRequest(http.MethodDelete, "/api/orgs/org_1/invitations/does_not_exist", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d; body=%s", w2.Code, w2.Body.String())
	}
}

func TestInviteBulkMissingFileReturnsBadRequest(t *testing.T) {
	h, _ := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/employees/invite/bulk", nil)
	req.Header.Set("X-User-ID", "user_admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestInviteBulkInvalidExcelReturnsBadRequest(t *testing.T) {
	h, _ := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", "employees.xlsx")
	if err != nil {
		t.Fatalf("failed creating form file: %v", err)
	}
	if _, err := part.Write([]byte("not-a-real-xlsx")); err != nil {
		t.Fatalf("failed writing form file data: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed closing multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/employees/invite/bulk", &payload)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "user_admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestInviteBulkProcessesRowsWithPartialFailures(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "Eng", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	xlsxBytes := buildBulkInviteWorkbook(t)

	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", "employees.xlsx")
	if err != nil {
		t.Fatalf("failed creating form file: %v", err)
	}
	if _, err := part.Write(xlsxBytes); err != nil {
		t.Fatalf("failed writing xlsx bytes: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed closing multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/orgs/org_1/employees/invite/bulk", &payload)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "user_admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		TotalRows  int `json:"total_rows"`
		Successful int `json:"successful"`
		Failed     int `json:"failed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed unmarshalling bulk response: %v", err)
	}
	if resp.TotalRows != 2 || resp.Successful != 1 || resp.Failed != 1 {
		t.Fatalf("unexpected bulk response values: %+v", resp)
	}
}

func TestListEmployeesReturnsOrgEmployees(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	orgID := "org_1"
	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"user_1", "one@example.com", "One", "User", orgID, true, now, now,
		"user_2", "two@example.com", "Two", "User", "org_2", true, now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding users: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/org_1/employees", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var got []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed unmarshalling employees: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 employee for org_1, got %d", len(got))
	}
}

func TestGetDepartmentSuccessThenNotFound(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "Eng", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodGet, "/api/orgs/org_1/departments/dept_1", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w1.Code, w1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/orgs/org_1/departments/dept_missing", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d; body=%s", w2.Code, w2.Body.String())
	}
}

func TestUpdateDepartmentSuccess(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "Core team", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	body := `{"name":"Platform","description":"Updated team"}`
	req := httptest.NewRequest(http.MethodPut, "/api/orgs/org_1/departments/dept_1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var name string
	if err := db.Table("departments").Select("name").Where("id = ?", "dept_1").Scan(&name).Error; err != nil {
		t.Fatalf("failed reading updated department: %v", err)
	}
	if name != "Platform" {
		t.Fatalf("expected updated department name Platform, got %q", name)
	}
}

func TestUpdateDepartmentConflict(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?)
	`,
		"dept_1", "Engineering", "org_1", "Eng", now, now,
		"dept_2", "Support", "org_1", "Support", now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding departments: %v", err)
	}

	body := `{"name":"Support","description":"Duplicate name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/orgs/org_1/departments/dept_1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestDeleteDepartmentSuccess(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "Eng", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/org_1/departments/dept_1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var count int64
	if err := db.Table("departments").Where("id = ?", "dept_1").Count(&count).Error; err != nil {
		t.Fatalf("failed counting departments: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected department to be deleted, remaining rows=%d", count)
	}
}

func TestDeleteDepartmentNotFound(t *testing.T) {
	h, _ := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/org_1/departments/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestDeleteDepartmentWithAssignedEmployeeReturnsBadRequest(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	now := time.Now()
	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "Eng", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, department_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_1", "one@example.com", "One", "User", "org_1", "dept_1", true, now, now).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/org_1/departments/dept_1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestUpdateRoleSuccess(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)
	now := time.Now()

	if err := db.Exec(`
		INSERT INTO roles (id, name, description, organization_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "role_1", "IT", "", "org_1", now, now).Error; err != nil {
		t.Fatalf("failed seeding role: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_1", "one@example.com", "One", "User", "org_1", true, now, now).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}

	body := `{"name":"IT Ops","description":"Updated","member_ids":["user_1"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/orgs/org_1/roles/role_1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var roleName string
	if err := db.Table("roles").Select("name").Where("id = ?", "role_1").Scan(&roleName).Error; err != nil {
		t.Fatalf("failed reading role name: %v", err)
	}
	if roleName != "IT Ops" {
		t.Fatalf("expected role name IT Ops, got %q", roleName)
	}
}

func TestUpdateRolePreservesNameWhenOmitted(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)
	now := time.Now()

	if err := db.Exec(`
		INSERT INTO roles (id, name, description, organization_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "role_1", "IT", "Original", "org_1", now, now).Error; err != nil {
		t.Fatalf("failed seeding role: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_1", "one@example.com", "One", "User", "org_1", true, now, now).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}

	body := `{"description":"Updated","member_ids":["user_1"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/orgs/org_1/roles/role_1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var stored struct {
		Name        string
		Description string
	}
	if err := db.Table("roles").Select("name, description").Where("id = ?", "role_1").Take(&stored).Error; err != nil {
		t.Fatalf("failed reading role row: %v", err)
	}
	if stored.Name != "IT" {
		t.Fatalf("expected role name to remain IT, got %q", stored.Name)
	}
	if stored.Description != "Updated" {
		t.Fatalf("expected description Updated, got %q", stored.Description)
	}

	var membershipCount int64
	if err := db.Table("user_role_memberships").Where("organization_id = ? AND role_id = ? AND user_id = ?", "org_1", "role_1", "user_1").Count(&membershipCount).Error; err != nil {
		t.Fatalf("failed counting memberships: %v", err)
	}
	if membershipCount != 1 {
		t.Fatalf("expected user_1 membership for role_1, got %d", membershipCount)
	}
}

func TestUpdateRoleNotFound(t *testing.T) {
	h, _ := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	body := `{"name":"IT Ops","description":"Updated","member_ids":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/orgs/org_1/roles/missing", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestDeleteRoleSuccessClearsMemberships(t *testing.T) {
	h, db := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)
	now := time.Now()

	if err := db.Exec(`
		INSERT INTO roles (id, name, description, organization_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "role_1", "IT", "", "org_1", now, now).Error; err != nil {
		t.Fatalf("failed seeding role: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, role_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_1", "one@example.com", "One", "User", "org_1", "role_1", true, now, now).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO user_role_memberships (id, organization_id, user_id, role_id, assigned_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "m_1", "org_1", "user_1", "role_1", "user_admin", now, now).Error; err != nil {
		t.Fatalf("failed seeding role membership: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/org_1/roles/role_1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var roleCount int64
	if err := db.Table("roles").Where("id = ?", "role_1").Count(&roleCount).Error; err != nil {
		t.Fatalf("failed counting roles: %v", err)
	}
	if roleCount != 0 {
		t.Fatalf("expected role to be deleted, remaining rows=%d", roleCount)
	}

	var membershipCount int64
	if err := db.Table("user_role_memberships").Where("role_id = ?", "role_1").Count(&membershipCount).Error; err != nil {
		t.Fatalf("failed counting memberships: %v", err)
	}
	if membershipCount != 0 {
		t.Fatalf("expected memberships to be deleted, remaining rows=%d", membershipCount)
	}

}

func TestDeleteRoleNotFound(t *testing.T) {
	h, _ := newEmployeeHandlerForTest(t)
	r := newEmployeeTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/org_1/roles/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestDeleteEmployee(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h, db := newEmployeeHandlerForTestWithClerkSecret(t, "test_clerk_secret")
		r := newEmployeeTestRouter(h)

		prevDeleteFn := service.ClerkDeleteMembershipFunc
		service.ClerkDeleteMembershipFunc = func(_ string, _ string, _ string) error { return nil }
		defer func() { service.ClerkDeleteMembershipFunc = prevDeleteFn }()

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO users (id, email, first_name, last_name, organization_id, is_admin, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "user_member", "member@example.com", "Mem", "Ber", "org_1", false, true, now, now).Error; err != nil {
			t.Fatalf("failed seeding user: %v", err)
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/orgs/org_1/employees/user_member", nil)
		req.Header.Set("X-User-ID", "admin_actor")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("admin target rejected", func(t *testing.T) {
		h, db := newEmployeeHandlerForTest(t)
		r := newEmployeeTestRouter(h)

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO users (id, email, first_name, last_name, organization_id, is_admin, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "user_admin", "admin@example.com", "Ad", "Min", "org_1", true, true, now, now).Error; err != nil {
			t.Fatalf("failed seeding user: %v", err)
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/orgs/org_1/employees/user_admin", nil)
		req.Header.Set("X-User-ID", "admin_actor")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d; body=%s", w.Code, w.Body.String())
		}
	})
}

func newEmployeeHandlerForTest(t *testing.T) (*EmployeeHandler, *gorm.DB) {
	return newEmployeeHandlerForTestWithClerkSecret(t, "")
}

func newEmployeeHandlerForTestWithClerkSecret(t *testing.T, clerkSecret string) (*EmployeeHandler, *gorm.DB) {
	t.Helper()
	db := setupEmployeeHandlerTestDB(t)
	prevRevokeFn := service.ClerkRevokeOrgInvitationsByEmailFunc
	service.ClerkRevokeOrgInvitationsByEmailFunc = func(_ string, _ string, _ string) error { return nil }
	t.Cleanup(func() { service.ClerkRevokeOrgInvitationsByEmailFunc = prevRevokeFn })
	svc := service.NewEmployeeService(db, clerkSecret)
	return NewEmployeeHandler(svc), db
}

func newEmployeeTestRouter(h *EmployeeHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if userID := c.GetHeader("X-User-ID"); userID != "" {
			c.Set("user_id", userID)
		}
		c.Next()
	})

	r.GET("/api/orgs/:orgId/settings", h.GetOrganizationSettings)
	r.PUT("/api/orgs/:orgId/settings", h.UpdateOrganizationSettings)
	r.POST("/api/orgs/:orgId/departments", h.CreateDepartment)
	r.GET("/api/orgs/:orgId/departments", h.ListDepartments)
	r.PUT("/api/orgs/:orgId/departments/:deptID", h.UpdateDepartment)
	r.DELETE("/api/orgs/:orgId/departments/:deptID", h.DeleteDepartment)
	r.POST("/api/orgs/:orgId/employees/invite", h.InviteSingle)
	r.GET("/api/orgs/:orgId/invitations", h.ListInvitations)
	r.DELETE("/api/orgs/:orgId/invitations/:invitationId", h.RevokeInvitation)
	r.POST("/api/orgs/:orgId/employees/invite/bulk", h.InviteBulk)
	r.GET("/api/orgs/:orgId/employees", h.ListEmployees)
	r.DELETE("/api/orgs/:orgId/employees/:employeeId", h.DeleteEmployee)
	r.POST("/api/orgs/:orgId/roles", h.CreateRole)
	r.GET("/api/orgs/:orgId/roles", h.ListRoles)
	r.PUT("/api/orgs/:orgId/roles/:roleID", h.UpdateRole)
	r.DELETE("/api/orgs/:orgId/roles/:roleID", h.DeleteRole)
	r.GET("/api/orgs/:orgId/departments/:deptID", h.GetDepartment)
	r.POST("/api/orgs/:orgId/invitations/:invitationId/accept", h.AcceptInvitation)
	return r
}

func setupEmployeeHandlerTestDB(t *testing.T) *gorm.DB {
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
		CREATE TABLE organization_settings (
			id TEXT PRIMARY KEY,
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
		`
		CREATE TABLE roles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			organization_id TEXT,
			created_by_user_id TEXT,
			permissions TEXT,
			is_system_role BOOLEAN DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)
		`,
		`
		CREATE TABLE user_role_memberships (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role_id TEXT NOT NULL,
			assigned_by TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(organization_id, user_id, role_id)
		)
		`,
		`
		CREATE TABLE employee_invitations (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			organization_id TEXT NOT NULL,
			department_id TEXT NOT NULL,
			email TEXT NOT NULL,
			first_name TEXT,
			last_name TEXT,
			role_name TEXT,
			role_names TEXT,
			job_title TEXT,
			token TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL DEFAULT 'pending',
			invited_by TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			accepted_at DATETIME,
			accepted_user_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
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

func buildBulkInviteWorkbook(t *testing.T) []byte {
	t.Helper()

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	headers := []string{"email", "first_name", "last_name", "department", "role", "job_title"}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			t.Fatalf("failed creating header cell name: %v", err)
		}
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			t.Fatalf("failed setting header cell: %v", err)
		}
	}

	// First row should succeed (department exists), second should fail (department missing).
	row1 := []string{"ok@example.com", "Ok", "User", "Engineering", "member", "Engineer"}
	row2 := []string{"fail@example.com", "Fail", "User", "MissingDept", "member", "Engineer"}
	rows := [][]string{row1, row2}
	for r, row := range rows {
		for c, v := range row {
			cell, err := excelize.CoordinatesToCellName(c+1, r+2)
			if err != nil {
				t.Fatalf("failed creating row cell name: %v", err)
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				t.Fatalf("failed setting row cell: %v", err)
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("failed writing workbook: %v", err)
	}
	return buf.Bytes()
}
