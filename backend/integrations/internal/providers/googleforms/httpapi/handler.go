package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/example/business-automation/backend/integrations/internal/googleapi"
	"github.com/example/business-automation/backend/integrations/internal/integrations"
	"github.com/example/business-automation/backend/integrations/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrgIDExtractor func(ctx context.Context) string

type Handler struct {
	store        Store
	provider     integrations.Provider
	extractOrgID OrgIDExtractor
}

type Store interface {
	SaveWatch(ctx context.Context, watch *models.FormWatch) error
	GetWatch(ctx context.Context, id string) (*models.FormWatch, error)
	ListWatchesByProvider(ctx context.Context, orgID, provider string) ([]*models.FormWatch, error)
	UpdateWatch(ctx context.Context, watch *models.FormWatch) error
	DeleteWatch(ctx context.Context, id string) error
}

func NewHandler(store Store, provider integrations.Provider, extractOrgID OrgIDExtractor) *Handler {
	return &Handler{store: store, provider: provider, extractOrgID: extractOrgID}
}

func (h *Handler) HandleForms(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createForm(w, r)
	case http.MethodGet:
		h.listForms(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) HandleFormByPath(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/forms/"), "/"), "/")
	if len(parts) == 0 {
		writeError(w, http.StatusBadRequest, "form_id required")
		return
	}
	formID := parts[0]
	if formID == "" {
		writeError(w, http.StatusBadRequest, "form_id required")
		return
	}
	if len(parts) > 1 {
		if len(parts) > 2 {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		switch parts[1] {
		case "responses":
			h.listFormResponses(w, r, formID)
			return
		case "fields":
			h.listFormFields(w, r, formID)
			return
		default:
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
	}
	h.getForm(w, r, formID)
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
	id := strings.TrimPrefix(r.URL.Path, "/watches/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "watch id required")
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

func (h *Handler) createForm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title     string               `json:"title"`
		Questions []googleapi.FormItem `json:"questions"`
		Publish   bool                 `json:"publish"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	client, ok := h.getOAuthClientOrFail(w, r, orgID)
	if !ok {
		return
	}

	form, err := googleapi.CreateForm(client, req.Title)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	publishWarning := ""

	if len(req.Questions) > 0 {
		if err := googleapi.AddQuestions(client, form.FormID, req.Questions); err != nil {
			log.Printf("forms.createForm add questions failed for form_id=%q: %v", form.FormID, err)
			warning := "form created but adding questions failed"
			if rollbackErr := googleapi.DeleteForm(client, form.FormID); rollbackErr != nil {
				log.Printf("forms.createForm rollback delete failed for form_id=%q: %v", form.FormID, rollbackErr)
				warning = "form was created but adding questions failed and rollback delete failed"
				writeJSON(w, http.StatusCreated, map[string]interface{}{
					"form_id": form.FormID,
					"stage":   "add_questions",
					"warning": warning,
				})
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"form_id":     form.FormID,
				"stage":       "add_questions",
				"warning":     warning,
				"rolled_back": true,
			})
			return
		}
	}

	if req.Publish {
		if err := googleapi.SetPublished(client, form.FormID, true); err != nil {
			log.Printf("warn: set published for form %s: %v", form.FormID, err)
			publishWarning = "form created but publish step failed"
		}
	}

	updated, err := googleapi.GetForm(client, form.FormID)
	if err != nil {
		if publishWarning != "" {
			writeJSON(w, http.StatusCreated, map[string]interface{}{
				"form":    form,
				"warning": publishWarning,
			})
			return
		}
		writeJSON(w, http.StatusCreated, form)
		return
	}
	if publishWarning != "" {
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"form":    updated,
			"warning": publishWarning,
		})
		return
	}
	writeJSON(w, http.StatusCreated, updated)
}

func (h *Handler) listForms(w http.ResponseWriter, r *http.Request) {
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	client, ok := h.getOAuthClientOrFail(w, r, orgID)
	if !ok {
		return
	}

	forms, err := googleapi.ListForms(client, 50)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "status 403") {
			writeError(w, http.StatusForbidden, "Google account lacks permission to list forms (required scope: drive.metadata.readonly). Reconnect the account and grant requested permissions.")
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"forms": forms,
		"count": len(forms),
	})
}

func (h *Handler) getForm(w http.ResponseWriter, r *http.Request, formID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	client, ok := h.getOAuthClientOrFail(w, r, orgID)
	if !ok {
		return
	}
	form, err := googleapi.GetForm(client, formID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, form)
}

func (h *Handler) listFormResponses(w http.ResponseWriter, r *http.Request, formID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	client, ok := h.getOAuthClientOrFail(w, r, orgID)
	if !ok {
		return
	}
	responses, err := googleapi.ListResponses(client, formID, r.URL.Query().Get("since"))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"form_id":   formID,
		"responses": responses,
		"count":     len(responses),
	})
}

func (h *Handler) listFormFields(w http.ResponseWriter, r *http.Request, formID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	orgID := h.orgID(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	client, ok := h.getOAuthClientOrFail(w, r, orgID)
	if !ok {
		return
	}

	form, err := googleapi.GetForm(client, formID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	type formField struct {
		QuestionID string `json:"question_id"`
		ItemID     string `json:"item_id,omitempty"`
		Title      string `json:"title"`
		Required   bool   `json:"required"`
		FieldType  string `json:"field_type,omitempty"`
	}

	fields := make([]formField, 0)
	fields = append(fields, formField{
		QuestionID: "_respondent_email",
		Title:      "Respondent Email",
		Required:   false,
		FieldType:  "email",
	})
	for idx, item := range form.Items {
		if item.QuestionItem == nil {
			continue
		}
		qid := strings.TrimSpace(item.QuestionItem.Question.QuestionID)
		if qid == "" {
			continue
		}
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = "Question " + strconv.Itoa(idx+1)
		}
		fieldType := detectFormFieldType(item.QuestionItem.Question)
		fields = append(fields, formField{
			QuestionID: qid,
			ItemID:     item.ItemID,
			Title:      title,
			Required:   item.QuestionItem.Question.Required,
			FieldType:  fieldType,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"form_id": formID,
		"title":   form.Info.Title,
		"count":   len(fields),
		"fields":  fields,
	})
}

func (h *Handler) createWatch(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil {
		writeError(w, http.StatusServiceUnavailable, "integration provider unavailable")
		return
	}
	authorizedOrgID := h.orgID(r.Context())

	var watch models.FormWatch
	if err := json.NewDecoder(r.Body).Decode(&watch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(watch.Provider) == "" {
		watch.Provider = h.provider.ID()
	}
	if strings.TrimSpace(watch.Provider) != h.provider.ID() {
		writeError(w, http.StatusBadRequest, "provider mismatch")
		return
	}
	if authorizedOrgID != "" {
		if strings.TrimSpace(watch.OrgID) != "" && strings.TrimSpace(watch.OrgID) != authorizedOrgID {
			writeError(w, http.StatusForbidden, "forbidden for org")
			return
		}
		watch.OrgID = authorizedOrgID
	}
	if watch.OrgID == "" || watch.FormID == "" || watch.WorkflowID == "" {
		writeError(w, http.StatusBadRequest, "org_id, form_id, and workflow_id are required")
		return
	}
	if err := h.store.SaveWatch(r.Context(), &watch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, watch)
}

func (h *Handler) listWatches(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil {
		writeError(w, http.StatusServiceUnavailable, "integration provider unavailable")
		return
	}

	authorizedOrgID := h.orgID(r.Context())
	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if authorizedOrgID != "" {
		if orgID != "" && orgID != authorizedOrgID {
			writeError(w, http.StatusForbidden, "forbidden for org")
			return
		}
		orgID = authorizedOrgID
	}
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org_id required")
		return
	}

	watches, err := h.store.ListWatchesByProvider(r.Context(), orgID, h.provider.ID())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if watches == nil {
		watches = []*models.FormWatch{}
	}
	writeJSON(w, http.StatusOK, watches)
}

func (h *Handler) getWatch(w http.ResponseWriter, r *http.Request, id string) {
	authorizedOrgID := h.orgID(r.Context())
	watch, err := h.store.GetWatch(r.Context(), id)
	if err != nil {
		if errors.Is(err, primitive.ErrInvalidHex) {
			writeError(w, http.StatusBadRequest, "invalid watch id")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if watch == nil {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}
	if h.provider != nil && strings.TrimSpace(watch.Provider) != h.provider.ID() {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}
	if authorizedOrgID != "" && strings.TrimSpace(watch.OrgID) != authorizedOrgID {
		writeError(w, http.StatusForbidden, "forbidden for org")
		return
	}
	writeJSON(w, http.StatusOK, watch)
}

func (h *Handler) updateWatch(w http.ResponseWriter, r *http.Request, id string) {
	authorizedOrgID := h.orgID(r.Context())
	existing, err := h.store.GetWatch(r.Context(), id)
	if err != nil {
		if errors.Is(err, primitive.ErrInvalidHex) {
			writeError(w, http.StatusBadRequest, "invalid watch id")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}
	if h.provider != nil && strings.TrimSpace(existing.Provider) != h.provider.ID() {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}
	if authorizedOrgID != "" && strings.TrimSpace(existing.OrgID) != authorizedOrgID {
		writeError(w, http.StatusForbidden, "forbidden for org")
		return
	}

	var patch struct {
		Active       *bool             `json:"active"`
		FieldMapping map[string]string `json:"field_mapping"`
		WorkflowID   string            `json:"workflow_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if patch.Active != nil {
		existing.Active = *patch.Active
	}
	if patch.FieldMapping != nil {
		existing.FieldMapping = patch.FieldMapping
	}
	if patch.WorkflowID != "" {
		existing.WorkflowID = patch.WorkflowID
	}

	if err := h.store.UpdateWatch(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *Handler) deleteWatch(w http.ResponseWriter, r *http.Request, id string) {
	authorizedOrgID := h.orgID(r.Context())
	existing, err := h.store.GetWatch(r.Context(), id)
	if err != nil {
		if errors.Is(err, primitive.ErrInvalidHex) {
			writeError(w, http.StatusBadRequest, "invalid watch id")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}
	if h.provider != nil && strings.TrimSpace(existing.Provider) != h.provider.ID() {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}
	if authorizedOrgID != "" && strings.TrimSpace(existing.OrgID) != authorizedOrgID {
		writeError(w, http.StatusForbidden, "forbidden for org")
		return
	}
	if err := h.store.DeleteWatch(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getOAuthClientOrFail(w http.ResponseWriter, r *http.Request, orgID string) (*http.Client, bool) {
	if h.provider == nil {
		writeError(w, http.StatusServiceUnavailable, "integration provider unavailable")
		return nil, false
	}
	client, err := h.provider.GetClient(r.Context(), orgID)
	if err == nil {
		return client, true
	}
	if h.provider.IsNotConfiguredError(err) {
		writeError(w, http.StatusServiceUnavailable, "Google Forms integration is not configured yet. Ask a platform admin to set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URI.")
		return nil, false
	}
	if h.provider.IsNotConnectedError(err) || h.provider.IsReconnectRequiredError(err) {
		writeError(w, http.StatusUnauthorized, "Google connection expired or became invalid. Please reconnect Google Forms from the Integrations page.")
		return nil, false
	}
	log.Printf("googleforms.getOAuthClientOrFail failed org_id=%q: %v", orgID, err)
	writeError(w, http.StatusInternalServerError, "internal error fetching integration OAuth client")
	return nil, false
}

func (h *Handler) orgID(ctx context.Context) string {
	if h.extractOrgID == nil {
		return ""
	}
	return strings.TrimSpace(h.extractOrgID(ctx))
}

func detectFormFieldType(question googleapi.Question) string {
	if question.TextQuestion != nil {
		if question.TextQuestion.Paragraph {
			return "paragraph"
		}
		return "text"
	}
	if question.ChoiceQuestion != nil {
		switch strings.ToUpper(strings.TrimSpace(question.ChoiceQuestion.Type)) {
		case "RADIO":
			return "choice"
		case "CHECKBOX":
			return "checkbox"
		case "DROP_DOWN":
			return "dropdown"
		default:
			return "choice"
		}
	}
	if question.DateQuestion != nil {
		return "date"
	}
	if question.TimeQuestion != nil {
		return "time"
	}
	if question.ScaleQuestion != nil {
		return "scale"
	}
	if question.FileUploadQuestion != nil {
		return "file"
	}
	return "text"
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
