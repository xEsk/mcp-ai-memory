package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrProjectDeleted prevents an upsert from silently restoring a deleted project.
var ErrProjectDeleted = errors.New("project is deleted")

// ErrProjectAlreadyDeleted reports a repeated project deletion request.
var ErrProjectAlreadyDeleted = errors.New("project is already deleted")

// ErrProjectNotDeleted reports a restore request for an active project.
var ErrProjectNotDeleted = errors.New("project is not deleted")

// Repository persists projects in SQLite.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a project repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Upsert creates or updates a project by key.
func (r *Repository) Upsert(ctx context.Context, p Project) (*Project, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (id, key, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			updated_at = excluded.updated_at
		WHERE projects.deleted_at IS NULL
	`, p.ID, p.Key, p.Name, p.Description, formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	if err != nil {
		return nil, fmt.Errorf("upserting project %s: %w", p.Key, err)
	}
	stored, err := r.GetByKey(ctx, p.Key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectDeleted
	}
	return stored, err
}

// GetByKey finds a project by its stable key.
func (r *Repository) GetByKey(ctx context.Context, key string) (*Project, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, key, name, description, created_at, updated_at, deleted_at
		FROM projects
		WHERE key = ? AND deleted_at IS NULL
	`, key)
	return scanProject(row)
}

// GetAnyByKey finds a project regardless of deletion state for offline management.
func (r *Repository) GetAnyByKey(ctx context.Context, key string) (*Project, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, key, name, description, created_at, updated_at, deleted_at
		FROM projects
		WHERE key = ?
	`, key)
	return scanProject(row)
}

// List returns all projects ordered by key.
func (r *Repository) List(ctx context.Context) ([]Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, key, name, description, created_at, updated_at, deleted_at
		FROM projects
		WHERE deleted_at IS NULL
		ORDER BY key ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating projects: %w", err)
	}
	return projects, nil
}

// ListAll returns active and deleted projects for offline management.
func (r *Repository) ListAll(ctx context.Context) ([]Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, key, name, description, created_at, updated_at, deleted_at
		FROM projects
		ORDER BY deleted_at IS NOT NULL, key ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing all projects: %w", err)
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating all projects: %w", err)
	}
	return projects, nil
}

// Stats returns aggregate memory statistics for one project without recording access.
func (r *Repository) Stats(ctx context.Context, projectID string) (Stats, error) {
	var stats Stats
	var latest sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN deleted_at IS NULL THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN deleted_at IS NOT NULL THEN 1 ELSE 0 END), 0),
			COUNT(*),
			COALESCE(SUM(access_count), 0),
			COALESCE(SUM(CASE WHEN access_count = 0 THEN 1 ELSE 0 END), 0),
			MAX(updated_at)
		FROM memories WHERE project_id = ?
	`, projectID).Scan(&stats.ActiveMemories, &stats.DeletedMemories, &stats.TotalMemories, &stats.Retrievals, &stats.NeverRetrieved, &latest)
	if err != nil {
		return Stats{}, fmt.Errorf("reading project statistics: %w", err)
	}
	if latest.Valid {
		stats.LatestMemoryAt = new(parseTime(latest.String))
	}
	stats.MemoriesByType = make(map[string]int)
	rows, err := r.db.QueryContext(ctx, `SELECT type, COUNT(*) FROM memories WHERE project_id = ? AND deleted_at IS NULL GROUP BY type ORDER BY type`, projectID)
	if err != nil {
		return Stats{}, fmt.Errorf("reading project type statistics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var memoryType string
		var count int
		if err := rows.Scan(&memoryType, &count); err != nil {
			return Stats{}, fmt.Errorf("scanning project type statistics: %w", err)
		}
		stats.MemoriesByType[memoryType] = count
	}
	if err := rows.Err(); err != nil {
		return Stats{}, fmt.Errorf("iterating project type statistics: %w", err)
	}
	return stats, nil
}

// SoftDelete hides an active project while preserving all of its data.
func (r *Repository) SoftDelete(ctx context.Context, key string, deletedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE projects SET deleted_at = ?, updated_at = ?
		WHERE key = ? AND deleted_at IS NULL
	`, formatTime(deletedAt), formatTime(deletedAt), key)
	if err != nil {
		return fmt.Errorf("soft deleting project %s: %w", key, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted project rows: %w", err)
	}
	if affected == 0 {
		p, lookupErr := r.GetAnyByKey(ctx, key)
		if errors.Is(lookupErr, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		if lookupErr != nil {
			return lookupErr
		}
		if p.DeletedAt != nil {
			return ErrProjectAlreadyDeleted
		}
		return sql.ErrNoRows
	}
	return nil
}

// Restore makes a soft-deleted project active again.
func (r *Repository) Restore(ctx context.Context, key string, restoredAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE projects SET deleted_at = NULL, updated_at = ?
		WHERE key = ? AND deleted_at IS NOT NULL
	`, formatTime(restoredAt), key)
	if err != nil {
		return fmt.Errorf("restoring project %s: %w", key, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking restored project rows: %w", err)
	}
	if affected == 0 {
		p, lookupErr := r.GetAnyByKey(ctx, key)
		if errors.Is(lookupErr, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		if lookupErr != nil {
			return lookupErr
		}
		if p.DeletedAt == nil {
			return ErrProjectNotDeleted
		}
		return sql.ErrNoRows
	}
	return nil
}

// PreviewCleanup returns exact counts for rows eligible for permanent cleanup.
func (r *Repository) PreviewCleanup(ctx context.Context) (CleanupStats, error) {
	return cleanupStats(ctx, r.db)
}

// Cleanup permanently removes deleted project data, deleted memories, and stale derived rows atomically.
func (r *Repository) Cleanup(ctx context.Context) (CleanupStats, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CleanupStats{}, fmt.Errorf("starting cleanup transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	preview, err := cleanupStats(ctx, tx)
	if err != nil {
		return CleanupStats{}, err
	}
	statements := []string{
		`DELETE FROM memory_fts WHERE NOT EXISTS (
			SELECT 1 FROM memories m JOIN projects p ON p.id = m.project_id
			WHERE m.id = memory_fts.memory_id AND p.key = memory_fts.project_key
				AND m.deleted_at IS NULL AND p.deleted_at IS NULL
		)`,
		`DELETE FROM memory_enrichments WHERE NOT EXISTS (SELECT 1 FROM memories m WHERE m.id = memory_enrichments.memory_id)`,
		`DELETE FROM memories WHERE deleted_at IS NOT NULL OR project_id IN (SELECT id FROM projects WHERE deleted_at IS NOT NULL)`,
		`DELETE FROM projects WHERE deleted_at IS NOT NULL`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return CleanupStats{}, fmt.Errorf("cleaning database: %w", err)
		}
	}
	remaining, err := cleanupStats(ctx, tx)
	if err != nil {
		return CleanupStats{}, err
	}
	if remaining != (CleanupStats{}) {
		return CleanupStats{}, fmt.Errorf("cleanup left eligible rows: %+v", remaining)
	}
	if err := tx.Commit(); err != nil {
		return CleanupStats{}, fmt.Errorf("committing cleanup transaction: %w", err)
	}
	return preview, nil
}

type rowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func cleanupStats(ctx context.Context, q rowQueryer) (CleanupStats, error) {
	var stats CleanupStats
	queries := []struct {
		destination *int
		query       string
	}{
		{&stats.DeletedProjects, `SELECT COUNT(*) FROM projects WHERE deleted_at IS NOT NULL`},
		{&stats.DeletedProjectMemories, `SELECT COUNT(*) FROM memories m JOIN projects p ON p.id = m.project_id WHERE p.deleted_at IS NOT NULL`},
		{&stats.DeletedMemories, `SELECT COUNT(*) FROM memories m JOIN projects p ON p.id = m.project_id WHERE p.deleted_at IS NULL AND m.deleted_at IS NOT NULL`},
		{&stats.StaleFTS, `SELECT COUNT(*) FROM memory_fts f WHERE NOT EXISTS (
			SELECT 1 FROM memories m JOIN projects p ON p.id = m.project_id
			WHERE m.id = f.memory_id AND p.key = f.project_key
				AND m.deleted_at IS NULL AND p.deleted_at IS NULL
		)`},
		{&stats.OrphanedEnrichments, `SELECT COUNT(*) FROM memory_enrichments e WHERE NOT EXISTS (SELECT 1 FROM memories m WHERE m.id = e.memory_id)`},
	}
	for _, item := range queries {
		if err := q.QueryRowContext(ctx, item.query).Scan(item.destination); err != nil {
			return CleanupStats{}, fmt.Errorf("counting cleanup rows: %w", err)
		}
	}
	return stats, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(row scanner) (*Project, error) {
	var p Project
	var createdAt, updatedAt string
	var deletedAt sql.NullString
	if err := row.Scan(&p.ID, &p.Key, &p.Name, &p.Description, &createdAt, &updatedAt, &deletedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("scanning project: %w", err)
	}
	p.CreatedAt = parseTime(createdAt)
	p.UpdatedAt = parseTime(updatedAt)
	if deletedAt.Valid {
		p.DeletedAt = new(parseTime(deletedAt.String))
	}
	return &p, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}
