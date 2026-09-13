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
	return f.search(ctx, projectID, projectKey, input, true, true)
}

// SearchWithoutRecording runs the unrestricted candidate search used by
// suggestion without changing retrieval history.
func (f *FTS) SearchWithoutRecording(ctx context.Context, projectID, projectKey string, input Input) ([]Result, error) {
	return f.search(ctx, projectID, projectKey, input, false, false)
}

func (f *FTS) search(ctx context.Context, projectID, projectKey string, input Input, recordAccess, applyVocabularyGuard bool) ([]Result, error) {
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
	if applyVocabularyGuard && filter.Mode == ModeBalanced {
		mostlyUnknown, err := f.queryMostlyUnknownToProject(ctx, projectID, projectKey, filter.Query)
		if err != nil {
			return nil, err
		}
		if mostlyUnknown {
			return nil, nil
		}
	}
	query := `SELECT m.id, m.project_id, m.type, m.title, m.content,
		COALESCE(e.summary, ''), COALESCE(e.short_summary, ''),
		COALESCE(e.keywords_json, '[]'), COALESCE(e.tags_json, '[]'),
		COALESCE(e.entities_json, '[]'), COALESCE(e.aliases_json, '[]'),
		COALESCE(e.search_phrases_json, '[]'),
		m.importance, m.source, m.access_count, m.last_accessed_at, m.created_at, m.updated_at, m.deleted_at,
		highlight(memory_fts, 2, char(30), char(31)),
		highlight(memory_fts, 3, char(30), char(31)),
		highlight(memory_fts, 4, char(30), char(31)),
		highlight(memory_fts, 5, char(30), char(31)),
		highlight(memory_fts, 6, char(30), char(31)),
		highlight(memory_fts, 7, char(30), char(31)),
		highlight(memory_fts, 8, char(30), char(31)),
		highlight(memory_fts, 9, char(30), char(31)),
		highlight(memory_fts, 10, char(30), char(31)),
		highlight(memory_fts, 11, char(30), char(31)),
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
		var highlights [10]string
		var rank float64
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Type, &m.Title, &m.Content, &m.Summary, &m.ShortSummary,
			&keywords, &tags, &entities, &aliases, &phrases, &m.Importance, &m.Source, &m.AccessCount, &lastAccessedAt, &createdAt, &updatedAt, &deletedAt,
			&highlights[0], &highlights[1], &highlights[2], &highlights[3], &highlights[4],
			&highlights[5], &highlights[6], &highlights[7], &highlights[8], &highlights[9], &rank); err != nil {
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
		originals := [10]string{
			m.Type, m.Title, m.Summary, m.ShortSummary, strings.Join(m.Keywords, " "),
			strings.Join(m.Tags, " "), strings.Join(m.Entities, " "), strings.Join(m.Aliases, " "),
			strings.Join(m.SearchPhrases, " "), m.Content,
		}
		matchedFields, snippetField, snippet, explanation := searchEvidence(filter.Mode, originals, highlights)
		results = append(results, Result{
			Memory:           m,
			MatchedFields:    matchedFields,
			SnippetField:     snippetField,
			Snippet:          snippet,
			Score:            -rank,
			MatchExplanation: explanation,
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

const (
	highlightStart = '\x1e'
	highlightEnd   = '\x1f'
)

var searchEvidencePriority = []struct {
	name  string
	index int
}{
	{name: "type", index: 0},
	{name: "summary", index: 2},
	{name: "short_summary", index: 3},
	{name: "keywords", index: 4},
	{name: "tags", index: 5},
	{name: "entities", index: 6},
	{name: "aliases", index: 7},
	{name: "search_phrases", index: 8},
	{name: "content", index: 9},
	{name: "title", index: 1},
}

func searchEvidence(mode string, originals, highlights [10]string) ([]string, string, string, string) {
	var matched []string
	selected := -1
	for _, field := range searchEvidencePriority {
		if highlights[field.index] == originals[field.index] {
			continue
		}
		matched = append(matched, field.name)
		if selected < 0 {
			selected = field.index
		}
	}
	if selected < 0 {
		return nil, "", "", mode + " search matched FTS5 without highlight evidence"
	}
	return matched, searchEvidencePriorityName(selected), renderHighlight(originals[selected], highlights[selected]),
		mode + " search matched indexed fields: " + strings.Join(matched, ", ")
}

func searchEvidencePriorityName(index int) string {
	for _, field := range searchEvidencePriority {
		if field.index == index {
			return field.name
		}
	}
	return ""
}

func renderHighlight(original, highlighted string) string {
	originalRunes := []rune(original)
	highlightedRunes := []rune(highlighted)
	runes := make([]rune, 0, len(highlightedRunes))
	match := -1
	matchEnd := -1
	for originalIndex, highlightedIndex := 0, 0; highlightedIndex < len(highlightedRunes); highlightedIndex++ {
		r := highlightedRunes[highlightedIndex]
		if originalIndex < len(originalRunes) && r == originalRunes[originalIndex] {
			switch r {
			case highlightStart:
				runes = append(runes, []rune(`\u001e`)...)
			case highlightEnd:
				runes = append(runes, []rune(`\u001f`)...)
			default:
				runes = append(runes, r)
			}
			originalIndex++
			continue
		}
		switch r {
		case highlightStart:
			if match < 0 {
				match = len(runes)
			}
			runes = append(runes, '[')
		case highlightEnd:
			runes = append(runes, ']')
			if match >= 0 && matchEnd < 0 {
				matchEnd = len(runes) - 1
			}
		default:
			runes = append(runes, r)
		}
	}
	const snippetRunes = 240
	start := 0
	end := len(runes)
	if len(runes) > snippetRunes && match >= 0 {
		start = max(0, match-80)
		end = min(len(runes), start+snippetRunes)
		if end == len(runes) {
			start = max(0, end-snippetRunes)
		}
		if matchEnd >= end {
			end = matchEnd + 1
			start = min(start, match)
		}
	}
	text := string(runes[start:end])
	if start > 0 {
		text = "..." + text
	}
	if end < len(runes) {
		text += "..."
	}
	return text
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
	if filter.Mode == ModeBalanced {
		mostlyUnknown, err := f.queryMostlyUnknownToProject(ctx, projectID, projectKey, filter.Query)
		if err != nil {
			return 0, "", err
		}
		if mostlyUnknown {
			return 0, filter.Mode, nil
		}
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

// queryMostlyUnknownToProject detects long balanced queries whose vocabulary is
// mostly absent from the active memories in the requested project.
func (f *FTS) queryMostlyUnknownToProject(ctx context.Context, projectID, projectKey string, query CompiledQuery) (bool, error) {
	if len(query.Terms) < 4 {
		return false, nil
	}
	unknown := 0
	threshold := (len(query.Terms) + 1) / 2
	const termExistsQuery = `SELECT EXISTS(
		SELECT 1
		FROM memory_fts
		JOIN memories m ON m.id = memory_fts.memory_id
		WHERE memory_fts MATCH ?
			AND memory_fts.project_key = ?
			AND m.project_id = ?
			AND m.deleted_at IS NULL
		LIMIT 1
	)`
	for _, term := range query.Terms {
		var exists bool
		if err := f.db.QueryRowContext(ctx, termExistsQuery, compileTermExpression(term), projectKey, projectID).Scan(&exists); err != nil {
			return false, fmt.Errorf("checking project query vocabulary: %w", err)
		}
		if !exists {
			unknown++
			if unknown >= threshold {
				return true, nil
			}
		}
	}
	return false, nil
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
