package mcpserver

import (
	"strings"
	"time"

	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/search"
)

type memoryOutput struct {
	ID              string     `json:"id"`
	Project         string     `json:"project"`
	Type            string     `json:"type"`
	Title           string     `json:"title"`
	Content         string     `json:"content,omitempty"`
	Importance      int        `json:"importance"`
	Source          string     `json:"source,omitempty"`
	RetrievalCount  int        `json:"retrieval_count"`
	LastRetrievedAt *time.Time `json:"last_retrieved_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type searchCardOutput struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Source       string    `json:"source"`
	UpdatedAt    time.Time `json:"updated_at"`
	MatchExcerpt string    `json:"match_excerpt"`
	MatchField   string    `json:"match_field"`
	Content      string    `json:"content,omitempty"`
}

type searchDebugOutput struct {
	Memory           memoryOutput `json:"memory"`
	MatchedFields    []string     `json:"matched_fields"`
	SnippetField     string       `json:"snippet_field"`
	Snippet          string       `json:"snippet"`
	Score            float64      `json:"score"`
	MatchExplanation string       `json:"match_explanation"`
}

type suggestCandidateOutput struct {
	Rank             int          `json:"rank"`
	IsRecommended    bool         `json:"is_recommended"`
	CandidateQuality string       `json:"candidate_quality"`
	Memory           memoryOutput `json:"memory"`
	MatchReason      string       `json:"match_reason"`
}

func newMemoryOutput(m memory.Memory, includeContent bool) memoryOutput {
	out := memoryOutput{
		ID:              m.ID,
		Project:         m.ProjectKey,
		Type:            m.Type,
		Title:           m.Title,
		Importance:      m.Importance,
		Source:          m.Source,
		RetrievalCount:  m.AccessCount,
		LastRetrievedAt: m.LastAccessedAt,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	if includeContent {
		out.Content = m.Content
	}
	return out
}

func newMemoryOutputs(memories []memory.Memory, includeContent bool) []memoryOutput {
	out := make([]memoryOutput, 0, len(memories))
	for _, item := range memories {
		out = append(out, newMemoryOutput(item, includeContent))
	}
	return out
}

func newSearchOutputs(results []search.Result, includeContent bool) []searchCardOutput {
	out := make([]searchCardOutput, 0, len(results))
	for _, result := range results {
		card := searchCardOutput{
			ID:           result.Memory.ID,
			Title:        result.Memory.Title,
			Description:  searchDescription(result.Memory),
			Source:       result.Memory.Source,
			UpdatedAt:    result.Memory.UpdatedAt,
			MatchExcerpt: result.Snippet,
			MatchField:   result.SnippetField,
		}
		if includeContent {
			card.Content = result.Memory.Content
		}
		out = append(out, card)
	}
	return out
}

func searchDescription(m memory.Memory) string {
	description := strings.TrimSpace(m.ShortSummary)
	if description == "" {
		description = strings.TrimSpace(m.Content)
	}
	description = strings.Join(strings.Fields(description), " ")
	const descriptionLimit = 160
	runes := []rune(description)
	if len(runes) <= descriptionLimit {
		return description
	}
	return strings.TrimSpace(string(runes[:descriptionLimit-1])) + "…"
}

func newSearchDebugOutputs(results []search.Result) []searchDebugOutput {
	out := make([]searchDebugOutput, 0, len(results))
	for _, result := range results {
		out = append(out, searchDebugOutput{
			Memory:           newMemoryOutput(result.Memory, false),
			MatchedFields:    result.MatchedFields,
			SnippetField:     result.SnippetField,
			Snippet:          result.Snippet,
			Score:            result.Score,
			MatchExplanation: result.MatchExplanation,
		})
	}
	return out
}

func newSuggestCandidateOutputs(candidates []suggestCandidateEvaluation) []suggestCandidateOutput {
	out := make([]suggestCandidateOutput, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, suggestCandidateOutput{
			Rank:             candidate.Rank,
			IsRecommended:    candidate.IsRecommended,
			CandidateQuality: candidate.Quality,
			Memory:           newMemoryOutput(candidate.Result.Memory, false),
			MatchReason:      candidate.MatchReason,
		})
	}
	return out
}
