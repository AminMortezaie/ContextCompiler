package compiler_test

import (
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

func TestExpandFollowsTypedEdges(t *testing.T) {
	st, _ := fixture.Day0()
	// Contract that lexically hits proj-x only (project hint), not the freeze task.
	contract := compiler.TaskContract{
		Question:      "What is the status of Project X?",
		ProjectHints:  []string{"project x", "proj-x"},
		RequiredKinds: []string{"project", "decision", "ticket"},
		Keywords:      []string{"project x"},
	}
	res := compiler.Compile(st.All(), contract, compiler.Options{
		TokenBudget: 2000,
		Graph:       memory.DefaultGraph(st.All()),
		Handle:      memory.InlineHandle,
		Hops:        1,
		TopK:        8,
	})
	got := idSet(res.SelectedIDs)
	if !got["proj-x"] {
		t.Fatalf("seed proj-x not packed: %v", res.SelectedIDs)
	}
	// 1-hop typed neighbors of proj-x include dec-001 (decides) and tkt-042 (blocks).
	if !got["dec-001"] || !got["tkt-042"] {
		t.Fatalf("expand should pack dec-001 and tkt-042 via typed edges: %v audit=%v", res.SelectedIDs, res.Audit)
	}
	if !hasAuditSource(res.Audit, "expand") && !hasPackedRel(res, "blocks") {
		t.Fatalf("expected expand provenance or packed blocks edge; edges=%v audit=%v", res.SelectedEdges, res.Audit)
	}
}

func TestExpandRespectsNoRefExpansion(t *testing.T) {
	// Isolated seed: only "alpha" has the keyword; "beta" is reachable only via a typed edge.
	ents := []state.Entity{
		{ID: "alpha", Kind: state.KindProject, Title: "Alpha", Text: "alpha seed"},
		{ID: "beta", Kind: state.KindDecision, Title: "Beta", Text: "hidden neighbor"},
	}
	g := memory.NewStaticGraph(ents, []memory.Edge{{
		ID: "e-blocks-beta-alpha", FromID: "beta", ToID: "alpha", Rel: memory.RelBlocks, Fact: "beta blocks alpha",
	}})
	contract := compiler.TaskContract{
		Question:      "Tell me about alpha",
		Keywords:      []string{"alpha"},
		RequiredKinds: []string{"project", "decision"},
	}
	full := compiler.Compile(ents, contract, compiler.Options{TokenBudget: 500, Graph: g, Hops: 1})
	abl := compiler.Compile(ents, contract, compiler.Options{
		TokenBudget: 500, Graph: g, Hops: 1,
		Ablation: compiler.Ablation{NoRefExpansion: true},
	})
	if !idSet(full.SelectedIDs)["beta"] {
		t.Fatalf("full compile should expand to beta: %v", full.SelectedIDs)
	}
	if idSet(abl.SelectedIDs)["beta"] {
		t.Fatalf("no-ref-expansion should not add beta: %v", abl.SelectedIDs)
	}
}

func TestHopCapStopsAtTwo(t *testing.T) {
	ents := []state.Entity{
		{ID: "a", Kind: state.KindProject, Title: "A", Text: "seed a"},
		{ID: "b", Kind: state.KindTicket, Title: "B", Text: "hop1"},
		{ID: "c", Kind: state.KindDecision, Title: "C", Text: "hop2"},
		{ID: "d", Kind: state.KindTask, Title: "D", Text: "hop3"},
	}
	g := memory.NewStaticGraph(ents, []memory.Edge{
		{FromID: "a", ToID: "b", Rel: memory.RelOwns, Fact: "a owns b"},
		{FromID: "b", ToID: "c", Rel: memory.RelBlocks, Fact: "b blocks c"},
		{FromID: "c", ToID: "d", Rel: memory.RelImplements, Fact: "c implements d"},
	})
	contract := compiler.TaskContract{
		Question: "seed a", Keywords: []string{"seed a"},
		RequiredKinds: []string{"project", "ticket", "decision", "task"},
	}
	one := compiler.Compile(ents, contract, compiler.Options{TokenBudget: 800, Graph: g, Hops: 1})
	two := compiler.Compile(ents, contract, compiler.Options{TokenBudget: 800, Graph: g, Hops: 2})
	if idSet(one.SelectedIDs)["c"] {
		t.Fatalf("hops=1 should not reach c: %v", one.SelectedIDs)
	}
	if !idSet(two.SelectedIDs)["c"] {
		t.Fatalf("hops=2 should reach c: %v", two.SelectedIDs)
	}
	if idSet(two.SelectedIDs)["d"] {
		t.Fatalf("hops=2 should not reach d: %v", two.SelectedIDs)
	}
}

func idSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func hasAuditSource(audit []compiler.AuditEntry, src string) bool {
	for _, a := range audit {
		if a.Source == src {
			return true
		}
	}
	return false
}

func hasPackedRel(res compiler.Result, rel string) bool {
	for _, e := range res.SelectedEdges {
		if e.Rel == rel {
			return true
		}
	}
	return false
}
