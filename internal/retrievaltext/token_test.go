package retrievaltext

import (
	"reflect"
	"testing"
)

func TestTokenizeUnicodeTechnicalText(t *testing.T) {
	tokens := Tokenize("München Mu\u0308nchen io.Reader context.Context memory_fts x-request-id C DB x509")
	var got []string
	for _, token := range tokens {
		got = append(got, token.Text)
	}
	want := []string{"münchen", "mu\u0308nchen", "io", "reader", "context", "memory", "fts", "x", "request", "id", "c", "db", "x509"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens = %v, want %v", got, want)
	}
}

func TestTokenizeDeduplicatesCaseInsensitivelyAndHandlesMalformedUTF8(t *testing.T) {
	tokens := Tokenize("Go GO go " + string([]byte{0xff}) + " DB")
	if len(tokens) != 2 || tokens[0].Text != "go" || tokens[1].Text != "db" {
		t.Fatalf("tokens = %#v, want go and db", tokens)
	}
}
