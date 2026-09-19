# Supermemory

Public sources: [How it works](https://supermemory.ai/docs/concepts/how-it-works), [Graph memory](https://supermemory.ai/docs/concepts/graph-memory), [Searching memories](https://supermemory.ai/docs/memory-api/searching/searching-memories), [hybrid search blog (Apr 2026)](https://supermemory.ai/blog/hybrid-search-guide/).

Documented enough for Phase A **at the API/model level**. The “custom learning model” and internal graph engine are proprietary — do not pretend we reverse-engineered those.

## Architecture overview

Managed ingest → two indexes, one isolation key:

1. **Document path:** queued → extract (OCR/ASR/fetch) → chunk → embed → index. `status: "done"` means **chunks** are searchable (RAG / “SuperRAG”).
2. **Dreaming:** second phase that writes **memories** (graph facts). Default `dreaming: "dynamic"` groups related documents (e.g. a session) before extracting; may continue **after** `done`. `dreaming: "instant"` processes this document alone (extra billed operation; demos).

Third output: **Profile** — sample of memories + static/dynamic summary for always-on context (Profile API). Isolation: `containerTag` (hard), metadata filters (soft), scoped API keys.

This is a **memory + RAG product**. The compiler should treat it as a possible future `Layer` / `GraphStore`, not a design to reimplement (“dreaming” especially).

## Data model

Docs are explicit: **not** a hand-maintained entity–relation–entity triple store. “Living knowledge graph of **facts on top of other facts**.”

| Object | Role |
|--------|------|
| Document | Raw input (chat, PDF, URL, connectors) |
| Chunk | Grounding for RAG |
| Memory | Atomic fact about one topic |
| Profile | Always-on user/org summary |

Memory **relationship types** ([graph memory](https://supermemory.ai/docs/concepts/graph-memory)):

| Rel | Meaning |
|-----|---------|
| `updates` | New fact replaces old for search; history kept; `isLatest` (and related fields) steer retrieval |
| `extends` | Adds detail; both stay valid |
| `derives` | Inferred fact never stated in one place (entity-chain style) |

Memory **types:** facts (until updated), preferences (strengthen with repetition), episodes (decay unless significant).

**Automatic forgetting:** time-based expiry for temporary facts, contradiction (`updates` win), noise filtering of chatter. Search hides forgotten/expired unless `include.forgottenMemories`.

**Unknown:** node identity (are people first-class entities, or only fact nodes?), derive confidence, exact temporal schema beyond `isLatest`, internal DB.

## Retrieval + compress / summarize

Search API (`POST` search; docs show v3-style body): `q`, `limit`, `threshold` (default 0.6), `containerTag`, `rerank`, `rewriteQuery`, metadata `filters`, `include.{documents,summaries,relatedMemories,forgottenMemories}`.

Public description: **vector + full-text**, optional rerank, temporal validity, related-memory expansion. Hybrid-search blog (marketing, still public): hybrid retrieval feeds a memory graph + profile + temporal layer; they claim sub-300ms recall at large token volume. Treat performance numbers as vendor claims.

**Compress:**

- Chunking + memory extraction (write path).
- Profile as a compiled-ish always-on blob (close to Mastra working memory).
- `derives` / dynamic dreaming as extra LLM work after ingest.

No published “pack these memories to N tokens with an audit.” Caller (or their profile) does the windowing.

## How relevance is decided

- Similarity + `threshold` cut.
- Optional query rewrite and rerank.
- Prefer **latest** updated facts for “what is true now.”
- `relatedMemories` expands along graph edges (updates/extends/derives / entity chains).
- Container tag is a hard filter.

**Unknown:** fusion (RRF vs weighted sum), how derive edges are scored vs explicit facts.

## Failure modes / limits

- **Async graph.** Searching immediately after add with `dynamic` dreaming can miss memories (`done` ≠ graph ready). Instant mode costs extra and is worse for multi-doc coherence.
- **Derived facts can be wrong.** They document a review/forget path; a compiler that packs `derives` without labeling them will fail audits.
- **Forgetting / decay** can drop episodes we still need for “why was Project X delayed last quarter?”
- **Opaque engine.** Cannot reproduce dreaming offline; cannot unit-test their graph.
- **Hybrid blog mentions LangChain EnsembleRetriever** as industry context. We do not use that stack.
- LongMemEval numbers appear on **Mastra’s** comparison table (Supermemory 81.6–85.2% depending on model). Vendor-reported, chat-memory, not our bakeoff.

## Steal vs avoid (Context Compiler)

**Steal**

- **`updates` vs `extends` vs source text.** Compile should prefer current state for “what should the team do?” while still being able to pack the *superseded* decision when the question is historical — and **audit which**.
- **`include.relatedMemories`-style expansion** after hybrid seeds (same as GraphStore.Neighbors).
- **Keep chunks (or episodes) for grounding** plus facts for ranking. GraphRAG TextUnits / Graphiti episodes are the same idea.
- **Hard container** = our handle.
- **Profile as a memory-side artifact** the compiler may include as one candidate, not as the whole pack.

**Avoid**

- Rebuilding dreaming / proprietary temporal vector-graph.
- Silent `derives` in `compiled_context` without `reason=derived`.
- Blocking compile on ingest completion.
- Product clone (connectors, profiles, forget UX).
- Treating vendor latency/benchmark claims as Phase B acceptance tests.
