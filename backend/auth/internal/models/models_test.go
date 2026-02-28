package models

import (
	"testing"

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
			name:      "organization memberships",
			model:     OrganizationMembership{},
			tableName: "organization_memberships",
		},
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

func TestOrganizationMembershipIsOrgAdmin(t *testing.T) {
	testCases := []struct {
		name      string
		clerkRole string
		want      bool
	}{
		{
			name:      "org admin role",
			clerkRole: "org:admin",
			want:      true,
		},
		{
			name:      "legacy admin role",
			clerkRole: "admin",
			want:      true,
		},
		{
			name:      "member role",
			clerkRole: "org:member",
			want:      false,
		},
		{
			name:      "empty role",
			clerkRole: "",
			want:      false,
		},
		{
			name:      "case sensitive mismatch",
			clerkRole: "ORG:ADMIN",
			want:      false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			membership := &OrganizationMembership{ClerkRole: tc.clerkRole}
			if got := membership.IsOrgAdmin(); got != tc.want {
				t.Fatalf("IsOrgAdmin() = %v, want %v", got, tc.want)
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
