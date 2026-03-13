package securestore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu      sync.Mutex
	path    string
	entries map[string]string
}

func New(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("secure store path is required")
	}
	s := &Store{
		path:    path,
		entries: map[string]string{},
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
	protected, err := protectData([]byte(secret))
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
	unprotected, err := unprotectData(raw)
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
