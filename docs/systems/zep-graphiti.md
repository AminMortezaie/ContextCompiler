# Zep / Graphiti

Public sources: [Graphiti overview](https://help.getzep.com/graphiti/graphiti/overview), [Searching the Graph](https://help.getzep.com/searching-the-graph), [bi-temporal model](https://getzep-graphiti.mintlify.app/concepts/temporal-model), [Zep: A Temporal Knowledge Graph Architecture for Agent Memory](https://arxiv.org/html/2501.13956), [graphiti search recipes](https://github.com/getzep/graphiti/blob/main/graphiti_core/search/search_config_recipes.py).

## What it is (do not clone)

**Graphiti** is the open-source temporal knowledge-graph framework: ingest episodes, extract/resolve entities and fact edges, invalidate contradictions, hybrid-search the graph.

**Zep** is the commercial agent-memory service built on Graphiti (managed extraction, retrieval, governance, “Context Lake”). Graphiti docs contrast the two explicitly: Graphiti = one Context Graph per subject locally; Zep = enterprise-scale managed memory.

Context Compiler should treat Graphiti as the **closest retrieval design** to steal from, and Zep as a **possible later `memory.Layer` adapter** — not as a product to copy.

## Architecture overview

Three-tier graph ([paper §2](https://arxiv.org/html/2501.13956)):

1. **Episode subgraph** — raw messages / text / JSON. Non-lossy store. Episodic edges (`MENTIONS`) link an episode to extracted entities.
2. **Semantic entity subgraph** — deduplicated `EntityNode`s + `EntityEdge` facts (`RELATES_TO`) with natural-language `fact` text.
3. **Community subgraph** — clusters of strongly connected entities with summaries (GraphRAG-inspired; Graphiti uses label propagation + dynamic assignment, then periodic refresh).

Ingest path (`add_episode`, described in Graphiti concepts + paper):

1. Fetch last *n* episodes (paper: *n*=4 messages / two turns; Graphiti code comments mention a default episode window of 3) as extraction context.
2. LLM entity extraction + reflexion pass; embed names; hybrid recall of similar existing nodes; LLM entity resolution.
3. Edge/fact extraction; dedup only against edges **between the same entity pair**.
4. Temporal extraction from `reference_time`; contradiction check; invalidate old edges (set `invalid_at` / `expired_at`) instead of deleting them.
5. Optional community membership update.

Writes are LLM-heavy. **Reads** (Graphiti `search`) are hybrid index lookups plus rerank — default path advertises **no LLM-in-the-loop reranking**.

Zep Cloud adds **Auto Search** (`scope="auto"`): parallel retrieve across edges, nodes, episodes, observations, and thread summaries → cross-scope rerank → pack a character budget into one `context` string ([searching the graph](https://help.getzep.com/searching-the-graph)).

## Data model

| Object | Role | Time |
|--------|------|------|
| `EpisodicNode` | Raw episode (`content`, `source`, `source_description`, `group_id`) | `valid_at` (when it occurred), `created_at` (ingest) |
| `EpisodicEdge` (`MENTIONS`) | Provenance: episode ↔ entity | — |
| `EntityNode` | Canonical entity (`name`, `labels` / types, summary, embedding) | `created_at` only — validity lives on edges |
| `EntityEdge` | Fact between two entities (`fact`, embeddings) | **Bi-temporal:** `valid_at` / `invalid_at` (world) + `created_at` / `expired_at` (system) |
| Community node | Cluster summary + keyword-ish name for search | Rebuilt / incrementally extended |

Facts can be extracted multiple times between different entity pairs (paper: a hyper-edge-like encoding of multi-entity facts).

**Unknown:** exact production schema of Zep Cloud “observations” vs Graphiti OSS communities; Auto Search rerank weights.

## Retrieval + compress / summarize

**Graphiti OSS search** (configurable recipes):

- Methods: cosine similarity, BM25, optional BFS.
- Targets: edges (search `fact`), nodes (search name/summary), episodes (BM25), communities (name + cosine).
- Rerankers: RRF (default hybrid), MMR, `node_distance` from a centroid UUID, `episode_mentions` (frequency), optional cross-encoder.
- Default `search()`: `EDGE_HYBRID_SEARCH_RRF`, or `EDGE_HYBRID_SEARCH_NODE_DISTANCE` if `center_node_uuid` is set.
- If embeddings are down, Zep docs say search continues **BM25-only**.

**Constructor** (paper §3): format top edges as `FACT (Date range: from–to)` plus entity name/summary (and optionally community summary) into one context string.

**Zep Auto Search:** `max_characters` default **2500**, cap **50000**. Queries truncated at **400 characters**. Can interpret relative time (“last week”) as a retrieval window; caller-supplied datetime filters win. Optional `return_raw_results` for citation.

**Compress:** not a Mastra-style observation log. Compression is *structural*: store facts + summaries, keep raw episodes for provenance. Community summaries are the GraphRAG-like rollup.

## How relevance is decided

1. Recall: union of lexical + vector (+ BFS neighborhood / recent-episode seeds).
2. Precision: RRF or graph-local rerank (distance to user/entity centroid, mention frequency).
3. Optional cross-encoder for quality at higher latency.
4. Temporal filters (`valid_time`) so “who is CEO?” can mean “as of date T.”
5. Auto Search: unpublished cross-scope rerank, then **budget pack**.

Relevance is **query-conditioned retrieval**, not “keep a stable prefix.”

## Failure modes / limits

- **Ingest cost / lag.** Every episode is one or more LLM calls (extract, resolve, contradict). Not free, not instant.
- **Entity resolution errors.** Paper uses hybrid recall + LLM merge; collisions and splits are inherent.
- **Invalidation is LLM-judged.** Wrong contradiction → silent history damage (mitigated by keeping invalidated edges, but retrieval must filter them).
- **Community drift.** Dynamic label-propagation assignment diverges from a full rebuild; periodic refresh required (paper §2.3).
- **Query design.** Long queries are truncated (Zep: 400 chars). Complex questions want multiple targeted searches — Auto Search is their answer to that UX.
- **Backend ops.** Self-hosting Graphiti implies Neo4j / FalkorDB / Neptune (or similar). Hindsight’s blog argues this is the real self-host cost. We should not take that dependency in Phase B.
- **Benchmarks are not our problem.** Graphiti/Zep publish LoCoMo / LongMemEval numbers. Those measure *chat memory*, not org-state compilation under a 2k pack budget. Do not treat their scores as a compiler target.

## Steal vs avoid (Context Compiler)

**Steal**

- Episode (or ticket/decision source) as **non-lossy provenance**, facts as packable edges.
- Bi-temporal fields *if* the memory backend has them; compile should pass `valid_at`/`invalid_at` through the audit when present.
- Hybrid BM25 + vector + 1-hop BFS, then **one** ranker. Default compile path: RRF-class fusion, no cross-encoder.
- Centroid / seed-entity distance as a rank feature (our `project_hints` + `Seeds` in the Phase B `Query`).
- Character/token **budget constructor** that mixes facts + entity summaries (Zep Auto Search is the productized version of what `budgetFit` already does for flat entities).
- `group_id` / user-graph isolation → our handle + permissions.

**Avoid**

- Rebuilding Graphiti (drivers, `add_episode`, community maintenance, custom entity-type Pydantic models).
- LLM extract/resolve/invalidate on `POST /v1/compile`.
- Marketing “Context Graph / Context Lake” as our product.
- Treating community summaries as required for Day-0 causal tasks (local search is enough).
- Hardcoding Neo4j into the compiler.
