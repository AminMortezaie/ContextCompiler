package compiler_test

import (
	"strings"
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/permissions"
)

func TestContractToCompileBudgetPermissions(t *testing.T) {
	st, task := fixture.Day0()
	contract := compiler.BuildContractFromQuestion(task.Question)
	res := compiler.Compile(st.All(), contract, compiler.Options{
		TokenBudget: 2000,
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
}
