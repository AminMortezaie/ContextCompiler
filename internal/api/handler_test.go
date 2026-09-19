package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/api"
	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/permissions"
)

func TestCompileHappyPath_HandleBudgetPermissions(t *testing.T) {
	mem := memory.NewInMemory().WithFixtures()
	srv := api.NewServer(mem)

	body := map[string]any{
		"task_contract": map[string]any{
			"question": "Why was Project X delayed, who made the decision, and what should backend do?",
		},
		"state": map[string]any{"handle": "day0"},
		"budget": map[string]any{
			"token_budget": 2000,
		},
		"permissions": map[string]any{
			"allow_kinds": []string{"project", "decision", "ticket", "user", "conversation", "task", "team"},
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/compile", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp compiler.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Context == "" {
		t.Fatal("empty compiled context")
	}
	if len(resp.SelectedIDs) == 0 {
		t.Fatal("expected selected IDs")
	}
	if resp.Budget.TokenBudget != 2000 {
		t.Fatalf("budget token_budget=%d", resp.Budget.TokenBudget)
	}
	if resp.Budget.TokensUsed <= 0 || resp.Budget.TokensUsed > resp.Budget.TokenBudget+50 {
		t.Fatalf("tokens_used=%d budget=%d", resp.Budget.TokensUsed, resp.Budget.TokenBudget)
	}
	hasInclude, hasExclude := false, false
	for _, a := range resp.Audit {
		switch a.Action {
		case "include":
			hasInclude = true
		case "exclude":
			hasExclude = true
		}
	}
	if !hasInclude || !hasExclude {
		t.Fatalf("audit need include+exclude: %+v", resp.Audit)
	}
	if !strings.Contains(resp.Context, "proj-x") {
		t.Fatal("expected project X in compiled context")
	}
	if !resp.Sufficiency.Sufficient {
		t.Fatalf("day0 compile should be sufficient: %+v", resp.Sufficiency)
	}
	if !strings.Contains(resp.Context, "[edge:") {
		t.Fatal("expected typed edges in compiled context")
	}
}

func TestCompileInlineEntitiesDenyKind(t *testing.T) {
	st, task := fixture.Day0()
	mem := memory.NewInMemory()
	srv := api.NewServer(mem)

	body := api.CompileRequest{
		TaskContract: compiler.TaskContract{Question: task.Question},
		State:        api.StateRef{Entities: st.All()},
		Budget:       api.BudgetConfig{TokenBudget: 1500},
		Permissions:  permissions.Policy{DenyKinds: []string{"document"}},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/compile", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d %s", rec.Code, rec.Body.String())
	}
	var resp compiler.Result
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	foundPerm := false
	for _, a := range resp.Audit {
		if a.Action == "exclude" && strings.Contains(a.Reason, "permissions:") {
			foundPerm = true
			break
		}
	}
	if !foundPerm {
		t.Fatal("expected permissions exclusion in audit")
	}
}
