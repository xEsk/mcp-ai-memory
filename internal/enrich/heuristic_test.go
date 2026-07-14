package enrich

import (
	"context"
	"testing"
)

func TestLocalHeuristicEnricher_ExtractsRetrievalFields(t *testing.T) {
	enricher := NewLocalHeuristicEnricher()
	out, err := enricher.EnrichMemory(context.Background(), EnrichInput{
		Project:    "panel",
		Type:       "api",
		Title:      "Axios Server wrapper",
		Content:    "The Server class wraps Axios and uses /api/phproute as the base endpoint.",
		Tags:       []string{"frontend"},
		Importance: 3,
	})
	if err != nil {
		t.Fatalf("enrich memory: %v", err)
	}
	if out.Summary == "" {
		t.Fatalf("summary is empty")
	}
	if !contains(out.Tags, "frontend") || !contains(out.Tags, "api") {
		t.Fatalf("tags = %v, want frontend and api", out.Tags)
	}
	if !contains(out.Entities, "Server") && !contains(out.Entities, "/api/phproute") {
		t.Fatalf("entities = %v, want technical names", out.Entities)
	}
	if len(out.SearchPhrases) == 0 {
		t.Fatalf("search phrases are empty")
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
