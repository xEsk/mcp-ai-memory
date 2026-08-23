// Package retrievaltext provides shared Unicode-aware technical text normalization.
package retrievaltext

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token describes one normalized token aligned with SQLite unicode61 boundaries.
type Token struct {
	Text      string
	Original  string
	RuneCount int
}

// Tokenize returns stable, case-insensitively deduplicated letter and number tokens.
func Tokenize(text string) []Token {
	tokens := TokenizeAll(text)
	seen := map[string]bool{}
	out := make([]Token, 0, len(tokens))
	for _, token := range tokens {
		if seen[token.Text] {
			continue
		}
		seen[token.Text] = true
		out = append(out, token)
	}
	return out
}

// TokenizeAll returns normalized tokens in source order, including repetitions.
func TokenizeAll(text string) []Token {
	var tokens []Token
	var current []rune
	flush := func() {
		if len(current) == 0 {
			return
		}
		original := string(current)
		normalized := strings.ToLower(original)
		current = current[:0]
		tokens = append(tokens, Token{Text: normalized, Original: original, RuneCount: utf8.RuneCountInString(normalized)})
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) {
			current = append(current, r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}
