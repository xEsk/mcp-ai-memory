# Phase 1 Completion Evidence

## Scope and safety

This transcript closes Phase 1 at repository state `fbfce0028a73265c4093427b777305e7d0b6bebf` plus the Phase 1 closeout commit recorded in the final section.

The RED reproductions used source-only trees beneath `/tmp/mcp-ai-memory-phase1-closeout-red`. Each tree came from `git archive HEAD`; only the relevant production-file diff was reversed, while the regression tests stayed unchanged. The trees contained no `.git` directory and no database copied from the repository. All SQLite tests created their own database beneath `t.TempDir()`.

The real-database guard is deliberately a conservative command-argument filter and a pre/post filesystem-entry metadata tripwire. It rejects normalized protected paths, embedded canonical path tokens, and `sh`, `bash`, or `zsh` indirection. It does not claim to be an operating-system audit capable of proving that an arbitrary executable performed no read. The additional source inspection below proves that repository test code uses explicit `t.TempDir()` database paths and does not name `data/memory.db`.

## RED reproductions

### Soft-delete transaction cleanup

Command in the source-only tree with `internal/memory/repository.go` restored to `57ff78b^`:

```bash
go test ./internal/memory -run '^TestRepository_SoftDeleteReleasesSoleConnection$' -count=1 -v
```

Exit status: `1`.

```text
=== RUN   TestRepository_SoftDeleteReleasesSoleConnection
=== RUN   TestRepository_SoftDeleteReleasesSoleConnection/missing_memory
    repository_soft_delete_test.go:82: connections in use after soft delete = 1, want 0
    repository_soft_delete_test.go:85: follow-up ping after soft delete: context deadline exceeded
=== RUN   TestRepository_SoftDeleteReleasesSoleConnection/repeated_delete
    repository_soft_delete_test.go:82: connections in use after soft delete = 1, want 0
    repository_soft_delete_test.go:85: follow-up ping after soft delete: context deadline exceeded
=== RUN   TestRepository_SoftDeleteReleasesSoleConnection/wrong_project
    repository_soft_delete_test.go:82: connections in use after soft delete = 1, want 0
    repository_soft_delete_test.go:85: follow-up ping after soft delete: context deadline exceeded
--- FAIL: TestRepository_SoftDeleteReleasesSoleConnection (0.77s)
FAIL
FAIL mcp-ai-memory/internal/memory 0.974s
FAIL
```

### FTS5 BM25 positional mapping

Command in the source-only tree with `internal/search/fts.go` restored to `940f5aa^`:

```bash
go test ./internal/search -run '^TestFTS_SearchUsesDocumentedBM25FieldOrder$' -count=1 -v
```

Exit status: `1`.

```text
=== RUN   TestFTS_SearchUsesDocumentedBM25FieldOrder
    fts_test.go:155: keywords score 1.6382978723404257e-06 should rank ahead of tags score 1.6382978723404257e-06
    fts_test.go:146: tags score 1.6382978723404257e-06 and entities score 1.5714285714285712e-06 should have equal rank
    fts_test.go:146: tags score 1.6382978723404257e-06 and aliases score 1.2222222222222223e-06 should have equal rank
    fts_test.go:155: search_phrases score 1e-06 should rank ahead of content score 1e-06
    fts_test.go:155: content score 1e-06 should rank ahead of title score 1.6923076923076922e-06
--- FAIL: TestFTS_SearchUsesDocumentedBM25FieldOrder (0.03s)
FAIL
FAIL mcp-ai-memory/internal/search 0.396s
FAIL
```

### Explicit trace consent

Command in the source-only tree with the production files from `412bc4a` restored to `412bc4a^`:

```bash
go test ./internal/app -run 'Trace' -count=1 -v
```

Exit status: `1`.

```text
# mcp-ai-memory/internal/app [mcp-ai-memory/internal/app.test]
internal/app/config_test.go:11:2: "mcp-ai-memory/internal/trace" imported and not used
internal/app/config_test.go:20:9: cfg.TraceMode undefined (type Config has no field or method TraceMode)
internal/app/config_test.go:20:22: undefined: TraceModeOff
internal/app/config_test.go:26:15: assignment mismatch: 2 variables but buildTraceSink returns 1 value
internal/app/config_test.go:27:3: unknown field TraceMode in struct literal of type Config
internal/app/config_test.go:42:3: unknown field TraceMode in struct literal of type Config
FAIL mcp-ai-memory/internal/app [build failed]
FAIL
```

### Guard indirection closeout

Command after adding the closeout regression tests but before hardening the guard:

```bash
./scripts/verify-real-db-untouched_test.sh
```

Exit status: `1`.

```text
guard accepted an embedded protected path
```

## GREEN focal evidence

### Soft delete

```bash
go test ./internal/memory -run '^TestRepository_SoftDelete(ReleasesSoleConnection|CommitsMemoryAndFTSStateTogether|RollsBackMemoryWhenFTSDeleteFails)$' -count=1 -v
```

Exit status: `0`.

```text
--- PASS: TestRepository_SoftDeleteReleasesSoleConnection (0.01s)
    --- PASS: TestRepository_SoftDeleteReleasesSoleConnection/missing_memory (0.01s)
    --- PASS: TestRepository_SoftDeleteReleasesSoleConnection/repeated_delete (0.00s)
    --- PASS: TestRepository_SoftDeleteReleasesSoleConnection/wrong_project (0.00s)
--- PASS: TestRepository_SoftDeleteCommitsMemoryAndFTSStateTogether (0.00s)
--- PASS: TestRepository_SoftDeleteRollsBackMemoryWhenFTSDeleteFails (0.00s)
PASS
ok mcp-ai-memory/internal/memory 0.211s
```

### BM25 mapping

```bash
go test ./internal/search -run '^TestFTS_SearchUsesDocumentedBM25FieldOrder$' -count=1 -v
```

Exit status: `0`.

```text
=== RUN   TestFTS_SearchUsesDocumentedBM25FieldOrder
--- PASS: TestFTS_SearchUsesDocumentedBM25FieldOrder (0.03s)
PASS
ok mcp-ai-memory/internal/search 0.390s
```

### Trace consent

```bash
go test ./internal/app -run 'Trace' -count=1 -v
```

Exit status: `0`.

```text
=== RUN   TestLoadConfigTraceDefaultsOffEvenWithMonitorURL
--- PASS: TestLoadConfigTraceDefaultsOffEvenWithMonitorURL (0.00s)
=== RUN   TestBuildTraceSinkMonitorURLAloneDoesNotEnableTracing
--- PASS: TestBuildTraceSinkMonitorURLAloneDoesNotEnableTracing (0.00s)
=== RUN   TestNewRejectsUnknownTraceModeAtStartup
--- PASS: TestNewRejectsUnknownTraceModeAtStartup (0.00s)
PASS
ok mcp-ai-memory/internal/app 0.344s
```

### Guard hardening

```bash
./scripts/verify-real-db-untouched_test.sh
```

Exit status: `0`.

```text
verify-real-db-untouched guard tests passed
```

## Temporary database source proof

Command:

```bash
rg -n 't\.TempDir\(\).*memory\.db' --glob '*_test.go' .
```

Exit status: `0`.

```text
./internal/mcpserver/tools_test.go:379: db, err := storage.OpenSQLite(ctx, filepath.Join(t.TempDir(), "memory.db"))
./internal/search/fts_test.go:18: db, err := storage.OpenSQLite(ctx, filepath.Join(t.TempDir(), "memory.db"))
./internal/memory/memory_integration_test.go:18: db, err := storage.OpenSQLite(ctx, filepath.Join(t.TempDir(), "memory.db"))
./internal/memory/memory_integration_test.go:141: db, err := storage.OpenSQLite(ctx, filepath.Join(t.TempDir(), "memory.db"))
./internal/memory/repository_soft_delete_test.go:162: db, err := storage.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "memory.db"))
./internal/app/config_test.go:39: dbPath := filepath.Join(t.TempDir(), "memory.db")
```

Command:

```bash
rg -n 'data/memory\.db' --glob '*_test.go' .
```

Exit status: `1`, meaning no test source matched the protected path.

## Final verification

The first restricted-sandbox run reached the `httptest` packages but could not bind loopback sockets (`listen tcp6 [::1]:0: bind: operation not permitted`). The exact command was rerun with local loopback binding allowed; no code was changed in response to that environmental failure.

```bash
./scripts/verify-real-db-untouched_test.sh
```

Exit status: `0`.

```text
verify-real-db-untouched guard tests passed
```

```bash
./scripts/verify-real-db-untouched.sh -- go test -race -count=1 ./...
```

Exit status: `0`.

```text
?  mcp-ai-memory/cmd/memory-mcp [no test files]
?  mcp-ai-memory/cmd/memory-monitor [no test files]
ok mcp-ai-memory/internal/app 1.284s
ok mcp-ai-memory/internal/enrich 2.083s
ok mcp-ai-memory/internal/mcpserver 1.862s
ok mcp-ai-memory/internal/memory 1.702s
ok mcp-ai-memory/internal/monitor 1.785s
ok mcp-ai-memory/internal/search 2.221s
ok mcp-ai-memory/internal/trace 2.310s
real database filesystem metadata unchanged
```

The remaining packages reported either `[no test files]` or exited successfully.

```bash
go vet ./...
go build -o /tmp/mcp-ai-memory-phase1-closeout-mcp ./cmd/memory-mcp
go build -o /tmp/mcp-ai-memory-phase1-closeout-monitor ./cmd/memory-monitor
git diff --check
```

Each command exited `0` with no diagnostic output. The final accepted Phase 1 implementation baseline is recorded in the Phase 1 specification and the Phase 2 handoff after the closeout commit is created.
