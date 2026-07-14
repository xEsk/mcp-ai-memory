package enrich

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var tokenPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_./:-]{2,}`)

// LocalHeuristicEnricher generates basic metadata without external calls.
type LocalHeuristicEnricher struct{}

// NewLocalHeuristicEnricher creates the default local enricher.
func NewLocalHeuristicEnricher() *LocalHeuristicEnricher {
	return &LocalHeuristicEnricher{}
}

// EnrichMemory extracts summaries, keywords, and entities with deterministic heuristics.
func (e *LocalHeuristicEnricher) EnrichMemory(ctx context.Context, input EnrichInput) (*EnrichOutput, error) {
	_ = ctx
	text := strings.TrimSpace(input.Title + "\n" + input.Content)
	summary := firstSentence(input.Content, 420)
	short := firstSentence(input.Content, 160)
	keywords := topTokens(text, 24)
	entities := technicalEntities(text, 24)
	tags := mergeUnique(input.Tags, inferTags(input.Type, text))
	phrases := searchPhrases(input, keywords, entities)

	return &EnrichOutput{
		Summary:             summary,
		ShortSummary:        short,
		Keywords:            keywords,
		Tags:                tags,
		Entities:            entities,
		Aliases:             aliasesFor(keywords, entities),
		SearchPhrases:       phrases,
		SuggestedType:       input.Type,
		SuggestedImportance: normalizeImportance(input.Importance),
	}, nil
}

func firstSentence(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= limit {
		return text
	}
	cut := strings.LastIndexAny(text[:limit], ".;")
	if cut < limit/3 {
		cut = strings.LastIndex(text[:limit], " ")
	}
	if cut <= 0 {
		cut = limit
	}
	return strings.TrimSpace(text[:cut])
}

func topTokens(text string, limit int) []string {
	counts := map[string]int{}
	for _, token := range tokenPattern.FindAllString(text, -1) {
		key := strings.ToLower(strings.Trim(token, ".,;()[]{}"))
		if stopWords[key] || len(key) < 3 {
			continue
		}
		counts[key]++
	}
	type pair struct {
		token string
		count int
	}
	var pairs []pair
	for token, count := range counts {
		pairs = append(pairs, pair{token: token, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].token < pairs[j].token
		}
		return pairs[i].count > pairs[j].count
	})
	var out []string
	for _, pair := range pairs {
		out = append(out, pair.token)
		if len(out) == limit {
			break
		}
	}
	return out
}

func technicalEntities(text string, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, token := range tokenPattern.FindAllString(text, -1) {
		if seen[token] {
			continue
		}
		if strings.ContainsAny(token, "./:_-") || hasUpper(token) {
			seen[token] = true
			out = append(out, token)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

func inferTags(memoryType, text string) []string {
	lower := strings.ToLower(text)
	tags := []string{memoryType}
	candidates := map[string][]string{
		"api":        {"api", "endpoint", "http", "route", "axios", "fetch"},
		"database":   {"sql", "sqlite", "repository", "migration", "query"},
		"frontend":   {"ui", "component", "react", "vue", "css"},
		"backend":    {"service", "handler", "controller", "middleware"},
		"workflow":   {"build", "test", "deploy", "makefile", "command"},
		"bug":        {"bug", "error", "fallback", "fix", "regression"},
		"convention": {"convention", "style", "pattern", "naming"},
	}
	for tag, words := range candidates {
		for _, word := range words {
			if strings.Contains(lower, word) {
				tags = append(tags, tag)
				break
			}
		}
	}
	return tags
}

func searchPhrases(input EnrichInput, keywords, entities []string) []string {
	base := []string{
		"how does " + input.Project + " handle " + input.Title,
		"where is " + input.Title + " implemented",
		"project convention " + input.Title,
	}
	for _, token := range append(keywords, entities...) {
		base = append(base, "how do we use "+token, "where is "+token)
		if len(base) > 18 {
			break
		}
	}
	return mergeUnique(base, nil)
}

func aliasesFor(keywords, entities []string) []string {
	var aliases []string
	for _, token := range append(keywords, entities...) {
		if strings.Contains(token, "api") {
			aliases = append(aliases, "http client", "route helper", "endpoint")
		}
		if strings.Contains(token, "repository") {
			aliases = append(aliases, "storage", "persistence", "queries")
		}
	}
	return mergeUnique(aliases, nil)
}

func normalizeImportance(value int) int {
	if value < 1 {
		return 1
	}
	if value > 5 {
		return 5
	}
	return value
}

func mergeUnique(groups ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, group := range groups {
		for _, value := range group {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func hasUpper(value string) bool {
	for _, r := range value {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true, "this": true,
	"from": true, "are": true, "como": true, "para": true, "que": true, "con": true,
	"los": true, "las": true, "una": true, "por": true, "del": true,
}
