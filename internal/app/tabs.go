package app

import (
	"path/filepath"
	"strings"
)

// MaxRestoredFiles caps how many SQL files are reopened at startup.
const MaxRestoredFiles = 20

// FilesToRestore returns the saved open-file list cleaned up for reopening:
// blanks, duplicates (case-insensitive, as on Windows) and files that no
// longer exist are dropped, order is kept, and at most MaxRestoredFiles
// are returned.
func FilesToRestore(paths []string, exists func(string) bool) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(p))
		if seen[key] || !exists(p) {
			continue
		}
		seen[key] = true
		out = append(out, p)
		if len(out) == MaxRestoredFiles {
			break
		}
	}
	return out
}
