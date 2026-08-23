package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenSQLiteBootstrapsVersionedSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")

	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var version int
	var name, checksum string
	if err := db.QueryRowContext(ctx, `SELECT version, name, checksum FROM schema_migrations`).Scan(&version, &name, &checksum); err != nil {
		t.Fatalf("read migration ledger: %v", err)
	}
	if version != 1 || name != "initial_schema" || checksum != migrationChecksum(migrations[0]) {
		t.Fatalf("migration ledger = (%d, %q, %q), want initial migration", version, name, checksum)
	}
}

func TestOpenSQLiteRejectsUnknownSchemaWithoutWrites(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE unexpected (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create fixture table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}

	opened, err := OpenSQLite(ctx, path)
	if err == nil {
		_ = opened.Close()
		t.Fatal("opening unknown schema succeeded")
	}
	if !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("open error = %v, want ErrUnsupportedSchema", err)
	}

	check, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		t.Fatalf("open fixture read-only: %v", err)
	}
	t.Cleanup(func() { _ = check.Close() })
	var count int
	if err := check.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&count); err != nil {
		t.Fatalf("check ledger table: %v", err)
	}
	if count != 0 {
		t.Fatalf("unknown schema gained %d migration ledger tables", count)
	}
}

func TestOpenSQLiteRejectsSchemaWithExpectedNamesButWrongDefinition(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open current schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close current schema: %v", err)
	}

	fixture, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open malformed fixture: %v", err)
	}
	if _, err := fixture.ExecContext(ctx, `DROP TABLE projects`); err != nil {
		t.Fatalf("drop projects table: %v", err)
	}
	if _, err := fixture.ExecContext(ctx, `CREATE TABLE projects (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create malformed projects table: %v", err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatalf("close malformed fixture: %v", err)
	}

	opened, err := OpenSQLite(ctx, path)
	if err == nil {
		_ = opened.Close()
		t.Fatal("opening malformed current schema succeeded")
	}
	if !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("open error = %v, want ErrUnsupportedSchema", err)
	}
}

func TestOpenSQLiteRejectsSchemaWithExpectedColumnsButWrongSignature(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open current schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close current schema: %v", err)
	}

	fixture, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open malformed fixture: %v", err)
	}
	if _, err := fixture.ExecContext(ctx, `DROP TABLE projects`); err != nil {
		t.Fatalf("drop projects table: %v", err)
	}
	if _, err := fixture.ExecContext(ctx, `CREATE TABLE projects (
		id TEXT PRIMARY KEY,
		key TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		description INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create wrong-signature projects table: %v", err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatalf("close malformed fixture: %v", err)
	}

	opened, err := OpenSQLite(ctx, path)
	if err == nil {
		_ = opened.Close()
		t.Fatal("opening wrong-signature schema succeeded")
	}
	if !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("open error = %v, want ErrUnsupportedSchema", err)
	}
}

func TestOpenSQLiteRejectsSchemaWithExpectedColumnsButMissingConstraint(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open current schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close current schema: %v", err)
	}

	fixture, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open malformed fixture: %v", err)
	}
	if _, err := fixture.ExecContext(ctx, `DROP TABLE projects`); err != nil {
		t.Fatalf("drop projects table: %v", err)
	}
	if _, err := fixture.ExecContext(ctx, `CREATE TABLE projects (
		id TEXT PRIMARY KEY,
		key TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create unconstrained projects table: %v", err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatalf("close malformed fixture: %v", err)
	}

	opened, err := OpenSQLite(ctx, path)
	if err == nil {
		_ = opened.Close()
		t.Fatal("opening schema without unique key constraint succeeded")
	}
	if !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("open error = %v, want ErrUnsupportedSchema", err)
	}
}

func TestOpenSQLiteRejectsUnexpectedSchemaObject(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open current schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close current schema: %v", err)
	}
	fixture, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if _, err := fixture.ExecContext(ctx, `CREATE TABLE unexpected (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create unexpected table: %v", err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}

	opened, err := OpenSQLite(ctx, path)
	if err == nil {
		_ = opened.Close()
		t.Fatal("opening schema with unexpected object succeeded")
	}
	if !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("open error = %v, want ErrUnsupportedSchema", err)
	}
}

func TestOpenSQLiteRecoversFromStaleMigrationLockFile(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	if err := os.WriteFile(path+".migrate.lock", []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale migration lock: %v", err)
	}

	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open with stale migration lock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
}

func TestMaintenanceLockRejectsActiveRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	runtimeLock, err := AcquireRuntimeLock(path)
	if err != nil {
		t.Fatalf("acquire runtime lock: %v", err)
	}
	t.Cleanup(func() { _ = runtimeLock.Close() })

	maintenanceLock, err := AcquireMaintenanceLock(path)
	if err == nil {
		_ = maintenanceLock.Close()
		t.Fatal("maintenance lock succeeded while runtime was active")
	}
}

func TestOperationLockAllowsRecoveryFromStaleLockFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	if err := os.WriteFile(path+".operation.lock", []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale lock file: %v", err)
	}

	lock, err := AcquireRuntimeLock(path)
	if err != nil {
		t.Fatalf("acquire runtime lock with stale file: %v", err)
	}
	t.Cleanup(func() { _ = lock.Close() })
}

func TestCreateBackupProducesReadableSnapshot(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, key, name, created_at, updated_at) VALUES ('project', 'backup', 'Backup', 'now', 'now')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	backup, err := CreateBackup(ctx, db, path)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if backup == path {
		t.Fatal("backup path equals source path")
	}
	readOnly, err := sql.Open("sqlite", backup+"?mode=ro")
	if err != nil {
		t.Fatalf("open backup read-only: %v", err)
	}
	t.Cleanup(func() { _ = readOnly.Close() })
	var count int
	if err := readOnly.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE key = 'backup'`).Scan(&count); err != nil {
		t.Fatalf("read backup project: %v", err)
	}
	if count != 1 {
		t.Fatalf("backup project count = %d, want 1", count)
	}
}

func TestWriteBackupManifestRejectsExistingFileAndSymlink(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func(*testing.T, string) string
	}{
		{
			name: "existing file",
			prepare: func(t *testing.T, manifestPath string) string {
				t.Helper()
				if err := os.WriteFile(manifestPath, []byte("sentinel"), 0o600); err != nil {
					t.Fatalf("write existing manifest: %v", err)
				}
				return manifestPath
			},
		},
		{
			name: "symlink",
			prepare: func(t *testing.T, manifestPath string) string {
				t.Helper()
				target := filepath.Join(t.TempDir(), "target")
				if err := os.WriteFile(target, []byte("sentinel"), 0o600); err != nil {
					t.Fatalf("write symlink target: %v", err)
				}
				if err := os.Symlink(target, manifestPath); err != nil {
					t.Fatalf("create manifest symlink: %v", err)
				}
				return target
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			backupPath := filepath.Join(t.TempDir(), "snapshot.db")
			if err := os.WriteFile(backupPath, []byte("snapshot"), 0o600); err != nil {
				t.Fatalf("write snapshot: %v", err)
			}
			protectedPath := test.prepare(t, backupPath+".manifest")

			if err := writeBackupManifest(backupPath); err == nil {
				t.Fatal("manifest creation replaced an existing path")
			}
			content, err := os.ReadFile(protectedPath)
			if err != nil {
				t.Fatalf("read protected path: %v", err)
			}
			if string(content) != "sentinel" {
				t.Fatalf("protected content = %q, want sentinel", content)
			}
		})
	}
}

func TestBackupDatabaseRejectsMissingStoreWithoutCreatingIt(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "missing")
	path := filepath.Join(parent, "memory.db")

	if _, err := BackupDatabase(context.Background(), path); err == nil {
		t.Fatal("backup succeeded for a missing store")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(parent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database parent stat error = %v, want not exist", err)
	}
}

func TestRebuildDatabaseFTSRejectsMissingStoreWithoutCreatingIt(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "missing")
	path := filepath.Join(parent, "memory.db")

	if err := RebuildDatabaseFTS(context.Background(), path); err == nil {
		t.Fatal("FTS rebuild succeeded for a missing store")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(parent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database parent stat error = %v, want not exist", err)
	}
}

func TestBackupDatabaseDoesNotAdoptRecognizedUnversionedStore(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open current store: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE schema_migrations`); err != nil {
		t.Fatalf("remove migration ledger: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close unversioned store: %v", err)
	}

	backup, err := BackupDatabase(ctx, path)
	if err != nil {
		t.Fatalf("backup unversioned store: %v", err)
	}
	if _, err := os.Stat(backup + ".manifest"); err != nil {
		t.Fatalf("backup manifest: %v", err)
	}

	readOnly, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		t.Fatalf("open source read-only: %v", err)
	}
	defer readOnly.Close()
	var ledgerCount int
	if err := readOnly.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&ledgerCount); err != nil {
		t.Fatalf("count source ledger: %v", err)
	}
	if ledgerCount != 0 {
		t.Fatalf("source gained %d migration ledgers during backup", ledgerCount)
	}
}

func TestRebuildFTSRestoresOnlyActiveAuthoritativeMemories(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, statement := range []string{
		`INSERT INTO projects (id, key, name, created_at, updated_at) VALUES ('project', 'project-key', 'Project', 'now', 'now')`,
		`INSERT INTO memories (id, project_id, type, title, content, created_at, updated_at) VALUES ('active', 'project', 'implementation', 'Active', 'active content', 'now', 'now')`,
		`INSERT INTO memories (id, project_id, type, title, content, created_at, updated_at, deleted_at) VALUES ('deleted', 'project', 'implementation', 'Deleted', 'deleted content', 'now', 'now', 'now')`,
		`INSERT INTO memory_enrichments (memory_id, updated_at) VALUES ('active', 'now')`,
		`INSERT INTO memory_enrichments (memory_id, updated_at) VALUES ('deleted', 'now')`,
		`INSERT INTO memory_fts (memory_id, project_key, type, title, summary, short_summary, keywords, tags, entities, aliases, search_phrases, content) VALUES ('orphan', 'wrong', '', '', '', '', '', '', '', '', '', '')`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed fixture: %v", err)
		}
	}

	if err := RebuildFTS(ctx, db); err != nil {
		t.Fatalf("rebuild fts: %v", err)
	}
	var memoryID, projectKey string
	if err := db.QueryRowContext(ctx, `SELECT memory_id, project_key FROM memory_fts`).Scan(&memoryID, &projectKey); err != nil {
		t.Fatalf("read rebuilt fts: %v", err)
	}
	if memoryID != "active" || projectKey != "project-key" {
		t.Fatalf("rebuilt fts row = (%q, %q), want active project row", memoryID, projectKey)
	}
}

func TestRestoreBackupQuarantinesDatabaseSidecars(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, key, name, created_at, updated_at) VALUES ('project', 'restore', 'Restore', 'now', 'now')`); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	backup, err := CreateBackup(ctx, db, path)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close source: %v", err)
	}
	for _, sidecar := range []string{path + "-wal", path + "-shm"} {
		if err := os.WriteFile(sidecar, []byte("synthetic sidecar"), 0o600); err != nil {
			t.Fatalf("create sidecar: %v", err)
		}
	}

	quarantine, err := RestoreBackup(ctx, path, backup)
	if err != nil {
		t.Fatalf("restore backup: %v", err)
	}
	for _, sidecar := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(quarantine + sidecar); err != nil {
			t.Fatalf("quarantined sidecar %q: %v", sidecar, err)
		}
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup was not preserved: %v", err)
	}
}

func TestRestoreBackupRejectsMissingManifestWithoutChangingSource(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, key, name, created_at, updated_at) VALUES ('source', 'source', 'Source', 'now', 'now')`); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	backup, err := CreateBackup(ctx, db, path)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close source: %v", err)
	}
	if err := os.Remove(backup + ".manifest"); err != nil {
		t.Fatalf("remove manifest: %v", err)
	}

	if _, err := RestoreBackup(ctx, path, backup); err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("restore error = %v, want missing manifest error", err)
	}
	assertProjectExists(t, ctx, path, "source")
}

func TestRestoreBackupRejectsValidSQLiteWithManifestChecksumDrift(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, key, name, created_at, updated_at) VALUES ('source', 'source', 'Source', 'now', 'now')`); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	backup, err := CreateBackup(ctx, db, path)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close source: %v", err)
	}

	tampered, err := sql.Open("sqlite", backup)
	if err != nil {
		t.Fatalf("open backup for tampering: %v", err)
	}
	if _, err := tampered.ExecContext(ctx, `INSERT INTO projects (id, key, name, created_at, updated_at) VALUES ('tampered', 'tampered', 'Tampered', 'now', 'now')`); err != nil {
		t.Fatalf("tamper valid backup: %v", err)
	}
	if err := tampered.Close(); err != nil {
		t.Fatalf("close tampered backup: %v", err)
	}

	if _, err := RestoreBackup(ctx, path, backup); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("restore error = %v, want checksum error", err)
	}
	assertProjectExists(t, ctx, path, "source")
}

func TestRestoreBackupRollsBackWhenFinalValidationFails(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, key, name, created_at, updated_at) VALUES ('before', 'before', 'Before', 'now', 'now')`); err != nil {
		t.Fatalf("seed backup state: %v", err)
	}
	backup, err := CreateBackup(ctx, db, path)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, key, name, created_at, updated_at) VALUES ('after', 'after', 'After', 'now', 'now')`); err != nil {
		t.Fatalf("seed source state: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close source: %v", err)
	}

	originalVerifier := verifySnapshotFile
	t.Cleanup(func() { verifySnapshotFile = originalVerifier })
	calls := 0
	verifySnapshotFile = func(ctx context.Context, snapshotPath string) error {
		calls++
		if calls == 2 {
			return errors.New("injected restored snapshot validation failure")
		}
		return originalVerifier(ctx, snapshotPath)
	}
	if _, err := RestoreBackup(ctx, path, backup); err == nil {
		t.Fatal("restore succeeded despite final validation failure")
	}

	restored, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		t.Fatalf("open rolled-back source: %v", err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	var count int
	if err := restored.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE key = 'after'`).Scan(&count); err != nil {
		t.Fatalf("read rolled-back source: %v", err)
	}
	if count != 1 {
		t.Fatalf("rolled-back source count = %d, want 1", count)
	}
}

func TestRestoreBackupRollsBackPartialSidecarQuarantine(t *testing.T) {
	for _, failedSuffix := range []string{"-wal", "-shm"} {
		t.Run(failedSuffix, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "memory.db")
			db, err := OpenSQLite(ctx, path)
			if err != nil {
				t.Fatalf("open sqlite: %v", err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, key, name, created_at, updated_at) VALUES ('source', 'source', 'Source', 'now', 'now')`); err != nil {
				t.Fatalf("seed source: %v", err)
			}
			backup, err := CreateBackup(ctx, db, path)
			if err != nil {
				t.Fatalf("create backup: %v", err)
			}
			if err := db.Close(); err != nil {
				t.Fatalf("close source: %v", err)
			}
			for _, suffix := range []string{"-wal", "-shm"} {
				if err := os.WriteFile(path+suffix, []byte("source"+suffix), 0o600); err != nil {
					t.Fatalf("write sidecar %s: %v", suffix, err)
				}
			}

			originalRename := renameMaintenanceFile
			t.Cleanup(func() { renameMaintenanceFile = originalRename })
			renameMaintenanceFile = func(oldPath, newPath string) error {
				if oldPath == path+failedSuffix {
					return errors.New("injected sidecar move failure")
				}
				return originalRename(oldPath, newPath)
			}

			if _, err := RestoreBackup(ctx, path, backup); err == nil {
				t.Fatal("restore succeeded despite sidecar move failure")
			}
			for _, suffix := range []string{"-wal", "-shm"} {
				content, err := os.ReadFile(path + suffix)
				if err != nil {
					t.Fatalf("read restored sidecar %s: %v", suffix, err)
				}
				if string(content) != "source"+suffix {
					t.Fatalf("sidecar %s content = %q", suffix, content)
				}
			}
			assertProjectExists(t, ctx, path, "source")
		})
	}
}

func TestRestoreBackupRemovesFailedInstallWithoutOriginalSource(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "source.db")
	db, err := OpenSQLite(ctx, sourcePath)
	if err != nil {
		t.Fatalf("open backup source: %v", err)
	}
	backup, err := CreateBackup(ctx, db, sourcePath)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close backup source: %v", err)
	}
	destinationPath := filepath.Join(directory, "memory.db")
	originalVerifier := verifySnapshotFile
	t.Cleanup(func() { verifySnapshotFile = originalVerifier })
	calls := 0
	verifySnapshotFile = func(ctx context.Context, snapshotPath string) error {
		calls++
		if calls == 2 {
			return errors.New("injected final validation failure")
		}
		return originalVerifier(ctx, snapshotPath)
	}

	if _, err := RestoreBackup(ctx, destinationPath, backup); err == nil {
		t.Fatal("restore succeeded despite final validation failure")
	}
	if _, err := os.Stat(destinationPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed destination stat error = %v, want not exist", err)
	}
}

func TestRestoreBackupReportsRollbackFailureAndPreservesQuarantine(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	backup, err := CreateBackup(ctx, db, path)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close source: %v", err)
	}
	originalVerifier := verifySnapshotFile
	originalRename := renameMaintenanceFile
	t.Cleanup(func() {
		verifySnapshotFile = originalVerifier
		renameMaintenanceFile = originalRename
	})
	calls := 0
	verifySnapshotFile = func(ctx context.Context, snapshotPath string) error {
		calls++
		if calls == 2 {
			return errors.New("injected final validation failure")
		}
		return originalVerifier(ctx, snapshotPath)
	}
	renameMaintenanceFile = func(oldPath, newPath string) error {
		if strings.Contains(oldPath, ".quarantine-") && newPath == path {
			return errors.New("injected rollback rename failure")
		}
		return originalRename(oldPath, newPath)
	}

	_, err = RestoreBackup(ctx, path, backup)
	if err == nil || !strings.Contains(err.Error(), "restoring quarantined artifact") {
		t.Fatalf("restore error = %v, want reported rollback failure", err)
	}
	quarantines, globErr := filepath.Glob(path + ".quarantine-*")
	if globErr != nil {
		t.Fatalf("find quarantine: %v", globErr)
	}
	if len(quarantines) != 1 {
		t.Fatalf("quarantines = %v, want one preserved source", quarantines)
	}
}

func assertProjectExists(t *testing.T, ctx context.Context, path, key string) {
	t.Helper()
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		t.Fatalf("open database read-only: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE key = ?`, key).Scan(&count); err != nil {
		t.Fatalf("count project %q: %v", key, err)
	}
	if count != 1 {
		t.Fatalf("project %q count = %d, want 1", key, count)
	}
}
