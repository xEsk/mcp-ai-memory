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

The default path must work locally without API keys. The heuristic enricher is the fallback and should remain available.

The server speaks MCP over stdio. Do not write normal logs to stdout because stdout is protocol transport.

Tracing must require explicit payload consent. Keep `off` as the default; `metadata` is a closed non-sensitive allowlist, while `raw` is an explicit confidentiality decision. Enabled trace delivery is loopback-only, asynchronous, non-blocking, fail-silent, and must not follow redirects.

Memory read paths that return memories to clients should update usage metrics. This lets future agents identify whether stored memories are actually used.

Normal MCP responses should not leak internal retrieval machinery. Keep summaries, keywords, entities, aliases, search phrases, snippets, scores, and match explanations internal unless adding an explicit debug tool.

## Do Not Do Without Explicit Request

- Redesign the application into microservices.
- Replace SQLite as the primary local store.
- Remove FTS5 without an equivalent local search replacement.
- Make OpenAI or Ollama required for basic operation.
- Introduce cross-project search by default.
- Add background workers or daemon processes for v1 behavior unless explicitly requested.
- Treat `data/memory.db` as source-controlled project knowledge.

## Sensitive Areas

Persistence and search are tightly coupled through `memory_fts`.

Project isolation spans MCP handlers, project service resolution, memory repository filters, and search filters. A change in one layer can break the invariant.

Enrichment output affects search quality because indexed FTS fields include summary, keywords, tags, entities, aliases, search phrases, and content.

## If Documentation Conflicts With Code

Trust code first. Then update `docs/ai/` so future agents do not inherit stale operational memory.
