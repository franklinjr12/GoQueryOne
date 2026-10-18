package export

import (
	"io"
	"strings"
)

// CSVOptions configures WriteCSV.
type CSVOptions struct {
	Header    bool // write a header row with column names
	Delimiter rune // field delimiter; zero value defaults to ','
	CRLF      bool // use CRLF line endings instead of LF
}

func (o CSVOptions) delimiter() rune {
	if o.Delimiter == 0 {
		return ','
	}
	return o.Delimiter
}

func (o CSVOptions) newline() string {
	if o.CRLF {
		return "\r\n"
	}
	return "\n"
}

// WriteCSV writes t to w as RFC 4180 CSV. NULL cells are rendered as an
// empty, unquoted field; empty (non-null) strings are rendered as a quoted
// empty field ("") so the two remain distinguishable.
func WriteCSV(w io.Writer, t Table, opts CSVOptions) error {
	nl := opts.newline()
	delim := opts.delimiter()
	var sb strings.Builder

	writeRow := func(fields []string, nulls func(c int) bool) {
		for c, f := range fields {
			if c > 0 {
				sb.WriteRune(delim)
			}
			if nulls != nil && nulls(c) {
				continue
			}
			sb.WriteString(csvQuoteField(f, delim))
		}
		sb.WriteString(nl)
	}

	if opts.Header {
		writeRow(t.Columns, nil)
	}
	for r, row := range t.Rows {
		writeRow(row, func(c int) bool { return t.IsNull(r, c) })
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

func csvQuoteField(f string, delim rune) string {
	needsQuote := f == "" ||
		strings.ContainsRune(f, delim) ||
		strings.ContainsAny(f, "\"\r\n") ||
		strings.HasPrefix(f, " ") || strings.HasSuffix(f, " ") ||
		strings.HasPrefix(f, "\t") || strings.HasSuffix(f, "\t")
	if !needsQuote {
		return f
	}
	var sb strings.Builder
	sb.WriteByte('"')
	sb.WriteString(strings.ReplaceAll(f, "\"", "\"\""))
	sb.WriteByte('"')
	return sb.String()
}
