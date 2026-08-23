# Phase 2 Synthetic Completion Evidence

## Scope and safety

This evidence covers Phase 2 construction and synthetic verification only. No command opened, copied, classified, migrated, backed up, restored, or rebuilt `data/memory.db`. Every database fixture uses a newly created path beneath `t.TempDir()`.

The real-store C1/C2/C3 gates remain deliberately pending separate operator authorization:

1. C1 permits only read-only classification.
2. C2 permits an offline validated snapshot after C1 acceptance.
3. C3 permits only the accepted migration range after C2 acceptance.

## Implemented boundaries

- `MEMORY_DB_PATH` is required and absolute before application storage access; the quickstart and `.env.example` now describe environment-only configuration.
- Storage classifies existing databases read-only under an exclusive migration lock, rejects unknown or ledger-mismatched shapes, bootstraps or adopts the current v1 schema transactionally, and records the checksummed `schema_migrations` ledger.
- `memory-maintenance` supports explicit offline inspect, backup, restore, and FTS rebuild operations. Backups use `VACUUM INTO`, restrictive permissions, a read-only integrity check, and a SHA-256 manifest. Restore preserves the source and WAL/SHM sidecars under a quarantine name.
- MCP responses expose aggregate `retrieval_count` and nullable `last_retrieved_at`; the old internal names remain private.
- The versioned synthetic quality corpus runs through real storage/services with deterministic local enrichment and gates Hit@1, Recall@5, MRR@10, nDCG@5, project leakage, no-match false positives, and critical grade-3 placement.

## RED/GREEN evidence

- Missing and relative `MEMORY_DB_PATH` startup tests first failed because startup created/opened a default or relative store. They pass after pre-storage validation.
- Ledger tests first failed because the migration symbols and unsupported-schema error did not exist. They pass with transactional bootstrap and unknown-schema rejection.
- Restore sidecar test first failed because WAL/SHM files were not quarantined. It passes after sidecar quarantine is included in restore.
- Retrieval response test first failed because `retrieval_count` was absent. It passes with the new public projection.
- Quality metric tests first failed before the metric primitives existed, then pass through the real temporary database harness.

## Final synthetic verification

```bash
./scripts/verify-real-db-untouched.sh -- go test -race -count=1 ./...
go build -o /private/tmp/mcp-ai-memory-phase2-mcp ./cmd/memory-mcp
go build -o /private/tmp/mcp-ai-memory-phase2-monitor ./cmd/memory-monitor
go build -o /private/tmp/mcp-ai-memory-phase2-maintenance ./cmd/memory-maintenance
git diff --check
```

The guarded race suite and all three builds exited zero. The guard reported `real database filesystem metadata unchanged`.

Quality corpus `2026-07-14-v1` reported global Hit@1, Recall@5, MRR@10, and nDCG@5 of `1.00`, with zero leakage, no-match false positives, and critical grade-3 misses. The initial baseline and all per-slice gates pass.

## Post-proof storage hardening

The pre-Phase-3 review added and passed synthetic regressions for complete schema-definition validation (including constraints and unexpected objects), stale migration-lock recovery, runtime-versus-maintenance exclusion, and rollback when final restored-snapshot validation fails. Both operation and migration locks are kernel-managed advisory locks, so stale lock files no longer block a later process after a crash. The final guarded race suite was rerun after these corrections.

## Reliability follow-up

The August 2026 follow-up removed the remaining incidental-write path from offline maintenance. Backup and FTS rebuild now reject missing paths before lock or directory creation and open recognised stores without runtime migration or adoption. Restore requires the generated manifest, verifies its snapshot identity, size, and SHA-256 before quarantine, repeats SQLite and manifest validation after installation, and rolls the source back if final validation fails. Synthetic regressions cover missing stores, current unversioned stores, absent manifests, valid-but-substituted snapshots, rollback, and sidecar handling.

Enabled tracing now has explicit application lifecycle ownership. Graceful shutdown closes the asynchronous sink exactly once, drains accepted events, and emits a payload-free `trace.queue` health event when queue pressure caused drops. The monitor renders the cumulative count, and the metadata privacy allowlist includes only the numeric `dropped_events` addition.

The repository now uses `.github/workflows/verify.yml` and the same `make verify` entry point locally. The verification target tests the real-store guard, runs the full guarded race suite, builds all three commands, checks Go formatting and diff whitespace, lists temporary test-database fixtures, and rejects any test source that names `data/memory.db`. The complete target exited zero after these corrections and reported `real database filesystem metadata unchanged`.
