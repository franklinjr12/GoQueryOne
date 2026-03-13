package odbc

import "testing"

func TestSplitSQLScript(t *testing.T) {
	script := `
SELECT 'a;b' AS v;
-- comment ; still comment
SELECT 1;
/* block ; comment */
INSERT INTO x VALUES ('semi;colon', 2);
`
	got := SplitSQLScript(script)
	if len(got) != 3 {
		t.Fatalf("expected 3 statements, got %d: %#v", len(got), got)
	}
	if got[0] != "SELECT 'a;b' AS v" {
		t.Fatalf("unexpected first statement: %q", got[0])
	}
	if got[1] != "-- comment ; still comment\nSELECT 1" {
		t.Fatalf("unexpected second statement: %q", got[1])
	}
	if got[2] != "/* block ; comment */\nINSERT INTO x VALUES ('semi;colon', 2)" {
		t.Fatalf("unexpected third statement: %q", got[2])
	}
}

func TestCountPositionalParams(t *testing.T) {
	query := `SELECT * FROM t WHERE a = ? AND b = '?' AND c = ? -- ? ignored`
	got := CountPositionalParams(query)
	if got != 2 {
		t.Fatalf("expected 2 params, got %d", got)
	}
}
