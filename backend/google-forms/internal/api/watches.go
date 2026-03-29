package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/example/business-automation/backend/google-forms/internal/models"
)

func (s *Server) handleWatches(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createWatch(w, r)
	case http.MethodGet:
		s.listWatches(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleWatchByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/watches/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "watch id required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getWatch(w, r, id)
	case http.MethodPut:
		s.updateWatch(w, r, id)
	case http.MethodDelete:
		s.deleteWatch(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) createWatch(w http.ResponseWriter, r *http.Request) {
	var watch models.FormWatch
	if err := json.NewDecoder(r.Body).Decode(&watch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if watch.OrgID == "" || watch.FormID == "" || watch.WorkflowID == "" {
		writeError(w, http.StatusBadRequest, "org_id, form_id, and workflow_id are required")
		return
	}
	if err := s.store.SaveWatch(r.Context(), &watch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, watch)
}

func (s *Server) listWatches(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "org_id required")
		return
	}
	watches, err := s.store.ListWatches(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, watches)
}

func (s *Server) getWatch(w http.ResponseWriter, r *http.Request, id string) {
	watch, err := s.store.GetWatch(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if watch == nil {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}
	writeJSON(w, http.StatusOK, watch)
}

func (s *Server) updateWatch(w http.ResponseWriter, r *http.Request, id string) {
	existing, err := s.store.GetWatch(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "watch not found")
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

	if err := s.store.UpdateWatch(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) deleteWatch(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.store.DeleteWatch(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
