# Critical Rules

## Purpose

This document lists project-wide constraints future agents must preserve.

Read this before any change that touches persistence, search, MCP protocol behavior, or project isolation.

## Rules That Must Hold

Every memory operation must be project-scoped. `project` is required at the MCP boundary and must resolve to a project before touching memory data.

Search must never return memories from another project. Keep both `project_key` and `project_id` filters in the search path unless replacing them with an equally strict design.

Soft delete is the v1 deletion behavior. Do not physically delete memories unless the user explicitly changes the product requirement.

Original `content` must always be preserved. Enrichment fields can improve retrieval but must not replace source content.

Memory upsert must keep original memory data, enrichment data, and FTS indexing synchronized. Writes currently happen inside a transaction.

`memories` and `memory_enrichments` are authoritative; `memory_fts` is derived state. An FTS rebuild may regenerate only that index from active authoritative rows.

The default path must work locally without API keys. The heuristic enricher is the fallback and should remain available.

The server speaks MCP over stdio. Do not write normal logs to stdout because stdout is protocol transport.

`MEMORY_DB_PATH` must be explicit and absolute. Never run backup, restore, migration, or rebuild while its MCP server can be running; the operation lock enforces this exclusion. Read-only inspection is the sole zero-write exception and must not create or reserve the store.

Existing stores must be classified before writes. Unknown, partial, future, version-gapped, or checksum-drifted schemas must fail closed. A recognized pre-ledger store is backed up before startup adopts it.

Backup and restore are explicit offline operations. Backup must not apply runtime migrations. Restore verifies SQLite integrity and the snapshot manifest, quarantines the original store, and does not silently overwrite an active database.

Tracing must require explicit payload consent. Keep `off` as the default; `metadata` is a closed non-sensitive allowlist, while `raw` is an explicit confidentiality decision. Enabled trace delivery is loopback-only, asynchronous, non-blocking, fail-silent, and must not follow redirects. Queue pressure may drop trace events but never MCP work; graceful shutdown must drain accepted events and report the cumulative drop count through payload-free health metadata.

Memory read paths that return memories to clients should update exposure metrics. They show what the server returned, not whether a client used or valued a memory.

Normal full-memory responses should not leak internal retrieval machinery. Compact search cards may expose only the bounded `description`, selected `match_excerpt`, and `match_field` needed for candidate selection. Keep raw summaries, keywords, entities, aliases, search phrases, scores, and match explanations internal unless using the explicit debug tool. Never present card guidance as authoritative memory content.

## Do Not Do Without Explicit Request

- Redesign the application into microservices.
- Replace SQLite as the primary local store.
- Remove FTS5 without an equivalent local search replacement.
- Make OpenAI or Ollama required for basic operation.
- Introduce cross-project search by default.
- Add background workers or daemon processes for v1 behavior unless explicitly requested.
- Treat `data/memory.db` as source-controlled project knowledge.
- Use `data/memory.db` in automated tests, fixtures, smoke checks, migration rehearsal, or retrieval evaluation.

## Sensitive Areas

Persistence and search are tightly coupled through `memory_fts`.

Project isolation spans MCP handlers, project service resolution, memory repository filters, and search filters. A change in one layer can break the invariant.

Enrichment output affects search quality because indexed FTS fields include summary, keywords, tags, entities, aliases, search phrases, and content.

## If Documentation Conflicts With Code

Trust code first. Then update `docs/ai/` so future agents do not inherit stale operational memory.
