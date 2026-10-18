//go:build windows

package ui

import (
	"github.com/franklinjr12/GoQueryOne/internal/core"
)

// findInEditor asks for a search text (Edit > Find…, Ctrl+F) and selects
// the next match in the current SQL tab.
func (u *applicationUI) findInEditor() {
	if u.currentEditorTab() == nil {
		return
	}
	needle, ok := u.promptLineOK("Find", "Find what (case-insensitive)", u.lastFind, "Find")
	if !ok || needle == "" {
		return
	}
	u.lastFind = needle
	u.findNext()
}

// findNext selects the next match of the last search after the caret
// (F3), wrapping to the top.
func (u *applicationUI) findNext() {
	tab := u.currentEditorTab()
	if tab == nil {
		return
	}
	if u.lastFind == "" {
		u.findInEditor()
		return
	}
	text := tab.Editor.Text()
	_, selEnd := tab.Editor.TextSelection()
	from := core.UTF16OffsetToByteOffset(text, selEnd)
	start, end, wrapped, ok := core.FindNext(text, u.lastFind, from, false, true)
	if !ok {
		u.setActivity("\"" + u.lastFind + "\" not found")
		return
	}
	tab.Editor.SetTextSelection(core.ByteOffsetToUTF16Offset(text, start), core.ByteOffsetToUTF16Offset(text, end))
	tab.Editor.ScrollToCaret()
	_ = tab.Editor.SetFocus()
	if wrapped {
		u.setActivity("Search wrapped to the top")
	} else {
		u.setActivity("")
	}
}
