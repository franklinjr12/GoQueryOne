package export

import (
	"bytes"
	"encoding/json"
	"testing"
)

func renderJSON(tbl Table, opts JSONOptions) string {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, tbl, opts); err != nil {
		panic(err)
	}
	return buf.String()
}

func TestJSONNulls(t *testing.T) {
	got := renderJSON(tNulls, JSONOptions{})
	checkGolden(t, "json_nulls", got)
}

func TestJSONDupCols(t *testing.T) {
	got := renderJSON(tDupCols, JSONOptions{})
	checkGolden(t, "json_dupcols", got)
}

func TestJSONIndent(t *testing.T) {
	got := renderJSON(tUTF8, JSONOptions{Indent: true})
	checkGolden(t, "json_indent", got)
}

func TestJSONEmptyTable(t *testing.T) {
	got := renderJSON(tEmpty, JSONOptions{})
	checkGolden(t, "json_empty", got)
	if got != "[]" {
		t.Errorf("expected empty array, got %q", got)
	}
}

func TestJSONValidAndColumnOrderPreserved(t *testing.T) {
	got := renderJSON(tDupCols, JSONOptions{})
	// Validate it's syntactically valid JSON.
	var rows []map[string]any
	if err := json.Unmarshal([]byte(got), &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	// Column order must appear in the raw text in the same order as
	// UniqueColumnNames(tDupCols.Columns), since map order is undefined.
	keys := UniqueColumnNames(tDupCols.Columns)
	lastIdx := -1
	for _, k := range keys {
		idx := bytes.Index([]byte(got), []byte(`"`+k+`":`))
		if idx == -1 {
			t.Fatalf("key %q not found in %s", k, got)
		}
		if idx <= lastIdx {
			t.Fatalf("key %q out of order in %s", k, got)
		}
		lastIdx = idx
	}
}
