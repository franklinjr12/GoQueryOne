package odbc

import (
	"strings"
	"testing"
	"time"

	"github.com/franklinjr12/GoQueryOne/internal/config"
)

func TestHistoryHooks(t *testing.T) {
	m, s := newSQLiteManager(t)
	var saved []HistoryEntry
	m.SetHistoryHooks(func(profileID string, e HistoryEntry) {
		if profileID != s.Profile.ID {
			t.Errorf("profile = %q", profileID)
		}
		saved = append(saved, e)
	}, nil)
	mustExec(t, m, s.ID, "CREATE TABLE t (a INTEGER)")
	if len(saved) != 1 || !strings.Contains(saved[0].SQL, "CREATE TABLE t") {
		t.Fatalf("saved = %+v", saved)
	}

	// load pre-fills a new session and is capped.
	m2 := NewManagerWithOpener(m.opener)
	var old []HistoryEntry
	for i := 0; i < maxHistoryEntries+5; i++ {
		old = append(old, HistoryEntry{When: time.Now(), SQL: "SELECT 1", Status: "ok"})
	}
	m2.SetHistoryHooks(nil, func(profileID string) []HistoryEntry {
		if profileID != "p2" {
			t.Errorf("load profile = %q", profileID)
		}
		return old
	})
	s2, err := m2.Connect(config.ConnectionProfile{ID: "p2", Name: "two", Type: "connection_string", ConnectionString: s.connStr}, "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.History) != maxHistoryEntries {
		t.Fatalf("preloaded %d entries", len(s2.History))
	}
}

func TestScriptStopsAfterCancel(t *testing.T) {
	m, s := newSQLiteManager(t)
	calls := 0
	// The save hook runs under the Manager lock after each statement:
	// simulate Cancel pressed while the first statement ran.
	m.SetHistoryHooks(func(string, HistoryEntry) {
		calls++
		s.scriptCanceled = true
	}, nil)
	res, err := m.ExecuteScript(s.ID, "CREATE TABLE a (x INTEGER); CREATE TABLE b (x INTEGER); CREATE TABLE c (x INTEGER);", ScriptOptions{})
	if err == nil || !res.Canceled {
		t.Fatalf("err=%v canceled=%v", err, res.Canceled)
	}
	if calls != 1 || len(res.Results) != 1 {
		t.Fatalf("ran %d statements (%d results), want 1", calls, len(res.Results))
	}
	// The next script starts fresh.
	m.SetHistoryHooks(nil, nil)
	if _, err := m.ExecuteScript(s.ID, "CREATE TABLE d (x INTEGER);", ScriptOptions{}); err != nil {
		t.Fatal(err)
	}
}
