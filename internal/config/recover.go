package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// writeFileAtomic writes data to a temp file in the same directory and
// renames it over path, so a crash or full disk never leaves a truncated
// config behind (os.Rename replaces the target on Windows too).
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	_ = os.Chmod(name, perm)
	if err := os.Rename(name, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// LoadOrRecover loads the config. When an existing file cannot be read or
// parsed it is moved aside (config.json.broken-YYYYMMDD-HHMMSS) so that the
// next save does not silently overwrite the user's profiles, and defaults
// are returned with a user-facing notice. notice is "" on success.
func LoadOrRecover(path string, now time.Time) (cfg *Config, notice string) {
	cfg, err := LoadConfig(path)
	if err == nil {
		return cfg, ""
	}
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		return DefaultConfig(), fmt.Sprintf("The settings file could not be created (%v). Changes will not be saved.", err)
	}
	aside := path + ".broken-" + now.Format("20060102-150405")
	if renErr := os.Rename(path, aside); renErr != nil {
		return DefaultConfig(), fmt.Sprintf("The settings file %s could not be read (%v) and could not be moved aside (%v). Defaults are used; saving may overwrite it.", path, err, renErr)
	}
	return DefaultConfig(), fmt.Sprintf("The settings file could not be read (%v).\n\nIt was kept as %s and default settings are used. Saved connections can be copied back from that file.", err, aside)
}
