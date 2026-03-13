package ui

import (
	"fmt"
	"strings"

	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

func FormatResultAsCSVLike(result *odbc.StatementResult) string {
	if result == nil || !result.HasRows {
		return ""
	}

	var builder strings.Builder

	for i, col := range result.ResultSet.Columns {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.WriteString(col.Name)
	}
	builder.WriteString("\n")

	for _, row := range result.ResultSet.Rows {
		for c, val := range row {
			if c > 0 {
				builder.WriteString(",")
			}
			builder.WriteString(fmt.Sprintf("%v", val))
		}
		builder.WriteString("\n")
	}

	if result.ResultSet.Truncated {
		builder.WriteString(fmt.Sprintf("... (truncated to %d rows)\n", result.ResultSet.TruncatedAt))
	}
	builder.WriteString(fmt.Sprintf("Execution time: %v\n", result.ExecutionTime))
	builder.WriteString(fmt.Sprintf("Rows: %d\n", result.ResultSet.RowCount))

	return builder.String()
}
