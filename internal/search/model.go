package search

import "mcp-ai-memory/internal/memory"

const (
	ModePrecise  = "precise"
	ModeBalanced = "balanced"
	ModeBroad    = "broad"
)

// Input contains contextual search filters.
type Input struct {
	Project string   `json:"project"`
	Query   string   `json:"query"`
	Mode    string   `json:"mode"`
	Limit   int      `json:"limit"`
	Types   []string `json:"types"`
	Tags    []string `json:"tags"`
}

// Result represents a matched memory and why it matched.
type Result struct {
	Memory           memory.Memory `json:"memory"`
	MatchedFields    []string      `json:"matched_fields"`
	SnippetField     string        `json:"snippet_field"`
	Snippet          string        `json:"snippet"`
	Score            float64       `json:"score"`
	MatchExplanation string        `json:"match_explanation"`
}
