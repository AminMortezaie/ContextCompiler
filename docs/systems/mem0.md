# Mem0

Public sources: [How Mem0 Works](https://docs.mem0.ai/core-concepts/how-it-works), [Add](https://docs.mem0.ai/core-concepts/memory-operations/add), [Graph Memory](https://docs.mem0.ai/platform/features/graph-memory).

## Architecture overview

Mem0 is a **memory API**, not a compiler. The app:

1. `add(messages, user_id, …)` after a turn — extract durable facts.
2. `search(query, filters)` before the next model call — get ranked memories.
3. **The app decides** which rows go into the prompt.

Two implementations that must not be conflated:

| | **Mem0 Platform** | **Mem0 OSS** |
|--|-------------------|--------------|
| Graph | Native, always-on entity graph; no Neo4j to provision | No graph memory; entity **overlap boost** only |
| Retrieval | Fuses semantic + BM25 + entity + temporal in the service | Your vector store + optional reranker + entity overlap |
| History | Service pulls prior messages for the same identifiers as extract context | You pass what you have |

Older OSS/Platform `enable_graph` + external Neo4j/Memgraph/Kuzu/AGE/Neptune is **deprecated**. Platform `relations` is now an empty compatibility field. Connections are folded into the combined `score`.

## Data model

**Messages** (input) vs **memories** (stored facts). Default `infer=True`: LLM extracts preferences, decisions, plans. `infer=False`: store raw text; skip extract/dedup; duplicates allowed.

Stores ([how it works](https://docs.mem0.ai/core-concepts/how-it-works)):

| Store | Holds |
|-------|--------|
| SQL | Facts + metadata (source of truth) |
| Vector | Embeddings |
| Entity store | People / places / orgs / concepts mentioned in a memory |

**Platform graph** ([Graph Memory](https://docs.mem0.ai/platform/features/graph-memory)):

- **Graph entity node** — extracted proper noun / key phrase (embedded, resolved).
- **Memory node** — one stored fact.
- **Connection** — entity ↔ memories that mention it. Two entities are “related” if they **co-occur**. There are **no typed predicates** (`manages`, `blocks`, …).

Scoping IDs (`user_id`, `agent_id`, `app_id`, `run_id`) are **not** graph entities. They partition records. Cross-entity queries do not magically union partitions.

Write algorithm is described as **additive / ADD-only**: “I moved from Austin to Seattle” can store the new fact without silently rewriting the old. Correction is explicit `update` / `delete`. Optional `expiration_date` hides memories from `search` / `get_all` unless `show_expired`.

**Unknown:** Platform fusion weights; entity-resolution algorithm; how “multi-hop” is implemented beyond “boost memories sharing query entities.”

## Retrieval + compress / summarize

There is no Mastra-style observation log and no GraphRAG community report.

**Write-side compress:** extraction *is* the compression (chat → short facts). Context lookup at add-time avoids storing the same fact again (dedup, not invalidation).

**Read-side:** `search` returns ranked memories. No official “pack to N tokens” constructor — that is the caller’s job (this is the closest cousin to our compiler).

Platform signals:

| Signal | Use |
|--------|-----|
| Semantic | Conceptual match |
| Keyword (BM25) | Names, IDs |
| Entity | Boost memories linked to query entities (graph) |
| Temporal | Time metadata vs query intent (“when”, current state, recency) |

## How relevance is decided

A single fused `score` per memory after filters. Entity graph **changes ranking, not response shape**. Always filter by `user_id` (and friends) or you mix tenants.

OSS relevance ≈ vector (and optional rerank) + entity-overlap boost. Do not document OSS as “has Graph Memory.”

## Failure modes / limits

- **Extraction loss.** Decisions that were implicit in a long thread may never become a memory.
- **Additive history vs “current state.”** Without `update`/`delete` or expiration, stale facts still retrieve. Temporal scoring may down-rank them; it does not remove them.
- **Co-occurrence ≠ dependency.** “Alice” and “Project X” in one memory does not encode *Alice approved the freeze*. Our Day-0 tasks need typed edges.
- **Platform / OSS split.** A local adapter that speaks OSS will not reproduce Platform graph behavior.
- **`infer=False` + `infer=True`** on the same content can double-store.
- **Secrets.** Their docs warn: do not store credentials. Same for any compiler handle.
- **Latency.** Managed extract is async-ish from the app’s point of view (processing delay is listed as a Platform tradeoff in their architecture notes). Compile must not wait on that.

## Steal vs avoid (Context Compiler)

**Steal**

- Clean **add vs search vs pack** split. We already own pack (`Compile`). Memory backends can look like Mem0 `search`; they must not own the budget audit.
- **Hybrid signals** (semantic + keyword + entity + time) as candidate generators.
- **Hard scoping** (`handle` / org / user) before ranking — maps to `state.handle` + `permissions`.
- **Additive facts + explicit mutate** rather than silent overwrite (pairs with Graphiti-style invalidation *when the backend supports it*).
- Entity extraction as a **backend** feature we can consume, not implement in v0.

**Avoid**

- Advertising a “graph” that is only co-occurrence.
- Letting `search()` top-k *be* the compiled context (that is Arm B).
- Depending on Platform-only graph for Phase B correctness.
- LLM `add()` on the compile hot path.
- Re-introducing an external graph-DB config in *our* repo to mimic old Mem0.
