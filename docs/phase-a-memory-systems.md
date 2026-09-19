# Phase A — Live memory systems (engineering notes)

Research notes for upgrading the **Context Compiler** path so graphs are first-class, without building a Zep / Graphiti product clone.

**Not a product brief.** These notes reverse-engineer public systems so Phase B can extend `POST /v1/compile` (task contract → candidates → rank → budget → auditable pack) above a pluggable memory layer. We do not claim to beat any of these systems. Benchmarks below are **their** published numbers, not ours.

**Out of scope:** LangChain / LangGraph, RotD / outreach copy, a competing “agent memory” SaaS.

**Today (v0 compiler):** `internal/compiler` ranks in-memory `state.Entity` rows with kind priors, keyword hits, project hints, and a hardcoded `RefIDs` boost (`proj-x` / `dec-001` / `tkt-042`), then density-packs into a token budget with an include/exclude audit. `internal/memory.Layer` is `Load(handle) []Entity`. There is no lexical+vector hybrid retrieval, no typed graph, no sufficiency gate, and no episode provenance.

**Target compiler shape (Phase B+):** hybrid lexical + vector + graph candidates → dependency expansion → rank → budget → sufficiency → auditable pack, still sitting **above** a pluggable memory backend.

---

## How to read this

| File | What it is |
|------|------------|
| This document | Cross-system comparison, steal/avoid synthesis, Phase B interface sketch |
| [`systems/zep-graphiti.md`](systems/zep-graphiti.md) | Zep product + Graphiti engine |
| [`systems/mastra.md`](systems/mastra.md) | Mastra Observational Memory |
| [`systems/mem0.md`](systems/mem0.md) | Mem0 (Platform vs OSS) |
| [`systems/microsoft-graphrag.md`](systems/microsoft-graphrag.md) | Microsoft GraphRAG |
| [`systems/hindsight.md`](systems/hindsight.md) | Hindsight (Vectorize) |
| [`systems/supermemory.md`](systems/supermemory.md) | Supermemory |

Every architecture claim is cited. Where public docs are thin, that is marked **Unknown**.

---

## Context Compiler lens

We are not shopping for a memory product to resell. We are asking: *what retrieval / graph / compress ideas belong inside the compiler, and what belongs in a backend we can plug in later?*

Current beachhead (`README.md`, `internal/api`):

```
POST /v1/compile
  task_contract + state.handle|entities + budget + permissions
  → compiled_context + audit + selected_ids + excluded_ids + budget_usage
```

Memory is already declared pluggable (`internal/memory.Layer`). Phase B should keep that split: **compiler owns selection policy; memory owns storage and optional graph indexes.**

---

## Comparison (compiler-relevant)

| System | Primary job | Data model | Retrieval / compress | Relevance | Graph? | Fits our product? |
|--------|-------------|------------|----------------------|-----------|--------|-------------------|
| **Zep / Graphiti** | Agent memory layer + temporal KG engine | Episodes → entity nodes + fact edges + communities; bi-temporal facts | Hybrid BM25 + cosine + BFS; RRF / node-distance / optional cross-encoder; Zep Auto Search packs a character budget | Fusion + rerank; temporal filters; centroid distance | First-class temporal KG | **Steal retrieval + provenance.** Do not become Zep. |
| **Mastra OM** | Long-thread context compression | Dated observation log + reflections; optional raw-message ranges | Default: **no query-time retrieval**. Optional `recall` tool + vector search | Observer/Reflector LLM judgment; token thresholds | None (text log) | **Steal compress + cacheable prefix.** Not a graph. |
| **Mem0** | Extract-then-search memory API | Additive facts + entity store; Platform graph is co-occurrence | Semantic + BM25 + entity boost + temporal (Platform); app packs the prompt | Fused `score`; filters by `user_id` / `agent_id` / `run_id` | Platform: schema-free entity graph. OSS: entity overlap only | **Steal add/search split + scoping.** Avoid silent “we have a graph.” |
| **MS GraphRAG** | Corpus index + query-focused summarization | TextUnits → entities / relationships / claims → Leiden communities + reports | Local (entity fan-out + budget pack); Global (map-reduce reports); DRIFT; Basic RAG | Embedding seeds + ranking/filtering to a context window; map-step importance ratings | Static hierarchical KG | **Steal community reports + local fan-out.** Avoid batch-only index as the product. |
| **Hindsight** | Retain / recall / reflect memory banks | World vs experience facts; entities; 4 link types; observations | TEMPR: semantic + BM25 + graph + temporal → RRF → cross-encoder; `budget` | Fused + reranked; tag isolation | Entity/temporal/semantic/causal links on Postgres | **Steal 4-way hybrid + write-time bounded expansion.** Avoid disposition/reflect product. |
| **Supermemory** | Managed ingest + living fact graph | Documents/chunks + atomic memories (`updates` / `extends` / `derives`) + profile | Hybrid vector + FTS; optional rerank / related memories / temporal validity | Threshold + rerank + `isLatest`; container isolation | Fact-on-fact graph (not classic E-R-E) | **Steal latest-vs-history + related expansion.** Avoid proprietary “dreaming” as a product. |

---

## Steal vs avoid (synthesis)

### Steal for the compiler path

1. **Hybrid candidate generation, then one ranker.** Graphiti, Hindsight, Mem0 Platform, and Zep Auto Search all refuse to pick BM25 *or* vectors *or* graph. Our Arm B is vector-only; Arm C is keyword/kind/ref. Phase B should union lexical + vector + graph seeds, then rank once.
2. **Graph as expansion, not as the only index.** Graphiti BFS from recent episodes / a centroid, GraphRAG local fan-out, Hindsight write-time-bounded link expansion, Supermemory `include.relatedMemories`. Our `RefIDs` boost is the stub of this — but it is score-only and hardcoded to Day-0 IDs.
3. **Provenance / non-lossy source.** Graphiti keeps episodes and `MENTIONS`; Mastra retrieval mode stores `startId:endId` ranges; Hindsight observations cite source quotes; GraphRAG TextUnits back answers. Our audit says *why packed*, not *where the fact came from*.
4. **Explicit budget packing of mixed types.** Zep Auto Search (`max_characters`, default 2500, cap 50000) and GraphRAG local search both pack heterogeneous objects (facts, summaries, chunks, reports) into one window. We already budget-fit entities. Phase B should pack **nodes + edges + optional episode snippets** under the same `token_budget`.
5. **Sufficiency / “do we have enough?”** None of these products expose a first-class sufficiency API, but several imply it: Mastra’s observation log is “always in context” (no miss if it was observed); GraphRAG global map-reduce is “cover the corpus”; Hindsight `reflect` loops until the agent stops. We should make sufficiency **compiler-owned and auditable**, not an implicit LLM loop.
6. **Compress off the hot path.** Mastra Observer/Reflector and Hindsight observation consolidation run in the background. Compile latency is our experiment proxy (`compile_ms` vs `llm_ms`). Do not put LLM extraction on `POST /v1/compile`.
7. **Scope before relevance.** Mem0 `user_id` / `run_id`, Hindsight banks + tags, Supermemory `containerTag`, Graphiti `group_id`. Our `permissions` hook is the right place — extend it, do not invent a second ACL.

### Avoid

1. **Shipping a memory SaaS.** Zep, Mem0, Hindsight, and Supermemory *are* the memory layer. We compile **over** one.
2. **LLM-in-the-loop on every compile.** Graphiti’s default hybrid search advertises no LLM rerank; Zep/Hindsight optional cross-encoders add latency/cost. Keep compile local (Go), same as v0.
3. **Batch-only GraphRAG as the org-state store.** Great for static corpus sensemaking; poor for continuously updated tickets/decisions. Graphiti’s own docs call this out.
4. **Schema-free co-occurrence pretending to be a dependency graph.** Mem0 Platform links memories that share an entity; it does **not** store typed `owns` / `blocks` / `decided` edges. Our Day-0 tasks are causal/ownership/action — we need typed refs.
5. **Silent invalidation / derived facts without audit.** Supermemory `derives` and Graphiti edge invalidation are powerful and easy to lie about. If we invalidate or infer, the pack must say so.
6. **Hardcoded fixture IDs as “graph.”** `ref-boost` on `proj-x` / `dec-001` / `tkt-042` is eval scaffolding, not a graph.
7. **LangChain / agent-framework memory wrappers.** Stack constraint stands.

---

## Implications for Phase B

Goal: graphs inside the **compiler**, not a new memory product. Beachhead stays `POST /v1/compile`.

### What does not change

- HTTP: `POST /v1/compile`, `GET /healthz`.
- Request: `task_contract`, `state.handle | state.entities`, `budget.token_budget`, `permissions`.
- Response: `compiled_context`, `contract`, `audit`, `selected_ids`, `excluded_ids`, `budget_usage`.
- Memory remains a **handle layer**. Inline entities still bypass it.
- No LangChain. Compile stays a local Go function (no LLM spend on the hot path).
- Arms A/B stay baselines. Arm C / compile API share the same compiler.

### What changes inside compile

Replace “load all entities → keyword rank → density pack” with an explicit pipeline. Suggested internal stages (names are sketch, not API):

```
TaskContract
    │
    ▼
CandidateSet.Retrieve     // hybrid: lexical ∪ vector ∪ graph seeds
    │
    ▼
CandidateSet.Expand       // typed dependency / 1-hop (maybe 2-hop cap)
    │
    ▼
CandidateSet.Rank         // existing multi-signal + new graph features
    │
    ▼
BudgetFit                 // density pack (already exists)
    │
    ▼
Sufficiency               // contract coverage check; optional second retrieve
    │
    ▼
Pack + Audit              // include/exclude reasons stay mandatory
```

### Interface sketch (compiler-owned, memory-pluggable)

Keep `memory.Layer` as the backend. Grow it *behind* compile; do not add `POST /v1/remember`.

```go
// internal/memory — Phase B sketch (not implemented)

type Node struct {
    Entity state.Entity
    // optional: embeddings live in the store, not necessarily here
}

type Edge struct {
    ID         string
    FromID     string
    ToID       string
    Rel        string            // "owns", "decided", "blocks", "mentions", ...
    Fact       string            // optional natural-language fact
    ValidAt    *time.Time        // optional; memory may not have time
    InvalidAt  *time.Time
    EpisodeIDs []string          // provenance, if the backend has episodes
}

type Episode struct {
    ID      string
    Text    string
    ValidAt *time.Time
    Source  string
}

type Neighborhood struct {
    Nodes    []Node
    Edges    []Edge
    Episodes []Episode // may be empty
}

type Query struct {
    Text     string
    Kinds    []string
    Keywords []string
    Seeds    []string          // known entity IDs from the contract
    Limit    int
}

// Layer stays the v0 contract. GraphStore is optional.
type Layer interface {
    Load(ctx context.Context, handle string) ([]state.Entity, error)
}

type GraphStore interface {
    Layer
    Search(ctx context.Context, handle string, q Query) (Neighborhood, error)
    Neighbors(ctx context.Context, handle string, ids []string, hops int) (Neighborhood, error)
}
```

`compiler.Compile` signature can stay `([]state.Entity, TaskContract, Options)`. Phase B adds an overload or `Options.Graph` so the API handler can do:

1. Resolve handle via `Layer.Load` **or** `GraphStore.Search` (hybrid seeds).
2. If the backend is not a `GraphStore`, synthesize a neighborhood from `Entity.RefIDs` (typed later; today’s `RefIDs` are untyped).
3. Expand → rank → budget → sufficiency → pack.
4. Audit rows gain `source` (`lexical` | `vector` | `graph` | `expand` | `permission`) and optional `edge_id` / `episode_id`.

HTTP stays the same. Optional later fields (additive, not required for Phase B merge):

```json
{
  "task_contract": { "question": "...", "required_kinds": ["decision", "ticket"] },
  "state": { "handle": "day0" },
  "budget": { "token_budget": 2000 },
  "retrieve": { "topk": 32, "hops": 1 },
  "sufficiency": { "require_kinds": ["decision"], "min_seed_hits": 1 }
}
```

If those fields are omitted, compile behaves as today plus graph expansion when `RefIDs` / `GraphStore` exist.

### Sufficiency (new, compiler-owned)

A cheap, deterministic check after the first pack, **before** any LLM:

- Required kinds from the contract present in `selected_ids`?
- At least one project-hint / seed entity packed?
- If the contract looks causal (“why / delayed / caused”) and no `decision`/`ticket` edge was expanded, mark `sufficient=false` and either (a) expand one more hop or (b) return that flag in the audit / a new response field.

Do **not** start an agentic retrieve loop in Phase B. That is Hindsight `reflect` / GraphRAG global map-reduce — useful later, expensive, and off-beachhead.

### Data-model upgrade (minimal)

Today: `Entity{ID, Kind, Title, Text, RefIDs, Meta}`.

Phase B minimum:

- Keep `Entity` as the packable unit (nodes).
- Promote `RefIDs` to typed edges **inside the compiler’s neighborhood**, even if the in-memory fixture still stores `RefIDs []string`.
- Do not require episodes in v0 fixtures. Episode IDs are optional provenance.
- Do not add community reports unless a bench task needs corpus-level themes (GraphRAG global). Day-0 / scale-ladder tasks are local/causal, not “top themes in the corpus.”

### What we will not build in Phase B

- A Graphiti-compatible temporal KG service.
- LLM entity extraction on ingest (Mem0 `add`, Graphiti `add_episode`, Hindsight `retain`, Supermemory dreaming). If we need extraction later, it is a **memory adapter**, not compile.
- Cross-encoder rerank on the compile hot path.
- Working-memory / observation-log product (Mastra). We may *consume* a compressed handle later.

### Suggested Phase B slices (implementation order)

1. **Typed expansion:** replace hardcoded ID boosts with hop-limited walk over `RefIDs` / edges; ablation already has `NoRefExpansion`.
2. **Hybrid candidates:** lexical (existing keywords) ∪ vector (Arm B store) ∪ graph seeds; union then rank.
3. **Audit sources:** each include/exclude names the stage that produced the candidate.
4. **Sufficiency flag** on `compiler.Result` (additive JSON).
5. **`GraphStore` interface** with in-memory implementation; Zep/Mem0/Hindsight adapters are later and optional.

---

## Unknowns (honest)

- Exact Zep Cloud Auto Search cross-scope rerank weights — unpublished; only the behavior is documented.
- Mem0 Platform fusion formula (semantic vs BM25 vs entity vs temporal) — not public.
- Supermemory “custom learning model” and internal graph DB — proprietary; we only have the documented fact/edge types and search API.
- Whether Mastra OM’s LongMemEval numbers transfer to **org-state compilation** (entities/tickets/decisions, not chat transcripts). Different problem.
- Hindsight link-expansion fan-out bounds and scoring weights — described qualitatively in the 2026 parallel-search blog, not as a spec.
- GraphRAG Leiden stability / incremental update story for a live org graph — docs assume re-index; incremental community refresh is a research problem Graphiti also only partially solves (label propagation + periodic refresh).

---

## Source index

Primary public docs used in this survey:

- Graphiti / Zep: [Graphiti overview](https://help.getzep.com/graphiti/graphiti/overview), [Searching the Graph](https://help.getzep.com/searching-the-graph), [bi-temporal model](https://getzep-graphiti.mintlify.app/concepts/temporal-model), [Zep paper](https://arxiv.org/html/2501.13956), [search recipes](https://github.com/getzep/graphiti/blob/main/graphiti_core/search/search_config_recipes.py)
- Mastra: [Memory overview](https://mastra.ai/docs/memory/overview), [Observational Memory](https://mastra.ai/docs/memory/observational-memory), [OM research writeup](https://mastra.ai/research/observational-memory)
- Mem0: [How it works](https://docs.mem0.ai/core-concepts/how-it-works), [Graph Memory](https://docs.mem0.ai/platform/features/graph-memory), [Add](https://docs.mem0.ai/core-concepts/memory-operations/add)
- GraphRAG: [Welcome](https://microsoft.github.io/graphrag/), [Local search](https://microsoft.github.io/graphrag/query/local_search/), [Global search](https://microsoft.github.io/graphrag/query/global_search/), [paper](https://arxiv.org/abs/2404.16130)
- Hindsight: [Overview](https://hindsight.vectorize.io/), [Retain](https://hindsight.vectorize.io/developer/retain), [Recall API](https://hindsight.vectorize.io/developer/api/recall), [parallel hybrid blog](https://hindsight.vectorize.io/blog/2026/03/27/parallel-hybrid-search)
- Supermemory: [How it works](https://supermemory.ai/docs/concepts/how-it-works), [Graph memory](https://supermemory.ai/docs/concepts/graph-memory), [Search](https://supermemory.ai/docs/memory-api/searching/searching-memories)
