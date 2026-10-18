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
}

// Limits for the settings dialog.
const (
	MaxTimeoutSeconds = 24 * 60 * 60
	MinRows           = 1
	MaxRowsLimit      = 1_000_000
)

// SettingsFromConfig reads the settings from the config.
func SettingsFromConfig(q config.QueryConfig) Settings {
	return Settings{
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
	}
	return nil
}

// Apply writes valid settings into the config.
func (s Settings) Apply(q *config.QueryConfig) error {
	if err := s.Validate(); err != nil {
		return err
	}
	q.DefaultTimeoutMs = s.TimeoutSeconds * 1000
	q.DefaultMaxRows = s.MaxRows
	q.FetchPageSize = s.PageSize
	q.StopOnErrorInScript = s.StopOnError
	return nil
}
