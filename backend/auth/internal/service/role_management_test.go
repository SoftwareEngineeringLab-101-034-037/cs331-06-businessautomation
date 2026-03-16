package service

import (
	"testing"
	"time"
)

func TestUpdateRole_UserRoleMembershipsWithoutLegacyTable(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := NewEmployeeService(db, "")
	now := time.Now()

	if err := db.Exec(`
		INSERT INTO roles (id, name, description, organization_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "role_1", "IT", "IT team", "org_1", now, now).Error; err != nil {
		t.Fatalf("failed seeding role: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, is_active, created_at, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"user_1", "u1@example.com", "U", "One", "org_1", true, now, now,
		"user_2", "u2@example.com", "U", "Two", "org_1", true, now, now,
		"user_3", "u3@example.com", "U", "Three", "org_1", true, now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding users: %v", err)
	}

	// Seed initial memberships: user_1 and user_3 assigned to role_1
	if err := db.Exec(`
		INSERT INTO user_role_memberships (id, organization_id, user_id, role_id, created_at, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?)
	`,
		"m_1", "org_1", "user_1", "role_1", now, now,
		"m_3", "org_1", "user_3", "role_1", now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding memberships: %v", err)
	}

	updated, err := svc.UpdateRole("org_1", "role_1", "IT Ops", "Ops team", "", []string{"user_1", "user_2"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != "IT Ops" {
		t.Fatalf("expected updated role name IT Ops, got %q", updated.Name)
	}

	type membership struct {
		UserID string `gorm:"column:user_id"`
	}
	var members []membership
	if err := db.Table("user_role_memberships").Select("user_id").Where("organization_id = ? AND role_id = ?", "org_1", "role_1").Order("user_id asc").Find(&members).Error; err != nil {
		t.Fatalf("failed reading memberships: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 memberships after diff update, got %d", len(members))
	}
	if members[0].UserID != "user_1" {
		t.Fatalf("expected user_1 in role, got %q", members[0].UserID)
	}
	if members[1].UserID != "user_2" {
		t.Fatalf("expected user_2 in role, got %q", members[1].UserID)
	}
}

func TestListRoleSummaries_UserRoleMemberships(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := NewEmployeeService(db, "")
	now := time.Now()

	if err := db.Exec(`
		INSERT INTO departments (id, name, organization_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dept_1", "Engineering", "org_1", "", now, now).Error; err != nil {
		t.Fatalf("failed seeding department: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO roles (id, name, description, organization_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?)
	`,
		"role_1", "IT", "", "org_1", now, now,
		"role_2", "Finance", "", "org_1", now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding roles: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, department_id, job_title, is_active, created_at, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"user_1", "u1@example.com", "A", "One", "org_1", "dept_1", "Engineer", true, now, now,
		"user_2", "u2@example.com", "B", "Two", "org_1", "dept_1", "Engineer", true, now, now,
		"user_3", "u3@example.com", "C", "Three", "org_1", "dept_1", "Analyst", true, now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding users: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO user_role_memberships (id, organization_id, user_id, role_id, created_at, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?)
	`,
		"m_1", "org_1", "user_1", "role_1", now, now,
		"m_2", "org_1", "user_2", "role_1", now, now,
		"m_3", "org_1", "user_3", "role_2", now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding memberships: %v", err)
	}

	summaries, err := svc.ListRoleSummaries("org_1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 role summaries, got %d", len(summaries))
	}

	if summaries[0].Name != "Finance" || summaries[0].MemberCount != 1 {
		t.Fatalf("unexpected first summary: %+v", summaries[0])
	}
	if summaries[1].Name != "IT" || summaries[1].MemberCount != 2 {
		t.Fatalf("unexpected second summary: %+v", summaries[1])
	}
}

func TestUpdateRole_UserRoleMembershipsWithLegacyTable(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := NewEmployeeService(db, "")
	now := time.Now()

	if err := db.Exec(`
		INSERT INTO roles (id, name, description, organization_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "role_1", "IT", "", "org_1", now, now).Error; err != nil {
		t.Fatalf("failed seeding role: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO users (id, email, first_name, last_name, organization_id, is_active, created_at, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"user_1", "u1@example.com", "U", "One", "org_1", true, now, now,
		"user_2", "u2@example.com", "U", "Two", "org_1", true, now, now,
	).Error; err != nil {
		t.Fatalf("failed seeding users: %v", err)
	}

	if err := db.Exec(`
		INSERT INTO user_role_memberships (id, organization_id, user_id, role_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "m_1", "org_1", "user_1", "role_1", now, now).Error; err != nil {
		t.Fatalf("failed seeding membership: %v", err)
	}

	if _, err := svc.UpdateRole("org_1", "role_1", "IT Ops", "", "admin_1", []string{"user_2"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	if err := db.Table("user_role_memberships").Where("organization_id = ? AND role_id = ?", "org_1", "role_1").Count(&count).Error; err != nil {
		t.Fatalf("failed counting memberships: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 membership after reset/reassign, got %d", count)
	}

	type membership struct {
		UserID string `gorm:"column:user_id"`
	}
	var m membership
	if err := db.Table("user_role_memberships").Select("user_id").Where("organization_id = ? AND role_id = ?", "org_1", "role_1").Take(&m).Error; err != nil {
		t.Fatalf("failed reading remaining membership: %v", err)
	}
	if m.UserID != "user_2" {
		t.Fatalf("expected user_2 to be assigned, got %q", m.UserID)
	}
}
