# Project Overview

## Purpose

This document captures the project identity and operating model future agents need before changing code.

Read this before making product-level, architecture-level, or cross-package changes.

## What This Project Does

The server provides MCP tools that let AI clients create, update, search, retrieve, list, suggest, and soft-delete technical memories. The same executable also provides an offline terminal manager for local operators.

Every memory belongs to exactly one project. The MCP client must pass a project key for memory operations, and the service resolves that key to a persistent project row.

## Product Constraints

This is a local-first internal/personal tool. It should remain easy to run as a single binary and should not require external services for the default path.

The useful unit is project-specific technical context, not human notes. Memory should help agents answer questions like how routes work, where API calls are implemented, which conventions matter, and what decisions were made.

## Runtime Model

The process runs as an MCP stdio server. Codex or another MCP client starts the binary and communicates over stdin/stdout.

Because stdout is protocol transport, application diagnostics should go to stderr or structured logging that does not corrupt stdio.

The primary executable has two modes: `memory-mcp` reserves stdin/stdout for MCP, while `memory-mcp manage` takes the database operation lock exclusively and uses the terminal interactively. Multiple MCP processes may share a current store, but management cannot run against that store while any MCP process is active. Two optional companion commands remain outside the normal MCP process: `cmd/memory-monitor` displays explicitly enabled local trace events, and `cmd/memory-maintenance` performs low-level schema inspection, snapshot, restore, or FTS recovery. They must not become runtime dependencies of the MCP server.

## Core Data Model

Projects have a stable `key`, human `name`, optional `description`, and `deleted_at`. Deleted projects remain recoverable through the terminal manager but are excluded from normal MCP project resolution and memory operations.

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

## Store Continuity

`MEMORY_DB_PATH` is required and absolute for the MCP server. Management mode may instead receive an explicit absolute `--db` path. Startup classifies an existing SQLite store before writes, adopts a recognized unversioned current schema, migrates recognized older schemas, and records versioned schema state in `schema_migrations`. Unknown, partial, future, or checksum-inconsistent schemas fail closed.

Each server holds a shared operation lock for its database path and database identity. A current versioned store can therefore serve multiple stdio processes through its canonical path or symlink aliases. Runtime, management, and mutating maintenance opening reject a database with multiple hard links because SQLite journal recovery is pathname-dependent; identity locks still prevent hardlink aliases from bypassing active operations before the open is rejected. Schema initialization, adoption, or migration temporarily requires an exclusive reservation and fails closed while another runtime is active. Management, backup, restore, and FTS rebuild use the same exclusive reservation and require MCP instances to stop. Read-only schema inspection does not reserve or modify the store.

## Related Documents

Also read:

- `repository-map.md`
- `mcp-tools-guide.md`
- `storage-search-guide.md`
- `enrichment-guide.md`
