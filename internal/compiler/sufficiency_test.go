package compiler_test

import (
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

func TestSufficiencyDay0CausalPass(t *testing.T) {
	st, task := fixture.Day0()
	res := compiler.Compile(st.All(), compiler.BuildContractFromQuestion(task.Question), compiler.Options{
		TokenBudget: 2000,
		Graph:       memory.DefaultGraph(st.All()),
		Handle:      memory.InlineHandle,
	})
	if !res.Sufficiency.Sufficient {
		t.Fatalf("Day-0 delay task should be sufficient: %+v", res.Sufficiency)
	}
	if len(res.SelectedEdges) == 0 {
		t.Fatal("expected packed typed edges")
	}
}

func TestSufficiencyExplicitMiss(t *testing.T) {
	ents := []state.Entity{
		{ID: "doc-1", Kind: state.KindDocument, Title: "Handbook", Text: "pto policy only"},
	}
	res := compiler.Compile(ents, compiler.TaskContract{
		Question:      "Why was Project X delayed?",
		ProjectHints:  []string{"project x"},
		Keywords:      []string{"delay", "project x"},
		RequiredKinds: []string{"decision", "ticket"},
	}, compiler.Options{TokenBudget: 200})
	if res.Sufficiency.Sufficient {
		t.Fatal("handbook-only state must be insufficient for a causal delay question")
	}
	if len(res.Sufficiency.Missing) == 0 {
		t.Fatal("expected explicit missing checks")
	}
	found := false
	for _, a := range res.Audit {
		if a.ID == "sufficiency:required_kind:decision" || a.Reason == "sufficiency miss: required_kind:decision" {
			found = true
		}
	}
	if !found {
		t.Fatalf("audit should list sufficiency miss, got %+v", res.Audit)
	}
}
