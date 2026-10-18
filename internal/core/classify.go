// Package core holds pure domain logic for GoQueryOne: statement
// classification, script splitting, caret/statement extraction, connection
// string building and human-friendly formatting. Everything here builds and
// tests on any OS with CGO_ENABLED=0 - no database driver, no GUI toolkit.
package core

import "strings"

// StatementKind says whether a statement should be run through the query
// path (expects a row cursor) or the exec path (expects a rows-affected
// count).
type StatementKind int

const (
	KindExec StatementKind = iota
	KindQuery
)

// queryKeywords are the leading keywords that indicate a statement returns a
// row cursor and must be run through the query path.
var queryKeywords = map[string]bool{
	"SELECT":   true,
	"WITH":     true,
	"SHOW":     true,
	"EXPLAIN":  true,
	"DESCRIBE": true,
	"DESC":     true,
	"PRAGMA":   true,
	"VALUES":   true,
	"EXEC":     true,
	"EXECUTE":  true,
	"CALL":     true,
	"TABLE":    true,
}

// ClassifyStatement inspects the leading keyword of sqlText - after skipping
// whitespace, "--" and "/* */" comments, and any number of optional leading
// "(" - to decide whether it belongs on the query path or the exec path.
// Classification happens once per statement; callers must not retry a
// statement on the other path based on an execution error.
func ClassifyStatement(sqlText string) StatementKind {
	if queryKeywords[leadingKeyword(sqlText)] {
		return KindQuery
	}
	return KindExec
}

func leadingKeyword(sqlText string) string {
	s := sqlText
	for {
		s = skipWhitespaceAndComments(s)
		if strings.HasPrefix(s, "(") {
			s = s[1:]
			continue
		}
		break
	}
	end := 0
	for end < len(s) {
		c := s[end]
		isLetter := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
		if !isLetter {
			break
		}
		end++
	}
	return strings.ToUpper(s[:end])
}

// skipWhitespaceAndComments strips leading ASCII whitespace and SQL
// comments ("--" line comments, "/* */" block comments) from s, repeating
// until none remain at the front.
func skipWhitespaceAndComments(s string) string {
	for {
		trimmed := strings.TrimLeft(s, " \t\r\n\v\f")
		if trimmed != s {
			s = trimmed
			continue
		}
		if strings.HasPrefix(s, "--") {
			idx := strings.IndexByte(s, '\n')
			if idx < 0 {
				return ""
			}
			s = s[idx+1:]
			continue
		}
		if strings.HasPrefix(s, "/*") {
			idx := strings.Index(s, "*/")
			if idx < 0 {
				return ""
			}
			s = s[idx+2:]
			continue
		}
		return s
	}
}
