package search

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/storage"
)

func TestFTSShortTechnicalTermsRemainExactAndProjectScoped(t *testing.T) {
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
		projectID  = "technical-project-id"
		projectKey = "technical-project"
		otherID    = "other-project-id"
		otherKey   = "other-project"
	)
	frozenAt := time.Date(2026, time.August, 24, 10, 0, 0, 0, time.UTC)
	projects := project.NewRepository(db)
	for _, fixture := range []project.Project{
		{ID: projectID, Key: projectKey, Name: "Technical", CreatedAt: frozenAt, UpdatedAt: frozenAt},
		{ID: otherID, Key: otherKey, Name: "Other", CreatedAt: frozenAt, UpdatedAt: frozenAt},
	} {
		if _, err := projects.Upsert(ctx, fixture); err != nil {
			t.Fatalf("upsert project %q: %v", fixture.Key, err)
		}
	}

	memories := memory.NewRepository(db)
	for _, fixture := range []memory.Memory{
		{ID: "go-language", ProjectID: projectID, ProjectKey: projectKey, Type: "fact", Title: "Go language", Content: "Go uses goroutines", Importance: 3, CreatedAt: frozenAt, UpdatedAt: frozenAt},
		{ID: "db-guide", ProjectID: projectID, ProjectKey: projectKey, Type: "fact", Title: "DB guide", Content: "DB transaction rules", Importance: 3, CreatedAt: frozenAt, UpdatedAt: frozenAt},
		{ID: "io-reader", ProjectID: projectID, ProjectKey: projectKey, Type: "fact", Title: "io.Reader", Content: "Use io Reader interfaces", Importance: 3, CreatedAt: frozenAt, UpdatedAt: frozenAt},
		{ID: "iostream", ProjectID: projectID, ProjectKey: projectKey, Type: "fact", Title: "iostream", Content: "C++ stream API", Importance: 3, CreatedAt: frozenAt, UpdatedAt: frozenAt},
		{ID: "other-go", ProjectID: otherID, ProjectKey: otherKey, Type: "fact", Title: "Go elsewhere", Content: "Go", Importance: 3, CreatedAt: frozenAt, UpdatedAt: frozenAt},
	} {
		if _, err := memories.Upsert(ctx, fixture); err != nil {
			t.Fatalf("upsert memory %q: %v", fixture.ID, err)
		}
	}

	fts := NewFTS(db)
	for _, test := range []struct {
		query string
		want  string
	}{
		{query: "Go", want: "go-language"},
		{query: "DB", want: "db-guide"},
		{query: "io", want: "io-reader"},
	} {
		t.Run(test.query, func(t *testing.T) {
			input := Input{Project: projectKey, Query: test.query, Mode: ModePrecise, Limit: 10}
			count, mode, err := fts.Count(ctx, projectID, projectKey, input)
			if err != nil {
				t.Fatalf("count query %q: %v", test.query, err)
			}
			if mode != ModePrecise || count != 1 {
				t.Fatalf("count query %q = (%d, %q), want (1, %q)", test.query, count, mode, ModePrecise)
			}
			results, err := fts.Search(ctx, projectID, projectKey, input)
			if err != nil {
				t.Fatalf("search query %q: %v", test.query, err)
			}
			if len(results) != 1 || results[0].Memory.ID != test.want {
				t.Fatalf("search query %q returned %#v, want only %q", test.query, results, test.want)
			}
		})
	}
}
