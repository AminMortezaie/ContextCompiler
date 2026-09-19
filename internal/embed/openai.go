package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// OpenAIProvider calls an OpenAI-compatible /v1/embeddings endpoint.
// Vectors are truncated or zero-padded to Dim (384) so they fit the Day-1
// pgvector schema. Prefer HashProvider for offline reproducibility.
type OpenAIProvider struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewOpenAI(apiKey, baseURL, model string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIProvider{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string { return "openai:" + p.Model }
func (p *OpenAIProvider) Dim() int     { return Dim }

type embReq struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embResp struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *OpenAIProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, _ := json.Marshal(embReq{Model: p.Model, Input: texts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var parsed embResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("embeddings API: %s", parsed.Error.Message)
	}
	out := make([][]float32, len(texts))
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= len(out) {
			continue
		}
		out[d.Index] = fitDim(d.Embedding, Dim)
	}
	for i := range out {
		if out[i] == nil {
			out[i] = make([]float32, Dim)
		}
	}
	return out, nil
}

func fitDim(src []float64, dim int) []float32 {
	out := make([]float32, dim)
	n := len(src)
	if n > dim {
		n = dim
	}
	for i := 0; i < n; i++ {
		out[i] = float32(src[i])
	}
	return l2normalize(out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// NewFromEnv returns HashProvider by default. If OPENAI_API_KEY is set and
// CC_EMBED_PROVIDER=openai, returns OpenAIProvider.
func NewFromEnv() Provider {
	if os.Getenv("CC_EMBED_PROVIDER") == "openai" {
		key := os.Getenv("OPENAI_API_KEY")
		if key != "" {
			return NewOpenAI(key, os.Getenv("OPENAI_BASE_URL"), os.Getenv("OPENAI_EMBED_MODEL"))
		}
	}
	return NewHash()
}
