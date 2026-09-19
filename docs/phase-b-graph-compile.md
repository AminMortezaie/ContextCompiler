# Phase B — Graph-aware compile (heuristic)

`POST /v1/compile` now runs a **heuristic graph-aware compiler**: hybrid candidates → typed dependency expansion → one ranker → budget pack → deterministic sufficiency.

This is **not** a memory product. It is not Zep, Graphiti, Mem0, Hindsight, or GraphRAG. There is no `/v1/remember`, no LangChain, and no LLM on the compile hot path. We do not claim to beat anyone on LongMemEval or similar.

Phase A notes (`phase-a-memory-systems.md`) stay research. This file is what shipped.

## What changed

| Piece | Behavior |
|-------|----------|
| `memory.GraphStore` | Optional on `memory.Layer`: `Search` + hop-limited `Neighbors`. In-memory impl for tests/fixtures. Other backends can plug in later. |
| Day-0 / scale fixtures | Real typed edges (`blocks`, `owns`, `decided_by`, `assigned_to`, `implements`, `mentions`, …). Scale noise gets **disconnected** local clusters — not co-occurrence soup, not edges into Project X. |
| Candidate gather | Union of **lexical** ∪ **vector** (hash-bow-384 cosine, thresholded) ∪ **graph seeds**. |
| Expand | BFS over typed edges, default **1 hop**, cap 3. Ablation `NoRefExpansion` skips this. |
| Rank | One multi-signal ranker. Graph features use **relation weights + hop distance**. Hardcoded `proj-x` / `dec-001` / `tkt-042` hub boosts are gone. |
| Pack | Same token budget now includes **nodes + edge facts + optional episode snippets**. |
| Sufficiency | Deterministic contract-coverage (required focus kinds, keywords, project hint, causal edge). Misses are listed on `sufficiency.missing` and in the audit. No retrieve loop. |
| HTTP | URL unchanged: `POST /v1/compile`. Response **adds** `sufficiency` and `selected_edges`; audit rows may include `source` (`lexical` / `vector` / `graph` / `expand` / `permission`). Old clients keep working. Optional request field: `retrieve: { "topk": 32, "hops": 1 }`. |

Compress stays **off** the hot path. Compile is still local Go.

## How to try graph expansion

No API keys. No Docker. Go 1.24+.

```bash
go test ./...
make api          # :8080
```

Default (handle `day0`, 1-hop typed expand):

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
  }' | jq '{
    sufficient: .sufficiency.sufficient,
    missing: .sufficiency.missing,
    selected_ids,
    selected_edges: [.selected_edges[] | {rel, from: .from_id, to: .to_id}],
    sources: [.audit[] | select(.action=="include") | {id, source, score}]
  }'
```

Expect HTTP 200. `compiled_context` should contain `[edge:blocks tkt-042→proj-x]` (or similar typed facts) plus entity nodes. `sufficiency.sufficient` should be true for this Day-0 question.

Two-hop walk (reaches standup / freeze task from `proj-x`):

```bash
curl -sS http://localhost:8080/v1/compile \
  -H 'Content-Type: application/json' \
  -d '{
    "task_contract": { "question": "Why was Project X delayed?" },
    "state": { "handle": "day0" },
    "budget": { "token_budget": 2000 },
    "retrieve": { "topk": 16, "hops": 2 }
  }' | jq '{sufficient: .sufficiency.sufficient, selected_ids, hops_note: "hops=2"}'
```

Inline entities still work (compiler attaches `fixture.GraphSpecs` + leftover `RefIDs` as `related`).

Mock bench (unchanged entry point):

```bash
make bench-multitask
```

Arm C now compiles through the same graph path. Suite rollup prints `MEAN_PACKED_TOKENS` as a **Phase C hook** (task success vs packed-token cost). No external bakeoff in this phase.

## Honest limits

- Hash embeddings, synthetic fixtures, chars/4 budget — same caveats as the README.
- Sufficiency is a checklist, not an LLM judge.
- GraphStore is in-process memory. A Zep/Mem0 adapter is out of scope.
- Unknown relation types rank as weak `related`.
- We still exclude `meta.noise=true` entities before gather (scale-ladder behavior).

## Layout

```
internal/memory/graph.go          GraphStore interface
internal/memory/graph_index.go    hop-limited walk
internal/memory/graph_static.go   StaticGraph + DefaultGraph
internal/fixture/graph.go         Day-0 + scale typed edges
internal/compiler/gather.go       lexical ∪ vector ∪ graph
internal/compiler/expand.go       typed expand
internal/compiler/rank.go         single ranker
internal/compiler/pack.go         nodes + edges + snippets
internal/compiler/sufficiency.go  deterministic coverage
```
