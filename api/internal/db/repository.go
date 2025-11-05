package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

type Question struct {
	ID        string
	Title     string
	Text      string
	Topic     string
	CreatedAt time.Time
	Choices   []string
	ChoiceSig string
}

func (r *Repository) GetLatestQuestion(ctx context.Context) (Question, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, title, text, topic, created_at, COALESCE(choices, '[]'::jsonb), choices_signature FROM questions ORDER BY created_at DESC LIMIT 1`)
	var q Question
	var choicesRaw []byte
	if err := row.Scan(&q.ID, &q.Title, &q.Text, &q.Topic, &q.CreatedAt, &choicesRaw, &q.ChoiceSig); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Question{}, ErrNotFound
		}
		return Question{}, err
	}
	if len(choicesRaw) > 0 {
		_ = json.Unmarshal(choicesRaw, &q.Choices)
	}
	return q, nil
}

func (r *Repository) ExistsQuestionBySHA(ctx context.Context, sha string) (bool, error) {
	row := r.pool.QueryRow(ctx, `SELECT 1 FROM questions WHERE sha256=$1`, sha)
	var one int
	if err := row.Scan(&one); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Repository) ExistsQuestionByChoiceSignature(ctx context.Context, sig string) (bool, error) {
	row := r.pool.QueryRow(ctx, `SELECT 1 FROM questions WHERE choices_signature=$1`, sig)
	var one int
	if err := row.Scan(&one); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func floatsToVectorLiteral(v []float32) string {
	b := strings.Builder{}
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%g", f))
	}
	b.WriteByte(']')
	return b.String()
}

// MaxSimilarity implements cosine similarity via pgvector <=> and returns max(1 - distance)
func (r *Repository) MaxSimilarity(ctx context.Context, emb []float32) (float64, error) {
	vec := floatsToVectorLiteral(emb)
	row := r.pool.QueryRow(ctx, `SELECT 1 - (embedding <=> $1::vector) AS sim FROM questions ORDER BY embedding <=> $1::vector LIMIT 1`, vec)
	var sim float64
	if err := row.Scan(&sim); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return 0, nil
		}
		return 0, err
	}
	return sim, nil
}

func (r *Repository) InsertQuestion(ctx context.Context, title, text, topic, sha string, emb []float32, choices []string, normalized []string, choiceSig string) (Question, error) {
	vec := floatsToVectorLiteral(emb)
	var choicesJSON string
	if len(choices) > 0 {
		b, _ := json.Marshal(choices)
		choicesJSON = string(b)
	} else {
		choicesJSON = "null"
	}
	var normalizedJSON string
	if len(normalized) > 0 {
		b, _ := json.Marshal(normalized)
		normalizedJSON = string(b)
	} else {
		normalizedJSON = "null"
	}
	row := r.pool.QueryRow(ctx, `INSERT INTO questions (id, title, text, topic, sha256, embedding, choices, choices_normalized, choices_signature) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5::vector, $6::jsonb, $7::jsonb, $8) RETURNING id, title, text, topic, created_at, choices_signature`, title, text, topic, sha, vec, choicesJSON, normalizedJSON, nullableText(choiceSig))
	var q Question
	if err := row.Scan(&q.ID, &q.Title, &q.Text, &q.Topic, &q.CreatedAt, &q.ChoiceSig); err != nil {
		return Question{}, err
	}
	q.Choices = choices
	return q, nil
}

func (r *Repository) GetQuestionByID(ctx context.Context, id string) (Question, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, title, text, topic, created_at, COALESCE(choices, '[]'::jsonb), choices_signature FROM questions WHERE id=$1`, id)
	var q Question
	var choicesRaw []byte
	if err := row.Scan(&q.ID, &q.Title, &q.Text, &q.Topic, &q.CreatedAt, &choicesRaw, &q.ChoiceSig); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Question{}, ErrNotFound
		}
		return Question{}, err
	}
	if len(choicesRaw) > 0 {
		_ = json.Unmarshal(choicesRaw, &q.Choices)
	}
	return q, nil
}

func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *Repository) HasChoiceOverlap(ctx context.Context, normalized []string) (bool, error) {
	if len(normalized) == 0 {
		return false, nil
	}
	row := r.pool.QueryRow(ctx, `SELECT 1 FROM questions WHERE COALESCE(choices_normalized, '[]'::jsonb) ?| $1`, normalized)
	var one int
	if err := row.Scan(&one); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Repository) InsertAnswer(ctx context.Context, questionID, text string, score int, rubric map[string]int, feedback string) error {
	rub, _ := json.Marshal(map[string]any{"rubric_scores": rubric, "total": score, "feedback": feedback})
	_, err := r.pool.Exec(ctx, `INSERT INTO answers (id, question_id, text, score, rubric_json, feedback) VALUES (gen_random_uuid(), $1, $2, $3, $4::jsonb, $5)`, questionID, text, score, string(rub), feedback)
	return err
}
