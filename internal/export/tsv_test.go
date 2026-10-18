package export

import (
	"bytes"
	"testing"
)

func TestTSVNulls(t *testing.T) {
	got := TSVString(tNulls, true)
	checkGolden(t, "tsv_nulls", got)
}

func TestTSVEscaping(t *testing.T) {
	got := TSVString(tEscaping, true)
	checkGolden(t, "tsv_escaping", got)
}

func TestWriteTSVMatchesString(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTSV(&buf, tNulls, TSVOptions{Header: true}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != TSVString(tNulls, true) {
		t.Error("WriteTSV output differs from TSVString")
	}
}

func TestTSVNoHeader(t *testing.T) {
	got := TSVString(tNulls, false)
	checkGolden(t, "tsv_no_header", got)
}
