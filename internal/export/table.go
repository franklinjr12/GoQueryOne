// Package export provides pure-Go writers that render a generic result
// table into various text formats (CSV, TSV, JSON, Markdown, plain text).
package export

import "fmt"

// Table is a generic, format-agnostic representation of tabular data.
type Table struct {
	Columns []string   // column names, may contain duplicates or be empty strings
	Rows    [][]string // cell text; NULL cells hold ""
	Nulls   [][]bool   // same shape as Rows (may be nil or shorter rows = not null); true = SQL NULL
}

// IsNull reports whether the cell at row r, column c is a SQL NULL.
// It is safe to call with indexes beyond the bounds of Nulls; such cells
// are treated as not null.
func (t Table) IsNull(r, c int) bool {
	if r < 0 || r >= len(t.Nulls) {
		return false
	}
	row := t.Nulls[r]
	if c < 0 || c >= len(row) {
		return false
	}
	return row[c]
}

// SelectRows returns a new Table containing only the rows at the given
// indexes, in the given order. Columns are shared with t.
func SelectRows(t Table, rows []int) Table {
	out := Table{
		Columns: t.Columns,
		Rows:    make([][]string, len(rows)),
		Nulls:   make([][]bool, len(rows)),
	}
	for i, r := range rows {
		if r < 0 || r >= len(t.Rows) {
			panic(fmt.Sprintf("export: row index %d out of range [0,%d)", r, len(t.Rows)))
		}
		out.Rows[i] = t.Rows[r]
		if r < len(t.Nulls) {
			out.Nulls[i] = t.Nulls[r]
		}
	}
	return out
}

// UniqueColumnNames returns a copy of cols where empty names are replaced
// with "column_N" (1-based position) and duplicate names (after that
// substitution) are disambiguated by appending "_2", "_3", etc.
func UniqueColumnNames(cols []string) []string {
	out := make([]string, len(cols))
	seen := make(map[string]int, len(cols))
	for i, c := range cols {
		name := c
		if name == "" {
			name = fmt.Sprintf("column_%d", i+1)
		}
		seen[name]++
		if n := seen[name]; n > 1 {
			for {
				candidate := fmt.Sprintf("%s_%d", name, n)
				if seen[candidate] == 0 {
					seen[candidate]++
					name = candidate
					break
				}
				n++
			}
		}
		out[i] = name
	}
	return out
}
