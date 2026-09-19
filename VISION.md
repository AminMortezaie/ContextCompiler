# Context Compiler — vision / experiment brief

## Core question
Does task-aware context compilation measurably outperform naïve insertion-order packing from the full candidate org-state (Arm A) and standard retrieval (Arm B) under the same fixed LLM packing budget?

This is a systems + evaluation problem, not an ML research project.

## Stack (prototype)
Go + PostgreSQL + pgvector + one LLM API. No LangChain/LangGraph/agent frameworks.

## Three systems under test

All three use the **same fixed LLM context packing budget** (bench default 2000 tokens). The scale ladder (100K–5M) grows the **candidate org-state corpus**, not the prompt size.

1. **Full-state baseline (Arm A)** — treat the entire in-memory org-state as candidates; pack entities in insertion order until the shared budget (not “send the whole 1M/5M corpus to the model”).
2. **RAG (Arm B)** — embed query → vector search → top-k → pack to the same budget → LLM
3. **Context Compiler (Arm C)** — task analysis → state selection → rank → fit the same token budget → assemble context → LLM

## Compiler pipeline (v0, deliberately simple)
Task → Identify required entities → Retrieve candidate state → Rank state → Fit into token budget → Construct context → LLM → Result

## Org state to generate (synthetic)
Company → Users, Projects, Teams, Documents, Conversations, Tickets, Decisions, Tasks, Events

Scale ladder (synthetic **corpus** size, chars/4): 100K, 500K, 1M, 5M, … — independent of the **packing budget** sent to the LLM (~2K in bench).

## Example task
"Why was Project X delayed, who made the relevant decision, and what action should the backend team take?"

## Metrics (must measure every run)
- Task success
- Context / input tokens
- Total tokens
- Latency
- Cost
- Relevant-state recall
- Irrelevant-state ratio
- Context precision where applicable

## AI knowledge needed (prototype)
LLM APIs & prompting, embeddings + vector search, RAG, LLM evaluation — medium depth.
NOT needed initially: transformers internals, training, PyTorch, DL math, RL/RLHF, building an LLM.

## Risk to avoid
Building a sophisticated compiler before establishing a clean experimental question and baselines.

---

## Day-1 status

Local tree at `/workspace/context-compiler`.

- Arm A (full-dump): full corpus candidates, insertion-order pack to shared budget — implemented
- Arm B (RAG): hash-bow-384 embeddings + pgvector (memory fallback)
- Arm C (compiler): task-contract + multi-signal rank + density budget-fit + include/exclude audit
- Fixture: deterministic Scale(TargetTokens100K, seed) (~100K tokens, chars/4)
- Eval harness: task success, tokens, est. cost, latency, relevant-state recall, irrelevant-state ratio
- LLM: mock default; OpenAI-compatible via env when key present

Run: `go test ./...`, `make db-up`, `make bench`.
