package arms

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/llm"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// ArmC is the context compiler:
// task-contract → multi-signal select/rank → budget-fit (score/token, same tokenBudget as A/B) → assemble + include/exclude audit → LLM.
// Deliberately distinct from arm B (no vector search; contract + kind priors + ref-graph boost).
type ArmC struct {
	LLM      llm.Client
	Ablation compiler.Ablation
}

func NewArmC(client llm.Client) *ArmC { return &ArmC{LLM: client} }

// NewArmCAblation constructs arm C with a single-knob ablation for science-track runs.
func NewArmCAblation(client llm.Client, ab compiler.Ablation) *ArmC {
	return &ArmC{LLM: client, Ablation: ab}
}

func (a *ArmC) Name() string {
	if lbl := a.Ablation.Label(); lbl != "" {
		return "C:compiler-" + lbl
	}
	return "C:compiler"
}

func (a *ArmC) Run(ctx context.Context, st *state.Store, task fixture.Task, tokenBudget int) (RunResult, error) {
	start := time.Now()

	contract := compiler.BuildContractFromQuestion(task.Question)
	compiled := compiler.Compile(st.All(), contract, compiler.Options{
		TokenBudget: tokenBudget,
		Ablation:    a.Ablation,
		Graph:       memory.DefaultGraph(st.All()),
		Handle:      memory.InlineHandle,
	})

	auditBytes, _ := json.Marshal(struct {
		Contract compiler.TaskContract `json:"contract"`
		Audit    []compiler.AuditEntry `json:"audit"`
	}{Contract: compiled.Contract, Audit: compiled.Audit})
	compileDur := time.Since(start)

	system := "You answer questions about organizational state using only the provided context."
	prompt := fmt.Sprintf("CONTEXT (compiled):\n%s\n\nQUESTION:\n%s\n\nAnswer concisely.", compiled.Context, task.Question)

	llmStart := time.Now()
	resp, err := a.LLM.Generate(ctx, llm.Request{System: system, Prompt: prompt})
	llmDur := time.Since(llmStart)
	if err != nil {
		return RunResult{}, err
	}

	return RunResult{
		ArmName:        a.Name(),
		PackedContext:  compiled.Context,
		RetrievedIDs:   compiled.RetrievedIDs,
		SelectedIDs:    compiled.SelectedIDs,
		ExcludedIDs:    compiled.ExcludedIDs,
		AuditJSON:      string(auditBytes),
		Answer:         resp.Text,
		InputTokens:    resp.InputTokens,
		OutputTokens:   resp.OutputTokens,
		TotalTokens:    resp.InputTokens + resp.OutputTokens,
		Latency:        time.Since(start),
		CompileLatency: compileDur,
		LLMLatency:     llmDur,
		IsStub:         false,
		PackedTokens:   compiled.Budget.TokensUsed,
		Sufficient:     compiled.Sufficiency.Sufficient,
		HasSufficiency: true,
		Notes:          "hybrid lexical∪vector∪graph → typed expand → rank → budget pack + sufficiency",
	}, nil
}
