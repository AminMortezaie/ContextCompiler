package arms

import (
	"context"
	"fmt"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/llm"
	"github.com/aminmortezaie/contextcompiler/internal/state"
	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// ArmA is the naïve baseline: the full org-state corpus is the candidate pool;
// entities are concatenated in store order until the shared tokenBudget (same cap as B/C), then LLM.
type ArmA struct {
	LLM llm.Client
}

func NewArmA(client llm.Client) *ArmA { return &ArmA{LLM: client} }

func (a *ArmA) Name() string { return "A:full-dump" }

func (a *ArmA) Run(ctx context.Context, store *state.Store, task fixture.Task, tokenBudget int) (RunResult, error) {
	start := time.Now()

	all := store.All()
	packed, selected := PackEntities(all, tokenBudget)
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
		RetrievedIDs:   store.IDs(),
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
		PackedTokens:   tokens.Estimate(packed),
		Notes:          "full corpus candidates; insertion-order pack to shared budget",
	}, nil
}
