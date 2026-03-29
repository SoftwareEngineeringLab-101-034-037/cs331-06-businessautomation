package storage

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/example/business-automation/backend/google-forms/internal/models"
)

type MongoStore struct {
	client  *mongo.Client
	tokens  *mongo.Collection
	watches *mongo.Collection
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
		client:  client,
		tokens:  db.Collection("oauth_tokens"),
		watches: db.Collection("form_watches"),
	}
	s.ensureIndexes(ctx)
	return s, nil
}

func (s *MongoStore) ensureIndexes(ctx context.Context) {
	s.tokens.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "org_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	s.watches.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "org_id", Value: 1}},
	})
	s.watches.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "active", Value: 1}},
	})
}

func (s *MongoStore) SaveToken(ctx context.Context, token *models.OAuthToken) error {
	filter := bson.M{"org_id": token.OrgID}
	update := bson.M{"$set": token}
	_, err := s.tokens.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func (s *MongoStore) GetToken(ctx context.Context, orgID string) (*models.OAuthToken, error) {
	var t models.OAuthToken
	err := s.tokens.FindOne(ctx, bson.M{"org_id": orgID}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &t, err
}

func (s *MongoStore) DeleteToken(ctx context.Context, orgID string) error {
	_, err := s.tokens.DeleteOne(ctx, bson.M{"org_id": orgID})
	return err
}

func (s *MongoStore) SaveWatch(ctx context.Context, watch *models.FormWatch) error {
	watch.ID = primitive.NewObjectID()
	watch.CreatedAt = time.Now()
	watch.Active = true
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
	return &w, err
}

func (s *MongoStore) ListWatches(ctx context.Context, orgID string) ([]*models.FormWatch, error) {
	cur, err := s.watches.Find(ctx, bson.M{"org_id": orgID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []*models.FormWatch
	return out, cur.All(ctx, &out)
}

func (s *MongoStore) ListActiveWatches(ctx context.Context) ([]*models.FormWatch, error) {
	cur, err := s.watches.Find(ctx, bson.M{"active": true})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []*models.FormWatch
	return out, cur.All(ctx, &out)
}

func (s *MongoStore) UpdateWatch(ctx context.Context, watch *models.FormWatch) error {
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

func (s *MongoStore) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.client.Disconnect(ctx)
}
