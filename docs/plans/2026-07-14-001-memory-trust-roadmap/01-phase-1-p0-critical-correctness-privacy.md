# Phase 1 — P0 Critical Correctness and Privacy

## 1. Objective and outcome

Phase 1 makes the current local-memory promise safe enough to build on. It corrects two proven runtime defects and closes the implicit trace-payload disclosure boundary without changing the storage schema or expanding the product.

After this phase:

- every `SoftDelete` transaction terminates deterministically, including missing, repeated, and wrong-project deletes;
- SQLite FTS5 applies the already-documented BM25 weights to the intended indexed columns;
- no trace payload leaves `memory-mcp` under default configuration;
- metadata and raw tracing require explicit modes, and every enabled trace destination is restricted to loopback in this phase;
- project isolation, atomic soft delete, FTS5, local-first operation, and fail-silent asynchronous monitoring remain intact.

## 2. Current state, prerequisites, and Required Reading

### Execution baseline

- Repository: `mcp-ai-memory`
- Baseline commit: `f56c19430c986cd9c758a3050954772d1eecaf83`
- Branch at design time: `main`
- Worktree at design time: clean
- History at design time: one initial commit
- IDE index: unavailable because this repository was not open in the configured JetBrains instance; evidence was verified with bounded source reads and focal commands.
- Real database: `data/memory.db` exists and contains real project data. It is forbidden for construction and verification.

### Required Reading

- [AI knowledge index](../../ai/README.md)
- [Critical rules](../../ai/critical-rules.md)
- [Development guide](../../ai/development-guide.md)
- [Historical decisions](../../ai/historical-decisions.md)
- [Memory lifecycle internals](../../ai/memory-lifecycle-internals.md)
- [Storage and search guide](../../ai/storage-search-guide.md)
- [MCP tools guide](../../ai/mcp-tools-guide.md)
- `internal/memory/repository.go`
- `internal/storage/sqlite.go`
- `internal/search/fts.go`
- `internal/storage/migrations.go`
- `internal/app/config.go`
- `internal/app/app.go`
- `internal/mcpserver/server.go`
- `internal/trace/`
- `internal/monitor/`
- the focal tests in `internal/memory/`, `internal/mcpserver/`, `internal/trace/`, and `internal/monitor/`

### Proven defects

1. `Repository.SoftDelete` can return `ErrNotFound` without committing or rolling back. With one SQLite connection, the next database operation waits for the caller context to release the stranded transaction.
2. `bm25(memory_fts, ...)` receives ten indexed-field weights even though two `UNINDEXED` columns precede those fields. SQLite applies weights positionally, so the intended mapping is shifted.
3. Normal application assembly always injects an HTTP trace sink. Every tool wrapper copies raw arguments and responses into trace events even though no trace consent or payload mode exists.

## 3. Scope and architectural invariants

### In scope

- Deterministic transaction cleanup for every `SoftDelete` return path.
- Regression coverage for missing, repeated, wrong-project, and successful delete behavior.
- Two positional BM25 placeholders for `memory_id` and `project_key`.
- A frozen, ephemeral ranking fixture that proves the documented relative field weights.
- Trace modes with the contract `off | metadata | raw`.
- `off` as the default trace mode.
- An explicit metadata allowlist that excludes arguments, responses, search queries, project keys, memory content, and raw error text.
- Explicit opt-in and loopback-only delivery for both metadata and raw tracing in Phase 1.
- A repository verification guard that rejects real database paths in test commands and compares pre/post filesystem entry metadata for the ignored real database and sidecars without opening their contents.
- Configuration, README, `.env.example`, and `docs/ai/` updates needed to make these contracts visible.

### Non-goals

- Retuning the ten documented BM25 weights.
- Vector or hybrid search.
- Authentication, TLS, or any remote tracing.
- Trace completeness, persistence, or guaranteed delivery.
- Changes to soft-delete product semantics.
- New database columns, tables, migrations, reindexing, or re-enrichment.
- General configuration validation; Phase 1 validates only the new trace-mode contract.
- Refactoring transaction handling outside the proven `SoftDelete` boundary.

### Invariants

- Every memory operation remains project-scoped.
- Search retains both `project_key` and `project_id` filters and excludes soft-deleted rows.
- Successful delete updates the memory row and removes the FTS row atomically.
- FTS5 remains the active search implementation.
- Trace delivery, when enabled, remains asynchronous, non-blocking, and fail-silent.
- MCP stdout remains protocol-only.
- No phase test or smoke command may open `data/memory.db`.

## 4. Acceptance Contract and end-to-end journey

### User-visible journey

1. A trusted local MCP client starts `memory-mcp` with default configuration.
2. No trace HTTP event or raw tool payload leaves the process.
3. The client performs normal project, memory, search, and delete operations against a configured database.
4. Deleting an unknown or already-deleted memory returns the existing not-found tool error and the next operation succeeds immediately.
5. Searches use the documented field-weight ordering while preserving matching, filters, isolation, and count behavior.
6. If the operator explicitly selects metadata tracing and a loopback endpoint, the monitor receives only allowlisted operational fields.
7. If the operator explicitly selects raw tracing and a loopback endpoint, the existing full debugging payload is available with an explicit confidentiality warning.

### Durable proof

- Focused regression tests use `t.TempDir()` databases.
- Ranking tests exercise the real schema and `FTS.Search`, not a copied ranking helper.
- Trace policy tests use recording sinks or `httptest`; they do not contact an external network service.
- Documentation states the default, payload classification, loopback restriction, and restart requirement.
- A repository verification guard records only filesystem entry metadata for `data/memory.db`, `data/memory.db-wal`, and `data/memory.db-shm` before tests and fails if any entry is created, removed, resized, replaced, or timestamp-modified; it never opens or hashes database content.

### Authority owner

- Repository methods own transaction finalization.
- `docs/ai/memory-lifecycle-internals.md` owns the intended BM25 field mapping.
- Application configuration owns trace consent and payload mode.
- The trace wrapper enforces the chosen payload policy uniformly for all registered tools.

### Explicit failure conditions

Phase 1 is not accepted if any of the following is true:

- a zero-row delete leaves `db.Stats().InUse` nonzero or a bounded follow-up operation times out;
- a successful delete loses atomic memory/FTS behavior;
- title/content or the full relative ranking fixture still reflects the shifted mapping;
- default configuration emits any trace event to a configured test receiver;
- metadata mode contains sentinel request, response, project, query, content, or raw-error values;
- metadata or raw mode can send to a non-loopback effective destination, follow an HTTP redirect, or dial a non-loopback address;
- an unknown trace mode does not fail startup closed before a sink is built;
- implementation introduces a schema change or accesses the real database.

## 5. End-to-end flow and trust boundaries

### Delete flow

`memory.delete` → `memory.Service.Delete` → project resolution → `Repository.SoftDelete` → transaction → scoped memory update → scoped FTS delete → commit/rollback.

The transaction is the authority boundary. After `BeginTx` succeeds, every exit path must attempt rollback unless commit has completed.

### Search flow

`memory.search`/`memory.suggest` → `search.Service` → `FTS.Search` → FTS5 `MATCH` + project filters → BM25 rank → result limit → access recording.

The Phase 1 change is query-time weighting only. Stored rows, FTS documents, enrichment, and filters are unaffected.

### Trace flow

MCP tool wrapper → payload policy → optional async sink → loopback monitor.

Configuration is the consent boundary. A URL alone is routing information and must never activate tracing. Metadata mode is an allowlist; omission is safer than redaction-by-pattern. Raw mode is a separate explicit trust decision. Enabled modes accept only literal loopback IPs or the exact hostname `localhost`, disable redirects, and verify the connected remote address is loopback on every dial.

## 6. Walking Skeleton

The first observable checkpoint is the `SoftDelete` regression:

1. Add a temporary-SQLite test that deletes a missing ID.
2. Assert `ErrNotFound` remains the domain result.
3. Assert the database is immediately reusable without canceling the original context.
4. Preserve the existing successful-delete proof.

This walking skeleton crosses the MCP-relevant repository and SQLite boundary while remaining isolated from real data. It proves the phase can change critical behavior through a focused RED/GREEN contract before search and tracing work begins.

## 7. Vertical slices and grouped checkpoints

### Slice 1 — Deterministic delete transaction finalization

- **Behavior:** missing, repeated, and wrong-project deletes return `ErrNotFound` and release the connection; successful delete remains atomic.
- **Likely boundaries:** `internal/memory/repository.go`, `internal/memory/memory_integration_test.go` or a focused repository test.
- **RED:** a follow-up `PingContext` or normal repository operation times out after zero-row delete.
- **GREEN:** the same follow-up succeeds immediately; success-path delete tests remain green.
- **Dependency:** none.
- **Risk:** a timeout-only assertion can be flaky; pair `db.Stats().InUse` with a bounded behavioral operation.

### Slice 2 — Correct positional BM25 mapping

- **Behavior:** the documented weights apply to `type` through `content` in their actual table positions.
- **Likely boundaries:** `internal/search/fts.go`, focused ephemeral ranking tests, `docs/ai/memory-lifecycle-internals.md`.
- **RED:** with equal importance/timestamps and equal-length matches, title ranks ahead of content under the current shifted expression.
- **GREEN:** content ranks ahead of title, and the complete relative ordering matches the documented weights.
- **Dependency:** Slice 1 is not technically required, but both form the first correctness checkpoint.
- **Risk:** exact float scores vary by SQLite version; assert relative ordering and use tolerances only where needed.

### Slice 3 — Explicit trace consent and payload policy

- **Behavior:** default `off`; metadata is allowlisted; raw is explicit; both enabled modes are loopback-only and unknown modes fail startup closed.
- **Likely boundaries:** `internal/app/config.go`, `internal/app/app.go`, `internal/mcpserver/server.go`, `internal/trace/`, trace tests, README and config examples.
- **RED:** default assembly delivers a sentinel payload to an `httptest` receiver; metadata mode includes sentinel content.
- **GREEN:** default assembly uses `NoopSink`; metadata excludes all sentinels; raw includes them only under explicit loopback configuration; redirects, arbitrary hostnames, non-loopback IPv4/IPv6, and non-loopback dial results are rejected.
- **Dependency:** the mode and remote policy are resolved in this specification.
- **Risk:** this intentionally changes the convenience of starting the monitor without MCP-client reconfiguration.

### Slice 4 — Documentation and operational proof

- **Behavior:** operators can tell what each trace mode exposes, and maintainers can reproduce the P0 proofs without real data.
- **Likely boundaries:** `README.md`, `.env.example`, `docs/ai/development-guide.md`, `docs/ai/memory-lifecycle-internals.md`, `docs/ai/storage-search-guide.md`, `scripts/verify-real-db-untouched.sh`, Project Memory MCP.
- **RED:** current docs describe an optional monitor but not optional trace capture or payload consent.
- **GREEN:** all active docs agree on trace defaults, BM25 placeholders, delete cleanup, and safe test database rules.
- **Dependency:** Slices 1–3.
- **Risk:** stale redistributable skill guidance must be checked if trace configuration is referenced there.

### Checkpoint A — Critical correctness

Complete Slices 1 and 2, then run only the focal delete and search suites plus project-isolation checks.

### Checkpoint B — Privacy boundary

Complete Slice 3, then run focused app/config, MCP trace-wrapper, trace-sink, and monitor tests.

### Checkpoint C — Phase integration

Complete Slice 4, run the complete final verification sequence, and verify the real database remains untouched.

## 8. Applicable risk hardening

### Privacy

- Treat arguments, responses, queries, project keys, memory content, and raw error text as sensitive.
- Metadata is a closed allowlist: tool name, phase, duration, session identifier, process identifier, and a non-sensitive status classification.
- Raw mode must display/document a confidentiality warning.
- Phase 1 refuses non-loopback trace destinations for every enabled mode. Only literal loopback IPs and exact `localhost` are accepted, redirects are disabled, and the effective dial address is checked. Remote tracing requires a later authenticated/TLS design.

### Isolation and data safety

- All database tests use `t.TempDir()`.
- In-memory SQLite is acceptable for ranking-only observations; real integration tests use a temporary file through `storage.OpenSQLite`.
- No test copies real rows into a fixture.
- Project scoping assertions remain part of focal search and delete verification.

### Idempotency and recovery

- Repeated delete is explicitly tested.
- Unconditional rollback after successful commit may return `sql.ErrTxDone`; cleanup ignores that expected result.
- Trace delivery failure never changes MCP results.

## 9. Test, visible acceptance, and final verification strategy

### Focal construction tests

```bash
go test ./internal/memory -run 'SoftDelete|MemoryLifecycle_SearchIsProjectScoped' -count=1 -v
go test ./internal/search ./internal/memory -run 'BM25|Ranking|MemorySearchModesAndCount' -count=1 -v
go test ./internal/app -run 'Trace' -count=1 -v
go test ./internal/mcpserver -run '^TestTraceTool' -count=1 -v
go test ./internal/trace -run '^(TestMonitorSink|TestAsyncSink|TestNoopSink)' -count=1 -v
go test ./internal/monitor -run '^TestServer' -count=1 -v
```

Before a checkpoint is accepted, its filtered command must emit at least one matching `run` and `pass` event under `go test -json`; an exit-zero command with no matching test is a failure. Final test names are frozen in the implementation checklist when each RED test is introduced.

### Grouped verification

- Checkpoint A: delete, ranking, project isolation, count behavior, and successful soft delete.
- Checkpoint B: default-off, metadata allowlist, enabled-mode loopback enforcement, redirect/dial protection, async/fail-silent delivery, and monitor behavior.
- Checkpoint C: complete suite, race detector, both binaries, documentation consistency, and the real-database filesystem-metadata guard.

### Final sequence

```bash
./scripts/verify-real-db-untouched.sh -- go test -race ./...
go build ./cmd/memory-mcp
go build ./cmd/memory-monitor
git diff --check
```

The guard must report unchanged entries for the real database and sidecars without reading their contents. Any test that needs a database must prove its path is under `t.TempDir()` or `/tmp`.

## 10. Recovery and operations

- No schema or data migration is required.
- No FTS rebuild or memory re-enrichment is required because BM25 weights are query-time inputs.
- If ranking changes produce an unexpected regression, revert the query-time change; stored data remains compatible.
- If trace policy creates an operational regression, revert configuration/assembly changes; no stored data is involved.
- Existing real data must never be used to validate rollback, ranking, or trace behavior.

## 11. Documentation and decision impact

Update active documentation in English:

- `README.md`: trace is off by default, mode setup, loopback restriction, confidentiality warning.
- `.env.example`: explicit safe trace mode.
- `docs/ai/development-guide.md`: safe trace verification and temporary database rule.
- `docs/ai/memory-lifecycle-internals.md`: deterministic delete cleanup and positional BM25 placeholders.
- `docs/ai/storage-search-guide.md`: ranking fixture and no-reindex consequence.
- `docs/ai/critical-rules.md`: preserve async/fail-silent behavior while requiring explicit payload consent.
- Project Memory MCP: update the three P0 memories after implementation proof.

## 12. Stop Gates and gotchas

### Resolved decisions

1. **Trace default:** `off`.
2. **Trace modes:** `off | metadata | raw`.
3. **Metadata policy:** fixed allowlist; never heuristic redaction.
4. **Network policy:** both enabled modes are loopback-only; redirects are disabled; effective dial addresses must remain loopback.
5. **Invalid mode policy:** any value outside `off | metadata | raw` fails startup before a sink is built.
6. **BM25 policy:** preserve documented weights; add two positional placeholders only.
7. **Database policy:** no schema/data migration, reindex, or real-data smoke test.

### Hard Stop Gates during implementation

- Stop if any P0 correction appears to require a schema change.
- Stop if a proposed trace solution permits any remote delivery, redirect, or non-loopback effective dial without a separate authenticated/TLS design.
- Stop if the ranking fixture suggests the documented weights themselves must change; weight tuning belongs to a later measured phase.
- Stop if tests cannot prove their database path is temporary.
- Stop if a fix alters project scoping, soft-delete semantics, or synchronous access metrics.

### Gotchas

- `MEMORY_TRACE_MONITOR_URL` is not consent and must not enable tracing by itself.
- Error strings may contain technical or user-controlled data and are excluded from metadata mode.
- `memory.suggest` depends on search ordering and may return different candidates after the BM25 correction; that is an expected downstream effect and needs focal proof.
- The existing overlay reproduction under `/tmp` is diagnostic evidence, not a repository test artifact.

## 13. Evidence-backed checklist and Completion Proof

### Checklist

- [x] Missing-ID delete returns `ErrNotFound` and immediately releases the sole connection.
- [x] Repeated delete returns `ErrNotFound` and leaves the database reusable.
- [x] Wrong-project delete preserves isolation and releases the connection.
- [x] Successful delete remains atomic across memory and FTS state.
- [x] Real-schema ranking fixture fails before and passes after the placeholder correction.
- [x] Existing search matching, count, filters, isolation, and access behavior remain green.
- [x] Default application configuration uses no active trace sink.
- [x] Metadata trace contains only the documented allowlist.
- [x] Metadata and raw trace require explicit modes and loopback endpoints; redirects and non-loopback dials are rejected.
- [x] Unknown trace modes fail startup closed.
- [x] Async, non-blocking, fail-silent trace behavior remains green.
- [x] README, `.env.example`, `docs/ai/`, and Project Memory agree with runtime behavior.
- [x] Full race-enabled tests and both builds pass.
- [x] `git diff --check` passes.
- [x] The pre/post filesystem-entry guard reports the real database and sidecars unchanged without opening their contents.
- [x] No schema, migration, reindex, or real-data test was introduced.

### Completion evidence — 2026-07-14

- Delete RED: missing, repeated, and wrong-project cases retained one connection and timed out on the bounded follow-up operation before unconditional rollback cleanup. The same focal suite passed after the fix, including successful commit and forced FTS-failure rollback.
- Ranking RED: the ten weights were shifted by the two `UNINDEXED` positions, producing the wrong relative bands. The real-schema `t.TempDir()` fixture passed after adding the two neutral placeholders without retuning weights.
- Trace RED: the new mode, metadata-policy, invalid-mode, redirect, and effective-peer contracts failed to compile against the previous always-raw assembly. The focal app, MCP wrapper, trace sink, and monitor suites passed after implementation.
- Final: `./scripts/verify-real-db-untouched.sh -- go test -race ./...`, both documented `go build` commands, and `git diff --check` exited zero. The guard reported unchanged entry metadata for the real database and sidecars.
- Database fixtures: every new storage test opens `filepath.Join(t.TempDir(), "memory.db")`; no construction or verification command opened or copied `data/memory.db`.
- Review: the critical correctness review and full final review found no unresolved runtime finding. The guard path-normalization finding was reproduced RED, fixed, and independently re-reviewed clean.
- Durable memory: SoftDelete `1e05cfae-49a8-4390-9ef4-d79ae109e6fe`, BM25 mapping `616289c9-cdc8-4ece-a01f-d09e1e4131f9`, and trace privacy `0df59dda-14fd-42c0-9329-10ccfa265d17`.

### Completion Proof

Phase 1 is complete only when the implementation handoff contains:

1. the pre-fix RED output for delete, BM25 mapping, and trace-default tests;
2. the post-fix GREEN output for the same focal tests;
3. the final verification output with zero failures;
4. the exact temporary database paths or `t.TempDir()` evidence used by storage tests;
5. unchanged pre/post filesystem metadata for `data/memory.db` and its sidecars, plus proof that every storage test used an explicit temporary path;
6. updated English operational documentation;
7. updated durable Project Memory entries with the final implementation locations and gotchas.

## 14. Git and delivery expectations

- Implement on a dedicated `codex/` branch when the environment permits.
- Keep commits reviewable by boundary: delete transaction, BM25 mapping, trace privacy, documentation/verification.
- Do not commit temporary overlays, generated binaries, coverage files, or databases.
- Do not combine P1/P2 work with this phase.
- After user approval, hand this specification to `implement-project-phase`; do not implement it implicitly.
