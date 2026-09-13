package memory_test

import (
	"context"
	"path/filepath"
	"testing"

	"mcp-ai-memory/internal/enrich"
	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/platform"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/search"
	"mcp-ai-memory/internal/storage"
)

func TestMemoryLifecycle_SearchIsProjectScoped(t *testing.T) {
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

	clock := platform.NewClock()
	projects := project.NewService(project.NewRepository(db), clock)
	memories := memory.NewService(memory.NewRepository(db), projects, enrich.NewLocalHeuristicEnricher(), clock)
	searchService := search.NewService(search.NewFTS(db))

	apiProject, err := projects.Upsert(ctx, project.UpsertInput{Key: "api", Name: "API"})
	if err != nil {
		t.Fatalf("upsert api project: %v", err)
	}
	uiProject, err := projects.Upsert(ctx, project.UpsertInput{Key: "ui", Name: "UI"})
	if err != nil {
		t.Fatalf("upsert ui project: %v", err)
	}

	apiMemory, err := memories.Upsert(ctx, memory.UpsertInput{
		Project:    "api",
		Type:       memory.TypeArchitecture,
		Title:      "Routes use Server class",
		Content:    "API calls go through the Server class and phproute base endpoint helper.",
		Tags:       []string{"api"},
		Importance: 3,
		Enrich:     true,
	})
	if err != nil {
		t.Fatalf("upsert api memory: %v", err)
	}
	if _, err := memories.Upsert(ctx, memory.UpsertInput{
		Project:    "ui",
		Type:       memory.TypeImplementation,
		Title:      "Wallet recharge component",
		Content:    "Wallet recharge UI is handled by RechargePanel.",
		Tags:       []string{"frontend"},
		Importance: 2,
		Enrich:     true,
	}); err != nil {
		t.Fatalf("upsert ui memory: %v", err)
	}

	results, err := searchService.Search(ctx, apiProject.ID, apiProject.Key, search.Input{
		Project: "api",
		Query:   "how do we call the API?",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search api memories: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("api search returned %d results, want 1", len(results))
	}
	if results[0].Memory.ID != apiMemory.ID {
		t.Fatalf("api search returned %q, want %q", results[0].Memory.ID, apiMemory.ID)
	}
	if results[0].Memory.AccessCount != 1 {
		t.Fatalf("search access count = %d, want 1", results[0].Memory.AccessCount)
	}
	if results[0].Memory.LastAccessedAt == nil {
		t.Fatalf("search last accessed at is nil")
	}

	got, err := memories.Get(ctx, "api", apiMemory.ID)
	if err != nil {
		t.Fatalf("get api memory: %v", err)
	}
	if got.AccessCount != 2 {
		t.Fatalf("get access count = %d, want 2", got.AccessCount)
	}
	if got.LastAccessedAt == nil {
		t.Fatalf("get last accessed at is nil")
	}

	listed, err := memories.List(ctx, memory.ListInput{Project: "api", Limit: 10})
	if err != nil {
		t.Fatalf("list api memories: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list returned %d memories, want 1", len(listed))
	}
	if listed[0].AccessCount != 3 {
		t.Fatalf("list access count = %d, want 3", listed[0].AccessCount)
	}

	uiResults, err := searchService.Search(ctx, uiProject.ID, uiProject.Key, search.Input{
		Project: "ui",
		Query:   "Server class API route",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search ui memories: %v", err)
	}
	if len(uiResults) != 0 {
		t.Fatalf("ui search leaked %d api results", len(uiResults))
	}

	if err := memories.Delete(ctx, "api", apiMemory.ID); err != nil {
		t.Fatalf("delete api memory: %v", err)
	}
	afterDelete, err := searchService.Search(ctx, apiProject.ID, apiProject.Key, search.Input{
		Project: "api",
		Query:   "Server class",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if len(afterDelete) != 0 {
		t.Fatalf("deleted memory returned in search: %d results", len(afterDelete))
	}
}

func TestMemorySearchModesAndCount(t *testing.T) {
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

	clock := platform.NewClock()
	projects := project.NewService(project.NewRepository(db), clock)
	repo := memory.NewRepository(db)
	memories := memory.NewService(repo, projects, enrich.NewLocalHeuristicEnricher(), clock)
	searchService := search.NewService(search.NewFTS(db))

	apiProject, err := projects.Upsert(ctx, project.UpsertInput{Key: "api", Name: "API"})
	if err != nil {
		t.Fatalf("upsert api project: %v", err)
	}
	if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "ops", Name: "Ops"}); err != nil {
		t.Fatalf("upsert ops project: %v", err)
	}

	target, err := memories.Upsert(ctx, memory.UpsertInput{
		Project:    "api",
		Type:       memory.TypeImplementation,
		Title:      "Invoice route handler",
		Content:    "Invoice route handler calls InvoiceController to validate invoice payloads.",
		Tags:       []string{"api"},
		Importance: 3,
		Enrich:     true,
	})
	if err != nil {
		t.Fatalf("upsert target memory: %v", err)
	}
	distractor, err := memories.Upsert(ctx, memory.UpsertInput{
		Project:    "api",
		Type:       memory.TypeWorkflow,
		Title:      "Handler work notes",
		Content:    "Workflow notes explain where handler tasks are documented for project work.",
		Tags:       []string{"workflow"},
		Importance: 1,
		Enrich:     true,
	})
	if err != nil {
		t.Fatalf("upsert distractor memory: %v", err)
	}
	if _, err := memories.Upsert(ctx, memory.UpsertInput{
		Project:    "ops",
		Type:       memory.TypeImplementation,
		Title:      "Invoice route handler",
		Content:    "Invoice route handler belongs to another project and must not leak.",
		Importance: 3,
		Enrich:     true,
	}); err != nil {
		t.Fatalf("upsert other project memory: %v", err)
	}

	input := search.Input{
		Project: "api",
		Query:   "where is invoice route handler",
		Limit:   10,
	}
	count, mode, err := searchService.Count(ctx, apiProject.ID, apiProject.Key, input)
	if err != nil {
		t.Fatalf("count precise memories: %v", err)
	}
	if mode != search.ModePrecise {
		t.Fatalf("count mode = %q, want %q", mode, search.ModePrecise)
	}
	if count != 1 {
		t.Fatalf("precise count = %d, want 1", count)
	}
	storedTarget, err := repo.GetByID(ctx, apiProject.ID, target.ID)
	if err != nil {
		t.Fatalf("get target after count: %v", err)
	}
	if storedTarget.AccessCount != 0 {
		t.Fatalf("count changed access count = %d, want 0", storedTarget.AccessCount)
	}

	preciseResults, err := searchService.Search(ctx, apiProject.ID, apiProject.Key, input)
	if err != nil {
		t.Fatalf("search precise memories: %v", err)
	}
	if len(preciseResults) != 1 {
		t.Fatalf("precise search returned %d results, want 1", len(preciseResults))
	}
	if preciseResults[0].Memory.ID != target.ID {
		t.Fatalf("precise search returned %q, want %q", preciseResults[0].Memory.ID, target.ID)
	}

	broadInput := input
	broadInput.Mode = search.ModeBroad
	broadResults, err := searchService.Search(ctx, apiProject.ID, apiProject.Key, broadInput)
	if err != nil {
		t.Fatalf("search broad memories: %v", err)
	}
	if len(broadResults) != 2 {
		t.Fatalf("broad search returned %d results, want 2", len(broadResults))
	}
	if !containsMemoryID(broadResults, target.ID) || !containsMemoryID(broadResults, distractor.ID) {
		t.Fatalf("broad search did not include target and distractor")
	}
	broadCount, _, err := searchService.Count(ctx, apiProject.ID, apiProject.Key, broadInput)
	if err != nil {
		t.Fatalf("count broad memories: %v", err)
	}
	if broadCount != len(broadResults) {
		t.Fatalf("broad count = %d, want %d", broadCount, len(broadResults))
	}
	storedDistractor, err := repo.GetByID(ctx, apiProject.ID, distractor.ID)
	if err != nil {
		t.Fatalf("get distractor after count: %v", err)
	}
	if storedDistractor.AccessCount != 1 {
		t.Fatalf("count changed distractor access count = %d, want 1", storedDistractor.AccessCount)
	}
}

func containsMemoryID(results []search.Result, id string) bool {
	for _, result := range results {
		if result.Memory.ID == id {
			return true
		}
	}
	return false
}
