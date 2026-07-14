# Memory Lifecycle Internals

## Purpose

This document explains exactly how project memory is stored, enriched, indexed, searched, returned, listed, read, suggested, and deleted.

It is intentionally isolated from task-oriented guides. Use it when you need a detailed mental model of the internal data flow before changing persistence, enrichment, search, or MCP responses.

## Main Components

The MCP boundary is in `internal/mcpserver/tools.go`.

Memory write/read orchestration is in `internal/memory/service.go`.

SQLite persistence and FTS synchronization are in `internal/memory/repository.go`.

Search query execution is in `internal/search/fts.go`.

Response shaping is in `internal/mcpserver/responses.go`.

The schema is initialized in `internal/storage/migrations.go`.

## Stored Tables

`projects` stores project identity:

- `id`: internal project UUID.
- `key`: stable user-facing project key.
- `name`, `description`, timestamps.

`memories` stores source memory data and usage metrics:

- `id`
- `project_id`
- `type`
- `title`
- `content`
- `importance`
- `source`
- `access_count`
- `last_accessed_at`
- `created_at`
- `updated_at`
- `deleted_at`

`memory_enrichments` stores derived retrieval metadata:

- `memory_id`
- `summary`
- `short_summary`
- `keywords_json`
- `tags_json`
- `entities_json`
- `aliases_json`
- `search_phrases_json`
- `updated_at`

`memory_fts` is the SQLite FTS5 table used by search:

- `memory_id` is `UNINDEXED`.
- `project_key` is `UNINDEXED`.
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

## Write Flow: `memory.upsert`

The MCP handler binds JSON arguments into `memoryUpsertInput`.

If the caller omits `enrich`, the handler sets `input.Enrich = true`. This means enrichment is enabled by default at the MCP boundary.

`memory.Service.Upsert` resolves the input `project` key through `project.Service.MustGetByKey`. The service then works with both the internal `project_id` and the public `project_key`.

The service trims `title` and `content`. Both must be non-empty. The project must already exist. The memory type is normalized and validated. Importance is clamped into the `1..5` range.

The service calls an `enrich.Enricher` when `input.Enrich` is true. If that configured enricher fails, it falls back to `LocalHeuristicEnricher`.

When `input.Enrich` is false, the service still calls `LocalHeuristicEnricher`. This keeps local retrieval metadata available without using an external provider.

The original input `content` is always preserved. Enrichment output only fills retrieval metadata such as summary, keywords, entities, aliases, and search phrases.

If enrichment suggests a valid type and the caller did not provide a type, the suggested type can become the stored type. If enrichment suggests an importance of at least `1`, the service can use it, capped at `5`.

The service creates a new ID when the caller did not provide one. It then calls `memory.Repository.Upsert`.

## Persistence Flow Inside `Repository.Upsert`

`Repository.Upsert` starts a SQL transaction.

It checks whether a memory with the same `id` already exists in the same `project_id`, including deleted rows.

If the memory exists:

- It preserves the original `created_at`.
- It updates `type`, `title`, `content`, `importance`, `source`, and `updated_at`.
- It sets `deleted_at = NULL`, so an upsert can restore a soft-deleted memory with the same ID.

If the memory does not exist:

- It inserts a new row into `memories`.

After saving the source memory row, it upserts the enrichment row in `memory_enrichments`.

Then it replaces the FTS row in `memory_fts`: first it deletes the old FTS document for that memory ID, then inserts the current indexed fields.

The source memory row, enrichment row, and FTS row are committed together. This is the key consistency boundary for writes.

After commit, the repository returns the active memory through `GetByID`.

## FTS Indexing

The FTS table indexes both original and enriched fields.

Original or user-controlled searchable fields:

- `type`
- `title`
- `content`

Derived enrichment searchable fields:

- `summary`
- `short_summary`
- `keywords`
- `tags`, which are merged from caller-provided tags and enrichment output.
- `entities`
- `aliases`
- `search_phrases`

`project_key` is stored in the FTS table but is not indexed. It is used as a strict filter, not as a searchable term.

`memory_id` is also stored but not indexed. It links FTS rows back to `memories`.

JSON enrichment arrays are joined into space-separated strings before insertion into `memory_fts`.

## Search Flow: `memory.search`

The MCP handler requires both `project` and `query`.

The handler resolves the project key through `project.Service.MustGetByKey`, then calls `search.Service.Search` with the internal `project_id` and public `project_key`.

`search.Service` currently delegates directly to `FTS.Search`.

`FTS.Search` clamps the result limit:

- Default limit is `10`.
- Maximum limit is `50`.

It builds an FTS query with a search mode:

- `precise` is the default.
- `balanced` is wider than `precise`.
- `broad` is widest.

The query terms are split on whitespace, cleaned, lowercased, and converted to quoted prefix terms. Very short terms and stopwords can be skipped depending on mode.

`precise` joins terms with `AND`.

`balanced` and `broad` join terms with `OR`.

The SQL query searches `memory_fts MATCH ?`, but it also filters:

- `memory_fts.project_key = ?`
- `m.project_id = ?`
- `m.deleted_at IS NULL`

Both `project_key` and `project_id` filters are intentional. Together they preserve project isolation across the FTS and relational paths.

Type filters use `m.type IN (...)`.

Tag filters currently use `COALESCE(e.tags_json, '[]') LIKE ?`. This is a simple v1 JSON-text filter, not a normalized tag join table.

## Search Ranking

Search uses SQLite FTS5 `bm25`.

The FTS table positions begin with the two `UNINDEXED` columns `memory_id` and `project_key`. The `bm25` call therefore supplies two leading neutral placeholders before the ten indexed-field weights below; FTS5 applies all weights positionally.

Weights are configured in `internal/search/fts.go`:

- `type`: `6.0`
- `title`: `1.0`
- `summary`: `5.0`
- `short_summary`: `4.0`
- `keywords`: `4.0`
- `tags`: `3.5`
- `entities`: `3.5`
- `aliases`: `3.5`
- `search_phrases`: `3.0`
- `content`: `1.5`

Results are ordered by:

1. FTS rank ascending.
2. `importance` descending.
3. `updated_at` descending.

The result score exposed by debug output is `-rank`.

## What Search Returns

`FTS.Search` internally scans both source fields and enrichment fields into `memory.Memory`.

If `IncludeContent` is false, `FTS.Search` clears `m.Content` before appending the result.

The normal MCP response then passes each result through `newSearchOutputs`.

Normal `memory.search` output exposes only:

- `id`
- `project`
- `type`
- `title`
- `content`, only when `include_content` is true.
- `importance`
- `source`
- `created_at`
- `updated_at`

Normal `memory.search` does not expose:

- `summary`
- `short_summary`
- `keywords`
- `tags`
- `entities`
- `aliases`
- `search_phrases`
- FTS snippets
- scores
- match explanations

This means a memory can be found through an enrichment-only match, while the normal returned object still contains only the public memory fields.

## Debug Search: `memory.search.debug`

`memory.search.debug` uses the same search path as `memory.search`, but response shaping is different.

It returns each result as:

- `memory`: the public memory output without content.
- `snippet`: an FTS snippet from the `content` column.
- `score`: the debug score.
- `match_explanation`: a generated explanation of which memory fields appear to contain query terms.

Debug output is for retrieval diagnosis. It is not the normal memory API.

## Count Flow: `memory.search.count`

`memory.search.count` validates `project` and `query`, resolves the project, builds the same search filter, and returns only:

- `project`
- `query`
- `mode`
- `count`

It does not return memories.

It does not record access metrics.

## Read Flow: `memory.get`

`memory.get` binds `project` and `id`, then calls `memory.Service.Get`.

The service resolves the project key to `project_id`, fetches only an active memory from that project, and records one access.

`memory.get` returns the public memory output with `content` included.

Even though the repository scans enrichment fields into the internal `memory.Memory`, the MCP response does not expose enrichment metadata.

## List Flow: `memory.list`

`memory.list` resolves the project and returns active memories from that project.

It supports optional type and tag filters.

It orders rows by `m.updated_at DESC`.

It records access for every returned memory.

It returns public memory outputs with `content` included.

It does not use the FTS table.

## Suggest Flow: `memory.suggest`

`memory.suggest` is non-mutating.

It requires `project` and `input`. It searches for similar memories using:

- Query: `input + " " + context`
- Mode: `precise`, then `balanced` only when precise returns no candidates.
- Limit: `5`
- `IncludeContent: false`

The suggested action is heuristic:

- Default is `create`.
- If the best candidate passes the local semantic quality gate, action becomes `update_existing`.
- If candidates exist but none are strong enough, action remains `create` and the candidates are returned for duplicate awareness.
- If the input has fewer than five words, action becomes `ignore`.
- If the input has at least two blank-line section breaks, action becomes `split`.

The local quality gate uses deterministic signals from the search result and enrichment metadata. It checks important-term overlap against the candidate title, summary, keywords, tags, entities, aliases, and search phrases. It also penalizes matches that are only broad project vocabulary such as `project`, `memory`, `task`, `tool`, or `work`. The first candidate is recommended only when its evidence is strong and clearly better than nearby candidates.

It returns ranked candidates without content. Each candidate wraps the public memory output with:

- `rank`: 1-based relevance rank for this suggestion input.
- `is_recommended`: true only for the best `update_existing` candidate.
- `candidate_quality`: structured local match quality: `strong`, `possible`, or `weak`.
- `memory`: public memory output without content.
- `match_reason`: a short deterministic reason for the candidate's role in the suggestion.

The top-level response includes:

- `recommended_candidate_id`: the safe default candidate ID for `update_existing`, or an empty string.
- `confidence`: a coarse heuristic (`high`, `medium`, `low`, or `none`).
- `next_step`: compact agent workflow guidance: `create_focused_memory`, `inspect_recommended_candidate`, `split_input`, or `ignore_input`.
- `candidate_count`: the number of returned candidates.
- `search_mode`: whether candidates came from the initial precise search or the balanced fallback.

Candidate order and `rank` describe retrieval relevance to the current suggestion input. `candidate_quality` describes local semantic match quality. A candidate memory's `importance` remains the memory's global project importance, not the quality of this match.

`match_reason` values are intentionally short semantic guidance, for example `strong title and metadata overlap`, `possible semantic overlap, review before updating`, or `matched only broad project terms`.

Normal `memory.suggest` output does not expose raw search scores, snippets, enrichment fields, aliases, entities, keywords, or debug match explanations. Use `memory.search.debug` for retrieval diagnosis.

Because it calls `Search`, current v1 behavior records access metrics for candidate memories. If precise finds no candidates and balanced is attempted, each search call keeps the normal search access behavior for any results it returns.

## Delete Flow: `memory.delete`

`memory.delete` calls `memory.Service.Delete`, which resolves project ownership and delegates to `Repository.SoftDelete`.

`SoftDelete` starts a transaction.

After the transaction starts, cleanup always attempts rollback on every return path. A successful commit makes that rollback a harmless `sql.ErrTxDone`; missing, repeated, and wrong-project deletes return `ErrNotFound` without retaining the sole SQLite connection.

It sets `deleted_at` and `updated_at` on the `memories` row when the row belongs to the project and is not already deleted.

It deletes the corresponding row from `memory_fts`.

It commits both changes together.

The source memory and enrichment rows remain in SQLite. Normal get, list, and search paths exclude them through `deleted_at IS NULL`, and search also loses the FTS row.

## Access Metrics

Memory usage metrics are stored in `memories.access_count` and `memories.last_accessed_at`.

The following paths record access:

- `memory.search`
- `memory.search.debug`
- `memory.get`
- `memory.list`
- `memory.suggest`, indirectly because it calls search.

The following path does not record access:

- `memory.search.count`

Access recording only updates active memories in the resolved project.

## Public Versus Internal Fields

Internal search can use enrichment fields.

Normal MCP responses should expose only public memory fields.

This separation is deliberate:

- Enrichment is retrieval machinery.
- `content` is the source of truth.
- Public responses stay small and stable.
- Debug information is available through explicit debug tooling.

## Invariants To Preserve

Always resolve the project before memory operations.

Always filter memory reads by `project_id`.

Always filter search by both `project_key` and `project_id` unless replacing that design with an equally strict project-isolation mechanism.

Always keep `memories`, `memory_enrichments`, and `memory_fts` synchronized during upsert.

Never replace original `content` with a summary.

Never expose enrichment fields in normal MCP responses unless changing the product contract intentionally.

Never return soft-deleted memories through get, list, or search.

Never write normal logs to stdout in MCP stdio mode.

## Quick Example

A memory may contain content saying:

```text
Routes are registered in internal/mcpserver/tools.go.
```

The enrichment may add:

```text
search_phrases: ["where are MCP tools registered?", "route registration", "tool schemas"]
aliases: ["endpoint", "handler"]
```

A later search for `where are tool handlers added` may match through enriched fields even if the exact words are not present in `content`.

The normal search result will still return the public memory object. If `include_content` is true, it returns the original content, not the generated search phrases.
