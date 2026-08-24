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
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
	'ç': 'c',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'ñ': 'n',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
	'ý': 'y', 'ÿ': 'y',
	'ā': 'a', 'ă': 'a', 'ą': 'a', 'ć': 'c', 'ĉ': 'c', 'ċ': 'c', 'č': 'c', 'ď': 'd',
	'ē': 'e', 'ĕ': 'e', 'ė': 'e', 'ę': 'e', 'ě': 'e', 'ĝ': 'g', 'ğ': 'g', 'ġ': 'g', 'ģ': 'g',
	'ĥ': 'h', 'ĩ': 'i', 'ī': 'i', 'ĭ': 'i', 'į': 'i', 'ĵ': 'j', 'ķ': 'k',
	'ĺ': 'l', 'ļ': 'l', 'ľ': 'l', 'ń': 'n', 'ņ': 'n', 'ň': 'n',
	'ō': 'o', 'ŏ': 'o', 'ő': 'o', 'ŕ': 'r', 'ŗ': 'r', 'ř': 'r',
	'ś': 's', 'ŝ': 's', 'ş': 's', 'š': 's', 'ţ': 't', 'ť': 't',
	'ũ': 'u', 'ū': 'u', 'ŭ': 'u', 'ů': 'u', 'ű': 'u', 'ų': 'u',
	'ŵ': 'w', 'ŷ': 'y', 'ź': 'z', 'ż': 'z', 'ž': 'z', 'ſ': 's',
	'ơ': 'o', 'ư': 'u', 'ǎ': 'a', 'ǐ': 'i', 'ǒ': 'o', 'ǔ': 'u', 'ǧ': 'g', 'ǩ': 'k',
	'ǫ': 'o', 'ǰ': 'j', 'ǵ': 'g', 'ǹ': 'n',
	'ȁ': 'a', 'ȃ': 'a', 'ȅ': 'e', 'ȇ': 'e', 'ȉ': 'i', 'ȋ': 'i', 'ȍ': 'o', 'ȏ': 'o',
	'ȑ': 'r', 'ȓ': 'r', 'ȕ': 'u', 'ȗ': 'u', 'ș': 's', 'ț': 't', 'ȟ': 'h',
	'ȧ': 'a', 'ȩ': 'e', 'ȯ': 'o', 'ȳ': 'y',
	'ḁ': 'a', 'ḃ': 'b', 'ḅ': 'b', 'ḇ': 'b', 'ḋ': 'd', 'ḍ': 'd', 'ḏ': 'd', 'ḑ': 'd', 'ḓ': 'd',
	'ḙ': 'e', 'ḛ': 'e', 'ḟ': 'f', 'ḡ': 'g', 'ḣ': 'h', 'ḥ': 'h', 'ḧ': 'h', 'ḩ': 'h', 'ḫ': 'h',
	'ḭ': 'i', 'ḱ': 'k', 'ḳ': 'k', 'ḵ': 'k', 'ḷ': 'l', 'ḻ': 'l', 'ḽ': 'l',
	'ḿ': 'm', 'ṁ': 'm', 'ṃ': 'm', 'ṅ': 'n', 'ṇ': 'n', 'ṉ': 'n', 'ṋ': 'n',
	'ṕ': 'p', 'ṗ': 'p', 'ṙ': 'r', 'ṛ': 'r', 'ṟ': 'r', 'ṡ': 's', 'ṣ': 's',
	'ṫ': 't', 'ṭ': 't', 'ṯ': 't', 'ṱ': 't', 'ṳ': 'u', 'ṵ': 'u', 'ṷ': 'u',
	'ṽ': 'v', 'ṿ': 'v', 'ẁ': 'w', 'ẃ': 'w', 'ẅ': 'w', 'ẇ': 'w', 'ẉ': 'w',
	'ẋ': 'x', 'ẍ': 'x', 'ẏ': 'y', 'ẑ': 'z', 'ẓ': 'z', 'ẕ': 'z',
	'ẖ': 'h', 'ẗ': 't', 'ẘ': 'w', 'ẙ': 'y', 'ẛ': 's',
	'ạ': 'a', 'ả': 'a', 'ẹ': 'e', 'ẻ': 'e', 'ẽ': 'e', 'ỉ': 'i', 'ị': 'i',
	'ọ': 'o', 'ỏ': 'o', 'ụ': 'u', 'ủ': 'u', 'ỳ': 'y', 'ỵ': 'y', 'ỷ': 'y', 'ỹ': 'y',
}
