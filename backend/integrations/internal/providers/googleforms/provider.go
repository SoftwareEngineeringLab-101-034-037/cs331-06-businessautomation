package googleforms

import (
	"context"
	"net/http"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/integrations"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/models"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/integrations/internal/oauth"
)

const ProviderID = "google_forms"
const TriggerEventPath = "/integrations/google-forms/events"

type Provider struct {
	oauthSvc *oauth.Service
}

func NewProvider(oauthSvc *oauth.Service) integrations.Provider {
	return &Provider{oauthSvc: oauthSvc}
}

func (p *Provider) ID() string {
	return ProviderID
}

func (p *Provider) DisplayName() string {
	return "Google Forms"
}

func (p *Provider) IsConfigured() bool {
	return p.oauthSvc != nil && p.oauthSvc.IsConfigured()
}

func (p *Provider) MissingFields() []string {
	if p.oauthSvc == nil {
		return []string{"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URI"}
	}
	return p.oauthSvc.MissingFields()
}

func (p *Provider) GetClient(ctx context.Context, orgID string) (*http.Client, error) {
	if p.oauthSvc == nil {
		return nil, oauth.ErrOAuthNotConfigured
	}
	return p.oauthSvc.GetClient(ctx, orgID)
}

func (p *Provider) ListConnections(ctx context.Context, orgID string) ([]*models.OAuthToken, error) {
	if p.oauthSvc == nil {
		return []*models.OAuthToken{}, nil
	}
	return p.oauthSvc.ListConnections(ctx, orgID)
}

func (p *Provider) Disconnect(ctx context.Context, orgID string) error {
	if p.oauthSvc == nil {
		return nil
	}
	return p.oauthSvc.Disconnect(ctx, orgID)
}

func (p *Provider) DisconnectAccount(ctx context.Context, orgID, accountID string) error {
	if p.oauthSvc == nil {
		return nil
	}
	return p.oauthSvc.DisconnectAccount(ctx, orgID, accountID)
}

func (p *Provider) IsNotConfiguredError(err error) bool {
	return oauth.IsNotConfiguredError(err)
}

func (p *Provider) IsReconnectRequiredError(err error) bool {
	return oauth.IsReconnectRequiredError(err)
}

func (p *Provider) IsNotConnectedError(err error) bool {
	return oauth.IsNotConnectedError(err)
}

func (p *Provider) TriggerEventPath() string {
	return TriggerEventPath
}

func (p *Provider) SupportedActions() []string {
	return []string{}
}

func (p *Provider) WebhookPath() string {
	return ""
}
