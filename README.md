# Context Compiler (Phase 1)

## Experimental question

Does **task-aware context compilation** beat naïve full-context and standard retrieval under a **fixed token budget** without dropping quality?

Primary scored task (narrative):

> Why was Project X delayed, who made the relevant decision, and what action should the backend team take?

Org-state scale ladder (chars/4 estimator, labeled):

| Rung | Target tokens | Helper |
|------|---------------|--------|
| Day-1 | ~100K | `fixture.Scale100K()` / `-tokens 100000` |
| Next  | ~500K | `fixture.Scale500K()` / `-tokens 500000` |
| Next  | ~1M | `fixture.Scale1M()` / `-tokens 1000000` |
| Next  | ~5M | `fixture.Scale5M()` / `-tokens 5000000` |

See `VISION.md`.

### Bench scale ladder

Run the same A/B/C bakeoff at each rung with a **fixed packing budget** (default **2000**) so cross-scale comparisons stay fair:

```bash
make bench        # ~100K org state
make bench-500k   # ~500K
make bench-1m     # ~1M
make bench-5m     # ~5M
```

Equivalent `go run` invocations:

```bash
go run ./cmd/bench -tokens 100000  -budget 2000
go run ./cmd/bench -tokens 500000  -budget 2000
go run ./cmd/bench -tokens 1000000 -budget 2000
go run ./cmd/bench -tokens 5000000 -budget 2000
```

Add `-mock` to force the mock LLM (no API key). Day-0 tiny fixture: `make bench-small` or `-small`.

## Stack

Go + PostgreSQL + pgvector + one LLM API. **No** LangChain / LangGraph.

## Quick start (mock LLM, optional Postgres)

```bash
# From the repository root

# Unit tests — no Docker / no API keys required
go test ./...

# Optional: bring up Postgres + pgvector (Arm B persists embeddings here)
make db-up          # or: docker compose up -d
# Schema auto-migrates on connect; or: make migrate

# 100K-scale A vs B vs C bakeoff (uses .env LLM if present; -mock to force mock)
make bench
# or: go run ./cmd/bench
# tiny fixture: go run ./cmd/bench -small

# 500K-scale bakeoff (same packing budget=2000 for fair cross-scale comparison)
make bench-500k
# or: go run ./cmd/bench -tokens 500000 -budget 2000

# 1M-scale bakeoff (same packing budget=2000)
make bench-1m
# or: go run ./cmd/bench -tokens 1000000 -budget 2000

# 5M-scale bakeoff (same packing budget=2000)
make bench-5m
# or: go run ./cmd/bench -tokens 5000000 -budget 2000
```

### Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-budget` | 2000 | Packing budget into the LLM prompt (chars/4) |
| `-tokens` | 100000 | Target org-state size |
| `-seed` | 42 | Deterministic fixture seed |
| `-small` | false | Use Day-0 tiny fixture |
| `-topk` | 16 | RAG top-k for arm B |
| `-mock` | false | Force mock LLM even if API key is set |

### Real LLM / embeddings

Keys load automatically from `.env` (and optionally `/home/box/.config/context-compiler/env`).
Do **not** commit or print secrets.

```bash
# Typical Groq / OpenAI-compatible setup (already in .env for this box):
# GROQ_API_KEY=...
# OPENAI_API_KEY=...          # or same as Groq key
# OPENAI_BASE_URL=https://api.groq.com/openai/v1
# OPENAI_MODEL=openai/gpt-oss-20b

go run ./cmd/bench -tokens 500000 -budget 2000
# force mock:
go run ./cmd/bench -mock -tokens 500000
```

Without a key, mock is used and a warning is printed.

### Embeddings

| Provider | When | Dim | Notes |
|----------|------|-----|-------|
| **hash-bow-384** (default) | always | 384 | Deterministic bag-of-words → FNV buckets → L2; offline-reproducible |
| OpenAI-compatible | `CC_EMBED_PROVIDER=openai` + key | padded/truncated to 384 | Optional; not required for bakeoff |

**Caveat:** hash embeddings are a stand-in when real embeddings are unavailable; Arm B quality is not a verdict on production RAG embedding quality.

### Database

`docker-compose.yml` runs `pgvector/pgvector:pg16`.

- User/db/password: `contextcompiler`
- DSN: `postgres://contextcompiler:contextcompiler@localhost:5432/contextcompiler?sslmode=disable`
- Override with `DATABASE_URL`
- If Postgres is down, Arm B falls back to an **in-memory** vector store (or keyword fallback in unit tests without indexing). `go test ./...` stays green without Docker.

## Three arms

| Arm | Pipeline | Day-1 status |
|-----|----------|--------------|
| **A: full-dump** | concatenate state until packing budget → LLM | Working |
| **B: rag** | embed → pgvector (or memory) top-k → pack → LLM | Working (hash embeddings default) |
| **C: compiler** | task-contract → multi-signal rank → score/token budget-fit → assemble + **include/exclude audit** → LLM | Working (heuristic; measurable vs B) |

## Eval harness

Every run prints:

- `task_success` (golden required phrases)
- tokens (in / out / total)
- `est_cost` (USD; mock rate `$0.002 / 1K`)
- latency (ms) end-to-end
- **Overhead split (all arms):**
  - `compile_ms` — select / rank / pack (and query retrieval for B); local work
  - `llm_ms` — `Generate()` wall time only
  - `compile_cost_usd` — **$0** in this prototype (compiler is local Go; no LLM spend)
  - `llm_cost_usd` — estimated from LLM token usage
  - `overhead_pct_of_e2e_cost` — `compile_cost / (compile_cost + llm_cost)` → **0%** while compile is free
  - `overhead_pct_of_e2e_latency` — `compile_ms / total_ms` — **latency proxy** for the experiment’s “compile overhead < ~30% of e2e $” criterion when $ overhead is trivially 0
- `relevant_state_recall`
- `irrelevant_state_ratio`

Indexing (Arm B embed-all) is done **once** in the bench harness before arms run and is **not** counted in per-arm `compile_ms`.

### What “overhead” means in this prototype

Product criterion: compiler overhead should stay under ~**30% of end-to-end cost**.

In Phase 1 the compiler is pure local compute (no LLM calls in the compile/select/pack path), so:

1. **Cost overhead** (`overhead_pct_of_e2e_cost`) is **0%** by construction — compile adds no billed tokens.
2. We therefore also report **latency overhead** (`overhead_pct_of_e2e_latency`) as a measurable proxy: how much of the arm’s wall time is compile vs waiting on the LLM.
3. Prefer comparing arms at the **same packing budget** (default **2000**) across the 100K → 500K → 1M → 5M scale ladder so quality/cost changes are attributable to scale + selection policy, not a changed prompt budget.

## Pass / kill (experiment-level — do not invent new criteria)

**Scaffold pass:**

- `docker compose up -d` works; schema migrated
- Arm B uses vector search when DB (or memory index) available
- Fixture scales to ~100K, ~500K, ~1M, and ~5M; `go run ./cmd/bench -tokens …` prints A/B/C + overhead tables
- `go test ./...` green without Docker

**Kill (live-LLM bakeoff):** compiler never beats A and B on quality under the same budget across the scale ladder, or gains are noise / not worth the complexity.

## Live bakeoff logs

| Scale | Log |
|-------|-----|
| 100K | `testdata/live-groq-100k-bakeoff.txt` |
| 500K | `testdata/live-groq-500k-bakeoff.txt` |
| 1M | `testdata/live-groq-1m-bakeoff.txt` |

## Layout

```
  VISION.md
  README.md
  Makefile
  docker-compose.yml
  migrations/001_init.sql
  cmd/bench/
  internal/state/
  internal/fixture/     # Day0 + Scale / Scale100K … Scale5M
  internal/llm/         # Mock + OpenAI-compatible
  internal/embed/       # Hash-384 + optional OpenAI
  internal/store/       # Memory + Postgres/pgvector
  internal/arms/        # A / B / C
  internal/eval/
  internal/tokens/      # chars/4 estimator
```
