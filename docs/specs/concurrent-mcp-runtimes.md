# Concurrent MCP Runtimes

Status: APPROVED
Level: CRITICAL

## Outcome

Multiple `memory-mcp` stdio processes may serve the same current SQLite store concurrently, so every open Codex task can initialize Project Memory and receive its MCP tools. Normal runtimes share the store reservation. Terminal management and mutating maintenance remain exclusive, and schema creation, adoption, or migration never runs while another runtime is active.

## User and value

The caller is a local MCP client, such as Codex, that starts one `memory-mcp` process per task. Today the first process holds an exclusive operation lock for its lifetime, so later tasks fail before MCP initialization and omit the Project Memory tools.

Success is observable when two or more clients can initialize against the same already-current database, read and write project-scoped memories, and shut down independently without making offline operations or schema changes unsafe.

## In scope

- Shared long-lived operation reservations for normal MCP runtimes.
- Exclusive operation reservations for `memory-mcp manage`, backup, restore, FTS rebuild, and any startup path that creates, adopts, or migrates the schema.
- The same shared/exclusive semantics for both the canonical-path lock and the device/inode identity lock. Symlink aliases converge on the canonical path. Runtime, management, and mutating maintenance opening fail closed when the database has multiple hard links because SQLite journal recovery is pathname-dependent; identity locks still prevent a hardlink alias from bypassing an already-active operation before the open is rejected.
- A fail-closed startup flow that distinguishes a current store, which may be opened under a shared reservation, from a store that requires mutation, which requires an exclusive reservation.
- Actionable startup and maintenance errors for reservation conflicts and migration-required conflicts.
- Multiprocess regression coverage for runtime coexistence, exclusive operations, aliases, and migration safety.

## Out of scope

- A daemon, shared network service, proxy process, or transport other than the existing MCP stdio transport.
- Replacing SQLite, changing MCP tool schemas, changing project isolation, or changing memory lifecycle behavior.
- Adding WAL mode, a write queue, distributed locking, automatic process termination, or an unbounded startup retry loop.
- Supporting MCP runtime, management, or mutating maintenance access through hardlink aliases of the database.
- Guaranteeing cross-version runtime compatibility beyond refusing to mutate a schema while another runtime is active.
- Running terminal management or mutating maintenance concurrently with any MCP runtime.
- Making simultaneous first-time bootstrap wait indefinitely. Contenders may fail with an actionable transient conflict while one process initializes the store.

## Existing behavior and reuse

- `internal/storage/operation_lock.go` owns canonical-path and database-identity reservations and remains the only owner of operation-lock semantics.
- `internal/app.New` owns MCP runtime startup and lifetime cleanup.
- `internal/app.NewManagement` and the maintenance entry points continue to acquire exclusive reservations before opening the store.
- `internal/storage.OpenSQLite` owns migration-capable opening, schema classification, backup-before-migration, and the existing transient migration lock.
- SQLite keeps one connection per process and a bounded busy timeout. Repository transactions and constraints remain responsible for normal write atomicity and conflict handling.
- The existing absolute-path, stale-lock-file, schema-classification, and fail-closed rules remain unchanged.

## Runtime flow

### Current store

1. Startup validates the configured absolute database path.
2. It acquires a shared operation reservation for the canonical database path and, when the file exists, its device/inode identity.
3. While that shared reservation is held, storage reclassifies the store and opens it only if it is already current and versioned. This path must not create, adopt, migrate, back up, restore, or rebuild schema state.
4. The shared reservation remains held until the database and MCP application close. Closing one runtime must not release or invalidate another runtime's reservation.

The classification performed while holding the shared reservation is authoritative. Any earlier unlocked classification may be used only to select a path and must not authorize opening or mutation.

### Store requiring initialization or migration

1. If the authoritative current-only open reports that schema mutation is required, startup releases its shared reservation without serving MCP requests.
2. It attempts to acquire an exclusive operation reservation using the same canonical-path and identity rules as maintenance.
3. Only while that exclusive reservation is held may the existing migration-capable opening path create, adopt, back up, or migrate the store. The existing migration lock continues to serialize migration internals; it does not replace the operation reservation.
4. After mutation completes, startup closes the migration-capable database handle, releases the exclusive reservation, and re-enters the current-store flow. It must reacquire a shared reservation and reclassify before serving.
5. If another runtime prevents exclusive acquisition, startup fails without schema writes and reports that all runtimes must stop before migration.

This two-stage flow permits ordinary concurrent startup without weakening migration exclusion or relying on an atomic shared-to-exclusive lock upgrade.

### Maintenance and management

Mutating maintenance and terminal management acquire exclusive canonical-path and identity reservations. They fail before opening the database when any shared runtime or another exclusive operation owns either reservation. Read-only `inspect` remains the existing zero-reservation exception.

## Lock contract

- Runtime mode is shared; maintenance/schema-mutation mode is exclusive.
- Acquisition remains non-blocking. A conflict returns promptly and never waits indefinitely.
- Canonical-path and device/inode locks use the same requested mode and a consistent acquisition order.
- Partial acquisition failure releases every reservation already acquired by that attempt.
- Binding the identity of a newly created database uses the mode of the owning reservation.
- Runtime acquisition and every mutating database-open path reject an existing database whose link count is greater than one. Maintenance still uses the identity reservation so a hardlink alias cannot bypass an active runtime before its own open is rejected.
- Close is idempotent at the application boundary and releases identity and path reservations owned by that instance only.
- Lock files may remain on disk after close or process death; ownership is determined by `flock`, not file existence.

## Failure and recovery

- A runtime conflict with active maintenance fails before database open and suggests retrying after maintenance completes.
- A schema mutation conflict with active runtimes fails before backup or schema writes and instructs the operator to stop all MCP runtimes.
- A maintenance conflict with active runtimes preserves the existing offline requirement and performs no operation-specific writes.
- If schema mutation fails, existing transactional migration and backup behavior remains authoritative. The exclusive reservation is released during cleanup.
- If the post-migration shared reopen or reclassification fails, the process does not start serving; a later startup may retry normally.
- SQLite busy or constraint errors from concurrent normal writes remain request-level errors. They must not cause lock-mode changes, implicit maintenance, or process termination.

## Stop gates

- Do not serve MCP requests unless the process holds shared canonical-path and identity reservations for the database it opened.
- Do not create, adopt, back up, or migrate a schema while only a shared reservation is held.
- Do not treat the migration lock as sufficient exclusion from already-running MCP processes.
- Do not weaken exclusive management, backup, restore, or FTS rebuild semantics.
- Do not allow a symlink alias to bypass shared runtime coordination or exclusive maintenance, and do not open a runtime through a multiply linked database inode.
- Do not enable concurrent runtime behavior only in in-process tests; the acceptance boundary is separate OS processes.
- Do not change journal mode or introduce a service/daemon as part of this feature.

## Ordered behavior slices

1. **Mode-aware operation reservation.** Extend the existing lock owner so runtime acquisition is shared and maintenance acquisition is exclusive across both path and identity locks. Prove mode compatibility, cleanup after partial failure, and alias behavior before wiring startup.
2. **Current-only runtime open.** Add or expose a storage path that reclassifies under the held shared reservation and opens only an already-current versioned store without schema mutation.
3. **Exclusive initialization and migration.** Wire MCP startup to leave shared mode, acquire exclusive mode, run the existing migration-capable open, release it, and re-enter the current-only shared path. Preserve management's existing exclusive migration-capable flow.
4. **Multiprocess acceptance and documentation.** Exercise real subprocess lifetimes and update operator and AI guidance after behavior is proven.

Slices 2 and 3 depend on the mode-aware reservation contract. Multiprocess acceptance depends on the final startup flow; documentation depends on verified behavior.

## Acceptance and proof

1. A focal regression first reproduces the original symptom: while one real `memory-mcp` subprocess is initialized against a temporary current database, a second current binary cannot initialize under the old exclusive-runtime behavior.
2. After the change, two real subprocesses using the same absolute database path both complete MCP initialization and expose the expected Project Memory tools. Terminating either process leaves the other usable.
3. Cross-process integration proves that a project or memory written by one runtime becomes readable from the other and that authoritative memory, enrichment, and FTS state remain synchronized.
4. Lock tests prove multiple shared runtime reservations coexist; exclusive maintenance conflicts with every active runtime; a shared runtime conflicts with active maintenance; and releasing the final runtime permits maintenance.
5. Alias tests prove canonical and symlink runtime paths share safely, a hardlinked database is rejected for runtime, management, and mutating maintenance opening, and maintenance cannot bypass an already-active runtime through a symlink or hardlink path. Tests cover partial-acquisition cleanup.
6. Migration tests prove a current store opens concurrently without writes; a migration-required store refuses mutation while another runtime is active; and, after all runtimes stop, migration succeeds with the existing backup and classification guarantees.
7. Simultaneous bootstrap testing proves at most one process mutates an empty store and any losing contender fails cleanly without corruption or partial schema state.
8. Existing management and maintenance tests continue proving exclusive behavior, restore recovery, FTS rebuild integrity, and stale lock-file recovery.
9. Focused storage/app/CLI tests pass, followed by the repository-required `make verify` sequence against the final implementation.
10. An independent critical review covers lock-mode compatibility, acquisition order and cleanup, the classification-to-open race, migration exclusion, alias handling, subprocess evidence, and the final diff. Any unresolved concurrency, corruption, migration, or maintenance-exclusion finding blocks completion.

## Documentation impact

Update the root README and affected `docs/ai` guides to state that normal MCP runtimes may share a current store, while management, mutating maintenance, initialization, adoption, and migration remain exclusive. Replace instructions that require stopping all MCP instances for ordinary startup, but preserve that requirement for schema changes and offline operations. Document actionable conflict behavior and the bounded SQLite writer-contention model.

## Done condition

The feature is complete only when the multiprocess acceptance evidence, affected integration suite, repository verification, documentation updates, and independent critical review all pass with no unresolved Stop Gate. The specification must be approved before implementation begins.
