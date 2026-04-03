package storage

import (
	"context"

	"github.com/example/business-automation/backend/integrations/internal/models"
)

type Store interface {
	SaveToken(ctx context.Context, token *models.OAuthToken) error
	GetToken(ctx context.Context, orgID string) (*models.OAuthToken, error)
	GetTokenByAccount(ctx context.Context, orgID, provider, accountID string) (*models.OAuthToken, error)
	ListTokens(ctx context.Context, orgID, provider string) ([]*models.OAuthToken, error)
	DeleteToken(ctx context.Context, orgID string) error
	DeleteTokenByAccount(ctx context.Context, orgID, provider, accountID string) error

	SaveWatch(ctx context.Context, watch *models.FormWatch) error
	GetWatch(ctx context.Context, id string) (*models.FormWatch, error)
	ListWatches(ctx context.Context, orgID string) ([]*models.FormWatch, error)
	ListWatchesByProvider(ctx context.Context, orgID, provider string) ([]*models.FormWatch, error)
	ListActiveWatches(ctx context.Context) ([]*models.FormWatch, error)
	ListActiveWatchesByProvider(ctx context.Context, provider string) ([]*models.FormWatch, error)
	UpdateWatch(ctx context.Context, watch *models.FormWatch) error
	DeleteWatch(ctx context.Context, id string) error

	SaveGmailWatch(ctx context.Context, watch *models.GmailWatch) error
	GetGmailWatch(ctx context.Context, id string) (*models.GmailWatch, error)
	ListGmailWatches(ctx context.Context, orgID string) ([]*models.GmailWatch, error)
	ListActiveGmailWatches(ctx context.Context) ([]*models.GmailWatch, error)
	UpdateGmailWatch(ctx context.Context, watch *models.GmailWatch) error
	DeleteGmailWatch(ctx context.Context, id string) error

	Close() error
}
