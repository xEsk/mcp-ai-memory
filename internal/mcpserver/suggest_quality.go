package mcpserver

import (
	"sort"
	"strings"

	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/retrievaltext"
	"mcp-ai-memory/internal/search"
)

const (
	suggestQualityStrong   = "strong"
	suggestQualityPossible = "possible"
	suggestQualityWeak     = "weak"
)

type suggestCandidateEvaluation struct {
	Rank          int
	Result        search.Result
	Quality       string
	EvidenceScore int
	IsRecommended bool
	MatchReason   string
}

func evaluateSuggestCandidates(query search.CompiledQuery, results []search.Result) []suggestCandidateEvaluation {
	queryTerms := compiledSuggestTerms(query)
	candidates := make([]suggestCandidateEvaluation, 0, len(results))
	for i, result := range results {
		quality, evidence := evaluateSuggestCandidate(queryTerms, query.Mode, result.Memory)
		candidates = append(candidates, suggestCandidateEvaluation{
			Rank:          i + 1,
			Result:        result,
			Quality:       quality,
			EvidenceScore: evidence,
			MatchReason:   suggestMatchReason(quality),
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := suggestQualityRank(candidates[i].Quality), suggestQualityRank(candidates[j].Quality)
		if left != right {
			return left > right
		}
		if candidates[i].EvidenceScore != candidates[j].EvidenceScore {
			return candidates[i].EvidenceScore > candidates[j].EvidenceScore
		}
		return candidates[i].Rank < candidates[j].Rank
	})
	for i := range candidates {
		candidates[i].Rank = i + 1
	}
	markRecommendedSuggestCandidate(candidates)
	return candidates
}

func suggestQualityRank(quality string) int {
	switch quality {
	case suggestQualityStrong:
		return 3
	case suggestQualityPossible:
		return 2
	default:
		return 1
	}
}

func evaluateSuggestCandidate(queryTerms map[string]bool, mode string, candidate memory.Memory) (string, int) {
	titleTerms := importantTerms(candidate.Title)
	metadataTerms := importantTerms(candidateMetadataText(candidate))

	titleOverlap, titleSpecific := overlapCounts(queryTerms, titleTerms)
	metadataOverlap, metadataSpecific := overlapCounts(queryTerms, metadataTerms)
	exactTitleBonus := 0
	if containsNormalizedPhrase(queryTerms, candidate.Title) {
		exactTitleBonus = 4
	}

	evidence := titleOverlap*4 + metadataOverlap*2 + exactTitleBonus
	specificEvidence := titleSpecific*4 + metadataSpecific*2 + exactTitleBonus
	if mode == search.ModePrecise {
		evidence++
		specificEvidence++
	}

	if specificEvidence == 0 {
		return suggestQualityWeak, evidence
	}
	if titleSpecific >= 2 && specificEvidence >= 9 {
		return suggestQualityStrong, evidence
	}
	if mode == search.ModePrecise && titleSpecific >= 1 && metadataSpecific >= 3 && specificEvidence >= 9 {
		return suggestQualityStrong, evidence
	}
	if titleSpecific >= 1 || metadataSpecific >= 2 {
		return suggestQualityPossible, evidence
	}
	return suggestQualityWeak, evidence
}

func markRecommendedSuggestCandidate(candidates []suggestCandidateEvaluation) {
	if len(candidates) == 0 || candidates[0].Quality != suggestQualityStrong {
		return
	}
	if len(candidates) > 1 && candidates[1].Quality != suggestQualityWeak {
		if candidates[0].EvidenceScore-candidates[1].EvidenceScore < 2 {
			for i := range candidates {
				if candidates[i].Quality == suggestQualityStrong {
					candidates[i].Quality = suggestQualityPossible
					candidates[i].MatchReason = suggestMatchReason(suggestQualityPossible)
				}
			}
			return
		}
	}
	candidates[0].IsRecommended = true
}

func applySuggestActionToCandidates(candidates []suggestCandidateEvaluation, action string) []suggestCandidateEvaluation {
	for i := range candidates {
		switch action {
		case "update_existing":
			if candidates[i].IsRecommended {
				candidates[i].MatchReason = suggestMatchReason(suggestQualityStrong)
			}
		case "ignore":
			candidates[i].IsRecommended = false
			candidates[i].Quality = suggestQualityWeak
			candidates[i].MatchReason = "input is too short to recommend an update"
		case "split":
			candidates[i].IsRecommended = false
			candidates[i].Quality = suggestQualityPossible
			candidates[i].MatchReason = "input may contain multiple memories"
		default:
			candidates[i].IsRecommended = false
		}
	}
	return candidates
}

func recommendedSuggestCandidateID(candidates []suggestCandidateEvaluation) string {
	for _, candidate := range candidates {
		if candidate.IsRecommended {
			return candidate.Result.Memory.ID
		}
	}
	return ""
}

func suggestMatchReason(quality string) string {
	switch quality {
	case suggestQualityStrong:
		return "strong title and metadata overlap"
	case suggestQualityPossible:
		return "possible semantic overlap, review before updating"
	default:
		return "matched only broad project terms"
	}
}

func candidateMetadataText(m memory.Memory) string {
	parts := []string{
		m.Content,
		m.Summary,
		m.ShortSummary,
		strings.Join(m.Keywords, " "),
		strings.Join(m.Tags, " "),
		strings.Join(m.Entities, " "),
		strings.Join(m.Aliases, " "),
		strings.Join(m.SearchPhrases, " "),
	}
	return strings.Join(parts, " ")
}

func importantTerms(text string) map[string]bool {
	terms := map[string]bool{}
	for _, token := range retrievaltext.Tokenize(text) {
		if suggestStopWords[token.Identity] {
			continue
		}
		terms[token.Identity] = true
	}
	return terms
}

func compiledSuggestTerms(query search.CompiledQuery) map[string]bool {
	terms := make(map[string]bool, len(query.Terms))
	for _, term := range query.Terms {
		terms[term.Identity] = true
	}
	return terms
}

func overlapCounts(queryTerms, candidateTerms map[string]bool) (int, int) {
	overlap := 0
	specific := 0
	for term := range queryTerms {
		if !candidateTerms[term] {
			continue
		}
		overlap++
		if !suggestGenericTerms[term] {
			specific++
		}
	}
	return overlap, specific
}

func containsNormalizedPhrase(queryTerms map[string]bool, phrase string) bool {
	phraseTerms := importantTerms(phrase)
	if len(phraseTerms) == 0 {
		return false
	}
	for term := range phraseTerms {
		if !queryTerms[term] {
			return false
		}
	}
	return true
}

var suggestStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true,
	"at": true, "be": true, "by": true, "can": true, "do": true,
	"does": true, "for": true, "from": true, "has": true, "have": true,
	"how": true, "in": true, "is": true, "it": true, "of": true,
	"on": true, "or": true, "that": true, "the": true, "this": true,
	"to": true, "we": true, "what": true, "when": true, "where": true,
	"which": true, "who": true, "why": true, "with": true,
	"com": true, "como": true, "del": true, "dels": true, "els": true,
	"las": true, "les": true, "los": true, "per": true, "por": true,
	"que": true, "una": true,
}

var suggestGenericTerms = map[string]bool{
	"agent": true, "agents": true, "create": true, "created": true,
	"document": true, "documented": true, "documentation": true,
	"future": true, "memory": true, "memories": true, "mcp": true,
	"note": true, "notes": true, "project": true, "task": true,
	"tasks": true, "tool": true, "tools": true, "update": true,
	"updated": true, "work": true, "workflow": true, "works": true,
}
