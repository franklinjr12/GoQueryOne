// Package history persists executed statements (opt-in) as JSON lines next
// to config.json. Statements are stored already masked (passwords removed)
// by the caller; the store never sees connection strings.
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Record is one executed statement.
type Record struct {
	ProfileID  string    `json:"profile"`
	When       time.Time `json:"when"`
	SQL        string    `json:"sql"`
	Status     string    `json:"status"`
	DurationMs int64     `json:"ms"`
}

// Store appends records to a JSON-lines file and keeps it bounded.
type Store struct {
	mu   sync.Mutex
	path string
	max  int // records kept after compaction
}

// New returns a store writing to path, keeping about max records.
func New(path string, max int) *Store {
	if max <= 0 {
		max = 1000
	}
	return &Store{path: path, max: max}
}

// Path is the file the store writes to.
func (s *Store) Path() string { return s.path }

// Append adds a record; when the file holds more than twice the limit it is
// compacted to the newest max records.
func (s *Store) Append(r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	line, err := json.Marshal(r)
	if err == nil {
		_, err = f.Write(append(line, '\n'))
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	all, err := s.readAll()
	if err != nil || len(all) <= 2*s.max {
		return err
	}
	return s.writeAll(all[len(all)-s.max:])
}

// Recent returns up to n newest records for a profile (all profiles when
// profileID is empty), oldest first.
func (s *Store) Recent(profileID string, n int) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.readAll()
	if err != nil {
		return nil, err
	}
	var out []Record
	for i := len(all) - 1; i >= 0 && (n <= 0 || len(out) < n); i-- {
		if profileID == "" || all[i].ProfileID == profileID {
			out = append(out, all[i])
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Clear deletes the history file.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Store) readAll() ([]Record, error) {
	f, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var r Record
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r) // corrupt lines are skipped
		}
	}
	return out, sc.Err()
}

func (s *Store) writeAll(records []Record) error {
	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, r := range records {
		line, _ := json.Marshal(r)
		_, _ = w.Write(append(line, '\n'))
	}
	err = w.Flush()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, s.path)
}
