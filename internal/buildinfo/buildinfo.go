// Package buildinfo exposes version metadata injected at build time via
// -ldflags "-X github.com/franklinjr12/GoQueryOne/internal/buildinfo.Version=..."
package buildinfo

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version (e.g. 1.0.0); "dev" for local builds.
	Version = "dev"
	// Commit is the short git commit hash.
	Commit = "unknown"
	// Date is the UTC build timestamp (RFC 3339).
	Date = "unknown"
)

// Arch returns a user-facing architecture label (x64 / x86 / GOARCH).
func Arch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	default:
		return runtime.GOARCH
	}
}

// String returns a one-line description such as "GoQueryOne 1.0.0 (abc1234, 2026-10-18T03:00:00Z, x64)".
func String() string {
	return fmt.Sprintf("GoQueryOne %s (%s, %s, %s)", Version, Commit, Date, Arch())
}
