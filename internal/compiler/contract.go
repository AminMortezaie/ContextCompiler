package compiler

import "strings"

// TaskContract is the structured view of what a task needs for context compilation.
type TaskContract struct {
	Question      string   `json:"question"`
	ProjectHints  []string `json:"project_hints,omitempty"`
	RequiredKinds []string `json:"required_kinds,omitempty"`
	Keywords      []string `json:"keywords,omitempty"`
}

// AuditEntry records why an entity was included or excluded.
type AuditEntry struct {
	ID     string  `json:"id"`
	Action string  `json:"action"` // "include" | "exclude"
	Reason string  `json:"reason"`
	Score  float64 `json:"score,omitempty"`
}

var defaultKinds = []string{
	"project", "decision", "ticket", "user", "conversation", "task", "team",
}

// BuildContractFromQuestion heuristically derives a contract from natural-language task text.
// Callers may pass a fully specified TaskContract instead.
func BuildContractFromQuestion(question string) TaskContract {
	q := strings.ToLower(question)
	var keywords []string
	for _, kw := range []string{
		"project x", "delay", "delayed", "decision", "backend", "ticket",
		"auth", "api redesign", "freeze", "sso", "scope creep",
		"schedule slip", "slip", "caused", "cause", "approved", "approval",
		"decision id", "owns", "owner", "ownership", "responsible", "implementing",
		"prioritized", "action", "right now",
	} {
		if strings.Contains(q, kw) {
			keywords = append(keywords, kw)
		}
	}
	var projects []string
	if strings.Contains(q, "project x") {
		projects = append(projects, "project x", "proj-x")
	}
	kinds := append([]string(nil), defaultKinds...)
	if strings.Contains(q, "who") || strings.Contains(q, "owner") || strings.Contains(q, "responsible") {
		kinds = appendUniqueKind(kinds, "user", "team")
	}
	if strings.Contains(q, "decision") || strings.Contains(q, "approved") {
		kinds = appendUniqueKind(kinds, "decision")
	}
	if strings.Contains(q, "action") || strings.Contains(q, "freeze") || strings.Contains(q, "ticket") {
		kinds = appendUniqueKind(kinds, "ticket", "task")
	}
	return TaskContract{
		Question:      question,
		ProjectHints:  projects,
		RequiredKinds: kinds,
		Keywords:      keywords,
	}
}

func appendUniqueKind(kinds []string, add ...string) []string {
	seen := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		seen[k] = true
	}
	for _, k := range add {
		if !seen[k] {
			kinds = append(kinds, k)
			seen[k] = true
		}
	}
	return kinds
}

// Normalize fills in a contract when optional fields are omitted.
func (c TaskContract) Normalize() TaskContract {
	out := c
	if out.Question == "" {
		return out
	}
	if len(out.RequiredKinds) == 0 && len(out.Keywords) == 0 && len(out.ProjectHints) == 0 {
		return BuildContractFromQuestion(out.Question)
	}
	if len(out.RequiredKinds) == 0 {
		out.RequiredKinds = append([]string(nil), defaultKinds...)
	}
	return out
}
