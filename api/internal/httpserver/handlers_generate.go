package httpserver

import (
	"errors"
	"net/http"
	"strconv"

	"qotd/api/internal/service"
)

func (s *Server) handleGenerateToday(w http.ResponseWriter, r *http.Request) {
	if s.cronKey == "" || r.Header.Get("X-CRON-KEY") != s.cronKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	result, err := s.svc.GenerateQuestion(r.Context())
	if err != nil {
		if errors.Is(err, service.ErrGenerateFailed) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "could not generate novel question"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	resp := map[string]any{
		"id":         result.Question.ID,
		"title":      result.Question.Title,
		"text":       result.Question.Text,
		"topic":      result.Question.Topic,
		"created_at": result.Question.CreatedAt,
		"similarity": strconv.FormatFloat(result.Similarity, 'f', 3, 64),
	}
	if len(result.Choices) > 0 {
		resp["choices"] = result.Choices
	}
	writeJSON(w, http.StatusOK, resp)
}
