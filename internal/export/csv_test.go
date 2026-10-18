package export

import (
	"bytes"
	"testing"
)

func renderCSV(t Table, opts CSVOptions) string {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, t, opts); err != nil {
		panic(err)
	}
	return buf.String()
}

func TestCSVNulls(t *testing.T) {
	got := renderCSV(tNulls, CSVOptions{Header: true})
	checkGolden(t, "csv_nulls", got)
}

func TestCSVEscaping(t *testing.T) {
	got := renderCSV(tEscaping, CSVOptions{Header: true})
	checkGolden(t, "csv_escaping", got)
}

func TestCSVCustomDelimiterCRLF(t *testing.T) {
	got := renderCSV(tEscaping, CSVOptions{Header: true, Delimiter: ';', CRLF: true})
	checkGolden(t, "csv_semicolon_crlf", got)
}

func TestCSVEmptyTable(t *testing.T) {
	got := renderCSV(tEmpty, CSVOptions{Header: true})
	checkGolden(t, "csv_empty", got)
}

func TestCSVNoHeader(t *testing.T) {
	got := renderCSV(tNulls, CSVOptions{Header: false})
	checkGolden(t, "csv_no_header", got)
}

func TestCSVQuoteFieldLeadingTrailingSpace(t *testing.T) {
	got := csvQuoteField(" x", ',')
	if got != "\" x\"" {
		t.Errorf("got %q", got)
	}
}
