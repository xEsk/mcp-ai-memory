package mcpserver

import (
	"encoding/json"
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
		newSearchOutputs([]search.Result{result}, false)[0],
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
