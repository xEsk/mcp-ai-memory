package retrievaltext

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"unicode"

	_ "modernc.org/sqlite"
)

func TestFoldIdentityMatchesUnicode61LatinRanges(t *testing.T) {
	db := openUnicode61ParityDB(t)
	ctx := context.Background()
	for _, bounds := range [][2]rune{{0x00a0, 0x024f}, {0x1e00, 0x1eff}} {
		for r := bounds[0]; r <= bounds[1]; r++ {
			if unicode.ToLower(r) != r || !unicode.IsLetter(r) {
				continue
			}
			if _, err := db.ExecContext(ctx, `DELETE FROM probe`); err != nil {
				t.Fatalf("clear probe: %v", err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO probe(value) VALUES (?)`, string(r)); err != nil {
				t.Fatalf("insert %U: %v", r, err)
			}
			for base := 'a'; base <= 'z'; base++ {
				var count int
				if err := db.QueryRowContext(ctx, `SELECT count(*) FROM probe WHERE probe MATCH ?`, string(base)).Scan(&count); err != nil {
					t.Fatalf("match %U against %q: %v", r, base, err)
				}
				if got, want := FoldIdentity(string(r)) == string(base), count == 1; got != want {
					t.Fatalf("FoldIdentity(%q) = %q parity with %q is %t, unicode61 match is %t", r, FoldIdentity(string(r)), base, got, want)
				}
			}
		}
	}
}

func TestFoldIdentityUnicode61ReviewedMatrix(t *testing.T) {
	db := openUnicode61ParityDB(t)
	ctx := context.Background()
	for _, test := range []struct {
		stored string
		query  string
		match  bool
	}{
		{stored: "München", query: "Munchen", match: true},
		{stored: "Mu\u0308nchen", query: "Munchen", match: true},
		{stored: "ά", query: "α", match: false},
		{stored: "α\u0301", query: "α", match: true},
		{stored: "București", query: "bucuresti", match: true},
		{stored: "ơ", query: "o", match: true},
		{stored: "ł", query: "l", match: false},
		{stored: "đ", query: "d", match: false},
		{stored: "שָׁלוֹם", query: "שלום", match: false},
		{stored: "नमस्ते", query: "नमसत", match: false},
	} {
		if _, err := db.ExecContext(ctx, `DELETE FROM probe`); err != nil {
			t.Fatalf("clear probe: %v", err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO probe(value) VALUES (?)`, test.stored); err != nil {
			t.Fatalf("insert %q: %v", test.stored, err)
		}
		var count int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM probe WHERE probe MATCH ?`, test.query).Scan(&count); err != nil {
			t.Fatalf("match %q against %q: %v", test.stored, test.query, err)
		}
		if got := count == 1; got != test.match {
			t.Fatalf("unicode61 match %q against %q = %t, want %t", test.stored, test.query, got, test.match)
		}
		if got := FoldIdentity(test.stored) == FoldIdentity(test.query); got != test.match {
			t.Fatalf("identity match %q against %q = %t, want %t", test.stored, test.query, got, test.match)
		}
	}
}

func TestTokenizeMatchesUnicode61CombiningMarkBoundaries(t *testing.T) {
	db := openUnicode61ParityDB(t)
	ctx := context.Background()
	for _, letters := range [][2]rune{{'a', 'b'}, {'α', 'β'}, {'а', 'б'}, {'ש', 'ל'}, {'न', 'म'}} {
		for _, bounds := range [][2]rune{{0x0300, 0x036f}, {0x0591, 0x05c7}, {0x0900, 0x097f}} {
			for mark := bounds[0]; mark <= bounds[1]; mark++ {
				if !unicode.IsMark(mark) {
					continue
				}
				stored := string([]rune{letters[0], mark, letters[1]})
				if _, err := db.ExecContext(ctx, `DELETE FROM probe`); err != nil {
					t.Fatalf("clear probe: %v", err)
				}
				if _, err := db.ExecContext(ctx, `INSERT INTO probe(value) VALUES (?)`, stored); err != nil {
					t.Fatalf("insert %U+%U+%U: %v", letters[0], mark, letters[1], err)
				}
				rows, err := db.QueryContext(ctx, `SELECT term FROM probe_vocab ORDER BY term`)
				if err != nil {
					t.Fatalf("read vocab %U+%U+%U: %v", letters[0], mark, letters[1], err)
				}
				var want []string
				for rows.Next() {
					var term string
					if err := rows.Scan(&term); err != nil {
						rows.Close()
						t.Fatalf("scan vocab: %v", err)
					}
					want = append(want, term)
				}
				if err := rows.Close(); err != nil {
					t.Fatalf("close vocab rows: %v", err)
				}
				var got []string
				for _, token := range TokenizeAll(stored) {
					got = append(got, token.Identity)
				}
				sort.Strings(got)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("tokens for %U+%U+%U = %q, unicode61 vocab = %q", letters[0], mark, letters[1], got, want)
				}
			}
		}
	}
}

func openUnicode61ParityDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "unicode61.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})
	if _, err := db.Exec(`CREATE VIRTUAL TABLE probe USING fts5(value, tokenize='unicode61')`); err != nil {
		t.Fatalf("create probe: %v", err)
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE probe_vocab USING fts5vocab(probe, 'row')`); err != nil {
		t.Fatalf("create probe vocab: %v", err)
	}
	return db
}
