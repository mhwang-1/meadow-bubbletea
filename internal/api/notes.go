package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleReadNotes(w http.ResponseWriter, r *http.Request) {
	content, err := s.svc.ReadNotes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"content": content})
}

func (s *Server) handleWriteNotes(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := s.svc.WriteNotes(body.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
