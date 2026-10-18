package export

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func goldenPath(name string) string {
	return filepath.Join("testdata", name+".golden")
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v (run with -update to generate)", path, err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, string(want))
	}
}

// Fixtures shared across test files.

// tNulls exercises the three-way distinction between a true SQL NULL, an
// empty (non-null) string, and the literal text "NULL".
var tNulls = Table{
	Columns: []string{"id", "val"},
	Rows: [][]string{
		{"1", "NULL"}, // literal text "NULL", not a real null
		{"2", ""},     // empty, non-null string
		{"3", "ignored-because-null"},
	},
	Nulls: [][]bool{
		{false, false},
		{false, false},
		{false, true},
	},
}

// tDupCols exercises duplicate and empty column names.
var tDupCols = Table{
	Columns: []string{"name", "", "name", ""},
	Rows: [][]string{
		{"a", "b", "c", "d"},
	},
}

// tEscaping exercises characters that need escaping in various formats:
// delimiter, quote, newline, tab, pipe, leading/trailing space.
var tEscaping = Table{
	Columns: []string{"a", "b"},
	Rows: [][]string{
		{"has,comma", "quote\"here"},
		{"new\nline", "tab\there"},
		{"pipe|here", "  lead/trail  "},
	},
}

// tUTF8 exercises multi-byte, multi-rune display width.
var tUTF8 = Table{
	Columns: []string{"city", "note"},
	Rows: [][]string{
		{"São Paulo", "ação"},
		{"日本", "emoji😀test"},
	},
}

// tNumeric exercises right-alignment of numeric-looking columns.
var tNumeric = Table{
	Columns: []string{"id", "amount", "label"},
	Rows: [][]string{
		{"1", "10.5", "abc"},
		{"2", "-3", "de"},
		{"3", "7", "f"},
	},
}

// tEmpty has columns but no rows.
var tEmpty = Table{
	Columns: []string{"a", "b"},
	Rows:    [][]string{},
}

// tNilNulls has a nil Nulls slice.
var tNilNulls = Table{
	Columns: []string{"a"},
	Rows: [][]string{
		{"x"},
		{"y"},
	},
}
