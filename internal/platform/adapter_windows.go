//go:build windows

package platform

import "github.com/lxn/walk"

// WalkClipboard adapts walk.Clipboard() to the Clipboard interface.
type WalkClipboard struct{}

func NewWalkClipboard() WalkClipboard {
	return WalkClipboard{}
}

func (WalkClipboard) SetText(text string) error {
	return walk.Clipboard().SetText(text)
}

func (WalkClipboard) Text() (string, error) {
	return walk.Clipboard().Text()
}

// WalkFileDialogs adapts walk.FileDialog to the FileDialogs interface. A
// dialog needs an owner Form to parent itself to; owner is resolved lazily
// through ownerFn so the adapter can be constructed before the main window
// exists and still pick up its current value (e.g. across dialog reopens).
type WalkFileDialogs struct {
	ownerFn func() walk.Form
}

func NewWalkFileDialogs(ownerFn func() walk.Form) WalkFileDialogs {
	return WalkFileDialogs{ownerFn: ownerFn}
}

func (d WalkFileDialogs) owner() walk.Form {
	if d.ownerFn == nil {
		return nil
	}
	return d.ownerFn()
}

func (d WalkFileDialogs) ShowOpen(req OpenFileRequest) (string, bool, error) {
	dlg := walk.FileDialog{
		Title:       req.Title,
		Filter:      req.Filter,
		FilterIndex: req.FilterIndex,
		FilePath:    req.InitialPath,
	}
	accepted, err := dlg.ShowOpen(d.owner())
	if err != nil {
		return "", false, err
	}
	if !accepted {
		return "", false, nil
	}
	return dlg.FilePath, true, nil
}

func (d WalkFileDialogs) ShowSave(req SaveFileRequest) (string, bool, error) {
	dlg := walk.FileDialog{
		Title:       req.Title,
		Filter:      req.Filter,
		FilterIndex: req.FilterIndex,
		FilePath:    req.InitialPath,
	}
	accepted, err := dlg.ShowSave(d.owner())
	if err != nil {
		return "", false, err
	}
	if !accepted {
		return "", false, nil
	}
	return dlg.FilePath, true, nil
}
