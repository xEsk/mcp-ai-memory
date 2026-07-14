package mcpserver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"mcp-ai-memory/internal/enrich"
	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/platform"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/search"
	"mcp-ai-memory/internal/storage"
)

func TestMemorySuggestStrongMatchRecommendsUpdate(t *testing.T) {
	ctx, server, projects, memories := newSuggestTestServer(t)

	if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "api", Name: "API"}); err != nil {
		t.Fatalf("upsert project: %v", err)
	}
	stored, err := memories.Upsert(ctx, memory.UpsertInput{
		Project:    "api",
		Type:       memory.TypeImplementation,
		Title:      "Invoice route handler",
		Content:    "Invoice route handler calls InvoiceController to validate invoice payloads.",
		Importance: 3,
		Enrich:     true,
	})
	if err != nil {
		t.Fatalf("upsert memory: %v", err)
	}

	result, err := server.handleMemorySuggest(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"project": "api",
				"input":   "Invoice route handler calls InvoiceController validate invoice payloads",
			},
		},
	})
	if err != nil {
		t.Fatalf("suggest memory: %v", err)
	}
	if result.IsError {
		t.Fatalf("suggest returned tool error")
	}
	content, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any", result.StructuredContent)
	}
	if content["suggested_action"] != "update_existing" {
		t.Fatalf("suggested action = %q, want update_existing", content["suggested_action"])
	}
	if content["search_mode"] != search.ModePrecise {
		t.Fatalf("search mode = %q, want %q", content["search_mode"], search.ModePrecise)
	}
	if content["recommended_candidate_id"] != stored.ID {
		t.Fatalf("recommended candidate id = %q, want %q", content["recommended_candidate_id"], stored.ID)
	}
	if content["confidence"] != "high" {
		t.Fatalf("confidence = %q, want high", content["confidence"])
	}
	if content["next_step"] != "inspect_recommended_candidate" {
		t.Fatalf("next step = %q, want inspect_recommended_candidate", content["next_step"])
	}
	if content["candidate_count"] != 1 {
		t.Fatalf("candidate count = %v, want 1", content["candidate_count"])
	}
	candidates, ok := content["candidates"].([]suggestCandidateOutput)
	if !ok {
		t.Fatalf("candidates type = %T, want []suggestCandidateOutput", content["candidates"])
	}
	if len(candidates) != 1 {
		t.Fatalf("suggest candidates = %d, want 1", len(candidates))
	}
	if candidates[0].Rank != 1 {
		t.Fatalf("candidate rank = %d, want 1", candidates[0].Rank)
	}
	if !candidates[0].IsRecommended {
		t.Fatal("first candidate should be recommended")
	}
	if candidates[0].CandidateQuality != "strong" {
		t.Fatalf("candidate quality = %q, want strong", candidates[0].CandidateQuality)
	}
	if candidates[0].Memory.Title != "Invoice route handler" {
		t.Fatalf("suggest candidate title = %q, want Invoice route handler", candidates[0].Memory.Title)
	}
	if candidates[0].MatchReason != "strong title and metadata overlap" {
		t.Fatalf("match reason = %q, want strong semantic reason", candidates[0].MatchReason)
	}
}

func TestMemorySuggestWeakBalancedMatchDoesNotRecommendUpdate(t *testing.T) {
	ctx, server, projects, memories := newSuggestTestServer(t)

	if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "api", Name: "API"}); err != nil {
		t.Fatalf("upsert project: %v", err)
	}
	if _, err := memories.Upsert(ctx, memory.UpsertInput{
		Project:    "api",
		Type:       memory.TypeWorkflow,
		Title:      "Project task workflow",
		Content:    "Project task workflow describes how agents should record recurring work patterns.",
		Importance: 3,
		Enrich:     true,
	}); err != nil {
		t.Fatalf("upsert memory: %v", err)
	}

	result, err := server.handleMemorySuggest(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"project": "api",
				"input":   "document project task notes for future work",
			},
		},
	})
	if err != nil {
		t.Fatalf("suggest memory: %v", err)
	}
	content, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any", result.StructuredContent)
	}
	if content["suggested_action"] != "create" {
		t.Fatalf("suggested action = %q, want create", content["suggested_action"])
	}
	if content["search_mode"] != search.ModeBalanced {
		t.Fatalf("search mode = %q, want %q", content["search_mode"], search.ModeBalanced)
	}
	if content["recommended_candidate_id"] != "" {
		t.Fatalf("recommended candidate id = %q, want empty", content["recommended_candidate_id"])
	}
	if content["confidence"] != "low" {
		t.Fatalf("confidence = %q, want low", content["confidence"])
	}
	if content["next_step"] != "create_focused_memory" {
		t.Fatalf("next step = %q, want create_focused_memory", content["next_step"])
	}
	if content["candidate_count"] != 1 {
		t.Fatalf("candidate count = %v, want 1", content["candidate_count"])
	}
	candidates, ok := content["candidates"].([]suggestCandidateOutput)
	if !ok {
		t.Fatalf("candidates type = %T, want []suggestCandidateOutput", content["candidates"])
	}
	if len(candidates) != 1 {
		t.Fatalf("suggest candidates = %d, want 1", len(candidates))
	}
	if candidates[0].IsRecommended {
		t.Fatal("weak candidate should not be recommended")
	}
	if candidates[0].CandidateQuality != "weak" {
		t.Fatalf("candidate quality = %q, want weak", candidates[0].CandidateQuality)
	}
	if candidates[0].MatchReason != "matched only broad project terms" {
		t.Fatalf("match reason = %q, want weak semantic reason", candidates[0].MatchReason)
	}
}

func TestMemorySuggestAmbiguousCandidatesAreNotAutoRecommended(t *testing.T) {
	ctx, server, projects, memories := newSuggestTestServer(t)

	if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "api", Name: "API"}); err != nil {
		t.Fatalf("upsert project: %v", err)
	}
	for _, item := range []memory.UpsertInput{
		{
			Project:    "api",
			Type:       memory.TypeImplementation,
			Title:      "Invoice route handler",
			Content:    "Invoice route handler validates invoice payloads for API requests.",
			Importance: 3,
			Enrich:     true,
		},
		{
			Project:    "api",
			Type:       memory.TypeImplementation,
			Title:      "Invoice webhook handler",
			Content:    "Invoice webhook handler validates invoice payloads for webhook callbacks.",
			Importance: 3,
			Enrich:     true,
		},
	} {
		if _, err := memories.Upsert(ctx, item); err != nil {
			t.Fatalf("upsert memory: %v", err)
		}
	}

	result, err := server.handleMemorySuggest(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"project": "api",
				"input":   "Invoice handler validates invoice payloads",
			},
		},
	})
	if err != nil {
		t.Fatalf("suggest memory: %v", err)
	}
	content, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any", result.StructuredContent)
	}
	if content["suggested_action"] != "create" {
		t.Fatalf("suggested action = %q, want create", content["suggested_action"])
	}
	if content["recommended_candidate_id"] != "" {
		t.Fatalf("recommended candidate id = %q, want empty", content["recommended_candidate_id"])
	}
	if content["next_step"] != "create_focused_memory" {
		t.Fatalf("next step = %q, want create_focused_memory", content["next_step"])
	}
	candidates, ok := content["candidates"].([]suggestCandidateOutput)
	if !ok {
		t.Fatalf("candidates type = %T, want []suggestCandidateOutput", content["candidates"])
	}
	if len(candidates) != 2 {
		t.Fatalf("suggest candidates = %d, want 2", len(candidates))
	}
	for _, candidate := range candidates {
		if candidate.IsRecommended {
			t.Fatalf("ambiguous candidate rank %d should not be recommended", candidate.Rank)
		}
	}
	if candidates[0].MatchReason != "possible semantic overlap, review before updating" {
		t.Fatalf("match reason = %q, want possible semantic reason", candidates[0].MatchReason)
	}
	if candidates[0].CandidateQuality != "possible" {
		t.Fatalf("candidate quality = %q, want possible", candidates[0].CandidateQuality)
	}
}

func TestMemorySuggestNoCandidatesRecommendsCreate(t *testing.T) {
	ctx, server, projects, _ := newSuggestTestServer(t)
	if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "api", Name: "API"}); err != nil {
		t.Fatalf("upsert project: %v", err)
	}

	result, err := server.handleMemorySuggest(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"project": "api",
				"input":   "document websocket observability handoff protocol",
			},
		},
	})
	if err != nil {
		t.Fatalf("suggest memory: %v", err)
	}
	content, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any", result.StructuredContent)
	}
	if content["suggested_action"] != "create" {
		t.Fatalf("suggested action = %q, want create", content["suggested_action"])
	}
	if content["recommended_candidate_id"] != "" {
		t.Fatalf("recommended candidate id = %q, want empty", content["recommended_candidate_id"])
	}
	if content["confidence"] != "none" {
		t.Fatalf("confidence = %q, want none", content["confidence"])
	}
	if content["next_step"] != "create_focused_memory" {
		t.Fatalf("next step = %q, want create_focused_memory", content["next_step"])
	}
	if content["candidate_count"] != 0 {
		t.Fatalf("candidate count = %v, want 0", content["candidate_count"])
	}
	candidates, ok := content["candidates"].([]suggestCandidateOutput)
	if !ok {
		t.Fatalf("candidates type = %T, want []suggestCandidateOutput", content["candidates"])
	}
	if len(candidates) != 0 {
		t.Fatalf("suggest candidates = %d, want 0", len(candidates))
	}
}

func TestMemorySuggestShortInputIsNotRecommended(t *testing.T) {
	ctx, server, projects, memories := newSuggestTestServer(t)
	if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "api", Name: "API"}); err != nil {
		t.Fatalf("upsert project: %v", err)
	}
	if _, err := memories.Upsert(ctx, memory.UpsertInput{
		Project:    "api",
		Type:       memory.TypeImplementation,
		Title:      "Invoice route handler",
		Content:    "Invoice route handler calls InvoiceController to validate invoice payloads.",
		Importance: 3,
		Enrich:     true,
	}); err != nil {
		t.Fatalf("upsert memory: %v", err)
	}

	result, err := server.handleMemorySuggest(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"project": "api",
				"input":   "Invoice route",
				"context": "Invoice route handler calls InvoiceController to validate invoice payloads.",
			},
		},
	})
	if err != nil {
		t.Fatalf("suggest memory: %v", err)
	}
	content, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any", result.StructuredContent)
	}
	if content["suggested_action"] != "ignore" {
		t.Fatalf("suggested action = %q, want ignore", content["suggested_action"])
	}
	if content["modifies_memory"] != false {
		t.Fatalf("modifies memory = %v, want false", content["modifies_memory"])
	}
	if content["next_step"] != "ignore_input" {
		t.Fatalf("next step = %q, want ignore_input", content["next_step"])
	}
	candidates, ok := content["candidates"].([]suggestCandidateOutput)
	if !ok {
		t.Fatalf("candidates type = %T, want []suggestCandidateOutput", content["candidates"])
	}
	for _, candidate := range candidates {
		if candidate.IsRecommended {
			t.Fatalf("candidate rank %d should not be recommended for ignore action", candidate.Rank)
		}
		if candidate.CandidateQuality != "weak" {
			t.Fatalf("candidate quality = %q, want weak", candidate.CandidateQuality)
		}
		if candidate.MatchReason != "input is too short to recommend an update" {
			t.Fatalf("match reason = %q, want ignore reason", candidate.MatchReason)
		}
	}
}

func TestMemorySuggestMultiSectionInputRequestsSplit(t *testing.T) {
	ctx, server, projects, _ := newSuggestTestServer(t)
	if _, err := projects.Upsert(ctx, project.UpsertInput{Key: "api", Name: "API"}); err != nil {
		t.Fatalf("upsert project: %v", err)
	}

	result, err := server.handleMemorySuggest(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"project": "api",
				"input":   "First durable fact about route handlers.\n\nSecond durable fact about repository indexing.\n\nThird durable fact about test workflow.",
			},
		},
	})
	if err != nil {
		t.Fatalf("suggest memory: %v", err)
	}
	content, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any", result.StructuredContent)
	}
	if content["suggested_action"] != "split" {
		t.Fatalf("suggested action = %q, want split", content["suggested_action"])
	}
	if content["next_step"] != "split_input" {
		t.Fatalf("next step = %q, want split_input", content["next_step"])
	}
	if content["recommended_candidate_id"] != "" {
		t.Fatalf("recommended candidate id = %q, want empty", content["recommended_candidate_id"])
	}
}

func newSuggestTestServer(t *testing.T) (
	context.Context,
	*Server,
	*project.Service,
	*memory.Service,
) {
	t.Helper()
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
	server := New(projects, memories, search.NewService(search.NewFTS(db)))

	return ctx, server, projects, memories
}
