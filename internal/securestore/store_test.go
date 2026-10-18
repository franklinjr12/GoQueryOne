package securestore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeRawEntries(path string, entries map[string]string) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return writeRawFile(path, data)
}

func writeRawFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// fakeProtector is a reversible, non-secure stand-in for DPAPI used only in
// tests, so Store round-tripping can be exercised on any OS.
type fakeProtector struct {
	failProtect   error
	failUnprotect error
}

func (f fakeProtector) Protect(plain []byte) ([]byte, error) {
	if f.failProtect != nil {
		return nil, f.failProtect
	}
	out := make([]byte, len(plain))
	for i, b := range plain {
		out[i] = b ^ 0x5A
	}
	return out, nil
}

func (f fakeProtector) Unprotect(cipher []byte) ([]byte, error) {
	if f.failUnprotect != nil {
		return nil, f.failUnprotect
	}
	out := make([]byte, len(cipher))
	for i, b := range cipher {
		out[i] = b ^ 0x5A
	}
	return out, nil
}

func TestNewRequiresPath(t *testing.T) {
	if _, err := NewWithProtector("", fakeProtector{}); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestSaveLoadDeleteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	store, err := NewWithProtector(path, fakeProtector{})
	if err != nil {
		t.Fatalf("NewWithProtector: %v", err)
	}

	if err := store.Save("ref1", "s3cr3t"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load("ref1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "s3cr3t" {
		t.Fatalf("Load() = %q, want %q", got, "s3cr3t")
	}

	if err := store.Delete("ref1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Load("ref1"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestSaveRejectsEmptyRef(t *testing.T) {
	store, err := NewWithProtector(filepath.Join(t.TempDir(), "c.json"), fakeProtector{})
	if err != nil {
		t.Fatalf("NewWithProtector: %v", err)
	}
	if err := store.Save("", "secret"); err == nil {
		t.Fatal("expected error for empty secretRef")
	}
}

func TestLoadRejectsEmptyRef(t *testing.T) {
	store, err := NewWithProtector(filepath.Join(t.TempDir(), "c.json"), fakeProtector{})
	if err != nil {
		t.Fatalf("NewWithProtector: %v", err)
	}
	if _, err := store.Load(""); err == nil {
		t.Fatal("expected error for empty secretRef")
	}
}

func TestLoadMissingSecret(t *testing.T) {
	store, err := NewWithProtector(filepath.Join(t.TempDir(), "c.json"), fakeProtector{})
	if err != nil {
		t.Fatalf("NewWithProtector: %v", err)
	}
	if _, err := store.Load("missing"); err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestSavePropagatesProtectError(t *testing.T) {
	store, err := NewWithProtector(filepath.Join(t.TempDir(), "c.json"), fakeProtector{failProtect: errors.New("boom")})
	if err != nil {
		t.Fatalf("NewWithProtector: %v", err)
	}
	if err := store.Save("ref", "secret"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected protect error to propagate, got %v", err)
	}
}

func TestLoadPropagatesUnprotectError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	store, err := NewWithProtector(path, fakeProtector{})
	if err != nil {
		t.Fatalf("NewWithProtector: %v", err)
	}
	if err := store.Save("ref", "secret"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	broken, err := NewWithProtector(path, fakeProtector{failUnprotect: errors.New("kaboom")})
	if err != nil {
		t.Fatalf("NewWithProtector: %v", err)
	}
	if _, err := broken.Load("ref"); err == nil || !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("expected unprotect error to propagate, got %v", err)
	}
}

func TestLoadRejectsCorruptBase64(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	if err := writeRawEntries(path, map[string]string{"ref": "not-valid-base64!!"}); err != nil {
		t.Fatalf("writeRawEntries: %v", err)
	}
	store, err := NewWithProtector(path, fakeProtector{})
	if err != nil {
		t.Fatalf("NewWithProtector: %v", err)
	}
	if _, err := store.Load("ref"); err == nil {
		t.Fatal("expected error for corrupt base64 payload")
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "credentials.json")

	first, err := NewWithProtector(path, fakeProtector{})
	if err != nil {
		t.Fatalf("NewWithProtector: %v", err)
	}
	if err := first.Save("ref", "value"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	second, err := NewWithProtector(path, fakeProtector{})
	if err != nil {
		t.Fatalf("NewWithProtector (reopen): %v", err)
	}
	got, err := second.Load("ref")
	if err != nil {
		t.Fatalf("Load after reopen: %v", err)
	}
	if got != "value" {
		t.Fatalf("Load() = %q, want %q", got, "value")
	}
}

func TestNewLoadsExistingEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	if err := writeRawFile(path, nil); err != nil {
		t.Fatalf("writeRawFile: %v", err)
	}
	if _, err := NewWithProtector(path, fakeProtector{}); err != nil {
		t.Fatalf("NewWithProtector on empty file: %v", err)
	}
}

func TestDefaultProtectorViaNew(t *testing.T) {
	// New() wires the platform default protector. On non-Windows this is
	// the "unsupported" implementation; Save must surface that error
	// rather than silently succeeding.
	store, err := New(filepath.Join(t.TempDir(), "credentials.json"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = store.Save("ref", "secret")
	if runtime.GOOS == "windows" {
		if err != nil {
			t.Fatalf("Save on windows: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatal("expected unsupported error on non-windows")
	}
}
