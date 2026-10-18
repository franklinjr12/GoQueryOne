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

// MaxRecentFiles caps File > Recent files.
const MaxRecentFiles = 10

// AddRecentFile puts path at the front of the recent-files list, dropping an
// older entry for the same file (case-insensitive, as on Windows) and
// keeping at most MaxRecentFiles entries. The input slice is not modified.
func AddRecentFile(list []string, path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return list
	}
	out := []string{path}
	for _, p := range RemoveRecentFile(list, path) {
		if len(out) == MaxRecentFiles {
			break
		}
		out = append(out, p)
	}
	return out
}

// RemoveRecentFile returns list without path (and without blank entries).
func RemoveRecentFile(list []string, path string) []string {
	key := strings.ToLower(filepath.Clean(strings.TrimSpace(path)))
	var out []string
	for _, p := range list {
		if strings.TrimSpace(p) == "" || strings.ToLower(filepath.Clean(p)) == key {
			continue
		}
		out = append(out, p)
	}
	return out
}

// RecentFileLabel is the menu text for entry i (0-based): "&1 name  (dir)".
// Mnemonics 1-9 then 0; ampersands in the path are escaped.
func RecentFileLabel(i int, path string) string {
	esc := strings.ReplaceAll(path, "&", "&&")
	n := (i + 1) % 10
	return "&" + string(rune('0'+n)) + " " + esc
}
