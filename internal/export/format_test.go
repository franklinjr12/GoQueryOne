package export

import (
	"bytes"
	"testing"
)

func TestByExtension(t *testing.T) {
	cases := map[string]struct {
		f  Format
		ok bool
	}{
		"out.csv":   {CSV, true},
		"out.CSV":   {CSV, true},
		"out.tsv":   {TSV, true},
		"out.json":  {JSON, true},
		"out.md":    {Markdown, true},
		"out.txt":   {Text, true},
		"out.xlsx":  {0, false},
		"out":       {0, false},
		"a/b/c.csv": {CSV, true},
	}
	for path, want := range cases {
		f, ok := ByExtension(path)
		if ok != want.ok {
			t.Errorf("ByExtension(%q) ok = %v, want %v", path, ok, want.ok)
			continue
		}
		if ok && f != want.f {
			t.Errorf("ByExtension(%q) = %v, want %v", path, f, want.f)
		}
	}
}

func TestWriteDispatch(t *testing.T) {
	formats := []Format{CSV, TSV, JSON, Markdown, Text}
	for _, f := range formats {
		var buf bytes.Buffer
		if err := Write(&buf, tNulls, f); err != nil {
			t.Errorf("Write format %v: %v", f, err)
		}
		if buf.Len() == 0 {
			t.Errorf("Write format %v produced no output", f)
		}
	}
}

func TestWriteUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, tNulls, Format(999)); err == nil {
		t.Error("expected error for unknown format")
	}
}
