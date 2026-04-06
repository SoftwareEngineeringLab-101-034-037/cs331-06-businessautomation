package googleforms

import (
	"testing"

	"github.com/example/business-automation/backend/integrations/internal/integrations"
	"github.com/example/business-automation/backend/integrations/internal/oauth"
)

func TestProviderIdentityAndDefaults(t *testing.T) {
	p := NewProvider(nil)
	if p.ID() != ProviderID {
		t.Fatalf("expected provider id %q", ProviderID)
	}
	triggerSource, ok := p.(integrations.TriggerSource)
	if !ok || triggerSource.TriggerEventPath() == "" {
		t.Fatalf("trigger path capability must be available")
	}
	actionProvider, ok := p.(integrations.ActionExecutor)
	if !ok {
		t.Fatalf("supported actions capability must be available")
	}
	if got := actionProvider.SupportedActions(); len(got) != 0 {
		t.Fatalf("expected no provider actions, got %#v", got)
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
