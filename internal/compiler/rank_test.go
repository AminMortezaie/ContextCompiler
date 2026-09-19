package compiler_test

import (
	"os"
	"strings"
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

func TestRankHasNoHardcodedHubIDs(t *testing.T) {
	// The ranker must work for a graph that does not use Day-0 IDs.
	ents := []state.Entity{
		{ID: "hub", Kind: state.KindProject, Title: "Hub", Text: "the hub project slipped"},
		{ID: "choice", Kind: state.KindDecision, Title: "Choice", Text: "approved the slip"},
		{ID: "stop", Kind: state.KindTicket, Title: "Stop", Text: "blocks the hub"},
	}
	g := memory.NewStaticGraph(ents, []memory.Edge{
		{FromID: "choice", ToID: "hub", Rel: memory.RelDecides, Fact: "choice decides hub"},
		{FromID: "stop", ToID: "hub", Rel: memory.RelBlocks, Fact: "stop blocks hub"},
	})
	res := compiler.Compile(ents, compiler.TaskContract{
		Question:      "Why did the hub project slip?",
		ProjectHints:  []string{"hub"},
		Keywords:      []string{"slip", "hub"},
		RequiredKinds: []string{"project", "decision", "ticket"},
	}, compiler.Options{TokenBudget: 400, Graph: g, Hops: 1})
	got := idSet(res.SelectedIDs)
	for _, id := range []string{"hub", "choice", "stop"} {
		if !got[id] {
			t.Fatalf("missing %s in %v", id, res.SelectedIDs)
		}
	}
}

func TestRankSourceFilesOmitMagicIDs(t *testing.T) {
	for _, name := range []string{"rank.go", "gather.go", "expand.go", "compile.go"} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, magic := range []string{`"proj-x"`, `"dec-001"`, `"tkt-042"`} {
			if strings.Contains(s, magic) {
				t.Fatalf("%s still special-cases %s", name, magic)
			}
		}
		if strings.Contains(s, "ref-boost") {
			t.Fatalf("%s still has hardcoded ref-boost", name)
		}
	}
}
