//go:build windows

package ui

import (
	"syscall"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

// monoFamilies are tried in order; the first one GDI renders as a
// fixed-pitch font wins. Consolas ships with every supported Windows
// version; the rest are fallbacks (Wine, stripped-down images).
var monoFamilies = []string{"Consolas", "Cascadia Mono", "Lucida Console", "Courier New", "DejaVu Sans Mono"}

// monoFont returns a monospace font of the given point size. GDI silently
// substitutes a proportional face when a family is missing, which breaks the
// aligned text view, so each candidate is checked before it is used.
func monoFont(pointSize int) *walk.Font {
	for _, family := range monoFamilies {
		if isFixedPitch(family) {
			if f, err := walk.NewFont(family, pointSize, 0); err == nil {
				return f
			}
		}
	}
	f, _ := walk.NewFont(monoFamilies[0], pointSize, 0)
	return f
}

func isFixedPitch(family string) bool {
	var lf win.LOGFONT
	lf.LfHeight = -12
	lf.LfCharSet = win.DEFAULT_CHARSET
	name, err := syscall.UTF16FromString(family)
	if err != nil || len(name) > len(lf.LfFaceName) {
		return false
	}
	copy(lf.LfFaceName[:], name)
	hFont := win.CreateFontIndirect(&lf)
	if hFont == 0 {
		return false
	}
	defer win.DeleteObject(win.HGDIOBJ(hFont))
	hdc := win.CreateCompatibleDC(0)
	if hdc == 0 {
		return false
	}
	defer win.DeleteDC(hdc)
	old := win.SelectObject(hdc, win.HGDIOBJ(hFont))
	defer win.SelectObject(hdc, old)
	var tm win.TEXTMETRIC
	if !win.GetTextMetrics(hdc, &tm) {
		return false
	}
	// Despite its name, TMPF_FIXED_PITCH set means *variable* pitch.
	const tmpfFixedPitch = 0x01
	return tm.TmPitchAndFamily&tmpfFixedPitch == 0
}
