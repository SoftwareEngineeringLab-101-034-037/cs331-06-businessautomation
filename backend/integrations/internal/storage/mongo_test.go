package storage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestWatchProviderFilterDefaultsToGoogleForms(t *testing.T) {
	filter := watchProviderFilter("")
	orFilters, ok := filter["$or"].([]bson.M)
	if !ok {
		t.Fatalf("expected default filter to include $or as []bson.M")
	}
	if len(orFilters) != 2 {
		t.Fatalf("expected 2 default provider alternatives, got %d", len(orFilters))
	}
	if orFilters[0]["provider"] != defaultWatchProvider {
		t.Fatalf("expected first fallback provider %q, got %v", defaultWatchProvider, orFilters[0]["provider"])
	}
	if _, found := orFilters[1]["provider"]; !found {
		t.Fatalf("expected default filter to include $or")
	}
}

func TestWatchProviderFilterAllReturnsExplicitProvider(t *testing.T) {
	filter := watchProviderFilter("gmail")
	provider, ok := filter["provider"].(string)
	if !ok {
		t.Fatalf("expected provider filter")
	}
	if provider != "gmail" {
		t.Fatalf("expected provider=gmail, got %q", provider)
	}
}

func TestEnsureIndexesCallsAllIndexes(t *testing.T) {
	prevCreate := createOneIndex
	t.Cleanup(func() { createOneIndex = prevCreate })

	calls := 0
	createOneIndex = func(_ context.Context, _ *mongo.Collection, _ mongo.IndexModel) error {
		calls++
		return nil
	}

	store := &MongoStore{}
	if err := store.ensureIndexes(context.Background()); err != nil {
		t.Fatalf("expected ensureIndexes success, got %v", err)
	}

	const expectedCalls = 10
	if calls != expectedCalls {
		t.Fatalf("expected %d index calls, got %d", expectedCalls, calls)
	}
}

func TestEnsureIndexesReturnsError(t *testing.T) {
	prevCreate := createOneIndex
	t.Cleanup(func() { createOneIndex = prevCreate })

	createOneIndex = func(_ context.Context, _ *mongo.Collection, _ mongo.IndexModel) error {
		return errors.New("boom")
	}

	store := &MongoStore{}
	err := store.ensureIndexes(context.Background())
	if err == nil {
		t.Fatalf("expected ensureIndexes to fail")
	}
	if !strings.Contains(err.Error(), "create oauth_tokens") {
		t.Fatalf("expected wrapped index error, got %v", err)
	}
}
