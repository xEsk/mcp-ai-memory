# Historical Decisions

## Purpose

This document preserves decisions already visible in the repository so future agents do not casually reverse them.

Use it when a requested change touches architecture, persistence, search, or provider integration.

## Decisions

### Single-binary local-first MCP server

The project is organized as one Go binary under `cmd/memory-mcp`. Keep this shape unless explicitly asked to introduce another runtime model.

### Local Go module path

The Go module path is `mcp-ai-memory` because the code is only used by this MCP server. Do not change it to a GitHub import path unless the project is intentionally published as an importable remote module.

### SQLite as primary local persistence

SQLite is the main database. The code uses `modernc.org/sqlite`, which keeps the project pure Go and supports FTS5 in the tested path.

### Explicit database identity and fail-closed schema handling

The server takes its required, absolute database path only from `MEMORY_DB_PATH`; it does not load `.env` files. Existing stores are classified before writes, recognized pre-ledger current stores are backed up before adoption, and unsupported schema states fail closed rather than being modified opportunistically.

### Offline continuity operations

Backup, restore, and FTS rebuild are explicit `memory-maintenance` operations, excluded from the normal MCP runtime by a per-database operation lock. Authoritative rows remain the source of truth; FTS is derived and may be rebuilt from them.

### FTS5 before vector search

Search is implemented with SQLite FTS5 over original and enriched fields. Embedding/vector interfaces exist, but vector search is an extension point rather than active v1 behavior.

### Precise search by default with explicit broadening

`memory.search` defaults to `precise` mode to reduce unrelated retrieval. Agents can opt into `balanced` or `broad` mode when they need wider recall, and can call `memory.search.count` before fetching memories to reduce token use.

### Project isolation by design

Memory tools require a project key. Repositories and search then use project ownership filters. Do not add cross-project behavior by accident.

### Enrichment is optional and additive

Heuristic enrichment is the default and works offline. OpenAI-compatible enrichment can improve metadata but must not be required.

### Soft delete for v1

`memory.delete` marks `deleted_at` and removes the FTS row. It does not physically delete memory rows.

### Enrichment separated from original memory rows

`memories` stores original content and usage metadata. Derived retrieval fields live in `memory_enrichments`, while `memory_fts` remains the search index. Existing older databases may still physically contain legacy enrichment columns on `memories`, but current code reads and writes `memory_enrichments`.

## Unknowns

[UNKNOWN] Production packaging and release workflow are not visible in the repository.

[UNKNOWN] Long-term vector storage backend has not been selected.

## Related Documents

Also read:

- `critical-rules.md`
- `storage-search-guide.md`
