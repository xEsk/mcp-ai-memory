# Storage And Search Guide

## Purpose

This document explains SQLite persistence and FTS5 search behavior.

Read this before changing migrations, repositories, search ranking, or deletion behavior.

## How This Area Works

SQLite is opened in `internal/storage/sqlite.go` using `modernc.org/sqlite`. `OpenSQLite` classifies an existing database read-only before it allows writes. The current schema is applied through a checksummed `schema_migrations` ledger; recognized current unversioned stores are adopted, while unknown, partial, future, version-gapped, or checksum-drifted stores fail closed.

The main tables are `projects`, `memories`, `memory_enrichments`, and virtual table `memory_fts`. Projects and memories have independent `deleted_at` fields. A deleted project is an inactive ownership boundary even when some of its memories remain individually active.

Memory upsert writes original memory data to `memories`, writes derived retrieval metadata to `memory_enrichments`, and replaces the FTS row in a single transaction. Memory soft delete sets `memories.deleted_at` and removes the FTS row. Project soft delete sets `projects.deleted_at`; normal project resolution prevents every memory operation for that project while preserving its data for restoration.

Returned memories track aggregate retrieval/exposure history internally with `access_count` and `last_accessed_at`. Full public memory output projects these as `retrieval_count` and `last_retrieved_at`; compact search cards omit them. Search, get, list, and debug search still increment the fields for active memories they return. Suggestion performs its precise and optional balanced searches without recording, deduplicates the final public candidates, and records each final ID once. `memory.search.count` returns only the match count and must not increment access metrics.

## Main Code Locations

- `internal/storage/sqlite.go`
- `internal/storage/migrations.go`
- `internal/project/repository.go`
- `internal/memory/repository.go`
- `internal/search/fts.go`

## Search Fields

FTS indexes:

- `type`
- `title`
- `summary`
- `short_summary`
- `keywords`
- `tags`
- `entities`
- `aliases`
- `search_phrases`
- `content`

`memory_id` and `project_key` are stored as unindexed fields for lookup and scoping.

## Ranking Behavior

Search supports three modes. `precise` is the default and removes common stopwords before requiring all remaining terms. `balanced` removes common stopwords and keeps OR matching, but a query with four or more normalized terms returns no matches when at least half of those terms do not occur in any active memory in the requested project. `broad` stays close to the original expansion behavior with OR prefix matching and deliberately bypasses that project-vocabulary guard.

`internal/retrievaltext` provides the shared Unicode-aware technical token boundaries used by search and local enrichment. Tokens retain display text and a lowercase, Latin-diacritic-folded comparison identity aligned with SQLite `unicode61` evidence. Search compiles one- and two-rune terms as escaped exact FTS terms and terms of three or more runes as escaped prefixes. Precise and balanced use the same normalized terms and differ only in `AND` versus `OR`; broad retains stopwords for wider measured recall. Empty, punctuation-only, or stopword-only normalized input returns `search.ErrInvalidQuery` instead of executing an empty FTS expression. Quotes are treated as separators, not user phrase syntax.

Ranking uses SQLite `bm25`. Because `memory_id` and `project_key` occupy the first two FTS column positions even though they are `UNINDEXED`, the query supplies two leading neutral placeholders before the documented weights for `type` through `content`.

Results are ordered by rank, then `importance`, then `updated_at`.

The balanced project-vocabulary guard probes term existence with the same exact/prefix expressions as the compiled query and preserves both project ownership filters. It prevents long unsupported queries from ranking memories solely through peripheral project vocabulary. `memory.search` and `memory.search.count` apply the same guard, and the probes do not record memory access. It is not semantic confidence: a passing query still relies on normal FTS5 matching and BM25 ordering.

Search obtains match evidence from SQLite `highlight()` across all ten indexed fields. Normal cards project only the selected highlighted context as `match_excerpt` and its source as `match_field`, so an agent can select candidates without treating the preview as authoritative content. Debug search additionally returns ordered `matched_fields`, `snippet_field`, `snippet`, score, and an explanation derived from the same evidence.

A versioned synthetic temporary-database corpus exercises the real schema and services across exact identifiers, conceptual questions, noisy multi-intent queries, shared vocabulary, and no-answer cases. It reports Hit@1, Recall@5, MRR@10, nDCG@5, false positives, leakage, and critical misses separately by mode. A separate ranking fixture asserts relative field ordering rather than SQLite-version-specific floating-point scores. Query-time ranking and vocabulary changes require no schema migration, FTS rebuild, or re-enrichment.

## How To Extend Existing Code

When adding searchable metadata:

1. Add the storage column to `memory_enrichments` if it is derived retrieval metadata.
2. Add the FTS column if it must affect search.
3. Update `replaceFTS`.
4. Update scan helpers and model structs.
5. Add an integration test proving search behavior and project isolation.

## Existing Conventions

- `project_id` is the relational ownership key.
- `project_key` is duplicated into FTS for project-scoped search filtering.
- Timestamps are stored as RFC3339Nano strings.
- JSON arrays are stored as JSON text in `memory_enrichments.*_json` columns.
- Access metrics live on `memories`; they are not indexed in FTS.
- `memories` should remain focused on original content, source metadata, timestamps, soft delete, and usage metrics.
- `memory_enrichments` and `memories` are authoritative; `memory_fts` is derived and can be rebuilt from active rows.

## Offline Maintenance

Read this section before operating on any existing database. Normal MCP runtimes hold compatible shared reservations when the schema is already current; SQLite serializes their writes with the configured bounded busy timeout. Symlinks resolve to the canonical database path, while a database with multiple hard links is rejected for every runtime, management, or mutating maintenance open because rollback recovery files are pathname-derived. Schema initialization or migration requires an exclusive reservation and fails without writes while another runtime is active. `inspect` is a zero-write classification operation and does not acquire the operation lock. Stop all MCP instances before `backup`, `restore`, or `rebuild-fts`; those commands require an exclusive `<db>.operation.lock` reservation.

All commands require absolute paths and write status only to stderr:

```bash
go run ./cmd/memory-maintenance --operation inspect --db /absolute/path/to/memory.db
go run ./cmd/memory-maintenance --operation backup --db /absolute/path/to/memory.db
go run ./cmd/memory-maintenance --operation restore --db /absolute/path/to/memory.db --backup /absolute/path/to/snapshot.db
go run ./cmd/memory-maintenance --operation rebuild-fts --db /absolute/path/to/memory.db
```

- `inspect` classifies the store without opening it for writes or creating lock/database files.
- `backup` requires an existing recognized current store, opens it without runtime migration, uses SQLite `VACUUM INTO`, verifies `PRAGMA integrity_check`, and exclusively creates a same-directory manifest containing filename, size, SHA-256, and creation time. An existing manifest path, including a symlink, is never replaced or followed.
- `restore` requires the manifest, verifies its identity, size, checksum, and SQLite integrity before preserving each existing database artifact as a timestamped quarantine file; it verifies the installed copy again and rolls back only the artifacts that were moved. A failed installation is removed even when no original source existed, and any rollback failure is returned with the surviving quarantine path.
- `rebuild-fts` requires an existing recognized current store, opens it without runtime migration, and replaces only `memory_fts` rows from active authoritative records in one transaction.

Do not use normal MCP startup to run offline maintenance or validate a backup: startup is migration-capable, whereas maintenance opening is deliberately side-effect-free except for the named operation. Never run automated commands against `data/memory.db`; use a new path under `t.TempDir()` and the guard described in `development-guide.md`.

`memory-mcp manage` is also an exclusive offline operation, but it uses the normal classified migration path before opening its interface. Its explicitly confirmed cleanup transaction purges soft-deleted projects and their owned data, soft-deleted memories in active projects, orphaned enrichments, and FTS rows that do not map to an active memory in an active project. It checks that no eligible rows remain before commit. This does not securely erase bytes from SQLite free pages, WAL files, backups, or storage media.

## Common Mistakes To Avoid

- Do not query `memory_fts` without joining/filtering active `memories`.
- Do not forget `m.deleted_at IS NULL`.
- Do not use string-concatenated user input in SQL. Dynamic placeholders are allowed only for controlled filter lists.
- Do not treat tag filtering with `LIKE` as a perfect normalized tag query; it is a simple v1 filter.
- Do not add memory-returning APIs that bypass access tracking.

## Validation

Run:

```bash
go test ./...
```

The current memory integration test uses a temporary SQLite database and checks project-scoped search, access metrics, and soft delete.

## Related Documents

Also read:

- `enrichment-guide.md`
- `critical-rules.md`
