package core

// StatementRange is a byte-offset range [Start, End) into the original
// script identifying one trimmed statement.
type StatementRange struct {
	Start int
	End   int
}

// SplitSQLScriptRanges scans script and returns the byte ranges of each
// non-empty statement, splitting on ';' while respecting '...' and "..."
// string literals (with doubled-quote escaping) and "--"/"/* */" comments.
// Comments remain part of the statement text; only the delimiter itself and
// surrounding whitespace are excluded from the returned ranges.
func SplitSQLScriptRanges(script string) []StatementRange {
	ranges := make([]StatementRange, 0)

	inSingle := false
	inDouble := false
	inLineComment := false
	inBlockComment := false

	segStart := 0

	emit := func(end int) {
		start := segStart
		for start < end && isSQLSpace(script[start]) {
			start++
		}
		for end > start && isSQLSpace(script[end-1]) {
			end--
		}
		if end > start {
			ranges = append(ranges, StatementRange{Start: start, End: end})
		}
	}

	for i := 0; i < len(script); i++ {
		ch := script[i]
		next := byte(0)
		if i+1 < len(script) {
			next = script[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				i++
				inBlockComment = false
			}
			continue
		}
		if inSingle {
			if ch == '\'' {
				if next == '\'' {
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}
		if inDouble {
			if ch == '"' {
				if next == '"' {
					i++
				} else {
					inDouble = false
				}
			}
			continue
		}

		if ch == '-' && next == '-' {
			i++
			inLineComment = true
			continue
		}
		if ch == '/' && next == '*' {
			i++
			inBlockComment = true
			continue
		}
		if ch == '\'' {
			inSingle = true
			continue
		}
		if ch == '"' {
			inDouble = true
			continue
		}
		if ch == ';' {
			emit(i)
			segStart = i + 1
			continue
		}
	}
	emit(len(script))
	return ranges
}

// SplitSQLScript returns the trimmed text of each statement produced by
// SplitSQLScriptRanges.
func SplitSQLScript(script string) []string {
	ranges := SplitSQLScriptRanges(script)
	out := make([]string, len(ranges))
	for i, r := range ranges {
		out[i] = script[r.Start:r.End]
	}
	return out
}

// CountPositionalParams counts '?' placeholders in sqlText outside of
// string literals and comments.
func CountPositionalParams(sqlText string) int {
	count := 0
	inSingle := false
	inDouble := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		next := byte(0)
		if i+1 < len(sqlText) {
			next = sqlText[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				i++
				inBlockComment = false
			}
			continue
		}
		if inSingle {
			if ch == '\'' {
				if next == '\'' {
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}
		if inDouble {
			if ch == '"' {
				if next == '"' {
					i++
				} else {
					inDouble = false
				}
			}
			continue
		}

		if ch == '-' && next == '-' {
			i++
			inLineComment = true
			continue
		}
		if ch == '/' && next == '*' {
			i++
			inBlockComment = true
			continue
		}
		if ch == '\'' {
			inSingle = true
			continue
		}
		if ch == '"' {
			inDouble = true
			continue
		}
		if ch == '?' {
			count++
		}
	}

	return count
}

func isSQLSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\v', '\f':
		return true
	default:
		return false
	}
}
