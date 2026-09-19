# Context Compiler

## Phase 2 — Compile API (middleware beachhead)

HTTP service that accepts a **task contract** plus org state (handle or inline entities) and returns **compiled context**, **include/exclude audit**, and **token budget usage**. Permissions hooks (allow/deny entity kinds) run before ranking. Org state sits on a pluggable in-memory handle layer for v0 (`internal/memory`); Phase 1 bench arms are unchanged.

### Run locally

```bash
go test ./...
make api                    # listens on :8080
# or: go run ./cmd/api -addr :8080
```

Built-in state handle: `day0` (Project X fixture).

### Example request

```bash
curl -sS http://localhost:8080/v1/compile \
  -H 'Content-Type: application/json' \
  -d '{
    "task_contract": {
      "question": "Why was Project X delayed, who made the decision, and what should the backend team do?"
    },
    "state": { "handle": "day0" },
    "budget": { "token_budget": 2000 },
    "permissions": {
      "allow_kinds": ["project", "decision", "ticket", "user", "conversation", "task", "team"]
    }
  }' | jq .
```

Response fields: `compiled_context`, `contract`, `audit`, `selected_ids`, `excluded_ids`, `budget_usage` (`token_budget`, `tokens_used`).

Health check: `GET /healthz`.

---

## Phase 3 — Multi-task science track

Deterministic **narrative task suite** over the same Day-0 Project X org state (causal, decision, ownership, action, plus the original composite delay task). Each task carries golden **relevant entity IDs** and **required answer phrases**.

### Split metrics (every arm, every task)

| Metric | Meaning |
|--------|---------|
| **retrieval_recall** | \|retrieved candidate IDs ∩ golden relevant\| / \|golden relevant\| — after retrieval/ranking, **before** budget-fit packing |
| **context_recall** | \|packed context entity IDs ∩ golden relevant\| / \|golden relevant\| — what actually reached the LLM |
| **task_success** | Mock/live answer contains all `RequiredPhrases` for that task |

Retrieval and context recall can diverge when budget-fit drops high-recall entities (especially Arm B/C).

### Run multi-task bench (mock, no API keys)

```bash
go test ./...
make bench-multitask
# or:
go run ./cmd/bench-multitask -small -mock -budget 2000
go run ./cmd/bench-multitask -small -mock -ablations   # adds C:compiler-no-* single-knob ablations
```

Phase 1 `make bench` and Phase 2 `make api` / `/v1/compile` are unchanged. Compiler **ablation knobs** live in `internal/compiler.Ablation` and are exercised via `arms.NewArmCAblation` in the multi-task bench only (production compile API stays full pipeline).

---

## Phase 1 — Experiment harness

## Experimental question

Does **task-aware context compilation** beat naïve **insertion-order packing from the full candidate org-state** (Arm A) and standard retrieval (Arm B) under the **same fixed LLM context budget** without dropping quality?

Primary scored task (narrative):

> Why was Project X delayed, who made the relevant decision, and what action should the backend team take?

Org-state **corpus** scale ladder (chars/4 estimator). This is how large the synthetic org-state **candidate pool** is in memory — **not** how many tokens the LLM receives:

| Rung | Corpus (~tokens) | `-tokens` | `Scale(...)` |
|------|------------------|-----------|--------------|
| Day-1 | ~100K | `100000` | `Scale(fixture.TargetTokens100K, fixture.DefaultSeed)` |
| Next | ~500K | `500000` | `Scale(fixture.TargetTokens500K, fixture.DefaultSeed)` |
| Next | ~1M | `1000000` | `Scale(fixture.TargetTokens1M, fixture.DefaultSeed)` |
| Next | ~5M | `5000000` | `Scale(fixture.TargetTokens5M, fixture.DefaultSeed)` |

### Budget equivalence (A / B / C)

Every arm receives the **same** `-budget` (default **2000** tokens, chars/4) for the packed context passed to the LLM. Arms differ in **how** they choose entities from the corpus; they do **not** differ in prompt packing budget.

| | What scales with `-tokens` | What stays at `-budget` |
|---|---------------------------|-------------------------|
| Corpus | Entity count / noise dilution (100K → 5M) | — |
| LLM context | — | Packed prompt size cap (~2K default) |

- **Arm A** walks the **full candidate state** in fixture order and **packs until the budget** — it does **not** send the entire 1M/5M corpus to the model.
- **Arm B** retrieves top-k, then packs to the same budget.
- **Arm C** ranks by task contract, budget-fits by score density, same budget.

Run the same A/B/C bakeoff at each corpus rung with that **shared packing budget**:

```bash
go test ./...                              # no Docker / API keys
make bench                                 # ~100K (default TOKENS=100000)
make bench TOKENS=500000                   # ~500K
make bench TOKENS=1000000                  # ~1M
make bench TOKENS=5000000                  # ~5M
go run ./cmd/bench -tokens 500000 -budget 2000 -mock   # explicit flags
make bench-small                           # Day-0 tiny fixture (-small)
```

See `VISION.md`.

## Stack

Go + PostgreSQL + pgvector + one LLM API. **No** LangChain / LangGraph.

## Quick start (mock LLM, optional Postgres)

```bash
make db-up          # optional: docker compose up -d
make bench -mock    # or: go run ./cmd/bench -mock
```

### Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-budget` | 2000 | **Shared** packing budget for A/B/C into the LLM prompt (chars/4) |
| `-tokens` | 100000 | Target **corpus** size (candidate org-state in memory; not LLM input size) |
| `-seed` | 42 | Deterministic fixture seed |
| `-small` | false | Use Day-0 tiny fixture |
| `-topk` | 16 | RAG top-k for arm B |
| `-mock` | false | Force mock LLM even if API key is set |

### Real LLM

Keys load from `.env` (and optionally `/home/box/.config/context-compiler/env`). Do **not** commit secrets.

```bash
# GROQ_API_KEY=...  OPENAI_BASE_URL=https://api.groq.com/openai/v1  OPENAI_MODEL=openai/gpt-oss-20b
go run ./cmd/bench -tokens 500000 -budget 2000
go run ./cmd/bench -mock -tokens 500000
```

Without a key, mock is used and a warning is printed.

### Embeddings

**hash-bow-384** (default): deterministic bag-of-words → FNV buckets → L2; offline-reproducible bakeoff path (dim 384).

### Database

`docker-compose.yml` runs `pgvector/pgvector:pg16`.

- DSN: `postgres://contextcompiler:contextcompiler@localhost:5432/contextcompiler?sslmode=disable`
- Override with `DATABASE_URL`
- Schema: `migrations/001_init.sql` (applied on Postgres connect or via `make migrate`)
- If Postgres is down, Arm B uses an **in-memory** vector store or keyword fallback. `go test ./...` stays green without Docker.

## Three arms

All three call the LLM with a context pack capped at `-budget` (default 2000 tokens).

| Arm | Pipeline |
|-----|----------|
| **A: full-dump** | **Full corpus as candidates** → concatenate entities in store order **until the same packing budget** → LLM (naïve baseline; most of a 1M/5M corpus is never packed) |
| **B: rag** | embed query → pgvector (or memory) top-k → **pack to the same budget** → LLM |
| **C: compiler** | task-contract → multi-signal rank → score/token **budget-fit** → assemble + **include/exclude audit** → LLM |

## Eval harness

Every run prints **task_success**, **retrieval_recall**, **context_recall** (plus the Phase 1 `relevant_state_recall` alias), token/cost estimates, latency, **compile_ms vs llm_ms**, **overhead_pct_of_e2e_latency**, and irrelevant ratio. Indexing for Arm B runs once in the bench harness before arms and is not counted in per-arm `compile_ms`.

Compile is local Go (no LLM spend); latency overhead is the measurable proxy for the “compile overhead” experiment criterion.

## Pass / kill (experiment-level)

**Scaffold pass:** docker compose, vector RAG when DB/memory available, fixture scales through ~5M, `go test ./...` green.

**Kill (live bakeoff):** compiler never beats A and B on quality under the same budget across the ladder, or gains are noise.

## Live bakeoff logs

| Scale | Log |
|-------|-----|
| 100K | `testdata/live-groq-100k-bakeoff.txt` |
| 500K | `testdata/live-groq-500k-bakeoff.txt` |
| 1M | `testdata/live-groq-1m-bakeoff.txt` |

## Layout

```
  cmd/api/              # Phase 2 HTTP entrypoint
  cmd/bench/
  cmd/bench-multitask/  # Phase 3 multi-task suite + ablations
  internal/api/
  internal/compiler/    # task-contract compile + audit (shared with arm C)
  internal/memory/    # pluggable org-state handles (v0 in-memory)
  internal/permissions/
  internal/arms/        # A / B / C
  internal/eval/
  internal/embed/       # hash-bow-384
  internal/fixture/     # Day0 + Scale(...)
  internal/llm/
  internal/store/
  migrations/001_init.sql
```
