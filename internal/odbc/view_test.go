package odbc

import "testing"

func TestSessionView(t *testing.T) {
	m, s := newSQLiteManager(t)
	mustExec(t, m, s.ID, "CREATE TABLE t (a INTEGER)")
	v, ok := m.ActiveView()
	if !ok || v.ID != s.ID || !v.HasDB || v.TxOpen || v.LastResult == nil || len(v.History) != 1 {
		t.Fatalf("view = %+v", v)
	}
	v.History[0].SQL = "changed"
	if s.History[0].SQL == "changed" {
		t.Fatal("history not copied")
	}
	if _, ok := m.View("nope"); ok {
		t.Fatal("unknown session")
	}
	empty := NewManagerWithOpener(m.opener)
	if _, ok := empty.ActiveView(); ok {
		t.Fatal("no active session")
	}
}
