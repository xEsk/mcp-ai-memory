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
- `memory.search.debug`: inspect SQLite-backed matched fields, selected highlighted snippets, scores, and evidence-derived explanations.
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
- Search, count, debug, and suggest return a tool error for normalized empty queries, including punctuation-only and stopword-only input.
- `memory.suggest` must not mutate memory content in v1. It searches with `precise` first, then retries with `balanced` when precise has no safe recommendation. Its response includes `search_mode` so clients can tell whether broadening was required.
- `memory.suggest` candidates are ranked by relevance to the suggestion input, then passed through a local semantic quality gate. `recommended_candidate_id` is set only when the best candidate is strong enough for `update_existing`. Candidate `memory.importance` remains global memory importance, not match quality. Use `candidate_quality` for structured match quality and `next_step` for compact agent workflow guidance.
- Successful structured handlers use `mcp.NewToolResultStructuredOnly`. In the pinned `mcp-go` version this produces both `structuredContent` and an equivalent JSON `TextContent` fallback in required `content`.
- Memory responses should expose the saved memory core, not generated retrieval fields. Use `internal/mcpserver/responses.go` for response shaping.
- Debug results add `matched_fields`, `snippet_field`, `snippet`, `score`, and `match_explanation`. These fields remain exclusive to `memory.search.debug`.

## Common Mistakes To Avoid

- Do not implement persistence directly in MCP handlers.
- Do not silently accept missing project keys.
- Do not rename tools casually; clients may depend on names.
- Do not expose deleted memories through get/list/search.
- Do not expose `summary`, `keywords`, `entities`, `aliases`, `search_phrases`, snippets, ranking scores, or debug match explanations in normal memory responses. Use `memory.search.debug` for diagnostic ranking data.
- Do not remove `content`: it is required by the MCP `CallToolResult` schema and is still the only model-visible result path in some clients.
- Do not strip `structuredContent` merely to reduce Codex context. Codex direct calls select one representation before model context, while typed clients use structured data without reparsing JSON text.
- Do not add an MCP `outputSchema` until the server can declare and maintain accurate schemas for every successful structured result. Once declared, successful `structuredContent` must conform and clients should validate it; tool errors remain text in `content` with `isError: true`.

## Server, Client, And Model Boundaries

The server owns the MCP wire result. It sends `content`, optional `structuredContent`, and `isError`; keeping equivalent text and structured forms follows MCP compatibility guidance and does not by itself prove duplicate model input.

The client owns storage and projection of that result. The installed Codex is 0.154.0; the nearest published OpenAI source tag (`rust-v0.154.0-alpha.9`) documents the direct-call path, and live 0.154.0 `functions.exec` calls confirm the orchestration path:

- A direct MCP tool call uses non-null `structuredContent` when present and otherwise uses `content`. If both exist, it does not add both to model context.
- A tool called inside `functions.exec` returns the complete `CallToolResult` object to JavaScript, except `_meta`. Nested tool results do not enter parent context automatically.
- `text(result)` JSON-stringifies the whole object and therefore duplicates equivalent payloads. `text(result.structuredContent)` emits only structured data. Extracting and emitting `content[].text` emits only the fallback.
- The redistributable Project Memory skill makes the choice explicit: preserve `content` error text when `isError` is true; otherwise prefer non-null `structuredContent` and fall back to text content. It forbids emitting the complete wrapper.

This policy preserves both wire representations because current clients differ. VS Code currently prefers `structuredContent` for model text when present. Reported Claude Code 2.1.126 behavior also prefers it, while a version-specific image issue shows that this precedence can hide non-text blocks. Cursor reports from 2026 conflict by surface and version: model chat can receive structured data, hooks may omit it, and structured-only results have been reported empty. OpenCode has a current report that model projection uses `content` and drops `structuredContent`. These client observations justify the compatible dual wire response; they do not imply that every client inserts both copies into its model context.

Character measurements must distinguish the entire JSON-RPC wire response from the selected value emitted to model context. Token counts are estimates unless obtained with the exact target model tokenizer. Local evidence uses the explicitly labelled approximation `ceil(UTF-8 characters / 4)`.

## Validation

Run `make verify` after handler, response, or redistributable skill changes. The skill regression check guards the first duplication boundary by rejecting whole-result emission and requiring success, fallback, and error branches.

[LIMITATION] The `functions.exec` projection is confirmed on the installed Codex 0.154.0. The direct-call conclusion comes from the nearest published OpenAI source tag (`rust-v0.154.0-alpha.9`), not a captured final prompt from this desktop host. Other hosts and future versions may project MCP results differently. `functions.exec` only exposes what its JavaScript emits, so server-side wire size and parent-context size remain separate measurements. The pinned `mcp-go` 0.54.1 negotiates legacy MCP through 2025-11-25; the 2026-07-28 specification was used to check current field requirements, not to claim modern-era transport support.

## Related Documents

Also read:

- `development-guide.md`
- `storage-search-guide.md`
