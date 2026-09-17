# MCP IA Memory

Local-first MCP server for project-specific technical memory.

It stores memories per project, enriches them for AI retrieval, and searches them with SQLite FTS5. The goal is to help coding agents preserve architecture, decisions, APIs, conventions, bugs, workflows, and technical context across sessions.

## Run

```bash
MEMORY_DB_PATH=/absolute/path/to/memory.db go run ./cmd/memory-mcp
```

The process environment is the MCP server's runtime configuration authority. The database path is required and must be absolute; the binary does not load `.env` files.

The server uses MCP over stdio. For desktop MCP clients, build the binary and adapt `examples/claude_desktop_config.json`. Multiple MCP processes may share the same current database through its canonical path or symlink aliases; SQLite serializes normal writes with a bounded busy timeout. Runtime, management, and mutating maintenance opening reject a database with multiple hard links because SQLite recovery files are pathname-dependent. If startup must initialize or migrate the schema, every other MCP process for that database must stop first.

## Terminal manager

The same binary includes an offline terminal interface for inspecting and managing the memory store. Stop every MCP instance using the database, then run:

```bash
MEMORY_DB_PATH=/absolute/path/to/memory.db memory-mcp manage
```

An explicit path can override the environment for this mode:

```bash
memory-mcp manage --db /absolute/path/to/memory.db
```

The manager lists projects and statistics, browses memories from compact summary to full detail, soft-deletes and restores projects, and previews permanent cleanup. Project deletion requires typing the exact project key. `Clean junk` requires typing `CLEAN` and permanently purges deleted projects, soft-deleted memories, and stale derived search/enrichment rows in one transaction. This cleanup is logical database cleanup, not forensic secure erasure.

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

Trace production remains non-blocking when the local queue is full. Dropped events are counted, and graceful server shutdown drains queued events and sends a payload-free `trace.queue` health event to the monitor when any drops occurred. The monitor keeps the cumulative drop count visible in its connection status.

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

Use `memory.search` to obtain compact candidate cards, select plausible IDs, and call `memory.get` before relying on remembered facts or making code changes. Card descriptions and match excerpts are navigation aids, not authoritative memory content. Set `include_content: true` on search only when immediate bulk reading is justified.

## Offline maintenance

The interactive manager is part of `memory-mcp`. The separate low-level `memory-maintenance` command remains for recovery operations. `inspect` is read-only and does not reserve or modify the store. Stop every MCP instance before `backup`, `restore`, or `rebuild-fts`; those operations reserve the database path exclusively. Use an explicit absolute path:

```bash
go run ./cmd/memory-maintenance --operation inspect --db /absolute/path/to/memory.db
go run ./cmd/memory-maintenance --operation backup --db /absolute/path/to/memory.db
go run ./cmd/memory-maintenance --operation restore --db /absolute/path/to/memory.db --backup /absolute/path/to/snapshot.db
go run ./cmd/memory-maintenance --operation rebuild-fts --db /absolute/path/to/memory.db
```

Backup and rebuild require an existing recognized store and never apply runtime migrations as a side effect. A backup includes a `.manifest` with its filename, size, SHA-256 checksum, and creation time. Restore rejects a missing or mismatched manifest before quarantining the current store.

## Documentation

AI-first operational memory lives in `docs/ai/`. Future coding agents should start with `docs/ai/README.md`.

## Redistributable Skill

The Codex skill for this MCP server lives in `skills/project-memory-mcp/` so it can be redistributed with the repository.
