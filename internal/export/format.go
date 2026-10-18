package export

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Format identifies one of the supported export formats.
type Format int

const (
	CSV Format = iota
	TSV
	JSON
	Markdown
	Text
)

// ByExtension maps a file path's extension to a Format. It returns
// false if the extension is not recognized.
func ByExtension(path string) (Format, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return CSV, true
	case ".tsv":
		return TSV, true
	case ".json":
		return JSON, true
	case ".md":
		return Markdown, true
	case ".txt":
		return Text, true
	default:
		return 0, false
	}
}

// Write renders t to w using f with default options for that format
// (CSV and TSV are written with a header row).
func Write(w io.Writer, t Table, f Format) error {
	switch f {
	case CSV:
		return WriteCSV(w, t, CSVOptions{Header: true})
	case TSV:
		return WriteTSV(w, t, TSVOptions{Header: true})
	case JSON:
		return WriteJSON(w, t, JSONOptions{})
	case Markdown:
		return WriteMarkdown(w, t)
	case Text:
		return WriteText(w, t, TextOptions{})
	default:
		return fmt.Errorf("export: unknown format %v", f)
	}
}
