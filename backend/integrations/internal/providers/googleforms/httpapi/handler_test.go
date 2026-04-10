package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleFormsMethodNotAllowed(t *testing.T) {
	h := NewHandler(nil, nil, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/forms", nil)
	h.HandleForms(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleFormByPathRequiresFormID(t *testing.T) {
	h := NewHandler(nil, nil, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/forms/", nil)
	h.HandleFormByPath(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleWatchesMethodNotAllowed(t *testing.T) {
	h := NewHandler(nil, nil, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/watches", nil)
	h.HandleWatches(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleWatchByIDRequiresID(t *testing.T) {
	h := NewHandler(nil, nil, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/watches/", nil)
	h.HandleWatchByID(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
