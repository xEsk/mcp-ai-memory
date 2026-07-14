package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

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
	`, p.ID, p.Key, p.Name, p.Description, formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	if err != nil {
		return nil, fmt.Errorf("upserting project %s: %w", p.Key, err)
	}
	return r.GetByKey(ctx, p.Key)
}

// GetByKey finds a project by its stable key.
func (r *Repository) GetByKey(ctx context.Context, key string) (*Project, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, key, name, description, created_at, updated_at
		FROM projects
		WHERE key = ?
	`, key)
	return scanProject(row)
}

// List returns all projects ordered by key.
func (r *Repository) List(ctx context.Context) ([]Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, key, name, description, created_at, updated_at
		FROM projects
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

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(row scanner) (*Project, error) {
	var p Project
	var createdAt, updatedAt string
	if err := row.Scan(&p.ID, &p.Key, &p.Name, &p.Description, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("scanning project: %w", err)
	}
	p.CreatedAt = parseTime(createdAt)
	p.UpdatedAt = parseTime(updatedAt)
	return &p, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}
