package service

import (
	"errors"
	"testing"
	"time"
)

func TestAssignment9Defect_AcceptInvitationEmailCaseMismatch(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := NewEmployeeService(db, "")
	now := time.Now()

	if err := db.Exec(`
		INSERT INTO users (
			id, email, first_name, last_name, organization_id,
			is_admin, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "user_case", "User.Case@Example.com", "User", "Case", "org_1", false, true, now, now).Error; err != nil {
		t.Fatalf("failed seeding user: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO employee_invitations (
			id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
			invited_by, expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"inv_case", "org_1", "dept_1", "user.case@example.com", "User", "Case", "", "", "tok_case", "pending", "admin", now.Add(24*time.Hour), now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding invitation: %v", err)
	}

	if err := svc.AcceptInvitationByID("inv_case", "org_1", "user_case"); err != nil {
		t.Fatalf("expected acceptance to be case-insensitive for email; got %v", err)
	}
}

func TestAssignment9Defect_ResolveDepartmentIDTrimmedInput(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := NewEmployeeService(db, "")
	now := time.Now()

	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "Core team", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	got, err := svc.resolveDepartmentID("org_1", " Engineering ")
	if err != nil {
		t.Fatalf("expected department lookup to trim input; got %v", err)
	}
	if got != "dept_1" {
		t.Fatalf("expected department dept_1, got %q", got)
	}
}

func TestAssignment9Defect_DuplicateInviteEmailCaseInsensitive(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := NewEmployeeService(db, "")
	now := time.Now()

	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "Core team", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	if _, err := svc.InviteAndNotify(InviteInput{
		OrgID:        "org_1",
		Email:        "First.User@Example.com",
		FirstName:    "First",
		LastName:     "User",
		DepartmentID: "Engineering",
		InvitedBy:    "admin",
	}); err != nil {
		t.Fatalf("failed creating initial invitation: %v", err)
	}

	_, err := svc.InviteAndNotify(InviteInput{
		OrgID:        "org_1",
		Email:        "first.user@example.com",
		FirstName:    "First",
		LastName:     "User",
		DepartmentID: "Engineering",
		InvitedBy:    "admin",
	})
	if err == nil {
		t.Fatal("expected duplicate invitation blocked for case-variant email")
	}
	if !errors.Is(err, ErrDuplicateInvite) {
		t.Fatalf("expected ErrDuplicateInvite for case-variant email, got %v", err)
	}
}
