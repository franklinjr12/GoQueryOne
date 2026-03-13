package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigMigratesLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	legacy := `{
  "database": {
    "dsn": "LegacyDSN",
    "username": "legacyUser",
    "password": "legacyPass",
    "timeout": "30s"
  },
  "app": {
    "log_level": "debug",
    "query_timeout": "45s",
    "max_rows": 123
  }
}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("failed to load migrated config: %v", err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", CurrentSchemaVersion, cfg.SchemaVersion)
	}
	if cfg.Query.DefaultTimeoutMs != 45000 {
		t.Fatalf("expected migrated timeout 45000, got %d", cfg.Query.DefaultTimeoutMs)
	}
	if cfg.Query.DefaultMaxRows != 123 {
		t.Fatalf("expected migrated max rows 123, got %d", cfg.Query.DefaultMaxRows)
	}
	if len(cfg.Connections) != 1 || cfg.Connections[0].DSN != "LegacyDSN" {
		t.Fatalf("expected migrated connection with LegacyDSN, got %#v", cfg.Connections)
	}
	if _, err := os.Stat(path + ".legacy.bak"); err != nil {
		t.Fatalf("expected backup file to exist: %v", err)
	}
}

func TestSaveAndLoadConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := DefaultConfig()
	cfg.Connections = append(cfg.Connections, ConnectionProfile{
		ID:       "abc",
		Name:     "Main",
		Type:     "dsn",
		DSN:      "MyDSN",
		Username: "u",
	})
	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("failed saving config: %v", err)
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("failed loading config: %v", err)
	}
	if len(loaded.Connections) != 1 {
		t.Fatalf("expected one connection, got %d", len(loaded.Connections))
	}
	if loaded.Connections[0].Name != "Main" {
		t.Fatalf("expected connection name Main, got %s", loaded.Connections[0].Name)
	}
}
