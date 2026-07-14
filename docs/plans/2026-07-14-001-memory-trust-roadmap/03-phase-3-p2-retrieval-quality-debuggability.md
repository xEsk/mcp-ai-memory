# Phase 3 — P2 Retrieval Quality and Debuggability

## 1. Objective and outcome

Phase 3 improves lexical retrieval where current behavior is brittle, but only through changes that pass the accepted Phase 2 corpus and quality gates.

After this phase:

- short technical terms such as `C`, `R`, `Go`, `DB`, and `io` are not silently discarded;
- search, suggestion quality, debug evidence, and local enrichment share one Unicode-aware technical-token contract;
- `precise` and `balanced` compile the same searchable terms and differ by `AND` versus `OR` combination;
- empty normalized queries return an explicit typed error rather than `MATCH ""`;
- local heuristic enrichment is deterministic, UTF-8 safe, and uses exact token/phrase rules instead of arbitrary substrings;
- `memory.suggest` broadens to balanced when precise does not produce a safe recommendation, deduplicates candidates, and records each exposed retrieval once;
- `memory.search.debug` reports highlighted evidence from the fields that actually matched;
- existing stored enrichment is not rewritten without a separately approved provenance and migration design.

## 2. Prerequisites, baseline, and Required Reading

### Entry gates

- Phase 1 is accepted, including the corrected positional BM25 mapping.
- Phase 2 is accepted, including the synthetic corpus, reviewed judgments, frozen baseline, migration safety primitives, and honest retrieval metrics.
- There is no unresolved project-isolation, database-continuity, or access-counting defect.

### Execution baseline

- Repository: `mcp-ai-memory`
- Design baseline commit: `f56c19430c986cd9c758a3050954772d1eecaf83`
- Real database: `data/memory.db` is prohibited for implementation tests, corpus construction, re-enrichment, reindexing, or smoke checks.
- Every integration fixture uses an explicit path under `t.TempDir()`.

### Required Reading

- [Roadmap entry point](README.md)
- [Phase 1 specification](01-phase-1-p0-critical-correctness-privacy.md)
- [Phase 2 specification](02-phase-2-p1-continuity-observability-quality.md)
- [Critical rules](../../ai/critical-rules.md)
- [Storage and search guide](../../ai/storage-search-guide.md)
- [Memory lifecycle internals](../../ai/memory-lifecycle-internals.md)
- [Enrichment guide](../../ai/enrichment-guide.md)
- [MCP tools guide](../../ai/mcp-tools-guide.md)
- `internal/search/fts.go`
- `internal/search/hybrid.go`
- `internal/enrich/heuristic.go`
- `internal/mcpserver/tools.go`
- `internal/mcpserver/suggest_quality.go`
- `internal/mcpserver/responses.go`
- the accepted Phase 2 corpus, judgments, baseline report, and metric tests

### Proven P2 weaknesses

1. `shouldSkipTerm` applies byte length to whitespace fragments before SQLite `unicode61` tokenization. `Go UI` produces no precise result while broad finds the target; one-character terms fail in every mode.
2. Every retained fragment becomes a prefix query. Short broad terms can expand to unrelated tokens, while quoted input does not preserve phrase intent.
3. The heuristic ASCII regex truncates Unicode words (`München` → `nchen`) and drops many short technical identifiers.
4. Tag and alias inference uses substring matching: `build` implies `frontend`, and `capital` implies API aliases.
5. Heuristic tag order depends on map iteration, summaries can split UTF-8 by byte, and generated phrases are noisy and inconsistently capped.
6. Suggestion fallback runs balanced only when precise returns zero rows, not when precise returns only weak/ambiguous candidates.
7. Suggestion evidence uses a tokenizer different from search, omits content, and can double-count retrievals if a second search is added naively.
8. Debug snippets always use `content`; explanations inspect only four fields with substring checks. A content match for `api` can be mislabeled as title because `rapid` contains that substring.

## 3. Scope, non-goals, and invariants

### In scope

- A small dependency-free internal technical-text package shared by search, local enrichment, suggestion evaluation, and debug evidence.
- Unicode rune-aware normalization aligned and integration-tested against SQLite `unicode61`.
- A query compiler that returns the escaped FTS expression, normalized terms, exact/prefix decisions, and effective mode.
- Exact matching for one- and two-rune technical tokens; the existing prefix behavior remains for searchable terms of three or more runes.
- Typed invalid-query behavior when no searchable term remains.
- Prospective deterministic local-enrichment corrections.
- Quality-aware precise-to-balanced suggestion fallback, candidate deduplication, and final-only retrieval recording.
- Structured debug fields: `matched_fields`, `snippet_field`, `snippet`, and a compatible explanation derived from real highlights.
- Phase 2 corpus extensions and metric gates for the affected slices.

### Non-goals

- Changing FTS schema, tokenizer, token characters, or BM25 weights.
- Distinguishing `C#` from `C++`; current `unicode61` indexing reduces both to the same core token.
- Giving user quotes new phrase-search semantics. Quotes remain safely escaped input, and this limitation is documented.
- Adding vectors, stemming, fuzzy search, language detection, or external thesauri.
- Tuning suggestion weights or thresholds merely to make new fixtures pass.
- Automatically re-enriching existing rows or rebuilding FTS from newly generated metadata.
- Claiming that local heuristic aliases are semantic understanding.

### Invariants

- All FTS input remains escaped data; users cannot inject raw FTS operators.
- Project and soft-delete filters remain identical.
- Search and count compile the same query contract.
- Normal public search responses remain slim and do not expose enrichment or content unless already requested.
- Suggest remains conservative and non-mutating with respect to memory content.
- Debug evidence may expose diagnostic indexed text only through the existing explicit debug tool.
- Source content, timestamps, stored enrichment, and FTS rows are unchanged by query-time corrections.

## 4. Acceptance contract and user journeys

### Search journey

1. The client submits a project-scoped query.
2. The shared normalizer emits Unicode-aware logical terms and technical-token metadata.
3. The compiler removes only applicable natural-language stopwords, uses exact matching for one/two-rune terms, uses escaped prefixes for terms of three or more runes, and combines the same term set with `AND` or `OR` for precise/balanced.
4. If no term remains, search and count return the same typed invalid-query result.
5. SQLite performs matching and the accepted BM25 ranking remains unchanged.

### Suggest journey

1. Suggest runs a no-recording precise candidate search.
2. It evaluates candidates with the same normalized terms as retrieval.
3. If precise yields one unambiguous safe recommendation, it does not broaden.
4. Otherwise it runs no-recording balanced search, deduplicates by memory ID, and reevaluates the final set without loosening thresholds.
5. Only candidates returned to the caller are recorded, once per memory ID.
6. Original content remains available to internal evidence evaluation but is omitted from public candidate DTOs.

### Debug journey

1. Search captures real highlight evidence for all indexed FTS fields.
2. `matched_fields` lists fields containing actual highlights.
3. A deterministic field priority based on the accepted field-weight order selects `snippet_field` and a highlighted snippet.
4. `match_explanation` is derived from that evidence and never says a field “matched” based only on substring inference.

### Explicit failure conditions

Phase 3 fails if:

- any accepted Phase 2 global or slice quality gate regresses beyond its threshold;
- short exact tokens expand as prefixes (`io` must not match only `iostream`);
- precise and balanced normalize different terms;
- invalid/empty input reaches SQLite as `MATCH ""`;
- tag/alias hard negatives still fire on substrings;
- equal heuristic input produces different ordered output;
- suggest lowers the existing safe-update precision, recommends an ambiguous candidate, or records one public candidate twice;
- debug attributes a field without an actual highlight;
- normal responses expose internal content or enrichment accidentally;
- implementation changes schema, re-enriches stored rows, reindexes the real store, or touches `data/memory.db`.

## 5. Authority boundaries and end-to-end flow

### Shared normalization

Raw text → `internal/retrievaltext` rune scanner → normalized token records → consumer-specific policy.

The shared package owns token boundaries, Unicode case normalization, technical separators, rune counts, case-insensitive identity, and deterministic deduplication. Search owns stopword, exact/prefix, mode, and FTS escaping policy. Enrichment owns corpus-backed tag/alias rules. Suggest and debug consume search-normalized terms rather than reconstructing them.

### Search evidence

Compiled query → FTS5 match → per-field highlight evidence → ranking → public/debug projection → optional retrieval recording.

Matching/ranking and retrieval recording must be separable so internal candidate searches do not mutate metrics before final projection.

### Enrichment lifecycle

New or explicitly updated memory → local heuristic → corrected derived metadata → transactional source/enrichment/FTS upsert.

Existing rows keep their current metadata. A plain FTS rebuild would only copy that old metadata and is not historical remediation.

## 6. Walking skeleton

The first observable slice is an integration fixture containing `Go`, `DB`, `io.Reader`, `iostream`, and a cross-project duplicate:

1. prove current precise search misses `Go` and `DB`;
2. compile short technical terms as exact escaped terms;
3. prove precise finds the intended same-project rows;
4. prove `io` does not match only `iostream`;
5. prove project leakage remains zero and count/search share behavior.

This crosses the shared normalizer, compiler, real temporary SQLite schema, service, and project filter before changing enrichment or suggestion logic.

## 7. Vertical slices and checkpoints

### Slice 1 — Shared technical normalization and query compiler

- **Likely boundaries:** new focused `internal/retrievaltext/`, `internal/search/fts.go`, search model/errors, unit and integration tests.
- **RED:** `C`, `R`, `Go`, `DB`, and `io` disappear; all retained terms are prefixes; stopword-only input becomes `MATCH ""`.
- **GREEN:** exact short tokens, long prefixes, typed empty-query error, common term set for precise/balanced, unchanged isolation and BM25 ordering.
- **Safety:** test raw `AND`, `OR`, `NOT`, `*`, quotes, parentheses, punctuation, malformed UTF-8, paths, and compound identifiers; none may become FTS syntax.
- **Compatibility:** `ErrNot` continues to find `ErrNotFound`; API/MCP/SQL remain searchable; quoted input remains documented as non-phrase input.

### Slice 2 — Deterministic prospective local enrichment

- **Likely boundaries:** `internal/enrich/heuristic.go`, shared normalizer, exact-output tests, Phase 2 corpus slices.
- **Behavior:** rune-safe summary truncation; Unicode keywords; preserved reviewed short technical tokens; ordered tag rules; whole-token/phrase tag inference; exact alias groups; case-insensitive stable deduplication; hard phrase cap.
- **Hard negatives:** `build ≠ ui`, `contest ≠ test`, `capitalization ≠ api`.
- **Positive slices:** exact identifiers/paths, acronyms, Catalan/Spanish/English technical words, and explicitly reviewed alias/tag vocabulary.
- **Limit:** only new or explicitly upserted memories receive the new metadata.
- **Dependency:** Slice 1 shared normalization.

### Slice 3 — Quality-aware suggestion fallback and one-time retrieval accounting

- **Likely boundaries:** `internal/search` candidate/no-recording API, `internal/mcpserver/tools.go`, `suggest_quality.go`, response tests.
- **Behavior:** evaluate precise first; broaden when no safe recommendation exists; deduplicate; preserve conservative thresholds and ambiguity abstention; record only final public IDs once.
- **Internal content:** retain original content on internal `search.Result`; public response builders remain the only content projection boundary and must prove sentinel content is absent.
- **RED:** one weak precise result blocks a strong balanced duplicate.
- **GREEN:** the balanced strong candidate is considered without creating a false update, ambiguous cases abstain, and counters increment once.
- **Dependency:** Slice 1 and accepted suggestion corpus judgments.

### Slice 4 — Truthful structured debug evidence

- **Likely boundaries:** FTS select/evidence scanning, search result model, `internal/mcpserver/responses.go`, debug tool tests.
- **Behavior:** determine matched fields from actual SQLite highlights across `type`, `title`, `summary`, `short_summary`, `keywords`, `tags`, `entities`, `aliases`, `search_phrases`, and `content`.
- **Projection:** expose ordered `matched_fields`, selected `snippet_field`, highlighted `snippet`, and compatible explanation derived from the same evidence.
- **Selection:** use the accepted field-weight priority, then schema order for equal weights.
- **GREEN:** one fixture per field has correct attribution/highlight; multi-field fixtures list all matches; no heuristic causal claim remains.
- **Dependency:** Slice 1 compiled terms.

### Slice 5 — Measured integration, documentation, and release proof

- Extend the versioned corpus with short-tech, Unicode, exact-identifier, hard-negative, suggest-duplicate, ambiguous, and one-field-debug slices.
- Preserve Phase 2 leakage and aggregate thresholds.
- Require exact short-term Hit@1 `1.00` for reviewed known-item queries.
- Require heuristic hard-negative false-positive rate `0` on frozen cases and byte-for-byte deterministic output across repeated runs.
- Require no regression in false `update_existing`; improve recall on the frozen precise-weak/balanced-strong cases.
- Require debug matched-field and snippet-field accuracy `1.00` on single-field fixtures.

### Checkpoint A — Query correctness

Complete Slice 1 and rerun the full accepted Phase 2 quality report. Do not continue if global or slice regressions exceed gates.

### Checkpoint B — Prospective metadata quality

Complete Slice 2 and freeze reviewed additions to the synthetic corpus. Confirm no stored fixture is implicitly re-enriched unless that fixture is explicitly recreated by the test.

### Checkpoint C — Downstream behavior

Complete Slices 3 and 4. Verify suggestion precision/recall, one-time retrieval counts, DTO privacy, and exact debug attribution.

### Checkpoint D — Phase integration

Complete Slice 5, update active documentation and Project Memory, then run final verification with the real database diff empty.

## 8. Test strategy and visible acceptance

### Required query slices

- Short technical: `C`, `R`, `x`, `Go`, `DB`, `UI`, `io`, `v2`, `S3`, `x509`.
- Acronyms and stopword collisions: `API`, `MCP`, `SQL`, `JWT`, `OR`, `IN`, `com.example`.
- Compounds: `io.Reader`, `context.Context`, `ErrNotFound`, `memory_fts`, `x-request-id`.
- Paths: `internal/search/fts.go`, `go.mod`, `.env`, and Windows-style paths.
- Safety: FTS operators, wildcards, quotes, unbalanced punctuation, and Unicode.
- Modes: precise AND and balanced OR over identical terms; broad retains its measured wider behavior.

### Focal commands

```bash
go test ./internal/search -run 'Query|Compile|Short|Evidence|Snippet|MatchedField' -count=1 -v
go test ./internal/enrich -run 'Heuristic|Unicode|Determin|HardNegative' -count=1 -v
go test ./internal/mcpserver -run 'Suggest|SearchDebug|RetrievalCount|ContentProjection' -count=1 -v
go test ./internal/search -run 'RetrievalQuality|Corpus|Metric' -count=1 -v
```

Each filtered boundary is also captured with `go test -json`; completion proof requires at least one matching `run` and `pass` event. Exit zero with no executed matching test is failure.

### Final sequence

```bash
./scripts/verify-real-db-untouched.sh -- go test -race -count=1 ./...
go build ./cmd/memory-mcp
go build ./cmd/memory-monitor
go build ./cmd/memory-maintenance
git diff --check
```

The quality report must include global and per-slice before/after values. The guard itself must report unchanged filesystem entries for the real database and sidecars without opening their contents, while source inspection separately proves explicit temporary paths and absence of protected-path references.

## 9. Data, migration, recovery, and privacy

- The approved Phase 3 implementation has no schema migration, stored-data migration, global re-enrichment, or required reindex.
- Query, suggest, and debug changes are query-time and reversible by binary rollback.
- Local heuristic changes apply prospectively through normal atomic upsert behavior.
- Debug remains an explicit diagnostic tool; normal memory responses must not gain snippets, matched fields, enrichment, or hidden content.
- Broader suggestion search must remain project-scoped and final-only retrieval recording must not invent or repair historical counts.

### Historical re-enrichment Hard Stop Gate

Current storage does not record `enricher_kind` or `enricher_version`. Existing enrichment may be heuristic or provider-generated and cannot be classified reliably.

Therefore Phase 3 must not overwrite existing enrichment automatically. Historical remediation requires a separately approved feature/phase that:

1. adds provenance/version through the accepted Phase 2 migration system;
2. marks existing rows `legacy_unknown` rather than guessing;
3. defines explicit operator policy for `legacy_unknown`;
4. performs authorized project-scoped batches with verified backup, rollback, progress, and resumability;
5. replaces enrichment and FTS in the same transaction without changing original content/source timestamps;
6. proves quality improvement on the synthetic corpus before real-data authorization.

A simple FTS rebuild is not a substitute because it preserves the same old metadata.

## 10. Documentation impact

Update active English documentation:

- `docs/ai/storage-search-guide.md`: shared normalization, exact/prefix policy, typed empty-query behavior, debug evidence.
- `docs/ai/enrichment-guide.md`: Unicode/deterministic prospective heuristic contract and historical limitation.
- `docs/ai/memory-lifecycle-internals.md`: suggestion fallback, candidate deduplication, final-only retrieval recording.
- `docs/ai/mcp-tools-guide.md`: invalid-query behavior and structured debug output.
- `docs/ai/historical-decisions.md`: evidence-gated lexical improvement and no automatic historical re-enrichment.
- Project Memory MCP: update retrieval and suggestion behavior only after implementation proof.

## 11. Stop Gates and resolved decisions

### Resolved decisions

1. Keep SQLite `unicode61` and current FTS schema.
2. Exact-match one/two-rune technical tokens; prefix searchable terms of three or more runes.
3. Precise/balanced share terms; empty terms return a typed error.
4. Do not add phrase semantics for user quotes in this phase.
5. Share normalization through one small dependency-free internal package.
6. Keep current suggestion thresholds until corpus evidence justifies a separate change.
7. Run balanced only when precise has no safe recommendation; record final exposed candidates once.
8. Debug attribution comes from actual highlights, not substring heuristics.
9. Local-enrichment changes are prospective; historical rows are not guessed or overwritten.

### Hard Stop Gates

- Stop if the shared scanner cannot be shown to align with `unicode61` on the accepted token matrix.
- Stop if exact short-token support requires changing FTS tokenization or distinguishing `C#`/`C++`; that requires a versioned schema/rebuild design.
- Stop if any suggestion change lowers safe-update precision or weakens ambiguity abstention.
- Stop if debug evidence requires exposing content through normal tools.
- Stop if corpus thresholds must be relaxed to accept the implementation.
- Stop if historical consistency is required for acceptance; route it to the separate provenance/migration design above.
- Stop any command that may access `data/memory.db`.

## 12. Completion checklist and proof

- [ ] Shared Unicode-aware technical token tests pass.
- [ ] Search and count use one compiler and return the same typed empty-query error.
- [ ] Reviewed short technical terms are exact and retrievable; long prefix compatibility remains.
- [ ] FTS syntax injection tests pass and project leakage remains zero.
- [ ] Heuristic Unicode, UTF-8, deterministic-order, hard-negative, alias, and phrase-cap tests pass.
- [ ] Existing stored enrichment is untouched.
- [ ] Precise-safe suggestion avoids balanced; precise-weak/ambiguous evaluates balanced conservatively.
- [ ] Candidate IDs are deduplicated and retrieval counts increment once per public candidate.
- [ ] Internal content helps evidence where approved but never leaks through candidate DTOs.
- [ ] Debug reports correct matched fields and highlighted snippet for every indexed field.
- [ ] Phase 2 aggregate/slice gates pass without relaxed thresholds.
- [ ] Full race-enabled tests and required binaries build.
- [ ] Active docs and Project Memory match implemented behavior.
- [ ] The guard reports the real database and sidecars unchanged, and source inspection proves explicit temporary test paths with no protected-path reference.

Completion proof contains pre-fix RED outputs, post-fix focal GREEN outputs, before/after quality reports, project-leakage and suggestion-precision results, DTO sentinel checks, exact debug attribution fixtures, the final verification output, and temporary fixture paths.

## 13. Git and delivery expectations

- Implement on a dedicated `codex/` branch after Phase 2 acceptance.
- Keep commits reviewable by boundary: shared query normalization, heuristic enrichment, suggestion/accounting, debug evidence, corpus/docs.
- Never commit databases, snapshots, generated metric reports, temporary overlays, or binaries.
- Do not include historical re-enrichment or tokenizer migration in this phase.
- After user approval and predecessor acceptance, hand this specification to `implement-project-phase`.
