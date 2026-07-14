# Development Guide

## Purpose

This document explains how to make code changes while preserving the project shape.

Read this before implementing features, tests, or local verification.

## How Development Works

The project uses a simple Go package layout. `main.go` stays minimal, `internal/app` wires dependencies, services coordinate behavior, and repositories own SQLite access.

Avoid adding frameworks unless they remove real complexity. Manual constructor wiring is the current pattern.

## Commands

Use:

```bash
go test ./...
go build ./cmd/memory-mcp
```

Make targets mirror these commands:

```bash
make test
make build
make run
make tidy
```

## Configuration

Runtime config comes from environment variables:

- `MEMORY_DB_PATH`: SQLite path, default `./data/memory.db`.
- `MEMORY_ENRICHER`: `heuristic`, `openai-compatible`, or `openai`.
- `MEMORY_LLM_BASE_URL`: OpenAI-compatible base URL.
- `MEMORY_LLM_API_KEY`: provider token.
- `MEMORY_LLM_MODEL`: chat model.
- `MEMORY_EMBEDDINGS_ENABLED`: currently reserved for future hybrid search wiring.
- `MEMORY_EMBEDDING_BASE_URL`: embedding provider base URL.
- `MEMORY_EMBEDDING_MODEL`: embedding model.
- `MEMORY_TRACE_MONITOR_URL`: optional local trace monitor endpoint, default `http://127.0.0.1:7379/events`.
- `MEMORY_TRACE_BUFFER_SIZE`: non-blocking trace queue size, default `256`.
- `MEMORY_TRACE_DEBUG`: write trace delivery errors to stderr when `true`, default `false`.

## Trace Monitor

The MCP server always keeps stdio reserved for protocol traffic. Tool-call tracing is delivered through `internal/trace` to an optional local monitor over HTTP. Delivery is asynchronous and fail-silent: if the monitor is closed, unreachable, slow, or returns an error, MCP behavior must remain unchanged.

Run the monitor manually when needed:

```bash
go run ./cmd/memory-monitor
```

Then open `http://127.0.0.1:7379`. MCP clients such as Codex should continue to execute `memory-mcp` directly; do not configure clients to execute the monitor instead.

MCP client config examples live in `examples/`:

- `examples/claude_desktop_config.json` for Claude Desktop.
- `examples/opencode.json` for OpenCode. The OpenCode MCP entry must use `type: "local"`, a `command` array with an absolute binary path, and absolute paths in environment variables such as `MEMORY_DB_PATH`.

## How To Extend Existing Code

When adding a new MCP tool:

1. Add or update input structs in `internal/mcpserver/schemas.go`.
2. Register the tool schema in `internal/mcpserver/tools.go`.
3. Add handler logic that delegates to a service.
4. Keep business behavior out of the handler when it belongs in `internal/memory`, `internal/project`, `internal/search`, or `internal/enrich`.
5. Add focused tests around the service/repository behavior.

When adding storage fields:

1. Update `internal/storage/migrations.go`.
2. Update model structs.
3. Update repository scan/write code.
4. Update FTS indexing if the field affects retrieval.
5. Add or update integration tests with temporary SQLite.

When adding a new read path that returns memories:

1. Decide whether returning the memory should count as access.
2. Increment `access_count` and set `last_accessed_at` for active returned memories.
3. Reflect the incremented values in the response when practical.
4. Add an integration test if the path is part of public MCP behavior.

## Existing Conventions

- Use `context.Context` for database and provider calls.
- Wrap returned errors with operation context.
- Keep repository SQL explicit and parameterized.
- Keep new methods documented with concise comments matching nearby style.
- Preserve original memory content; enrichment is additive metadata.

## Common Mistakes To Avoid

- Do not add generic note-taking features that weaken project-specific memory behavior.
- Do not write diagnostics to stdout in MCP stdio mode.
- Do not introduce an ORM.
- Do not make project optional for memory operations.
- Do not update memory rows without updating FTS when search-relevant fields change.
- Do not add memory-returning APIs that bypass access tracking.

## Validation

Run:

```bash
go test ./...
go build ./cmd/memory-mcp
go build ./cmd/memory-monitor
```

If build cache permissions fail in a sandboxed environment, request escalation rather than changing project paths.

## Related Documents

Also read:

- `mcp-tools-guide.md`
- `storage-search-guide.md`
- `enrichment-guide.md`
