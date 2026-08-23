package enrich

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"unicode/utf8"
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

func TestLocalHeuristicEnricherUnicodeDeterministicExactOutput(t *testing.T) {
	input := EnrichInput{
		Project:    "alpha",
		Type:       "fact",
		Title:      "München Go API repository",
		Content:    "München uses io.Reader with API repository.",
		Tags:       []string{"Frontend", "frontend"},
		Importance: 7,
	}
	want := &EnrichOutput{
		Summary:      "München uses io.Reader with API repository.",
		ShortSummary: "München uses io.Reader with API repository.",
		Keywords:     []string{"api", "münchen", "repository", "go", "io", "reader", "uses"},
		Tags:         []string{"Frontend", "fact", "api", "database"},
		Entities:     []string{"München", "Go", "API", "io.Reader"},
		Aliases:      []string{"http client", "route helper", "endpoint", "storage", "persistence", "queries"},
		SearchPhrases: []string{
			"how does alpha handle München Go API repository",
			"where is München Go API repository implemented",
			"project convention München Go API repository",
			"how do we use api", "where is api", "how do we use münchen", "where is münchen",
			"how do we use repository", "where is repository", "how do we use go", "where is go",
			"how do we use io", "where is io", "how do we use reader", "where is reader",
			"how do we use uses", "where is uses", "how do we use io.Reader",
		},
		SuggestedType:       "fact",
		SuggestedImportance: 5,
	}

	enricher := NewLocalHeuristicEnricher()
	var encoded []byte
	for i := 0; i < 20; i++ {
		got, err := enricher.EnrichMemory(context.Background(), input)
		if err != nil {
			t.Fatalf("enrich memory: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("output = %#v, want %#v", got, want)
		}
		current, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal output: %v", err)
		}
		if encoded != nil && string(current) != string(encoded) {
			t.Fatalf("output changed between runs:\n%s\n%s", encoded, current)
		}
		encoded = current
	}
}

func TestLocalHeuristicEnricherHardNegativeSubstrings(t *testing.T) {
	enricher := NewLocalHeuristicEnricher()
	for _, test := range []struct {
		text         string
		forbiddenTag string
		wantAliases  int
	}{
		{text: "build", forbiddenTag: "frontend"},
		{text: "contest", forbiddenTag: "workflow"},
		{text: "capitalization", forbiddenTag: "api"},
	} {
		t.Run(test.text, func(t *testing.T) {
			out, err := enricher.EnrichMemory(context.Background(), EnrichInput{Type: "fact", Title: test.text, Importance: 3})
			if err != nil {
				t.Fatalf("enrich memory: %v", err)
			}
			if contains(out.Tags, test.forbiddenTag) {
				t.Fatalf("tags = %v, must not contain %q", out.Tags, test.forbiddenTag)
			}
			if len(out.Aliases) != test.wantAliases {
				t.Fatalf("aliases = %v, want none", out.Aliases)
			}
		})
	}
}

func TestLocalHeuristicEnricherUsesRuneSafeSummaryAndHardPhraseCap(t *testing.T) {
	content := "München alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima november oscar papa quebec romeo sierra tango "
	for i := 0; i < 100; i++ {
		content += "àmbit "
	}
	out, err := NewLocalHeuristicEnricher().EnrichMemory(context.Background(), EnrichInput{
		Project: "alpha", Type: "fact", Title: "Unicode", Content: content, Importance: 3,
	})
	if err != nil {
		t.Fatalf("enrich memory: %v", err)
	}
	if !utf8.ValidString(out.Summary) || !utf8.ValidString(out.ShortSummary) {
		t.Fatalf("summaries are not valid UTF-8: %q / %q", out.Summary, out.ShortSummary)
	}
	if len([]rune(out.Summary)) > 420 || len([]rune(out.ShortSummary)) > 160 {
		t.Fatalf("summary rune limits exceeded: %d / %d", len([]rune(out.Summary)), len([]rune(out.ShortSummary)))
	}
	if len(out.SearchPhrases) != searchPhraseLimit {
		t.Fatalf("search phrases = %d, want hard cap %d", len(out.SearchPhrases), searchPhraseLimit)
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
