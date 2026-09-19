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
	TaskID               string
	TaskSuccess          bool
	InputTokens          int
	OutputTokens         int
	TotalTokens          int
	EstCostUSD           float64 // estimated LLM cost (compile is local)
	LatencyMS            int64   // end-to-end wall ms
	CompileMS            int64   // select/rank/pack (+ retrieval for B)
	LLMMS                int64   // LLM Generate wall ms
	OverheadPctOfE2ELat  float64 // compile_ms / total_ms * 100
	RetrievalRecall      float64 // |retrieved ∩ relevant| / |relevant|
	ContextRecall        float64 // |packed selected ∩ relevant| / |relevant|
	IrrelevantStateRatio float64 // |selected − relevant| / |selected|
	SelectedCount        int
	RetrievalHitCount    int
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

	retrievalHit := countHits(res.RetrievedIDs, relSet)
	contextHit := countHits(res.SelectedIDs, relSet)

	irrelevant := 0
	for _, id := range res.SelectedIDs {
		if _, ok := relSet[id]; !ok {
			irrelevant++
		}
	}

	retrievalRecall := recallFraction(retrievalHit, len(task.RelevantIDs))
	contextRecall := recallFraction(contextHit, len(task.RelevantIDs))
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
		TaskID:               task.ID,
		TaskSuccess:          success,
		InputTokens:          res.InputTokens,
		OutputTokens:         res.OutputTokens,
		TotalTokens:          res.TotalTokens,
		EstCostUSD:           llmCost,
		LatencyMS:            totalMS,
		CompileMS:            compileMS,
		LLMMS:                llmMS,
		OverheadPctOfE2ELat:  overheadLatPct,
		RetrievalRecall:      retrievalRecall,
		ContextRecall:        contextRecall,
		IrrelevantStateRatio: irrRatio,
		SelectedCount:        len(res.SelectedIDs),
		RetrievalHitCount:    retrievalHit,
		RelevantHitCount:     contextHit,
		IsStub:               res.IsStub,
		Notes:                res.Notes,
		AnswerPreview:        preview,
	}
}

func countHits(ids []string, golden map[string]struct{}) int {
	hit := 0
	for _, id := range ids {
		if _, ok := golden[id]; ok {
			hit++
		}
	}
	return hit
}

func recallFraction(hit, goldenN int) float64 {
	if goldenN <= 0 {
		return 0
	}
	return float64(hit) / float64(goldenN)
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
