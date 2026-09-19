package arms

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/embed"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/llm"
	"github.com/aminmortezaie/contextcompiler/internal/store"
)

func TestAllArmsRun(t *testing.T) {
	st, task := fixture.Day0()
	client := llm.NewMock()
	armsList := []Arm{NewArmA(client), NewArmB(client, nil, nil, 0), NewArmC(client)}
	for _, arm := range armsList {
		res, err := arm.Run(context.Background(), st, task, DefaultTokenBudget)
		if err != nil {
			t.Fatalf("%s: %v", arm.Name(), err)
		}
		if res.Answer == "" {
			t.Errorf("%s: empty answer", arm.Name())
		}
		if res.TotalTokens <= 0 {
			t.Errorf("%s: expected tokens > 0", arm.Name())
		}
		if len(res.SelectedIDs) == 0 {
			t.Errorf("%s: no selected IDs", arm.Name())
		}
	}
}

func TestArmBWithMemoryVectors(t *testing.T) {
	st, task := fixture.Day0()
	ctx := context.Background()
	vs := store.NewMemory()
	emb := embed.NewHash()
	if err := store.IndexStore(ctx, vs, st, emb); err != nil {
		t.Fatal(err)
	}
	res, err := NewArmB(llm.NewMock(), emb, vs, 8).Run(ctx, st, task, DefaultTokenBudget)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsStub {
		t.Fatalf("expected non-stub RAG path, notes=%s", res.Notes)
	}
	if !strings.Contains(res.Notes, "vector RAG") {
		t.Fatalf("expected vector RAG notes, got %q", res.Notes)
	}
	// Should retrieve at least one golden relevant entity.
	hit := false
	for _, id := range res.SelectedIDs {
		for _, g := range task.RelevantIDs {
			if id == g {
				hit = true
			}
		}
	}
	if !hit {
		t.Fatalf("RAG selected none of golden IDs: %v", res.SelectedIDs)
	}
}

func TestArmCEmitsAudit(t *testing.T) {
	st, task := fixture.Day0()
	res, err := NewArmC(llm.NewMock()).Run(context.Background(), st, task, DefaultTokenBudget)
	if err != nil {
		t.Fatal(err)
	}
	if res.AuditJSON == "" {
		t.Fatal("expected audit JSON from arm C")
	}
	var parsed struct {
		Contract compiler.TaskContract `json:"contract"`
		Audit    []compiler.AuditEntry `json:"audit"`
	}
	if err := json.Unmarshal([]byte(res.AuditJSON), &parsed); err != nil {
		t.Fatalf("audit JSON: %v", err)
	}
	if len(parsed.Audit) == 0 {
		t.Fatal("empty audit")
	}
	hasInclude, hasExclude := false, false
	for _, a := range parsed.Audit {
		if a.Action == "include" {
			hasInclude = true
		}
		if a.Action == "exclude" {
			hasExclude = true
		}
	}
	if !hasInclude || !hasExclude {
		t.Fatalf("audit must have include and exclude; include=%v exclude=%v", hasInclude, hasExclude)
	}
	if len(res.ExcludedIDs) == 0 {
		t.Fatal("expected some excluded IDs from compiler")
	}
}

func TestArmCDiffersFromArmB(t *testing.T) {
	st, task := fixture.Day0()
	ctx := context.Background()
	client := llm.NewMock()
	vs := store.NewMemory()
	emb := embed.NewHash()
	_ = store.IndexStore(ctx, vs, st, emb)

	b, err := NewArmB(client, emb, vs, 5).Run(ctx, st, task, 600)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewArmC(client).Run(ctx, st, task, 600)
	if err != nil {
		t.Fatal(err)
	}
	if c.AuditJSON == "" {
		t.Fatal("C must have audit; B typically does not")
	}
	if b.AuditJSON != "" {
		t.Fatal("B should not emit compiler audit")
	}
	// Selection sets need not be identical — measurable difference via audit presence + notes.
	if b.Notes == c.Notes {
		t.Fatal("arms B and C should have different notes")
	}
}
