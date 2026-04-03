package gmail

import (
	"context"
	"net/http"

	"github.com/example/business-automation/backend/integrations/internal/integrations"
	"github.com/example/business-automation/backend/integrations/internal/models"
	"github.com/example/business-automation/backend/integrations/internal/oauth"
)

const ProviderID = "gmail"
const TriggerEventPath = "/integrations/gmail/events"

var SupportedActions = []string{"send_email"}

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
	return "Gmail"
}

func (p *Provider) IsConfigured() bool {
	return p.oauthSvc != nil && p.oauthSvc.IsConfigured()
}

func (p *Provider) MissingFields() []string {
	if p.oauthSvc == nil {
		return nil
	}
	return p.oauthSvc.MissingFields()
}

func (p *Provider) GetClient(ctx context.Context, orgID string) (*http.Client, error) {
	if p.oauthSvc == nil {
		return nil, oauth.ErrOAuthNotConfigured
	}
	return p.oauthSvc.GetClientForProvider(ctx, orgID, ProviderID)
}

func (p *Provider) GetClientForAccount(ctx context.Context, orgID, accountID string) (*http.Client, error) {
	if p.oauthSvc == nil {
		return nil, oauth.ErrOAuthNotConfigured
	}
	return p.oauthSvc.GetClientForProviderAndAccount(ctx, orgID, ProviderID, accountID)
}

func (p *Provider) ListConnections(ctx context.Context, orgID string) ([]*models.OAuthToken, error) {
	if p.oauthSvc == nil {
		return []*models.OAuthToken{}, nil
	}
	return p.oauthSvc.ListConnectionsForProvider(ctx, orgID, ProviderID)
}

func (p *Provider) Disconnect(ctx context.Context, orgID string) error {
	if p.oauthSvc == nil {
		return nil
	}
	return p.oauthSvc.DisconnectForProvider(ctx, orgID, ProviderID)
}

func (p *Provider) DisconnectAccount(ctx context.Context, orgID, accountID string) error {
	if p.oauthSvc == nil {
		return nil
	}
	return p.oauthSvc.DisconnectAccountForProvider(ctx, orgID, ProviderID, accountID)
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
	return append([]string{}, SupportedActions...)
}

func (p *Provider) WebhookPath() string {
	return ""
}
