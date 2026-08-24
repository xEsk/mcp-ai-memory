# Storage And Search Guide

## Purpose

This document explains SQLite persistence and FTS5 search behavior.

Read this before changing migrations, repositories, search ranking, or deletion behavior.

## How This Area Works

SQLite is opened in `internal/storage/sqlite.go` using `modernc.org/sqlite`. `OpenSQLite` classifies an existing database read-only before it allows writes. The current schema is applied through a checksummed `schema_migrations` ledger; recognized current unversioned stores are adopted, while unknown, partial, future, version-gapped, or checksum-drifted stores fail closed.

The main tables are `projects`, `memories`, `memory_enrichments`, and virtual table `memory_fts`.

Memory upsert writes original memory data to `memories`, writes derived retrieval metadata to `memory_enrichments`, and replaces the FTS row in a single transaction. Soft delete sets `deleted_at` and removes the FTS row.

Returned memories track aggregate retrieval/exposure history internally with `access_count` and `last_accessed_at`. Public MCP output projects these as `retrieval_count` and `last_retrieved_at`; they do not demonstrate that a client read, reused, or valued a memory. Search, get, list, and debug search increment these fields for active memories they return. Suggestion performs its precise and optional balanced searches without recording, deduplicates the final public candidates, and records each final ID once. `memory.search.count` returns only the match count and must not increment access metrics.

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

Search supports three modes. `precise` is the default and removes common stopwords before requiring all remaining terms. `balanced` removes common stopwords but keeps OR matching. `broad` stays close to the original expansion behavior with OR prefix matching.

`internal/retrievaltext` provides the shared Unicode-aware technical token boundaries used by search and local enrichment. Tokens retain display text and a lowercase, Latin-diacritic-folded comparison identity aligned with SQLite `unicode61` evidence. Search compiles one- and two-rune terms as escaped exact FTS terms and terms of three or more runes as escaped prefixes. Precise and balanced use the same normalized terms and differ only in `AND` versus `OR`; broad retains stopwords for wider measured recall. Empty, punctuation-only, or stopword-only normalized input returns `search.ErrInvalidQuery` instead of executing an empty FTS expression. Quotes are treated as separators, not user phrase syntax.

Ranking uses SQLite `bm25`. Because `memory_id` and `project_key` occupy the first two FTS column positions even though they are `UNINDEXED`, the query supplies two leading neutral placeholders before the documented weights for `type` through `content`.

Results are ordered by rank, then `importance`, then `updated_at`.

Debug search obtains match evidence from SQLite `highlight()` across all ten indexed fields. It returns ordered `matched_fields`, the weight-prioritized `snippet_field`, a highlighted snippet, and an explanation derived from that same evidence. Normal search DTOs do not expose these diagnostic fields.

A synthetic temporary-database ranking fixture exercises the real schema and `FTS.Search` and asserts relative field ordering rather than SQLite-version-specific floating-point scores. The correction is query-time only, so it requires no schema migration, FTS rebuild, or re-enrichment.

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

Read this section before operating on any existing database. `inspect` is a zero-write classification operation and does not acquire the operation lock. Stop all MCP instances before `backup`, `restore`, or `rebuild-fts`; those commands use the same exclusive `<db>.operation.lock` reservation as the server.

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
