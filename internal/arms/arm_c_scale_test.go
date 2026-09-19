package arms

import (
	"context"
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/llm"
)

func TestArmCAt5MRecallUnchanged(t *testing.T) {
	st, task := fixture.Scale(fixture.TargetTokens5M, fixture.DefaultSeed)
	res, err := NewArmC(llm.NewMock()).Run(context.Background(), st, task, DefaultTokenBudget)
	if err != nil {
		t.Fatal(err)
	}
	rel := make(map[string]struct{}, len(task.RelevantIDs))
	for _, id := range task.RelevantIDs {
		rel[id] = struct{}{}
	}
	hits := 0
	for _, id := range res.SelectedIDs {
		if _, ok := rel[id]; ok {
			hits++
		}
	}
	recall := float64(hits) / float64(len(task.RelevantIDs))
	if recall < 0.999 {
		t.Fatalf("recall=%v selected=%v", recall, res.SelectedIDs)
	}
}
