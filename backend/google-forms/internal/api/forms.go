package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/example/business-automation/backend/google-forms/internal/googleapi"
)

func (s *Server) handleForms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s.createForm(w, r)
}

// handleFormByPath routes:
//
//	GET /forms/{formId}              → getForm
//	GET /forms/{formId}/responses    → listFormResponses
func (s *Server) handleFormByPath(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/forms/"), "/", 2)
	formID := parts[0]
	if formID == "" {
		writeError(w, http.StatusBadRequest, "form_id required")
		return
	}
	if len(parts) > 1 && parts[1] == "responses" {
		s.listFormResponses(w, r, formID)
		return
	}
	s.getForm(w, r, formID)
}

func (s *Server) createForm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrgID     string               `json:"org_id"`
		Title     string               `json:"title"`
		Questions []googleapi.FormItem `json:"questions"`
		Publish   bool                 `json:"publish"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.OrgID == "" || req.Title == "" {
		writeError(w, http.StatusBadRequest, "org_id and title are required")
		return
	}

	client, err := s.oauthSvc.GetClient(r.Context(), req.OrgID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	form, err := googleapi.CreateForm(client, req.Title)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if len(req.Questions) > 0 {
		if err := googleapi.AddQuestions(client, form.FormID, req.Questions); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	}

	if req.Publish {
		if err := googleapi.SetPublished(client, form.FormID, true); err != nil {
			log.Printf("warn: set published for form %s: %v", form.FormID, err)
		}
	}

	updated, err := googleapi.GetForm(client, form.FormID)
	if err != nil {
		writeJSON(w, http.StatusCreated, form)
		return
	}
	writeJSON(w, http.StatusCreated, updated)
}

func (s *Server) getForm(w http.ResponseWriter, r *http.Request, formID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org_id required")
		return
	}
	client, err := s.oauthSvc.GetClient(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
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
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org_id required")
		return
	}
	client, err := s.oauthSvc.GetClient(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
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
