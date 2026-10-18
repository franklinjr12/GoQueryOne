package core

import "strings"

// AppendStatement adds stmt to the end of an editor buffer as a new
// paragraph instead of replacing the user's text. It returns the new text and
// the byte range of stmt inside it, so the caller can select it (F5 then runs
// exactly that statement). Trailing whitespace is trimmed, a ';' is added
// after the previous last statement when it has none (so the two do not merge
// into one), and line breaks use CRLF like the Windows edit control.
func AppendStatement(text, stmt string) (newText string, start, end int) {
	stmt = strings.TrimSpace(stmt)
	trimmed := strings.TrimRight(text, " \t\r\n")
	if trimmed == "" {
		return stmt, 0, len(stmt)
	}
	var b strings.Builder
	b.WriteString(trimmed)
	if !strings.HasSuffix(trimmed, ";") && !endsWithLineComment(trimmed) {
		b.WriteString(";")
	}
	b.WriteString("\r\n\r\n")
	start = b.Len()
	b.WriteString(stmt)
	end = b.Len()
	return b.String(), start, end
}

// endsWithLineComment reports whether the last line of text is a "--"
// comment outside of quotes (appending ';' there would be swallowed).
func endsWithLineComment(text string) bool {
	last := text
	if i := strings.LastIndexAny(text, "\r\n"); i >= 0 {
		last = text[i+1:]
	}
	inSingle, inDouble := false, false
	for i := 0; i < len(last); i++ {
		switch c := last[i]; {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '-' && !inSingle && !inDouble && i+1 < len(last) && last[i+1] == '-':
			return true
		}
	}
	return false
}
