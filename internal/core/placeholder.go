package core

import "regexp"

// placeholderRE matches template placeholders such as <server> or
// <database name> left in a connection string.
var placeholderRE = regexp.MustCompile(`<[A-Za-z][A-Za-z0-9 _.-]*>`)

// UnfilledPlaceholders returns the distinct template placeholders still
// present in s, in order of appearance.
func UnfilledPlaceholders(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range placeholderRE.FindAllString(s, -1) {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}
