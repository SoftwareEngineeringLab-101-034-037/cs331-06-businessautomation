package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleSendRequiresOrgAuthorization(t *testing.T) {
	h := NewHandler(nil, nil, func(context.Context) string { return "" })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/integrations/gmail/send", nil)
	h.HandleSend(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleMessagesMethodNotAllowed(t *testing.T) {
	h := NewHandler(nil, nil, func(context.Context) string { return "org_1" })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/integrations/gmail/messages", nil)
	h.HandleMessages(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleWatchByIDRouteValidation(t *testing.T) {
	h := NewHandler(nil, nil, func(context.Context) string { return "org_1" })

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/integrations/gmail/watches/", nil)
	h.HandleWatchByID(w1, req1)
	if w1.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/integrations/gmail/watches/a/b", nil)
	h.HandleWatchByID(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w2.Code)
	}
}
