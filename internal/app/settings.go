package app

import (
	"fmt"

	"github.com/franklinjr12/GoQueryOne/internal/config"
)

// Settings are the user-editable query settings (Tools → Settings).
type Settings struct {
	TimeoutSeconds int  // per statement (the executor needs a deadline)
	MaxRows        int  // rows fetched before "Load more"
	PageSize       int  // rows added by "Load more"
	StopOnError    bool // stop a script at the first failing statement
	SaveHistory    bool // persist executed statements (masked) to history.jsonl
	EditorFontSize int  // SQL editor font size in points
	ShowSystemTabs bool // show the "System tables" group in the schema tree
}

// Editor font size limits and default (points).
const (
	MinFontSize     = 6
	MaxFontSize     = 36
	DefaultFontSize = 10
)

// EditorFontSize returns the configured editor font size, or the default
// when unset or out of range.
func EditorFontSize(cfg config.Config) int {
	n := cfg.App.EditorFontSize
	if n < MinFontSize || n > MaxFontSize {
		return DefaultFontSize
	}
	return n
}

// Limits for the settings dialog.
const (
	MaxTimeoutSeconds = 24 * 60 * 60
	MinRows           = 1
	MaxRowsLimit      = 1_000_000
)

// SettingsFromConfig reads the settings from the config.
func SettingsFromConfig(cfg config.Config) Settings {
	q := cfg.Query
	return Settings{
		SaveHistory:    cfg.App.SaveHistory,
		EditorFontSize: EditorFontSize(cfg),
		ShowSystemTabs: !cfg.App.HideSystemTables,
		TimeoutSeconds: q.DefaultTimeoutMs / 1000,
		MaxRows:        q.DefaultMaxRows,
		PageSize:       q.FetchPageSize,
		StopOnError:    q.StopOnErrorInScript,
	}
}

// Validate checks the ranges and returns a user-facing message for the
// first problem.
func (s Settings) Validate() error {
	switch {
	case s.TimeoutSeconds < 1 || s.TimeoutSeconds > MaxTimeoutSeconds:
		return fmt.Errorf("timeout must be between 1 and %d seconds", MaxTimeoutSeconds)
	case s.MaxRows < MinRows || s.MaxRows > MaxRowsLimit:
		return fmt.Errorf("maximum rows must be between %d and %d", MinRows, MaxRowsLimit)
	case s.PageSize < MinRows || s.PageSize > MaxRowsLimit:
		return fmt.Errorf("load-more page size must be between %d and %d", MinRows, MaxRowsLimit)
	case s.EditorFontSize < MinFontSize || s.EditorFontSize > MaxFontSize:
		return fmt.Errorf("editor font size must be between %d and %d", MinFontSize, MaxFontSize)
	}
	return nil
}

// Apply writes valid settings into the config.
func (s Settings) Apply(cfg *config.Config) error {
	if err := s.Validate(); err != nil {
		return err
	}
	cfg.App.SaveHistory = s.SaveHistory
	cfg.App.EditorFontSize = s.EditorFontSize
	cfg.App.HideSystemTables = !s.ShowSystemTabs
	q := &cfg.Query
	q.DefaultTimeoutMs = s.TimeoutSeconds * 1000
	q.DefaultMaxRows = s.MaxRows
	q.FetchPageSize = s.PageSize
	q.StopOnErrorInScript = s.StopOnError
	return nil
}
