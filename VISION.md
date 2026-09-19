# Context Compiler — vision / experiment brief

## Core question
Does task-aware context compilation measurably outperform naïve full-context and standard retrieval approaches under a fixed context/token budget?

This is a systems + evaluation problem, not an ML research project.

## Stack (prototype)
Go + PostgreSQL + pgvector + one LLM API. No LangChain/LangGraph/agent frameworks.

## Three systems under test
1. Full Context — dump as much organizational state as fits / allowed into the prompt
2. RAG — embed query → vector search → top-k → prompt → LLM
3. Context Compiler — task analysis → state selection → rank → fit token budget → assemble context → LLM

## Compiler pipeline (v0, deliberately simple)
Task → Identify required entities → Retrieve candidate state → Rank state → Fit into token budget → Construct context → LLM → Result

## Org state to generate (synthetic)
Company → Users, Projects, Teams, Documents, Conversations, Tickets, Decisions, Tasks, Events

Scale ladder (token budgets of state): 100K, 500K, 1M, 5M, 10M, 50M

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

- Arm A (full-dump): implemented
- Arm B (RAG): hash-bow-384 embeddings + pgvector (memory fallback)
- Arm C (compiler): task-contract + multi-signal rank + density budget-fit + include/exclude audit
- Fixture: deterministic Scale100K (~100K tokens, chars/4)
- Eval harness: task success, tokens, est. cost, latency, relevant-state recall, irrelevant-state ratio
- LLM: mock default; OpenAI-compatible via env when key present

Run: `go test ./...`, `make db-up`, `make bench`.
