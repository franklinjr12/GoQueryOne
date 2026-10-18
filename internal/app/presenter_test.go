package app

import (
	"strings"
	"testing"
	"time"
)

func TestComputeActionsDisconnected(t *testing.T) {
	a := ComputeActions(State{TabCount: 1, EditorHasSQL: true})
	if a.Run || a.RunScript || a.Disconnect || a.RefreshSchema || a.Cancel || a.Begin || a.Commit || a.Rollback {
		t.Fatalf("db actions must be disabled when disconnected: %+v", a)
	}
	if !a.Connect || !a.NewTab || !a.Open || !a.Save || !a.CloseTab || !a.ODBCAdmin || !a.Help || !a.About {
		t.Fatalf("always-available actions disabled: %+v", a)
	}
	if a.CopyRows || a.Export || a.CopyAll || a.ToggleText {
		t.Fatalf("result actions enabled without results: %+v", a)
	}
}

func TestComputeActionsConnected(t *testing.T) {
	s := State{Connected: true, TabCount: 1, EditorHasSQL: true}
	a := ComputeActions(s)
	if !a.Run || !a.RunScript || !a.Disconnect || !a.RefreshSchema || !a.Begin {
		t.Fatalf("expected run actions enabled: %+v", a)
	}
	if a.Commit || a.Rollback || a.Cancel || a.LoadMore {
		t.Fatalf("commit/rollback/cancel/loadmore must be disabled: %+v", a)
	}
	s.EditorHasSQL = false
	if a := ComputeActions(s); a.Run || a.RunScript {
		t.Fatal("run must be disabled with an empty editor")
	}
}

func TestComputeActionsTxAndExecuting(t *testing.T) {
	s := State{Connected: true, TabCount: 1, EditorHasSQL: true, TxOpen: true, HasResultRows: true, CanLoadMore: true}
	a := ComputeActions(s)
	if a.Begin || !a.Commit || !a.Rollback || !a.LoadMore || !a.Export || !a.CopyRows {
		t.Fatalf("tx state wrong: %+v", a)
	}
	s.Executing = true
	a = ComputeActions(s)
	if !a.Cancel || a.Run || a.RunScript || a.Commit || a.Rollback || a.Disconnect || a.Export || a.Connect || a.LoadMore {
		t.Fatalf("executing state wrong: %+v", a)
	}
	if !a.CopyRows {
		t.Fatal("copy should stay available while executing")
	}
}

func TestStatusSections(t *testing.T) {
	now := time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC)
	got := StatusSections(State{}, now)
	if got[SectionConnection] != "Not connected" || got[SectionActivity] != "Ready" || got[SectionTransaction] != "" || got[SectionCaret] != "" {
		t.Fatalf("disconnected sections: %q", got)
	}
	s := State{Connected: true, ConnectionName: "Demo", DBMS: "SQLite 3.46", SessionCount: 2, TxOpen: true,
		Executing: true, ExecutingSince: now.Add(-1500 * time.Millisecond), Line: 3, Col: 7,
		Last: &Outcome{HasRows: true, Rows: 10000, Duration: 12 * time.Millisecond, Truncated: true, TruncatedAt: 10000}}
	got = StatusSections(s, now)
	want := []string{"Demo · SQLite 3.46 (+1)", "TX open", "Executing 1.5 s", "10,000 rows · 12 ms (truncated at 10,000)", "Ln 3, Col 7"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("section %d = %q, want %q", i, got[i], want[i])
		}
	}
	s = State{Connected: true, ConnectionName: "Demo", SessionCount: 1}
	got = StatusSections(s, now)
	if got[SectionConnection] != "Demo" || got[SectionTransaction] != "Autocommit" {
		t.Fatalf("got %q", got)
	}
	s.Executing = true
	if got := StatusSections(s, now); got[SectionActivity] != "Executing" {
		t.Fatalf("got %q", got[SectionActivity])
	}
}

func TestOutcomeSummary(t *testing.T) {
	ms := 4 * time.Millisecond
	cases := []struct {
		o    *Outcome
		want string
	}{
		{nil, ""},
		{&Outcome{HasRows: true, Rows: 1, Duration: ms}, "1 row · 4.0 ms"},
		{&Outcome{RowsAffected: 3, Duration: ms}, "3 rows affected · 4.0 ms"},
		{&Outcome{RowsAffected: 1, Duration: ms}, "1 row affected · 4.0 ms"},
		{&Outcome{RowsAffected: -1, Duration: ms}, "Command executed · 4.0 ms"},
		{&Outcome{Err: "boom", Duration: ms}, "Error after 4.0 ms"},
		{&Outcome{Canceled: true, Err: "x", Duration: ms}, "Canceled after 4.0 ms"},
		{&Outcome{TimedOut: true, Err: "x", Duration: ms}, "Timed out after 4.0 ms"},
	}
	for _, c := range cases {
		if got := OutcomeSummary(c.o); got != c.want {
			t.Errorf("OutcomeSummary(%+v) = %q, want %q", c.o, got, c.want)
		}
	}
}

func TestWindowTitle(t *testing.T) {
	if got := WindowTitle(State{}); got != "GoQueryOne" {
		t.Fatalf("got %q", got)
	}
	if got := WindowTitle(State{TabTitle: "SQL 1"}); got != "GoQueryOne — SQL 1" {
		t.Fatalf("got %q", got)
	}
	got := WindowTitle(State{Connected: true, ConnectionName: "Prod", TabTitle: "report.sql", TabDirty: true})
	if got != "GoQueryOne — Prod — report.sql *" {
		t.Fatalf("got %q", got)
	}
}

func TestResultTabFor(t *testing.T) {
	if ResultTabFor(Outcome{HasRows: true}) != TabResults {
		t.Fatal("rows -> results")
	}
	if ResultTabFor(Outcome{RowsAffected: 2}) != TabMessages {
		t.Fatal("dml -> messages")
	}
	if ResultTabFor(Outcome{Err: "x"}) != TabErrors || ResultTabFor(Outcome{Canceled: true}) != TabErrors || ResultTabFor(Outcome{TimedOut: true}) != TabErrors {
		t.Fatal("errors -> errors")
	}
}

func TestMessageFor(t *testing.T) {
	at := time.Date(2026, 10, 18, 1, 2, 3, 0, time.Local)
	d := 1100 * time.Microsecond
	cases := []struct {
		o    Outcome
		want string
	}{
		{Outcome{At: at, HasRows: true, Rows: 2, Duration: d}, "01:02:03  2 rows retrieved in 1.1 ms."},
		{Outcome{At: at, HasRows: true, Rows: 10, Duration: d, Truncated: true, TruncatedAt: 10}, "01:02:03  10 rows retrieved in 1.1 ms. Truncated at 10 rows (use Load More)."},
		{Outcome{At: at, RowsAffected: 1, Duration: d}, "01:02:03  Command executed in 1.1 ms. 1 row affected."},
		{Outcome{At: at, RowsAffected: -1, Duration: d}, "01:02:03  Command executed in 1.1 ms."},
		{Outcome{At: at, Err: "no such table: x\nmore", Duration: d}, "01:02:03  Error after 1.1 ms: no such table: x"},
		{Outcome{At: at, Canceled: true, Duration: d}, "01:02:03  Canceled by user after 1.1 ms."},
		{Outcome{At: at, TimedOut: true, Duration: d}, "01:02:03  Timed out after 1.1 ms."},
	}
	for _, c := range cases {
		if got := MessageFor(c.o); got != c.want {
			t.Errorf("MessageFor = %q, want %q", got, c.want)
		}
		if c.o.Err != "" && strings.Contains(MessageFor(c.o), "executed") {
			t.Error("failure must not say executed")
		}
	}
	if !strings.Contains(MessageFor(Outcome{RowsAffected: 0}), "0 rows affected") {
		t.Error("zero time default")
	}
}

func TestFormatCount(t *testing.T) {
	for n, want := range map[int64]string{0: "0", 999: "999", 1000: "1,000", 1234567: "1,234,567", -12345: "-12,345"} {
		if got := FormatCount(n); got != want {
			t.Errorf("FormatCount(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestLineCol(t *testing.T) {
	text := "SELECT 1;\r\nSELECT 'ção';\nX"
	cases := []struct{ off, line, col int }{
		{0, 1, 1}, {6, 1, 7}, {9, 1, 10}, {11, 2, 1}, {len("SELECT 1;\r\nSELECT 'ç"), 2, 10}, {len(text), 3, 2}, {-5, 1, 1}, {999, 3, 2},
	}
	for _, c := range cases {
		l, col := LineCol(text, c.off)
		if l != c.line || col != c.col {
			t.Errorf("LineCol(%d) = %d,%d want %d,%d", c.off, l, col, c.line, c.col)
		}
	}
}

type fakeView struct {
	actions Actions
	status  []string
	title   string
	calls   int
}

func (f *fakeView) SetActionsEnabled(a Actions) { f.actions = a; f.calls++ }
func (f *fakeView) SetStatus(s []string)        { f.status = s }
func (f *fakeView) SetTitle(t string)           { f.title = t }

func TestPresenterUpdate(t *testing.T) {
	v := &fakeView{}
	p := NewPresenter(v)
	p.now = func() time.Time { return time.Unix(0, 0) }
	s := State{Connected: true, ConnectionName: "Demo", SessionCount: 1, TabCount: 1, TabTitle: "SQL 1", EditorHasSQL: true, Line: 1, Col: 1}
	p.Update(s)
	if v.calls != 1 || !v.actions.Run || v.title != "GoQueryOne — Demo — SQL 1" || v.status[SectionConnection] != "Demo" || v.status[SectionCaret] != "Ln 1, Col 1" {
		t.Fatalf("view not updated: %+v", v)
	}
	if p.State().ConnectionName != "Demo" {
		t.Fatal("State() should return last state")
	}
	NewPresenter(nil).Update(s) // must not panic
}
