package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"qotd/api/internal/db"
	"qotd/api/internal/llm"
	txt "qotd/api/internal/text"
)

type Server struct {
	repo       *db.Repository
	grader     *llm.Grader
	embedder   *llm.Embedder
	generator  *llm.Generator
	cronKey    string
	gradeModel string
	embedModel string
}

const similarityThreshold = 0.6

func main() {
	ctx := context.Background()
	dbURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/qotd?sslmode=disable")
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	repo := db.NewRepository(pool)

	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Println("warning: OPENAI_API_KEY not set; LLM calls will fail at runtime")
	}
	embedModel := getenv("OPENAI_EMBED_MODEL", "text-embedding-3-small")
	gradeModel := getenv("OPENAI_GRADE_MODEL", "gpt-4o-mini")

	s := &Server{
		repo:       repo,
		grader:     llm.NewGrader(openaiKey, gradeModel),
		embedder:   llm.NewEmbedder(openaiKey, embedModel),
		generator:  llm.NewGenerator(openaiKey, gradeModel),
		cronKey:    os.Getenv("CRON_KEY"),
		gradeModel: gradeModel,
		embedModel: embedModel,
	}

	r := chi.NewRouter()
	r.Use(simpleCORS)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	r.Get("/v1/question/today", s.handleGetToday)
	r.Post("/v1/answers", s.handlePostAnswer)
	r.Post("/v1/admin/generate-today", s.handleGenerateToday)
	r.Options("/*", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	addr := getenv("ADDR", ":8080")
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

func (s *Server) handleGetToday(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, err := s.repo.GetLatestQuestion(ctx)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no question yet"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
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

type postAnswerRequest struct {
	QuestionID string `json:"question_id"`
	Text       string `json:"text"`
}

func (s *Server) handlePostAnswer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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

	q, err := s.repo.GetQuestionByID(ctx, req.QuestionID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "question not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	if len(q.Choices) > 0 {
		if matchesChoice(req.Text, q.Choices) {
			const score = 10
			const feedback = "Accepted choice."
			if err := s.repo.InsertAnswer(ctx, req.QuestionID, req.Text, score, nil, feedback); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"score":    score,
				"feedback": feedback,
			})
			return
		}

		grade, err := s.grader.Grade(ctx, req.Text, q.Choices)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "grading failed"})
			return
		}
		score := 0
		feedback := grade.Reason
		if feedback == "" {
			feedback = "Answer not recognized as acceptable."
		}
		if grade.Match {
			score = 10
			if feedback == "" {
				feedback = "Accepted choice."
			}
		}
		if err := s.repo.InsertAnswer(ctx, req.QuestionID, req.Text, score, nil, feedback); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"score":    score,
			"feedback": feedback,
		})
		return
	}

	grade, err := s.grader.Grade(ctx, req.Text, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "grading failed"})
		return
	}
	score := 0
	feedback := grade.Reason
	if grade.Match {
		score = 10
		if feedback == "" {
			feedback = "Accepted answer."
		}
	}
	if feedback == "" {
		feedback = "Answer not recognized."
	}
	if err := s.repo.InsertAnswer(ctx, req.QuestionID, req.Text, score, nil, feedback); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"score":    score,
		"feedback": feedback,
	})
}

// matchesChoice returns true if the user input corresponds to any accepted choice.
// It supports direct text matches (case-insensitive), ignores leading articles, and
// accepts simple alphanumeric labels (A, B, 1, 2, etc.).
func matchesChoice(input string, choices []string) bool {
	if len(choices) == 0 {
		return false
	}
	normInput := normalizeAnswer(input)
	if normInput != "" {
		for _, c := range choices {
			if normInput == normalizeAnswer(c) {
				return true
			}
		}
	}
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return false
	}
	firstRune, _ := utf8.DecodeRuneInString(trimmed)
	if firstRune >= 'a' && firstRune <= 'z' {
		idx := int(firstRune - 'a')
		return idx >= 0 && idx < len(choices)
	}
	if firstRune >= '1' && firstRune <= '9' {
		idx := int(firstRune - '1')
		return idx >= 0 && idx < len(choices)
	}
	return false
}

func normalizeAnswer(s string) string {
	if s == "" {
		return ""
	}
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "the ")
	s = strings.TrimPrefix(s, "an ")
	s = strings.TrimPrefix(s, "a ")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sanitizeChoices(question string, choices []string) []string {
	if len(choices) == 0 {
		return choices
	}
	questionNorm := strings.ReplaceAll(txt.NormalizeQuestion(question), " ", "")
	seen := make(map[string]struct{}, len(choices))
	filtered := make([]string, 0, len(choices))
	for _, choice := range choices {
		clean := strings.TrimSpace(choice)
		if clean == "" {
			continue
		}
		if len(strings.Fields(clean)) > 4 {
			continue
		}
		norm := txt.NormalizeAnswer(clean)
		if norm == "" {
			continue
		}
		if questionNorm != "" && len(norm) > 0 && strings.Contains(questionNorm, norm) && len(strings.Fields(clean)) > 1 {
			// Likely descriptive phrase lifted from the question
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		filtered = append(filtered, clean)
	}
	if len(filtered) == 0 && len(choices) > 0 {
		primary := strings.TrimSpace(choices[0])
		if primary != "" {
			filtered = append(filtered, primary)
		}
	}
	return filtered
}

func (s *Server) handleGenerateToday(w http.ResponseWriter, r *http.Request) {
	if s.cronKey == "" || r.Header.Get("X-CRON-KEY") != s.cronKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	ctx := r.Context()
	const maxTries = 5
	for i := 0; i < maxTries; i++ {
		log.Printf("[generate] attempt %d/%d", i+1, maxTries)
		q, err := s.generator.GenerateQuestion(ctx)
		if err != nil {
			log.Printf("[generate] llm error: %v", err)
			continue
		}
		if len(q.Text) < 20 || len(q.Text) > 400 {
			log.Printf("[generate] text length out of range: %d", len(q.Text))
			continue
		}
		if len(q.Choices) == 0 {
			log.Printf("[generate] no choices returned")
			continue
		}
		q.Choices = sanitizeChoices(q.Text, q.Choices)
		if len(q.Choices) == 0 {
			log.Printf("[generate] choices rejected after sanitization")
			continue
		}
		normalizedChoices := txt.NormalizedChoices(q.Choices)
		choiceSig := txt.ChoiceSignature(q.Choices)
		if len(normalizedChoices) > 0 {
			overlap, err := s.repo.HasChoiceOverlap(ctx, normalizedChoices)
			if err != nil {
				log.Printf("[generate] db error HasChoiceOverlap: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
				return
			}
			if overlap {
				log.Printf("[generate] duplicate choices overlap detected")
				continue
			}
		}
		if choiceSig != "" {
			exists, err := s.repo.ExistsQuestionByChoiceSignature(ctx, choiceSig)
			if err != nil {
				log.Printf("[generate] db error ExistsQuestionByChoiceSignature: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
				return
			}
			if exists {
				log.Printf("[generate] duplicate choice signature detected")
				continue
			}
		}
		norm := txt.NormalizeQuestion(q.Text)
		sha := txt.SHA256Hex(norm)
		exists, err := s.repo.ExistsQuestionBySHA(ctx, sha)
		if err != nil {
			log.Printf("[generate] db error ExistsQuestionBySHA: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
			return
		}
		if exists {
			log.Printf("[generate] duplicate sha detected")
			continue
		}

		emb, err := s.embedder.Embed(ctx, q.Text)
		if err != nil || len(emb) == 0 {
			if err != nil {
				log.Printf("[generate] embed error: %v", err)
			} else {
				log.Printf("[generate] embed empty result")
			}
			continue
		}
		maxSim, err := s.repo.MaxSimilarity(ctx, emb)
		if err != nil {
			log.Printf("[generate] db error MaxSimilarity: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
			return
		}
		if maxSim >= similarityThreshold {
			log.Printf("[generate] too similar: sim=%.3f", maxSim)
			continue
		}

		saved, err := s.repo.InsertQuestion(ctx, q.Title, q.Text, q.Topic, sha, emb, q.Choices, normalizedChoices, choiceSig)
		if err != nil {
			log.Printf("[generate] db error InsertQuestion: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
			return
		}
		log.Printf("[generate] inserted question id=%s sim=%.3f", saved.ID, maxSim)
		resp := map[string]any{
			"id":         saved.ID,
			"title":      saved.Title,
			"text":       saved.Text,
			"topic":      saved.Topic,
			"created_at": saved.CreatedAt,
			"similarity": strconv.FormatFloat(maxSim, 'f', 3, 64),
		}
		if len(q.Choices) > 0 {
			resp["choices"] = q.Choices
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	log.Printf("[generate] failed after %d attempts", maxTries)
	writeJSON(w, http.StatusConflict, map[string]string{"error": "could not generate novel question"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func simpleCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CRON-KEY")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
