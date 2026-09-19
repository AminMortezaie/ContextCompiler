package compiler_test

import (
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
)

func TestAblationNoTaskContractStripsHints(t *testing.T) {
	st, task := fixture.Day0()
	contract := compiler.BuildContractFromQuestion(task.Question)
	abl := compiler.Compile(st.All(), contract, compiler.Options{
		TokenBudget: 600,
		Ablation:    compiler.Ablation{NoTaskContract: true},
	})
	if len(abl.Contract.Keywords) > 0 || len(abl.Contract.ProjectHints) > 0 {
		t.Fatal("no-task-contract should strip derived keywords and project hints")
	}
}

func TestAblationNoAuditOmitsRows(t *testing.T) {
	st, task := fixture.Day0()
	contract := compiler.BuildContractFromQuestion(task.Question)
	res := compiler.Compile(st.All(), contract, compiler.Options{
		TokenBudget: 2000,
		Ablation:    compiler.Ablation{NoAudit: true},
	})
	if len(res.Audit) != 0 {
		t.Fatalf("expected empty audit, got %d rows", len(res.Audit))
	}
	if res.Context == "" {
		t.Fatal("context should still be packed")
	}
}

func TestAblationLabels(t *testing.T) {
	if (compiler.Ablation{}).Label() != "" {
		t.Fatal("full compiler should have empty label")
	}
	abl := compiler.Ablation{NoRanking: true}
	if abl.Label() != "no-ranking" {
		t.Fatal("label mismatch")
	}
}
