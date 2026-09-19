# Hindsight (Vectorize)

Public sources: [Overview](https://hindsight.vectorize.io/), [Retain](https://hindsight.vectorize.io/developer/retain), [Recall API](https://hindsight.vectorize.io/developer/api/recall), [Memory banks](https://hindsight.vectorize.io/developer/api/memory-banks), [Reflect](https://hindsight.vectorize.io/developer/api/reflect), [4-way hybrid blog (2026-03-27)](https://hindsight.vectorize.io/blog/2026/03/27/parallel-hybrid-search), [self-host / Postgres argument](https://hindsight.vectorize.io/blog/2026/05/12/case-against-external-vector-dbs-agent-memory), [API README](https://github.com/vectorize-io/hindsight/blob/main/hindsight-api/README.md).

Hindsight is publicly documented enough for Phase A. It is an **agent memory product** (retain / recall / reflect), not a compiler.

## Architecture overview

One isolated **memory bank** per subject (user, tenant, agent, project). API surface:

| Call | Role |
|------|------|
| `retain` | LLM-extract facts, entities, time, links; index for search |
| `recall` | Four-way retrieve → fuse → rerank → ranked **facts** (not raw docs) |
| `reflect` | Agentic loop over the bank; disposition / directives / mental models |

Storage pitch: **PostgreSQL + pgvector** (semantic + BM25 + temporal + graph in one DB). Their blog contrasts this with self-hosted Mem0 (extra vector DB) and Graphiti (Neo4j/FalkorDB/Kuzu). Useful as an ops note; not a reason to adopt Hindsight.

After retain, **observation consolidation** runs in the background: merge overlapping facts, keep evidence quotes + proof counts, refine instead of overwrite, mark observations stale when newer raw facts exist.

`reflect` is out of scope for Phase B (LLM loop, personality traits). `recall` is the interesting retrieval design.

## Data model

**Fact types** (perspective, not grammar):

- **world** — other people / things / events.
- **experience** — the *bank’s agent* acting. Speaker `context` is required so “I bought a Tesla” said by a user stays a world fact about them.

**Entities:** people, orgs, places, products/concepts. Fuzzy name merge, reinforced by co-occurrence + temporal proximity. Short names can be absorbed into the wrong long-lived entity (they document this). `entity_labels` (`key:value`) never fuzzy-merge.

**Four link types** ([retain](https://hindsight.vectorize.io/developer/retain)):

| Link | Enables |
|------|---------|
| Entity | “everything about Alice” |
| Time | “what else happened around then” |
| Meaning / semantic | thematically related facts |
| Causal | “why” chains |

**Time:** event occurrence vs *when learned* (same bi-temporal intuition as Graphiti, different vocabulary).

**Tags / banks:** bank = hard isolation; tags = soft visibility inside a bank. Directives and disposition apply to **reflect only**.

**Observations / mental models:** consolidated beliefs; mental models are user-curated summaries for common queries (docs mention them on the overview). Treat internals beyond that as **Unknown**.

Retain can be steered with `retain_mission` and extraction modes (`concise`, `verbose`, `custom`, `verbatim`, `chunks`). A tight mission can yield **zero memories** — document stored, but recall/reflect cannot find it. Extraction is not fully deterministic.

## Retrieval + compress / summarize

**TEMPR recall** (parallel):

1. **Semantic** — vector similarity.
2. **Keyword** — BM25.
3. **Graph** — entity / temporal / causal (and semantic) links.
4. **Temporal** — parse a time window; pick by **relevance inside the window**, then spread across buckets so a “what happened in 2023?” query does not collapse to the densest week.

Then: **RRF** merge → **cross-encoder** rerank vs the raw query → multiplicative boosts (they mention recency / temporal / evidence). Queries > **500 tokens** rejected.

The 2026 blog: early graph used multi-hop BFS (O(hops) queries). They replaced it with **link expansion** — fan-out **bounded at retain time**, one CTE at read time. Semantic+BM25+temporal share one DB connection; graph runs separately (pool contention).

`recall` takes a `budget` (`low` / `mid` / `high`) mapped by `recall_budget_function` to an internal thinking budget per retriever. This is **retrieval breadth**, not our LLM pack budget — do not confuse the two.

**Compress:** retain extraction + background observations. `chunks` mode skips LLM. `reflect` synthesizes; we do not want that on compile.

## How relevance is decided

Documented pipeline: four ranked lists → RRF (position, not calibrated scores) → cross-encoder pair score → boosts. Response `scores` are for debugging / cutting a tail, **not** probabilities.

Temporal relevance ≠ recency: in-window semantic pick first, then diversity across the window.

Tag filters apply before/with retrieval (bank isolation is absolute).

## Failure modes / limits

- **Extract/mission miss.** Zero facts ⇒ invisible document. Borderline docs flap across runs.
- **Entity merge errors.** Common names + shared coworkers → wrong person.
- **Cross-encoder on every recall.** Quality vs latency/cost; Graphiti’s default avoids this. Our compile path should too.
- **Graph only as good as write-time links.** Bounded expansion cannot discover an edge nobody extracted.
- **Reflect ≠ compile.** Disposition (skepticism, etc.) must not leak into an audit pack.
- **Published LongMemEval rows** (Mastra’s table lists Hindsight 91.4% with gemini-3-pro, etc.) are chat-memory scores. Not a compiler target.
- **Unknown:** exact RRF k, boost formulas, link fan-out caps, observation consolidation triggers.

## Steal vs avoid (Context Compiler)

**Steal**

- **Four candidate families** that match our target: lexical + vector + graph + (optional) time.
- **Write-time bounded expansion** so query-time hops are cheap and cap-able — better than unbounded BFS on a 5M-token fixture.
- **Causal + entity links** as first-class edge types (our Day-0 “why delayed / who decided”).
- **Occurrence time vs ingest time** on edges when the backend has them.
- **Bank/tag isolation** → handle + permissions.
- Returning **structured facts with scores** for an outer packer (their recall is a `search`; we still pack).
- Postgres/pgvector as a *possible* GraphStore implementation (we already have pgvector for Arm B) — without taking their product.

**Avoid**

- Implementing retain/reflect, missions, disposition, Memory Defense, VLM attachments.
- Cross-encoder + agentic reflect inside `POST /v1/compile`.
- Treating `recall.budget` as a substitute for `token_budget`.
- Building “Hindsight but in Go” as Phase B.
