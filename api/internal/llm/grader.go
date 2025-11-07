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

type Grader struct {
	apiKey string
	model  string
	client *http.Client
}

type GradeResult struct {
	Match  bool   `json:"match"`
	Reason string `json:"reason"`
	Choice string `json:"matched_choice"`
}

func NewGrader(apiKey, model string) *Grader {
	return &Grader{apiKey: apiKey, model: model, client: &http.Client{Timeout: 30 * time.Second}}
}

func (g *Grader) Grade(ctx context.Context, answer string, choices []string) (GradeResult, error) {
	if len(choices) == 0 {
		return GradeResult{Match: false, Reason: "no choices configured"}, nil
	}
	payload := g.buildPayload(answer, choices)
	res, err := g.call(ctx, payload)
	if err == nil {
		return res, nil
	}
	var parseErr *json.SyntaxError
	if errors.As(err, &parseErr) || errors.Is(err, io.ErrUnexpectedEOF) {
		return g.call(ctx, payload)
	}
	return GradeResult{}, err
}

func (g *Grader) buildPayload(answer string, choices []string) map[string]any {
	sys := "You verify if a response matches any exact item in a provided list of acceptable answers. Only acknowledge exact equivalence, never approximate matches. Return strict JSON only."
	user := map[string]any{
		"answer":       answer,
		"choices":      choices,
		"instructions": "Return match=true only if the answer clearly references the same entity as one of the choices (allowing spelling/spacing variants). When match=true, set matched_choice to the exact string from the choices array. Reject other entities even if similar. Always provide a short reason.",
		"output_format": map[string]any{
			"match":          false,
			"matched_choice": "",
			"reason":         "",
		},
	}
	return map[string]any{
		"model":       g.model,
		"temperature": 0,
		"messages": []map[string]any{
			{"role": "system", "content": sys},
			{"role": "user", "content": mustJSON(user)},
		},
	}
}

func (g *Grader) call(ctx context.Context, body map[string]any) (GradeResult, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	resp, err := g.client.Do(req)
	if err != nil {
		return GradeResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return GradeResult{}, fmt.Errorf("llm status %d: %s", resp.StatusCode, string(data))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return GradeResult{}, err
	}
	if len(out.Choices) == 0 {
		return GradeResult{}, errors.New("no choices returned")
	}
	content := out.Choices[0].Message.Content
	var gr GradeResult
	if err := json.Unmarshal([]byte(extractJSON(content)), &gr); err != nil {
		return GradeResult{}, err
	}
	return gr, nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func extractJSON(s string) string {
	i := bytes.IndexByte([]byte(s), '{')
	j := bytes.LastIndexByte([]byte(s), '}')
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}
