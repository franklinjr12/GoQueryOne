package export

import (
	"encoding/json"
	"io"
	"strings"
)

// JSONOptions configures WriteJSON.
type JSONOptions struct {
	Indent bool // pretty-print with two-space indentation
}

// WriteJSON writes t as a JSON array of objects, one per row, preserving
// column order (encoding/json's map-based marshaling would sort keys
// alphabetically, so this encoder is hand-written; it still relies on
// encoding/json to escape individual string values). NULL cells are
// rendered as JSON null. Duplicate or empty column names are disambiguated
// via UniqueColumnNames.
func WriteJSON(w io.Writer, t Table, opts JSONOptions) error {
	keys := make([]string, len(t.Columns))
	for i, c := range UniqueColumnNames(t.Columns) {
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		keys[i] = string(b)
	}

	var sb strings.Builder
	nl, indent1, indent2 := "", "", ""
	if opts.Indent {
		nl = "\n"
		indent1 = "  "
		indent2 = "    "
	}

	sb.WriteByte('[')
	for r, row := range t.Rows {
		if r > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(nl)
		sb.WriteString(indent1)
		sb.WriteByte('{')
		for c := range keys {
			if c > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(nl)
			sb.WriteString(indent2)
			sb.WriteString(keys[c])
			sb.WriteByte(':')
			if opts.Indent {
				sb.WriteByte(' ')
			}
			if c < len(row) && !t.IsNull(r, c) {
				b, err := json.Marshal(row[c])
				if err != nil {
					return err
				}
				sb.Write(b)
			} else {
				sb.WriteString("null")
			}
		}
		if len(keys) > 0 {
			sb.WriteString(nl)
			sb.WriteString(indent1)
		}
		sb.WriteByte('}')
	}
	if len(t.Rows) > 0 {
		sb.WriteString(nl)
	}
	sb.WriteByte(']')

	_, err := io.WriteString(w, sb.String())
	return err
}
