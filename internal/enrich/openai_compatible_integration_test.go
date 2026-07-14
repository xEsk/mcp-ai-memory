//go:build integration

package enrich

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestOpenAICompatibleEnricher_EnrichMemory(t *testing.T) {
	baseURL := os.Getenv("MEMORY_LLM_BASE_URL")
	apiKey := os.Getenv("MEMORY_LLM_API_KEY")
	model := os.Getenv("MEMORY_LLM_MODEL")
	if baseURL == "" || apiKey == "" || model == "" {
		t.Skip("MEMORY_LLM_BASE_URL, MEMORY_LLM_API_KEY and MEMORY_LLM_MODEL are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	enricher := NewOpenAICompatibleEnricher(baseURL, apiKey, model)
	out, err := enricher.EnrichMemory(ctx, EnrichInput{
		Project:    "mcp-ai-memory",
		Title:      "OpenAI-compatible enrichment integration test",
		Content:    "memory.upsert sends technical content to a chat completions endpoint and stores strict JSON retrieval metadata while preserving original content.",
		Tags:       []string{"ai-enrichment", "integration-test"},
		Importance: 2,
	})
	if err != nil {
		t.Fatalf("enrich memory: %v", err)
	}
	if out.Summary == "" {
		t.Fatalf("summary is empty")
	}
	if len(out.Keywords) == 0 {
		t.Fatalf("keywords are empty")
	}
	if len(out.SearchPhrases) == 0 {
		t.Fatalf("search phrases are empty")
	}
}
