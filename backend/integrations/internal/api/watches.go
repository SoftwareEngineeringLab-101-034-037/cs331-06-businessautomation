package api

import (
	"net/http"
)

func (s *Server) handleWatches(w http.ResponseWriter, r *http.Request) {
	handler := s.googleFormsHandler()
	if handler == nil {
		writeError(w, http.StatusServiceUnavailable, "Google Forms provider is unavailable")
		return
	}
	handler.HandleWatches(w, r)
}

func (s *Server) handleWatchByID(w http.ResponseWriter, r *http.Request) {
	handler := s.googleFormsHandler()
	if handler == nil {
		writeError(w, http.StatusServiceUnavailable, "Google Forms provider is unavailable")
		return
	}
	handler.HandleWatchByID(w, r)
}
