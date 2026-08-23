# MCP IA Memory

Local-first MCP server for project-specific technical memory.

It stores memories per project, enriches them for AI retrieval, and searches them with SQLite FTS5. The goal is to help coding agents preserve architecture, decisions, APIs, conventions, bugs, workflows, and technical context across sessions.

## Run

```bash
MEMORY_DB_PATH=/absolute/path/to/memory.db go run ./cmd/memory-mcp
```

The process environment is the only runtime configuration authority. The database path is required and must be absolute; the binary does not load `.env` files.

The server uses MCP over stdio. For desktop MCP clients, build the binary and adapt `examples/claude_desktop_config.json`.

## Trace Monitor

The MCP server can report tool calls to an optional local monitor without changing the configured MCP command. Codex and other clients should continue to run `memory-mcp` directly, but tracing must be enabled explicitly by adding `MEMORY_TRACE_MODE` to that process's environment.

Tracing is off by default. Start the monitor when you want to inspect explicitly enabled trace events:

```bash
go run ./cmd/memory-monitor
```

Then open:

```text
http://127.0.0.1:7379
```

If the monitor is not running, the MCP server ignores trace delivery failures and continues normally. The MCP stdio transport remains reserved for protocol messages; trace diagnostics never write to stdout from `memory-mcp`.

Trace configuration:

- `MEMORY_TRACE_MODE`: `off`, `metadata`, or `raw`; default `off`. An invalid value prevents startup.
- `MEMORY_TRACE_MONITOR_URL`: monitor event endpoint, default `http://127.0.0.1:7379/events`.
- `MEMORY_TRACE_BUFFER_SIZE`: async trace queue size, default `256`.
- `MEMORY_TRACE_DEBUG`: when `true`, trace delivery errors are written to stderr.

`metadata` sends only tool name, phase, duration, session identifier, process identifier, and a non-sensitive status classification. It never sends tool arguments, responses, search queries, project keys, memory content, or raw error text.

`raw` sends full tool arguments, responses, and errors. **Raw traces may contain confidential project data. Enable this mode only when that disclosure is intentional.**

Both enabled modes accept only loopback endpoints: the exact hostname `localhost` or a literal loopback IP address. Redirects and connections that resolve or dial outside loopback are rejected. Setting `MEMORY_TRACE_MONITOR_URL` alone does not enable tracing. Restart the MCP server or its client after changing trace environment variables.

## OpenCode

Build the MCP server binary first:

```bash
go build -o memory-mcp ./cmd/memory-mcp
go build -o memory-monitor ./cmd/memory-monitor
```

Then adapt `examples/opencode.json` with absolute paths for your machine. OpenCode config can live in a project config such as `./opencode.json` or `.opencode/opencode.json`, or in the global config at `~/.config/opencode/opencode.json`.

Minimal MCP entry:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "project-memory": {
      "type": "local",
      "command": ["/absolute/path/to/memory-mcp"],
      "enabled": true,
      "env": {
        "MEMORY_DB_PATH": "/absolute/path/to/memory.db",
        "MEMORY_ENRICHER": "heuristic"
      }
    }
  }
}
```

OpenCode loads config at startup, so quit and restart OpenCode after changing the MCP config.

## Tools

- `project.upsert`
- `project.list`
- `memory.upsert`
- `memory.search`
- `memory.search.count`
- `memory.search.debug`
- `memory.get`
- `memory.list`
- `memory.delete`
- `memory.suggest`

All memory tools require `project` to avoid cross-project context leakage.

## Offline maintenance

Stop MCP instances before maintenance. Create a verified snapshot or rebuild the derived FTS index from authoritative rows with an explicit absolute path:

```bash
go run ./cmd/memory-maintenance --operation inspect --db /absolute/path/to/memory.db
go run ./cmd/memory-maintenance --operation backup --db /absolute/path/to/memory.db
go run ./cmd/memory-maintenance --operation restore --db /absolute/path/to/memory.db --backup /absolute/path/to/snapshot.db
go run ./cmd/memory-maintenance --operation rebuild-fts --db /absolute/path/to/memory.db
```

## Documentation

AI-first operational memory lives in `docs/ai/`. Future coding agents should start with `docs/ai/README.md`.

## Redistributable Skill

The Codex skill for this MCP server lives in `skills/project-memory-mcp/` so it can be redistributed with the repository.
