package executor

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestHTTPRoleDirectoryUsesInternalEndpointForServiceToken(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotSystemKey string
	var gotSystemCaller string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotSystemKey = r.Header.Get("X-System-Key")
		gotSystemCaller = r.Header.Get("X-System-Caller")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"role-1","name":"Finance","members":[{"id":"user-1"}]}]`))
	}))
	defer server.Close()

	directory, err := NewHTTPRoleDirectory(server.URL, "service-token")
	if err != nil {
		t.Fatalf("NewHTTPRoleDirectory returned error: %v", err)
	}

	memberIDs, err := directory.ListMemberIDs("org-1", "Finance")
	if err != nil {
		t.Fatalf("ListMemberIDs returned error: %v", err)
	}
	if len(memberIDs) != 1 || memberIDs[0] != "user-1" {
		t.Fatalf("unexpected member IDs: %#v", memberIDs)
	}
	if gotPath != "/api/system/orgs/org-1/roles" {
		t.Fatalf("expected system endpoint, got %q", gotPath)
	}
	if gotAuth != "" {
		t.Fatalf("expected no Authorization header for system endpoint, got %q", gotAuth)
	}
	if gotSystemKey != "service-token" {
		t.Fatalf("expected X-System-Key header, got %q", gotSystemKey)
	}
	if gotSystemCaller != "workflow-service" {
		t.Fatalf("expected X-System-Caller header, got %q", gotSystemCaller)
	}
}

func TestHTTPRoleDirectoryUsesPublicEndpointWhenAuthHeaderPresent(t *testing.T) {
	var gotPath string
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"role-1","name":"Finance","members":[{"id":"user-1"}]}]`))
	}))
	defer server.Close()

	directory, err := NewHTTPRoleDirectory(server.URL, "service-token")
	if err != nil {
		t.Fatalf("NewHTTPRoleDirectory returned error: %v", err)
	}

	type authAware interface {
		ListMemberIDsWithAuth(orgID, roleName, authHeader string) ([]string, error)
	}
	memberIDs, err := directory.(authAware).ListMemberIDsWithAuth("org-1", "Finance", "Bearer user-token")
	if err != nil {
		t.Fatalf("ListMemberIDsWithAuth returned error: %v", err)
	}
	if len(memberIDs) != 1 || memberIDs[0] != "user-1" {
		t.Fatalf("unexpected member IDs: %#v", memberIDs)
	}
	if gotPath != "/api/orgs/org-1/roles" {
		t.Fatalf("expected public endpoint, got %q", gotPath)
	}
	if gotAuth != "Bearer user-token" {
		t.Fatalf("expected forwarded user auth header, got %q", gotAuth)
	}
}

func TestHTTPRoleDirectory404IncludesEndpointDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	directory, err := NewHTTPRoleDirectory(server.URL, "service-token")
	if err != nil {
		t.Fatalf("NewHTTPRoleDirectory returned error: %v", err)
	}

	_, err = directory.ListMemberIDs("org-1", "Finance")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "endpoint not found") || !strings.Contains(err.Error(), "/api/system/orgs/org-1/roles") {
		t.Fatalf("expected endpoint-specific 404 error, got %v", err)
	}
}
