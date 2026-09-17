# Terminal Memory Management

Status: APPROVED
Level: CRITICAL

## Outcome

The existing `memory-mcp` executable gains an offline `manage` mode so a user can inspect and manage the configured memory database from an attractive, compact terminal interface without installing a second binary.

Running `memory-mcp` with no arguments preserves the current MCP stdio server behavior. Running `memory-mcp manage` opens the management interface. The database path comes from `MEMORY_DB_PATH`, with an optional absolute `--db` override for this mode.

## User and value

The caller is the local operator who owns the SQLite memory store. The interface makes project selection, basic statistics, summary-to-detail memory browsing, reversible project deletion, and deliberate permanent cleanup possible without SQL or an AI client.

Success is observable when the operator can complete those journeys in one terminal session and the resulting database state remains project-scoped, internally consistent, and compatible with normal MCP startup.

## In scope

- One binary with two mutually exclusive modes: existing stdio MCP service and offline terminal management.
- A standard-library ANSI terminal interface; no TUI framework or new runtime dependency.
- Project list and per-project statistics: active/deleted state, active/deleted/total memory counts, type distribution, total retrieval exposure, never-retrieved count, and latest memory update.
- Active memory browsing as compact summaries, followed by authoritative full detail for one selected memory.
- Project soft delete by setting `projects.deleted_at`; deleted projects disappear from normal MCP project resolution and all memory operations, while their data remains available for restoration from the management interface.
- Project restoration before permanent cleanup.
- A cleanup preview followed by an explicit typed confirmation. Cleanup permanently removes:
  - every soft-deleted project and all memories, enrichments, and FTS rows it owns;
  - soft-deleted memories that belong to active projects and their enrichments;
  - FTS rows that do not correspond to an active memory in an active project;
  - enrichment rows whose source memory no longer exists.
- Cleanup returns exact category counts. Authoritative and derived row deletion happens in one transaction.
- A schema migration adding project deletion state and its supporting index.

## Out of scope

- Running management concurrently with the MCP server.
- Arrow-key/raw-terminal navigation, mouse input, search, editing memory content, audit, repair, automatic retention rules, or deleting data based on age, importance, or retrieval count.
- Secure erasure of bytes from SQLite free pages, WAL files, snapshots, or filesystem storage; cleanup guarantees logical database consistency, not forensic erasure.
- Changing the existing `memory.delete` soft-delete contract or adding new MCP administration tools.

## Behavior and reuse

- `cmd/memory-mcp` dispatches to `manage` only when that explicit subcommand is present. Unknown arguments fail on stderr without opening the database.
- Management takes the existing exclusive maintenance operation lock before opening the store and refuses to run while the MCP server or another maintenance operation owns it.
- Storage opening and schema migration continue through the existing storage boundary. Project lifecycle and aggregate queries remain owned by `internal/project`; normal memory detail/list behavior remains owned by `internal/memory`.
- Normal `project.list`, project resolution, upsert, and all memory tools see active projects only. Upserting a soft-deleted key must not silently restore it.
- The interface accepts line-oriented choices and renders headings, panels, status markers, and concise help with ANSI styling when enabled. EOF and `q` exit cleanly.
- Destructive confirmation requires typing the exact project key for soft deletion and the literal `CLEAN` for permanent cleanup. Any other input cancels without writes.

## Failure and recovery

- Project deletion and restoration update one row atomically and report not-found/already-state errors without partial mutation.
- Permanent cleanup rolls back all authoritative and derived row deletion if any statement or commit fails.
- A failed or interrupted migration follows the existing transactional migration behavior. Existing recognized stores are backed up before migration by the normal storage-opening path.
- A cleanup committed successfully is irreversible through this interface; recovery requires an existing database backup.

## Stop gates

- Do not open the management store without the maintenance lock.
- Do not accept a relative or empty database path.
- Do not expose memories from a deleted project through normal MCP services.
- Do not commit cleanup if related authoritative or derived rows remain inside its declared scope.
- Do not treat cleanup as secure erasure.

## Acceptance and proof

1. A migration test upgrades an existing v1 store without losing project or memory data and classifies the resulting schema as current.
2. Project integration tests prove active-only resolution/listing, explicit soft delete, restoration, and no accidental restoration through upsert.
3. Cleanup integration tests use a temporary database and prove exact deletion of deleted projects, their active and deleted memories, soft-deleted memories of active projects, enrichments, and stale FTS rows while preserving active project data. A forced cleanup failure proves rollback.
4. CLI tests prove no-argument server dispatch remains distinct, invalid arguments fail before storage access, management requires an absolute database path, confirmation cancellation is non-mutating, and the summary-to-detail journey renders expected data.
5. The management mode build and focused tests pass, followed by the repository-required `make verify` sequence against the final implementation.
6. An independent critical review covers the migration, project visibility boundary, destructive transaction, confirmations, and final diff. Any unresolved correctness or data-loss finding blocks completion.

## Documentation impact

Update the root README and affected `docs/ai` guides so command usage, schema, project visibility, cleanup semantics, locking, and verification guidance match the implementation.
