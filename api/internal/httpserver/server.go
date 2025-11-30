package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"qotd/api/internal/service"
)

type Server struct {
	svc     *service.QuestionService
	cronKey string
}

func New(svc *service.QuestionService, cronKey string) *Server {
	return &Server{svc: svc, cronKey: cronKey}
}

func (s *Server) Start(addr string) error {
	r := chi.NewRouter()
	r.Use(simpleCORS)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	r.Get("/v1/question/today", s.handleGetToday)
	r.Post("/v1/answers", s.handlePostAnswer)
	r.Post("/v1/admin/generate-today", s.handleGenerateToday)
	r.Options("/*", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	log.Printf("listening on %s", addr)
	return http.ListenAndServe(addr, r)
}
