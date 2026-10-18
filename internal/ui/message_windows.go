//go:build windows

package ui

import "github.com/lxn/walk"

// ShowMessage shows a standalone message box (no owner window), for
// startup problems and crashes that happen outside the main window.
func ShowMessage(title, text string, isError bool) {
	style := walk.MsgBoxOK | walk.MsgBoxIconInformation
	if isError {
		style = walk.MsgBoxOK | walk.MsgBoxIconError
	}
	walk.MsgBox(nil, title, text, style)
}
