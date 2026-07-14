package search

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/storage"
)

func TestFTS_SearchUsesDocumentedBM25FieldOrder(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenSQLite(ctx, filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})

	const (
		projectID  = "ranking-project-id"
		projectKey = "ranking-project"
		otherID    = "other-project-id"
		otherKey   = "other-project"
		query      = "needle"
	)
	frozenAt := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC)
	projects := project.NewRepository(db)
	for _, p := range []project.Project{
		{ID: projectID, Key: projectKey, Name: "Ranking", CreatedAt: frozenAt, UpdatedAt: frozenAt},
		{ID: otherID, Key: otherKey, Name: "Other", CreatedAt: frozenAt, UpdatedAt: frozenAt},
	} {
		if _, err := projects.Upsert(ctx, p); err != nil {
			t.Fatalf("upsert project %q: %v", p.Key, err)
		}
	}

	fields := []struct {
		name  string
		match func(*memory.Memory)
	}{
		{name: "type", match: func(m *memory.Memory) { m.Type = query }},
		{name: "title", match: func(m *memory.Memory) { m.Title = query }},
		{name: "summary", match: func(m *memory.Memory) { m.Summary = query }},
		{name: "short_summary", match: func(m *memory.Memory) { m.ShortSummary = query }},
		{name: "keywords", match: func(m *memory.Memory) { m.Keywords = []string{query} }},
		{name: "tags", match: func(m *memory.Memory) { m.Tags = []string{query} }},
		{name: "entities", match: func(m *memory.Memory) { m.Entities = []string{query} }},
		{name: "aliases", match: func(m *memory.Memory) { m.Aliases = []string{query} }},
		{name: "search_phrases", match: func(m *memory.Memory) { m.SearchPhrases = []string{query} }},
		{name: "content", match: func(m *memory.Memory) { m.Content = query }},
	}
	memories := memory.NewRepository(db)
	for _, fixtureProject := range []struct {
		id     string
		key    string
		prefix string
	}{
		{id: projectID, key: projectKey, prefix: projectKey},
		{id: otherID, key: otherKey, prefix: otherKey},
		{id: projectID, key: otherKey, prefix: "other-fts-key"},
		{id: otherID, key: projectKey, prefix: "other-project-id"},
	} {
		for _, field := range fields {
			fixture := memory.Memory{
				ID:            fixtureProject.prefix + "-" + field.name,
				ProjectID:     fixtureProject.id,
				ProjectKey:    fixtureProject.key,
				Type:          "neutral",
				Title:         "neutral",
				Content:       "neutral",
				Summary:       "neutral",
				ShortSummary:  "neutral",
				Keywords:      []string{"neutral"},
				Tags:          []string{"neutral"},
				Entities:      []string{"neutral"},
				Aliases:       []string{"neutral"},
				SearchPhrases: []string{"neutral"},
				Importance:    3,
				CreatedAt:     frozenAt,
				UpdatedAt:     frozenAt,
			}
			field.match(&fixture)
			if _, err := memories.Upsert(ctx, fixture); err != nil {
				t.Fatalf("upsert %s fixture for project %q: %v", field.name, fixtureProject.key, err)
			}
		}
	}

	fts := NewFTS(db)
	input := Input{Project: projectKey, Query: query, Limit: 50}
	count, mode, err := fts.Count(ctx, projectID, projectKey, input)
	if err != nil {
		t.Fatalf("count ranking fixtures: %v", err)
	}
	if mode != ModePrecise {
		t.Fatalf("count mode = %q, want %q", mode, ModePrecise)
	}
	if count != len(fields) {
		t.Fatalf("count = %d, want %d", count, len(fields))
	}

	results, err := fts.Search(ctx, projectID, projectKey, input)
	if err != nil {
		t.Fatalf("search ranking fixtures: %v", err)
	}
	if len(results) != len(fields) {
		t.Fatalf("search returned %d results, want %d", len(results), len(fields))
	}

	scores := make(map[string]float64, len(results))
	for _, result := range results {
		prefix := projectKey + "-"
		if result.Memory.ProjectID != projectID || result.Memory.ProjectKey != projectKey || !strings.HasPrefix(result.Memory.ID, prefix) {
			t.Fatalf("search leaked memory %q from project ID %q and key %q", result.Memory.ID, result.Memory.ProjectID, result.Memory.ProjectKey)
		}
		field := result.Memory.ID[len(prefix):]
		scores[field] = result.Score
	}
	for _, field := range fields {
		if _, ok := scores[field.name]; !ok {
			t.Errorf("search did not return %s match", field.name)
		}
	}

	expectedRanks := [][]string{
		{"type"},
		{"summary"},
		{"short_summary", "keywords"},
		{"tags", "entities", "aliases"},
		{"search_phrases"},
		{"content"},
		{"title"},
	}
	for i, rank := range expectedRanks {
		for _, field := range rank[1:] {
			if !sameRankingScore(scores[rank[0]], scores[field]) {
				t.Errorf("%s score %g and %s score %g should have equal rank", rank[0], scores[rank[0]], field, scores[field])
			}
		}
		if i == len(expectedRanks)-1 {
			continue
		}
		current := rank[len(rank)-1]
		next := expectedRanks[i+1][0]
		if scores[current] <= scores[next] || sameRankingScore(scores[current], scores[next]) {
			t.Errorf("%s score %g should rank ahead of %s score %g", current, scores[current], next, scores[next])
		}
	}
}

func sameRankingScore(left, right float64) bool {
	scale := math.Max(math.Abs(left), math.Abs(right))
	return math.Abs(left-right) <= scale*1e-9
}
