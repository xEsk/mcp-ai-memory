# Phase 2 — P1 Continuity, Operability, and Retrieval Evidence

## 1. Objective and outcome

Phase 2 makes the local store unambiguous, its evolution recoverable, its retrieval activity honestly observable, and retrieval quality measurable before Phase 3 changes search behavior.

After this phase:

- the process environment is the single configuration authority and the quickstart no longer implies that copying `.env` configures the binary;
- `MEMORY_DB_PATH` is explicit and absolute, so startup cannot silently select a different store from a different working directory;
- every supported database shape is classified read-only before any write;
- schema evolution uses a checksummed migration ledger, transactional steps, a verified pre-migration backup, and an offline recovery path;
- FTS can be validated and rebuilt from authoritative source tables without changing memory content;
- existing access history is exposed honestly as retrieval frequency and recency without migrating or inventing usage data;
- a versioned synthetic corpus and quality metrics form the mandatory gate for Phase 3.

## 2. Current state, prerequisites, and Required Reading

### Prerequisite

Phase 1 must be fully implemented and accepted before any Phase 2 slice begins. It overlaps configuration and documentation boundaries, and its BM25 correction must already be present when the retrieval-quality baseline is frozen.

### Execution baseline

- Repository: `mcp-ai-memory`
- Baseline commit: `f56c19430c986cd9c758a3050954772d1eecaf83`
- Branch at design time: `main`
- Real database: `data/memory.db` contains real project data and must not be opened, copied, classified, migrated, or rebuilt during construction or automated verification.
- All database fixtures must live under `t.TempDir()`; no fixture may be derived from real rows.

### Required Reading

- [AI knowledge index](../../ai/README.md)
- [Critical rules](../../ai/critical-rules.md)
- [Development guide](../../ai/development-guide.md)
- [Historical decisions](../../ai/historical-decisions.md)
- [Storage and search guide](../../ai/storage-search-guide.md)
- [Memory lifecycle internals](../../ai/memory-lifecycle-internals.md)
- [Enrichment guide](../../ai/enrichment-guide.md)
- `README.md`
- `.env.example`
- `cmd/memory-mcp/main.go`
- `internal/app/config.go`
- `internal/app/app.go`
- `internal/storage/sqlite.go`
- `internal/storage/migrations.go`
- `internal/memory/model.go`
- `internal/memory/service.go`
- `internal/memory/repository.go`
- `internal/search/fts.go`
- `internal/mcpserver/responses.go`
- current integration and MCP tool tests

### Proven P1 weaknesses

1. The quickstart copies `.env`, but `LoadConfig` only reads `os.Getenv`; `.env.example` and code also disagree on the LLM model fallback.
2. The default `./data/memory.db` resolves against the process working directory. `OpenSQLite` then creates and initializes the selected store, so a CWD change can look like data loss.
3. `ApplyMigrations` has no ledger or transaction. A synthetic incompatible schema proves that earlier DDL persists after a later statement fails.
4. `access_count` and `last_accessed_at` aggregate returned/exposed memories across search, debug, get, list, and suggest, but the public MCP memory response omits them and their names suggest stronger usage semantics than the system can prove.
5. The suite protects mechanics but has no representative corpus, judgments, aggregate retrieval metrics, or frozen regression thresholds.

## 3. Scope, non-goals, and invariants

### In scope

- Process environment as the only runtime configuration authority.
- Removal of the misleading `cp .env.example .env` quickstart contract.
- Alignment of examples and built-in fallbacks; no speculative model upgrade.
- Required, absolute `MEMORY_DB_PATH` validation before filesystem or SQLite access.
- Read-only schema classification for empty, recognized legacy, current unversioned v1, supported versioned, newer versioned, and partial/unknown stores.
- `schema_migrations(version, name, checksum, applied_at)` with strict ordering and immutable checksums.
- A cross-process exclusive migration lock covering reclassification, snapshot, migration, and post-validation.
- Per-migration transaction and same-transaction ledger entry.
- Verified SQLite snapshot before bootstrap or pending migrations on an existing recognized store.
- Explicit offline inspect, backup, restore, and FTS rebuild operations using existing storage boundaries and standard-library CLI parsing.
- Public `retrieval_count` and `last_retrieved_at` fields projected from legacy internal fields.
- Versioned synthetic retrieval corpus, judgments, metrics, baseline, and thresholds.

### Non-goals

- Loading `.env` inside the binary or adding a dotenv dependency.
- Choosing a newer LLM or embedding model.
- Automatically locating, moving, merging, or deleting possible legacy stores.
- Inspecting or migrating the real configured database as part of implementation.
- Claiming that retrieval counts prove reading, reuse, correctness, or business value.
- Splitting historical access by operation; the stored aggregate cannot be disaggregated.
- Changing ranking, tokenization, stopwords, enrichment, or suggestion heuristics.
- Vector or hybrid retrieval.
- Copying real memories into evaluation fixtures.

### Invariants

- Explicit environment values remain the runtime input mechanism for MCP clients.
- Startup validation writes errors to stderr, never stdout.
- Unknown, partial, checksummed-mismatch, gapped, or future schemas fail closed without writes.
- Migration checksums are lowercase hexadecimal SHA-256 over the exact UTF-8 bytes of `"mcp-ai-memory-migration-v1\x00" + decimal_version + "\x00" + name + "\x00" + canonical_payload`; no whitespace or newline normalization is permitted. `canonical_payload` is the immutable ordered migration descriptor committed with the version.
- A backup is never opened through the normal migrating `OpenSQLite` path during validation.
- Source memory and enrichment tables are authoritative; FTS is derived.
- Project isolation and soft-delete filtering remain unchanged.
- Existing `access_count` and `last_accessed_at` values are preserved byte-for-byte.

## 4. Acceptance contract and end-to-end journey

### Normal startup

1. The operator provides an absolute `MEMORY_DB_PATH` in the MCP client environment or shell.
2. Configuration validation rejects a missing, empty, or relative path before directory creation or SQLite open.
3. Storage acquires the migration lock when bootstrap or pending migrations may be needed, then reclassifies the database without mutating it.
4. A current supported database with no pending migration opens normally.
5. An empty path creates the current schema and ledger atomically.
6. A recognized existing database with pending bootstrap/migrations first creates and verifies a unique snapshot, then applies each migration and ledger record transactionally.
7. An unknown or newer database aborts with a non-secret, actionable stderr message.

### Retrieval observability

1. An MCP client retrieves memories through search, search debug, get, list, or suggest.
2. Existing internal counters continue to increment exactly as before.
3. Public memory objects include `retrieval_count` and nullable `last_retrieved_at`.
4. Upsert can report existing history but does not increment it; count and delete do not increment it.
5. Documentation calls these values aggregate retrieval/exposure signals, never demonstrated usage.

### Quality gate

1. The test harness creates a temporary SQLite database from a versioned synthetic corpus.
2. It executes versioned queries through the real services and schema with deterministic local enrichment.
3. It computes per-query, aggregate, per-slice, and per-mode metrics.
4. Project leakage must be zero and the frozen thresholds must pass.
5. Phase 3 cannot change retrieval behavior until this baseline is accepted.

### Explicit failure conditions

Phase 2 is not accepted if:

- two working directories can select different stores under the same accepted configuration;
- startup creates any file before rejecting missing/relative configuration;
- schema classification mutates the target;
- a failed migration leaves DDL, data, or ledger partially committed;
- an existing recognized store is migrated without a validated snapshot;
- two startup processes can classify/snapshot/migrate the same path concurrently, or the store can change between snapshot validation and migration;
- restore or FTS rebuild can run while normal MCP access remains active;
- public output calls the legacy aggregate “use” or loses historical values;
- quality tests use real data, external enrichment, exact BM25 floats, or project-leaking results.

## 5. Flows and authority boundaries

### Configuration and store identity

MCP client/shell environment → `LoadConfig` → validation → canonical absolute path → storage open.

Configuration owns store identity. Storage must never guess a path, inspect alternate CWD-relative candidates, or create a replacement store after validation failure.

### Migration

Read-only preflight → pending check → acquire exclusive migration lock → read-only reclassification under lock → verified snapshot → revalidate source identity/version → migration transaction → post-validation → ledger insert → commit → release lock.

Classification, backup, migration, and normal opening are distinct operations. Every new binary honors the same lock; the first real rollout also requires old binaries to be stopped because they cannot honor it. `IF NOT EXISTS` must not hide divergence in forward migrations.

### Recovery and FTS rebuild

Explicit offline maintenance command → absolute path validation → exclusive operation → integrity/ledger checks → requested backup, restore, or transactional FTS rebuild → post-validation.

Restore is never automatic. Rebuild reads `projects`, active `memories`, and `memory_enrichments`; it never edits source rows.

## 6. Walking skeleton

The first Phase 2 checkpoint is a configuration test that changes between two temporary working directories and proves that:

1. a missing path fails before any `data/` directory appears;
2. a relative path fails with an actionable error;
3. one explicit absolute temporary path reopens the same project from either CWD.

This closes the most immediate continuity risk without touching migration code or real data.

## 7. Vertical slices and grouped checkpoints

### Slice 1 — Honest configuration and stable store identity

- **Behavior:** environment-only configuration; required absolute DB path; aligned documented fallbacks.
- **Likely boundaries:** `README.md`, `.env.example`, `Makefile`, `internal/app/config.go`, `cmd/memory-mcp/main.go`, config tests.
- **RED:** copied `.env` is ignored; missing default resolves to different stores under two temporary CWDs.
- **GREEN:** quickstart passes an explicit absolute path; missing/relative values fail before storage; absolute values remain stable.
- **Migration:** none.

### Slice 2 — Read-only classification and versioned atomic migrations

- **Behavior:** exact schema signatures, canonical SHA-256 ledger checksums, exclusive migration locking, atomic bootstrap and forward migrations.
- **Likely boundaries:** `internal/storage/migrations.go`, focused classifier/migration files and tests, `internal/storage/sqlite.go`.
- **RED:** the synthetic incompatible fixture leaves partial tables after failure.
- **GREEN:** unknown/incomplete/future shapes cause zero writes; checksum byte drift fails closed; injected migration failure rolls back DDL, data, and ledger; two concurrent startups cannot both snapshot or migrate.
- **Dependency:** Slice 1, because every fixture and operation needs an explicit absolute path.
- **Schema change:** existing recognized databases gain `schema_migrations`; recognized inline-enrichment legacy data may be normalized only through the approved migration path.

### Slice 3 — Verified backup, offline restore, and FTS rebuild

- **Behavior:** existing recognized stores get a consistent verified snapshot while holding the migration lock before the first migration write; operators have explicit offline recovery and derived-index rebuild paths.
- **Likely boundaries:** `internal/storage/`, a small `cmd/memory-maintenance/` entrypoint using standard-library flags, storage tests, operational docs.
- **Backup contract:** `VACUUM INTO` a unique same-directory file, restrictive permissions, read-only `integrity_check`, version range, size, checksum, and timestamp manifest; never overwrite.
- **Restore contract:** stop all instances, verify snapshot/manifest, quarantine original and WAL/SHM sidecars, atomically install snapshot on the same filesystem, sync, validate read-only, and leave all artifacts intact on failure.
- **Rebuild contract:** replace FTS transactionally from authoritative active rows; assert one FTS row per active memory, no deleted/orphan rows, and coherent project keys.
- **Dependency:** Slice 2.

### Slice 4 — Honest public retrieval metrics

- **Behavior:** expose existing aggregate history without schema or semantic invention.
- **Likely boundaries:** `internal/mcpserver/responses.go`, response tests, `docs/ai/memory-lifecycle-internals.md`, `docs/ai/mcp-tools-guide.md`.
- **Mapping:** `retrieval_count := access_count`; `last_retrieved_at := last_accessed_at` with null preserved.
- **Counting:** search, search debug, get, list, and suggest count; count, upsert, and delete do not.
- **GREEN:** every public memory response uses only the new public names and preserves current increments.
- **Migration:** none; internal columns remain unchanged.

### Slice 5 — Versioned retrieval-quality gate

- **Behavior:** deterministic, reviewable quality evidence blocks regressions and Phase 3 guesses.
- **Likely boundaries:** `internal/search/testdata/`, `internal/search/retrieval_quality_test.go`, metric helpers scoped to tests, storage/search documentation.
- **Corpus:** synthetic stable IDs; multiple fictitious projects; same-project distractors; cross-project duplicates; exact identifiers/paths, natural-language paraphrases, enrichment-only terms, shared vocabulary, near-duplicates, no-match, and multilingual technical text.
- **Judgments:** grades `3` direct, `2` useful support, `1` tangential, `0` irrelevant.
- **Metrics:** exact-identifier Hit@1, Recall@5 for grade ≥2, MRR@10, nDCG@5, and project leakage.
- **Initial gates:** leakage `0`; exact identifier/path Hit@1 `1.00`; global Recall@5 `≥0.90`; global MRR@10 and nDCG@5 `≥0.80`; per-slice Recall@5 `≥0.80`; no critical grade-3 target outside top 5.
- **Baseline rule:** freeze after Phase 1; later global regression >0.02 or slice regression >0.05 fails. Baselines and thresholds may change only with reviewed corpus/judgment changes, never automatically to make code pass.

### Checkpoint A — Store continuity

Complete Slice 1. Verify no SQLite file is created by rejected configuration.

### Checkpoint B — Migration safety on synthetic fixtures

Complete Slices 2 and 3. Exercise only `t.TempDir()` fixtures, including WAL snapshot and interrupted migration/restore/rebuild cases.

### Checkpoint C1 — Real-store read-only classification authorization

Construction and automated verification stop here. After all synthetic fixtures pass, a separate explicit authorization may permit only read-only classification of the real configured path. It must report shape/version and pending action without content, snapshot, ledger adoption, or any write.

### Checkpoint C2 — Offline snapshot authorization

After the operator accepts the C1 classification, all processes are stopped and the exclusive migration lock is acquired. A second explicit authorization may create and validate the snapshot and manifest only. No bootstrap or migration begins, and the source is reclassified under lock.

### Checkpoint C3 — Version-specific migration authorization

After the operator accepts the verified C2 snapshot, source/target versions, free-space result, and restore procedure, a third explicit authorization may apply only the named migration range while the lock remains held. Each migration revalidates the expected predecessor and commits its ledger entry atomically.

### Checkpoint D — Product evidence

Complete Slices 4 and 5, freeze the reviewed baseline, update documentation, and run final verification.

## 8. Migration fixtures and quality corpus

### Required migration fixtures

All are hand-built synthetic databases in `t.TempDir()`:

- empty database → current v1 plus ledger; second open performs no write;
- exact current v1 without ledger → verified backup and ledger adoption without changing rows or FTS;
- complete recognized inline-enrichment legacy → backup, transactional normalization, preserved data, valid ledger;
- incomplete/partial legacy → classification error and byte/logical schema unchanged;
- injected failure → rollback of DDL, data, and ledger;
- checksum mismatch, gap, and future version → zero-write failure;
- two concurrent startup processes → one lock owner; the other waits or fails without snapshot/migration writes;
- backup failure → migration never starts;
- WAL fixture with confirmed writes → snapshot contains all confirmed rows;
- interrupted restore → original, quarantine, and backup remain recoverable;
- corrupt/missing/orphan/deleted FTS cases → correct rebuild; injected rebuild failure preserves previous index.

### Corpus governance

- Corpus and queries have independent explicit version fields.
- Query judgments reference existing stable memory IDs and valid projects.
- Harness validation fails before search on duplicate/missing IDs, invalid grades, unknown slices/modes, or cross-project judgments.
- Metric unit tests use perfect and deliberately degraded rankings.
- Search evaluation uses the real schema and services, not a copied ranking implementation.
- External LLMs, network calls, clocks, and random fixture generation are forbidden in the gate.

### Metric formulas and aggregation

- A query is either `evaluable` (at least one judgment with grade ≥2) or `no_match`; a slice with zero evaluable queries is invalid.
- Hit@1 is `1` when rank 1 has grade ≥2, otherwise `0`.
- Recall@5 is relevant grade-≥2 documents retrieved in the top 5 divided by all grade-≥2 judgments for that query.
- MRR@10 is the reciprocal rank of the first grade-≥2 result through rank 10, otherwise `0`.
- nDCG@5 uses gain `2^grade - 1` and log2 discount, divided by ideal DCG for that query.
- `no_match` queries are excluded from those four denominators and instead must return zero results; their false-positive rate is reported separately and must remain `0` on critical hard negatives.
- Metrics are reported separately per mode and slice. A global score is the query-count-weighted mean across all evaluable queries, never an unweighted mean of slice means.
- A `critical: true` query with a grade-3 judgment must place at least one grade-3 document in the top 5.
- Regression limits `0.02` and `0.05` are absolute score-point decreases from the frozen baseline, not relative percentages.

## 9. Test and final verification strategy

### Focal commands

```bash
go test ./internal/app -run 'Config|DBPath' -count=1 -v
go test ./internal/storage -run 'Classif|Migration|Backup|Restore|Rebuild' -count=1 -v
go test ./internal/mcpserver -run 'RetrievalCount|LastRetrieved|MemoryOutput' -count=1 -v
go test ./internal/search -run 'RetrievalQuality|Metric|Corpus' -count=1 -v
go test ./internal/memory ./internal/mcpserver -run 'Access|SearchIsProjectScoped|Suggest' -count=1 -v
```

Each filtered command is also captured with `go test -json`; completion proof must contain at least one matching `run` and `pass` event per acceptance boundary. Exit zero with no executed matching test is failure.

### Final sequence

```bash
./scripts/verify-real-db-untouched.sh -- go test -race ./...
go build ./cmd/memory-mcp
go build ./cmd/memory-monitor
go build ./cmd/memory-maintenance
git diff --check
```

The guard must report unchanged filesystem entries for the real database and sidecars without opening their contents. Test logs must identify temporary database paths and the quality report must show aggregate and per-slice metrics.

## 10. Recovery and operations

- Startup errors identify configuration or schema class/version without printing secrets or memory content.
- Existing DB migration is fail-closed if snapshot space, snapshot creation, checksum, or integrity validation fails.
- Snapshot retention is operator-controlled and documented; the implementation never silently deletes backups.
- Migration failure reports the last committed version and verified backup path; it does not restore automatically.
- Restore and FTS rebuild require normal MCP instances to be stopped and are never invoked during ordinary startup.
- Multiple old binaries are an operational hazard because they do not understand the new gate; the first real rollout requires all instances stopped.
- The lock covers reclassification, snapshot, source revalidation, migration, and post-validation; a second participating process fails or waits without creating another snapshot.
- A versioned dry-run reports classification and pending actions without writing.

## 11. Documentation impact

Update active English documentation:

- `README.md`: environment-only quickstart, absolute DB path, maintenance commands, retrieval metric meaning.
- `.env.example`: reference values aligned with code and clearly marked as not auto-loaded.
- `docs/ai/development-guide.md`: configuration validation, temporary DB rule, migration fixture workflow.
- `docs/ai/storage-search-guide.md`: ledger, backup/restore/rebuild, quality corpus and gates.
- `docs/ai/memory-lifecycle-internals.md`: retrieval metric mapping and counting paths.
- `docs/ai/mcp-tools-guide.md`: public retrieval fields.
- `docs/ai/historical-decisions.md`: environment authority, stable store identity, migration safety, synthetic evaluation contract.
- Project Memory MCP: store the accepted configuration, migration, recovery, and quality-gate decisions after implementation proof.

## 12. Stop Gates and resolved decisions

### Resolved decisions

1. Runtime configuration authority is the process environment; the binary does not load `.env`.
2. `MEMORY_DB_PATH` is required and absolute; missing/relative values fail before storage access.
3. No store is auto-discovered, moved, merged, overwritten, or deleted.
4. Existing access history is exposed as retrieval/exposure, not use; no access-field migration occurs.
5. Product schema versions use a checksummed ledger, not SQLite's internal schema version.
6. Ledger checksums use the exact canonical SHA-256 byte contract defined in Section 3.
7. Existing recognized stores receive a verified `VACUUM INTO` snapshot under an exclusive migration lock before migration writes.
8. Real-store classification, snapshot, and migration require the three separate C1/C2/C3 authorizations.
9. Restore is explicit and offline; FTS rebuild changes only derived index state.
10. Phase 3 is gated by the reviewed versioned corpus and baseline.

### Sequential C1/C2/C3 real-data gates

- **Before C1:** all synthetic migration, concurrent-startup, WAL backup, restore, and rebuild fixtures pass under race-enabled tests; restore has been rehearsed on synthetic WAL fixtures.
- **C1:** the operator separately authorizes read-only classification. Continue only if it recognizes exactly one supported shape without writes and the operator accepts the reported source/pending versions.
- **Before C2:** every process using the path is stopped; the migration lock is acquired; the source is reclassified under lock and still matches C1.
- **C2:** the operator separately authorizes snapshot creation only. Continue only if free space is sufficient and the snapshot/manifest pass checksum and `integrity_check`.
- **Before C3:** the operator reviews the exact source version, target version, canonical migration checksums, backup path, and recovery procedure.
- **C3:** the operator separately authorizes only that version range. The migration runs while the same lock remains held.

If any condition fails, stop without writes. Unknown/partial shape, checksum drift, version gap, or future version is never overridden in place.

### Additional Stop Gates

- Stop if absolute-path enforcement would silently select a different file; require explicit operator configuration instead.
- Stop if the canonical model fallback cannot be established from current project intent; align docs to the existing code value rather than upgrading speculatively.
- Stop if access paths must be separated or “real use” measured; that requires additive counters and a separately approved data migration because history cannot be reconstructed.
- Stop if quality judgments or thresholds are disputed; review them before freezing, and do not modify retrieval behavior meanwhile.

## 13. Completion checklist and proof

- [ ] Quickstart no longer claims copying `.env` configures the process.
- [ ] Missing and relative DB paths fail before filesystem/SQLite writes.
- [ ] One absolute path preserves identity across CWD changes.
- [ ] Exact read-only classification covers every supported and rejected shape.
- [ ] Ledger versions are consecutive and checksums immutable.
- [ ] Canonical checksum byte fixtures and concurrent-startup locking tests pass.
- [ ] Every migration and ledger record commit atomically.
- [ ] Existing recognized stores receive and validate a unique snapshot before writes.
- [ ] Restore and FTS rebuild are explicit, offline, failure-safe, and tested.
- [ ] Legacy access history is unchanged and exposed only as retrieval frequency/recency.
- [ ] Counting and non-counting MCP paths match the documented contract.
- [ ] Synthetic corpus, judgments, metric tests, aggregate report, and thresholds are versioned.
- [ ] Project leakage is zero and all quality gates pass.
- [ ] Full race-enabled tests and three binaries build.
- [ ] Documentation and Project Memory agree with runtime behavior.
- [ ] The pre/post filesystem-entry guard reports the real database and sidecars unchanged during automated verification without opening their contents.

Completion proof includes RED/GREEN focal output, fixture paths, before/after schema and ledger assertions, snapshot manifest and integrity evidence from synthetic fixtures, restore/rebuild failure evidence, quality reports, and the final verification output.

## 14. Git and delivery expectations

- Implement on a dedicated `codex/` branch.
- Keep commits reviewable by boundary: configuration, migration classifier/ledger, backup/recovery/rebuild, public metrics, quality harness, documentation.
- Never commit databases, snapshots, manifests containing local paths, generated reports, binaries, or temporary overlays.
- Do not begin Phase 3 retrieval changes in this phase.
- After user approval, hand this specification to `implement-project-phase`; crossing the real-data Stop Gate remains a separate explicit decision.
