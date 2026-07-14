package search

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"mcp-ai-memory/internal/memory"
)

// Service coordinates text search and leaves the extension point for vectors.
type Service struct {
	fts *FTS
}

// NewService creates the search service.
func NewService(fts *FTS) *Service {
	return &Service{fts: fts}
}

// Search always searches inside the requested project.
func (s *Service) Search(ctx context.Context, projectID, projectKey string, input Input) ([]Result, error) {
	return s.fts.Search(ctx, projectID, projectKey, input)
}

// Count counts search results without recording accesses.
func (s *Service) Count(ctx context.Context, projectID, projectKey string, input Input) (int, string, error) {
	return s.fts.Count(ctx, projectID, projectKey, input)
}

func explain(mode, query string, m memory.Memory) string {
	q := strings.ToLower(query)
	var parts []string
	if containsAny(m.Title, q) {
		parts = append(parts, "title")
	}
	if containsAny(strings.Join(m.Tags, " "), q) {
		parts = append(parts, "tags")
	}
	if containsAny(strings.Join(m.Entities, " "), q) {
		parts = append(parts, "entities")
	}
	if containsAny(strings.Join(m.SearchPhrases, " "), q) {
		parts = append(parts, "search phrases")
	}
	if len(parts) == 0 {
		parts = append(parts, "FTS5 contextual match over enriched memory fields")
	}
	return mode + " search matched by " + strings.Join(parts, ", ")
}

func containsAny(text, query string) bool {
	text = strings.ToLower(text)
	for _, word := range strings.Fields(query) {
		if len(word) > 2 && strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func jsonList(value string) []string {
	var out []string
	_ = json.Unmarshal([]byte(value), &out)
	return out
}

func parseTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}
