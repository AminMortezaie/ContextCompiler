package llm

import (
	"context"
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// Mock is a deterministic LLM stub: answers from golden substrings in the prompt.
// No network, no API keys.
type Mock struct{}

func NewMock() *Mock { return &Mock{} }

func (m *Mock) Name() string { return "mock" }

// Generate inspects the packed context for known Project X delay signals and
// returns a deterministic narrative answer.
func (m *Mock) Generate(_ context.Context, req Request) (Response, error) {
	prompt := req.System + "\n" + req.Prompt
	inTok := tokens.Estimate(prompt)

	var parts []string

	// Delay reason
	if containsAny(prompt, "API redesign", "api redesign", "scope creep from API redesign") {
		parts = append(parts, "Project X was delayed due to scope creep from the API redesign.")
	} else if containsAny(prompt, "Project X", "project-x", "proj-x") {
		parts = append(parts, "Project X appears delayed, but the specific root cause is not clear from the provided context.")
	} else {
		parts = append(parts, "Insufficient context to determine why Project X was delayed.")
	}

	// Who decided
	if containsAny(prompt, "dec-001", "Sarah Chen", "VP Engineering", "approve the delay") {
		parts = append(parts, "Sarah Chen (VP Engineering) made the decision to accept the delay (decision dec-001).")
	} else if containsAny(prompt, "Decision", "decision") {
		parts = append(parts, "A decision exists but the decision-maker is unclear from context.")
	} else {
		parts = append(parts, "No clear decision-maker identified in context.")
	}

	// Backend action
	if containsAny(prompt, "tkt-042", "backend freeze", "freeze the auth", "Auth service contract freeze") {
		parts = append(parts, "Backend should freeze the auth service API contract per ticket tkt-042 and await the redesign sign-off.")
	} else if containsAny(prompt, "backend", "Backend") {
		parts = append(parts, "Backend should investigate further; specific ticket guidance is missing from context.")
	} else {
		parts = append(parts, "No backend action specified in context.")
	}

	text := strings.Join(parts, " ")
	outTok := tokens.Estimate(text)
	return Response{Text: text, InputTokens: inTok, OutputTokens: outTok}, nil
}

func containsAny(s string, needles ...string) bool {
	lower := strings.ToLower(s)
	for _, n := range needles {
		if strings.Contains(lower, strings.ToLower(n)) {
			return true
		}
	}
	return false
}
