package core

import "testing"

func TestAppendStatement(t *testing.T) {
	cases := []struct {
		name, text, stmt, want string
	}{
		{"empty buffer", "", "SELECT 1;", "SELECT 1;"},
		{"whitespace only", " \r\n\t", "SELECT 1;", "SELECT 1;"},
		{"keeps text, adds paragraph", "SELECT a FROM t;", "SELECT 1;", "SELECT a FROM t;\r\n\r\nSELECT 1;"},
		{"adds missing semicolon", "SELECT a FROM t", "SELECT 1;", "SELECT a FROM t;\r\n\r\nSELECT 1;"},
		{"trims trailing blank lines", "SELECT a;\r\n\r\n\r\n", " SELECT 1; ", "SELECT a;\r\n\r\nSELECT 1;"},
		{"no semicolon after line comment", "SELECT a;\r\n-- note", "SELECT 1;", "SELECT a;\r\n-- note\r\n\r\nSELECT 1;"},
		{"dashes inside quotes are not a comment", "SELECT '--x'", "SELECT 1;", "SELECT '--x';\r\n\r\nSELECT 1;"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, start, end := AppendStatement(tc.text, tc.stmt)
			if got != tc.want {
				t.Fatalf("text = %q, want %q", got, tc.want)
			}
			if got[start:end] != "SELECT 1;" {
				t.Fatalf("range [%d:%d] = %q", start, end, got[start:end])
			}
			stmt, s, e, ok := StatementAt(got, end)
			if !ok || got[s:e] == "" || stmt == "" {
				t.Fatalf("StatementAt at end: ok=%v stmt=%q", ok, stmt)
			}
		})
	}
}
