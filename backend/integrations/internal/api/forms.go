package api

import (
	"net/http"
)

func (s *Server) handleForms(w http.ResponseWriter, r *http.Request) {
	handler := s.googleFormsHandler()
	if handler == nil {
		writeError(w, http.StatusServiceUnavailable, "Google Forms provider is unavailable")
		return
	}
	handler.HandleForms(w, r)
}
func (s *Server) handleFormByPath(w http.ResponseWriter, r *http.Request) {
	handler := s.googleFormsHandler()
	if handler == nil {
		writeError(w, http.StatusServiceUnavailable, "Google Forms provider is unavailable")
		return
	}
	handler.HandleFormByPath(w, r)
}
