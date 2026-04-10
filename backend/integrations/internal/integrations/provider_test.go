package integrations

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"testing"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/models"
)

type fakeProvider struct {
	id string
}

func (f fakeProvider) ID() string                                              { return f.id }
func (f fakeProvider) DisplayName() string                                     { return "fake" }
func (f fakeProvider) IsConfigured() bool                                      { return true }
func (f fakeProvider) MissingFields() []string                                 { return nil }
func (f fakeProvider) GetClient(context.Context, string) (*http.Client, error) { return nil, nil }
func (f fakeProvider) ListConnections(context.Context, string) ([]*models.OAuthToken, error) {
	return nil, nil
}
func (f fakeProvider) Disconnect(context.Context, string) error                { return nil }
func (f fakeProvider) DisconnectAccount(context.Context, string, string) error { return nil }
func (f fakeProvider) IsNotConfiguredError(error) bool                         { return false }
func (f fakeProvider) IsReconnectRequiredError(error) bool                     { return false }
func (f fakeProvider) IsNotConnectedError(error) bool                          { return false }

func TestRegistryRegisterGetAndIDs(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeProvider{id: "google_forms"})
	r.Register(fakeProvider{id: "gmail"})

	if _, ok := r.Get("google_forms"); !ok {
		t.Fatalf("expected google_forms provider")
	}
	ids := r.IDs()
	sort.Strings(ids)
	expected := []string{"gmail", "google_forms"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d (%v)", len(expected), len(ids), ids)
	}
	for idx, want := range expected {
		if ids[idx] != want {
			t.Fatalf("expected IDs %v, got %v", expected, ids)
		}
	}
}

func TestRegistryGetOrDefault(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeProvider{id: "gmail"})

	p, ok := r.GetOrDefault("", "gmail")
	if !ok || p.ID() != "gmail" {
		t.Fatalf("expected fallback provider")
	}
}

func TestRegistryIgnoresEmptyOrNil(t *testing.T) {
	r := NewRegistry()
	r.Register(nil)
	var typedNil *fakeProvider
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Register should not panic for typed-nil providers: %v", recovered)
		}
	}()
	r.Register(typedNil)
	r.Register(fakeProvider{id: ""})
	if len(r.IDs()) != 0 {
		t.Fatalf("expected no registered providers")
	}
}

func TestInterfaceSanity(t *testing.T) {
	var _ Provider = fakeProvider{id: "x"}
	if (fakeProvider{id: "x"}).IsReconnectRequiredError(errors.New("x")) {
		t.Fatalf("unexpected reconnect error classification")
	}
}
