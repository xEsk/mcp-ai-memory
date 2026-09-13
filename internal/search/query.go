package search

import (
	"errors"
	"fmt"
	"strings"

	"mcp-ai-memory/internal/retrievaltext"
)

var ErrInvalidQuery = errors.New("invalid search query")

// QueryTerm describes one normalized exact or prefix FTS term.
type QueryTerm struct {
	Text     string
	Identity string
	Exact    bool
}

// CompiledQuery contains the escaped FTS expression and its normalized terms.
type CompiledQuery struct {
	Expression string
	Terms      []QueryTerm
	Mode       string
}

// CompileQuery normalizes and safely compiles one query for SQLite FTS5.
func CompileQuery(query, mode string) (CompiledQuery, error) {
	normalizedMode, err := normalizeMode(mode)
	if err != nil {
		return CompiledQuery{}, err
	}
	var terms []QueryTerm
	for _, token := range retrievaltext.Tokenize(query) {
		if normalizedMode != ModeBroad && token.RuneCount >= 3 && searchStopWords[token.Identity] {
			continue
		}
		terms = append(terms, QueryTerm{Text: token.Text, Identity: token.Identity, Exact: token.RuneCount <= 2})
	}
	if len(terms) == 0 {
		return CompiledQuery{}, fmt.Errorf("%w: no searchable terms", ErrInvalidQuery)
	}
	operator := " OR "
	if normalizedMode == ModePrecise {
		operator = " AND "
	}
	expressions := make([]string, 0, len(terms))
	for _, term := range terms {
		expressions = append(expressions, compileTermExpression(term))
	}
	return CompiledQuery{Expression: strings.Join(expressions, operator), Terms: terms, Mode: normalizedMode}, nil
}

func compileTermExpression(term QueryTerm) string {
	expression := `"` + strings.ReplaceAll(term.Text, `"`, `""`) + `"`
	if !term.Exact {
		expression += "*"
	}
	return expression
}
