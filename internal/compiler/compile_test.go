package compiler_test

import (
	"strings"
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/permissions"
)

func TestContractToCompileBudgetPermissions(t *testing.T) {
	st, task := fixture.Day0()
	contract := compiler.BuildContractFromQuestion(task.Question)
	res := compiler.Compile(st.All(), contract, compiler.Options{
		TokenBudget: 2000,
		Graph:       memory.DefaultGraph(st.All()),
		Handle:      memory.InlineHandle,
		Permissions: permissions.Policy{
			AllowKinds: []string{"project", "decision", "ticket", "user", "conversation", "task", "team"},
		},
	})
	if res.Context == "" {
		t.Fatal("empty context")
	}
	if res.Budget.TokensUsed > res.Budget.TokenBudget {
		t.Fatalf("over budget: used=%d budget=%d", res.Budget.TokensUsed, res.Budget.TokenBudget)
	}
	for _, id := range []string{"proj-x", "dec-001", "tkt-042"} {
		found := false
		for _, s := range res.SelectedIDs {
			if s == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing golden id %s in %v", id, res.SelectedIDs)
		}
	}
	hasPerm := false
	for _, a := range res.Audit {
		if strings.Contains(a.Reason, "permissions:") {
			hasPerm = true
		}
	}
	if !hasPerm {
		t.Fatal("expected company/document excluded via allow_kinds")
	}
	if !res.Sufficiency.Sufficient {
		t.Fatalf("Day-0 compile should be sufficient: %+v", res.Sufficiency)
	}
	if len(res.SelectedEdges) == 0 {
		t.Fatal("expected packed typed edges")
	}
	hasSrc := false
	for _, a := range res.Audit {
		if a.Action == "include" && (a.Source == "lexical" || a.Source == "vector" || a.Source == "graph" || a.Source == "expand") {
			hasSrc = true
			break
		}
	}
	if !hasSrc {
		t.Fatalf("expected provenance source on include rows: %+v", res.Audit)
	}
	for _, a := range res.Audit {
		if strings.Contains(a.Reason, "ref-boost") {
			t.Fatal("hardcoded ref-boost should be gone")
		}
	}
}
