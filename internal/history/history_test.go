package history

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendRecentAndCompaction(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "sub", "history.jsonl"), 3)
	base := time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC)
	for i := 0; i < 7; i++ {
		p := "a"
		if i%2 == 1 {
			p = "b"
		}
		if err := s.Append(Record{ProfileID: p, When: base.Add(time.Duration(i) * time.Minute), SQL: fmt.Sprintf("SELECT %d", i), Status: "ok", DurationMs: int64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := s.Recent("", 0)
	if err != nil {
		t.Fatal(err)
	}
	// 7 > 2*3 triggered compaction to the newest 3 after the 7th append.
	if len(all) != 3 || all[0].SQL != "SELECT 4" || all[2].SQL != "SELECT 6" {
		t.Fatalf("after compaction: %+v", all)
	}
	a, _ := s.Recent("a", 10)
	if len(a) != 2 || a[0].SQL != "SELECT 4" || a[1].SQL != "SELECT 6" {
		t.Fatalf("profile a: %+v", a)
	}
	last, _ := s.Recent("", 1)
	if len(last) != 1 || last[0].SQL != "SELECT 6" {
		t.Fatalf("last: %+v", last)
	}
	info, err := os.Stat(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 && os.PathSeparator == '/' {
		t.Fatalf("history file should be private, mode %v", perm)
	}
}

func TestRecentMissingFileAndCorruptLines(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "h.jsonl"), 0)
	if got, err := s.Recent("", 5); err != nil || len(got) != 0 {
		t.Fatalf("missing file: %v %v", got, err)
	}
	if err := os.WriteFile(s.Path(), []byte("{not json}\n{\"sql\":\"SELECT 1\",\"profile\":\"p\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Recent("p", 5)
	if err != nil || len(got) != 1 || got[0].SQL != "SELECT 1" {
		t.Fatalf("corrupt line handling: %+v %v", got, err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal("clearing twice must not fail")
	}
	if _, err := os.Stat(s.Path()); !os.IsNotExist(err) {
		t.Fatal("file still exists")
	}
}
