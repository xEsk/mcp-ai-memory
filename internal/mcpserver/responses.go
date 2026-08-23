package mcpserver

import (
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

type searchDebugOutput struct {
	Memory           memoryOutput `json:"memory"`
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

func newSearchOutputs(results []search.Result, includeContent bool) []memoryOutput {
	out := make([]memoryOutput, 0, len(results))
	for _, result := range results {
		out = append(out, newMemoryOutput(result.Memory, includeContent))
	}
	return out
}

func newSearchDebugOutputs(results []search.Result) []searchDebugOutput {
	out := make([]searchDebugOutput, 0, len(results))
	for _, result := range results {
		out = append(out, searchDebugOutput{
			Memory:           newMemoryOutput(result.Memory, false),
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
