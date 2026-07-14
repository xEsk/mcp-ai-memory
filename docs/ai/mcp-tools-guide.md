# MCP Tools Guide

## Purpose

This document explains how MCP tools are exposed and how to extend them safely.

Read this before changing tool schemas, tool names, or handler behavior.

## How This Area Works

`internal/mcpserver/server.go` creates an MCP server with tool capabilities. `internal/mcpserver/tools.go` registers tools using `mcp.NewTool` and handles calls with typed input structs from `schemas.go`.

Handlers bind JSON arguments with `request.BindArguments`, validate required fields, resolve project ownership where needed, call services, and return structured MCP results.

Public memory responses are intentionally slim. Enrichment fields are internal retrieval machinery and should not be returned to agents unless a diagnostic/debug tool is explicitly added.

## Current Tools

- `project.upsert`: create or update a project by key.
- `project.list`: list projects.
- `memory.upsert`: create or update a memory and refresh search metadata/index.
- `memory.search`: search active memories inside one project with optional `mode`.
- `memory.search.count`: count matching active memories without returning memories or recording access.
- `memory.search.debug`: inspect ranking details, snippets, scores, and match explanations.
- `memory.get`: retrieve one active memory by project and ID.
- `memory.list`: list active memories with optional type/tag filters.
- `memory.delete`: soft-delete one memory.
- `memory.suggest`: search similar memories and return a non-mutating suggestion.
- `memory.server.info`: return the MCP server name and version.

## How To Extend Existing Code

When adding a tool:

1. Add a small input struct in `schemas.go`.
2. Register schema, descriptions, and required fields in `registerTools`.
3. Implement a handler that delegates to services.
4. Return tool errors with `mcp.NewToolResultError` for user input/domain failures.
5. Return Go errors only for unexpected technical failures.

## Existing Conventions

- Memory tools require `project`.
- `memory.server.info` intentionally returns only basic identity fields: `name` and `version`.
- `memory.upsert` defaults `enrich` to true if the argument is omitted.
- `memory.search` defaults to `precise` mode. Clients can request `balanced` or `broad` when they want wider recall.
- `memory.suggest` must not mutate memory in v1. It searches with `precise` first, then retries with `balanced` only when precise finds no candidates. Its response includes `search_mode` so clients can tell which mode produced the candidates, or that both modes found none.
- `memory.suggest` candidates are ranked by relevance to the suggestion input, then passed through a local semantic quality gate. `recommended_candidate_id` is set only when the best candidate is strong enough for `update_existing`. Candidate `memory.importance` remains global memory importance, not match quality. Use `candidate_quality` for structured match quality and `next_step` for compact agent workflow guidance.
- Tool responses use `mcp.NewToolResultStructuredOnly` for structured data.
- Memory responses should expose the saved memory core, not generated retrieval fields. Use `internal/mcpserver/responses.go` for response shaping.

## Common Mistakes To Avoid

- Do not implement persistence directly in MCP handlers.
- Do not silently accept missing project keys.
- Do not rename tools casually; clients may depend on names.
- Do not expose deleted memories through get/list/search.
- Do not expose `summary`, `keywords`, `entities`, `aliases`, `search_phrases`, snippets, ranking scores, or debug match explanations in normal memory responses. Use `memory.search.debug` for diagnostic ranking data.

## Validation

Run `go test ./...` after handler/service changes.

[INFERRED] Full MCP client integration testing currently depends on launching the binary from a configured client such as Codex.

## Related Documents

Also read:

- `development-guide.md`
- `storage-search-guide.md`
