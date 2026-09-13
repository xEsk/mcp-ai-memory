# Memory Search Relevance Evaluation Design

Status: APPROVED

Level: BOUNDED

## Outcome

Make `memory.search` relevance changes evidence-driven on a small, reproducible corpus that covers exact technical lookup, conceptual questions, noisy multi-intent queries, shared project vocabulary, and queries for which the project has no relevant memory.

The implementation cycle must establish the current result order first, evaluate only bounded local variants, and retain only the smallest variant that improves the new cases without regressing the accepted retrieval contracts. If no candidate satisfies every hard gate, the corpus and baseline remain as the useful change and production ranking stays unchanged.

## User And Present Problem

The caller is a coding agent searching one Project Memory project for relevant technical context before acting. Today:

- `precise` removes stopwords and requires every remaining normalized term with `AND`;
- `balanced` removes stopwords and matches remaining terms with `OR`;
- `broad` retains stopwords and matches terms with `OR`;
- FTS5 BM25 ranks matches across original and enriched fields, followed by importance and update time.

This works for focused identifiers but has two observed failure shapes:

1. Long queries can return no `precise` result because one absent term invalidates the entire conjunction.
2. `balanced` and `broad` can rank peripheral memories highly when a long query contains common project vocabulary and none of its distinguishing terms exist in memory.

The desired observable outcome is not that every query returns something. It is that focused exact queries retrieve their target, conceptual queries retrieve the best related memory in the appropriate recall mode, and unsupported concepts can return no result instead of plausible-looking noise.

## Current Owners And Reuse

Keep the existing architecture and owners:

- `internal/search/query.go` owns query token policy and mode compilation.
- `internal/search/fts.go` owns FTS filtering, BM25 ranking, result order, highlight evidence, and access recording.
- `internal/retrievaltext` owns Unicode-aware technical token boundaries and identities.
- `internal/enrich` remains the prospective metadata producer; historical enrichment is not rewritten.
- `internal/search/retrieval_quality_test.go` and its embedded JSON corpus own reproducible retrieval judgments and metrics.
- `memory.search.debug` remains the diagnostic interface. Its score is a ranking value, never a probability or correctness confidence.

The implementation must preserve the two leading neutral BM25 placeholders for the unindexed FTS columns, exact one/two-rune technical terms, longer prefix matching, project isolation, access accounting, and public response shapes.

## Scope

### In scope

- Replace the current v2 synthetic corpus with one v3 corpus containing the existing accepted cases plus 8–10 new logical queries and the minimum memories needed to create realistic distractors.
- Run every new logical query through `precise`, `balanced`, and `broad`, reporting metrics by mode and query slice.
- Freeze the pre-change baseline before modifying production query or ranking behavior.
- Evaluate the existing weights and a small finite set of title-weight variants.
- If weights alone are insufficient, evaluate one deterministic local query-quality/reranking variant based on normalized query-term evidence, including a distinction between discriminating terms and vocabulary common across the scoped project corpus.
- Apply at most one production ranking/query-quality adjustment: the smallest candidate satisfying all acceptance gates.
- Update the active search documentation if runtime ranking or mode behavior changes.

### Out of scope

- Embeddings, vector search, LLM reranking, external providers, or new dependencies.
- FTS schema or tokenizer changes, migrations, FTS rebuilds, or historical re-enrichment.
- Changes to `memory.suggest`, public result DTOs, retrieval/access semantics, filters, project scoping, or memory persistence.
- Automatic query rewriting, hidden `precise`-to-`balanced` fallback, stemming, or synonym generation.
- Hardcoded AICO-specific generic-word lists in production.
- Treating the real Project Memory database as an automated fixture or copying it into tests.

## Reproducible Corpus V3

The corpus remains synthetic and checked in. It may preserve the exact non-sensitive query strings supplied for this investigation, but its memories must be manually authored fixtures rather than exports of the real database.

The new logical cases are:

| Slice | Query shape | Expected judgment |
| --- | --- | --- |
| exact identifier | `POST /mcp/skills /mcp/case-memory` | Public namespaced MCP routes is grade 3. |
| exact identifier | `@aico workspace package scope` | Workspace package scope is grade 3. |
| conceptual | `How are Skills MCP and Case-memory exposed through the public edge?` | Public namespaced MCP routes is grade 3; private Case-memory boundary may be grade 2. |
| conceptual | `Who decides coverage and treatment after the claimant investigation?` | Claim investigation/handler authority boundary is grade 3. |
| conceptual | `How is prior claimant conversation retrieved when recent context is incomplete?` | Private Case-memory retrieval boundary is grade 3. |
| noisy multi-intent | `aico_skills MCP 424 Failed Dependency DEV policy not found regression runtime deployment image` | Public namespaced MCP routes is grade 3 and must be recalled; Docker dependency image may be grade 2. The absent `424` incident itself is not considered documented. |
| shared vocabulary/no answer | `voice audio realtime channel conversation telephony claimant intake architecture authority` | No match: no fixture documents a voice or telephony architecture. |
| focused no answer | `voice telephony audio realtime SIP media streaming` | No match. |
| no answer | `424 Failed Dependency exact incident root cause` | No match unless a fixture explicitly documents that incident. |
| no answer | `SAML certificate rotation identity provider metadata` | No match. |

The distractor set must be small but include the observed nearby topics: bounded claimant replies, conversation recovery, Web Chat invitation/completion communication, Docker dependency builds, policy `NOT_FOUND`, runtime authority, and public MCP routing. Each memory stays focused enough that judgments can explain why it is direct, supporting, peripheral, or irrelevant.

The corpus validator must continue rejecting missing memories, invalid grades, cross-project judgments, unknown slices, and contradictory no-match judgments. The v2 cases remain present in v3 so the existing identifier, Unicode, short-token, enrichment, isolation, and hard-negative contracts are not weakened.

## Minimal Evaluation Flow

1. Load and validate corpus v3 into a fresh `t.TempDir()` SQLite database through the real project, memory, enrichment, and search services.
2. Run every logical query in all three modes with the normal result limit of 10.
3. Record ordered IDs and calculate Hit@1, Recall@5, MRR@10, nDCG@5, no-match false positives, leakage, and critical grade-3 misses by mode and slice.
4. Freeze these values as the pre-change baseline before production behavior changes.
5. Evaluate candidates in this order:
   - current behavior;
   - title weights `1.5`, `2.0`, and `3.0`, leaving every other BM25 weight unchanged;
   - only if no title variant passes, one deterministic local quality/reranking candidate using existing normalized terms and scoped corpus frequency/evidence.
6. Discard any candidate failing a hard gate. Among passing candidates choose, in order: fewer no-match false positives, higher balanced nDCG@5, smaller production change. Do not select from raw score magnitude.
7. Remove experimental variants that were not selected. The final production path must contain one ranking policy, not a runtime tuning framework.

Project-generic evidence must be derived from the currently scoped project corpus or FTS evidence, not from domain-specific constants. A quality candidate may demote or suppress weak OR matches, but must not remove a direct identifier/title match merely because the query is long.

## Ordered Behavior Slices

### Slice 1: Corpus and baseline

Replace the embedded v2 artifact with v3, add the new logical queries and distractors, execute the three-mode matrix, and capture the failing pre-change evidence. This slice changes tests and fixtures only.

Dependency: judgments must be settled before ranking experiments; otherwise tuning has no independent authority.

### Slice 2: Bounded ranking experiment

Evaluate the finite title-weight matrix. If none meets acceptance, evaluate the single local query-quality candidate. Keep experiment plumbing test-local or remove it after selecting the winner.

Dependency: Slice 1 baseline and hard gates.

### Slice 3: Smallest justified production adjustment

Apply only the winning query-time adjustment. If there is no passing candidate, make no production search change. Update relative-field-order tests if and only if an accepted title weight changes that order; continue asserting the two unindexed placeholders and project filters.

Dependency: a candidate with recorded before/after evidence satisfying all gates.

### Slice 4: Documentation and closure

Update `docs/ai/storage-search-guide.md` and `docs/ai/memory-lifecycle-internals.md` only if runtime behavior changed. Record the final before/after metrics and explicitly classify the two original failures as ranking, formulation, or absent knowledge. Do not describe relevance scores as probabilities.

## Acceptance Checks

Hard gates:

- All existing v2 evaluable queries retain Hit@1 and Recall@5 of `1.00` and all existing no-match cases remain empty.
- Project leakage and critical grade-3 misses remain zero.
- Exact identifier queries have Hit@1 `1.00` in `precise`.
- The three conceptual targets have Hit@1 `1.00` in `balanced`.
- The public-routes grade-3 target from the noisy incident query appears within balanced top 5. The Docker memory may also appear; success does not require pretending the query has only one intent.
- The focused voice/telephony, long voice, and exact `424` incident no-answer queries return no results in `precise` and `balanced`. If suppressing their peripheral OR matches would fail another hard gate, no production quality filter is accepted.
- `broad` results and false positives are reported separately. Broad may remain noisier, but it must not regress exact-target Hit@1 from the frozen baseline.
- Search debug explanations continue to reflect actual highlighted fields, and scores remain documented as ranking values only.
- No migration, reindex, re-enrichment, provider call, new dependency, or real database access occurs.

The before/after report must show per-mode metrics and the rank of each direct target. An aggregate improvement cannot hide a regression in exact identifiers, no-answer behavior, or project isolation.

## Verification

Implementation proof uses the real search boundary and temporary SQLite stores:

1. Focused RED: run the v3 retrieval-quality test before any production adjustment and retain its per-query ordered IDs and metrics.
2. Focused GREEN: rerun the same test after the selected candidate.
3. Affected search suite: `go test ./internal/search -count=1`.
4. Repository acceptance: `make verify`, which guards the real database, runs the race suite, builds commands, checks formatting, and verifies test database paths.
5. Final diff self-review for public contract changes, accidental experiment scaffolding, hardcoded project vocabulary, and documentation accuracy.

This is BOUNDED rather than CRITICAL: the accepted change is query-time retrieval behavior and synthetic evaluation. It does not change authority, durable state, schema, recovery, privacy, or providers. Independent review is optional unless implementation expands beyond this boundary.

## Stop Conditions

Stop without a production ranking change if:

- no candidate satisfies every hard gate;
- improvement depends on AICO-specific hardcoded vocabulary;
- a title-weight change only improves the noisy incident query while degrading another slice;
- the quality candidate requires schema/tokenizer changes, historical enrichment provenance, embeddings, or a provider;
- automated proof cannot remain entirely under `t.TempDir()`; or
- the proposed behavior makes `precise`, `balanced`, and `broad` indistinguishable without an explicit product decision.

Escalate to a separate design only if evidence shows that a schema/tokenizer migration, historical re-enrichment, or semantic provider is necessary.

## Documentation Impact

- This specification is the approval baseline for the implementation task.
- Update `docs/ai/storage-search-guide.md` and `docs/ai/memory-lifecycle-internals.md` only for accepted runtime changes.
- Update the existing Project Memory fact about BM25 weights if a new weight is accepted; do not create a competing fact.
- Do not add an ADR: this remains within the existing FTS5 search architecture and public MCP contract.

## Implementation Result

The v3 RED baseline reproduced three balanced false positives across four new no-answer queries. Title weights `1.5`, `2.0`, and `3.0` produced the same result order and metrics as the existing `1.0`, so the title weight remains unchanged.

The accepted query-time adjustment applies only to balanced queries with at least four normalized terms. When at least half of those terms do not exist in any active memory in the requested project, search and count return zero before the OR query. The check uses the compiled exact/prefix term expressions and both project filters. It adds no schema, dependency, provider, reindex, or re-enrichment requirement.

After the adjustment, balanced no-answer false positives fell from `3` to `0`; balanced Hit@1, Recall@5, and MRR@10 remained `1.00`, balanced nDCG@5 remained `0.96`, and the public-routes target for the noisy incident query remained at rank 3. Precise retains strict AND behavior, while broad retains the three exploratory false positives and reports them separately.
