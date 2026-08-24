package retrievaltext

import (
	"context"
	"database/sql"
	"path/filepath"
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
		{stored: "București", query: "bucuresti", match: true},
		{stored: "ơ", query: "o", match: true},
		{stored: "ł", query: "l", match: false},
		{stored: "đ", query: "d", match: false},
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
	return db
}
