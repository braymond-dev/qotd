package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"qotd/api/internal/service"
)

type postAnswerRequest struct {
	QuestionID string `json:"question_id"`
	Text       string `json:"text"`
}

func (s *Server) handleGetToday(w http.ResponseWriter, r *http.Request) {
	q, err := s.svc.GetToday(r.Context())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNoQuestion):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no question yet"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         q.ID,
		"title":      q.Title,
		"text":       q.Text,
		"topic":      q.Topic,
		"created_at": q.CreatedAt,
		"choices":    q.Choices,
	})
}

func (s *Server) handlePostAnswer(w http.ResponseWriter, r *http.Request) {
	var req postAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.QuestionID == "" || req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question_id and text are required"})
		return
	}
	if len(req.Text) > 4000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "answer too long"})
		return
	}

	score, feedback, err := s.svc.SubmitAnswer(r.Context(), req.QuestionID, req.Text)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrQuestionNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "question not found"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"score": score, "feedback": feedback})
}
