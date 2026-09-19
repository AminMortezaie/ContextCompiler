package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// OpenAI is an OpenAI-compatible chat completions client.
// Env: OPENAI_API_KEY (required), OPENAI_BASE_URL (optional), OPENAI_MODEL (optional).
type OpenAI struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewOpenAI(apiKey, baseURL, model string) *OpenAI {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAI{
		APIKey:  apiKey,
		BaseURL: stringsTrimRightSlash(baseURL),
		Model:   model,
		Client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *OpenAI) Name() string { return "openai:" + c.Model }

type chatReq struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *OpenAI) Generate(ctx context.Context, req Request) (Response, error) {
	msgs := []chatMessage{}
	if req.System != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: req.System})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: req.Prompt})
	body, _ := json.Marshal(chatReq{Model: c.Model, Messages: msgs})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("chat HTTP %d: %s", resp.StatusCode, trunc(string(raw), 240))
	}
	var parsed chatResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Response{}, err
	}
	if parsed.Error != nil {
		return Response{}, fmt.Errorf("chat API: %s", parsed.Error.Message)
	}
	text := ""
	if len(parsed.Choices) > 0 {
		text = parsed.Choices[0].Message.Content
	}
	inTok, outTok := tokens.Estimate(req.System+"\n"+req.Prompt), tokens.Estimate(text)
	if parsed.Usage != nil {
		inTok = parsed.Usage.PromptTokens
		outTok = parsed.Usage.CompletionTokens
	}
	return Response{Text: text, InputTokens: inTok, OutputTokens: outTok}, nil
}

// ClientFromEnv returns Mock when OPENAI_API_KEY is unset; otherwise OpenAI client.
// Never errors solely because the key is missing.
func ClientFromEnv() Client {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		key = os.Getenv("GROQ_API_KEY")
	}
	if key == "" {
		return NewMock()
	}
	base := os.Getenv("OPENAI_BASE_URL")
	if base == "" && os.Getenv("GROQ_API_KEY") != "" {
		base = "https://api.groq.com/openai/v1"
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "openai/gpt-oss-20b"
	}
	return NewOpenAI(key, base, model)
}

func stringsTrimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
