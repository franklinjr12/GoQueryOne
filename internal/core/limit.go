package core

import (
	"strconv"
	"strings"
)

// Row-limit syntaxes for the "SELECT TOP" quick query.
const (
	LimitTop   = "TOP"   // SELECT TOP n * FROM t            (SQL Server, Access, Sybase)
	LimitLimit = "LIMIT" // SELECT * FROM t LIMIT n          (SQLite, MySQL, PostgreSQL, …)
	LimitFetch = "FETCH" // SELECT * FROM t FETCH FIRST n ROWS ONLY (Oracle 12c+, DB2, Derby)
	LimitFirst = "FIRST" // SELECT FIRST n * FROM t          (Firebird, Informix)
)

var limitHints = []struct {
	needle string
	syntax string
}{
	{"sqlite", LimitLimit},
	{"mysql", LimitLimit},
	{"maria", LimitLimit},
	{"postgres", LimitLimit},
	{"psqlodbc", LimitLimit},
	{"redshift", LimitLimit},
	{"duckdb", LimitLimit},
	{"clickhouse", LimitLimit},
	{"snowflake", LimitLimit},
	{"vertica", LimitLimit},
	{"hive", LimitLimit},
	{"spark", LimitLimit},
	{"databricks", LimitLimit},
	{"presto", LimitLimit},
	{"trino", LimitLimit},
	{"impala", LimitLimit},
	{"cockroach", LimitLimit},
	{"oracle", LimitFetch},
	{"db2", LimitFetch},
	{"ibm i access", LimitFetch},
	{"iseries", LimitFetch},
	{"derby", LimitFetch},
	{"firebird", LimitFirst},
	{"interbase", LimitFirst},
	{"informix", LimitFirst},
	{"sql server", LimitTop},
	{"sqlsrv", LimitTop},
	{"sql native client", LimitTop},
	{"microsoft access", LimitTop},
	{".mdb", LimitTop},
	{"sybase", LimitTop},
	{"adaptive server", LimitTop},
}

// NormalizeLimitSyntax maps a configured value to one of the Limit*
// constants, or "" for automatic detection (empty, "AUTO" or unknown).
func NormalizeLimitSyntax(v string) string {
	switch s := strings.ToUpper(strings.TrimSpace(v)); s {
	case LimitTop, LimitLimit, LimitFetch, LimitFirst:
		return s
	case "FETCH FIRST", "OFFSET FETCH":
		return LimitFetch
	default:
		return ""
	}
}

// DetectLimitSyntax guesses the row-limit syntax from hints such as the
// ODBC driver name, the connection string or the DSN name. The first hint
// that matches wins; with no match it returns LimitTop (the historical
// default of ODBC Query Tool users, mostly SQL Server and Access).
func DetectLimitSyntax(hints ...string) string {
	for _, h := range hints {
		l := strings.ToLower(h)
		if l == "" {
			continue
		}
		for _, k := range limitHints {
			if strings.Contains(l, k.needle) {
				return k.syntax
			}
		}
	}
	return LimitTop
}

// SelectTop returns a "first n rows" query for an already quoted table name.
func SelectTop(syntax, table string, n int) string {
	count := strconv.Itoa(n)
	switch NormalizeLimitSyntax(syntax) {
	case LimitLimit:
		return "SELECT * FROM " + table + " LIMIT " + count + ";"
	case LimitFetch:
		return "SELECT * FROM " + table + " FETCH FIRST " + count + " ROWS ONLY;"
	case LimitFirst:
		return "SELECT FIRST " + count + " * FROM " + table + ";"
	default:
		return "SELECT TOP " + count + " * FROM " + table + ";"
	}
}
