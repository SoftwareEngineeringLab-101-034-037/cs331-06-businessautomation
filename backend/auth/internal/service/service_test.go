package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestNewEmployeeService(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := NewEmployeeService(db, "secret")
	if svc == nil {
		t.Fatal("expected service instance, got nil")
	}
	if svc.db != db {
		t.Fatal("expected service to hold provided db")
	}
	if svc.clerkSecretKey != "secret" {
		t.Fatalf("expected clerk secret to be set, got %q", svc.clerkSecretKey)
	}
}

func TestCreateDepartment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		dept, err := svc.CreateDepartment("org_1", "Engineering", "Core team", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if dept.Name != "Engineering" || dept.OrganizationID != "org_1" {
			t.Fatalf("unexpected department returned: %+v", dept)
		}

		var count int64
		if err := db.Table("departments").Where("name = ? AND organization_id = ?", "Engineering", "org_1").Count(&count).Error; err != nil {
			t.Fatalf("failed counting departments: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 department row, got %d", count)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, "dept_1", "Engineering", "org_1", "Core", now, now).Error; err != nil {
			t.Fatalf("failed seeding department: %v", err)
		}

		_, err := svc.CreateDepartment("org_1", "Engineering", "Dup", "")
		if err == nil {
			t.Fatal("expected duplicate error")
		}
		if !errors.Is(err, ErrDuplicateDepartment) {
			t.Fatalf("expected ErrDuplicateDepartment, got %v", err)
		}
	})

	t.Run("trim and case-insensitive duplicate", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		created, err := svc.CreateDepartment("org_1", "  Engineering  ", "Core", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.Name != "Engineering" {
			t.Fatalf("expected trimmed department name Engineering, got %q", created.Name)
		}

		_, err = svc.CreateDepartment("org_1", " engineering ", "Dup", "")
		if err == nil {
			t.Fatal("expected duplicate error")
		}
		if !errors.Is(err, ErrDuplicateDepartment) {
			t.Fatalf("expected ErrDuplicateDepartment, got %v", err)
		}
	})

	t.Run("empty name after trim", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		_, err := svc.CreateDepartment("org_1", "   ", "Core", "")
		if err == nil || !strings.Contains(err.Error(), "department name is required") {
			t.Fatalf("expected required department name error, got %v", err)
		}
	})

	t.Run("lookup database error", func(t *testing.T) {
		db := setupEmptyServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		_, err := svc.CreateDepartment("org_1", "Engineering", "Core", "")
		if err == nil || !strings.Contains(err.Error(), "failed to check existing department") {
			t.Fatalf("expected lookup db error, got %v", err)
		}
	})
}

func TestListDepartments(t *testing.T) {
	t.Run("success filtered by org", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

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

		depts, err := svc.ListDepartments("org_1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(depts) != 1 || depts[0].ID != "dept_1" {
			t.Fatalf("unexpected departments: %+v", depts)
		}
	})

	t.Run("database error", func(t *testing.T) {
		db := setupEmptyServiceTestDB(t)
		svc := NewEmployeeService(db, "")
		_, err := svc.ListDepartments("org_1")
		if err == nil || !strings.Contains(err.Error(), "failed to list departments") {
			t.Fatalf("expected list error, got %v", err)
		}
	})
}

func TestListEmployees(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO users (id, email, first_name, last_name, organization_id, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"user_1", "one@example.com", "One", "User", "org_1", true, now, now,
			"user_2", "two@example.com", "Two", "User", "org_2", true, now, now,
		).Error; err != nil {
			t.Fatalf("failed seeding users: %v", err)
		}

		users, err := svc.ListEmployees("org_1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(users) != 1 || users[0].ID != "user_1" {
			t.Fatalf("unexpected users: %+v", users)
		}
	})

	t.Run("database error", func(t *testing.T) {
		db := setupEmptyServiceTestDB(t)
		svc := NewEmployeeService(db, "")
		_, err := svc.ListEmployees("org_1")
		if err == nil || !strings.Contains(err.Error(), "failed to list employees") {
			t.Fatalf("expected list employees error, got %v", err)
		}
	})
}

func TestRemoveEmployee(t *testing.T) {
	t.Run("success removes user and related rows", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "test_clerk_secret")

		prevDeleteFn := ClerkDeleteUserFunc
		ClerkDeleteUserFunc = func(_ string, _ string) error { return nil }
		defer func() { ClerkDeleteUserFunc = prevDeleteFn }()

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO users (id, email, first_name, last_name, organization_id, is_admin, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "user_member", "member@example.com", "Mem", "Ber", "org_1", false, true, now, now).Error; err != nil {
			t.Fatalf("failed seeding user: %v", err)
		}
		if err := db.Exec(`
			INSERT INTO roles (id, name, organization_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, "role_1", "reviewer", "org_1", now, now).Error; err != nil {
			t.Fatalf("failed seeding role: %v", err)
		}
		if err := db.Exec(`
			INSERT INTO user_role_memberships (id, organization_id, user_id, role_id, assigned_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, "urm_1", "org_1", "user_member", "role_1", "admin", now, now).Error; err != nil {
			t.Fatalf("failed seeding role membership: %v", err)
		}
		if err := db.Exec(`
			INSERT INTO employee_invitations (
				id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
				invited_by, expires_at, accepted_user_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"inv_1", "org_1", "dept_1", "member@example.com", "Mem", "Ber", "", "", "tok_1", "accepted", "admin", now.Add(24*time.Hour), "user_member", now, now,
		).Error; err != nil {
			t.Fatalf("failed seeding invitation: %v", err)
		}

		if err := svc.RemoveEmployee("org_1", "user_member", "admin_actor"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var userCount, roleMembershipCount, inviteCount int64
		if err := db.Table("users").Where("id = ?", "user_member").Count(&userCount).Error; err != nil {
			t.Fatalf("failed counting users: %v", err)
		}
		if err := db.Table("user_role_memberships").Where("user_id = ?", "user_member").Count(&roleMembershipCount).Error; err != nil {
			t.Fatalf("failed counting role memberships: %v", err)
		}
		if err := db.Table("employee_invitations").Where("email = ? OR accepted_user_id = ?", "member@example.com", "user_member").Count(&inviteCount).Error; err != nil {
			t.Fatalf("failed counting invitations: %v", err)
		}

		if userCount != 0 || roleMembershipCount != 0 || inviteCount != 0 {
			t.Fatalf("expected related auth rows removed, got users=%d roles=%d invites=%d", userCount, roleMembershipCount, inviteCount)
		}
	})

	t.Run("reject removing admin member", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "test_clerk_secret")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO users (id, email, first_name, last_name, organization_id, is_admin, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "user_admin", "admin@example.com", "Ad", "Min", "org_1", true, true, now, now).Error; err != nil {
			t.Fatalf("failed seeding admin user: %v", err)
		}

		err := svc.RemoveEmployee("org_1", "user_admin", "admin_actor")
		if err == nil || !errors.Is(err, ErrCannotRemoveAdmin) {
			t.Fatalf("expected ErrCannotRemoveAdmin, got %v", err)
		}
	})
}

func TestListInvitations(t *testing.T) {
	t.Run("success pending only", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

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
			"inv_1", "org_1", "dept_1", "pending@example.com", "Pen", "Ding", "member", "Analyst", "tok_1", "pending", "admin", now.Add(24*time.Hour), now, now,
			"inv_2", "org_1", "dept_1", "revoked@example.com", "Re", "Voked", "member", "Analyst", "tok_2", "revoked", "admin", now.Add(24*time.Hour), now, now,
		).Error; err != nil {
			t.Fatalf("failed seeding invitations: %v", err)
		}

		invites, err := svc.ListInvitations("org_1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(invites) != 1 || invites[0].ID != "inv_1" {
			t.Fatalf("unexpected invitations: %+v", invites)
		}
	})

	t.Run("database error", func(t *testing.T) {
		db := setupEmptyServiceTestDB(t)
		svc := NewEmployeeService(db, "")
		_, err := svc.ListInvitations("org_1")
		if err == nil || !strings.Contains(err.Error(), "failed to list invitations") {
			t.Fatalf("expected list invitation error, got %v", err)
		}
	})
}

func TestRevokeInvitation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
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

		if err := svc.RevokeInvitation("inv_1", "org_1"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var status string
		if err := db.Table("employee_invitations").Select("status").Where("id = ?", "inv_1").Scan(&status).Error; err != nil {
			t.Fatalf("failed reading invitation status: %v", err)
		}
		if status != "revoked" {
			t.Fatalf("expected status revoked, got %q", status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")
		err := svc.RevokeInvitation("missing", "org_1")
		if err == nil || !strings.Contains(err.Error(), "not found or already processed") {
			t.Fatalf("expected not found revoke error, got %v", err)
		}
	})

	t.Run("database error", func(t *testing.T) {
		db := setupEmptyServiceTestDB(t)
		svc := NewEmployeeService(db, "")
		err := svc.RevokeInvitation("inv_1", "org_1")
		if err == nil || !strings.Contains(err.Error(), "failed to revoke invitation") {
			t.Fatalf("expected revoke db error, got %v", err)
		}
	})
}

func TestGetDepartmentDetails(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, "dept_1", "Engineering", "org_1", "Eng", now, now).Error; err != nil {
			t.Fatalf("failed seeding department: %v", err)
		}

		dept, err := svc.GetDepartmentDetails("org_1", "dept_1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if dept.ID != "dept_1" {
			t.Fatalf("unexpected department: %+v", dept)
		}
	})

	t.Run("not found", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		_, err := svc.GetDepartmentDetails("org_1", "missing")
		if err == nil {
			t.Fatal("expected not found error")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("database error", func(t *testing.T) {
		db := setupEmptyServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		_, err := svc.GetDepartmentDetails("org_1", "dept_1")
		if err == nil || !strings.Contains(err.Error(), "failed to get department details") {
			t.Fatalf("expected db error, got %v", err)
		}
	})
}

func TestInviteAndNotify(t *testing.T) {
	t.Run("duplicate pending invite", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO employee_invitations (
				id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
				invited_by, expires_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"inv_1", "org_1", "dept_1", "dup@example.com", "Du", "P", "member", "Analyst", "tok_dup", "pending", "admin", now.Add(24*time.Hour), now, now,
		).Error; err != nil {
			t.Fatalf("failed seeding invitation: %v", err)
		}

		_, err := svc.InviteAndNotify(InviteInput{
			OrgID:        "org_1",
			Email:        "dup@example.com",
			FirstName:    "Du",
			LastName:     "P",
			DepartmentID: "dept_1",
			InvitedBy:    "admin",
		})
		if err == nil {
			t.Fatal("expected duplicate invite error")
		}
		if !errors.Is(err, ErrDuplicateInvite) {
			t.Fatalf("expected ErrDuplicateInvite, got %v", err)
		}
	})

	t.Run("lookup database error", func(t *testing.T) {
		db := setupEmptyServiceTestDB(t)
		svc := NewEmployeeService(db, "")
		_, err := svc.InviteAndNotify(InviteInput{
			OrgID:        "org_1",
			Email:        "a@example.com",
			FirstName:    "A",
			LastName:     "B",
			DepartmentID: "dept_1",
			InvitedBy:    "admin",
		})
		if err == nil || !strings.Contains(err.Error(), "database lookup failed") {
			t.Fatalf("expected database lookup error, got %v", err)
		}
	})

	t.Run("department lookup failed", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		_, err := svc.InviteAndNotify(InviteInput{
			OrgID:        "org_1",
			Email:        "a@example.com",
			FirstName:    "A",
			LastName:     "B",
			DepartmentID: "missing",
			InvitedBy:    "admin",
		})
		if err == nil || !strings.Contains(err.Error(), "department lookup failed") {
			t.Fatalf("expected department lookup error, got %v", err)
		}
	})

	t.Run("success creates invitation and tolerates clerk send error", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, "dept_1", "Engineering", "org_1", "Eng", now, now).Error; err != nil {
			t.Fatalf("failed seeding department: %v", err)
		}

		res, err := svc.InviteAndNotify(InviteInput{
			OrgID:        "org_1",
			Email:        "ok@example.com",
			FirstName:    "Ok",
			LastName:     "User",
			DepartmentID: "Engineering",
			Role:         "manager",
			JobTitle:     "Analyst",
			InvitedBy:    "admin",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res == nil || res.Invitation.Email != "ok@example.com" || res.Invitation.Status != "pending" {
			t.Fatalf("unexpected invite result: %+v", res)
		}
		if res.Invitation.Token == "" {
			t.Fatal("expected generated invite token hash")
		}

		var count int64
		if err := db.Table("employee_invitations").Where("email = ?", "ok@example.com").Count(&count).Error; err != nil {
			t.Fatalf("failed counting invitations: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 invitation row, got %d", count)
		}
	})
}

func TestSendClerkOrgInvitationWithoutSecret(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := NewEmployeeService(db, "")
	err := svc.sendClerkOrgInvitation("org_1", "user@example.com")
	if err == nil || !strings.Contains(err.Error(), "clerk secret key not configured") {
		t.Fatalf("expected missing secret key error, got %v", err)
	}
}

func TestAcceptInvitationByEmail(t *testing.T) {
	t.Run("no pending invitation", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")
		err := svc.AcceptInvitationByEmail("missing@example.com", "org_1", "user_1")
		if err == nil || !strings.Contains(err.Error(), "no pending invitation found") {
			t.Fatalf("expected no-pending error, got %v", err)
		}
	})

	t.Run("expired invitation marked expired", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO employee_invitations (
				id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
				invited_by, expires_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"inv_exp", "org_1", "dept_1", "expired@example.com", "Ex", "Pired", "member", "Analyst", "tok_exp", "pending", "admin", now.Add(-1*time.Hour), now, now,
		).Error; err != nil {
			t.Fatalf("failed seeding expired invite: %v", err)
		}

		err := svc.AcceptInvitationByEmail("expired@example.com", "org_1", "user_1")
		if err == nil || !strings.Contains(err.Error(), "has expired") {
			t.Fatalf("expected expired error, got %v", err)
		}

		var status string
		if err := db.Table("employee_invitations").Select("status").Where("id = ?", "inv_exp").Scan(&status).Error; err != nil {
			t.Fatalf("failed reading invite status: %v", err)
		}
		if status != "expired" {
			t.Fatalf("expected status expired, got %q", status)
		}
	})

	t.Run("success with role and job title", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO users (id, email, first_name, last_name, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, "user_1", "user1@example.com", "Old", "Name", true, now, now).Error; err != nil {
			t.Fatalf("failed seeding user: %v", err)
		}
		if err := db.Exec(`
			INSERT INTO roles (id, name, organization_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, "role_1", "manager", "org_1", now, now).Error; err != nil {
			t.Fatalf("failed seeding role: %v", err)
		}
		if err := db.Exec(`
			INSERT INTO employee_invitations (
				id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
				invited_by, expires_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"inv_ok", "org_1", "dept_1", "user1@example.com", "User", "One", "manager", "Senior Analyst", "tok_ok", "pending", "admin", now.Add(24*time.Hour), now, now,
		).Error; err != nil {
			t.Fatalf("failed seeding invitation: %v", err)
		}

		if err := svc.AcceptInvitationByEmail("user1@example.com", "org_1", "user_1"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var invite struct {
			Status         string
			AcceptedUserID *string `gorm:"column:accepted_user_id"`
		}
		if err := db.Table("employee_invitations").
			Select("status, accepted_user_id").
			Where("id = ?", "inv_ok").
			Take(&invite).Error; err != nil {
			t.Fatalf("failed reading invitation: %v", err)
		}
		if invite.Status != "accepted" || invite.AcceptedUserID == nil || *invite.AcceptedUserID != "user_1" {
			t.Fatalf("unexpected invitation values: %+v", invite)
		}

		var user struct {
			OrganizationID *string `gorm:"column:organization_id"`
			DepartmentID   *string `gorm:"column:department_id"`
			FirstName      string  `gorm:"column:first_name"`
			LastName       string  `gorm:"column:last_name"`
			JobTitle       string  `gorm:"column:job_title"`
			IsAdmin        bool    `gorm:"column:is_admin"`
		}
		if err := db.Table("users").
			Select("organization_id, department_id, first_name, last_name, job_title, is_admin").
			Where("id = ?", "user_1").
			Take(&user).Error; err != nil {
			t.Fatalf("failed reading user: %v", err)
		}
		if user.OrganizationID == nil || *user.OrganizationID != "org_1" {
			t.Fatalf("expected organization_id org_1, got %+v", user.OrganizationID)
		}
		if user.DepartmentID == nil || *user.DepartmentID != "dept_1" {
			t.Fatalf("expected department_id dept_1, got %+v", user.DepartmentID)
		}
		if user.FirstName != "User" || user.LastName != "One" {
			t.Fatalf("expected name from invitation User One, got %q %q", user.FirstName, user.LastName)
		}
		if user.JobTitle != "Senior Analyst" {
			t.Fatalf("expected job title update, got %q", user.JobTitle)
		}
		if user.IsAdmin {
			t.Fatal("expected invitation acceptance default dashboard access to member (is_admin=false)")
		}
		var membershipCount int64
		if err := db.Table("user_role_memberships").Where("user_id = ? AND role_id = ?", "user_1", "role_1").Count(&membershipCount).Error; err != nil {
			t.Fatalf("failed counting memberships: %v", err)
		}
		if membershipCount != 1 {
			t.Fatalf("expected 1 role membership for user_1/role_1, got %d", membershipCount)
		}
	})

	t.Run("dashboard access follows clerk admin membership", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO users (id, email, first_name, last_name, is_admin, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, "user_admin", "admin.invited@example.com", "Admin", "Invitee", true, true, now, now).Error; err != nil {
			t.Fatalf("failed seeding user: %v", err)
		}
		if err := db.Exec(`
			INSERT INTO employee_invitations (
				id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
				invited_by, expires_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"inv_admin", "org_1", "dept_admin", "admin.invited@example.com", "Admin", "Invitee", "", "Head Ops", "tok_admin", "pending", "admin", now.Add(24*time.Hour), now, now,
		).Error; err != nil {
			t.Fatalf("failed seeding invitation: %v", err)
		}

		if err := svc.AcceptInvitationByEmail("admin.invited@example.com", "org_1", "user_admin"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var user struct {
			OrganizationID *string `gorm:"column:organization_id"`
			DepartmentID   *string `gorm:"column:department_id"`
			JobTitle       string  `gorm:"column:job_title"`
			IsAdmin        bool    `gorm:"column:is_admin"`
		}
		if err := db.Table("users").
			Select("organization_id, department_id, job_title, is_admin").
			Where("id = ?", "user_admin").
			Take(&user).Error; err != nil {
			t.Fatalf("failed reading user: %v", err)
		}
		if user.OrganizationID == nil || *user.OrganizationID != "org_1" {
			t.Fatalf("expected organization_id org_1, got %+v", user.OrganizationID)
		}
		if user.DepartmentID == nil || *user.DepartmentID != "dept_admin" {
			t.Fatalf("expected department_id dept_admin, got %+v", user.DepartmentID)
		}
		if user.JobTitle != "Head Ops" {
			t.Fatalf("expected job_title Head Ops, got %q", user.JobTitle)
		}
		if !user.IsAdmin {
			t.Fatal("expected is_admin=true to be preserved for admin user")
		}
	})

	t.Run("unknown invited role returns error", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO users (id, email, first_name, last_name, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, "user_2", "user2@example.com", "User", "Two", true, now, now).Error; err != nil {
			t.Fatalf("failed seeding user: %v", err)
		}
		if err := db.Exec(`
			INSERT INTO employee_invitations (
				id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
				invited_by, expires_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"inv_no_role", "org_1", "dept_1", "user2@example.com", "User", "Two", "missing-role", "", "tok_no_role", "pending", "admin", now.Add(24*time.Hour), now, now,
		).Error; err != nil {
			t.Fatalf("failed seeding invitation: %v", err)
		}

		err := svc.AcceptInvitationByEmail("user2@example.com", "org_1", "user_2")
		if err == nil || !strings.Contains(err.Error(), "unknown role names") {
			t.Fatalf("expected unknown role names error, got %v", err)
		}

		var membershipCount int64
		if err := db.Table("user_role_memberships").Where("user_id = ?", "user_2").Count(&membershipCount).Error; err != nil {
			t.Fatalf("failed counting memberships: %v", err)
		}
		if membershipCount != 0 {
			t.Fatalf("expected no role membership for user_2, got %d", membershipCount)
		}

		var inviteStatus string
		if err := db.Table("employee_invitations").Select("status").Where("id = ?", "inv_no_role").Scan(&inviteStatus).Error; err != nil {
			t.Fatalf("failed reading invitation status: %v", err)
		}
		if inviteStatus != "pending" {
			t.Fatalf("expected invitation status pending after rollback, got %q", inviteStatus)
		}
	})

	t.Run("user not found rolls back invitation update", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO employee_invitations (
				id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
				invited_by, expires_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"inv_missing_user", "org_1", "dept_1", "nouser@example.com", "No", "User", "member", "", "tok_missing_user", "pending", "admin", now.Add(24*time.Hour), now, now,
		).Error; err != nil {
			t.Fatalf("failed seeding invitation: %v", err)
		}

		err := svc.AcceptInvitationByEmail("nouser@example.com", "org_1", "missing_user")
		if err == nil || !strings.Contains(err.Error(), "rolling back invitation acceptance") {
			t.Fatalf("expected user missing rollback error, got %v", err)
		}

		var status string
		if err := db.Table("employee_invitations").Select("status").Where("id = ?", "inv_missing_user").Scan(&status).Error; err != nil {
			t.Fatalf("failed reading invitation status: %v", err)
		}
		if status != "pending" {
			t.Fatalf("expected invitation status to stay pending after rollback, got %q", status)
		}
	})

	t.Run("user update database error", func(t *testing.T) {
		db := setupServiceDBWithoutUsersTable(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO employee_invitations (
				id, organization_id, department_id, email, first_name, last_name, role_name, job_title, token, status,
				invited_by, expires_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"inv_user_tbl_missing", "org_1", "dept_1", "u@example.com", "U", "Ser", "member", "", "tok_u_missing", "pending", "admin", now.Add(24*time.Hour), now, now,
		).Error; err != nil {
			t.Fatalf("failed seeding invitation: %v", err)
		}

		err := svc.AcceptInvitationByEmail("u@example.com", "org_1", "user_1")
		if err == nil || !strings.Contains(err.Error(), "failed to update user after invitation acceptance") {
			t.Fatalf("expected user update db error, got %v", err)
		}
	})
}

func TestResolveDepartmentID(t *testing.T) {
	t.Run("find by name", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, "dept_1", "Engineering", "org_1", "Eng", now, now).Error; err != nil {
			t.Fatalf("failed seeding department: %v", err)
		}

		got, err := svc.resolveDepartmentID("org_1", "Engineering")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got != "dept_1" {
			t.Fatalf("expected dept_1, got %q", got)
		}
	})

	t.Run("fallback to id", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")

		now := time.Now()
		if err := db.Exec(`
			INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, "dept_2", "Support", "org_1", "Support", now, now).Error; err != nil {
			t.Fatalf("failed seeding department: %v", err)
		}

		got, err := svc.resolveDepartmentID("org_1", "dept_2")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got != "dept_2" {
			t.Fatalf("expected dept_2, got %q", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		db := setupServiceTestDB(t)
		svc := NewEmployeeService(db, "")
		_, err := svc.resolveDepartmentID("org_1", "missing")
		if err == nil {
			t.Fatal("expected not found error")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("database error", func(t *testing.T) {
		db := setupEmptyServiceTestDB(t)
		svc := NewEmployeeService(db, "")
		_, err := svc.resolveDepartmentID("org_1", "dept_1")
		if err == nil || !strings.Contains(err.Error(), "DB error looking up department") {
			t.Fatalf("expected db error, got %v", err)
		}
	})
}

func setupServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed opening sqlite db: %v", err)
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
			t.Fatalf("failed creating schema: %v", err)
		}
	}

	return db
}

func setupEmptyServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed opening empty sqlite db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return db
}

func setupServiceDBWithoutUsersTable(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupEmptyServiceTestDB(t)

	schema := []string{
		`
		CREATE TABLE employee_invitations (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			organization_id TEXT NOT NULL,
			department_id TEXT NOT NULL,
			email TEXT NOT NULL,
			first_name TEXT,
			last_name TEXT,
			role_name TEXT,
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
		`
		CREATE TABLE roles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			organization_id TEXT,
			permissions TEXT,
			is_system_role BOOLEAN DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)
		`,
	}
	for _, stmt := range schema {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("failed creating partial schema: %v", err)
		}
	}
	return db
}
