# Context Compiler

Task-aware **context compilation**: select org-state **candidates** and pack them under a **shared ~2K token budget**, with an include/exclude audit. Experiment harness + a small compile API. **Not** a product, **not** a trained model, **no** LangChain.

Arm A is naïve **insertion-order packing from the full candidate corpus** — the LLM never sees a 100K–5M token dump. Every arm uses the same packing budget (default 2000, chars/4).

## How to try (<10 minutes)

No API keys. No Docker. Requires **Go 1.24+** (`go version`; see `go.mod`). `jq` is optional (pretty-print only).

```bash
git clone https://github.com/AminMortezaie/ContextCompiler.git
cd ContextCompiler
go test ./...          # no Docker / API keys
make api               # listens on :8080
# or: go run ./cmd/api -addr :8080
```

In another terminal:

```bash
curl -sS -o /tmp/compile.json -w "HTTP %{http_code}\n" http://localhost:8080/v1/compile \
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
  }'
# expect: HTTP 200
# jq is optional:
jq '{keys: keys, budget_usage, sufficient: .sufficiency.sufficient, edges: (.selected_edges|length), audit_len: (.audit|length), compiled_chars: (.compiled_context|length)}' /tmp/compile.json
```

Success: **HTTP 200** with `compiled_context`, `audit`, `budget_usage` (`token_budget`, `tokens_used`), plus Phase B `sufficiency` and `selected_edges`. Built-in state handle: `day0` (Project X fixture + typed graph). Health check: `GET /healthz`.

Graph expansion try-path (2-hop) and field notes: [`docs/phase-b-graph-compile.md`](docs/phase-b-graph-compile.md).

Optional, still no keys:

```bash
make bench-multitask   # Phase 3 mock suite; Postgres warning + in-memory fallback is expected
```

## What feedback we want

**Blunt try-path feedback**, not compliments. Did `clone → go test → make api → POST /v1/compile` work in under 10 minutes? Where did you get stuck (Go version, Makefile, curl path, missing dep, env surprise)? Did any sentence here overclaim what the LLM saw or what the compiler is?

## Caveats (read before citing numbers)

- **Hash embeddings.** Default embedder is **hash-bow-384** (bag-of-words → FNV buckets → L2). Deterministic and offline-reproducible. It is **not** a neural embedding model.
- **Synthetic fixtures.** Day-0 and the 100K→5M ladder are generated org-state, not production data.
- **Relative $ cost estimator.** Bench `est_cost_usd` is `chars/4` tokens × a constant **$0.002 / 1K tokens**. Use it to compare arms in one run. It is **not** a vendor invoice or tokenizer-accurate cost.
- **Heuristic graph-aware compile.** Arm C / `POST /v1/compile` is a deterministic pipeline: lexical ∪ vector ∪ graph seeds → hop-limited typed expand → one ranker → budget pack (nodes + edges + snippets) → sufficiency checklist. It is **not** a trained model, **not** a memory product, and **not** a Zep/Graphiti clone.

---

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
  }'
```

Response fields: `compiled_context`, `contract`, `audit`, `selected_ids`, `excluded_ids`, `budget_usage` (`token_budget`, `tokens_used`). Phase B adds `sufficiency` and `selected_edges` (additive). Audit rows may include `source`: `lexical` | `vector` | `graph` | `expand` | `permission`. Optional request: `retrieve.topk` / `retrieve.hops` (defaults 32 / 1).

See [`docs/phase-b-graph-compile.md`](docs/phase-b-graph-compile.md). This is heuristic graph-aware compile over a pluggable `memory.GraphStore`, **not** an agent-memory product.

Health check: `GET /healthz`.

---

## Phase B — Graph-aware compile

Heuristic only. Typed `GraphStore` on `memory.Layer`, hybrid candidates, hop-limited expand, one ranker, pack nodes+edges+snippets, deterministic sufficiency. **No** `/v1/remember`, **no** LLM on compile, **no** better-than-Zep claims. How to try 1-hop / 2-hop: [`docs/phase-b-graph-compile.md`](docs/phase-b-graph-compile.md).

---

## Phase 3 — Multi-task science track

Deterministic **narrative task suite** over the same Day-0 Project X org state (causal, decision, ownership, action, plus the original composite delay task). Each task carries golden **relevant entity IDs** and **required answer phrases**.

### Split metrics (every arm, every task)

| Metric | Meaning |
|--------|---------|
| **retrieval_recall** | \|retrieved candidate IDs ∩ golden relevant\| / \|golden relevant\| — after retrieval/ranking, **before** budget-fit packing |
| **context_recall** | \|packed context entity IDs ∩ golden relevant\| / \|golden relevant\| — what actually reached the LLM |
| **task_success** | Mock/live answer contains all `RequiredPhrases` for that task |

Retrieval and context recall can diverge when budget-fit drops high-recall entities (especially Arm B/C). Phase 1 single-task `make bench` uses the same split metrics in run detail and the comparison table (`CTX_RECALL`).

### Run multi-task bench (mock, no API keys)

```bash
go test ./...
make bench-multitask
# or:
go run ./cmd/bench -suite -small -mock -budget 2000
go run ./cmd/bench -suite -small -mock -ablations   # adds C:compiler-no-* single-knob ablations
```

Without Docker, expect a Postgres connection warning; Arm B falls back to the **in-memory** vector store. That is success, not a failed setup.

Phase 1 `make bench` and Phase 2 `make api` / `/v1/compile` are unchanged. Compiler **ablation knobs** live in `internal/compiler.Ablation` and are exercised via `arms.NewArmCAblation` in the multi-task bench only (production compile API stays full pipeline).

---

## Phase 1 — Experiment harness

### Experimental question

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
make bench                                 # ~100K (default TOKENS=100000); mock if no key
make bench MOCK=1                          # force mock even if an API key is set
make bench TOKENS=500000 MOCK=1            # ~500K
make bench TOKENS=1000000 MOCK=1           # ~1M
make bench TOKENS=5000000 MOCK=1           # ~5M
go run ./cmd/bench -tokens 500000 -budget 2000 -mock   # explicit flags
make bench-small MOCK=1                    # Day-0 tiny fixture (-small)
```

`make bench -mock` does **not** pass `-mock` into the binary (GNU make eats `-m` / `-o`). Use `MOCK=1` or `go run ./cmd/bench -mock`.

See `VISION.md`.

## Stack

Go + PostgreSQL + pgvector + one LLM API. **No** LangChain / LangGraph.

## Bench flags and optional Postgres

```bash
make db-up                 # optional: docker compose up -d
make bench MOCK=1          # or: go run ./cmd/bench -mock
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
| `-suite` | false | Phase 3 multi-task suite (`fixture.Day0TaskSuite`) |
| `-ablations` | false | Add single-knob `C:compiler-no-*` arms (usually with `-suite`) |

### Real LLM

Keys load from `.env` (and optionally `/home/box/.config/context-compiler/env`). Do **not** commit secrets.

```bash
# GROQ_API_KEY=...  OPENAI_BASE_URL=https://api.groq.com/openai/v1  OPENAI_MODEL=openai/gpt-oss-20b
go run ./cmd/bench -tokens 500000 -budget 2000
go run ./cmd/bench -mock -tokens 500000
```

Without a key, mock is used and a warning is printed.

### Embeddings

**hash-bow-384** (default): deterministic bag-of-words → FNV buckets → L2; offline-reproducible bakeoff path (dim 384). Not a neural embedder — see [Caveats](#caveats-read-before-citing-numbers).

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

Every run prints **task_success**, **retrieval_recall**, **context_recall**, token/cost estimates (see [Caveats](#caveats-read-before-citing-numbers)), latency, **compile_ms vs llm_ms**, **overhead_pct_of_e2e_latency**, and irrelevant ratio. Indexing for Arm B runs once in the bench harness before arms and is not counted in per-arm `compile_ms`.

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
  cmd/bench/            # Phase 1 bakeoff; -suite / -ablations for Phase 3
  internal/api/
  internal/compiler/    # hybrid gather → expand → rank → pack + sufficiency
  internal/memory/      # Layer + optional GraphStore (v0 in-memory)
  docs/                 # Phase A research notes + Phase B compile notes
  internal/permissions/
  internal/arms/        # A / B / C
  internal/eval/
  internal/embed/       # hash-bow-384
  internal/fixture/     # Day0 + Scale(...)
  internal/llm/
  internal/store/
  migrations/001_init.sql
```
