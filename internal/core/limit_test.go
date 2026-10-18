package core

import "testing"

func TestDetectLimitSyntax(t *testing.T) {
	cases := []struct {
		hints []string
		want  string
	}{
		{[]string{"SQLite3 ODBC Driver"}, LimitLimit},
		{[]string{"", "Driver={MySQL ODBC 8.0 Unicode Driver};Server=x"}, LimitLimit},
		{[]string{"PostgreSQL Unicode(x64)"}, LimitLimit},
		{[]string{"Oracle in OraClient19Home1"}, LimitFetch},
		{[]string{"IBM DB2 ODBC DRIVER"}, LimitFetch},
		{[]string{"Firebird/InterBase(r) driver"}, LimitFirst},
		{[]string{"ODBC Driver 18 for SQL Server"}, LimitTop},
		{[]string{"Microsoft Access Driver (*.mdb, *.accdb)"}, LimitTop},
		{[]string{"SQLite3"}, LimitLimit},
		{[]string{"Some Unknown Driver", "MyDSN"}, LimitTop},
		{nil, LimitTop},
		{[]string{"", "Production"}, LimitTop},
		// first matching hint wins
		{[]string{"SQL Server", "sqlite"}, LimitTop},
	}
	for _, tc := range cases {
		if got := DetectLimitSyntax(tc.hints...); got != tc.want {
			t.Errorf("DetectLimitSyntax(%q) = %q, want %q", tc.hints, got, tc.want)
		}
	}
}

func TestNormalizeLimitSyntax(t *testing.T) {
	cases := map[string]string{
		"": "", "auto": "", "AUTO": "", "bogus": "",
		"top": LimitTop, " Limit ": LimitLimit, "fetch": LimitFetch, "FETCH FIRST": LimitFetch,
		"offset fetch": LimitFetch, "first": LimitFirst,
	}
	for in, want := range cases {
		if got := NormalizeLimitSyntax(in); got != want {
			t.Errorf("NormalizeLimitSyntax(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSelectTop(t *testing.T) {
	cases := map[string]string{
		LimitTop:   "SELECT TOP 100 * FROM t;",
		"":         "SELECT TOP 100 * FROM t;",
		LimitLimit: "SELECT * FROM t LIMIT 100;",
		LimitFetch: "SELECT * FROM t FETCH FIRST 100 ROWS ONLY;",
		LimitFirst: "SELECT FIRST 100 * FROM t;",
	}
	for syntax, want := range cases {
		if got := SelectTop(syntax, "t", 100); got != want {
			t.Errorf("SelectTop(%q) = %q, want %q", syntax, got, want)
		}
	}
}
