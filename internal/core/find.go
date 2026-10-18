package core

import (
	"strings"
	"unicode/utf8"
)

// FindNext looks for needle in text starting at byte offset from and, when
// wrap is set, continues from the start. It returns the byte range of the
// match and whether the search wrapped. Case-insensitive matching uses
// Unicode simple folding (strings.EqualFold).
func FindNext(text, needle string, from int, matchCase, wrap bool) (start, end int, wrapped, ok bool) {
	if needle == "" || len(needle) > len(text) {
		return 0, 0, false, false
	}
	if from < 0 || from > len(text) {
		from = 0
	}
	find := func(lo, hi int) int {
		for i := lo; i+len(needle) <= hi; {
			if matchCase {
				if strings.HasPrefix(text[i:], needle) {
					return i
				}
			} else if strings.EqualFold(text[i:i+len(needle)], needle) {
				return i
			}
			_, size := utf8.DecodeRuneInString(text[i:])
			i += size
		}
		return -1
	}
	if i := find(from, len(text)); i >= 0 {
		return i, i + len(needle), false, true
	}
	if wrap && from > 0 {
		if i := find(0, min(len(text), from+len(needle)-1)); i >= 0 {
			return i, i + len(needle), true, true
		}
	}
	return 0, 0, false, false
}
