package arms

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/embed"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/llm"
	"github.com/aminmortezaie/contextcompiler/internal/state"
	"github.com/aminmortezaie/contextcompiler/internal/store"
	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// ArmB is RAG: embed query → vector top-k → pack → LLM.
// When VectorStore is nil, falls back to in-memory keyword top-k (unit-test safe).
// Indexing is done once outside the arm (bench harness); compile phase here is
// query-embed + search + pack only.
type ArmB struct {
	LLM      llm.Client
	Embedder embed.Provider
	Vectors  store.VectorStore // optional; nil → keyword fallback
	TopK     int
}

// NewArmB constructs arm B. Nil embedder defaults to hash; nil VectorStore uses keyword fallback.
func NewArmB(client llm.Client, emb embed.Provider, vs store.VectorStore, topK int) *ArmB {
	if emb == nil {
		emb = embed.NewHash()
	}
	if topK <= 0 {
		topK = 12
	}
	return &ArmB{LLM: client, Embedder: emb, Vectors: vs, TopK: topK}
}

func (a *ArmB) Name() string {
	if a.Vectors != nil {
		return "B:rag"
	}
	return "B:rag-fallback"
}

func (a *ArmB) Run(ctx context.Context, st *state.Store, task fixture.Task, tokenBudget int) (RunResult, error) {
	start := time.Now()

	var ranked []state.Entity
	notes := ""
	isStub := false

	if a.Vectors != nil && a.Embedder != nil {
		qEmb, err := embed.EmbedDocument(ctx, a.Embedder, task.Question, "")
		if err != nil {
			return RunResult{}, err
		}
		hits, err := a.Vectors.Search(ctx, qEmb, a.TopK)
		if err != nil {
			ranked = keywordTopK(st.All(), task.Question, a.TopK)
			notes = fmt.Sprintf("vector search failed (%v); keyword fallback", err)
			isStub = true
		} else {
			ranked = make([]state.Entity, len(hits))
			for i, h := range hits {
				ranked[i] = h.Entity
			}
			notes = fmt.Sprintf("vector RAG via %s + %s top-%d (pack by density)", a.Embedder.Name(), a.Vectors.Name(), a.TopK)
		}
	} else {
		ranked = keywordTopK(st.All(), task.Question, a.TopK)
		notes = "keyword-overlap fallback (no vector store); still packs top-k within budget"
		isStub = true
	}

	packed, selected := packByShortestFirst(ranked, tokenBudget)
	excluded := ExcludedFrom(st.IDs(), selected)
	compileDur := time.Since(start)

	system := "You answer questions about organizational state using only the provided context."
	prompt := fmt.Sprintf("CONTEXT (RAG top-%d):\n%s\n\nQUESTION:\n%s\n\nAnswer concisely.", a.TopK, packed, task.Question)

	llmStart := time.Now()
	resp, err := a.LLM.Generate(ctx, llm.Request{System: system, Prompt: prompt})
	llmDur := time.Since(llmStart)
	if err != nil {
		return RunResult{}, err
	}

	return RunResult{
		ArmName:        a.Name(),
		PackedContext:  packed,
		SelectedIDs:    selected,
		ExcludedIDs:    excluded,
		Answer:         resp.Text,
		InputTokens:    resp.InputTokens,
		OutputTokens:   resp.OutputTokens,
		TotalTokens:    resp.InputTokens + resp.OutputTokens,
		Latency:        time.Since(start),
		CompileLatency: compileDur,
		LLMLatency:     llmDur,
		IsStub:         isStub,
		Notes:          notes,
	}, nil
}

// packByShortestFirst keeps ranked order but skips candidates that do not fit,
// allowing later shorter entities to fill remaining budget.
func packByShortestFirst(entities []state.Entity, tokenBudget int) (packed string, selected []string) {
	var b strings.Builder
	used := 0
	for _, e := range entities {
		chunk := e.PackText() + "\n\n"
		cost := tokens.Estimate(chunk)
		if used+cost > tokenBudget && used > 0 {
			continue
		}
		b.WriteString(chunk)
		selected = append(selected, e.ID)
		used += cost
		if used >= tokenBudget {
			break
		}
	}
	return b.String(), selected
}

func keywordTopK(entities []state.Entity, query string, k int) []state.Entity {
	qSet := tokenSet(query)
	type scored struct {
		e     state.Entity
		score float64
	}
	var scoredList []scored
	for _, e := range entities {
		textSet := tokenSet(e.Title + " " + e.Text)
		score := 0.0
		for w := range qSet {
			if textSet[w] {
				score += 1.0
			}
		}
		lengthPen := float64(len(e.Text)) / 2000.0
		score -= lengthPen
		scoredList = append(scoredList, scored{e: e, score: score})
	}
	sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].score > scoredList[j].score })
	if k > len(scoredList) {
		k = len(scoredList)
	}
	out := make([]state.Entity, 0, k)
	for i := 0; i < k; i++ {
		out = append(out, scoredList[i].e)
	}
	return out
}

func tokenSet(s string) map[string]bool {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-'
	})
	stop := map[string]bool{"the": true, "a": true, "an": true, "was": true, "and": true, "what": true, "who": true, "why": true, "should": true, "do": true, "to": true, "of": true, "for": true, "is": true, "in": true, "on": true, "this": true, "that": true, "with": true, "from": true, "are": true, "be": true, "or": true, "as": true, "by": true, "at": true, "it": true, "not": true, "no": true, "other": true, "than": true}
	out := make(map[string]bool)
	for _, f := range fields {
		if len(f) < 2 || stop[f] {
			continue
		}
		for _, suf := range []string{"ing", "ed", "ly", "es", "s"} {
			if len(f) > len(suf)+3 && strings.HasSuffix(f, suf) {
				f = f[:len(f)-len(suf)]
				break
			}
		}
		out[f] = true
	}
	return out
}
