// Package eval is the evaluation harness — highest priority for Day-0.
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

// Harness runs all arms and prints the comparison table.
type Harness struct {
	Store       *state.Store
	Task        fixture.Task
	Arms        []arms.Arm
	TokenBudget int
	Out         io.Writer
	GoldenN     int // len(task.RelevantIDs), set in RunAll
}

// RunAll executes every arm, scores them, and prints the metrics table.
func (h *Harness) RunAll(ctx context.Context) ([]Metrics, error) {
	h.GoldenN = len(h.Task.RelevantIDs)
	var all []Metrics
	fmt.Fprintf(h.Out, "=== Context Compiler Day-1 Bench ===\n")
	fmt.Fprintf(h.Out, "Task: %s\n", h.Task.Question)
	fmt.Fprintf(h.Out, "Token budget: %d (estimator: chars/%d)\n", h.TokenBudget, tokens.CharsPerToken)
	fmt.Fprintf(h.Out, "Entities: %d | Golden relevant: %d\n", len(h.Store.All()), h.GoldenN)
	fmt.Fprintf(h.Out, "Started: %s\n\n", time.Now().Format(time.RFC3339))

	for _, arm := range h.Arms {
		res, err := arm.Run(ctx, h.Store, h.Task, h.TokenBudget)
		if err != nil {
			return all, fmt.Errorf("%s: %w", arm.Name(), err)
		}
		m := Score(res, h.Task)
		all = append(all, m)

		fmt.Fprintf(h.Out, "--- %s ---\n", m.ArmName)
		printRunDetail(h.Out, m, res, h.GoldenN)
		fmt.Fprintln(h.Out)
	}

	fmt.Fprintln(h.Out, "=== Comparison Table ===")
	PrintTable(h.Out, all)
	fmt.Fprintln(h.Out)
	fmt.Fprintln(h.Out, "=== Overhead (compile vs e2e latency) ===")
	PrintOverheadTable(h.Out, all)
	return all, nil
}

func printRunDetail(w io.Writer, m Metrics, res arms.RunResult, goldenN int) {
	fmt.Fprintf(w, "  stub: %v\n", m.IsStub)
	fmt.Fprintf(w, "  task_success: %v\n", m.TaskSuccess)
	fmt.Fprintf(w, "  tokens: in=%d out=%d total=%d\n", m.InputTokens, m.OutputTokens, m.TotalTokens)
	fmt.Fprintf(w, "  est_cost_usd: $%.6f\n", m.EstCostUSD)
	fmt.Fprintf(w, "  latency_ms: %d\n", m.LatencyMS)
	fmt.Fprintf(w, "  compile_ms: %d\n", m.CompileMS)
	fmt.Fprintf(w, "  llm_ms: %d\n", m.LLMMS)
	fmt.Fprintf(w, "  overhead_pct_of_e2e_latency: %.2f%%\n", m.OverheadPctOfE2ELat)
	fmt.Fprintf(w, "  retrieval_recall: %.3f (%d/%d golden)\n", m.RetrievalRecall, m.RetrievalHitCount, goldenN)
	fmt.Fprintf(w, "  context_recall: %.3f (%d/%d golden)\n", m.ContextRecall, m.RelevantHitCount, goldenN)
	fmt.Fprintf(w, "  irrelevant_state_ratio: %.3f\n", m.IrrelevantStateRatio)
	fmt.Fprintf(w, "  selected: %d ids: %s\n", m.SelectedCount, strings.Join(res.SelectedIDs, ","))
	fmt.Fprintf(w, "  notes: %s\n", m.Notes)
	fmt.Fprintf(w, "  answer: %s\n", m.AnswerPreview)
	if res.AuditJSON != "" {
		fmt.Fprintf(w, "  audit: (JSON, %d bytes)\n", len(res.AuditJSON))
	}
}

// PrintTable writes the A vs B vs C comparison table with required metrics.
func PrintTable(w io.Writer, rows []Metrics) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ARM\tSUCCESS\tTOKENS\tCOST_USD\tLAT_MS\tRETR_RECALL\tCTX_RECALL\tIRR_RATIO\tSTUB")
	for _, m := range rows {
		fmt.Fprintf(tw, "%s\t%v\t%d\t$%.6f\t%d\t%.3f\t%.3f\t%.3f\t%v\n",
			m.ArmName,
			m.TaskSuccess,
			m.TotalTokens,
			m.EstCostUSD,
			m.LatencyMS,
			m.RetrievalRecall,
			m.ContextRecall,
			m.IrrelevantStateRatio,
			m.IsStub,
		)
	}
	_ = tw.Flush()
}

// PrintOverheadTable prints compile vs LLM timing split (latency proxy for compile overhead).
func PrintOverheadTable(w io.Writer, rows []Metrics) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ARM\tCOMPILE_MS\tLLM_MS\tOH_%_LAT")
	for _, m := range rows {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%.2f%%\n",
			m.ArmName,
			m.CompileMS,
			m.LLMMS,
			m.OverheadPctOfE2ELat,
		)
	}
	_ = tw.Flush()
	fmt.Fprintln(w, "Note: compile is local Go (no LLM spend). OH_%_LAT = compile_ms/total_ms is the measurable overhead proxy.")
}
