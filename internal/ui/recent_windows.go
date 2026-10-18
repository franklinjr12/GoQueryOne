//go:build windows

package ui

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"

	"github.com/franklinjr12/GoQueryOne/internal/app"
)

// noteRecentFile records a file that was opened or saved in File > Recent files.
func (u *applicationUI) noteRecentFile(path string) {
	u.cfg.App.RecentFiles = app.AddRecentFile(u.cfg.App.RecentFiles, path)
	u.refreshRecentMenu()
}

// refreshRecentMenu rebuilds File > Recent files from the config.
func (u *applicationUI) refreshRecentMenu() {
	if u.actions == nil || u.actions.recentMenu == nil {
		return
	}
	list := u.actions.recentMenu.Actions()
	_ = list.Clear()
	if len(u.cfg.App.RecentFiles) == 0 {
		empty := walk.NewAction()
		_ = empty.SetText("(no recent files)")
		_ = empty.SetEnabled(false)
		_ = list.Add(empty)
		return
	}
	for i, p := range u.cfg.App.RecentFiles {
		path := p
		act := walk.NewAction()
		_ = act.SetText(app.RecentFileLabel(i, path))
		act.Triggered().Attach(func() { u.openRecentFile(path) })
		if err := list.Add(act); err != nil {
			log.Printf("recent files menu: %v", err)
		}
	}
	_ = list.Add(walk.NewSeparatorAction())
	clearAct := walk.NewAction()
	_ = clearAct.SetText("Clear list")
	clearAct.Triggered().Attach(func() {
		u.cfg.App.RecentFiles = nil
		u.refreshRecentMenu()
	})
	_ = list.Add(clearAct)
}

// openRecentFile switches to the tab that already shows path, or opens it.
// A file that no longer exists is dropped from the list.
func (u *applicationUI) openRecentFile(path string) {
	pages := u.editorTabs.Pages()
	for i := 0; i < pages.Len(); i++ {
		if tab := u.editorTabIndex[pages.At(i)]; tab != nil && strings.EqualFold(filepath.Clean(tab.FilePath), filepath.Clean(path)) {
			_ = u.editorTabs.SetCurrentIndex(i)
			_ = tab.Editor.SetFocus()
			return
		}
	}
	if err := u.openSQLPath(path); err != nil {
		u.cfg.App.RecentFiles = app.RemoveRecentFile(u.cfg.App.RecentFiles, path)
		u.refreshRecentMenu()
		showErrorBox(u.mainWindow, "Open recent file", err)
		return
	}
	u.noteRecentFile(path)
}
