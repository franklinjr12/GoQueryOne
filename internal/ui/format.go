package ui

import (
	"fmt"
	"strings"

	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

// FormatResultAsCSVLike converts a QueryResult into a CSV-like string with header and footer.
// If maxRows > 0 and result has more than maxRows, the output is truncated and a note is added.
func FormatResultAsCSVLike(result *odbc.QueryResult, maxRows int) string {
	if result == nil {
		return ""
	}

	var builder strings.Builder

	// Header line
	for i, col := range result.Columns {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.WriteString(col.Name)
	}
	builder.WriteString("\n")

	// Rows
	rowLimit := result.RowCount
	truncated := false
	if maxRows > 0 && rowLimit > maxRows {
		rowLimit = maxRows
		truncated = true
	}

	for r := 0; r < rowLimit; r++ {
		row := result.Rows[r]
		for c, val := range row {
			if c > 0 {
				builder.WriteString(",")
			}
			if val == nil {
				builder.WriteString("NULL")
			} else {
				builder.WriteString(fmt.Sprintf("%v", val))
			}
		}
		builder.WriteString("\n")
	}

	// Footer
	if truncated {
		builder.WriteString(fmt.Sprintf("... (truncated to %d rows)\n", rowLimit))
	}
	builder.WriteString(fmt.Sprintf("Execution time: %v\n", result.ExecutionTime))
	builder.WriteString(fmt.Sprintf("Rows: %d\n", result.RowCount))

	return builder.String()
}
