package arms

import (
	"context"
	"fmt"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/llm"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// ArmA is full-dump → LLM: concatenate state texts until token budget.
type ArmA struct {
	LLM llm.Client
}

func NewArmA(client llm.Client) *ArmA { return &ArmA{LLM: client} }

func (a *ArmA) Name() string { return "A:full-dump" }

func (a *ArmA) Run(ctx context.Context, store *state.Store, task fixture.Task, tokenBudget int) (RunResult, error) {
	start := time.Now()

	packed, selected := PackEntities(store.All(), tokenBudget)
	excluded := ExcludedFrom(store.IDs(), selected)
	compileDur := time.Since(start)

	system := "You answer questions about organizational state using only the provided context."
	prompt := fmt.Sprintf("CONTEXT:\n%s\n\nQUESTION:\n%s\n\nAnswer concisely.", packed, task.Question)

	llmStart := time.Now()
	resp, err := a.LLM.Generate(ctx, llm.Request{System: system, Prompt: prompt})
	llmDur := time.Since(llmStart)
	if err != nil {
		return RunResult{}, err
	}

	return RunResult{
		ArmName:        a.Name(),
		PackedContext:  packed,
		SelectedIDs:    selected,
		ExcludedIDs:    excluded,
		Answer:         resp.Text,
		InputTokens:    resp.InputTokens,
		OutputTokens:   resp.OutputTokens,
		TotalTokens:    resp.InputTokens + resp.OutputTokens,
		Latency:        time.Since(start),
		CompileLatency: compileDur,
		LLMLatency:     llmDur,
		IsStub:         false,
		Notes:          "full in-memory dump until budget",
	}, nil
}
