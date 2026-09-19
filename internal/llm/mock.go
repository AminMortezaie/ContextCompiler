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

// Generate inspects the packed context for known Project X narrative signals and
// returns a deterministic answer shaped to the question.
func (m *Mock) Generate(_ context.Context, req Request) (Response, error) {
	prompt := req.System + "\n" + req.Prompt
	inTok := tokens.Estimate(prompt)
	q := extractQuestion(req.Prompt)
	text := answerForQuestion(q, prompt)
	outTok := tokens.Estimate(text)
	return Response{Text: text, InputTokens: inTok, OutputTokens: outTok}, nil
}

func extractQuestion(prompt string) string {
	const marker = "QUESTION:\n"
	i := strings.LastIndex(prompt, marker)
	if i < 0 {
		return strings.ToLower(prompt)
	}
	rest := prompt[i+len(marker):]
	if j := strings.Index(rest, "\n\n"); j >= 0 {
		rest = rest[:j]
	}
	return strings.ToLower(strings.TrimSpace(rest))
}

func answerForQuestion(q, prompt string) string {
	switch {
	case strings.Contains(q, "why was") && strings.Contains(q, "who made"):
		return compositeDelayAnswer(prompt)
	case strings.Contains(q, "schedule slip") || strings.Contains(q, "what caused"):
		return causalAnswer(prompt)
	case strings.Contains(q, "decision id") || (strings.Contains(q, "approved") && strings.Contains(q, "delay")):
		return decisionAnswer(prompt)
	case strings.Contains(q, "owns implementing") || (strings.Contains(q, "who owns") && strings.Contains(q, "contract freeze")):
		return ownershipAnswer(prompt)
	case strings.Contains(q, "prioritized action"):
		return actionAnswer(prompt)
	default:
		return compositeDelayAnswer(prompt)
	}
}

func causalAnswer(prompt string) string {
	if containsAny(prompt, "API redesign", "api redesign", "scope creep from API redesign", "scope creep") {
		return "Project X slipped two sprints because of scope creep from the API redesign."
	}
	if containsAny(prompt, "Project X", "proj-x") {
		return "Project X is delayed but the root cause is unclear from context."
	}
	return "Insufficient context to explain the schedule slip."
}

func decisionAnswer(prompt string) string {
	if containsAny(prompt, "dec-001", "Sarah Chen", "VP Engineering") {
		return "Sarah Chen approved accepting the Project X delay; decision ID dec-001."
	}
	return "No decision-maker or decision ID found in context."
}

func ownershipAnswer(prompt string) string {
	if containsAny(prompt, "Marcus Lee", "usr-marcus", "tkt-042", "Backend Tech Lead") {
		return "Marcus Lee owns implementing the auth service contract freeze (ticket tkt-042)."
	}
	return "Ownership for the auth freeze is not clear from context."
}

func actionAnswer(prompt string) string {
	if containsAny(prompt, "tkt-042", "freeze the auth", "auth service contract freeze") {
		return "Backend should freeze the auth service API contract per tkt-042 and wait for redesign sign-off."
	}
	return "No prioritized backend action is specified in context."
}

func compositeDelayAnswer(prompt string) string {
	var parts []string

	if containsAny(prompt, "API redesign", "api redesign", "scope creep from API redesign") {
		parts = append(parts, "Project X was delayed due to scope creep from the API redesign.")
	} else if containsAny(prompt, "Project X", "project-x", "proj-x") {
		parts = append(parts, "Project X appears delayed, but the specific root cause is not clear from the provided context.")
	} else {
		parts = append(parts, "Insufficient context to determine why Project X was delayed.")
	}

	if containsAny(prompt, "dec-001", "Sarah Chen", "VP Engineering", "approve the delay") {
		parts = append(parts, "Sarah Chen (VP Engineering) made the decision to accept the delay (decision dec-001).")
	} else if containsAny(prompt, "Decision", "decision") {
		parts = append(parts, "A decision exists but the decision-maker is unclear from context.")
	} else {
		parts = append(parts, "No clear decision-maker identified in context.")
	}

	if containsAny(prompt, "tkt-042", "backend freeze", "freeze the auth", "Auth service contract freeze") {
		parts = append(parts, "Backend should freeze the auth service API contract per ticket tkt-042 and await the redesign sign-off.")
	} else if containsAny(prompt, "backend", "Backend") {
		parts = append(parts, "Backend should investigate further; specific ticket guidance is missing from context.")
	} else {
		parts = append(parts, "No backend action specified in context.")
	}

	return strings.Join(parts, " ")
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
