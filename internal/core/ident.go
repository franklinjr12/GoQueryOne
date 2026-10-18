package core

import "strings"

// QuoteIdent quotes an identifier with the driver's quote character
// (SQLGetInfo SQL_IDENTIFIER_QUOTE_CHAR) only when it needs quoting, so the
// generated SQL stays readable: names that are not plain
// [A-Za-z_][A-Za-z0-9_$#@]* (spaces, dashes, accents, leading digits) are
// quoted and embedded quote characters doubled. "[" as quote means SQL
// Server/Access style brackets. Without a quote character the name is
// returned unchanged.
func QuoteIdent(name, quote string) string {
	quote = strings.TrimSpace(quote)
	if name == "" || quote == "" || plainIdent(name) {
		return name
	}
	open, close := quote, quote
	if quote == "[" || quote == "]" {
		open, close = "[", "]"
	}
	return open + strings.ReplaceAll(name, close, close+close) + close
}

func plainIdent(name string) bool {
	for i, r := range name {
		switch {
		case r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
		case i > 0 && ((r >= '0' && r <= '9') || r == '$' || r == '#' || r == '@'):
		default:
			return false
		}
	}
	return true
}

// QualifiedName joins schema and table (each quoted when needed).
func QualifiedName(schema, table, quote string) string {
	if strings.TrimSpace(schema) == "" {
		return QuoteIdent(table, quote)
	}
	return QuoteIdent(schema, quote) + "." + QuoteIdent(table, quote)
}
