# Phase 3 Completion Evidence

## Scope and safety

Phase 3 changes retrieval normalization, prospective local enrichment, suggestion candidate evaluation/accounting, and explicit debug evidence. It adds no schema or tokenizer migration and performs no historical re-enrichment. Every database fixture uses a new path beneath `t.TempDir()`.

The implementation ran only through the real-store guard. The final guarded suite reported `real database filesystem metadata unchanged`; it did not open the configured project store. The separately authorised Phase 2 snapshots and adopted v1 migration ledger remain unchanged operator-controlled artifacts.

## Implemented boundaries

- `internal/retrievaltext` owns Unicode letter, number, and mark boundaries, lowercase identity, rune counts, source-order tokenization, and stable case-insensitive deduplication.
- Search compiles one- and two-rune terms as escaped exact FTS terms and longer terms as escaped prefixes. Precise and balanced share terms and differ only by `AND`/`OR`; normalized empty input returns `ErrInvalidQuery`.
- Local heuristic enrichment is rune-safe and deterministic, preserves reviewed short identifiers, dotfiles, and paths, uses ordered whole-token/phrase tag and alias rules, and caps search phrases at 18. It applies only to new or explicitly upserted memories.
- Suggestion evaluates precise results before broadening, runs candidate searches without recording, uses source content only inside the quality boundary, deduplicates the balanced/precise final set, preserves conservative ambiguity abstention, and records final public IDs once.
- Debug evidence comes from SQLite `highlight()` across all ten indexed fields. It exposes ordered `matched_fields`, the weight-prioritized `snippet_field`, a highlighted snippet, and an explanation from the same evidence. Normal search and suggestion projections omit hidden content and debug fields.

## RED/GREEN evidence

The pre-change implementation was characterized directly before each focal change: it removed one-rune and most two-rune terms, prefixed every retained term, returned `MATCH ""` for normalized empty input, used an ASCII enrichment regex and byte truncation, inferred tags/aliases through substrings and unordered maps, broadened suggestions only on zero precise rows while recording both searches, and attributed debug matches with substring guesses plus a content-only snippet.

The walking-skeleton and focal tests then established the corrected behavior:

- temporary SQLite fixtures retrieve `Go`, `DB`, and exact `io` without matching only `iostream`, while count/search share behavior and project leakage stays zero;
- operator, wildcard, quote, punctuation, malformed UTF-8, path, compound identifier, acronym, stopword-collision, exact-short, and long-prefix cases compile as data rather than FTS syntax;
- exact-output enrichment remains byte-for-byte stable across repeated runs, is valid UTF-8, respects rune and phrase limits, and has zero false positives for `build ≠ ui`, `contest ≠ test`, and `capitalization ≠ api`;
- a weak precise candidate no longer blocks a strong balanced candidate, duplicate IDs are removed, database exposure counters increment once, precise-safe cases do not broaden, ambiguous cases abstain, and a sentinel in internal content is absent from the public DTO;
- real SQLite fixtures attribute each indexed field exactly and select the accepted highest-weight field for multi-field snippets.

The required filtered commands were also run through `go test -json`; each filter produced matching `run` and `pass` events.

## Quality report

The accepted Phase 2 corpus `2026-07-14-v1` reported global Hit@1, Recall@5, MRR@10, and nDCG@5 of `1.00`.

The frozen Phase 3 corpus `2026-08-24-v2` adds short technical, Unicode, and heuristic hard-negative slices. Its final report is:

- global Hit@1 `1.00`, Recall@5 `1.00`, MRR@10 `1.00`, nDCG@5 `1.00`;
- 13 evaluable queries and 4 no-match queries;
- zero no-match false positives, project leakage, and critical grade-3 misses;
- short-technical Hit@1 `1.00` and heuristic hard-negative false-positive count `0`;
- every evaluable per-slice score remains `1.00`; no threshold was relaxed.

## Final verification

At implementation HEAD after the dotfile/path regression correction:

```bash
make verify
```

The command exited zero. It ran the guard self-tests, structural temporary-database checker, guarded race-enabled full suite, all three command builds, Go formatting check, protected-path source checks, and `git diff --check`. The guard reported unchanged real-database filesystem metadata.

The final delivery additionally builds ignored binaries as:

```bash
go build -o dist/memory-mcp ./cmd/memory-mcp
go build -o dist/memory-monitor ./cmd/memory-monitor
go build -o dist/memory-maintenance ./cmd/memory-maintenance
```

Independent CRITICAL review challenged short-token suggestion evidence, exact `unicode61` folding/boundaries, and forged debug-highlight sentinels. The implementation added focused regressions and exact temporary-SQLite parity coverage for each finding. Checkpoint D cleared at `af80daa` with no remaining actionable discrepancies; graph coverage matched HEAD with no recorded gaps in the reviewed files.
