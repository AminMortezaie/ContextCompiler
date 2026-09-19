package eval

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/arms"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/state"
	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// SuiteHarness runs a deterministic multi-task suite over the same org state.
type SuiteHarness struct {
	Store       *state.Store
	Tasks       []fixture.Task
	Arms        []arms.Arm
	TokenBudget int
	Out         io.Writer
}

// RunAll executes every task × arm and prints per-task tables plus a rollup.
func (h *SuiteHarness) RunAll(ctx context.Context) ([]Metrics, error) {
	if len(h.Tasks) == 0 {
		return nil, fmt.Errorf("suite: no tasks")
	}
	fmt.Fprintf(h.Out, "=== Context Compiler Phase 3 Multi-Task Suite ===\n")
	fmt.Fprintf(h.Out, "Tasks: %d | Arms: %d | Token budget: %d (chars/%d)\n",
		len(h.Tasks), len(h.Arms), h.TokenBudget, tokens.CharsPerToken)
	fmt.Fprintf(h.Out, "Entities: %d | Started: %s\n\n", len(h.Store.All()), time.Now().Format(time.RFC3339))

	var all []Metrics
	for _, task := range h.Tasks {
		goldenN := len(task.RelevantIDs)
		fmt.Fprintf(h.Out, "######## TASK %s ########\n", task.ID)
		fmt.Fprintf(h.Out, "Q: %s\n", task.Question)
		fmt.Fprintf(h.Out, "Golden relevant IDs (%d): %s\n\n", goldenN, strings.Join(task.RelevantIDs, ","))

		var taskRows []Metrics
		for _, arm := range h.Arms {
			res, err := arm.Run(ctx, h.Store, task, h.TokenBudget)
			if err != nil {
				return all, fmt.Errorf("%s/%s: %w", task.ID, arm.Name(), err)
			}
			m := Score(res, task)
			all = append(all, m)
			taskRows = append(taskRows, m)

			fmt.Fprintf(h.Out, "--- %s ---\n", m.ArmName)
			printRunDetail(h.Out, m, res, goldenN)
			fmt.Fprintln(h.Out)
		}
		fmt.Fprintln(h.Out, "=== Task comparison ===")
		PrintTable(h.Out, taskRows)
		fmt.Fprintln(h.Out)
	}

	fmt.Fprintln(h.Out, "=== Suite rollup (mean metrics by arm) ===")
	PrintSuiteRollup(h.Out, all)
	return all, nil
}

// PrintSuiteRollup averages split metrics per arm across tasks.
func PrintSuiteRollup(w io.Writer, rows []Metrics) {
	type acc struct {
		n               int
		taskOK          int
		retrievalRecall float64
		contextRecall   float64
	}
	byArm := map[string]*acc{}
	for _, m := range rows {
		a := byArm[m.ArmName]
		if a == nil {
			a = &acc{}
			byArm[m.ArmName] = a
		}
		a.n++
		if m.TaskSuccess {
			a.taskOK++
		}
		a.retrievalRecall += m.RetrievalRecall
		a.contextRecall += m.ContextRecall
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ARM\tTASKS\tTASK_SUCCESS_RATE\tMEAN_RETR_RECALL\tMEAN_CTX_RECALL")
	for arm, a := range byArm {
		if a.n == 0 {
			continue
		}
		fmt.Fprintf(tw, "%s\t%d\t%.3f\t%.3f\t%.3f\n",
			arm, a.n, float64(a.taskOK)/float64(a.n),
			a.retrievalRecall/float64(a.n), a.contextRecall/float64(a.n))
	}
	_ = tw.Flush()
}
