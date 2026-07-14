package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Repository persists memories and keeps the FTS index synchronized.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a memory repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Upsert saves a memory and updates its FTS document inside a transaction.
func (r *Repository) Upsert(ctx context.Context, m Memory) (*Memory, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting memory transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	existing, exists, err := r.getByIDTx(ctx, tx, m.ProjectID, m.ID, true)
	if err != nil {
		return nil, err
	}
	if exists {
		m.CreatedAt = existing.CreatedAt
		_, err = tx.ExecContext(ctx, `
			UPDATE memories
			SET type = ?, title = ?, content = ?, importance = ?, source = ?, updated_at = ?, deleted_at = NULL
			WHERE id = ? AND project_id = ?
		`, m.Type, m.Title, m.Content, m.Importance, m.Source, formatTime(m.UpdatedAt), m.ID, m.ProjectID)
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO memories (
				id, project_id, type, title, content, importance, source, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, m.ID, m.ProjectID, m.Type, m.Title, m.Content, m.Importance, m.Source, formatTime(m.CreatedAt), formatTime(m.UpdatedAt))
	}
	if err != nil {
		return nil, fmt.Errorf("saving memory %s: %w", m.ID, err)
	}
	if err = r.upsertEnrichment(ctx, tx, m); err != nil {
		return nil, err
	}

	if err = r.replaceFTS(ctx, tx, m); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing memory transaction: %w", err)
	}
	return r.GetByID(ctx, m.ProjectID, m.ID)
}

// GetByID returns an active memory after validating the owning project.
func (r *Repository) GetByID(ctx context.Context, projectID, id string) (*Memory, error) {
	m, exists, err := r.getByID(ctx, projectID, id, false)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	return m, nil
}

// List returns active memories for a project with simple filters.
func (r *Repository) List(ctx context.Context, input ListInput, projectID string) ([]Memory, error) {
	limit := clamp(input.Limit, 50, 1, 200)
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}

	query := `SELECT m.id, m.project_id, m.type, m.title, m.content,
		COALESCE(e.summary, ''), COALESCE(e.short_summary, ''),
		COALESCE(e.keywords_json, '[]'), COALESCE(e.tags_json, '[]'),
		COALESCE(e.entities_json, '[]'), COALESCE(e.aliases_json, '[]'),
		COALESCE(e.search_phrases_json, '[]'),
		m.importance, m.source, m.access_count, m.last_accessed_at, m.created_at, m.updated_at, m.deleted_at
		FROM memories m
		LEFT JOIN memory_enrichments e ON e.memory_id = m.id
		WHERE m.project_id = ? AND m.deleted_at IS NULL`
	args := []any{projectID}
	if input.Type != "" {
		query += " AND m.type = ?"
		args = append(args, input.Type)
	}
	if input.Tag != "" {
		query += " AND COALESCE(e.tags_json, '[]') LIKE ?"
		args = append(args, "%"+input.Tag+"%")
	}
	query += " ORDER BY m.updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing memories: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// RecordAccesses increments the usage counter for active project memories.
func (r *Repository) RecordAccesses(ctx context.Context, projectID string, ids []string, accessedAt time.Time) error {
	ids = compactIDs(ids)
	if len(ids) == 0 {
		return nil
	}
	args := []any{formatTime(accessedAt), projectID}
	for _, id := range ids {
		args = append(args, id)
	}
	query := `UPDATE memories
		SET access_count = access_count + 1, last_accessed_at = ?
		WHERE project_id = ? AND deleted_at IS NULL AND id IN (` + placeholders(len(ids)) + `)`
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("recording memory accesses: %w", err)
	}
	return nil
}

// SoftDelete marks a memory as deleted and removes it from the FTS index.
func (r *Repository) SoftDelete(ctx context.Context, projectID, id string, deletedAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting delete transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE memories
		SET deleted_at = ?, updated_at = ?
		WHERE id = ? AND project_id = ? AND deleted_at IS NULL
	`, formatTime(deletedAt), formatTime(deletedAt), id, projectID)
	if err != nil {
		return fmt.Errorf("soft deleting memory %s: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted memory rows: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM memory_fts WHERE memory_id = ? AND project_key = (SELECT key FROM projects WHERE id = ?)`, id, projectID); err != nil {
		return fmt.Errorf("removing memory %s from fts: %w", id, err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing delete transaction: %w", err)
	}
	return nil
}

func (r *Repository) getByID(ctx context.Context, projectID, id string, includeDeleted bool) (*Memory, bool, error) {
	return r.getByIDTx(ctx, r.db, projectID, id, includeDeleted)
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (r *Repository) getByIDTx(ctx context.Context, q queryer, projectID, id string, includeDeleted bool) (*Memory, bool, error) {
	query := `SELECT m.id, m.project_id, m.type, m.title, m.content,
		COALESCE(e.summary, ''), COALESCE(e.short_summary, ''),
		COALESCE(e.keywords_json, '[]'), COALESCE(e.tags_json, '[]'),
		COALESCE(e.entities_json, '[]'), COALESCE(e.aliases_json, '[]'),
		COALESCE(e.search_phrases_json, '[]'),
		m.importance, m.source, m.access_count, m.last_accessed_at, m.created_at, m.updated_at, m.deleted_at
		FROM memories m
		LEFT JOIN memory_enrichments e ON e.memory_id = m.id
		WHERE m.id = ? AND m.project_id = ?`
	if !includeDeleted {
		query += " AND m.deleted_at IS NULL"
	}
	m, err := scanMemory(q.QueryRowContext(ctx, query, id, projectID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return m, true, nil
}

func (r *Repository) upsertEnrichment(ctx context.Context, tx *sql.Tx, m Memory) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO memory_enrichments (
			memory_id, summary, short_summary, keywords_json, tags_json,
			entities_json, aliases_json, search_phrases_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(memory_id) DO UPDATE SET
			summary = excluded.summary,
			short_summary = excluded.short_summary,
			keywords_json = excluded.keywords_json,
			tags_json = excluded.tags_json,
			entities_json = excluded.entities_json,
			aliases_json = excluded.aliases_json,
			search_phrases_json = excluded.search_phrases_json,
			updated_at = excluded.updated_at
	`, m.ID, m.Summary, m.ShortSummary, mustJSON(m.Keywords), mustJSON(m.Tags), mustJSON(m.Entities),
		mustJSON(m.Aliases), mustJSON(m.SearchPhrases), formatTime(m.UpdatedAt))
	if err != nil {
		return fmt.Errorf("upserting memory enrichment %s: %w", m.ID, err)
	}
	return nil
}

func (r *Repository) replaceFTS(ctx context.Context, tx *sql.Tx, m Memory) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_fts WHERE memory_id = ?`, m.ID); err != nil {
		return fmt.Errorf("clearing memory fts %s: %w", m.ID, err)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO memory_fts (
			memory_id, project_key, type, title, summary, short_summary,
			keywords, tags, entities, aliases, search_phrases, content
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ID, m.ProjectKey, m.Type, m.Title, m.Summary, m.ShortSummary, strings.Join(m.Keywords, " "),
		strings.Join(m.Tags, " "), strings.Join(m.Entities, " "), strings.Join(m.Aliases, " "),
		strings.Join(m.SearchPhrases, " "), m.Content)
	if err != nil {
		return fmt.Errorf("indexing memory %s: %w", m.ID, err)
	}
	return nil
}

func scanMemories(rows *sql.Rows) ([]Memory, error) {
	var memories []Memory
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating memories: %w", err)
	}
	return memories, nil
}

type memoryScanner interface {
	Scan(dest ...any) error
}

func scanMemory(row memoryScanner) (*Memory, error) {
	var m Memory
	var keywords, tags, entities, aliases, phrases string
	var createdAt, updatedAt string
	var lastAccessedAt sql.NullString
	var deletedAt sql.NullString
	if err := row.Scan(&m.ID, &m.ProjectID, &m.Type, &m.Title, &m.Content, &m.Summary, &m.ShortSummary,
		&keywords, &tags, &entities, &aliases, &phrases, &m.Importance, &m.Source, &m.AccessCount, &lastAccessedAt, &createdAt, &updatedAt, &deletedAt); err != nil {
		return nil, err
	}
	m.Keywords = fromJSON(keywords)
	m.Tags = fromJSON(tags)
	m.Entities = fromJSON(entities)
	m.Aliases = fromJSON(aliases)
	m.SearchPhrases = fromJSON(phrases)
	m.CreatedAt = parseTime(createdAt)
	m.UpdatedAt = parseTime(updatedAt)
	if lastAccessedAt.Valid {
		m.LastAccessedAt = new(parseTime(lastAccessedAt.String))
	}
	if deletedAt.Valid {
		m.DeletedAt = new(parseTime(deletedAt.String))
	}
	return &m, nil
}

func compactIDs(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func placeholders(count int) string {
	values := make([]string, count)
	for i := range values {
		values[i] = "?"
	}
	return strings.Join(values, ",")
}

func mustJSON(values []string) string {
	data, _ := json.Marshal(values)
	return string(data)
}

func fromJSON(value string) []string {
	var out []string
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return nil
	}
	return out
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}

func clamp(value, fallback, min, max int) int {
	if value == 0 {
		value = fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
