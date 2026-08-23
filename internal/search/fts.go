package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"mcp-ai-memory/internal/memory"
)

var ErrInvalidMode = errors.New("invalid search mode")

// FTS searches memories with SQLite FTS5 inside a project.
type FTS struct {
	db *sql.DB
}

// NewFTS creates the FTS5 searcher.
func NewFTS(db *sql.DB) *FTS {
	return &FTS{db: db}
}

// Search runs text search with weighted enriched fields.
func (f *FTS) Search(ctx context.Context, projectID, projectKey string, input Input) ([]Result, error) {
	return f.search(ctx, projectID, projectKey, input, true)
}

// SearchWithoutRecording runs text search without changing retrieval history.
func (f *FTS) SearchWithoutRecording(ctx context.Context, projectID, projectKey string, input Input) ([]Result, error) {
	return f.search(ctx, projectID, projectKey, input, false)
}

func (f *FTS) search(ctx context.Context, projectID, projectKey string, input Input, recordAccess bool) ([]Result, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	filter, err := buildFilter(projectID, projectKey, input)
	if err != nil {
		return nil, err
	}
	query := `SELECT m.id, m.project_id, m.type, m.title, m.content,
		COALESCE(e.summary, ''), COALESCE(e.short_summary, ''),
		COALESCE(e.keywords_json, '[]'), COALESCE(e.tags_json, '[]'),
		COALESCE(e.entities_json, '[]'), COALESCE(e.aliases_json, '[]'),
		COALESCE(e.search_phrases_json, '[]'),
		m.importance, m.source, m.access_count, m.last_accessed_at, m.created_at, m.updated_at, m.deleted_at,
		snippet(memory_fts, 11, '[', ']', '...', 24) AS snippet,
		bm25(memory_fts, 0.0, 0.0, 6.0, 1.0, 5.0, 4.0, 4.0, 3.5, 3.5, 3.5, 3.0, 1.5) AS rank
		FROM memory_fts
		JOIN memories m ON m.id = memory_fts.memory_id
		LEFT JOIN memory_enrichments e ON e.memory_id = m.id
		WHERE ` + filter.Where
	query += " ORDER BY rank ASC, m.importance DESC, m.updated_at DESC LIMIT ?"
	args := append([]any{}, filter.Args...)
	args = append(args, limit)

	rows, err := f.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("searching memories: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var m memory.Memory
		var keywords, tags, entities, aliases, phrases string
		var createdAt, updatedAt string
		var lastAccessedAt sql.NullString
		var deletedAt sql.NullString
		var snippet string
		var rank float64
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Type, &m.Title, &m.Content, &m.Summary, &m.ShortSummary,
			&keywords, &tags, &entities, &aliases, &phrases, &m.Importance, &m.Source, &m.AccessCount, &lastAccessedAt, &createdAt, &updatedAt, &deletedAt,
			&snippet, &rank); err != nil {
			return nil, fmt.Errorf("scanning search result: %w", err)
		}
		m.ProjectKey = projectKey
		m.Keywords = jsonList(keywords)
		m.Tags = jsonList(tags)
		m.Entities = jsonList(entities)
		m.Aliases = jsonList(aliases)
		m.SearchPhrases = jsonList(phrases)
		m.CreatedAt = parseTime(createdAt)
		m.UpdatedAt = parseTime(updatedAt)
		if lastAccessedAt.Valid {
			m.LastAccessedAt = new(parseTime(lastAccessedAt.String))
		}
		if deletedAt.Valid {
			m.DeletedAt = new(parseTime(deletedAt.String))
		}
		if !input.IncludeContent {
			m.Content = ""
		}
		results = append(results, Result{
			Memory:           m,
			Snippet:          snippet,
			Score:            -rank,
			MatchExplanation: explain(filter.Mode, input.Query, m),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating search results: %w", err)
	}
	if recordAccess {
		if err := f.RecordAccesses(ctx, projectID, results); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// RecordAccesses records one retrieval for each distinct result memory.
func (f *FTS) RecordAccesses(ctx context.Context, projectID string, results []Result) error {
	if err := f.recordSearchAccesses(ctx, projectID, results); err != nil {
		return err
	}
	return nil
}

// Count counts search matches without returning memories or recording accesses.
func (f *FTS) Count(ctx context.Context, projectID, projectKey string, input Input) (int, string, error) {
	filter, err := buildFilter(projectID, projectKey, input)
	if err != nil {
		return 0, "", err
	}
	query := `SELECT COUNT(*)
		FROM memory_fts
		JOIN memories m ON m.id = memory_fts.memory_id
		LEFT JOIN memory_enrichments e ON e.memory_id = m.id
		WHERE ` + filter.Where

	var count int
	if err := f.db.QueryRowContext(ctx, query, filter.Args...).Scan(&count); err != nil {
		return 0, "", fmt.Errorf("counting memories: %w", err)
	}
	return count, filter.Mode, nil
}

func (f *FTS) recordSearchAccesses(ctx context.Context, projectID string, results []Result) error {
	if len(results) == 0 {
		return nil
	}
	accessedAt := time.Now().UTC()
	ids := make([]string, 0, len(results))
	for i := range results {
		ids = append(ids, results[i].Memory.ID)
		results[i].Memory.AccessCount++
		results[i].Memory.LastAccessedAt = &accessedAt
	}
	args := []any{accessedAt.Format(time.RFC3339Nano), projectID}
	for _, id := range ids {
		args = append(args, id)
	}
	query := `UPDATE memories
		SET access_count = access_count + 1, last_accessed_at = ?
		WHERE project_id = ? AND deleted_at IS NULL AND id IN (` + placeholders(len(ids)) + `)`
	if _, err := f.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("recording search accesses: %w", err)
	}
	return nil
}

type searchFilter struct {
	Where string
	Args  []any
	Mode  string
	Query CompiledQuery
}

func buildFilter(projectID, projectKey string, input Input) (searchFilter, error) {
	compiled, err := CompileQuery(input.Query, input.Mode)
	if err != nil {
		return searchFilter{}, err
	}
	where := "memory_fts MATCH ? AND memory_fts.project_key = ? AND m.project_id = ? AND m.deleted_at IS NULL"
	args := []any{compiled.Expression, projectKey, projectID}
	if len(input.Types) > 0 {
		where += " AND m.type IN (" + placeholders(len(input.Types)) + ")"
		for _, t := range input.Types {
			args = append(args, t)
		}
	}
	for _, tag := range input.Tags {
		where += " AND COALESCE(e.tags_json, '[]') LIKE ?"
		args = append(args, "%"+tag+"%")
	}
	return searchFilter{Where: where, Args: args, Mode: compiled.Mode, Query: compiled}, nil
}

func normalizeMode(mode string) (string, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return ModePrecise, nil
	}
	switch mode {
	case ModePrecise, ModeBalanced, ModeBroad:
		return mode, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidMode, mode)
	}
}

func placeholders(count int) string {
	values := make([]string, count)
	for i := range values {
		values[i] = "?"
	}
	return strings.Join(values, ",")
}

var searchStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "can": true, "do": true, "does": true, "for": true,
	"from": true, "has": true, "have": true, "how": true, "in": true, "is": true,
	"it": true, "of": true, "on": true, "or": true, "that": true, "the": true,
	"this": true, "to": true, "we": true, "what": true, "when": true, "where": true,
	"which": true, "who": true, "why": true, "with": true, "work": true, "works": true,
	"com": true, "como": true, "del": true, "dels": true, "els": true, "las": true,
	"les": true, "los": true, "per": true, "por": true, "que": true, "una": true,
}
