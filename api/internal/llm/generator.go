package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Generator struct {
	apiKey string
	model  string
	client *http.Client
}

type Question struct {
	Title   string   `json:"title"`
	Text    string   `json:"text"`
	Topic   string   `json:"topic"`
	Choices []string `json:"choices,omitempty"`
}

func NewGenerator(apiKey, model string) *Generator {
	return &Generator{apiKey: apiKey, model: model, client: &http.Client{Timeout: 30 * time.Second}}
}

func (g *Generator) GenerateQuestion(ctx context.Context) (Question, error) {
	sys := `You generate a single factual trivia question as strict JSON. The question must be specific, factual, and verifiable (no opinions). Avoid yes/no. Question text length ~100-160 chars. Output ONLY strict JSON with fields: {"title", "text", "topic", "choices"}. The "choices" array must contain 1-5 direct aliases or exact surface forms for the correct answer. Each choice should be 1-3 words, contain no descriptions or roles (e.g., avoid "first female UK PM"), and only include valid synonyms, alternate spellings, or common epithets. Do not include any prose or Markdown.`
	user := `Create a novel, accurate trivia question (history, science, geography, arts, or technology). Ensure the choices array contains only the explicit answer name and its close aliases; if no aliases exist, repeat the canonical name once.`
	body := map[string]any{
		"model":       g.model,
		"temperature": 0.7,
		"messages": []map[string]any{
			{"role": "system", "content": sys},
			{"role": "user", "content": user},
		},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	resp, err := g.client.Do(req)
	if err != nil {
		return Question{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return Question{}, fmt.Errorf("llm status %d: %s", resp.StatusCode, string(data))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Question{}, err
	}
	if len(out.Choices) == 0 {
		return Question{}, errors.New("no choices")
	}
	var q Question
	if err := json.Unmarshal([]byte(extractJSON(out.Choices[0].Message.Content)), &q); err != nil {
		return Question{}, err
	}
	return q, nil
}
