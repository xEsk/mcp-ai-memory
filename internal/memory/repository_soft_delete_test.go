package memory_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/platform"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/storage"
)

func TestRepository_SoftDeleteReleasesSoleConnection(t *testing.T) {
	tests := []struct {
		name      string
		projectID func(primaryProjectID, otherProjectID string) string
		memoryID  func(memoryID string) string
		prepare   func(t *testing.T, repo *memory.Repository, projectID, memoryID string)
	}{
		{
			name: "missing memory",
			projectID: func(primaryProjectID, _ string) string {
				return primaryProjectID
			},
			memoryID: func(string) string {
				return "missing-memory"
			},
		},
		{
			name: "repeated delete",
			projectID: func(primaryProjectID, _ string) string {
				return primaryProjectID
			},
			memoryID: func(memoryID string) string {
				return memoryID
			},
			prepare: func(t *testing.T, repo *memory.Repository, projectID, memoryID string) {
				t.Helper()
				if err := repo.SoftDelete(context.Background(), projectID, memoryID, time.Unix(2, 0)); err != nil {
					t.Fatalf("first soft delete: %v", err)
				}
			},
		},
		{
			name: "wrong project",
			projectID: func(_, otherProjectID string) string {
				return otherProjectID
			},
			memoryID: func(memoryID string) string {
				return memoryID
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, repo, primaryProjectID, otherProjectID, storedMemoryID := newSoftDeleteRepository(t)
			projectID := tt.projectID(primaryProjectID, otherProjectID)
			memoryID := tt.memoryID(storedMemoryID)
			if tt.prepare != nil {
				tt.prepare(t, repo, projectID, memoryID)
			}

			deleteCtx, cancelDelete := context.WithCancel(context.Background())
			err := repo.SoftDelete(deleteCtx, projectID, memoryID, time.Unix(3, 0))
			if !errors.Is(err, memory.ErrNotFound) {
				cancelDelete()
				t.Fatalf("soft delete error = %v, want %v", err, memory.ErrNotFound)
			}

			inUse := db.Stats().InUse
			followUpCtx, cancelFollowUp := context.WithTimeout(context.Background(), 250*time.Millisecond)
			followUpErr := db.PingContext(followUpCtx)
			cancelFollowUp()
			cancelDelete()

			if inUse != 0 {
				t.Errorf("connections in use after soft delete = %d, want 0", inUse)
			}
			if followUpErr != nil {
				t.Errorf("follow-up ping after soft delete: %v", followUpErr)
			}
		})
	}
}

func TestRepository_SoftDeleteCommitsMemoryAndFTSStateTogether(t *testing.T) {
	db, repo, projectID, _, memoryID := newSoftDeleteRepository(t)

	if err := repo.SoftDelete(context.Background(), projectID, memoryID, time.Unix(2, 0)); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	var deletedRows, ftsRows int
	if err := db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM memories WHERE id = ? AND project_id = ? AND deleted_at IS NOT NULL),
			(SELECT COUNT(*) FROM memory_fts WHERE memory_id = ?)
	`, memoryID, projectID, memoryID).Scan(&deletedRows, &ftsRows); err != nil {
		t.Fatalf("query committed delete state: %v", err)
	}
	if deletedRows != 1 {
		t.Errorf("soft-deleted source rows = %d, want 1", deletedRows)
	}
	if ftsRows != 0 {
		t.Errorf("fts rows after soft delete = %d, want 0", ftsRows)
	}
	if inUse := db.Stats().InUse; inUse != 0 {
		t.Errorf("connections in use after committed soft delete = %d, want 0", inUse)
	}
}

func TestRepository_SoftDeleteRollsBackMemoryWhenFTSDeleteFails(t *testing.T) {
	db, repo, projectID, _, memoryID := newSoftDeleteRepository(t)

	statements := []string{
		`DROP TABLE memory_fts`,
		`CREATE TABLE memory_fts (memory_id TEXT NOT NULL, project_key TEXT NOT NULL)`,
		`INSERT INTO memory_fts (memory_id, project_key) VALUES ('stored-memory', 'primary')`,
		`CREATE TRIGGER reject_fts_delete BEFORE DELETE ON memory_fts
		BEGIN
			SELECT RAISE(ABORT, 'forced fts delete failure');
		END`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("prepare forced fts failure: %v", err)
		}
	}

	err := repo.SoftDelete(context.Background(), projectID, memoryID, time.Unix(2, 0))
	if err == nil {
		t.Fatal("soft delete error = nil, want forced fts delete failure")
	}

	var activeRows, ftsRows int
	if err := db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM memories WHERE id = ? AND project_id = ? AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM memory_fts WHERE memory_id = ?)
	`, memoryID, projectID, memoryID).Scan(&activeRows, &ftsRows); err != nil {
		t.Fatalf("query rolled-back delete state: %v", err)
	}
	if activeRows != 1 {
		t.Errorf("active source rows after fts failure = %d, want 1", activeRows)
	}
	if ftsRows != 1 {
		t.Errorf("fts rows after fts failure = %d, want 1", ftsRows)
	}
	if inUse := db.Stats().InUse; inUse != 0 {
		t.Errorf("connections in use after rolled-back soft delete = %d, want 0", inUse)
	}
}

func newSoftDeleteRepository(t *testing.T) (*sql.DB, *memory.Repository, string, string, string) {
	t.Helper()

	db, err := storage.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close sqlite: %v", err)
		}
	})
	if maxOpen := db.Stats().MaxOpenConnections; maxOpen != 1 {
		t.Fatalf("max open connections = %d, want 1", maxOpen)
	}

	clock := platform.NewClock()
	projects := project.NewService(project.NewRepository(db), clock)
	primaryProject, err := projects.Upsert(context.Background(), project.UpsertInput{Key: "primary", Name: "Primary"})
	if err != nil {
		t.Fatalf("upsert primary project: %v", err)
	}
	otherProject, err := projects.Upsert(context.Background(), project.UpsertInput{Key: "other", Name: "Other"})
	if err != nil {
		t.Fatalf("upsert other project: %v", err)
	}

	repo := memory.NewRepository(db)
	storedMemory := memory.Memory{
		ID:         "stored-memory",
		ProjectID:  primaryProject.ID,
		ProjectKey: primaryProject.Key,
		Type:       memory.TypeImplementation,
		Title:      "Stored memory",
		Content:    "Content indexed for the soft-delete transaction test.",
		Importance: 3,
		CreatedAt:  time.Unix(1, 0),
		UpdatedAt:  time.Unix(1, 0),
	}
	if _, err := repo.Upsert(context.Background(), storedMemory); err != nil {
		t.Fatalf("upsert memory: %v", err)
	}

	return db, repo, primaryProject.ID, otherProject.ID, storedMemory.ID
}
