package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/example/business-automation/backend/google-forms/internal/googleapi"
	"github.com/example/business-automation/backend/google-forms/internal/oauth"
)

func (s *Server) getOAuthClientOrFail(w http.ResponseWriter, r *http.Request, orgID string) (*http.Client, bool) {
	client, err := s.oauthSvc.GetClient(r.Context(), orgID)
	if err == nil {
		return client, true
	}
	if oauth.IsNotConfiguredError(err) {
		writeError(w, http.StatusServiceUnavailable, "Google Forms integration is not configured yet. Ask a platform admin to set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URI.")
		return nil, false
	}
	if oauth.IsNotConnectedError(err) {
		writeError(w, http.StatusUnauthorized, "Google connection expired or became invalid. Please reconnect Google Forms from the Integrations page.")
		return nil, false
	}
	if oauth.IsReconnectRequiredError(err) {
		writeError(w, http.StatusUnauthorized, "Google connection expired or became invalid. Please reconnect Google Forms from the Integrations page.")
		return nil, false
	}
	log.Printf("forms.getOAuthClientOrFail failed org_id=%q: %v", orgID, err)
	writeError(w, http.StatusInternalServerError, "internal error fetching Google OAuth client")
	return nil, false
}

func (s *Server) handleForms(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createForm(w, r)
	case http.MethodGet:
		s.listForms(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleFormByPath routes:
//
//	GET /forms/{formId}              → getForm
//	GET /forms/{formId}/responses    → listFormResponses
//	GET /forms/{formId}/fields       → listFormFields
func (s *Server) handleFormByPath(w http.ResponseWriter, r *http.Request) {
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
			s.listFormResponses(w, r, formID)
			return
		case "fields":
			s.listFormFields(w, r, formID)
			return
		default:
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
	}
	s.getForm(w, r, formID)
}

func (s *Server) createForm(w http.ResponseWriter, r *http.Request) {
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

	orgID := authorizedOrgIDFromContext(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	client, ok := s.getOAuthClientOrFail(w, r, orgID)
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
			writeJSON(w, http.StatusBadGateway, map[string]interface{}{
				"error":   "questions could not be added to created form",
				"form_id": form.FormID,
				"stage":   "add_questions",
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

func (s *Server) listForms(w http.ResponseWriter, r *http.Request) {
	orgID := authorizedOrgIDFromContext(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	client, ok := s.getOAuthClientOrFail(w, r, orgID)
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

func (s *Server) getForm(w http.ResponseWriter, r *http.Request, formID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	orgID := authorizedOrgIDFromContext(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	client, ok := s.getOAuthClientOrFail(w, r, orgID)
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

func (s *Server) listFormResponses(w http.ResponseWriter, r *http.Request, formID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	orgID := authorizedOrgIDFromContext(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	client, ok := s.getOAuthClientOrFail(w, r, orgID)
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

func (s *Server) listFormFields(w http.ResponseWriter, r *http.Request, formID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	orgID := authorizedOrgIDFromContext(r.Context())
	if orgID == "" {
		writeError(w, http.StatusUnauthorized, "org authorization required")
		return
	}

	client, ok := s.getOAuthClientOrFail(w, r, orgID)
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
