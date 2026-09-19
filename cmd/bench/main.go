// Command bench runs A vs B vs C at configurable org-state scale (uses .env LLM if set; -mock to force mock).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/arms"
	"github.com/aminmortezaie/contextcompiler/internal/embed"
	"github.com/aminmortezaie/contextcompiler/internal/eval"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/llm"
	"github.com/aminmortezaie/contextcompiler/internal/state"
	"github.com/aminmortezaie/contextcompiler/internal/store"
)

func main() {
	loadDotEnv(".env")
	loadDotEnv("/home/box/.config/context-compiler/env")
	budget := flag.Int("budget", arms.DefaultTokenBudget, "token budget for packing into the prompt (chars/4)")
	target := flag.Int("tokens", fixture.TargetTokens100K, "target org-state size in tokens (chars/4)")
	seed := flag.Int64("seed", fixture.DefaultSeed, "RNG seed for synthetic fixture")
	small := flag.Bool("small", false, "use Day-0 tiny fixture instead of scaled org")
	topK := flag.Int("topk", 16, "RAG top-k for arm B")
	useMock := flag.Bool("mock", false, "force mock LLM even if API key is set")
	flag.Parse()

	ctx := context.Background()

	var client llm.Client
	if *useMock {
		client = llm.NewMock()
	} else {
		client = llm.ClientFromEnv()
		if client.Name() == "mock" {
			fmt.Fprintf(os.Stderr, "WARN: no OPENAI_API_KEY/GROQ_API_KEY; using mock\n")
		}
	}
	fmt.Fprintf(os.Stderr, "LLM client: %s\n", client.Name())

	var (
		st   *state.Store
		task fixture.Task
	)
	if *small {
		st, task = fixture.Day0()
	} else {
		st, task = fixture.Scale(*target, *seed)
	}
	est := fixture.EstimateStoreTokens(st)
	fmt.Fprintf(os.Stderr, "Fixture entities=%d est_tokens=%d (chars/%d) seed=%d\n",
		len(st.All()), est, 4, *seed)

	vs, warn := store.OpenOrMemory(ctx)
	if warn != "" {
		fmt.Fprintf(os.Stderr, "WARN: %s\n", warn)
	} else {
		fmt.Fprintf(os.Stderr, "Vector store: %s\n", vs.Name())
	}
	defer vs.Close()

	emb := embed.NewFromEnv()
	fmt.Fprintf(os.Stderr, "Embedder: %s (dim=%d)\n", emb.Name(), emb.Dim())

	if pg, ok := vs.(*store.Postgres); ok {
		if err := pg.Reset(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: reset: %v\n", err)
		}
	}

	indexStart := time.Now()
	if err := store.IndexStore(ctx, vs, st, emb); err != nil {
		fmt.Fprintf(os.Stderr, "index failed: %v\n", err)
		os.Exit(1)
	}
	n, _ := vs.Count(ctx)
	fmt.Fprintf(os.Stderr, "Indexed %d entities in %s\n", n, time.Since(indexStart).Round(time.Millisecond))

	h := &eval.Harness{
		Store:       st,
		Task:        task,
		Arms:        []arms.Arm{arms.NewArmA(client), arms.NewArmB(client, emb, vs, *topK), arms.NewArmC(client)},
		TokenBudget: *budget,
		Out:         os.Stdout,
	}

	fmt.Fprintf(os.Stderr, "Running A/B/C with packing budget=%d\n\n", *budget)
	if _, err := h.RunAll(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "bench failed: %v\n", err)
		os.Exit(1)
	}
}

// loadDotEnv loads KEY=VAL lines into the process env if not already set.
func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}
