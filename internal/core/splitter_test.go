package core

import (
	"reflect"
	"testing"
)

func TestSplitSQLScript(t *testing.T) {
	script := `
SELECT 'a;b' AS v;
-- comment ; still comment
SELECT 1;
/* block ; comment */
INSERT INTO x VALUES ('semi;colon', 2);
`
	got := SplitSQLScript(script)
	want := []string{
		"SELECT 'a;b' AS v",
		"-- comment ; still comment\nSELECT 1",
		"/* block ; comment */\nINSERT INTO x VALUES ('semi;colon', 2)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitSQLScript() = %#v, want %#v", got, want)
	}
}

func TestSplitSQLScriptDoubleQuotedIdentifierWithSemicolon(t *testing.T) {
	script := `SELECT "col;name" FROM t; SELECT 2;`
	got := SplitSQLScript(script)
	want := []string{`SELECT "col;name" FROM t`, "SELECT 2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitSQLScript() = %#v, want %#v", got, want)
	}
}

func TestSplitSQLScriptEscapedQuotes(t *testing.T) {
	script := `SELECT 'it''s; a test' AS v; SELECT "a""b;c" AS w;`
	got := SplitSQLScript(script)
	want := []string{`SELECT 'it''s; a test' AS v`, `SELECT "a""b;c" AS w`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitSQLScript() = %#v, want %#v", got, want)
	}
}

func TestSplitSQLScriptCRLF(t *testing.T) {
	script := "SELECT 1;\r\n-- comment\r\nSELECT 2;\r\n"
	got := SplitSQLScript(script)
	want := []string{"SELECT 1", "-- comment\r\nSELECT 2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitSQLScript() = %#v, want %#v", got, want)
	}
}

func TestSplitSQLScriptEmptyAndBlank(t *testing.T) {
	if got := SplitSQLScript(""); len(got) != 0 {
		t.Fatalf("expected no statements, got %#v", got)
	}
	if got := SplitSQLScript("   ;;  \n ; "); len(got) != 0 {
		t.Fatalf("expected no statements for blank/empty segments, got %#v", got)
	}
}

func TestSplitSQLScriptUnterminatedBlockComment(t *testing.T) {
	script := "SELECT 1; /* never closes ; SELECT 2;"
	got := SplitSQLScript(script)
	want := []string{"SELECT 1", "/* never closes ; SELECT 2;"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitSQLScript() = %#v, want %#v", got, want)
	}
}

func TestSplitSQLScriptRangesMatchSubstrings(t *testing.T) {
	script := "  SELECT 1 ;\nSELECT 2;  "
	ranges := SplitSQLScriptRanges(script)
	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d: %#v", len(ranges), ranges)
	}
	if got, want := script[ranges[0].Start:ranges[0].End], "SELECT 1"; got != want {
		t.Fatalf("range 0 = %q, want %q", got, want)
	}
	if got, want := script[ranges[1].Start:ranges[1].End], "SELECT 2"; got != want {
		t.Fatalf("range 1 = %q, want %q", got, want)
	}
}

func TestCountPositionalParams(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want int
	}{
		{"mixed", `SELECT * FROM t WHERE a = ? AND b = '?' AND c = ? -- ? ignored`, 2},
		{"none", "SELECT 1", 0},
		{"in double quoted identifier", `SELECT "col?name" FROM t WHERE a = ?`, 1},
		{"in block comment", "SELECT 1 /* ? */ WHERE a = ?", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CountPositionalParams(tc.sql); got != tc.want {
				t.Fatalf("CountPositionalParams(%q) = %d, want %d", tc.sql, got, tc.want)
			}
		})
	}
}
