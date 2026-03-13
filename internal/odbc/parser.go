package odbc

import "strings"

func SplitSQLScript(script string) []string {
	statements := make([]string, 0)
	var b strings.Builder

	inSingle := false
	inDouble := false
	inLineComment := false
	inBlockComment := false

	flush := func() {
		stmt := strings.TrimSpace(b.String())
		if stmt != "" {
			statements = append(statements, stmt)
		}
		b.Reset()
	}

	for i := 0; i < len(script); i++ {
		ch := script[i]
		next := byte(0)
		if i+1 < len(script) {
			next = script[i+1]
		}

		if inLineComment {
			b.WriteByte(ch)
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			b.WriteByte(ch)
			if ch == '*' && next == '/' {
				b.WriteByte(next)
				i++
				inBlockComment = false
			}
			continue
		}
		if inSingle {
			b.WriteByte(ch)
			if ch == '\'' {
				if next == '\'' {
					b.WriteByte(next)
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}
		if inDouble {
			b.WriteByte(ch)
			if ch == '"' {
				if next == '"' {
					b.WriteByte(next)
					i++
				} else {
					inDouble = false
				}
			}
			continue
		}

		if ch == '-' && next == '-' {
			b.WriteByte(ch)
			b.WriteByte(next)
			i++
			inLineComment = true
			continue
		}
		if ch == '/' && next == '*' {
			b.WriteByte(ch)
			b.WriteByte(next)
			i++
			inBlockComment = true
			continue
		}
		if ch == '\'' {
			inSingle = true
			b.WriteByte(ch)
			continue
		}
		if ch == '"' {
			inDouble = true
			b.WriteByte(ch)
			continue
		}
		if ch == ';' {
			flush()
			continue
		}
		b.WriteByte(ch)
	}

	flush()
	return statements
}

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
