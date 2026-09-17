package project

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"mcp-ai-memory/internal/storage"
)

func TestRepositorySoftDeleteAndRestoreProject(t *testing.T) {
	ctx := context.Background()
	db := openManagementTestDB(t)
	repo := NewRepository(db)
	p := Project{ID: "project", Key: "demo", Name: "Demo", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if _, err := repo.Upsert(ctx, p); err != nil {
		t.Fatalf("upsert project: %v", err)
	}
	if err := repo.SoftDelete(ctx, p.Key, time.Unix(2, 0)); err != nil {
		t.Fatalf("soft delete project: %v", err)
	}
	if err := repo.SoftDelete(ctx, p.Key, time.Unix(2, 0)); !errors.Is(err, ErrProjectAlreadyDeleted) {
		t.Fatalf("second delete error = %v, want ErrProjectAlreadyDeleted", err)
	}
	if _, err := repo.GetByKey(ctx, p.Key); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get deleted project error = %v, want sql.ErrNoRows", err)
	}
	projects, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list active projects: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("active projects = %d, want 0", len(projects))
	}
	all, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("list all projects: %v", err)
	}
	if len(all) != 1 || all[0].DeletedAt == nil {
		t.Fatalf("all projects = %#v, want one deleted project", all)
	}
	if _, err := repo.Upsert(ctx, p); !errors.Is(err, ErrProjectDeleted) {
		t.Fatalf("upsert deleted project error = %v, want ErrProjectDeleted", err)
	}
	if err := repo.Restore(ctx, p.Key, time.Unix(3, 0)); err != nil {
		t.Fatalf("restore project: %v", err)
	}
	if err := repo.Restore(ctx, p.Key, time.Unix(3, 0)); !errors.Is(err, ErrProjectNotDeleted) {
		t.Fatalf("second restore error = %v, want ErrProjectNotDeleted", err)
	}
	if _, err := repo.GetByKey(ctx, p.Key); err != nil {
		t.Fatalf("get restored project: %v", err)
	}
}

func TestRepositoryCleanupPurgesDeletedStateAndStaleDerivedRows(t *testing.T) {
	ctx := context.Background()
	db := openManagementTestDB(t)
	repo := NewRepository(db)
	seedCleanupFixture(t, db)

	preview, err := repo.PreviewCleanup(ctx)
	if err != nil {
		t.Fatalf("preview cleanup: %v", err)
	}
	if preview.DeletedProjects != 1 || preview.DeletedProjectMemories != 2 || preview.DeletedMemories != 1 || preview.StaleFTS != 5 || preview.OrphanedEnrichments != 1 {
		t.Fatalf("cleanup preview = %#v", preview)
	}
	cleaned, err := repo.Cleanup(ctx)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if cleaned != preview {
		t.Fatalf("cleanup result = %#v, want %#v", cleaned, preview)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM projects WHERE key = 'deleted'`, 0)
	assertCount(t, db, `SELECT COUNT(*) FROM memories WHERE id IN ('deleted-project-active', 'deleted-project-deleted', 'active-project-deleted')`, 0)
	assertCount(t, db, `SELECT COUNT(*) FROM memory_enrichments WHERE memory_id IN ('deleted-project-active', 'deleted-project-deleted', 'active-project-deleted')`, 0)
	assertCount(t, db, `SELECT COUNT(*) FROM memory_enrichments WHERE memory_id = 'orphan-enrichment'`, 0)
	assertCount(t, db, `SELECT COUNT(*) FROM memory_fts`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM memories WHERE id = 'active-project-active'`, 1)
}

func TestRepositoryCleanupRollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	db := openManagementTestDB(t)
	repo := NewRepository(db)
	seedCleanupFixture(t, db)
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER fail_project_cleanup BEFORE DELETE ON projects BEGIN SELECT RAISE(ABORT, 'forced cleanup failure'); END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	if _, err := repo.Cleanup(ctx); err == nil {
		t.Fatal("cleanup succeeded despite forced project failure")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM projects WHERE key = 'deleted'`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM memories`, 4)
	assertCount(t, db, `SELECT COUNT(*) FROM memory_fts`, 6)
	assertCount(t, db, `SELECT COUNT(*) FROM memory_enrichments`, 5)
}

func openManagementTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := storage.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedCleanupFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	statements := []string{
		`INSERT INTO projects (id, key, name, created_at, updated_at, deleted_at) VALUES ('active-project', 'active', 'Active', 'now', 'now', NULL)`,
		`INSERT INTO projects (id, key, name, created_at, updated_at, deleted_at) VALUES ('deleted-project', 'deleted', 'Deleted', 'now', 'now', 'now')`,
		`INSERT INTO memories (id, project_id, type, title, content, created_at, updated_at) VALUES ('active-project-active', 'active-project', 'implementation', 'Active', 'active', 'now', 'now')`,
		`INSERT INTO memories (id, project_id, type, title, content, created_at, updated_at, deleted_at) VALUES ('active-project-deleted', 'active-project', 'implementation', 'Deleted', 'deleted', 'now', 'now', 'now')`,
		`INSERT INTO memories (id, project_id, type, title, content, created_at, updated_at) VALUES ('deleted-project-active', 'deleted-project', 'implementation', 'Active under deleted', 'content', 'now', 'now')`,
		`INSERT INTO memories (id, project_id, type, title, content, created_at, updated_at, deleted_at) VALUES ('deleted-project-deleted', 'deleted-project', 'implementation', 'Deleted under deleted', 'content', 'now', 'now', 'now')`,
	}
	for _, id := range []string{"active-project-active", "active-project-deleted", "deleted-project-active", "deleted-project-deleted"} {
		statements = append(statements,
			`INSERT INTO memory_enrichments (memory_id, updated_at) VALUES ('`+id+`', 'now')`,
			`INSERT INTO memory_fts (memory_id, project_key) VALUES ('`+id+`', CASE WHEN '`+id+`' LIKE 'deleted-project%' THEN 'deleted' ELSE 'active' END)`,
		)
	}
	statements = append(statements,
		`INSERT INTO memory_fts (memory_id, project_key) VALUES ('orphan', 'missing')`,
		`INSERT INTO memory_fts (memory_id, project_key) VALUES ('active-project-active', 'wrong-project')`,
	)
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed cleanup fixture: %v", err)
		}
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("disable fixture foreign keys: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO memory_enrichments (memory_id, updated_at) VALUES ('orphan-enrichment', 'now')`); err != nil {
		t.Fatalf("seed orphan enrichment: %v", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("restore fixture foreign keys: %v", err)
	}
}

func assertCount(t *testing.T, db *sql.DB, query string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if got != want {
		t.Fatalf("count for %q = %d, want %d", query, got, want)
	}
}
