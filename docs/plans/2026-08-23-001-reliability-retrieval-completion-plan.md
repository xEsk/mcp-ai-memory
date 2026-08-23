# Reliability And Retrieval Completion Plan

## Outcome And Acceptance

Complete the currently identified reliability work, close Phase 2 only through its authorised gates, and then deliver the accepted Phase 3 retrieval-quality specification.

Acceptance is:

- active documentation reports the implementation on `main`, not the deleted Phase 2 branch;
- backup, restore, inspection, and FTS recovery have explicit non-overlapping contracts and never mutate a store as an incidental effect of opening it for maintenance;
- restore verifies both SQLite integrity and the generated snapshot manifest;
- tracing drains cleanly and makes queue loss observable without exposing tool payloads;
- CI runs the guarded race suite, all three builds, source safety checks, and formatting/diff validation;
- Phase 2 C1, C2, and C3 remain separately authorised real-store operations;
- after Phase 2 acceptance, every acceptance item in `03-phase-3-p2-retrieval-quality-debuggability.md` passes without schema change, historical re-enrichment, or real-store access;
- final ignored binaries exist under `dist/` and the Git worktree contains only intentional source changes.

## Level

`CRITICAL`: the work changes persistence, recovery, concurrency, trace privacy/operability, CI, retrieval accounting, and query behavior. Focal RED/GREEN evidence is required for changed critical behavior, followed by guarded integration proof and an independent final review.

## Scope

### In scope

- Phase 2 maintenance corrections discovered after synthetic closeout.
- Trace shutdown and queue-loss health.
- Repository CI for the existing verified commands.
- Status and operational documentation corrections.
- The complete canonical Phase 3 scope after its entry gate is satisfied.
- Project Memory updates only after behavior is verified.

### Out of scope

- Vectors, a vector database, stemming, fuzzy search, language detection, or external thesauri.
- Cross-project search, generic note-taking, microservices, or background workers.
- Physical memory purge, import/export, packaging/notarisation, or a new release system.
- Automatic historical re-enrichment or any schema/tokenizer migration.
- A new user-feedback tool; usefulness feedback needs a separate product decision and contract.

## Existing Owners And Reuse

- `internal/storage` owns classification, locks, snapshots, restore, migrations, and FTS rebuild.
- `cmd/memory-maintenance` owns explicit operator commands.
- `internal/trace`, `internal/app`, and `internal/monitor` own trace delivery, lifecycle, and presentation.
- The existing guard script and Go test/build commands are the CI contract.
- `internal/search`, `internal/enrich`, `internal/mcpserver`, and the accepted synthetic corpus own Phase 3 behavior and proof.
- `docs/ai/` and the Memory Trust Roadmap remain canonical documentation; no parallel ADR or design is added.

## Ordered Vertical Slices

### Slice 1 — Truthful status and executable baseline

Outcome: remove branch-status drift and establish this plan as the implementation handoff.

Boundary: roadmap and `docs/ai/README.md` only.

Dependency: none.

Verification: links resolve, `git diff --check` passes, and status text matches current Git ancestry.

Done evidence: plan and corrected status committed on a dedicated `codex/` branch.

### Slice 2 — Side-effect-free offline maintenance

Outcome: each maintenance operation performs only its named action against an existing classified store.

Boundary: `internal/storage` and `cmd/memory-maintenance`.

Behavior:

- `inspect` follows one explicit concurrency contract; if documented as offline, it must participate in the operation lock.
- `backup` and `rebuild-fts` reject a missing database and do not call the migration-capable runtime opener.
- maintenance opening classifies first and rejects unsupported or unaccepted schema states without creating, adopting, backing up, or migrating the source implicitly;
- restore requires and validates the manifest path, basename, size, and SHA-256 before quarantine/install, then repeats SQLite integrity validation after install;
- failed backup/manifest and restore paths leave recoverable artifacts with deterministic errors.

Dependency: Slice 1 only for documentation truth.

Verification: focal storage and CLI tests cover missing stores, unversioned/current/unsupported stores, lock exclusion, manifest absence/drift, successful round-trip, rollback, and no incidental ledger/source mutation. All fixtures use `t.TempDir()`.

Done evidence: focal tests pass and `docs/ai/storage-search-guide.md`, `critical-rules.md`, and the roadmap evidence describe the actual contract.

### Slice 3 — Trace lifecycle and loss health

Outcome: enabled tracing drains on application shutdown and operators can see event loss without raw payload disclosure.

Boundary: `internal/trace`, `internal/app`, and the existing monitor event/UI contract.

Behavior:

- application ownership retains and closes the async sink exactly once;
- queued events drain before shutdown completes;
- full-queue drops increment a concurrency-safe counter;
- the monitor receives a metadata-only health signal or equivalent payload-free value and renders it without changing normal MCP responses;
- `off` remains the default and all delivery remains loopback-only, asynchronous, non-blocking, and fail-silent.

Dependency: independent of Slice 2 after the baseline checkpoint.

Verification: trace concurrency tests, application close tests, monitor JSON/UI tests, and privacy allowlist tests.

Done evidence: enabled shutdown has no leaked worker, drop counts are observable, and metadata mode exposes no arguments, responses, queries, project keys, content, or raw errors.

### Slice 4 — Reproducible CI and Phase 2 synthetic closeout

Outcome: every change to `main` is checked by the same safety proof used locally.

Boundary: `.github/workflows`, existing scripts, Makefile only if a missing current command must be named.

Behavior: CI runs the guarded race suite, verifies temporary-database source discipline, builds all three commands, checks formatting/diffs, and never accesses ignored runtime data.

Dependency: Slices 2 and 3.

Verification: workflow syntax inspection plus local execution of every shell command used by CI.

Done evidence: one workflow with least required permissions and no new build framework; updated Phase 2 synthetic evidence records the corrected proof.

### Checkpoint A — Pre-real-store completion

Run the complete guarded synthetic sequence. Stop on any storage, privacy, project-isolation, retrieval-metric, or quality-corpus regression. Obtain an independent review of the changed recovery and trace boundaries and resolve validated findings.

### Slice 5 — Separately authorised C1, C2, and C3

Outcome: classify, snapshot, and—only if necessary and explicitly accepted—apply the named migration/adoption range to the real configured store.

Boundary: operator commands only; no construction code changes during an operation.

Dependency: Checkpoint A accepted.

Verification and done evidence are the outputs required by the Phase 2 C1/C2/C3 gates. Each gate requires a new explicit user authorisation after the previous output is reviewed. C1 cannot imply C2; C2 cannot imply C3.

### Checkpoint B — Phase 2 acceptance

Update roadmap status and Project Memory only after the authorised gate sequence is accepted. Phase 3 remains blocked until then.

### Slice 6 — Phase 3 query normalization and compiler

Outcome: complete canonical Phase 3 Slice 1 exactly as specified, including Unicode-aware shared tokenization, exact short technical terms, shared precise/balanced terms, safe FTS escaping, and typed empty-query errors.

Dependency: Checkpoint B.

Verification: the canonical query focal commands and the full accepted Phase 2 corpus gate.

Done evidence: Checkpoint A of the Phase 3 specification passes without BM25, schema, tokenizer, or isolation changes.

### Slice 7 — Prospective deterministic enrichment

Outcome: complete canonical Phase 3 Slice 2 for rune-safe deterministic local enrichment and reviewed hard negatives.

Dependency: Slice 6.

Verification: exact-output enrichment tests and extended corpus gates; no existing store is read or rewritten.

Done evidence: Phase 3 Checkpoint B passes.

### Slice 8 — Conservative suggestion and truthful debug evidence

Outcome: complete canonical Phase 3 Slices 3 and 4: quality-aware balanced fallback, final-only deduplicated exposure accounting, DTO privacy, and highlight-backed debug attribution.

Dependency: Slice 6; suggestion corpus judgments must remain accepted.

Verification: MCP suggestion/accounting/privacy tests and one fixture per indexed debug field.

Done evidence: Phase 3 Checkpoint C passes with no lowered safe-update precision.

### Slice 9 — Measured Phase 3 integration

Outcome: complete canonical Phase 3 Slice 5, documentation, corpus versioning, and Project Memory update.

Dependency: Slices 6–8.

Verification: all Phase 3 focal commands, before/after quality report, and the final guarded sequence.

Done evidence: every Phase 3 completion checklist item passes without threshold relaxation.

## Final Verification

```bash
./scripts/verify-real-db-untouched.sh -- go test -race -count=1 ./...
go build -o dist/memory-mcp ./cmd/memory-mcp
go build -o dist/memory-monitor ./cmd/memory-monitor
go build -o dist/memory-maintenance ./cmd/memory-maintenance
rg -n 'memory\.db' --glob '*_test.go' .
rg -n 'data/memory\.db' --glob '*_test.go' .
gofmt -l cmd internal
git diff --check
```

The protected-path source search must exit `1` with no matches. `dist/` remains ignored but contains the requested final binaries.

## Stop Gates And Handoff

- Stop on any command that may open `data/memory.db` before explicit C1 authorisation.
- Stop before C2 and C3 for separate operator authorisations.
- Stop Phase 3 until Phase 2 is accepted and no continuity/recovery finding remains.
- Stop if manifest verification cannot distinguish a valid but substituted SQLite snapshot.
- Stop if trace health requires raw payload exposure or blocking MCP calls.
- Stop if Phase 3 requires schema/tokenizer changes, historical re-enrichment, threshold relaxation, or real-store access.

Handoff: execute with `implement-project-phase`, committing coherent slices and stopping at the first unsatisfied gate.
