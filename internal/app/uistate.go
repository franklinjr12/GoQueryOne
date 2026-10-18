package app

import (
	"strings"
	"time"
)

// UIState is a small key/value store with the method set of walk.Settings,
// used to persist splitter positions inside config.json (portable mode
// keeps everything next to the exe) instead of walk's own ini file.
// Only keys with the given prefixes are kept, so stray widget state does
// not accumulate in the config.
type UIState struct {
	Values   map[string]string
	Prefixes []string
}

// NewUIState wraps values (nil starts empty).
func NewUIState(values map[string]string, prefixes ...string) *UIState {
	if values == nil {
		values = map[string]string{}
	}
	return &UIState{Values: values, Prefixes: prefixes}
}

func (s *UIState) allowed(key string) bool {
	if len(s.Prefixes) == 0 {
		return true
	}
	for _, p := range s.Prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// Get returns the value for key.
func (s *UIState) Get(key string) (string, bool) {
	v, ok := s.Values[key]
	return v, ok
}

// Timestamp is not tracked.
func (s *UIState) Timestamp(string) (time.Time, bool) { return time.Time{}, false }

// Put stores value under key when the key is allowed.
func (s *UIState) Put(key, value string) error {
	if s.allowed(key) {
		s.Values[key] = value
	}
	return nil
}

// PutExpiring is Put (entries never expire).
func (s *UIState) PutExpiring(key, value string) error { return s.Put(key, value) }

// Remove deletes key.
func (s *UIState) Remove(key string) error {
	delete(s.Values, key)
	return nil
}

// ExpireDuration is always zero.
func (s *UIState) ExpireDuration() time.Duration { return 0 }

// SetExpireDuration is a no-op.
func (s *UIState) SetExpireDuration(time.Duration) {}

// Load is a no-op: values come from the config.
func (s *UIState) Load() error { return nil }

// Save is a no-op: the config is saved by the caller.
func (s *UIState) Save() error { return nil }
