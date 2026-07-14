# Project Overview

## Purpose

This document captures the project identity and operating model future agents need before changing code.

Read this before making product-level, architecture-level, or cross-package changes.

## What This Project Does

The server provides MCP tools that let AI clients create, update, search, retrieve, list, suggest, and soft-delete technical memories.

Every memory belongs to exactly one project. The MCP client must pass a project key for memory operations, and the service resolves that key to a persistent project row.

## Product Constraints

This is a local-first internal/personal tool. It should remain easy to run as a single binary and should not require external services for the default path.

The useful unit is project-specific technical context, not human notes. Memory should help agents answer questions like how routes work, where API calls are implemented, which conventions matter, and what decisions were made.

## Runtime Model

The process runs as an MCP stdio server. Codex or another MCP client starts the binary and communicates over stdin/stdout.

Because stdout is protocol transport, application diagnostics should go to stderr or structured logging that does not corrupt stdio.

## Core Data Model

Projects have a stable `key`, human `name`, and optional `description`.

Memories store original project-scoped facts and usage metadata:

- `type`
- `title`
- `content`
- `importance`
- `source`
- `access_count`
- `last_accessed_at`
- timestamps and `deleted_at`

Retrieval metadata is derived and stored separately in `memory_enrichments`:

- `summary`
- `short_summary`
- `keywords_json`
- `tags_json`
- `entities_json`
- `aliases_json`
- `search_phrases_json`

## Extension Direction

The current search path is FTS5. Embedding and vector search interfaces already exist to support a later hybrid search implementation without changing MCP tool contracts.

## Related Documents

Also read:

- `repository-map.md`
- `mcp-tools-guide.md`
- `storage-search-guide.md`
- `enrichment-guide.md`
