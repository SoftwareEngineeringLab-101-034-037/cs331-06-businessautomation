package integrations

import (
	"context"
	"net/http"
	"reflect"
	"strings"

	"github.com/example/business-automation/backend/integrations/internal/models"
)

type Provider interface {
	ID() string
	DisplayName() string
	IsConfigured() bool
	MissingFields() []string

	GetClient(ctx context.Context, orgID string) (*http.Client, error)
	ListConnections(ctx context.Context, orgID string) ([]*models.OAuthToken, error)
	Disconnect(ctx context.Context, orgID string) error
	DisconnectAccount(ctx context.Context, orgID, accountID string) error

	IsNotConfiguredError(err error) bool
	IsReconnectRequiredError(err error) bool
	IsNotConnectedError(err error) bool
}

type TriggerSource interface {
	TriggerEventPath() string
}

type ActionExecutor interface {
	SupportedActions() []string
}

type WebhookSource interface {
	WebhookPath() string
}

type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(provider Provider) {
	if r == nil || provider == nil {
		return
	}
	value := reflect.ValueOf(provider)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return
		}
	}
	id := strings.TrimSpace(provider.ID())
	if id == "" {
		return
	}
	r.providers[id] = provider
}

func (r *Registry) Get(id string) (Provider, bool) {
	if r == nil {
		return nil, false
	}
	provider, ok := r.providers[strings.TrimSpace(id)]
	return provider, ok
}

func (r *Registry) GetOrDefault(id, defaultID string) (Provider, bool) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		trimmed = strings.TrimSpace(defaultID)
	}
	return r.Get(trimmed)
}

func (r *Registry) IDs() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.providers))
	for id := range r.providers {
		out = append(out, id)
	}
	return out
}
