# Memory Suggest Semantic Quality Design

## Goal

Improve `memory.suggest` so it recommends updating an existing memory only when the match is semantically strong enough. The tool must remain local-first, non-mutating, project-scoped, and safe for agents to use before deciding whether to create or update memory.

## Problem

The current behavior is too aggressive: if FTS returns any candidate, the first candidate can become the recommended update target. That is useful for discovery, but weak for precision. A broad or generic text overlap can make the agent overwrite a valid memory that should have stayed separate.

## Design

Keep FTS as the retrieval step, then add a deterministic local evaluation step before shaping the `memory.suggest` response.

The evaluator classifies candidates as:

- `strong`: safe default for `update_existing`.
- `possible`: show as a related candidate, but do not recommend updating it.
- `weak`: show only when useful for duplicate awareness, never recommend it.

Only a `strong` first candidate can set `recommended_candidate_id` and mark `is_recommended=true`.

## Local Signals

Use simple deterministic signals already available from search results and enrichment metadata:

- Important-term overlap between input/context and candidate title.
- Important-term overlap with summary, keywords, tags, entities, aliases, and search phrases.
- Match mode: `precise` is stronger than `balanced`.
- Candidate separation: the first candidate should be clearly better than the next candidate.
- Generic-term penalty for matches based mostly on broad project vocabulary.
- Short input remains `ignore`.
- Multi-section input remains `split`.

No raw score, snippet, debug explanation, enrichment fields, aliases, entities, or keywords should be exposed in the normal `memory.suggest` response.

## Response Behavior

If the best candidate is `strong`:

- `suggested_action`: `update_existing`
- `recommended_candidate_id`: best candidate ID
- `confidence`: `high` for precise, `medium` for balanced

If candidates exist but none are `strong`:

- `suggested_action`: `create`
- `recommended_candidate_id`: empty
- `confidence`: `low`
- candidates are still returned, ranked, and marked `is_recommended=false`

If there are no candidates:

- `suggested_action`: `create`
- `recommended_candidate_id`: empty
- `confidence`: `none`

## Candidate Reasons

Replace generic reasons with deterministic, agent-readable reasons:

- `strong title and metadata overlap`
- `possible semantic overlap, review before updating`
- `matched only broad project terms`
- `input is too short to recommend an update`
- `input may contain multiple memories`

These are guidance strings, not hidden model reasoning.

## Non-Goals

- No internal LLM or external semantic provider in this version.
- No vector search in this version.
- No mutation from `memory.suggest`.
- No change to `memory.search` response behavior.
- No raw search score exposure in normal `memory.suggest`.

## Testing

Add focused tests for:

- Strong match recommends `update_existing`.
- Weak broad match returns candidates but recommends `create`.
- Ambiguous close candidates are not auto-recommended.
- Short input keeps `ignore`.
- Multi-section input keeps `split`.
- Candidate ranks remain ordered and `candidate_count` remains correct.

## Documentation

Update the MCP tool guide and skill documentation to explain:

- `memory.suggest` is conservative by design.
- Candidates are ranked by relevance, but only `is_recommended=true` is safe as the default update target.
- `importance` is global memory importance, not match quality.
- Weak candidates are duplicate-awareness hints, not update instructions.
