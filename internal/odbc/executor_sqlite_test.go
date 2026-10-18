package odbc

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/franklinjr12/GoQueryOne/internal/config"
	sqlite "modernc.org/sqlite"
)

// This file exercises the execution engine against a real, pure-Go SQL
// engine (modernc.org/sqlite, test-only dependency) through Manager's
// injected Opener, with driver name "sqlite". It is the regression test for
// the historical bug where ExecuteStatement ran QueryContext and, on any
// error, retried the same statement with ExecContext - double-executing
// DML. callCounts/countingConn count calls at the database/sql/driver
// boundary directly, so the assertions hold regardless of how any given
// statement's success/failure affects table contents.

type callCounts struct {
	mu      sync.Mutex
	queries int
	execs   int
}

func (c *callCounts) incQuery() {
	c.mu.Lock()
	c.queries++
	c.mu.Unlock()
}

func (c *callCounts) incExec() {
	c.mu.Lock()
	c.execs++
	c.mu.Unlock()
}

func (c *callCounts) snapshot() (queries, execs int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.queries, c.execs
}

type countingConn struct {
	driver.Conn
	counts *callCounts
}

func (c *countingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.counts.incQuery()
	qc, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return qc.QueryContext(ctx, query, args)
}

func (c *countingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.counts.incExec()
	ec, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return ec.ExecContext(ctx, query, args)
}

type countingConnector struct {
	dsn    string
	counts *callCounts
}

func (c *countingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := (&sqlite.Driver{}).Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: conn, counts: c.counts}, nil
}

func (c *countingConnector) Driver() driver.Driver {
	return &sqlite.Driver{}
}

// newCountingSQLiteManager wires a Manager through NewManagerWithOpener to a
// file-backed sqlite database, counting QueryContext/ExecContext calls made
// at the driver boundary - the exact boundary at which the historical
// double-execution bug (Query then, on error, Exec) manifested.
func newCountingSQLiteManager(t *testing.T) (*Manager, *Session, *callCounts) {
	t.Helper()
	counts := &callCounts{}
	dbPath := filepath.Join(t.TempDir(), "test.db")

	mgr := NewManagerWithOpener(func(connStr string) (*sql.DB, error) {
		return sql.OpenDB(&countingConnector{dsn: connStr, counts: counts}), nil
	})

	session, err := mgr.Connect(config.ConnectionProfile{
		ID:               "t",
		Name:             "t",
		Type:             "connection_string",
		ConnectionString: dbPath,
	}, "", 5*time.Second)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(mgr.DisconnectAll)
	return mgr, session, counts
}

// newSQLiteManager wires a Manager to a plain file-backed sqlite database
// via the registered "sqlite" driver, for tests that don't need call
// counting.
func newSQLiteManager(t *testing.T) (*Manager, *Session) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	mgr := NewManagerWithOpener(func(connStr string) (*sql.DB, error) {
		return sql.Open("sqlite", connStr)
	})

	session, err := mgr.Connect(config.ConnectionProfile{
		ID:               "t",
		Name:             "t",
		Type:             "connection_string",
		ConnectionString: dbPath,
	}, "", 5*time.Second)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(mgr.DisconnectAll)
	return mgr, session
}

func mustExec(t *testing.T, m *Manager, sessionID, sqlText string) StatementResult {
	t.Helper()
	res, err := m.ExecuteStatement(sessionID, sqlText, nil, QueryOptions{})
	if err != nil {
		t.Fatalf("ExecuteStatement(%q): %v", sqlText, err)
	}
	return res
}

func scalarInt(t *testing.T, m *Manager, sessionID, query string) int64 {
	t.Helper()
	res, err := m.ExecuteStatement(sessionID, query, nil, QueryOptions{})
	if err != nil {
		t.Fatalf("ExecuteStatement(%q): %v", query, err)
	}
	if !res.HasRows || len(res.ResultSet.Rows) != 1 || len(res.ResultSet.Rows[0]) != 1 {
		t.Fatalf("expected a single scalar row/col, got %#v", res.ResultSet)
	}
	n, err := strconv.ParseInt(res.ResultSet.Rows[0][0], 10, 64)
	if err != nil {
		t.Fatalf("parse scalar %q: %v", res.ResultSet.Rows[0][0], err)
	}
	return n
}

func TestExecuteStatement_InsertExecutesExactlyOnce(t *testing.T) {
	mgr, session, counts := newCountingSQLiteManager(t)

	mustExec(t, mgr, session.ID, "CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)")
	qBefore, eBefore := counts.snapshot()

	res := mustExec(t, mgr, session.ID, "INSERT INTO t (name) VALUES ('alice')")
	if res.HasRows {
		t.Fatalf("expected an exec-path result, got HasRows=true: %#v", res)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("expected RowsAffected=1, got %d", res.RowsAffected)
	}

	qAfter, eAfter := counts.snapshot()
	if qAfter != qBefore {
		t.Fatalf("INSERT must never go through the query path: queries went from %d to %d", qBefore, qAfter)
	}
	if eAfter != eBefore+1 {
		t.Fatalf("INSERT must execute exactly once: execs went from %d to %d, want +1", eBefore, eAfter)
	}

	if n := scalarInt(t, mgr, session.ID, "SELECT COUNT(*) FROM t"); n != 1 {
		t.Fatalf("expected exactly 1 row inserted (this is the double-insert regression check), got %d", n)
	}
}

func TestExecuteStatement_FailingInsertNotRetried(t *testing.T) {
	mgr, session, counts := newCountingSQLiteManager(t)

	mustExec(t, mgr, session.ID, "CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT UNIQUE)")
	mustExec(t, mgr, session.ID, "INSERT INTO t (name) VALUES ('alice')")

	qBefore, eBefore := counts.snapshot()
	_, err := mgr.ExecuteStatement(session.ID, "INSERT INTO t (name) VALUES ('alice')", nil, QueryOptions{})
	if err == nil {
		t.Fatal("expected a unique constraint violation error")
	}
	qAfter, eAfter := counts.snapshot()

	if qAfter != qBefore {
		t.Fatalf("a failing INSERT must never fall back to the query path: queries went from %d to %d", qBefore, qAfter)
	}
	if eAfter != eBefore+1 {
		t.Fatalf("a failing INSERT must be attempted exactly once: execs went from %d to %d, want +1", eBefore, eAfter)
	}

	if n := scalarInt(t, mgr, session.ID, "SELECT COUNT(*) FROM t"); n != 1 {
		t.Fatalf("failed INSERT must not leave extra rows behind, got count=%d", n)
	}
}

func TestExecuteStatement_FailingSyntaxErrorNotRetried(t *testing.T) {
	mgr, session, counts := newCountingSQLiteManager(t)
	mustExec(t, mgr, session.ID, "CREATE TABLE t (id INTEGER PRIMARY KEY)")

	qBefore, eBefore := counts.snapshot()
	_, err := mgr.ExecuteStatement(session.ID, "INSERT INTO t (id) VALUES (", nil, QueryOptions{})
	if err == nil {
		t.Fatal("expected a syntax error")
	}
	qAfter, eAfter := counts.snapshot()
	if qAfter != qBefore {
		t.Fatalf("a syntactically invalid INSERT must never fall back to the query path: queries went from %d to %d", qBefore, qAfter)
	}
	if eAfter != eBefore+1 {
		t.Fatalf("a syntactically invalid INSERT must be attempted exactly once: execs went from %d to %d, want +1", eBefore, eAfter)
	}
}

func TestExecuteStatement_QueryWithZeroColumnsReportsCommandExecuted(t *testing.T) {
	mgr, session := newSQLiteManager(t)

	res := mustExec(t, mgr, session.ID, "PRAGMA foreign_keys=ON")
	if res.HasRows {
		t.Fatalf("expected HasRows=false for a zero-column query-path statement, got %#v", res.ResultSet)
	}
	if res.Message != "Command executed" {
		t.Fatalf("Message = %q, want %q", res.Message, "Command executed")
	}
}

func TestExecuteStatement_NullDistinctFromStringLiteral(t *testing.T) {
	mgr, session := newSQLiteManager(t)

	mustExec(t, mgr, session.ID, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")
	mustExec(t, mgr, session.ID, "INSERT INTO t (id, v) VALUES (1, NULL)")
	mustExec(t, mgr, session.ID, "INSERT INTO t (id, v) VALUES (2, 'NULL')")

	res := mustExec(t, mgr, session.ID, "SELECT v FROM t ORDER BY id")
	if !res.HasRows || len(res.ResultSet.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %#v", res.ResultSet)
	}

	if !res.ResultSet.IsNull(0, 0) {
		t.Fatal("row 0 should be NULL")
	}
	if got := res.ResultSet.Rows[0][0]; got != "" {
		t.Fatalf("NULL cell should render as empty string, got %q", got)
	}

	if res.ResultSet.IsNull(1, 0) {
		t.Fatal("row 1 holds the string 'NULL' and must not be flagged as SQL NULL")
	}
	if got := res.ResultSet.Rows[1][0]; got != "NULL" {
		t.Fatalf("row 1 should be the literal string NULL, got %q", got)
	}
}

func TestExecuteStatement_Truncation(t *testing.T) {
	mgr, session := newSQLiteManager(t)

	mustExec(t, mgr, session.ID, "CREATE TABLE t (id INTEGER PRIMARY KEY)")
	for i := 0; i < 5; i++ {
		mustExec(t, mgr, session.ID, "INSERT INTO t DEFAULT VALUES")
	}

	res, err := mgr.ExecuteStatement(session.ID, "SELECT id FROM t ORDER BY id", nil, QueryOptions{MaxRows: 2})
	if err != nil {
		t.Fatalf("ExecuteStatement: %v", err)
	}
	if !res.ResultSet.Truncated || res.ResultSet.TruncatedAt != 2 {
		t.Fatalf("expected truncation at 2, got %#v", res.ResultSet)
	}
	if res.ResultSet.RowCount != 2 {
		t.Fatalf("expected 2 rows returned, got %d", res.ResultSet.RowCount)
	}
}

func TestExecuteStatement_TimeoutOnLongRunningQuery(t *testing.T) {
	mgr, session := newSQLiteManager(t)

	res, err := mgr.ExecuteStatement(session.ID,
		"WITH RECURSIVE cnt(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM cnt) SELECT x FROM cnt",
		nil,
		QueryOptions{Timeout: 20 * time.Millisecond, MaxRows: 50_000_000},
	)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if !res.TimedOut {
		t.Fatalf("expected TimedOut=true, got %#v", res)
	}
}

func TestExecuteStatement_Cancel(t *testing.T) {
	mgr, session := newSQLiteManager(t)

	var (
		wg  sync.WaitGroup
		res StatementResult
		err error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		res, err = mgr.ExecuteStatement(session.ID,
			"WITH RECURSIVE cnt(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM cnt) SELECT x FROM cnt",
			nil,
			QueryOptions{Timeout: 5 * time.Second, MaxRows: 50_000_000},
		)
	}()

	time.Sleep(15 * time.Millisecond)
	if cancelErr := mgr.CancelExecution(session.ID); cancelErr != nil {
		t.Fatalf("CancelExecution: %v", cancelErr)
	}
	wg.Wait()

	if err == nil {
		t.Fatal("expected a cancellation error")
	}
	if !res.Canceled {
		t.Fatalf("expected Canceled=true, got %#v", res)
	}
}

func TestExecuteStatement_TransactionCommitAndRollback(t *testing.T) {
	mgr, session := newSQLiteManager(t)
	mustExec(t, mgr, session.ID, "CREATE TABLE t (id INTEGER PRIMARY KEY)")

	if err := mgr.BeginTx(session.ID); err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	mustExec(t, mgr, session.ID, "INSERT INTO t DEFAULT VALUES")
	if err := mgr.CommitTx(session.ID); err != nil {
		t.Fatalf("CommitTx: %v", err)
	}
	if n := scalarInt(t, mgr, session.ID, "SELECT COUNT(*) FROM t"); n != 1 {
		t.Fatalf("expected committed row to persist, got count=%d", n)
	}

	if err := mgr.BeginTx(session.ID); err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	mustExec(t, mgr, session.ID, "INSERT INTO t DEFAULT VALUES")
	if err := mgr.RollbackTx(session.ID); err != nil {
		t.Fatalf("RollbackTx: %v", err)
	}
	if n := scalarInt(t, mgr, session.ID, "SELECT COUNT(*) FROM t"); n != 1 {
		t.Fatalf("expected rolled-back row to be discarded, got count=%d", n)
	}
}

func TestExecuteScript_StopOnErrorVsContinue(t *testing.T) {
	script := "INSERT INTO t DEFAULT VALUES; INSERT INTO bogus_table_does_not_exist DEFAULT VALUES; INSERT INTO t DEFAULT VALUES;"

	t.Run("stop on error", func(t *testing.T) {
		mgr, session := newSQLiteManager(t)
		mustExec(t, mgr, session.ID, "CREATE TABLE t (id INTEGER PRIMARY KEY)")

		_, err := mgr.ExecuteScript(session.ID, script, ScriptOptions{StopOnErr: true})
		if err == nil {
			t.Fatal("expected an error from the failing statement")
		}
		if n := scalarInt(t, mgr, session.ID, "SELECT COUNT(*) FROM t"); n != 1 {
			t.Fatalf("expected only the first statement to run before stopping, got count=%d", n)
		}
	})

	t.Run("continue on error", func(t *testing.T) {
		mgr, session := newSQLiteManager(t)
		mustExec(t, mgr, session.ID, "CREATE TABLE t (id INTEGER PRIMARY KEY)")

		result, err := mgr.ExecuteScript(session.ID, script, ScriptOptions{StopOnErr: false})
		if err != nil {
			t.Fatalf("ExecuteScript should not return an error when StopOnErr is false: %v", err)
		}
		if len(result.Results) != 3 {
			t.Fatalf("expected all 3 statements to be attempted, got %d", len(result.Results))
		}
		if n := scalarInt(t, mgr, session.ID, "SELECT COUNT(*) FROM t"); n != 2 {
			t.Fatalf("expected statements 1 and 3 to both run, got count=%d", n)
		}
	})
}

func TestExecuteStatement_UTF8RoundTrip(t *testing.T) {
	mgr, session := newSQLiteManager(t)
	mustExec(t, mgr, session.ID, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")

	values := []string{"São Paulo", "ação", "😀"}
	for i, v := range values {
		_, err := mgr.ExecuteStatement(session.ID, "INSERT INTO t (id, v) VALUES (?, ?)", []any{i, v}, QueryOptions{})
		if err != nil {
			t.Fatalf("insert %q: %v", v, err)
		}
	}

	res := mustExec(t, mgr, session.ID, "SELECT v FROM t ORDER BY id")
	if len(res.ResultSet.Rows) != len(values) {
		t.Fatalf("expected %d rows, got %d", len(values), len(res.ResultSet.Rows))
	}
	for i, v := range values {
		if got := res.ResultSet.Rows[i][0]; got != v {
			t.Fatalf("row %d = %q, want %q", i, got, v)
		}
	}
}

func TestExecuteStatement_FailureKeepsSessionConnected(t *testing.T) {
	mgr, session := newSQLiteManager(t)
	_, err := mgr.ExecuteStatement(session.ID, "SELECT * FROM no_such_table", nil, QueryOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	s, _ := mgr.GetSession(session.ID)
	if s.State != SessionConnected {
		t.Fatalf("State = %s, want CONNECTED (state is connectivity only)", s.State)
	}
	if s.LastOutcome != "error" || s.LastError == "" {
		t.Fatalf("LastOutcome=%q LastError=%q", s.LastOutcome, s.LastError)
	}
	res := mustExec(t, mgr, session.ID, "SELECT 1")
	if !res.HasRows || res.Kind != "query" {
		t.Fatalf("unexpected result %#v", res)
	}
	if s.LastOutcome != "ok" {
		t.Fatalf("LastOutcome after success = %q", s.LastOutcome)
	}
	if !IsQueryStatement("  -- c\nselect 1") || IsQueryStatement("INSERT INTO t VALUES (1)") {
		t.Fatal("IsQueryStatement classification wrong")
	}
}
