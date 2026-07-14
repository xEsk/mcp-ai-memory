# Storage And Search Guide

## Purpose

This document explains SQLite persistence and FTS5 search behavior.

Read this before changing migrations, repositories, search ranking, or deletion behavior.

## How This Area Works

SQLite is opened in `internal/storage/sqlite.go` using `modernc.org/sqlite`. The schema is initialized idempotently from `internal/storage/migrations.go`.

The main tables are `projects`, `memories`, `memory_enrichments`, and virtual table `memory_fts`.

Memory upsert writes original memory data to `memories`, writes derived retrieval metadata to `memory_enrichments`, and replaces the FTS row in a single transaction. Soft delete sets `deleted_at` and removes the FTS row.

Returned memories track usage with `access_count` and `last_accessed_at`. Search, get, and list operations increment these fields for active memories they return. `memory.search.count` returns only the match count and must not increment access metrics.

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

Ranking uses SQLite `bm25` with higher weight on title and enriched retrieval fields.

Results are ordered by rank, then `importance`, then `updated_at`.

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
