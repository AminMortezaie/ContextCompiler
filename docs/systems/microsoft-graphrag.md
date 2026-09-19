# Microsoft GraphRAG

Public sources: [Welcome](https://microsoft.github.io/graphrag/), [Indexing overview](https://microsoft.github.io/graphrag/index/overview/), [Local search](https://microsoft.github.io/graphrag/query/local_search/), [Global search](https://github.com/microsoft/graphrag/blob/main/docs/query/global_search.md), [DRIFT / query overview](https://microsoft.github.io/graphrag/query/overview/), [From Local to Global (paper)](https://arxiv.org/abs/2404.16130), [indexing architecture](https://github.com/microsoft/graphrag/blob/main/docs/index/architecture.md). Cost context: [Azure AI Foundry — GraphRAG costs](https://techcommunity.microsoft.com/blog/azure-ai-foundry-blog/graphrag-costs-explained-what-you-need-to-know/4207978).

## Architecture overview

GraphRAG is a **batch index + query engine over a static(ish) corpus**, not an agent memory. Pipeline (default configuration):

1. Load documents → chunk into **TextUnits** (citation grain).
2. LLM-extract **entities**, **relationships**, optional **claims/covariates** per TextUnit; embed chunks.
3. Merge duplicate titles/types (concatenate descriptions).
4. **Hierarchical Leiden** community detection.
5. Bottom-up **community reports** (LLM summaries); embed entities + reports.

Query modes ([query overview](https://microsoft.github.io/graphrag/query/overview/)):

| Mode | When |
|------|------|
| **Local search** | Entity-centric questions; fan-out from seed entities |
| **Global search** | Corpus-level / theme questions; map-reduce over community reports |
| **DRIFT** | Local fan-out **plus** community context; follow-up questions |
| **Basic search** | Plain top-k TextUnits (vector RAG baseline) |

Graphiti’s own overview table is the right contrast: GraphRAG = static documents, batch, sequential LLM summarization at query time for global; Graphiti = incremental episodes, hybrid sub-200ms search, bi-temporal invalidation.

## Data model

| Object | Meaning |
|--------|---------|
| TextUnit | Chunk of source; fine-grained reference |
| Entity | Title, type, description (merged array of descriptions) |
| Relationship | Source, target, description (merged) |
| Claim / covariate | Optional extracted assertions attached to entities |
| Community | Leiden cluster at a hierarchy level |
| Community report | LLM summary of a community (and, at higher levels, of child reports) |

No first-class **episode** stream. Time, if present, is whatever the extractor wrote into descriptions — not Graphiti’s bi-temporal edge lifecycle.

Outputs are typically Parquet tables + a vector store. The “GraphRAG Knowledge Model” is an abstraction over storage ([architecture.md](https://github.com/microsoft/graphrag/blob/main/docs/index/architecture.md)).

## Retrieval + compress / summarize

**Index-time compress:** community reports and merged entity/relationship descriptions. This is the expensive part (LLM per chunk + per community). Microsoft’s cost note: most spend is extraction, not embeddings.

**Local search** ([docs](https://microsoft.github.io/graphrag/query/local_search/)):

1. Embed query (+ optional conversation history) → similar **entity descriptions**.
2. Those entities are **access points**. Collect: neighbor entities, relationships, covariates, mapped TextUnits, mapped community reports.
3. **Rank + filter** each list to fit **one pre-defined context window**.
4. One LLM answer over that pack.

This is the closest published cousin of our compiler: *heterogeneous candidates → prioritize → budget → generate*.

**Global search:**

1. Take community reports at a chosen hierarchy level.
2. **Map:** chunk reports, each chunk produces intermediate points with a **numeric importance rating**.
3. **Reduce:** keep the highest-value points, write the final answer.

Lower community levels = more thorough, more LLM time. Global search is documented as resource-intensive; GitHub issues report **minutes** on large indexes.

**DRIFT:** start from community-informed reformulation, then iterative local searches. More facts, more calls.

**Basic:** top-k TextUnits — our Arm B.

## How relevance is decided

- Local: embedding similarity to **entity descriptions**, then graph membership (neighbors, incident edges, attached chunks/reports). Final cut is **window fit**, not a published score formula.
- Global: map-step **importance ratings** (LLM-judged), then reduce. Communities that never enter the map never matter — and default map can still spend tokens on weakly related reports (operator reports of this are common).
- DRIFT: community-conditioned query refinement + local relevance.

There is no BM25 hybrid in the headline design (Basic/Local are embedding-first). **Unknown:** exact local-search ranking functions in the current `context_builder` (parameters are pluggable).

## Failure modes / limits

- **Index cost and staleness.** New tickets/decisions need re-extract and often community refresh. Bad fit for our continuously scaled fixture *as a live store*. Fine as a **one-shot analysis** of a document dump.
- **Global latency/cost.** Map-reduce over many reports; seconds to tens of seconds (Graphiti’s comparison table; user issues up to minutes).
- **Extraction quality.** Graph is only as good as chunk size, prompt, model, and merge-by-title. Name collisions merge wrongly; sparse relationships starve local search.
- **Leiden granularity.** Wrong resolution → useless or huge communities. Reports can hallucinate “themes.”
- **Wrong mode.** Local search on a “top themes” question ≈ Arm B. Global search on “who approved the freeze?” wastes reports.
- **Not incremental-first.** Incremental community research exists (Graphiti cites this gap); GraphRAG docs assume index jobs.

## Steal vs avoid (Context Compiler)

**Steal**

- **Local-search shape:** seed entities → fan-out to neighbors / relations / source chunks → **rank/filter to a budget**. That *is* Phase B expand + pack, with TextUnits ≈ our `Entity.Text` / future episodes.
- **Community reports as an optional candidate kind** if we ever add a “themes across the org” task. Not needed for Day-0 causal/ownership/action.
- **Explicit query modes.** We should not run global map-reduce for every `/v1/compile`. Contract → mode (local vs theme) is a compiler decision.
- **TextUnit-style citations** in the audit (`episode_id` / source span).
- **Basic search as a first-class baseline** — we already have Arm B.

**Avoid**

- Making Leiden + community reports the org-state database.
- Query-time map-reduce on the compile beachhead (kills `compile_ms` and needs an LLM).
- Batch-only thinking: our state is live tickets/decisions.
- Assuming GraphRAG “beats RAG” generally — Microsoft’s claim is about **global sensemaking** vs naive chunk RAG, not about our 2k pack bakeoff.
- Pulling the Python GraphRAG runtime into the Go service.
