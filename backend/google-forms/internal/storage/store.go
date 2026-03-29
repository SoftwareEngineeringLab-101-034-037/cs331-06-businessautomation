package storage

import (
	"context"

	"github.com/example/business-automation/backend/google-forms/internal/models"
)

type Store interface {
	SaveToken(ctx context.Context, token *models.OAuthToken) error
	GetToken(ctx context.Context, orgID string) (*models.OAuthToken, error)
	DeleteToken(ctx context.Context, orgID string) error

	SaveWatch(ctx context.Context, watch *models.FormWatch) error
	GetWatch(ctx context.Context, id string) (*models.FormWatch, error)
	ListWatches(ctx context.Context, orgID string) ([]*models.FormWatch, error)
	ListActiveWatches(ctx context.Context) ([]*models.FormWatch, error)
	UpdateWatch(ctx context.Context, watch *models.FormWatch) error
	DeleteWatch(ctx context.Context, id string) error

	Close() error
}
