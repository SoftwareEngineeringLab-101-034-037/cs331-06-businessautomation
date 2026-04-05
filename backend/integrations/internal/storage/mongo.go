package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/example/business-automation/backend/integrations/internal/models"
)

const defaultWatchProvider = "google_forms"

type MongoStore struct {
	client       *mongo.Client
	tokens       *mongo.Collection
	watches      *mongo.Collection
	gmailWatches *mongo.Collection
}

var createOneIndex = func(ctx context.Context, collection *mongo.Collection, model mongo.IndexModel) error {
	_, err := collection.Indexes().CreateOne(ctx, model)
	return err
}

func NewMongo(uri, dbName string) (*MongoStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	db := client.Database(dbName)
	s := &MongoStore{
		client:       client,
		tokens:       db.Collection("oauth_tokens"),
		watches:      db.Collection("form_watches"),
		gmailWatches: db.Collection("gmail_watches"),
	}
	if err := s.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return s, nil
}

func (s *MongoStore) ensureIndexes(ctx context.Context) error {
	if err := createOneIndex(ctx, s.tokens, mongo.IndexModel{
		Keys: bson.D{{Key: "org_id", Value: 1}},
	}); err != nil {
		return fmt.Errorf("create oauth_tokens org_id index: %w", err)
	}
	if err := createOneIndex(ctx, s.tokens, mongo.IndexModel{
		Keys: bson.D{{Key: "org_id", Value: 1}, {Key: "provider", Value: 1}, {Key: "is_primary", Value: 1}},
	}); err != nil {
		return fmt.Errorf("create oauth_tokens primary sort index: %w", err)
	}
	if err := createOneIndex(ctx, s.tokens, mongo.IndexModel{
		Keys: bson.D{{Key: "org_id", Value: 1}, {Key: "provider", Value: 1}, {Key: "connected_at", Value: -1}},
	}); err != nil {
		return fmt.Errorf("create oauth_tokens connected_at index: %w", err)
	}
	if err := createOneIndex(ctx, s.tokens, mongo.IndexModel{
		Keys:    bson.D{{Key: "org_id", Value: 1}, {Key: "provider", Value: 1}, {Key: "account_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return fmt.Errorf("create oauth_tokens unique account index: %w", err)
	}
	if err := createOneIndex(ctx, s.watches, mongo.IndexModel{
		Keys: bson.D{{Key: "org_id", Value: 1}},
	}); err != nil {
		return fmt.Errorf("create form_watches org_id index: %w", err)
	}
	if err := createOneIndex(ctx, s.watches, mongo.IndexModel{
		Keys: bson.D{{Key: "active", Value: 1}},
	}); err != nil {
		return fmt.Errorf("create form_watches active index: %w", err)
	}
	if err := createOneIndex(ctx, s.watches, mongo.IndexModel{
		Keys: bson.D{{Key: "provider", Value: 1}},
	}); err != nil {
		return fmt.Errorf("create form_watches provider index: %w", err)
	}
	if err := createOneIndex(ctx, s.gmailWatches, mongo.IndexModel{
		Keys: bson.D{{Key: "org_id", Value: 1}},
	}); err != nil {
		return fmt.Errorf("create gmail_watches org_id index: %w", err)
	}
	if err := createOneIndex(ctx, s.gmailWatches, mongo.IndexModel{
		Keys: bson.D{{Key: "active", Value: 1}},
	}); err != nil {
		return fmt.Errorf("create gmail_watches active index: %w", err)
	}
	if err := createOneIndex(ctx, s.gmailWatches, mongo.IndexModel{
		Keys: bson.D{{Key: "org_id", Value: 1}, {Key: "workflow_id", Value: 1}},
	}); err != nil {
		return fmt.Errorf("create gmail_watches workflow index: %w", err)
	}
	return nil
}

func (s *MongoStore) SaveToken(ctx context.Context, token *models.OAuthToken) error {
	if token.Provider == "" {
		token.Provider = "google_forms"
	}
	if token.AccountID == "" {
		token.AccountID = "primary"
	}
	if token.IsPrimary {
		_, _ = s.tokens.UpdateMany(ctx,
			bson.M{"org_id": token.OrgID, "provider": token.Provider},
			bson.M{"$set": bson.M{"is_primary": false}},
		)
	}

	filter := bson.M{"org_id": token.OrgID, "provider": token.Provider, "account_id": token.AccountID}
	update := bson.M{"$set": token}
	_, err := s.tokens.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func (s *MongoStore) GetToken(ctx context.Context, orgID string) (*models.OAuthToken, error) {
	filter := bson.M{
		"org_id": orgID,
		"$or": []bson.M{
			{"provider": "google_forms"},
			{"provider": bson.M{"$exists": false}},
		},
	}
	var t models.OAuthToken
	err := s.tokens.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{Key: "is_primary", Value: -1}, {Key: "connected_at", Value: -1}})).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	s.normalizeTokenDefaults(ctx, &t)
	return &t, err
}

func (s *MongoStore) GetTokenByAccount(ctx context.Context, orgID, provider, accountID string) (*models.OAuthToken, error) {
	conditions := []bson.M{{"org_id": orgID}, watchProviderFilter(provider)}
	if accountID == "primary" {
		conditions = append(conditions, bson.M{"$or": []bson.M{
			{"account_id": accountID},
			{"account_id": bson.M{"$exists": false}},
		}})
	} else {
		conditions = append(conditions, bson.M{"account_id": accountID})
	}
	filter := bson.M{"$and": conditions}

	var t models.OAuthToken
	err := s.tokens.FindOne(ctx, filter).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	s.normalizeTokenDefaults(ctx, &t)
	return &t, err
}

func (s *MongoStore) ListTokens(ctx context.Context, orgID, provider string) ([]*models.OAuthToken, error) {
	cur, err := s.tokens.Find(ctx,
		bson.M{"$and": []bson.M{{"org_id": orgID}, watchProviderFilter(provider)}},
		options.Find().SetSort(bson.D{{Key: "is_primary", Value: -1}, {Key: "connected_at", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []*models.OAuthToken
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	for _, tok := range out {
		if tok == nil {
			continue
		}
		if tok.Provider == "" {
			tok.Provider = "google_forms"
		}
		if tok.AccountID == "" {
			tok.AccountID = "primary"
		}
	}
	return out, nil
}

func (s *MongoStore) DeleteToken(ctx context.Context, orgID string) error {
	_, err := s.tokens.DeleteMany(ctx, bson.M{
		"org_id": orgID,
		"$or": []bson.M{
			{"provider": "google_forms"},
			{"provider": bson.M{"$exists": false}},
		},
	})
	return err
}

func (s *MongoStore) DeleteTokenByAccount(ctx context.Context, orgID, provider, accountID string) error {
	_, err := s.tokens.DeleteOne(ctx, bson.M{"org_id": orgID, "provider": provider, "account_id": accountID})
	return err
}

func (s *MongoStore) SaveWatch(ctx context.Context, watch *models.FormWatch) error {
	watch.ID = primitive.NewObjectID()
	watch.CreatedAt = time.Now()
	watch.Active = true
	if strings.TrimSpace(watch.Provider) == "" {
		watch.Provider = defaultWatchProvider
	}
	_, err := s.watches.InsertOne(ctx, watch)
	return err
}

func (s *MongoStore) GetWatch(ctx context.Context, id string) (*models.FormWatch, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var w models.FormWatch
	err = s.watches.FindOne(ctx, bson.M{"_id": oid}).Decode(&w)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if strings.TrimSpace(w.Provider) == "" {
		w.Provider = defaultWatchProvider
	}
	return &w, err
}

func (s *MongoStore) ListWatches(ctx context.Context, orgID string) ([]*models.FormWatch, error) {
	return s.ListWatchesByProvider(ctx, orgID, "")
}

func (s *MongoStore) ListWatchesByProvider(ctx context.Context, orgID, provider string) ([]*models.FormWatch, error) {
	filter := bson.M{"org_id": orgID}
	trimmedProvider := strings.TrimSpace(provider)
	if trimmedProvider != "" && !strings.EqualFold(trimmedProvider, "all") {
		for key, value := range watchProviderFilter(trimmedProvider) {
			filter[key] = value
		}
	}

	cur, err := s.watches.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []*models.FormWatch
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	for _, watch := range out {
		if watch == nil {
			continue
		}
		if strings.TrimSpace(watch.Provider) == "" {
			watch.Provider = defaultWatchProvider
		}
	}
	return out, nil
}

func (s *MongoStore) ListActiveWatches(ctx context.Context) ([]*models.FormWatch, error) {
	return s.ListActiveWatchesByProvider(ctx, "")
}

func (s *MongoStore) ListActiveWatchesByProvider(ctx context.Context, provider string) ([]*models.FormWatch, error) {
	filter := bson.M{"active": true}
	trimmedProvider := strings.TrimSpace(provider)
	if trimmedProvider != "" && !strings.EqualFold(trimmedProvider, "all") {
		for key, value := range watchProviderFilter(trimmedProvider) {
			filter[key] = value
		}
	}

	cur, err := s.watches.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []*models.FormWatch
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	for _, watch := range out {
		if watch == nil {
			continue
		}
		if strings.TrimSpace(watch.Provider) == "" {
			watch.Provider = defaultWatchProvider
		}
	}
	return out, nil
}

func (s *MongoStore) UpdateWatch(ctx context.Context, watch *models.FormWatch) error {
	if strings.TrimSpace(watch.Provider) == "" {
		watch.Provider = defaultWatchProvider
	}
	_, err := s.watches.ReplaceOne(ctx, bson.M{"_id": watch.ID}, watch)
	return err
}

func (s *MongoStore) DeleteWatch(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = s.watches.DeleteOne(ctx, bson.M{"_id": oid})
	return err
}

func (s *MongoStore) SaveGmailWatch(ctx context.Context, watch *models.GmailWatch) error {
	watch.Active = true
	watch.ID = primitive.NewObjectID()
	watch.CreatedAt = time.Now()
	_, err := s.gmailWatches.InsertOne(ctx, watch)
	return err
}

func (s *MongoStore) GetGmailWatch(ctx context.Context, id string) (*models.GmailWatch, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var watch models.GmailWatch
	err = s.gmailWatches.FindOne(ctx, bson.M{"_id": oid}).Decode(&watch)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &watch, err
}

func (s *MongoStore) ListGmailWatches(ctx context.Context, orgID string) ([]*models.GmailWatch, error) {
	cur, err := s.gmailWatches.Find(ctx, bson.M{"org_id": orgID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []*models.GmailWatch
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *MongoStore) ListActiveGmailWatches(ctx context.Context) ([]*models.GmailWatch, error) {
	cur, err := s.gmailWatches.Find(ctx, bson.M{"active": true})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []*models.GmailWatch
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *MongoStore) UpdateGmailWatch(ctx context.Context, watch *models.GmailWatch) error {
	_, err := s.gmailWatches.ReplaceOne(ctx, bson.M{"_id": watch.ID}, watch)
	return err
}

func (s *MongoStore) DeleteGmailWatch(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = s.gmailWatches.DeleteOne(ctx, bson.M{"_id": oid})
	return err
}

func (s *MongoStore) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.client.Disconnect(ctx)
}

func (s *MongoStore) normalizeTokenDefaults(ctx context.Context, tok *models.OAuthToken) {
	if tok == nil {
		return
	}
	update := bson.M{}
	if tok.Provider == "" {
		tok.Provider = "google_forms"
		update["provider"] = tok.Provider
	}
	if tok.AccountID == "" {
		tok.AccountID = "primary"
		update["account_id"] = tok.AccountID
	}
	if len(update) > 0 {
		_, _ = s.tokens.UpdateByID(ctx, tok.ID, bson.M{"$set": update})
	}
}

func watchProviderFilter(provider string) bson.M {
	resolved := strings.TrimSpace(provider)
	if resolved == "" {
		resolved = defaultWatchProvider
	}
	if resolved == defaultWatchProvider {
		return bson.M{
			"$or": []bson.M{
				{"provider": defaultWatchProvider},
				{"provider": bson.M{"$exists": false}},
			},
		}
	}
	return bson.M{"provider": resolved}
}
