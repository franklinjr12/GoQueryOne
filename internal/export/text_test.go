package export

import (
	"bytes"
	"strings"
	"testing"
)

func renderText(t Table, opts TextOptions) string {
	var buf bytes.Buffer
	if err := WriteText(&buf, t, opts); err != nil {
		panic(err)
	}
	return buf.String()
}

func TestTextNulls(t *testing.T) {
	got := renderText(tNulls, TextOptions{})
	checkGolden(t, "text_nulls", got)
}

func TestTextCustomNullText(t *testing.T) {
	got := renderText(tNulls, TextOptions{NullText: "<NULL>"})
	checkGolden(t, "text_custom_nulltext", got)
}

func TestTextUTF8Alignment(t *testing.T) {
	got := renderText(tUTF8, TextOptions{})
	checkGolden(t, "text_utf8", got)
	// every line must have the same rune count since columns are padded.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	width := -1
	for _, line := range lines {
		n := len([]rune(line))
		if width == -1 {
			width = n
		} else if n != width {
			t.Errorf("line %q has width %d, expected %d", line, n, width)
		}
	}
}

func TestTextNumericRightAligned(t *testing.T) {
	got := renderText(tNumeric, TextOptions{})
	checkGolden(t, "text_numeric", got)
}

func TestTextEmptyTable(t *testing.T) {
	got := renderText(tEmpty, TextOptions{})
	checkGolden(t, "text_empty", got)
}

func TestTextTruncation(t *testing.T) {
	long := strings.Repeat("x", 60)
	tbl := Table{
		Columns: []string{"col"},
		Rows:    [][]string{{long}},
	}
	got := TextString(tbl, TextOptions{MaxColWidth: 10})
	checkGolden(t, "text_truncation", got)
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis in truncated output: %q", got)
	}
}

func TestTextStringMatchesWriteText(t *testing.T) {
	if TextString(tNulls, TextOptions{}) != renderText(tNulls, TextOptions{}) {
		t.Error("TextString and WriteText disagree")
	}
}

func TestIsNumeric(t *testing.T) {
	cases := map[string]bool{
		"10.5": true,
		"-3":   true,
		"abc":  false,
		"":     false,
		" 5 ":  true,
	}
	for in, want := range cases {
		if got := isNumeric(in); got != want {
			t.Errorf("isNumeric(%q) = %v, want %v", in, got, want)
		}
	}
}
