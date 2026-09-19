# Mastra Observational Memory

Public sources: [Memory overview](https://mastra.ai/docs/memory/overview), [Observational Memory docs](https://mastra.ai/docs/memory/observational-memory), [OM reference](https://mastra.ai/reference/memory/observational-memory), [Observational Memory: 95% on LongMemEval](https://mastra.ai/research/observational-memory).

Naming: **Observational Memory (OM)** is the current documented name. Mastra still ships **message history**, **working memory**, and **semantic recall** as separate layers. OM is marked “Recommended” for long-running conversations.

## Architecture overview

Three agents, one context window:

| Role | Job |
|------|-----|
| **Actor** | The user-facing agent. Does not write memory APIs. |
| **Observer** | Background. When unobserved message history crosses a **token** threshold, compress those messages into dated, prioritized observations. Raw messages drop out of the prompt. |
| **Reflector** | Background. When the observation log crosses a second token threshold, rewrite it: merge related items, drop superseded notes, keep an append-mostly log. |

Default OM is **not retrieval**. The observation block is a stable system-message prefix; only not-yet-observed messages stay in the conversation. Mastra emphasizes prompt-cache friendliness: the prefix does not change every turn.

**Retrieval mode** (opt-in): each observation group stores a raw-message `range` (`startId:endId`). A `recall` tool can page that range, list threads, or (if `retrieval: { vector: true }`) semantically search. Default recall scope is `resource` (cross-thread); `scope: 'thread'` restricts to the current thread.

Other Mastra memory (not OM):

- **Working memory** — structured user profile injected as a system message (or state signal).
- **Semantic recall** — vector retrieve past messages; same-thread hits interleave by timestamp, other-thread hits become a system message.

## Data model

OM observations are **formatted text**, not a graph:

- Two-level bullets (event + details).
- Emoji priority (`🔴` / `🟡` / `🟢`) as Observer→Reflector hints.
- Dated / titled sections.
- Up to three dates per observation (research writeup): *observation date*, *referenced date* (mentioned in content), *relative date*.

Optional retrieval metadata: observation-group → message ID range.

No entities, edges, or episodes in the Graphiti sense. “Episode” here is just “a span of messages that got observed.”

**Unknown:** exact default token thresholds and Reflector drop policy in production configs (docs describe the mechanism, not one canonical number). Observer prompt internals live in the Mastra repo and can change.

## Retrieval + compress / summarize

**Compress (the product):**

- Trigger on **token counts**, not wall-clock or message count.
- Observer: ~3–6× compression on text (they report ~6× on LongMemEval); anecdotal 5–40× on tool-heavy traces.
- Reflector: restructure, not a one-shot “summarize the whole chat.” They contrast this with emergency compaction when a window is about to overflow.
- Observations replace the messages they cover. Continuity: a short reminder is placed at the start of remaining conversation messages.

**Retrieve (optional):**

- `recall` browse by range — no vector store required.
- `recall` `mode: "search"` — needs Memory’s embedder + vector store.
- Agent decides when to recall (tool call). Compile-time systems would instead always expand.

Mastra also lets you pass one-off `context` messages at call time (app state / your own RAG). Those are **not** persisted.

## How relevance is decided

Default OM: **relevance ≈ “the Observer thought it mattered, and it still fits after reflection.”** There is no per-turn query scoring. That is a deliberate bet: a stable log beats dynamic injection for cache and for “the model can see everything it was told.”

With retrieval mode: relevance is (a) whatever the Actor chooses to `recall`, plus (b) optional cosine search over observation groups.

Working memory / semantic recall use their own relevance (schema slots; vector similarity).

## Failure modes / limits

- **Lossy by design.** Exact wording and tool dumps disappear unless retrieval mode kept ranges. Causal org-state (“quote the decision comment”) needs the raw source.
- **Observer/Reflector LLM error.** Wrong priority, dropped constraint, merged two projects. Reflection can delete still-needed detail.
- **No graph expansion.** “Who owns the ticket that blocked Project X?” is not a walk; it is hope that the observation mentioned both.
- **Multi-session ceiling.** Their own writeup: multi-session is the hardest LongMemEval slice (~87.2% best, tied with Hindsight). Synthesis across scattered conversations is not solved.
- **Preference category is noisy** (30 questions; one flip = 3.3 points). Treat published OM leaderboard rows as **their** chat-memory result, not a compiler KPI.
- **Different problem than ours.** OM optimizes long *dialogues*. We compile a large *org-state corpus* into a 2k pack for a single task. A 30k stable observation window is not our budget model.
- **Framework coupling.** OM is a TypeScript agent feature (`@mastra/memory`). We stay Go, no LangChain, no Mastra runtime.

## Steal vs avoid (Context Compiler)

**Steal**

- **Background compression** of bulky traces (conversations, ticket comment storms) into a handle the compiler can `Load`. Do this **off** `POST /v1/compile`.
- **Keep a pointer to the raw span** (Mastra `range`). Our audit should be able to say “packed summary S; source messages M1–M4.”
- **Token-threshold triggers**, not “every N turns.”
- **Stable prefix** as an idea for *compiled* system instructions / working-memory blobs — not for the per-task org-state pack (that pack *must* change with the question).
- Honesty that **compaction ≠ retrieval**. We still need hybrid retrieve for 5M-token corpora.

**Avoid**

- Making OM (or any observation log) the compiler.
- Per-request Observer/Reflector LLM calls.
- Assuming “put the whole memory in context, compressed” scales to the 100k–5M **candidate** ladder. Our constraint is the **pack budget**, not “fit the chat.”
- Emoji-priority text as an interchange format. If we compress, emit structured `state.Entity` (or edges) the ranker already understands.
- Copying LongMemEval leaderboard claims into product copy.
