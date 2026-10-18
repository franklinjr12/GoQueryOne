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
