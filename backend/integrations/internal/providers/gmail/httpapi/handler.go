package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/example/business-automation/backend/integrations/internal/googleapi"
	"github.com/example/business-automation/backend/integrations/internal/integrations"
	"github.com/example/business-automation/backend/integrations/internal/models"
	"github.com/example/business-automation/backend/integrations/internal/oauth"
)

type OrgIDExtractor func(ctx context.Context) string

type Handler struct {
	store        Store
	provider     integrations.Provider
	extractOrgID OrgIDExtractor
}

type Store interface {
	SaveGmailWatch(ctx context.Context, watch *models.GmailWatch) error
	GetGmailWatch(ctx context.Context, id string) (*models.GmailWatch, error)
	ListGmailWatches(ctx context.Context, orgID string) ([]*models.GmailWatch, error)
	UpdateGmailWatch(ctx context.Context, watch *models.GmailWatch) error
	DeleteGmailWatch(ctx context.Context, id string) error
}

type accountScopedClientProvider interface {
	GetClientForAccount(ctx context.Context, orgID, accountID string) (*http.Client, error)
}

func NewHandler(store Store, provider integrations.Provider, extractOrgID OrgIDExtractor) *Handler {
	return &Handler{store: store, provider: provider, extractOrgID: extractOrgID}
}

func (h *Handler) HandleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	var req struct {
		googleapi.SendMailRequest
		AccountID     string `json:"account_id,omitempty"`
		FromAccountID string `json:"from_account_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	accountID := strings.TrimSpace(req.FromAccountID)
	if accountID == "" {
		accountID = strings.TrimSpace(req.AccountID)
	}

	client, ok := h.getOAuthClientOrFail(w, r, orgID, accountID)
	if !ok {
		return
	}
	result, err := googleapi.SendEmail(client, req.SendMailRequest)
	if err != nil {
		statusCode, message := classifyGmailSendError(err)
		log.Printf("gmail.HandleSend failed org_id=%q account_id=%q status=%d err=%v classified=%q", orgID, accountID, statusCode, err, message)
		writeError(w, statusCode, message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "sent",
		"message_id": result.MessageID,
		"thread_id":  result.ThreadID,
	})
}

func (h *Handler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = "in:inbox"
	}
	afterTS, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("after_ts")), 10, 64)
	maxResults, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("max")))

	client, ok := h.getOAuthClientOrFail(w, r, orgID, "")
	if !ok {
		return
	}

	messages, err := googleapi.ListMessages(client, query, afterTS, maxResults)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"query":    query,
		"count":    len(messages),
		"messages": messages,
	})
}

func (h *Handler) HandleWatches(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createWatch(w, r)
	case http.MethodGet:
		h.listWatches(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) HandleWatchByID(w http.ResponseWriter, r *http.Request) {
	prefix := "/integrations/gmail/watches/"
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, prefix))
	if id == "" {
		writeError(w, http.StatusBadRequest, "watch id required")
		return
	}
	if strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getWatch(w, r, id)
	case http.MethodPut:
		h.updateWatch(w, r, id)
	case http.MethodDelete:
		h.deleteWatch(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) createWatch(w http.ResponseWriter, r *http.Request) {
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	var req struct {
		WorkflowID string `json:"workflow_id"`
		Query      string `json:"query"`
		Active     *bool  `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.WorkflowID) == "" {
		writeError(w, http.StatusBadRequest, "workflow_id is required")
		return
	}

	watch := &models.GmailWatch{
		OrgID:      orgID,
		WorkflowID: strings.TrimSpace(req.WorkflowID),
		Query:      strings.TrimSpace(req.Query),
		Active:     true,
	}
	if watch.Query == "" {
		watch.Query = "in:inbox"
	}
	if req.Active != nil {
		watch.Active = *req.Active
	}
	if err := h.store.SaveGmailWatch(r.Context(), watch); err != nil {
		log.Printf("gmail.createWatch failed org_id=%q workflow_id=%q: %v", orgID, watch.WorkflowID, err)
		writeError(w, http.StatusInternalServerError, "failed to save gmail watch")
		return
	}

	writeJSON(w, http.StatusCreated, watch)
}

func (h *Handler) listWatches(w http.ResponseWriter, r *http.Request) {
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	watches, err := h.store.ListGmailWatches(r.Context(), orgID)
	if err != nil {
		log.Printf("gmail.listWatches failed org_id=%q: %v", orgID, err)
		writeError(w, http.StatusInternalServerError, "unable to retrieve watches")
		return
	}
	if watches == nil {
		watches = []*models.GmailWatch{}
	}
	writeJSON(w, http.StatusOK, watches)
}

func (h *Handler) getWatch(w http.ResponseWriter, r *http.Request, id string) {
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	watch, err := h.store.GetGmailWatch(r.Context(), id)
	if err != nil {
		log.Printf("gmail.getWatch failed org_id=%q watch_id=%q: %v", orgID, id, err)
		writeError(w, http.StatusInternalServerError, "unable to retrieve watch")
		return
	}
	if watch == nil || strings.TrimSpace(watch.OrgID) != orgID {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}
	writeJSON(w, http.StatusOK, watch)
}

func (h *Handler) updateWatch(w http.ResponseWriter, r *http.Request, id string) {
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	watch, err := h.store.GetGmailWatch(r.Context(), id)
	if err != nil {
		log.Printf("gmail.updateWatch load failed org_id=%q watch_id=%q: %v", orgID, id, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if watch == nil || strings.TrimSpace(watch.OrgID) != orgID {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}

	var patch struct {
		WorkflowID string `json:"workflow_id"`
		Query      string `json:"query"`
		Active     *bool  `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(patch.WorkflowID) != "" {
		watch.WorkflowID = strings.TrimSpace(patch.WorkflowID)
	}
	if strings.TrimSpace(patch.Query) != "" {
		watch.Query = strings.TrimSpace(patch.Query)
	}
	if patch.Active != nil {
		watch.Active = *patch.Active
	}

	if err := h.store.UpdateGmailWatch(r.Context(), watch); err != nil {
		log.Printf("gmail.updateWatch save failed org_id=%q watch_id=%q: %v", orgID, id, err)
		writeError(w, http.StatusInternalServerError, "failed to update watch")
		return
	}
	writeJSON(w, http.StatusOK, watch)
}

func (h *Handler) deleteWatch(w http.ResponseWriter, r *http.Request, id string) {
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}
	watch, err := h.store.GetGmailWatch(r.Context(), id)
	if err != nil {
		log.Printf("gmail.deleteWatch load failed org_id=%q watch_id=%q: %v", orgID, id, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if watch == nil || strings.TrimSpace(watch.OrgID) != orgID {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}
	if err := h.store.DeleteGmailWatch(r.Context(), id); err != nil {
		log.Printf("gmail.deleteWatch failed org_id=%q watch_id=%q: %v", orgID, id, err)
		writeError(w, http.StatusInternalServerError, "failed to delete watch")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getOAuthClientOrFail(w http.ResponseWriter, r *http.Request, orgID, accountID string) (*http.Client, bool) {
	if h.provider == nil {
		writeError(w, http.StatusServiceUnavailable, "integration provider unavailable")
		return nil, false
	}
	var (
		client *http.Client
		err    error
	)
	if accountID != "" {
		if scoped, ok := h.provider.(accountScopedClientProvider); ok {
			client, err = scoped.GetClientForAccount(r.Context(), orgID, accountID)
		} else {
			client, err = h.provider.GetClient(r.Context(), orgID)
		}
	} else {
		client, err = h.provider.GetClient(r.Context(), orgID)
	}
	if err == nil {
		return client, true
	}
	if h.provider.IsNotConfiguredError(err) {
		writeError(w, http.StatusServiceUnavailable, "Gmail integration is not configured yet. Ask a platform admin to set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URI.")
		return nil, false
	}
	if strings.TrimSpace(accountID) != "" && oauth.IsAccountNotFoundError(err) {
		writeError(w, http.StatusBadRequest, "from_account_id does not match any connected Gmail account for this organization")
		return nil, false
	}
	if h.provider.IsNotConnectedError(err) || h.provider.IsReconnectRequiredError(err) {
		writeError(w, http.StatusUnauthorized, "Google connection expired or became invalid. Please reconnect from the Integrations page.")
		return nil, false
	}
	log.Printf("gmail.getOAuthClientOrFail failed org_id=%q: %v", orgID, err)
	writeError(w, http.StatusInternalServerError, "internal error fetching integration OAuth client")
	return nil, false
}

func (h *Handler) orgID(ctx context.Context) string {
	if h.extractOrgID == nil {
		return ""
	}
	return strings.TrimSpace(h.extractOrgID(ctx))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func classifyGmailSendError(err error) (int, string) {
	if err == nil {
		return http.StatusBadGateway, "failed to send gmail message"
	}

	message := strings.TrimSpace(err.Error())
	if isGmailValidationError(message) {
		return http.StatusBadRequest, message
	}

	statusCode, upstreamBody, ok := parseGmailSendFailure(message)
	if !ok {
		return http.StatusBadGateway, message
	}

	switch statusCode {
	case http.StatusUnauthorized:
		return http.StatusUnauthorized, "Google connection expired or became invalid. Please reconnect from the Integrations page."
	case http.StatusForbidden:
		if upstreamBody != "" {
			return http.StatusForbidden, fmt.Sprintf("gmail rejected the send request (status=%d): %s", statusCode, upstreamBody)
		}
		return http.StatusForbidden, "Connected Gmail account is not allowed to send this email."
	}

	if statusCode >= 400 && statusCode < 500 {
		if upstreamBody != "" {
			return http.StatusBadRequest, fmt.Sprintf("gmail rejected the send request (status=%d): %s", statusCode, upstreamBody)
		}
		return http.StatusBadRequest, fmt.Sprintf("gmail rejected the send request (status=%d)", statusCode)
	}

	if upstreamBody != "" {
		return http.StatusBadGateway, fmt.Sprintf("gmail upstream error (status=%d): %s", statusCode, upstreamBody)
	}
	return http.StatusBadGateway, fmt.Sprintf("gmail upstream error (status=%d)", statusCode)
}

func isGmailValidationError(message string) bool {
	return strings.Contains(message, "at least one recipient in to is required") ||
		strings.Contains(message, "subject is required") ||
		strings.Contains(message, "body_text or body_html is required")
}

func parseGmailSendFailure(message string) (statusCode int, body string, ok bool) {
	const prefix = "gmail send failed status="
	if !strings.HasPrefix(message, prefix) {
		return 0, "", false
	}

	rest := strings.TrimPrefix(message, prefix)
	parts := strings.SplitN(rest, " body=", 2)
	code, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, "", false
	}

	upstreamBody := ""
	if len(parts) > 1 {
		upstreamBody = strings.TrimSpace(parts[1])
	}
	return code, upstreamBody, true
}
