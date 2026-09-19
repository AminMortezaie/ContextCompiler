package fixture

import "testing"

func TestDay0EdgesAreTypedNotSoup(t *testing.T) {
	st, _ := Day0()
	have := map[string]bool{}
	for _, e := range st.All() {
		have[e.ID] = true
	}
	edges := Day0Edges()
	if len(edges) < 10 {
		t.Fatalf("expected a real Day-0 graph, got %d edges", len(edges))
	}
	rels := map[string]int{}
	for _, e := range edges {
		if !have[e.From] || !have[e.To] {
			t.Fatalf("edge %s-%s-%s references missing entity", e.From, e.Rel, e.To)
		}
		if e.Rel == "" || e.Rel == "related" {
			t.Fatalf("Day-0 edge should be typed, got %q", e.Rel)
		}
		if e.Fact == "" {
			t.Fatalf("edge %s missing fact", e.Rel)
		}
		rels[e.Rel]++
	}
	for _, want := range []string{"blocks", "owns", "decided_by", "assigned_to", "implements"} {
		if rels[want] == 0 {
			t.Fatalf("missing typed rel %s in %v", want, rels)
		}
	}
}

func TestGraphSpecsScaleKeepsDay0AndNoiseClusters(t *testing.T) {
	st, _ := Scale(8_000, DefaultSeed)
	specs := GraphSpecs(st.All())
	var day0, noise int
	for _, s := range specs {
		if s.From == "tkt-042" && s.Rel == "blocks" && s.To == "proj-x" {
			day0++
		}
		if len(s.From) > 6 && s.From[:6] == "noise-" {
			noise++
		}
	}
	if day0 == 0 {
		t.Fatal("scaled graph dropped Day-0 tkt-042 blocks proj-x")
	}
	if noise == 0 {
		t.Fatal("scaled graph should include typed noise clusters")
	}
}
