package export

import (
	"io"
	"strings"
)

var (
	mdPipeEscaper = strings.NewReplacer("|", "\\|")
	mdNewlines    = strings.NewReplacer("\r\n", "<br>", "\r", "<br>", "\n", "<br>")
)

func mdEscapeCell(f string) string {
	return mdPipeEscaper.Replace(mdNewlines.Replace(f))
}

// WriteMarkdown writes t as a GitHub-flavored Markdown table. Pipes are
// escaped as "\|" and newlines become "<br>". NULL cells are rendered as
// an empty cell.
func WriteMarkdown(w io.Writer, t Table) error {
	var sb strings.Builder

	sb.WriteByte('|')
	for _, c := range t.Columns {
		sb.WriteByte(' ')
		sb.WriteString(mdEscapeCell(c))
		sb.WriteString(" |")
	}
	sb.WriteByte('\n')

	sb.WriteByte('|')
	for range t.Columns {
		sb.WriteString(" --- |")
	}
	sb.WriteByte('\n')

	for r, row := range t.Rows {
		sb.WriteByte('|')
		for c := range t.Columns {
			sb.WriteByte(' ')
			if c < len(row) && !t.IsNull(r, c) {
				sb.WriteString(mdEscapeCell(row[c]))
			}
			sb.WriteString(" |")
		}
		sb.WriteByte('\n')
	}

	_, err := io.WriteString(w, sb.String())
	return err
}
