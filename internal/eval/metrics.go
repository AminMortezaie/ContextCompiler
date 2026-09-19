package eval

import (
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/arms"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// Metrics are the scored outputs printed for every run.
type Metrics struct {
	ArmName              string
	TaskSuccess          bool
	InputTokens          int
	OutputTokens         int
	TotalTokens          int
	EstCostUSD           float64 // estimated LLM cost (compile is local)
	LatencyMS            int64   // end-to-end wall ms
	CompileMS            int64   // select/rank/pack (+ retrieval for B)
	LLMMS                int64   // LLM Generate wall ms
	OverheadPctOfE2ELat  float64 // compile_ms / total_ms * 100
	RelevantStateRecall  float64 // |selected ∩ relevant| / |relevant|
	IrrelevantStateRatio float64 // |selected − relevant| / |selected|
	SelectedCount        int
	RelevantHitCount     int
	IsStub               bool
	Notes                string
	AnswerPreview        string
}

// Score computes metrics from a RunResult against the golden task.
func Score(res arms.RunResult, task fixture.Task) Metrics {
	success := taskSuccess(res.Answer, task.RequiredPhrases)

	relSet := make(map[string]struct{}, len(task.RelevantIDs))
	for _, id := range task.RelevantIDs {
		relSet[id] = struct{}{}
	}

	hit := 0
	irrelevant := 0
	for _, id := range res.SelectedIDs {
		if _, ok := relSet[id]; ok {
			hit++
		} else {
			irrelevant++
		}
	}

	recall := 0.0
	if len(task.RelevantIDs) > 0 {
		recall = float64(hit) / float64(len(task.RelevantIDs))
	}
	irrRatio := 0.0
	if len(res.SelectedIDs) > 0 {
		irrRatio = float64(irrelevant) / float64(len(res.SelectedIDs))
	}

	preview := res.Answer
	if len(preview) > 120 {
		preview = preview[:117] + "..."
	}

	llmCost := tokens.EstimateCostUSD(res.TotalTokens)

	totalMS := res.Latency.Milliseconds()
	compileMS := res.CompileLatency.Milliseconds()
	llmMS := res.LLMLatency.Milliseconds()
	overheadLatPct := 0.0
	if totalMS > 0 {
		overheadLatPct = float64(compileMS) / float64(totalMS) * 100.0
	}

	return Metrics{
		ArmName:              res.ArmName,
		TaskSuccess:          success,
		InputTokens:          res.InputTokens,
		OutputTokens:         res.OutputTokens,
		TotalTokens:          res.TotalTokens,
		EstCostUSD:           llmCost,
		LatencyMS:            totalMS,
		CompileMS:            compileMS,
		LLMMS:                llmMS,
		OverheadPctOfE2ELat:  overheadLatPct,
		RelevantStateRecall:  recall,
		IrrelevantStateRatio: irrRatio,
		SelectedCount:        len(res.SelectedIDs),
		RelevantHitCount:     hit,
		IsStub:               res.IsStub,
		Notes:                res.Notes,
		AnswerPreview:        preview,
	}
}

func taskSuccess(answer string, required []string) bool {
	lower := strings.ToLower(answer)
	lower = strings.Map(func(r rune) rune {
		switch r {
		case '\u00a0', '\u202f', '\u2007', '\u2009':
			return ' '
		default:
			return r
		}
	}, lower)
	for _, p := range required {
		if !strings.Contains(lower, strings.ToLower(p)) {
			return false
		}
	}
	return true
}
