package compiler

import (
	"context"

	"github.com/aminmortezaie/contextcompiler/internal/embed"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/permissions"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// Candidate source labels written into audit provenance.
const (
	SourceLexical    = "lexical"
	SourceVector     = "vector"
	SourceGraph      = "graph"
	SourceExpand     = "expand"
	SourcePermission = "permission"
)

// BudgetUsage reports token budget consumption for the compiled context.
type BudgetUsage struct {
	TokenBudget int `json:"token_budget"`
	TokensUsed  int `json:"tokens_used"`
}

// Sufficiency is a deterministic contract-coverage check (no LLM).
type Sufficiency struct {
	Sufficient bool               `json:"sufficient"`
	Missing    []string           `json:"missing,omitempty"`
	Checks     []SufficiencyCheck `json:"checks,omitempty"`
}

// SufficiencyCheck is one coverage predicate.
type SufficiencyCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// PackedEdge is an edge that consumed budget in the compiled pack.
type PackedEdge struct {
	ID     string `json:"id"`
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Rel    string `json:"rel"`
	Fact   string `json:"fact,omitempty"`
}

// Result is the output of a compile pass (no LLM).
type Result struct {
	Contract      TaskContract `json:"contract"`
	Context       string       `json:"compiled_context"`
	RetrievedIDs  []string     `json:"retrieved_ids,omitempty"` // ranked candidates before budget-fit
	SelectedIDs   []string     `json:"selected_ids"`
	ExcludedIDs   []string     `json:"excluded_ids"`
	Audit         []AuditEntry `json:"audit"`
	Budget        BudgetUsage  `json:"budget_usage"`
	Sufficiency   Sufficiency  `json:"sufficiency"`
	SelectedEdges []PackedEdge `json:"selected_edges,omitempty"`
}

// Options configures compile, budget, permission hooks, graph, and ablation toggles.
type Options struct {
	TokenBudget int
	Permissions permissions.Policy
	Ablation    Ablation

	// Optional graph + handle. When Graph is nil, compile synthesizes RelRelated
	// edges from Entity.RefIDs (no hardcoded hub IDs).
	Graph  memory.GraphStore
	Handle string

	// Optional embedder for the vector candidate union. Nil → hash-bow-384.
	Embedder embed.Provider

	// Retrieve knobs (defaults: TopK=32, Hops=1, cap 3).
	TopK int
	Hops int

	Context context.Context
}

type scoredEnt struct {
	e       state.Entity
	score   float64
	reason  string
	source  string
	hops    int
	edgeIDs []string
}

type candidate struct {
	e      state.Entity
	source string // first stage that produced this ID
	hops   int
	lex    float64
	vec    float64
}
