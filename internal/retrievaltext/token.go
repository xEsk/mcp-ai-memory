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
	Identity  string
	Original  string
	RuneCount int
}

// Tokenize returns stable, case-insensitively deduplicated letter and number tokens.
func Tokenize(text string) []Token {
	tokens := TokenizeAll(text)
	seen := map[string]bool{}
	out := make([]Token, 0, len(tokens))
	for _, token := range tokens {
		if seen[token.Identity] {
			continue
		}
		seen[token.Identity] = true
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
		identity := FoldIdentity(normalized)
		current = current[:0]
		tokens = append(tokens, Token{Text: normalized, Identity: identity, Original: original, RuneCount: utf8.RuneCountInString(normalized)})
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

// FoldIdentity returns a lowercase diacritic-folded identity for token comparison.
func FoldIdentity(text string) string {
	var folded strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsMark(r) {
			continue
		}
		if replacement, ok := latinDiacriticFold[r]; ok {
			r = replacement
		}
		folded.WriteRune(r)
	}
	return folded.String()
}

var latinDiacriticFold = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a', 'ā': 'a', 'ă': 'a', 'ą': 'a',
	'ç': 'c', 'ć': 'c', 'ĉ': 'c', 'ċ': 'c', 'č': 'c',
	'ď': 'd', 'đ': 'd',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e', 'ē': 'e', 'ĕ': 'e', 'ė': 'e', 'ę': 'e', 'ě': 'e',
	'ĝ': 'g', 'ğ': 'g', 'ġ': 'g', 'ģ': 'g',
	'ĥ': 'h', 'ħ': 'h',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i', 'ĩ': 'i', 'ī': 'i', 'ĭ': 'i', 'į': 'i', 'ı': 'i',
	'ĵ': 'j', 'ķ': 'k',
	'ĺ': 'l', 'ļ': 'l', 'ľ': 'l', 'ŀ': 'l', 'ł': 'l',
	'ñ': 'n', 'ń': 'n', 'ņ': 'n', 'ň': 'n', 'ŉ': 'n', 'ŋ': 'n',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ō': 'o', 'ŏ': 'o', 'ő': 'o',
	'ŕ': 'r', 'ŗ': 'r', 'ř': 'r',
	'ś': 's', 'ŝ': 's', 'ş': 's', 'š': 's',
	'ţ': 't', 'ť': 't', 'ŧ': 't',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u', 'ũ': 'u', 'ū': 'u', 'ŭ': 'u', 'ů': 'u', 'ű': 'u', 'ų': 'u',
	'ŵ': 'w', 'ý': 'y', 'ÿ': 'y', 'ŷ': 'y',
	'ź': 'z', 'ż': 'z', 'ž': 'z',
}
