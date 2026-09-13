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

`memory.search` uses FTS5 over source and enrichment fields. It defaults to `precise`; `balanced` and `broad` widen recall. The shared Unicode scanner splits technical separators consistently. One- and two-rune terms are escaped exact terms; longer terms are escaped prefixes. Precise combines accepted terms with `AND`, while wider modes use `OR`. Precise and balanced share the same term set, and normalized empty input returns a typed invalid-query error. Before running a balanced query with at least four normalized terms, search checks the same exact/prefix terms against active memories in the requested project. If at least half are absent, search and count return zero; broad intentionally remains the unrestricted exploratory mode.

`memory.search.count` uses the same compiler and filter but returns only a count and does not record access. `memory.search.debug` uses the normal retrieval path and exposes SQLite-highlight-backed matched fields, selected snippet field, highlighted snippet, score, and an evidence-derived explanation.

`memory.get` and `memory.list` query active rows directly. Get returns authoritative full content. List supports one type and one simple JSON-text tag filter, orders by `updated_at DESC`, and does not use FTS.

Search results are ranked by FTS5 BM25, then memory importance, then update time. The two leading FTS columns are `UNINDEXED`; preserve the two leading neutral BM25 weights before field weights.

## Retrieval Metrics and Public Output

Search, debug search, get, list, and suggestion candidates increment `memories.access_count` and set `last_accessed_at` for active returned rows. These fields are aggregate server exposure, not proof of client reuse or value.

Full memory responses expose access fields as `retrieval_count` and `last_retrieved_at`. Normal search instead returns compact candidate cards: `id`, `title`, bounded `description`, `source`, `updated_at`, `match_excerpt`, and `match_field`. `content` is present only when `include_content` is true.

Card descriptions reuse stored `short_summary`, with a deterministic source-content fallback when it is empty. Match guidance reuses the selected SQLite highlight already computed by search. Both are navigation aids and may reflect generated enrichment; only full `content` from `memory.get` or explicit search inclusion is authoritative.

## Suggestion and Delete

`memory.suggest` is non-mutating with respect to stored memory content. It searches `input` plus optional context in `precise` mode without recording exposure, evaluates candidate quality using the compiled query terms and internal source/enrichment evidence, and retries with `balanced` whenever precise has no safe recommendation. Its internal no-recording candidate search deliberately bypasses the public balanced project-vocabulary guard so the existing weak/possible/strong evaluator retains authority over suggestion candidates. It merges and deduplicates the final set, preserves conservative ambiguity abstention, then records only the final public candidate IDs once. Internal content may support quality evaluation but is removed by the public candidate response projection.

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
- Keep search cards compact and explicitly non-authoritative. Retrieval machinery remains internal except for the bounded description and match guidance required to select candidates.

## Related Documents

- `mcp-tools-guide.md` for tool contracts and handlers.
- `storage-search-guide.md` for schema, ranking fields, and recovery.
- `enrichment-guide.md` for generated metadata behavior.
