# AI Project Knowledge Base

## What This Project Is

This project is a local-first MCP server for project-specific AI memory. It stores technical memory per project so coding agents can retrieve architecture, decisions, APIs, conventions, bugs, workflows, domain terms, and implementation context across sessions.

The server is not a general note-taking application. It is optimized for AI retrieval and project continuity.

## Stack

- Go 1.26.3.
- `github.com/mark3labs/mcp-go` for MCP stdio server behavior.
- `modernc.org/sqlite` for local SQLite with FTS5.
- SQLite FTS5 for contextual full-text search.
- Optional OpenAI-compatible chat completion provider for memory enrichment.
- Optional Ollama-compatible embedding client interface for future vector search.
- Optional local trace monitor for real-time MCP tool-call observability.

## Architecture Snapshot

The application is a single binary MCP server. `cmd/memory-mcp` wires startup, `internal/app` builds services, and `internal/mcpserver` exposes MCP tools.

Project isolation is a core invariant. Memory operations resolve the project key first and then use `project_id`/`project_key` filters in persistence and search paths.

Access metrics are stored on each memory so agents can tell whether retrieved memories are being used.

## Repository Structure

- `cmd/memory-mcp/`: process entrypoint.
- `internal/app/`: configuration and dependency wiring.
- `internal/mcpserver/`: MCP tool schemas and handlers.
- `internal/project/`: project model, service, and SQLite repository.
- `internal/memory/`: memory model, validation, service, and SQLite repository.
- `internal/search/`: FTS5 search and hybrid-search extension point.
- `internal/enrich/`: heuristic and OpenAI-compatible enrichment.
- `internal/embedding/`: embedding interfaces and Ollama client.
- `internal/storage/`: SQLite connection and schema initialization.
- `examples/`: MCP client config and sample payloads.
- `docs/ai/`: operational memory for future agents.

## Development Philosophy

Keep the server local-first and simple. Prefer explicit services, repositories, and SQL over broad frameworks.

New features should extend existing package boundaries before introducing new layers. Preserve the single-binary shape unless the user explicitly asks for a different runtime model.

## Current Plans

- [Memory Trust Roadmap](../plans/2026-07-14-001-memory-trust-roadmap/README.md): three ordered specifications covering P0 correctness/privacy, P1 continuity/operability/evidence, and P2 retrieval quality/debuggability. Phase 1 implementation and completion proof are complete; Phase 2 has not started and remains gated on explicit owner acceptance of Phase 1.

## Choose Documentation By Task

Need MCP tool changes:

→ `mcp-tools-guide.md`

Need database or search changes:

→ `storage-search-guide.md`

Need exact internal memory lifecycle details for storing, enrichment, FTS indexing, search, responses, access metrics, suggestions, or deletion:

→ `memory-lifecycle-internals.md`

Need enrichment, keywords, summaries, or embeddings:

→ `enrichment-guide.md`

Need repository navigation:

→ `repository-map.md`

Need test/build commands:

→ `development-guide.md`

Need safety constraints:

→ `critical-rules.md`

Need previous decisions:

→ `historical-decisions.md`

## Global Rules

- Always preserve project-scoped memory isolation.
- Use `mcp-ai-memory` as the canonical Project Memory MCP key for this repository.
- Never physically delete memories in v1 behavior; use soft delete.
- Do not bypass enrichment/index synchronization on memory writes.
- Preserve memory access metrics when changing get/list/search behavior.
- Do not write logs or diagnostics to stdout in stdio MCP mode.
- Trace monitor delivery must remain asynchronous and fail-silent; monitor failures must never affect MCP behavior.
- Keep generated operational memory in English.

## Warning

Do not load all documents by default. Open only the files relevant to the current task, then inspect code to verify details before editing.
