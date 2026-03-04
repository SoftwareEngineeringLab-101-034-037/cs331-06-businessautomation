package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/example/business-automation/backend/workflow/internal/models"
)

type MongoStore struct {
	client  *mongo.Client
	db      *mongo.Database
	wfCol   *mongo.Collection
	instCol *mongo.Collection
	taskCol *mongo.Collection
}

func NewMongoStore(ctx context.Context, uri string) (*MongoStore, error) {
	if uri == "" {
		return nil, errors.New("empty mongo uri")
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	db := client.Database("workflowdb")
	s := &MongoStore{
		client:  client,
		db:      db,
		wfCol:   db.Collection("workflows"),
		instCol: db.Collection("instances"),
		taskCol: db.Collection("tasks"),
	}
	if err := s.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return s, nil
}

func (m *MongoStore) ensureIndexes(ctx context.Context) error {
	ictx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := m.wfCol.Indexes().CreateMany(ictx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "org_id", Value: 1}}},
	}); err != nil {
		return err
	}
	if _, err := m.instCol.Indexes().CreateMany(ictx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "workflow_id", Value: 1}}},
		{Keys: bson.D{{Key: "org_id", Value: 1}}},
	}); err != nil {
		return err
	}
	if _, err := m.taskCol.Indexes().CreateMany(ictx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "org_id", Value: 1}, {Key: "assigned_role", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "instance_id", Value: 1}}},
	}); err != nil {
		return err
	}
	return nil
}

// -- Workflows --

func (m *MongoStore) SaveWorkflow(w models.Workflow) (string, error) {
	if w.ID == "" {
		w.ID = generateShortID()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := m.wfCol.ReplaceOne(ctx, bson.M{"id": w.ID}, w, options.Replace().SetUpsert(true))
	return w.ID, err
}

func (m *MongoStore) GetWorkflow(id string) (models.Workflow, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var w models.Workflow
	err := m.wfCol.FindOne(ctx, bson.M{"id": id}).Decode(&w)
	if err != nil {
		return models.Workflow{}, false
	}
	return w, true
}

func (m *MongoStore) ListWorkflows(orgID string) ([]models.Workflow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cursor, err := m.wfCol.Find(ctx, bson.M{"org_id": orgID}, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var out []models.Workflow
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (m *MongoStore) DeleteWorkflow(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := m.wfCol.DeleteOne(ctx, bson.M{"id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return errors.New("not found")
	}
	return nil
}

// -- Instances --

func (m *MongoStore) SaveInstance(inst models.Instance) (string, error) {
	if inst.ID == "" {
		inst.ID = generateShortID()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := m.instCol.ReplaceOne(ctx, bson.M{"id": inst.ID}, inst, options.Replace().SetUpsert(true))
	return inst.ID, err
}

func (m *MongoStore) GetInstance(id string) (models.Instance, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var inst models.Instance
	err := m.instCol.FindOne(ctx, bson.M{"id": id}).Decode(&inst)
	if err != nil {
		return models.Instance{}, false
	}
	return inst, true
}

func (m *MongoStore) ListInstancesByWorkflow(workflowID string) ([]models.Instance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cursor, err := m.instCol.Find(ctx, bson.M{"workflow_id": workflowID},
		options.Find().SetSort(bson.D{{Key: "started_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var out []models.Instance
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// -- Tasks --

func (m *MongoStore) SaveTask(t models.TaskAssignment) (string, error) {
	if t.ID == "" {
		t.ID = generateShortID()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := m.taskCol.ReplaceOne(ctx, bson.M{"id": t.ID}, t, options.Replace().SetUpsert(true))
	return t.ID, err
}

func (m *MongoStore) GetTask(id string) (models.TaskAssignment, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var t models.TaskAssignment
	err := m.taskCol.FindOne(ctx, bson.M{"id": id}).Decode(&t)
	if err != nil {
		return models.TaskAssignment{}, false
	}
	return t, true
}

func (m *MongoStore) ListTasksByRole(orgID, role string) ([]models.TaskAssignment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cursor, err := m.taskCol.Find(ctx, bson.M{
		"org_id":        orgID,
		"assigned_role": role,
		"status":        string(models.TaskPending),
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var out []models.TaskAssignment
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (m *MongoStore) ListTasksByInstance(instanceID string) ([]models.TaskAssignment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cursor, err := m.taskCol.Find(ctx, bson.M{"instance_id": instanceID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var out []models.TaskAssignment
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func generateShortID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
