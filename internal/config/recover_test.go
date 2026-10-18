package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadOrRecoverBrokenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 18, 3, 4, 5, 0, time.UTC)
	cfg, notice := LoadOrRecover(path, now)
	if cfg == nil || notice == "" {
		t.Fatalf("cfg=%v notice=%q", cfg, notice)
	}
	aside := path + ".broken-20261018-030405"
	if b, err := os.ReadFile(aside); err != nil || string(b) != "{not json" {
		t.Fatalf("aside: %v %q", err, b)
	}
	if !strings.Contains(notice, aside) {
		t.Fatalf("notice %q", notice)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("broken file should have been moved")
	}
}

func TestLoadOrRecoverOK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, notice := LoadOrRecover(path, time.Now())
	if cfg == nil || notice != "" {
		t.Fatalf("notice %q", notice)
	}
	cfg.App.Theme = "x"
	if err := SaveConfig(cfg, path); err != nil {
		t.Fatal(err)
	}
	again, notice := LoadOrRecover(path, time.Now())
	if notice != "" || again.App.Theme != "x" {
		t.Fatalf("reload: %q %+v", notice, again.App)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("temp file left: %s", e.Name())
		}
	}
}

func TestWriteFileAtomicBadDir(t *testing.T) {
	if err := writeFileAtomic(filepath.Join(t.TempDir(), "missing", "c.json"), []byte("x"), 0o644); err == nil {
		t.Fatal("expected error")
	}
}
