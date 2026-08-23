# Memory Lifecycle Internals

## Purpose

Read this before changing memory writes, retrieval semantics, response shaping, or the boundary between source data and retrieval metadata. For schema, backup, and recovery work, also read `storage-search-guide.md`.

## Main Flow

`internal/mcpserver/tools.go` binds and validates MCP requests. `internal/memory/service.go` resolves the project, validates memory input, requests enrichment, and delegates persistence. `internal/memory/repository.go` owns memory transactions and FTS synchronization. `internal/search/fts.go` queries and records retrieval exposure. `internal/mcpserver/responses.go` projects internal models into public MCP responses.

Every memory operation begins by resolving a project key to its internal `project_id`. Reads must remain constrained by that ID; FTS search additionally filters the duplicated `project_key`.

## Write: `memory.upsert`

- `project`, `title`, and `content` are required. The service trims title and content, normalizes type, and clamps importance to `1..5`.
- Omitting `enrich` means `true` at the MCP boundary. A configured provider failure falls back to `LocalHeuristicEnricher`.
- `enrich: false` still uses the local heuristic, so local retrieval metadata remains available without an external provider.
- A missing ID receives a generated ID. An upsert with an existing same-project ID preserves `created_at`, clears `deleted_at`, and replaces editable source fields.
- One repository transaction writes `memories`, upserts `memory_enrichments`, and replaces the `memory_fts` row. Keep these three steps together.

Original `content` is source data. Generated summary, keywords, tags, entities, aliases, and search phrases are additive retrieval metadata and must not replace it.

## Retrieval: Search, Get, and List

`memory.search` uses FTS5 over source and enrichment fields. It defaults to `precise`; `balanced` and `broad` widen recall. FTS terms are cleaned prefix terms; `precise` combines accepted terms with `AND`, while the wider modes use `OR`.

`memory.search.count` uses the same filter but returns only a count and does not record access. `memory.search.debug` uses the normal retrieval path but exposes diagnostic snippets, scores, and match explanations.

`memory.get` and `memory.list` query active rows directly. List supports one type and one simple JSON-text tag filter, orders by `updated_at DESC`, and does not use FTS.

Search results are ranked by FTS5 BM25, then memory importance, then update time. The two leading FTS columns are `UNINDEXED`; preserve the two leading neutral BM25 weights before field weights.

## Retrieval Metrics and Public Output

Search, debug search, get, list, and suggestion candidates increment `memories.access_count` and set `last_accessed_at` for active returned rows. These fields are aggregate server exposure, not proof of client reuse or value.

Public responses expose them as `retrieval_count` and `last_retrieved_at`. Normal memory output contains only core source fields; content is omitted from normal search unless `include_content` is true. Do not expose enrichment values, snippets, scores, or match explanations except through explicit diagnostics.

## Suggestion and Delete

`memory.suggest` is non-mutating. It searches `input` plus optional context in `precise` mode, retrying `balanced` only when precise finds none. It returns up to five candidates, local quality classification, and one recommended candidate only when the deterministic quality gate supports `update_existing`.

`memory.delete` soft-deletes an active same-project row and removes its FTS document in one transaction. Source and enrichment rows remain. Upserting the same ID can restore the row; get, list, and search must otherwise exclude it.

## Data Responsibilities

- `memories`: source content, ownership, timestamps, soft-delete state, and retrieval metrics.
- `memory_enrichments`: derived retrieval metadata as JSON arrays plus summaries.
- `memory_fts`: derived FTS5 documents; `memory_id` and `project_key` are unindexed linkage and scoping fields.

## Preserve These Invariants

- Resolve the project before every memory operation and keep reads scoped by project ownership.
- Keep both FTS `project_key` and relational `project_id` search filters.
- Keep source, enrichment, and FTS writes transactional.
- Do not return deleted rows or write protocol diagnostics to stdout.
- Treat normal MCP output as a stable public contract; retrieval machinery remains internal by default.

## Related Documents

- `mcp-tools-guide.md` for tool contracts and handlers.
- `storage-search-guide.md` for schema, ranking fields, and recovery.
- `enrichment-guide.md` for generated metadata behavior.
