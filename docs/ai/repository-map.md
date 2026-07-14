# Repository Map

## Purpose

This document tells future agents where to inspect code depending on the task.

Do not use it as a substitute for reading nearby code before editing.

## Main Code Locations

For process startup:

- `cmd/memory-mcp/main.go`
- `internal/app/app.go`
- `internal/app/config.go`

For MCP tool behavior:

- `internal/mcpserver/server.go`
- `internal/mcpserver/tools.go`
- `internal/mcpserver/schemas.go`

For project operations:

- `internal/project/model.go`
- `internal/project/service.go`
- `internal/project/repository.go`

For memory operations:

- `internal/memory/model.go`
- `internal/memory/validation.go`
- `internal/memory/service.go`
- `internal/memory/repository.go`

For search:

- `internal/search/model.go`
- `internal/search/fts.go`
- `internal/search/hybrid.go`

For enrichment:

- `internal/enrich/model.go`
- `internal/enrich/heuristic.go`
- `internal/enrich/openai_compatible.go`
- `internal/enrich/prompt.go`

For embeddings:

- `internal/embedding/embedder.go`
- `internal/embedding/noop.go`
- `internal/embedding/ollama.go`

For redistributable Codex skills:

- `skills/project-memory-mcp/SKILL.md`
- `skills/project-memory-mcp/references/tools.md`

For SQLite:

- `internal/storage/sqlite.go`
- `internal/storage/migrations.go`

For tests:

- `internal/enrich/heuristic_test.go`
- `internal/memory/memory_integration_test.go`

## Generated Or Runtime Files

- `data/memory.db` is runtime state and should not be treated as source.
- `memory-mcp` is a local build artifact and should not be edited.

## Before Modifying Also Inspect

Before changing MCP schemas, inspect both `internal/mcpserver/schemas.go` and `internal/mcpserver/tools.go`.

Before changing memory persistence, inspect `internal/storage/migrations.go`, `internal/memory/repository.go`, and `internal/search/fts.go`.

Before changing enrichment output, inspect `internal/enrich/model.go`, `internal/enrich/heuristic.go`, `internal/enrich/prompt.go`, and search indexing.

## Related Documents

Also read:

- `development-guide.md`
- `critical-rules.md`
