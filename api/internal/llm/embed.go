package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type Embedder struct {
    apiKey string
    model  string
    client *http.Client
}

func NewEmbedder(apiKey, model string) *Embedder {
    return &Embedder{apiKey: apiKey, model: model, client: &http.Client{Timeout: 20 * time.Second}}
}

func (e *Embedder) Embed(ctx context.Context, input string) ([]float32, error) {
    body := map[string]any{
        "model": e.model,
        "input": input,
    }
    b, _ := json.Marshal(body)
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/embeddings", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+e.apiKey)
    resp, err := e.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode/100 != 2 {
        return nil, fmt.Errorf("embed status %d", resp.StatusCode)
    }
    var out struct {
        Data []struct{ Embedding []float32 `json:"embedding"` } `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
        return nil, err
    }
    if len(out.Data) == 0 {
        return nil, fmt.Errorf("no embedding")
    }
    return out.Data[0].Embedding, nil
}

