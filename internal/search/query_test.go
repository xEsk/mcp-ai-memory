package search

import (
	"errors"
	"reflect"
	"testing"
)

func TestCompileQueryUsesExactShortTermsAndLongPrefixes(t *testing.T) {
	compiled, err := CompileQuery("C R Go DB io ErrNotFound", ModePrecise)
	if err != nil {
		t.Fatalf("compile query: %v", err)
	}
	wantExpression := `"c" AND "r" AND "go" AND "db" AND "io" AND "errnotfound"*`
	if compiled.Expression != wantExpression {
		t.Fatalf("expression = %q, want %q", compiled.Expression, wantExpression)
	}
}

func TestCompileQueryPreciseAndBalancedShareTerms(t *testing.T) {
	precise, err := CompileQuery("How Go context.Context works", ModePrecise)
	if err != nil {
		t.Fatalf("compile precise: %v", err)
	}
	balanced, err := CompileQuery("How Go context.Context works", ModeBalanced)
	if err != nil {
		t.Fatalf("compile balanced: %v", err)
	}
	if !reflect.DeepEqual(precise.Terms, balanced.Terms) {
		t.Fatalf("precise terms = %#v, balanced terms = %#v", precise.Terms, balanced.Terms)
	}
	if precise.Expression == balanced.Expression {
		t.Fatalf("precise and balanced expressions should differ by operator: %q", precise.Expression)
	}
}

func TestCompileQueryRejectsEmptyNormalizedInput(t *testing.T) {
	for _, query := range []string{"", "the and how", `*** "" ()`} {
		if _, err := CompileQuery(query, ModePrecise); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("CompileQuery(%q) error = %v, want ErrInvalidQuery", query, err)
		}
	}
}

func TestCompileQueryEscapesFTSOperatorsAsData(t *testing.T) {
	compiled, err := CompileQuery(`AND OR NOT * "unterminated (x) internal/search/fts.go`, ModeBalanced)
	if err != nil {
		t.Fatalf("compile query: %v", err)
	}
	want := `"or" OR "not"* OR "unterminated"* OR "x" OR "internal"* OR "search"* OR "fts"* OR "go"`
	if compiled.Expression != want {
		t.Fatalf("expression = %q, want %q", compiled.Expression, want)
	}
}

func TestCompileQueryTechnicalCompatibilityMatrix(t *testing.T) {
	for _, query := range []string{
		"C R x Go DB UI io v2 S3 x509",
		"API MCP SQL JWT OR IN com.example",
		"io.Reader context.Context ErrNotFound memory_fts x-request-id",
		`internal/search/fts.go go.mod .env C:\\workspace\\memory.db`,
		`C# C++ "quoted words"`,
	} {
		compiled, err := CompileQuery(query, ModePrecise)
		if err != nil {
			t.Fatalf("CompileQuery(%q): %v", query, err)
		}
		if compiled.Expression == "" || len(compiled.Terms) == 0 {
			t.Fatalf("CompileQuery(%q) = %#v, want searchable terms", query, compiled)
		}
	}
}
