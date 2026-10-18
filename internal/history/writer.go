package history

import (
	"log"
	"sync"
)

// Writer appends records to a Store on its own goroutine, so callers that
// hold locks (the ODBC Manager) never wait for disk I/O.
type Writer struct {
	store *Store
	ch    chan Record
	wg    sync.WaitGroup
	once  sync.Once
	// Dropped counts records dropped because the queue was full.
	mu      sync.Mutex
	dropped int
}

// NewWriter starts a writer with a queue of size buffer.
func NewWriter(store *Store, buffer int) *Writer {
	if buffer <= 0 {
		buffer = 256
	}
	w := &Writer{store: store, ch: make(chan Record, buffer)}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for r := range w.ch {
			if err := w.store.Append(r); err != nil {
				log.Printf("history: %v", err)
			}
		}
	}()
	return w
}

// Enqueue queues r without blocking; when the queue is full the record is
// dropped (and counted) rather than stalling query execution.
func (w *Writer) Enqueue(r Record) {
	select {
	case w.ch <- r:
	default:
		w.mu.Lock()
		w.dropped++
		w.mu.Unlock()
	}
}

// Dropped returns how many records were dropped.
func (w *Writer) Dropped() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dropped
}

// Close writes the queued records and stops the writer. Enqueue must not
// be called after Close.
func (w *Writer) Close() {
	w.once.Do(func() {
		close(w.ch)
		w.wg.Wait()
	})
}
