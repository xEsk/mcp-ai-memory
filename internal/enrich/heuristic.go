package enrich

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"mcp-ai-memory/internal/retrievaltext"
)

const searchPhraseLimit = 18

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
	keywords := topTokens(text, 24)
	entities := technicalEntities(text, 24)
	return &EnrichOutput{
		Summary:             firstSentence(input.Content, 420),
		ShortSummary:        firstSentence(input.Content, 160),
		Keywords:            keywords,
		Tags:                mergeUnique(input.Tags, inferTags(input.Type, text)),
		Entities:            entities,
		Aliases:             aliasesFor(text),
		SearchPhrases:       searchPhrases(input, keywords, entities),
		SuggestedType:       input.Type,
		SuggestedImportance: normalizeImportance(input.Importance),
	}, nil
}

func firstSentence(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	cut := lastRuneIndexAny(runes[:limit], ".;")
	if cut < limit/3 {
		cut = lastRuneIndexAny(runes[:limit], " ")
	}
	if cut <= 0 {
		cut = limit
	}
	return strings.TrimSpace(string(runes[:cut]))
}

func lastRuneIndexAny(runes []rune, chars string) int {
	for i := len(runes) - 1; i >= 0; i-- {
		if strings.ContainsRune(chars, runes[i]) {
			return i
		}
	}
	return -1
}

func topTokens(text string, limit int) []string {
	type tokenCount struct {
		text  string
		count int
	}
	counts := map[string]tokenCount{}
	for _, token := range retrievaltext.TokenizeAll(text) {
		if stopWords[token.Identity] || (token.RuneCount < 3 && !shortTechnicalTokens[token.Identity]) {
			continue
		}
		current := counts[token.Identity]
		if current.text == "" {
			current.text = token.Text
		}
		current.count++
		counts[token.Identity] = current
	}
	type pair struct {
		token string
		count int
	}
	pairs := make([]pair, 0, len(counts))
	for _, count := range counts {
		pairs = append(pairs, pair{token: count.text, count: count.count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].token < pairs[j].token
		}
		return pairs[i].count > pairs[j].count
	})
	out := make([]string, 0, min(limit, len(pairs)))
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
	for _, candidate := range technicalCandidates(text) {
		identity := retrievaltext.FoldIdentity(candidate)
		if seen[identity] || (!strings.ContainsAny(candidate, "./:_-") && !hasUpper(candidate)) {
			continue
		}
		seen[identity] = true
		out = append(out, candidate)
		if len(out) == limit {
			break
		}
	}
	return out
}

func technicalCandidates(text string) []string {
	var out []string
	var current []rune
	flush := func() {
		candidate := strings.Trim(string(current), `,;()[]{}!?¿¡"'`)
		candidate = strings.TrimRight(candidate, ".")
		current = current[:0]
		if candidate != "" {
			out = append(out, candidate)
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || strings.ContainsRune("./:_-", r) {
			current = append(current, r)
			continue
		}
		flush()
	}
	flush()
	return out
}

type tagRule struct {
	tag     string
	phrases []string
}

var orderedTagRules = []tagRule{
	{tag: "api", phrases: []string{"api", "endpoint", "http", "route", "axios", "fetch"}},
	{tag: "database", phrases: []string{"sql", "sqlite", "repository", "migration", "query"}},
	{tag: "frontend", phrases: []string{"ui", "component", "react", "vue", "css"}},
	{tag: "backend", phrases: []string{"service", "handler", "controller", "middleware"}},
	{tag: "workflow", phrases: []string{"build", "test", "deploy", "makefile", "command"}},
	{tag: "bug", phrases: []string{"bug", "error", "fallback", "fix", "regression"}},
	{tag: "convention", phrases: []string{"convention", "style", "pattern", "naming"}},
}

func inferTags(memoryType, text string) []string {
	tokens := normalizedTokenTexts(text)
	var tags []string
	if strings.TrimSpace(memoryType) != "" {
		tags = append(tags, memoryType)
	}
	for _, rule := range orderedTagRules {
		if containsAnyPhrase(tokens, rule.phrases) {
			tags = append(tags, rule.tag)
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
	for _, token := range append(append([]string{}, keywords...), entities...) {
		base = append(base, "how do we use "+token, "where is "+token)
	}
	return mergeUniqueLimit(searchPhraseLimit, base...)
}

type aliasRule struct {
	phrase string
	values []string
}

var orderedAliasRules = []aliasRule{
	{phrase: "api", values: []string{"http client", "route helper", "endpoint"}},
	{phrase: "repository", values: []string{"storage", "persistence", "queries"}},
}

func aliasesFor(text string) []string {
	tokens := normalizedTokenTexts(text)
	var aliases []string
	for _, rule := range orderedAliasRules {
		if containsPhrase(tokens, normalizedTokenTexts(rule.phrase)) {
			aliases = append(aliases, rule.values...)
		}
	}
	return mergeUnique(aliases)
}

func normalizedTokenTexts(text string) []string {
	tokens := retrievaltext.TokenizeAll(text)
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		out = append(out, token.Identity)
	}
	return out
}

func containsAnyPhrase(tokens []string, phrases []string) bool {
	for _, phrase := range phrases {
		if containsPhrase(tokens, normalizedTokenTexts(phrase)) {
			return true
		}
	}
	return false
}

func containsPhrase(tokens, phrase []string) bool {
	if len(phrase) == 0 || len(phrase) > len(tokens) {
		return false
	}
	for start := 0; start <= len(tokens)-len(phrase); start++ {
		matched := true
		for i := range phrase {
			if tokens[start+i] != phrase[i] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
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
	var values []string
	for _, group := range groups {
		values = append(values, group...)
	}
	return mergeUniqueLimit(0, values...)
}

func mergeUniqueLimit(limit int, values ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		identity := retrievaltext.FoldIdentity(value)
		if value == "" || seen[identity] {
			continue
		}
		seen[identity] = true
		out = append(out, value)
		if limit > 0 && len(out) == limit {
			break
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

var shortTechnicalTokens = map[string]bool{
	"c": true, "r": true, "go": true, "db": true, "ui": true, "io": true,
}

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true, "this": true,
	"from": true, "are": true, "como": true, "para": true, "que": true, "con": true,
	"los": true, "las": true, "una": true, "por": true, "del": true,
}
