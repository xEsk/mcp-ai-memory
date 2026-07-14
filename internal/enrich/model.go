package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Enricher generates retrieval metadata from an original memory.
type Enricher interface {
	EnrichMemory(ctx context.Context, input EnrichInput) (*EnrichOutput, error)
}

// EnrichInput describes the content to enrich.
type EnrichInput struct {
	Project    string
	Type       string
	Title      string
	Content    string
	Tags       []string
	Importance int
}

// EnrichOutput contains metadata optimized for agent retrieval.
type EnrichOutput struct {
	Summary             string        `json:"summary"`
	ShortSummary        string        `json:"short_summary"`
	Keywords            []string      `json:"keywords"`
	Tags                []string      `json:"tags"`
	Entities            []string      `json:"entities"`
	Aliases             []string      `json:"aliases"`
	SearchPhrases       []string      `json:"search_phrases"`
	SuggestedType       string        `json:"suggested_type"`
	SuggestedImportance int           `json:"suggested_importance"`
	Chunks              []ChunkOutput `json:"chunks"`
}

// UnmarshalJSON accepts suggested_importance as a number or numeric string.
func (o *EnrichOutput) UnmarshalJSON(data []byte) error {
	var raw struct {
		Summary             string          `json:"summary"`
		ShortSummary        string          `json:"short_summary"`
		Keywords            json.RawMessage `json:"keywords"`
		Tags                json.RawMessage `json:"tags"`
		Entities            json.RawMessage `json:"entities"`
		Aliases             json.RawMessage `json:"aliases"`
		SearchPhrases       json.RawMessage `json:"search_phrases"`
		SuggestedType       string          `json:"suggested_type"`
		SuggestedImportance any             `json:"suggested_importance"`
		Chunks              []rawChunk      `json:"chunks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	o.Summary = raw.Summary
	o.ShortSummary = raw.ShortSummary
	o.Keywords = parseStringSlice(raw.Keywords)
	o.Tags = parseStringSlice(raw.Tags)
	o.Entities = parseStringSlice(raw.Entities)
	o.Aliases = parseStringSlice(raw.Aliases)
	o.SearchPhrases = parseStringSlice(raw.SearchPhrases)
	o.SuggestedType = raw.SuggestedType
	o.SuggestedImportance = parseImportance(raw.SuggestedImportance)
	o.Chunks = parseChunks(raw.Chunks)
	return nil
}

type rawChunk struct {
	Title    string          `json:"title"`
	Summary  string          `json:"summary"`
	Content  string          `json:"content"`
	Keywords json.RawMessage `json:"keywords"`
}

func parseImportance(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case string:
		parsed, err := strconv.Atoi(v)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func parseChunks(chunks []rawChunk) []ChunkOutput {
	out := make([]ChunkOutput, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, ChunkOutput{
			Title:    chunk.Title,
			Summary:  chunk.Summary,
			Content:  chunk.Content,
			Keywords: parseStringSlice(chunk.Keywords),
		})
	}
	return out
}

func parseStringSlice(data json.RawMessage) []string {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var values []any
	if err := json.Unmarshal(data, &values); err == nil {
		out := make([]string, 0, len(values))
		for _, value := range values {
			if parsed := stringifyListValue(value); parsed != "" {
				out = append(out, parsed)
			}
		}
		return out
	}
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		return splitListString(single)
	}
	return nil
}

func stringifyListValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(v, 'f', -1, 64))
	case map[string]any:
		for _, key := range []string{"name", "value", "text", "term", "entity", "alias", "phrase", "keyword", "tag"} {
			if raw, ok := v[key]; ok {
				if parsed := stringifyListValue(raw); parsed != "" {
					return parsed
				}
			}
		}
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(data)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func splitListString(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	if len(parts) == 1 {
		return []string{value}
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ChunkOutput prepares future splitting of long memories.
type ChunkOutput struct {
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Content  string   `json:"content"`
	Keywords []string `json:"keywords"`
}
