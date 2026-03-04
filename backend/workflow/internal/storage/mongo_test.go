package storage

import (
	"context"
	"encoding/hex"
	"testing"
)

func TestNewMongoStoreEmptyURI(t *testing.T) {
	_, err := NewMongoStore(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error for empty mongo uri")
	}
	if err.Error() != "empty mongo uri" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateShortID(t *testing.T) {
	id1 := generateShortID()
	id2 := generateShortID()

	if len(id1) != 24 || len(id2) != 24 {
		t.Fatalf("expected 24-char IDs, got %d and %d", len(id1), len(id2))
	}
	if _, err := hex.DecodeString(id1); err != nil {
		t.Fatalf("id1 is not hex: %s", id1)
	}
	if _, err := hex.DecodeString(id2); err != nil {
		t.Fatalf("id2 is not hex: %s", id2)
	}
	if id1 == id2 {
		t.Fatalf("expected different IDs, got identical: %s", id1)
	}
}
