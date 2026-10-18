package export

import (
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// TextOptions configures WriteText.
type TextOptions struct {
	NullText    string // text used to render NULL cells; defaults to "(null)"
	MaxColWidth int    // maximum column display width in runes; defaults to 50
}

func (o TextOptions) nullText() string {
	if o.NullText == "" {
		return "(null)"
	}
	return o.NullText
}

func (o TextOptions) maxColWidth() int {
	if o.MaxColWidth <= 0 {
		return 50
	}
	return o.MaxColWidth
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max <= 0 {
		return ""
	}
	return string(runes[:max-1]) + "…"
}

func isNumeric(s string) bool {
	_, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return err == nil
}

// WriteText writes t as an aligned plain-text table resembling a SQL
// console's output: a header row, a separator line of dashes, then the
// data rows. Columns whose non-null values all parse as numbers are
// right-aligned; all other columns are left-aligned.
func WriteText(w io.Writer, t Table, opts TextOptions) error {
	_, err := io.WriteString(w, TextString(t, opts))
	return err
}

// TextString returns the aligned plain-text rendering of t.
func TextString(t Table, opts TextOptions) string {
	nullText := opts.nullText()
	maxWidth := opts.maxColWidth()
	numCols := len(t.Columns)

	cell := func(r, c int) (string, bool) {
		if t.IsNull(r, c) {
			return "", true
		}
		row := t.Rows[r]
		if c < len(row) {
			return row[c], false
		}
		return "", false
	}

	// Determine numeric alignment per column, based on untruncated values.
	numeric := make([]bool, numCols)
	for c := 0; c < numCols; c++ {
		sawValue := false
		allNumeric := true
		for r := range t.Rows {
			v, isNull := cell(r, c)
			if isNull {
				continue
			}
			sawValue = true
			if !isNumeric(v) {
				allNumeric = false
				break
			}
		}
		numeric[c] = sawValue && allNumeric
	}

	display := func(r, c int) string {
		v, isNull := cell(r, c)
		if isNull {
			v = nullText
		}
		return truncate(v, maxWidth)
	}

	widths := make([]int, numCols)
	for c := 0; c < numCols; c++ {
		widths[c] = utf8.RuneCountInString(truncate(t.Columns[c], maxWidth))
	}
	for r := range t.Rows {
		for c := 0; c < numCols; c++ {
			if w := utf8.RuneCountInString(display(r, c)); w > widths[c] {
				widths[c] = w
			}
		}
	}

	pad := func(s string, width int, rightAlign bool) string {
		gap := width - utf8.RuneCountInString(s)
		if gap <= 0 {
			return s
		}
		spaces := strings.Repeat(" ", gap)
		if rightAlign {
			return spaces + s
		}
		return s + spaces
	}

	var sb strings.Builder
	writeRow := func(values []string) {
		for c := 0; c < numCols; c++ {
			if c > 0 {
				sb.WriteString("  ")
			}
			v := ""
			if c < len(values) {
				v = values[c]
			}
			sb.WriteString(pad(v, widths[c], numeric[c]))
		}
		sb.WriteByte('\n')
	}

	headers := make([]string, numCols)
	for c := 0; c < numCols; c++ {
		headers[c] = truncate(t.Columns[c], maxWidth)
	}
	writeRow(headers)

	seps := make([]string, numCols)
	for c := 0; c < numCols; c++ {
		seps[c] = strings.Repeat("-", widths[c])
	}
	writeRow(seps)

	for r := range t.Rows {
		values := make([]string, numCols)
		for c := 0; c < numCols; c++ {
			values[c] = display(r, c)
		}
		writeRow(values)
	}

	return sb.String()
}
