package googleforms

import (
	"testing"

	"github.com/example/business-automation/backend/integrations/internal/oauth"
)

func TestProviderIdentityAndDefaults(t *testing.T) {
	p := NewProvider(nil)
	if p.ID() != ProviderID {
		t.Fatalf("expected provider id %q", ProviderID)
	}
	if p.DisplayName() == "" {
		t.Fatalf("display name must not be empty")
	}
	if p.IsConfigured() {
		t.Fatalf("nil oauth service must be unconfigured")
	}
	if got := p.MissingFields(); len(got) == 0 {
		t.Fatalf("expected missing fields for nil oauth")
	}
}

func TestProviderErrorClassifiers(t *testing.T) {
	p := NewProvider(nil)
	if !p.IsNotConfiguredError(oauth.ErrOAuthNotConfigured) {
		t.Fatalf("expected ErrOAuthNotConfigured to match")
	}
}
