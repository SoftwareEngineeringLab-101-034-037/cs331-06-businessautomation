package gmail

import (
	"testing"

	"github.com/example/business-automation/backend/integrations/internal/oauth"
)

func TestProviderIdentityAndActions(t *testing.T) {
	p := NewProvider(nil).(*Provider)
	if p.ID() != ProviderID {
		t.Fatalf("expected provider id %q", ProviderID)
	}
	if p.TriggerEventPath() == "" {
		t.Fatalf("trigger path must not be empty")
	}
	actions := p.SupportedActions()
	if len(actions) == 0 || actions[0] != "send_email" {
		t.Fatalf("expected send_email action")
	}
}

func TestProviderClassifiers(t *testing.T) {
	p := NewProvider(nil).(*Provider)
	if !p.IsNotConfiguredError(oauth.ErrOAuthNotConfigured) {
		t.Fatalf("expected ErrOAuthNotConfigured to match")
	}
}
