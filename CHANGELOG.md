# Changelog

All notable changes to this project are documented in this file.

## [0.2.0] - 2026-09-13

### Added

- Added `memory.server.info` for confirming the identity and version of the MCP server loaded by a client.
- Added safe offline database inspection, backup, restore, and FTS rebuild operations through `memory-maintenance`, including integrity checks, manifests, exclusive operation locks, quarantine, and rollback handling.
- Added versioned SQLite schema migrations with strict shape classification and checksummed migration history.
- Added retrieval counters and last-retrieved timestamps to public memory responses.
- Added Unicode-aware technical query compilation, exact short-token handling, and deterministic local enrichment for identifiers, paths, dotfiles, aliases, and search phrases.
- Added quality-aware `memory.suggest` fallback, truthful SQLite-highlight-backed `memory.search.debug` evidence, and compact `memory.search` candidate cards.
- Added versioned retrieval-quality corpora and per-mode Hit@1, Recall@5, MRR@10, nDCG@5, false-positive, leakage, and critical-miss gates.
- Added guarded CI and local verification that keep automated tests on temporary databases and detect access to the real project store.

### Changed

- Runtime configuration now comes only from the process environment; `MEMORY_DB_PATH` is required and must be absolute.
- `balanced` search now returns no matches for queries with at least four normalized terms when at least half of their vocabulary is absent from active memories in the requested project. `precise` retains strict `AND` semantics and `broad` remains unrestricted for exploratory recall.
- Search uses corrected FTS5 BM25 field positions while preserving the documented field weights and project isolation.
- Suggestion retrieval evaluates precise candidates before widening, deduplicates results, preserves ambiguity abstention, and records retrieval exposure only for the final public candidates.
- Tracing is disabled by default, requires explicit `MEMORY_TRACE_MODE`, restricts enabled endpoints to loopback, drains accepted events on shutdown, and reports queue drops without payload data.

### Fixed

- Fixed soft-delete transaction leaks and preserved atomic memory/FTS state on success and rollback.
- Fixed backup, restore, migration, and FTS rebuild edge cases involving missing stores, manifests, sidecars, symlinks, lock recovery, schema drift, and failed rollback.
- Fixed Unicode normalization and SQLite `unicode61` parity for combining marks, non-folded scripts, malformed input, technical separators, and one- or two-rune terms.
- Fixed heuristic enrichment substring false positives and nondeterministic output.
- Fixed misleading search-debug match attribution and duplicate MCP model context caused by redundant response projections.
- Fixed weak precise suggestion matches blocking stronger balanced candidates.

## [0.1.0] - 2026-07-14

- Initial local-first MCP server for project-scoped technical memories, deterministic or provider-backed enrichment, SQLite FTS5 retrieval, suggestions, soft deletion, and optional local trace monitoring.
