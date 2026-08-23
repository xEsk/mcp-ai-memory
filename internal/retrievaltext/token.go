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
	seen := map[string]bool{}
	var tokens []Token
	var current []rune
	flush := func() {
		if len(current) == 0 {
			return
		}
		original := string(current)
		normalized := strings.ToLower(original)
		current = current[:0]
		if seen[normalized] {
			return
		}
		seen[normalized] = true
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
