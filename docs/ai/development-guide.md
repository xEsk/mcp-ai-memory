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
go build ./cmd/memory-monitor
go build ./cmd/memory-maintenance
```

Make targets cover the MCP server and monitor:

```bash
make test
make build
make build-monitor
make run
make monitor
make tidy
```

## Configuration

Runtime config comes from environment variables:

- `MEMORY_DB_PATH`: required absolute SQLite path. The binary does not load `.env` files.
- `MEMORY_ENRICHER`: `heuristic`, `openai-compatible`, or `openai`.
- `MEMORY_LLM_BASE_URL`: OpenAI-compatible base URL.
- `MEMORY_LLM_API_KEY`: provider token.
- `MEMORY_LLM_MODEL`: chat model.
- `MEMORY_EMBEDDINGS_ENABLED`: currently reserved for future hybrid search wiring.
- `MEMORY_EMBEDDING_BASE_URL`: embedding provider base URL.
- `MEMORY_EMBEDDING_MODEL`: embedding model.
- `MEMORY_TRACE_MODE`: `off`, `metadata`, or `raw`; default `off`.
- `MEMORY_TRACE_MONITOR_URL`: local trace monitor endpoint, default `http://127.0.0.1:7379/events`. A URL alone does not enable tracing.
- `MEMORY_TRACE_BUFFER_SIZE`: non-blocking trace queue size, default `256`.
- `MEMORY_TRACE_DEBUG`: write trace delivery errors to stderr when `true`, default `false`.

## Trace Monitor

The MCP server always keeps stdio reserved for protocol traffic. Tracing is off by default and is delivered through `internal/trace` to an optional local monitor over HTTP only after `MEMORY_TRACE_MODE` is explicitly set to `metadata` or `raw`. Delivery is asynchronous and fail-silent: if the monitor is closed, unreachable, slow, or returns an error, MCP behavior must remain unchanged. A full queue drops new trace events without blocking MCP calls and increments a concurrency-safe counter. Graceful application shutdown drains accepted events and, when drops occurred, emits one payload-free `trace.queue` health event whose `dropped_events` value is rendered by the monitor.

Metadata mode emits only the documented operational allowlist (`session_id`, `pid`, `tool`, `phase`, `duration_ms`, `status`, and `dropped_events`) and excludes arguments, responses, queries, project keys, memory content, and raw errors. Raw mode emits full debugging payloads and must be treated as confidential. Both enabled modes require an exact `localhost` or literal loopback-IP endpoint, reject redirects, and verify the connected address remains loopback. Restart the MCP server or its client after changing trace configuration.

Run the monitor manually when needed:

```bash
go run ./cmd/memory-monitor
```

Then open `http://127.0.0.1:7379`. MCP clients such as Codex should continue to execute `memory-mcp` directly; do not configure clients to execute the monitor instead.

## Safe Database Verification

Every storage test must open a newly created database beneath that test's `t.TempDir()`. Never use, copy, hash, or open `data/memory.db` or its sidecars in automated tests or smoke commands.

Run repository-wide tests through the filesystem-metadata guard:

```bash
./scripts/verify-real-db-untouched.sh -- go test -race -count=1 ./...
```

`make verify` is the canonical local and CI entry point. It tests the guard itself, runs the guarded race suite, builds all three commands, checks Go formatting and diff whitespace, lists the expected temporary test stores, and fails if test source names the protected real-store path.

The guard rejects normalized or embedded command arguments that name the real database, refuses `sh`, `bash`, and `zsh` indirection, and compares only entry existence, inode, size, permissions, and timestamps for the database and sidecars before and after the command. It is a metadata-change tripwire, not an operating-system audit of arbitrary executable reads. Completion proof must pair it with source inspection showing that every test database uses `t.TempDir()` and no test source names `data/memory.db`.

Use these source checks when closing a phase:

```bash
rg -n 'memory\.db' --glob '*_test.go' .
rg -n 'data/memory\.db' --glob '*_test.go' .
```

Inspect every path listed by the first command and confirm it is created below `t.TempDir()`; relative-path validation fixtures must fail before storage access. The second command must exit `1` with no matches.

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

## Related Documents

Also read:

- `mcp-tools-guide.md`
- `storage-search-guide.md`
- `enrichment-guide.md`
