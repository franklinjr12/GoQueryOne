package history

import (
	"path/filepath"
	"testing"
	"time"
)

func TestWriterFlushesOnClose(t *testing.T) {
	st := New(filepath.Join(t.TempDir(), "h.jsonl"), 100)
	w := NewWriter(st, 10)
	for i := 0; i < 5; i++ {
		w.Enqueue(Record{ProfileID: "p", When: time.Now(), SQL: "SELECT 1"})
	}
	w.Close()
	w.Close() // idempotent
	recs, err := st.Recent("p", 10)
	if err != nil || len(recs) != 5 {
		t.Fatalf("recs=%d err=%v", len(recs), err)
	}
	if w.Dropped() != 0 {
		t.Fatal("dropped")
	}
}

func TestWriterDropsWhenFull(t *testing.T) {
	st := New(filepath.Join(t.TempDir(), "h.jsonl"), 100)
	w := &Writer{store: st, ch: make(chan Record)} // no reader, no buffer
	w.Enqueue(Record{ProfileID: "p"})
	if w.Dropped() != 1 {
		t.Fatal("expected a drop")
	}
	if NewWriter(st, 0) == nil {
		t.Fatal("default buffer")
	}
}
