package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/search"
)

func TestMemoryResponseHelpersExposeRetrievalHistory(t *testing.T) {
	retrievedAt := time.Date(2026, time.July, 14, 9, 30, 0, 0, time.UTC)
	m := memory.Memory{
		ID:             "memory-1",
		ProjectKey:     "project",
		Type:           memory.TypeDecision,
		Title:          "Response metrics",
		Importance:     3,
		AccessCount:    7,
		LastAccessedAt: &retrievedAt,
	}
	result := search.Result{Memory: m}

	outputs := []memoryOutput{
		newMemoryOutput(m, false),
		newMemoryOutputs([]memory.Memory{m}, false)[0],
		newSearchDebugOutputs([]search.Result{result})[0].Memory,
		newSuggestCandidateOutputs([]suggestCandidateEvaluation{{Result: result}})[0].Memory,
	}
	for _, output := range outputs {
		assertPublicRetrievalHistory(t, output, 7, retrievedAt.Format(time.RFC3339Nano))
	}
}

func TestMemoryResponseHelpersPreserveNilRetrievalTimestamp(t *testing.T) {
	output := newMemoryOutput(memory.Memory{AccessCount: 0}, false)
	assertPublicRetrievalHistory(t, output, 0, nil)
}

func TestSearchDebugResponseIncludesFullStructuredEvidence(t *testing.T) {
	result := search.Result{
		Memory:        memory.Memory{ID: "memory-1", ProjectKey: "project", Content: "PRIVATE-SENTINEL"},
		MatchedFields: []string{"summary", "content"}, SnippetField: "summary",
		Snippet: "[needle] summary", MatchExplanation: "precise search matched indexed fields: summary, content",
	}
	debug := newSearchDebugOutputs([]search.Result{result})[0]
	if len(debug.MatchedFields) != 2 || debug.SnippetField != "summary" || debug.Snippet != "[needle] summary" {
		t.Fatalf("debug evidence = %#v", debug)
	}
	encodedDebug, err := json.Marshal(debug)
	if err != nil {
		t.Fatalf("marshal debug response: %v", err)
	}
	if string(encodedDebug) == "" || json.Valid(encodedDebug) == false {
		t.Fatalf("invalid debug JSON: %s", encodedDebug)
	}
	if output := newSearchOutputs([]search.Result{result}, false)[0]; output.Content != "" {
		t.Fatalf("normal search response exposed content: %#v", output)
	}
}

func TestSearchResponseCardsGuideSelectionWithoutReplacingMemoryContent(t *testing.T) {
	updatedAt := time.Date(2026, time.September, 13, 10, 30, 0, 0, time.UTC)
	result := search.Result{
		Memory: memory.Memory{
			ID:           "memory-1",
			ProjectKey:   "project",
			Type:         memory.TypeDecision,
			Title:        "Invoice request contract",
			Content:      "Authoritative content with the complete invoice contract.",
			ShortSummary: "Preview of the invoice contract.",
			Importance:   5,
			Source:       "docs/invoices.md",
			UpdatedAt:    updatedAt,
		},
		SnippetField: "content",
		Snippet:      "Complete [invoice] contract.",
	}

	card := newSearchOutputs([]search.Result{result}, false)[0]
	if card.ID != result.Memory.ID || card.Title != result.Memory.Title {
		t.Fatalf("card identity = %#v", card)
	}
	if card.Description != result.Memory.ShortSummary {
		t.Fatalf("description = %q, want %q", card.Description, result.Memory.ShortSummary)
	}
	if card.Source != result.Memory.Source || card.UpdatedAt != updatedAt {
		t.Fatalf("card provenance = %#v", card)
	}
	if card.MatchField != result.SnippetField || card.MatchExcerpt != result.Snippet {
		t.Fatalf("card match guidance = %#v", card)
	}
	if card.Content != "" {
		t.Fatalf("default card exposed authoritative content: %#v", card)
	}

	encoded, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal card: %v", err)
	}
	requiredFields := []string{
		"id", "title", "description", "source", "updated_at", "match_excerpt", "match_field",
	}
	for _, required := range requiredFields {
		if _, found := fields[required]; !found {
			t.Fatalf("default card is missing required field %q: %s", required, encoded)
		}
	}
	excludedFields := []string{
		"project", "type", "importance", "retrieval_count", "last_retrieved_at", "created_at", "content",
	}
	for _, excluded := range excludedFields {
		if _, found := fields[excluded]; found {
			t.Fatalf("default card contains excluded field %q: %s", excluded, encoded)
		}
	}

	full := newSearchOutputs([]search.Result{result}, true)[0]
	if full.Content != result.Memory.Content {
		t.Fatalf("explicit full search content = %q, want %q", full.Content, result.Memory.Content)
	}
}

func TestSearchResponseDescriptionHasDeterministicFallbackAndBound(t *testing.T) {
	longContent := strings.Repeat("memòria ", 40)
	card := newSearchOutputs([]search.Result{{
		Memory: memory.Memory{
			ID:      "memory-1",
			Content: longContent,
		},
	}}, false)[0]

	if card.Description == "" {
		t.Fatal("description fallback is empty")
	}
	if got := len([]rune(card.Description)); got > 160 {
		t.Fatalf("description length = %d runes, want at most 160", got)
	}
}

func TestSearchResponseCardVolumeComparedWithPreviousProjections(t *testing.T) {
	content := strings.Repeat("authoritative project memory content ", 80)
	retrievedAt := time.Date(2026, time.September, 13, 9, 0, 0, 0, time.UTC)
	m := memory.Memory{
		ID:             "memory-1",
		ProjectKey:     "project",
		Type:           memory.TypeArchitecture,
		Title:          "Project memory architecture",
		Content:        content,
		ShortSummary:   "Compact topic preview for candidate selection.",
		Importance:     4,
		Source:         "docs/architecture.md",
		AccessCount:    12,
		LastAccessedAt: &retrievedAt,
		CreatedAt:      time.Date(2026, time.September, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, time.September, 13, 10, 30, 0, 0, time.UTC),
	}
	result := search.Result{
		Memory:       m,
		SnippetField: "content",
		Snippet:      "...[project] memory content...",
	}

	cardJSON, err := json.Marshal(newSearchOutputs([]search.Result{result}, false))
	if err != nil {
		t.Fatalf("marshal card projection: %v", err)
	}
	previousSlimJSON, err := json.Marshal(newMemoryOutputs([]memory.Memory{m}, false))
	if err != nil {
		t.Fatalf("marshal previous slim projection: %v", err)
	}
	previousFullJSON, err := json.Marshal(newMemoryOutputs([]memory.Memory{m}, true))
	if err != nil {
		t.Fatalf("marshal previous full projection: %v", err)
	}
	if len(cardJSON) >= len(previousFullJSON) {
		t.Fatalf("card bytes = %d, want fewer than previous full bytes %d", len(cardJSON), len(previousFullJSON))
	}
	t.Logf(
		"serialized bytes: card=%d previous_slim=%d previous_full=%d",
		len(cardJSON),
		len(previousSlimJSON),
		len(previousFullJSON),
	)
}

func assertPublicRetrievalHistory(t *testing.T, output memoryOutput, wantCount int, wantRetrievedAt any) {
	t.Helper()
	encoded, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("marshal memory output: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal memory output: %v", err)
	}
	if got := fields["retrieval_count"]; got != float64(wantCount) {
		t.Fatalf("retrieval_count = %#v, want %d", got, wantCount)
	}
	if got := fields["last_retrieved_at"]; got != wantRetrievedAt {
		t.Fatalf("last_retrieved_at = %#v, want %#v", got, wantRetrievedAt)
	}
	for _, legacyField := range []string{"access_count", "last_accessed_at"} {
		if _, found := fields[legacyField]; found {
			t.Fatalf("public output contains legacy field %q", legacyField)
		}
	}
}
