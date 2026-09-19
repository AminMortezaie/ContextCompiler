package eval

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/arms"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/llm"
)

func TestSuiteHarnessRunsMock(t *testing.T) {
	st, _ := fixture.Day0()
	client := llm.NewMock()
	var buf bytes.Buffer
	h := &SuiteHarness{
		Store:       st,
		Tasks:       fixture.Day0TaskSuite(),
		Arms:        []arms.Arm{arms.NewArmA(client), arms.NewArmB(client, nil, nil, 8), arms.NewArmC(client)},
		TokenBudget: arms.DefaultTokenBudget,
		Out:         &buf,
	}
	metrics, err := h.RunAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantRuns := len(fixture.Day0TaskSuite()) * 3
	if len(metrics) != wantRuns {
		t.Fatalf("want %d metric rows, got %d", wantRuns, len(metrics))
	}
	out := buf.String()
	if !strings.Contains(out, "retrieval_recall:") || !strings.Contains(out, "context_recall:") {
		t.Fatal("missing split recall metrics in output")
	}
	if !strings.Contains(out, "Suite rollup") {
		t.Fatal("missing rollup")
	}
}
