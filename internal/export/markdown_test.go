package export

import (
	"bytes"
	"testing"
)

func renderMarkdown(t Table) string {
	var buf bytes.Buffer
	if err := WriteMarkdown(&buf, t); err != nil {
		panic(err)
	}
	return buf.String()
}

func TestMarkdownEscaping(t *testing.T) {
	got := renderMarkdown(tEscaping)
	checkGolden(t, "markdown_escaping", got)
}

func TestMarkdownNulls(t *testing.T) {
	got := renderMarkdown(tNulls)
	checkGolden(t, "markdown_nulls", got)
}

func TestMarkdownEmptyTable(t *testing.T) {
	got := renderMarkdown(tEmpty)
	checkGolden(t, "markdown_empty", got)
}

func TestMarkdownUTF8(t *testing.T) {
	got := renderMarkdown(tUTF8)
	checkGolden(t, "markdown_utf8", got)
}
