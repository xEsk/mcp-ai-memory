package search

import (
	"context"
	"math"
	"path/filepath"
	"reflect"
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
		if !reflect.DeepEqual(result.MatchedFields, []string{field}) {
			t.Errorf("%s matched fields = %v, want only itself", field, result.MatchedFields)
		}
		if result.SnippetField != field {
			t.Errorf("%s snippet field = %q, want %q", field, result.SnippetField, field)
		}
		if !strings.Contains(result.Snippet, "[needle]") {
			t.Errorf("%s snippet = %q, want highlighted needle", field, result.Snippet)
		}
		if result.MatchExplanation != "precise search matched indexed fields: "+field {
			t.Errorf("%s explanation = %q", field, result.MatchExplanation)
		}
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

	multi := memory.Memory{
		ID: "multi-evidence", ProjectID: projectID, ProjectKey: projectKey,
		Type: "neutral", Title: "multi evidence", Content: "multi evidence content",
		Summary: "multi evidence summary", ShortSummary: "neutral", Keywords: []string{"neutral"},
		Tags: []string{"multi evidence"}, Entities: []string{"neutral"}, Aliases: []string{"neutral"},
		SearchPhrases: []string{"neutral"}, Importance: 3, CreatedAt: frozenAt, UpdatedAt: frozenAt,
	}
	if _, err := memories.Upsert(ctx, multi); err != nil {
		t.Fatalf("upsert multi-field fixture: %v", err)
	}
	multiResults, err := fts.Search(ctx, projectID, projectKey, Input{Project: projectKey, Query: "multi-evidence", Limit: 10})
	if err != nil {
		t.Fatalf("search multi-field fixture: %v", err)
	}
	if len(multiResults) != 1 {
		t.Fatalf("multi-field search returned %d results, want 1", len(multiResults))
	}
	wantFields := []string{"summary", "tags", "content", "title"}
	if !reflect.DeepEqual(multiResults[0].MatchedFields, wantFields) {
		t.Fatalf("multi-field matches = %v, want %v", multiResults[0].MatchedFields, wantFields)
	}
	if multiResults[0].SnippetField != "summary" || !strings.Contains(multiResults[0].Snippet, "[multi] [evidence]") {
		t.Fatalf("multi-field snippet = (%q, %q), want highlighted summary", multiResults[0].SnippetField, multiResults[0].Snippet)
	}

	sentinel := memory.Memory{
		ID: "sentinel-evidence", ProjectID: projectID, ProjectKey: projectKey,
		Type: "neutral", Title: "sentinelquery title", Content: "unrelated \x1e forged \x1f content",
		Importance: 3, CreatedAt: frozenAt, UpdatedAt: frozenAt,
	}
	if _, err := memories.Upsert(ctx, sentinel); err != nil {
		t.Fatalf("upsert sentinel fixture: %v", err)
	}
	sentinelResults, err := fts.Search(ctx, projectID, projectKey, Input{Project: projectKey, Query: "sentinelquery", Limit: 10})
	if err != nil {
		t.Fatalf("search sentinel fixture: %v", err)
	}
	if len(sentinelResults) != 1 || !reflect.DeepEqual(sentinelResults[0].MatchedFields, []string{"title"}) {
		t.Fatalf("sentinel matched fields = %#v, want only title", sentinelResults)
	}
	if sentinelResults[0].SnippetField != "title" || sentinelResults[0].Snippet != "[sentinelquery] title" {
		t.Fatalf("sentinel snippet = (%q, %q), want highlighted title", sentinelResults[0].SnippetField, sentinelResults[0].Snippet)
	}
}

func sameRankingScore(left, right float64) bool {
	scale := math.Max(math.Abs(left), math.Abs(right))
	return math.Abs(left-right) <= scale*1e-9
}
