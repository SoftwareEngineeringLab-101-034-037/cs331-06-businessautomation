package executor

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type authAwareDirectoryStub struct {
	orgID      string
	roleName   string
	authHeader string
	result     []string
	err        error
}

func (d *authAwareDirectoryStub) ListMemberIDs(orgID, roleName string) ([]string, error) {
	return nil, nil
}

func (d *authAwareDirectoryStub) ListMemberIDsWithAuth(orgID, roleName, authHeader string) ([]string, error) {
	d.orgID = orgID
	d.roleName = roleName
	d.authHeader = authHeader
	return d.result, d.err
}

func TestListRoleMemberIDsUsesAuthAwareDirectory(t *testing.T) {
	exec := NewExecutor(newMockStore(), &mockEmail{}, nil)
	directory := &authAwareDirectoryStub{result: []string{"user-1"}}

	memberIDs, err := exec.listRoleMemberIDs(directory, "org-1", "finance", "Bearer token-123")
	if err != nil {
		t.Fatalf("listRoleMemberIDs returned error: %v", err)
	}
	if len(memberIDs) != 1 || memberIDs[0] != "user-1" {
		t.Fatalf("unexpected member IDs: %#v", memberIDs)
	}
	if directory.orgID != "org-1" || directory.roleName != "finance" || directory.authHeader != "Bearer token-123" {
		t.Fatalf("auth-aware lookup did not receive expected inputs: %#v", directory)
	}
}

func TestHTTPRoleDirectoryReturnsEmptySliceWhenRoleHasNoMembers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"role-1","name":"Finance","members":[]}]`))
	}))
	defer server.Close()

	directory, err := NewHTTPRoleDirectory(server.URL, "")
	if err != nil {
		t.Fatalf("NewHTTPRoleDirectory returned error: %v", err)
	}

	memberIDs, err := directory.ListMemberIDs("org-1", "Finance")
	if err != nil {
		t.Fatalf("ListMemberIDs returned error: %v", err)
	}
	if memberIDs == nil {
		t.Fatalf("expected empty slice, got nil")
	}
	if len(memberIDs) != 0 {
		t.Fatalf("expected no members, got %#v", memberIDs)
	}
}
