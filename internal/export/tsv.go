package export

import (
	"io"
	"strings"
)

// TSVOptions configures WriteTSV.
type TSVOptions struct {
	Header bool // write a header row with column names
}

var tsvSanitizer = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ")

// WriteTSV writes t as tab-separated values suitable for pasting into
// Excel or similar spreadsheet clipboards: CRLF line endings, and any
// CR/LF/TAB inside a value replaced with a single space. NULL cells are
// rendered as an empty field.
func WriteTSV(w io.Writer, t Table, opts TSVOptions) error {
	_, err := io.WriteString(w, TSVString(t, opts.Header))
	return err
}

// TSVString returns the TSV rendering of t as a string.
func TSVString(t Table, header bool) string {
	var sb strings.Builder

	writeRow := func(fields []string, nulls func(c int) bool) {
		for c, f := range fields {
			if c > 0 {
				sb.WriteByte('\t')
			}
			if nulls != nil && nulls(c) {
				continue
			}
			sb.WriteString(tsvSanitizer.Replace(f))
		}
		sb.WriteString("\r\n")
	}

	if header {
		writeRow(t.Columns, nil)
	}
	for r, row := range t.Rows {
		writeRow(row, func(c int) bool { return t.IsNull(r, c) })
	}
	return sb.String()
}
