package search

import (
	"context"
	"errors"
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

	empty := Input{Project: projectKey, Query: "the and how", Mode: ModePrecise}
	if _, _, err := fts.Count(ctx, projectID, projectKey, empty); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("count empty normalized query error = %v, want ErrInvalidQuery", err)
	}
	if _, err := fts.Search(ctx, projectID, projectKey, empty); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("search empty normalized query error = %v, want ErrInvalidQuery", err)
	}
}

func TestFTSBalancedRejectsMostlyUnknownProjectVocabulary(t *testing.T) {
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
		projectID  = "vocabulary-project-id"
		projectKey = "vocabulary-project"
		otherID    = "voice-project-id"
		otherKey   = "voice-project"
	)
	frozenAt := time.Date(2026, time.September, 13, 10, 0, 0, 0, time.UTC)
	projects := project.NewRepository(db)
	for _, fixture := range []project.Project{
		{ID: projectID, Key: projectKey, Name: "Vocabulary", CreatedAt: frozenAt, UpdatedAt: frozenAt},
		{ID: otherID, Key: otherKey, Name: "Voice", CreatedAt: frozenAt, UpdatedAt: frozenAt},
	} {
		if _, err := projects.Upsert(ctx, fixture); err != nil {
			t.Fatalf("upsert project %q: %v", fixture.Key, err)
		}
	}

	memories := memory.NewRepository(db)
	for _, fixture := range []memory.Memory{
		{
			ID: "claimant-architecture", ProjectID: projectID, ProjectKey: projectKey,
			Type: "architecture", Title: "Claimant conversation authority", Content: "Claimant conversation authority.",
			Importance: 4, CreatedAt: frozenAt, UpdatedAt: frozenAt,
		},
		{
			ID: "voice-architecture", ProjectID: otherID, ProjectKey: otherKey,
			Type: "architecture", Title: "Voice audio realtime telephony", Content: "Voice telephony media streaming intake.",
			Importance: 4, CreatedAt: frozenAt, UpdatedAt: frozenAt,
		},
	} {
		if _, err := memories.Upsert(ctx, fixture); err != nil {
			t.Fatalf("upsert memory %q: %v", fixture.ID, err)
		}
	}

	fts := NewFTS(db)
	query := "voice audio realtime channel conversation telephony claimant intake architecture authority"
	balanced := Input{Project: projectKey, Query: query, Mode: ModeBalanced, Limit: 10}
	count, mode, err := fts.Count(ctx, projectID, projectKey, balanced)
	if err != nil {
		t.Fatalf("count balanced query: %v", err)
	}
	if mode != ModeBalanced || count != 0 {
		t.Fatalf("balanced count = (%d, %q), want (0, %q)", count, mode, ModeBalanced)
	}
	results, err := fts.Search(ctx, projectID, projectKey, balanced)
	if err != nil {
		t.Fatalf("search balanced query: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("balanced search returned %v, want no results", results)
	}

	broad := balanced
	broad.Mode = ModeBroad
	results, err = fts.Search(ctx, projectID, projectKey, broad)
	if err != nil {
		t.Fatalf("search broad query: %v", err)
	}
	if len(results) != 1 || results[0].Memory.ID != "claimant-architecture" {
		t.Fatalf("broad search returned %v, want claimant-architecture", results)
	}
}
