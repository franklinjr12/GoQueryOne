package odbc

import "github.com/franklinjr12/GoQueryOne/internal/core"

// SplitSQLScript splits a script into statements (quote/comment aware).
// Kept for compatibility; the implementation lives in internal/core.
func SplitSQLScript(script string) []string {
	return core.SplitSQLScript(script)
}

// CountPositionalParams counts '?' placeholders outside quotes/comments.
func CountPositionalParams(sqlText string) int {
	return core.CountPositionalParams(sqlText)
}
