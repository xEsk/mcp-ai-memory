# Phase 1 Closeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining Phase 1 evidence, verification-guard, documentation, and durable-memory gaps without changing product runtime behavior.

**Architecture:** Keep the runtime commits unchanged. Harden the repository-only verification guard with conservative argument checks, record reproducible evidence in the existing roadmap, align all active roadmap commands and claims, then accept Phase 1 and pin Phase 2 to the accepted closeout commit.

**Tech Stack:** POSIX shell, Go test/build tooling, Markdown, Git, Project Memory MCP.

## Global Constraints

- Do not open, read, copy, hash, migrate, or rebuild `data/memory.db` or its sidecars.
- Do not change the storage schema, migrations, BM25 tuning, trace runtime, or soft-delete semantics.
- Every Go database test must continue to use `filepath.Join(t.TempDir(), "memory.db")`.
- Final Go tests must run with `-race -count=1` through the metadata guard.
- Keep documentation and Project Memory in English.

---

### Task 1: Harden the real-database metadata guard

**Files:**
- Modify: `scripts/verify-real-db-untouched_test.sh`
- Modify: `scripts/verify-real-db-untouched.sh`

**Interfaces:**
- Consumes: `verify-real-db-untouched.sh -- command [args...]`
- Produces: conservative rejection of direct protected-path tokens and `sh`, `bash`, or `zsh` indirection while preserving direct command execution and metadata comparison.

- [ ] **Step 1: Add failing shell tests**

Add rejection cases for an embedded `data/memory.db` token and for `sh -c`, `bash -c`, `zsh -c`, including `env sh -c`.

- [ ] **Step 2: Verify RED**

Run: `./scripts/verify-real-db-untouched_test.sh`

Expected: failure reporting that an embedded path or shell interpreter was accepted.

- [ ] **Step 3: Implement the minimal guard checks**

Reject any argument containing the canonical relative or absolute protected path, and reject arguments whose basename is `sh`, `bash`, or `zsh`. Keep the existing normalized-path and metadata checks.

- [ ] **Step 4: Verify GREEN**

Run: `./scripts/verify-real-db-untouched_test.sh`

Expected: `verify-real-db-untouched guard tests passed`.

### Task 2: Record auditable Phase 1 evidence

**Files:**
- Create: `docs/plans/2026-07-14-001-memory-trust-roadmap/phase-1-completion-evidence.md`
- Modify: `docs/plans/2026-07-14-001-memory-trust-roadmap/01-phase-1-p0-critical-correctness-privacy.md`

**Interfaces:**
- Consumes: the accepted Phase 1 focal tests and implementation commits.
- Produces: durable RED/GREEN/final transcripts, source-level `t.TempDir()` evidence, and an explicit description of the guard's proof boundary.

- [ ] **Step 1: Reproduce RED safely**

Create disposable source trees beneath `/tmp` from `HEAD`, reverse only the relevant production-file changes for the delete, BM25, and trace commits, and run the unchanged focal tests. Do not include `.git`, databases, or temporary trees in the repository.

- [ ] **Step 2: Capture GREEN and isolation evidence**

Run the unchanged focal tests at `HEAD`, list every `_test.go` SQLite opening, and confirm each path uses `t.TempDir()` and no test source names `data/memory.db`.

- [ ] **Step 3: Write the evidence document**

Record exact commands, exit status, material output, temporary-source method, and the guard limitation: it is a metadata-change tripwire plus conservative argument filter, not an operating-system audit of arbitrary executable reads.

- [ ] **Step 4: Link the evidence from Phase 1**

Replace narrative-only proof claims with a link to the durable transcript and retain the concise acceptance summary.

### Task 3: Align active documentation and Phase 2 handoff

**Files:**
- Modify: `README.md`
- Modify: `docs/ai/development-guide.md`
- Modify: `docs/ai/README.md`
- Modify: `docs/plans/2026-07-14-001-memory-trust-roadmap/README.md`
- Modify: `docs/plans/2026-07-14-001-memory-trust-roadmap/01-phase-1-p0-critical-correctness-privacy.md`
- Modify: `docs/plans/2026-07-14-001-memory-trust-roadmap/02-phase-2-p1-continuity-observability-quality.md`
- Modify: `docs/plans/2026-07-14-001-memory-trust-roadmap/03-phase-3-p2-retrieval-quality-debuggability.md`

**Interfaces:**
- Consumes: verified closeout state and accepted Phase 1 commit identity.
- Produces: unambiguous trace opt-in wording, uncached verification commands, honest guard claims, and a Phase 2 baseline pinned to the final accepted closeout commit.

- [ ] **Step 1: Clarify trace configuration wording**

State that the MCP command is unchanged but `MEMORY_TRACE_MODE` must be added to the MCP process environment to enable tracing.

- [ ] **Step 2: Make verification uncached**

Change roadmap and development commands to `go test -race -count=1 ./...`.

- [ ] **Step 3: Align guard claims**

State exactly what the guard verifies and what source review supplies; remove claims that it can prove arbitrary read absence.

- [ ] **Step 4: Record formal acceptance and baseline**

After the closeout commit exists, replace the Phase 2 design baseline with that exact accepted commit and record the dedicated Phase 2 branch requirement.

### Task 4: Reconcile Project Memory and verify delivery

**Files:**
- Modify through MCP: stale BM25 memory `fca554c8-1897-41ec-9e44-1dd97326ef8e`
- Modify through MCP: roadmap memory `096edbff-9ad9-4a73-9b97-14b7f03b98ae`

**Interfaces:**
- Consumes: accepted closeout commit and documentation.
- Produces: one canonical BM25 implementation memory and a roadmap memory that identifies Phase 1 acceptance and the Phase 2 baseline.

- [ ] **Step 1: Delete the superseded BM25 bug memory**

Soft-delete `fca554c8-1897-41ec-9e44-1dd97326ef8e` with a reason that points to `616289c9-cdc8-4ece-a01f-d09e1e4131f9`.

- [ ] **Step 2: Run final verification**

Run:

```bash
./scripts/verify-real-db-untouched_test.sh
./scripts/verify-real-db-untouched.sh -- go test -race -count=1 ./...
go vet ./...
go build -o /tmp/mcp-ai-memory-phase1-closeout-mcp ./cmd/memory-mcp
go build -o /tmp/mcp-ai-memory-phase1-closeout-monitor ./cmd/memory-monitor
git diff --check
```

Expected: every command exits zero; the guard reports unchanged filesystem metadata.

- [ ] **Step 3: Commit and pin the baseline**

Commit the closeout changes, insert that exact commit as the accepted Phase 1 / Phase 2 execution baseline, amend the closeout commit, and rerun final verification.

- [ ] **Step 4: Update durable roadmap memory**

Update `096edbff-9ad9-4a73-9b97-14b7f03b98ae` with the accepted commit, evidence location, guard proof boundary, and the requirement that Phase 2 start on its own `codex/` branch from that commit.
