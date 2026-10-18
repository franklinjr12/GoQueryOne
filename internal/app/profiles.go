package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/franklinjr12/GoQueryOne/internal/config"
)

// NewProfileID derives a unique, readable profile id from a name.
func NewProfileID(name string, now time.Time) string {
	clean := strings.ToLower(strings.TrimSpace(name))
	clean = strings.Join(strings.Fields(clean), "-")
	if clean == "" {
		clean = "connection"
	}
	return fmt.Sprintf("%s-%d", clean, now.UnixNano())
}

// ProfileLabel is how a profile is listed in the connection dialog.
func ProfileLabel(p config.ConnectionProfile) string {
	return p.Name + " [" + p.Type + "]"
}

// UniqueProfileName returns base, or "base (2)", "base (3)"... so that it
// does not clash (case-insensitively) with an existing profile name.
func UniqueProfileName(base string, existing []config.ConnectionProfile) string {
	taken := map[string]bool{}
	for _, p := range existing {
		taken[strings.ToLower(p.Name)] = true
	}
	if !taken[strings.ToLower(base)] {
		return base
	}
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s (%d)", base, i)
		if !taken[strings.ToLower(name)] {
			return name
		}
	}
}

// DuplicateProfile copies p under a new id and a unique "<name> copy" name.
// The saved password is not shared: the copy has no credential reference
// (deleting one profile must never remove the other's secret), so the user
// re-enters the password for the copy.
func DuplicateProfile(p config.ConnectionProfile, existing []config.ConnectionProfile, now time.Time) config.ConnectionProfile {
	cp := p
	cp.Name = UniqueProfileName(p.Name+" copy", existing)
	cp.ID = NewProfileID(cp.Name, now)
	cp.CredentialRef = ""
	cp.SavePassword = false
	return cp
}
