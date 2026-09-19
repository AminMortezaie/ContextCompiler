package eval

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/arms"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/llm"
)

func TestHarnessRunsAllArms(t *testing.T) {
	store, task := fixture.Day0()
	client := llm.NewMock()
	var buf bytes.Buffer
	h := &Harness{
		Store:       store,
		Task:        task,
		Arms:        []arms.Arm{arms.NewArmA(client), arms.NewArmB(client, nil, nil, 0), arms.NewArmC(client)},
		TokenBudget: arms.DefaultTokenBudget,
		Out:         &buf,
	}
	metrics, err := h.RunAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 3 {
		t.Fatalf("want 3 metrics, got %d", len(metrics))
	}
	out := buf.String()
	if !strings.Contains(out, "Comparison Table") {
		t.Fatal("missing comparison table")
	}
	if !strings.Contains(out, "compile_ms:") {
		t.Fatal("missing compile_ms")
	}
	if !strings.Contains(out, "llm_ms:") {
		t.Fatal("missing llm_ms")
	}
	if !strings.Contains(out, "overhead_pct_of_e2e_latency") {
		t.Fatal("missing overhead_pct_of_e2e_latency")
	}
	if !strings.Contains(out, "Overhead (compile vs e2e latency)") {
		t.Fatal("missing overhead table")
	}
	for _, name := range []string{"A:full-dump", "B:rag", "C:compiler"} {
		// B may be B:rag-fallback when no vector store in this test.
		if name == "B:rag" {
			if !strings.Contains(out, "B:rag") {
				t.Errorf("missing arm B in output")
			}
			continue
		}
		if !strings.Contains(out, name) {
			t.Errorf("missing arm %s in output", name)
		}
	}
}

func TestScoreRecall(t *testing.T) {
	task := fixture.Task{
		RelevantIDs:     []string{"a", "b", "c"},
		RequiredPhrases: []string{"ok"},
	}
	res := arms.RunResult{
		ArmName:      "test",
		RetrievedIDs: []string{"a", "b", "c", "z"},
		SelectedIDs:  []string{"a", "x", "b"},
		Answer:       "ok",
		InputTokens:  10,
		OutputTokens: 5,
		TotalTokens:  15,
	}
	m := Score(res, task)
	if !m.TaskSuccess {
		t.Fatal("expected success")
	}
	if m.RetrievalRecall < 0.99 {
		t.Fatalf("retrieval recall=%v want 1.0", m.RetrievalRecall)
	}
	if m.ContextRecall < 0.66 || m.ContextRecall > 0.67 {
		t.Fatalf("context recall=%v want ~0.666", m.ContextRecall)
	}
	if m.IrrelevantStateRatio < 0.33 || m.IrrelevantStateRatio > 0.34 {
		t.Fatalf("irr=%v want ~0.333", m.IrrelevantStateRatio)
	}
}

func TestScoreOverhead(t *testing.T) {
	task := fixture.Task{RelevantIDs: []string{"a"}, RequiredPhrases: []string{"ok"}}
	res := arms.RunResult{
		ArmName: "test", SelectedIDs: []string{"a"}, Answer: "ok",
		InputTokens: 100, OutputTokens: 50, TotalTokens: 150,
		Latency: 100 * time.Millisecond, CompileLatency: 20 * time.Millisecond, LLMLatency: 80 * time.Millisecond,
	}
	m := Score(res, task)
	if m.CompileMS != 20 || m.LLMMS != 80 {
		t.Fatalf("timing split compile=%d llm=%d", m.CompileMS, m.LLMMS)
	}
	if m.OverheadPctOfE2ELat < 19.9 || m.OverheadPctOfE2ELat > 20.1 {
		t.Fatalf("lat overhead=%v want ~20", m.OverheadPctOfE2ELat)
	}
}
