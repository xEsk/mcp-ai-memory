# Memory Trust Roadmap

## Purpose and baseline

This roadmap turns the current local-memory server into a trustworthy foundation in three ordered phases: first make existing behavior correct and private, then make the store recoverable and retrieval quality measurable, and only then improve retrieval behavior and debuggability from evidence.

The design baseline is repository `mcp-ai-memory` at commit `f56c19430c986cd9c758a3050954772d1eecaf83` on `main`. The phase documents are specifications for review; they do not authorize implementation by themselves.

## Current status

Phase 1 was accepted on 2026-07-14 at implementation baseline `039f7c236096d94dfb6154cb0a4256b727afa9dd`. Its auditable closeout is recorded in [Phase 1 completion evidence](phase-1-completion-evidence.md). Phase 2 construction is integrated into `main`; its [synthetic completion evidence](phase-2-synthetic-completion-evidence.md) records the test proof. The real-data C1/C2/C3 Stop Gates remain pending separate authorization.

## Execution order

| Order | Priority and specification | Contains | Entry gate | Exit gate |
| --- | --- | --- | --- | --- |
| 1 | [Phase 1 — P0 Critical Correctness and Privacy](01-phase-1-p0-critical-correctness-privacy.md) | Deterministic soft-delete transaction cleanup, correct positional FTS5 BM25 weights, and explicit `off \| metadata \| raw` trace privacy with safe defaults. | Baseline confirmed; required project knowledge read; all database tests planned against temporary synthetic stores. | P0 acceptance criteria and verification pass without schema changes, reindexing, real-data access, or P1/P2 work. |
| 2 | [Phase 2 — P1 Continuity, Operability, and Retrieval Evidence](02-phase-2-p1-continuity-observability-quality.md) | Environment-only configuration, stable absolute database identity, schema classification, migration ledger, verified backup/restore and FTS recovery, honest retrieval metrics, and a versioned synthetic quality baseline. | Phase 1 accepted, including the BM25 correction that ranking evidence must measure. | Migration and recovery proof passes on synthetic fixtures; retrieval semantics are documented; the versioned corpus, judgments, metrics, and thresholds are reviewed and accepted. |
| 3 | [Phase 3 — P2 Retrieval Quality and Debuggability](03-phase-3-p2-retrieval-quality-debuggability.md) | Measured retrieval-quality improvements and clearer retrieval/suggestion diagnostics, constrained by the accepted Phase 2 evidence. | Phase 2 accepted; its corpus and baseline are frozen; no unresolved continuity, recovery, or semantic gate remains. | Every behavior change improves or preserves the approved quality gates, project isolation remains exact, diagnostics are truthful and privacy-safe, and full verification is accepted. |

Phases are executed sequentially. Work from a later priority must not be pulled into an earlier phase, and a failed or disputed exit gate blocks the next phase.

## Shared invariants

Every phase must preserve the existing local-first MCP server shape, SQLite with FTS5, strict project scoping, soft-delete semantics, and stdout as protocol-only output. Companion monitor or maintenance binaries remain optional and must not become runtime dependencies of the MCP server. Authoritative memory and enrichment tables remain the source of truth; FTS is derived state. Optional tracing remains asynchronous, non-blocking, fail-silent, and private by default. External services must not become required for core storage or retrieval. Existing public behavior and stored data remain backward compatible unless the approved phase explicitly changes them.

## Test-data safety

`data/memory.db` is real project data and is absolutely prohibited in construction tests, fixtures, smoke checks, automated classification, automated migration, automated backup/restore/rebuild, and quality evaluation. It must never be opened, read, copied, modified, or used as fixture input during automated roadmap implementation. The only exception is the explicit Phase 2 post-implementation C1/C2/C3 sequence: separately authorized read-only classification, offline snapshot, then version-specific migration.

Every database test must create a new database at an explicit path beneath that test's `t.TempDir()`. Fixtures and evaluation corpora must be synthetic and reproducible; no row or derived payload may come from the real database. Commands used as automated completion proof must also receive an explicit temporary path. A pre/post guard records real database and sidecar filesystem-entry metadata without opening their contents, rejects direct protected-path arguments and common shell indirection, and fails on entry changes. Because this is not an operating-system read audit, completion proof also inspects test sources for explicit `t.TempDir()` paths and absence of the protected path. Git diff alone is not evidence because these files are ignored. Any test or command that cannot establish this isolation is a Stop Gate.

## Migration, backup, and restore policy

Phase 1 performs no schema change, migration, reindex, or real-data operation. Phase 2 must establish the only permitted evolution path: read-only schema classification before writes, consecutive checksummed migration records, transactional migration steps, and a verified SQLite snapshot before modifying any recognized existing store. Unknown, partial, checksum-drifted, version-gapped, or future schemas fail closed.

Backup validation and restore are explicit offline operations. A backup must not be opened through the normal migration-capable startup path, and restore must never overwrite an active store implicitly. FTS rebuild may replace only derived index state from authoritative tables. Later phases may add a migration only through these accepted primitives and only after backup and restore have been rehearsed successfully on temporary synthetic fixtures. Destructive reset migrations and reconstruction of unknowable historical semantics are prohibited.

## Global Stop Gates

Stop the roadmap or current phase when any of the following is true:

- the previous phase has not been accepted;
- an automated test, construction command, or proposed automated proof would touch `data/memory.db` or cannot prove `t.TempDir()` isolation;
- a correction requires an unapproved schema, architecture, privacy, project-scope, or public-contract change;
- schema shape or migration history is unknown, partial, inconsistent, checksum-drifted, version-gapped, or from the future;
- backup integrity and offline restore have not been demonstrated before migration release;
- real-store classification, snapshot, and migration have not received their separate Phase 2 C1, C2, and C3 authorizations;
- any enabled trace delivery would be implicit, follow redirects, reach a non-loopback effective destination, or lack a separately approved authenticated and encrypted design for remote use;
- retrieval judgments, metrics, thresholds, or the Phase 2 baseline are disputed while Phase 3 behavior changes are proposed;
- legacy exposure counters would be represented as actual use, or unrecoverable historical distinctions would be invented;
- verification exposes project leakage, data loss, protocol output contamination, or an unresolved regression.

## Entry, completion, and handoff

Roadmap entry requires approved specifications, confirmation of the execution baseline, the relevant `docs/ai/` reading, and no unresolved gate for Phase 1. Each phase begins only from the accepted exit of its predecessor and with its own scope, branch/commit identity, focal tests, and temporary-fixture plan confirmed.

A phase exits only when its documented acceptance criteria are met, focal and full scoped verification pass, operational documentation and durable project knowledge match the implementation, automated verification leaves the real database untouched, and review accepts the completion proof. Any separately authorized Phase 2 C1/C2/C3 operation is reported as an explicit operational event, never hidden inside completion tests. The roadmap exits only after all three phases are accepted, recovery has been rehearsed, the quality gates pass, and runtime behavior, documentation, and Project Memory agree.

Phase 1 has completed the `implement-project-phase` workflow and is accepted. Phase 2 is now eligible for a separate explicit implementation handoff; that workflow must continue to execute and review one phase at a time, must not skip gates, and must not begin Phase 3 implicitly.
