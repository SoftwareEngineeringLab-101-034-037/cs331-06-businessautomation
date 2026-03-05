package models

import (
	"testing"
	"time"

	"gorm.io/datatypes"
)

type tableNamer interface {
	TableName() string
}

func TestTableNames(t *testing.T) {
	testCases := []struct {
		name      string
		model     tableNamer
		tableName string
	}{

		{
			name:      "organization settings",
			model:     OrganizationSettings{},
			tableName: "organization_settings",
		},
		{
			name:      "organizations",
			model:     Organization{},
			tableName: "organizations",
		},
		{
			name:      "roles",
			model:     Role{},
			tableName: "roles",
		},
		{
			name:      "permissions",
			model:     Permission{},
			tableName: "permissions",
		},
		{
			name:      "users",
			model:     User{},
			tableName: "users",
		},
		{
			name:      "departments",
			model:     Department{},
			tableName: "departments",
		},
		{
			name:      "employee invitations",
			model:     EmployeeInvitation{},
			tableName: "employee_invitations",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.model.TableName(); got != tc.tableName {
				t.Fatalf("expected table name %q, got %q", tc.tableName, got)
			}
		})
	}
}

func TestUserFullName(t *testing.T) {
	testCases := []struct {
		name      string
		firstName string
		lastName  string
		want      string
	}{
		{
			name:      "first and last name",
			firstName: "Ada",
			lastName:  "Lovelace",
			want:      "Ada Lovelace",
		},
		{
			name:      "missing first name",
			firstName: "",
			lastName:  "Lovelace",
			want:      " Lovelace",
		},
		{
			name:      "missing last name",
			firstName: "Ada",
			lastName:  "",
			want:      "Ada ",
		},
		{
			name:      "both names empty",
			firstName: "",
			lastName:  "",
			want:      " ",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			user := &User{
				FirstName: tc.firstName,
				LastName:  tc.lastName,
			}
			if got := user.FullName(); got != tc.want {
				t.Fatalf("FullName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRoleHasPermissionReturnsFalse(t *testing.T) {
	role := &Role{
		Permissions: datatypes.JSON(`[
			{"resource":"workflow","action":"read"},
			{"resource":"task","action":"write"}
		]`),
	}

	if role.HasPermission("workflow", "read") {
		t.Fatal("expected HasPermission to return false for current placeholder implementation")
	}
	if role.HasPermission("task", "write") {
		t.Fatal("expected HasPermission to return false for current placeholder implementation")
	}
	if role.HasPermission("workflow", "delete") {
		t.Fatal("expected HasPermission to return false for current placeholder implementation")
	}
}

func TestEmployeeInvitationStateHelpers(t *testing.T) {
	now := time.Now()

	pendingFuture := &EmployeeInvitation{
		Status:    "pending",
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if pendingFuture.IsExpired() {
		t.Fatal("expected future invitation not to be expired")
	}
	if !pendingFuture.IsPending() {
		t.Fatal("expected pending future invitation to be pending")
	}
	if !pendingFuture.CanAccept() {
		t.Fatal("expected pending future invitation to be acceptable")
	}

	pendingPast := &EmployeeInvitation{
		Status:    "pending",
		ExpiresAt: now.Add(-1 * time.Hour),
	}
	if !pendingPast.IsExpired() {
		t.Fatal("expected past invitation to be expired")
	}
	if pendingPast.IsPending() {
		t.Fatal("expected expired invitation not to be pending")
	}
	if pendingPast.CanAccept() {
		t.Fatal("expected expired invitation not to be acceptable")
	}

	acceptedFuture := &EmployeeInvitation{
		Status:    "accepted",
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if acceptedFuture.IsPending() {
		t.Fatal("expected accepted invitation not to be pending")
	}
	if acceptedFuture.CanAccept() {
		t.Fatal("expected accepted invitation not to be acceptable")
	}
}
