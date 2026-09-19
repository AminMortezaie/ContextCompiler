package memory_test

import (
	"context"
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
)

func TestInMemoryGraphHopLimit(t *testing.T) {
	mem := memory.NewInMemory().WithFixtures()
	ctx := context.Background()

	one, err := mem.Neighbors(ctx, "day0", []string{"proj-x"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	got := nodeSet(one)
	for _, id := range []string{"proj-x", "dec-001", "tkt-042", "usr-sarah"} {
		if !got[id] {
			t.Fatalf("1-hop from proj-x missing %s: %v", id, keys(got))
		}
	}
	if got["task-freeze"] {
		t.Fatalf("1-hop should not reach task-freeze: %v", keys(got))
	}

	two, err := mem.Neighbors(ctx, "day0", []string{"proj-x"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	got2 := nodeSet(two)
	for _, id := range []string{"task-freeze", "usr-marcus", "conv-standup"} {
		if !got2[id] {
			t.Fatalf("2-hop from proj-x missing %s: %v", id, keys(got2))
		}
	}
	if !hasRel(two.Edges, "blocks") || !hasRel(two.Edges, "decided_by") {
		t.Fatalf("expected typed blocks/decided_by edges, got %+v", rels(two.Edges))
	}
}

func TestGraphSearchSeeds(t *testing.T) {
	mem := memory.NewInMemory().WithFixtures()
	nb, err := mem.Search(context.Background(), "day0", memory.Query{
		Keywords: []string{"delay"},
		Seeds:    []string{"proj-x"},
		Limit:    8,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := nodeSet(nb)
	if !got["proj-x"] {
		t.Fatalf("seed proj-x missing: %v", keys(got))
	}
	if !got["dec-001"] {
		t.Fatalf("keyword delay should hit dec-001: %v", keys(got))
	}
}

func TestDefaultGraphNotCoOccurrenceSoup(t *testing.T) {
	st, _ := fixture.Scale(5_000, 42)
	g := memory.DefaultGraph(st.All())
	nb, err := g.Neighbors(context.Background(), memory.InlineHandle, []string{"proj-x"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nb.Nodes {
		if n.Entity.Meta != nil && n.Entity.Meta["noise"] == "true" {
			t.Fatalf("Project X neighborhood leaked into noise %s", n.Entity.ID)
		}
	}
	// Noise has its own typed clusters, disconnected from proj-x.
	var noiseTicket string
	for _, e := range st.All() {
		if e.Meta != nil && e.Meta["noise"] == "true" && e.Kind == "ticket" {
			noiseTicket = e.ID
			break
		}
	}
	if noiseTicket == "" {
		t.Fatal("expected a noise ticket")
	}
	nn, err := g.Neighbors(context.Background(), memory.InlineHandle, []string{noiseTicket}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRel(nn.Edges, "assigned_to") && !hasRel(nn.Edges, "blocks") {
		t.Fatalf("noise ticket should have typed assigned_to/blocks, got %v", rels(nn.Edges))
	}
	for _, n := range nn.Nodes {
		if n.Entity.ID == "proj-x" || n.Entity.ID == "dec-001" {
			t.Fatal("noise cluster must not connect to Project X")
		}
	}
}

func TestLayerStillLoads(t *testing.T) {
	mem := memory.NewInMemory().WithFixtures()
	ents, err := mem.Load(context.Background(), "day0")
	if err != nil || len(ents) < 10 {
		t.Fatalf("load day0: n=%d err=%v", len(ents), err)
	}
	if _, ok := memory.AsGraphStore(mem); !ok {
		t.Fatal("InMemory should implement GraphStore")
	}
}

func nodeSet(nb memory.Neighborhood) map[string]bool {
	m := make(map[string]bool, len(nb.Nodes))
	for _, n := range nb.Nodes {
		m[n.Entity.ID] = true
	}
	return m
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func hasRel(edges []memory.Edge, rel string) bool {
	for _, e := range edges {
		if e.Rel == rel {
			return true
		}
	}
	return false
}

func rels(edges []memory.Edge) []string {
	var out []string
	for _, e := range edges {
		out = append(out, e.Rel)
	}
	return out
}
