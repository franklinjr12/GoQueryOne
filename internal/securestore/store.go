package securestore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Protector encrypts/decrypts secret bytes at rest. The default
// implementation uses Windows DPAPI; non-Windows builds get an
// implementation that always returns an "unsupported" error, preserving the
// pre-existing behavior on those platforms. Tests construct a Store with
// NewWithProtector and a fake Protector instead.
type Protector interface {
	Protect(plain []byte) ([]byte, error)
	Unprotect(cipher []byte) ([]byte, error)
}

type Store struct {
	mu        sync.Mutex
	path      string
	entries   map[string]string
	protector Protector
}

// New builds a Store backed by the platform's default Protector (DPAPI on
// Windows; an "unsupported" error on every other OS).
func New(path string) (*Store, error) {
	return NewWithProtector(path, defaultProtector())
}

// NewWithProtector builds a Store that encrypts/decrypts secrets through
// protector instead of the platform default. Tests use this to inject a
// fake Protector so credential round-tripping can be exercised without
// Windows DPAPI.
func NewWithProtector(path string, protector Protector) (*Store, error) {
	if path == "" {
		return nil, errors.New("secure store path is required")
	}
	if protector == nil {
		protector = defaultProtector()
	}
	s := &Store{
		path:      path,
		entries:   map[string]string{},
		protector: protector,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &s.entries)
}

func (s *Store) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s *Store) Save(secretRef, secret string) error {
	if secretRef == "" {
		return errors.New("secret ref is required")
	}
	protected, err := s.protector.Protect([]byte(secret))
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(protected)
	s.mu.Lock()
	s.entries[secretRef] = encoded
	err = s.persist()
	s.mu.Unlock()
	return err
}

func (s *Store) Load(secretRef string) (string, error) {
	if secretRef == "" {
		return "", errors.New("secret ref is required")
	}
	s.mu.Lock()
	encoded, ok := s.entries[secretRef]
	s.mu.Unlock()
	if !ok {
		return "", errors.New("secret not found")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	unprotected, err := s.protector.Unprotect(raw)
	if err != nil {
		return "", err
	}
	return string(unprotected), nil
}

func (s *Store) Delete(secretRef string) error {
	s.mu.Lock()
	delete(s.entries, secretRef)
	err := s.persist()
	s.mu.Unlock()
	return err
}
