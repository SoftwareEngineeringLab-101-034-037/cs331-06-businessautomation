package api

import "net/http"

func (s *Server) handleGmailSend(w http.ResponseWriter, r *http.Request) {
	handler := s.gmailHandler()
	if handler == nil {
		writeError(w, http.StatusServiceUnavailable, "Gmail provider is unavailable")
		return
	}
	handler.HandleSend(w, r)
}

func (s *Server) handleGmailMessages(w http.ResponseWriter, r *http.Request) {
	handler := s.gmailHandler()
	if handler == nil {
		writeError(w, http.StatusServiceUnavailable, "Gmail provider is unavailable")
		return
	}
	handler.HandleMessages(w, r)
}

func (s *Server) handleGmailWatches(w http.ResponseWriter, r *http.Request) {
	handler := s.gmailHandler()
	if handler == nil {
		writeError(w, http.StatusServiceUnavailable, "Gmail provider is unavailable")
		return
	}
	handler.HandleWatches(w, r)
}

func (s *Server) handleGmailWatchByID(w http.ResponseWriter, r *http.Request) {
	handler := s.gmailHandler()
	if handler == nil {
		writeError(w, http.StatusServiceUnavailable, "Gmail provider is unavailable")
		return
	}
	handler.HandleWatchByID(w, r)
}
