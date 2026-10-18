package core

import "testing"

func TestClassifyStatement(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want StatementKind
	}{
		{"select", "SELECT * FROM t", KindQuery},
		{"lowercase select", "select 1", KindQuery},
		{"with cte", "WITH x AS (SELECT 1) SELECT * FROM x", KindQuery},
		{"show", "SHOW TABLES", KindQuery},
		{"explain", "EXPLAIN SELECT 1", KindQuery},
		{"describe", "DESCRIBE t", KindQuery},
		{"desc", "DESC t", KindQuery},
		{"pragma", "PRAGMA table_info(t)", KindQuery},
		{"values", "VALUES (1, 2, 3)", KindQuery},
		{"exec", "EXEC my_proc", KindQuery},
		{"execute", "EXECUTE my_proc", KindQuery},
		{"call", "CALL my_proc()", KindQuery},
		{"table shorthand", "TABLE foo", KindQuery},
		{"leading whitespace", "   \n\t SELECT 1", KindQuery},
		{"leading line comment", "-- note\nSELECT 1", KindQuery},
		{"leading block comment", "/* note */ SELECT 1", KindQuery},
		{"leading comment and paren", "-- note\n(SELECT 1)", KindQuery},
		{"nested parens", "((SELECT 1))", KindQuery},
		{"multiple comments", "-- a\n/* b */ -- c\nSELECT 1", KindQuery},

		{"insert", "INSERT INTO t VALUES (1)", KindExec},
		{"update", "UPDATE t SET a = 1", KindExec},
		{"delete", "DELETE FROM t", KindExec},
		{"create table", "CREATE TABLE t (a INT)", KindExec},
		{"drop table", "DROP TABLE t", KindExec},
		{"begin", "BEGIN", KindExec},
		{"empty", "", KindExec},
		{"only comment", "-- nothing here", KindExec},
		{"only whitespace", "   \n\t  ", KindExec},
		{"unterminated block comment", "/* unterminated", KindExec},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyStatement(tc.sql)
			if got != tc.want {
				t.Fatalf("ClassifyStatement(%q) = %v, want %v", tc.sql, got, tc.want)
			}
		})
	}
}
