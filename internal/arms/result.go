// Package arms implements the three experimental arms: A (full-corpus insertion-order pack),
// B (RAG), C (compiler). All arms pack context to the same tokenBudget before LLM.
package arms

import (
	"context"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// RunResult is the uniform output of every arm, consumed by the eval harness.
type RunResult struct {
	ArmName string

	// Packed context and selection
	PackedContext string
	RetrievedIDs  []string // candidates after retrieval/rank, before budget-fit packing
	SelectedIDs   []string // entity IDs present in PackedContext
	ExcludedIDs   []string
	AuditJSON     string // arm C include/exclude audit; empty for A/B

	// LLM I/O
	Answer       string
	InputTokens  int
	OutputTokens int
	TotalTokens  int

	// Timing (end-to-end and split)
	Latency        time.Duration // compile + LLM
	CompileLatency time.Duration // select/rank/pack (and retrieval for B); local work
	LLMLatency     time.Duration // Generate() wall time only

	// Stub flag (true only for degraded/fallback paths)
	IsStub bool
	Notes  string
}

// Arm is one experimental condition.
type Arm interface {
	Name() string
	Run(ctx context.Context, store *state.Store, task fixture.Task, tokenBudget int) (RunResult, error)
}

// DefaultTokenBudget is the shared packing budget for the LLM prompt (chars/4 estimator).
// The org-state corpus may be ~100K–5M tokens; every arm selects/packs within this budget only.
const DefaultTokenBudget = 2000
