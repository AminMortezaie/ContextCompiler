// Package llm defines the LLM client interface and mock implementation.
package llm

import "context"

// Request is a single generation call.
type Request struct {
	System string
	Prompt string
}

// Response is the model output plus usage accounting.
type Response struct {
	Text         string
	InputTokens  int
	OutputTokens int
}

// Client is the LLM abstraction. Day-0 uses Mock; Day-1+ will add a real API client.
type Client interface {
	Generate(ctx context.Context, req Request) (Response, error)
	Name() string
}
