//go:build windows

package ui

import (
	"bytes"
	"embed"
	"fmt"
	"image/png"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/franklinjr12/GoQueryOne/internal/app"
	"github.com/franklinjr12/GoQueryOne/internal/core"
	"github.com/franklinjr12/GoQueryOne/internal/odbc"
	"github.com/lxn/walk"
)

//go:embed icons/*.png
var iconFS embed.FS

const userGuideURL = "https://github.com/franklinjr12/GoQueryOne#readme"

// uiActions are shared by the menu bar and the toolbar so enablement stays
// consistent; enablement is driven by the presenter (internal/app).
type uiActions struct {
	connect, disconnect                             *walk.Action
	connectTo                                       *walk.Action // submenu: saved profiles
	connectToMenu                                   *walk.Menu
	recentMenu                                      *walk.Menu
	recent                                          *walk.Action
	newTab, open, save, saveAs, closeTab, renameTab *walk.Action
	exit                                            *walk.Action
	copyRows, copyAll, copyCSV, copyHeaders         *walk.Action
	find, findNext                                  *walk.Action
	run, runCtrlEnter, runScript, cancel, loadMore  *walk.Action
	begin, commit, rollback                         *walk.Action
	refreshSchema, toggleText                       *walk.Action
	odbcAdminNative, odbcAdmin64, odbcAdmin32       *walk.Action
	settings                                        *walk.Action
	export, help, about                             *walk.Action
	statusItems                                     []*walk.StatusBarItem
	// toolbar holds the icon twin of a menu action. Menus get no images
	// (24px bitmaps overlap menu text); the toolbar twin carries the icon.
	toolbar      map[*walk.Action]*walk.Action
	statusWidths []int
}

func (u *applicationUI) newAction(text, icon string, sc walk.Shortcut, handler func()) *walk.Action {
	trigger := func() {
		handler()
		u.refreshState()
	}
	a := walk.NewAction()
	_ = a.SetText(text)
	if sc.Key != 0 {
		_ = a.SetShortcut(sc)
	}
	a.Triggered().Attach(trigger)
	if icon != "" {
		if bmp := u.loadIcon(icon); bmp != nil {
			tb := walk.NewAction()
			label := strings.TrimSuffix(text, "…")
			if sc.Key != 0 {
				label += " (" + sc.String() + ")"
			}
			_ = tb.SetText(label)
			_ = tb.SetImage(bmp)
			tb.Triggered().Attach(trigger)
			u.actions.toolbar[a] = tb
		}
	}
	return a
}

func (u *applicationUI) loadIcon(name string) *walk.Bitmap {
	raw, err := iconFS.ReadFile("icons/" + name + ".png")
	if err != nil {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	bmp, err := walk.NewBitmapFromImageForDPI(img, 96)
	if err != nil {
		return nil
	}
	return bmp
}

func addSeparator(list *walk.ActionList) { _ = list.Add(walk.NewSeparatorAction()) }

func addMenu(mw *walk.MainWindow, title string, actions ...*walk.Action) error {
	menu, err := walk.NewMenu()
	if err != nil {
		return err
	}
	for _, a := range actions {
		if a == nil {
			addSeparator(menu.Actions())
			continue
		}
		if err := menu.Actions().Add(a); err != nil {
			return err
		}
	}
	ma := walk.NewMenuAction(menu)
	_ = ma.SetText(title)
	return mw.Menu().Actions().Add(ma)
}

// buildActions creates all actions, the menu bar, the icon toolbar (with
// tooltips = text + shortcut) and the status bar sections.
func (u *applicationUI) buildActions() error {
	ctrl := func(k walk.Key) walk.Shortcut { return walk.Shortcut{Modifiers: walk.ModControl, Key: k} }
	key := func(k walk.Key) walk.Shortcut { return walk.Shortcut{Key: k} }
	a := &uiActions{toolbar: map[*walk.Action]*walk.Action{}}
	u.actions = a

	a.connect = u.newAction("Connect…", "connect", walk.Shortcut{Modifiers: walk.ModControl | walk.ModShift, Key: walk.KeyC}, u.showConnectionWindow)
	a.disconnect = u.newAction("Disconnect", "disconnect", walk.Shortcut{}, u.disconnectActive)
	a.newTab = u.newAction("New tab", "newtab", ctrl(walk.KeyN), func() { u.addEditorTab("", "") })
	a.open = u.newAction("Open…", "open", ctrl(walk.KeyO), u.openSQLFile)
	a.save = u.newAction("Save", "save", ctrl(walk.KeyS), func() { u.saveTab(u.currentEditorTab(), false) })
	a.saveAs = u.newAction("Save as…", "", walk.Shortcut{}, func() { u.saveTab(u.currentEditorTab(), true) })
	a.closeTab = u.newAction("Close tab", "", ctrl(walk.KeyW), u.closeCurrentTab)
	a.renameTab = u.newAction("Rename tab…", "", walk.Shortcut{}, u.renameCurrentTab)
	a.exit = u.newAction("Exit", "", ctrl(walk.KeyQ), func() { _ = u.mainWindow.Close() })

	a.copyRows = u.newAction("Copy selected rows", "copy", walk.Shortcut{}, func() { u.copySelectedRows(false) })
	a.copyHeaders = u.newAction("Copy selected rows with headers", "", walk.Shortcut{}, func() { u.copySelectedRows(true) })
	a.copyAll = u.newAction("Copy all (TSV, Excel)", "", walk.Shortcut{}, func() { u.copyAllRows("\t") })
	a.copyCSV = u.newAction("Copy all (CSV)", "", walk.Shortcut{}, func() { u.copyAllRows(",") })

	a.find = u.newAction("Find…", "", ctrl(walk.KeyF), u.findInEditor)
	a.findNext = u.newAction("Find next", "", key(walk.KeyF3), u.findNext)

	a.run = u.newAction("Run statement", "run", key(walk.KeyF5), u.runStatement)
	a.runCtrlEnter = u.newAction("Run statement", "", ctrl(walk.KeyReturn), u.runStatement)
	a.runScript = u.newAction("Run script", "runscript", key(walk.KeyF6), u.runScript)
	a.cancel = u.newAction("Cancel", "cancel", key(walk.KeyEscape), u.cancelExecution)
	a.loadMore = u.newAction("Load more rows", "", walk.Shortcut{}, u.loadMoreRows)
	a.begin = u.newAction("Begin transaction", "begin", walk.Shortcut{}, func() { u.transactionAction("begin") })
	a.commit = u.newAction("Commit", "commit", walk.Shortcut{}, func() { u.transactionAction("commit") })
	a.rollback = u.newAction("Rollback", "rollback", walk.Shortcut{}, func() { u.transactionAction("rollback") })

	a.refreshSchema = u.newAction("Refresh schema", "refresh", key(walk.KeyF8), func() { u.loadSchema(true) })
	a.toggleText = u.newAction("Toggle grid / text", "toggle", key(walk.KeyF9), u.toggleTextView)

	nativeArch := "x64"
	if runtime.GOARCH == "386" {
		nativeArch = "x86"
	}
	a.odbcAdminNative = u.newAction("ODBC Administrator ("+nativeArch+")", "", ctrl(walk.KeyT), func() { u.openODBCAdmin(nativeArch) })
	a.odbcAdmin64 = u.newAction("ODBC Administrator x64", "", walk.Shortcut{}, func() { u.openODBCAdmin("x64") })
	a.odbcAdmin32 = u.newAction("ODBC Administrator x86", "", walk.Shortcut{}, func() { u.openODBCAdmin("x86") })
	a.settings = u.newAction("Settings…", "", walk.Shortcut{}, u.showSettings)
	a.export = u.newAction("Export results…", "export", walk.Shortcut{}, u.exportResults)
	a.help = u.newAction("User guide", "help", key(walk.KeyF1), u.openUserGuide)
	a.about = u.newAction("About GoQueryOne", "", walk.Shortcut{}, u.showAbout)

	if m, err := walk.NewMenu(); err == nil {
		a.connectToMenu = m
		a.connectTo = walk.NewMenuAction(m)
		_ = a.connectTo.SetText("Connect &to")
	}

	if m, err := walk.NewMenu(); err == nil {
		a.recentMenu = m
		a.recent = walk.NewMenuAction(m)
		_ = a.recent.SetText("&Recent files")
	}

	mw := u.mainWindow
	menus := []struct {
		title   string
		actions []*walk.Action
	}{
		{"&File", []*walk.Action{a.newTab, a.open, a.save, a.saveAs, nil, a.recent, nil, a.renameTab, a.closeTab, nil, a.exit}},
		{"&Edit", []*walk.Action{a.find, a.findNext, nil, a.copyRows, a.copyHeaders, a.copyAll, a.copyCSV}},
		{"&Query", []*walk.Action{a.run, a.runScript, a.cancel, a.loadMore, nil, a.begin, a.commit, a.rollback}},
		{"&View", []*walk.Action{a.refreshSchema, a.toggleText}},
		{"&Tools", []*walk.Action{a.connect, a.connectTo, a.disconnect, nil, a.odbcAdminNative, a.odbcAdmin64, a.odbcAdmin32, nil, a.export, nil, a.settings}},
		{"&Help", []*walk.Action{a.help, a.about}},
	}
	for _, m := range menus {
		if err := addMenu(mw, m.title, m.actions...); err != nil {
			return err
		}
	}
	// Ctrl+Enter is a second shortcut for Run; it lives only in the shortcut table.
	if err := mw.ShortcutActions().Add(a.runCtrlEnter); err != nil {
		return err
	}

	tb, err := walk.NewToolBarWithOrientationAndButtonStyle(mw, walk.Horizontal, walk.ToolBarButtonImageOnly)
	if err != nil {
		return err
	}
	il, err := walk.NewImageListForDPI(walk.Size{Width: 24, Height: 24}, 0, 96)
	if err != nil {
		return err
	}
	tb.SetImageList(il)
	old := mw.ToolBar()
	mw.SetToolBar(tb)
	if old != nil {
		old.Dispose()
	}
	for _, act := range []*walk.Action{
		a.connect, a.disconnect, nil,
		a.newTab, a.open, a.save, nil,
		a.run, a.runScript, a.cancel, nil,
		a.begin, a.commit, a.rollback, nil,
		a.copyRows, a.export, nil,
		a.refreshSchema, a.toggleText, nil,
		a.help,
	} {
		if act == nil {
			addSeparator(tb.Actions())
			continue
		}
		if twin, ok := a.toolbar[act]; ok {
			act = twin
		}
		if err := tb.Actions().Add(act); err != nil {
			return err
		}
	}

	sb := mw.StatusBar()
	a.statusWidths = []int{260, 90, 120, 330, 110}
	for _, w := range a.statusWidths {
		item := walk.NewStatusBarItem()
		_ = item.SetWidth(w)
		if err := sb.Items().Add(item); err != nil {
			return err
		}
		a.statusItems = append(a.statusItems, item)
	}
	sb.SetVisible(true)

	u.presenter = app.NewPresenter(u)
	return nil
}

// --- app.View implementation ---

func (u *applicationUI) setEnabled(a *walk.Action, v bool) {
	if a != nil && a.Enabled() != v {
		_ = a.SetEnabled(v)
	}
	if twin := u.actions.toolbar[a]; twin != nil && twin.Enabled() != v {
		_ = twin.SetEnabled(v)
	}
}

// SetActionsEnabled applies presenter enablement to the shared actions.
func (u *applicationUI) SetActionsEnabled(e app.Actions) {
	a := u.actions
	if a == nil {
		return
	}
	u.setEnabled(a.connect, e.Connect)
	u.setEnabled(a.disconnect, e.Disconnect)
	u.setEnabled(a.newTab, e.NewTab)
	u.setEnabled(a.open, e.Open)
	u.setEnabled(a.save, e.Save)
	u.setEnabled(a.saveAs, e.SaveAs)
	u.setEnabled(a.closeTab, e.CloseTab)
	u.setEnabled(a.renameTab, e.CloseTab)
	u.setEnabled(a.run, e.Run)
	u.setEnabled(a.runCtrlEnter, e.Run)
	u.setEnabled(a.runScript, e.RunScript)
	u.setEnabled(a.cancel, e.Cancel)
	u.setEnabled(a.loadMore, e.LoadMore)
	u.setEnabled(a.begin, e.Begin)
	u.setEnabled(a.commit, e.Commit)
	u.setEnabled(a.rollback, e.Rollback)
	u.setEnabled(a.refreshSchema, e.RefreshSchema)
	u.setEnabled(a.copyRows, e.CopyRows)
	u.setEnabled(a.copyHeaders, e.CopyRows)
	u.setEnabled(a.copyAll, e.CopyAll)
	u.setEnabled(a.copyCSV, e.CopyAll)
	u.setEnabled(a.export, e.Export)
	u.setEnabled(a.toggleText, e.ToggleText)
	if u.loadMoreButton != nil {
		u.loadMoreButton.SetVisible(e.LoadMore)
	}
}

// SetStatus renders the status bar sections.
func (u *applicationUI) SetStatus(sections []string) {
	if u.actions == nil {
		return
	}
	for i, item := range u.actions.statusItems {
		text := ""
		if i < len(sections) {
			text = sections[i]
		}
		if i == app.SectionActivity && u.activityText != "" {
			text = u.activityText
		}
		if item.Text() != text {
			_ = item.SetText(text)
		}
	}
}

// SetTitle sets the main window title.
func (u *applicationUI) SetTitle(title string) {
	if u.mainWindow != nil && u.mainWindow.Title() != title {
		_ = u.mainWindow.SetTitle(title)
	}
}

// setActivity shows a transient activity text ("Loading schema…") in the
// status bar until the next refreshState with an empty text.
func (u *applicationUI) setActivity(text string) {
	u.activityText = text
	u.refreshState()
}

// refreshState collects the current UI/session state and lets the presenter
// update enablement, status bar and title.
func (u *applicationUI) refreshState() {
	if u.presenter == nil {
		return
	}
	s := app.State{SessionCount: len(u.manager.SessionList()), Executing: u.executing, ExecutingSince: u.executingSince}
	if session, ok := u.manager.ActiveView(); ok {
		s.Connected = session.HasDB && session.State != odbc.SessionDisconnected
		s.ConnectionName = session.Name
		s.TxOpen = session.TxOpen
		if r := session.LastResult; r != nil {
			s.Last = outcomeOf(*r)
		}
	}
	if u.editorTabs != nil {
		s.TabCount = u.editorTabs.Pages().Len()
	}
	if tab := u.currentEditorTab(); tab != nil {
		s.TabTitle = tab.Title
		s.TabDirty = tab.Dirty
		s.TabHasPath = tab.FilePath != ""
		text := tab.Editor.Text()
		s.EditorHasSQL = strings.TrimSpace(text) != ""
		caret, _ := tab.Editor.TextSelection()
		s.Line, s.Col = app.LineCol(text, core.UTF16OffsetToByteOffset(text, caret))
	}
	s.HasResultRows = len(u.resultsColumns) > 0
	if u.resultsTable != nil {
		s.RowsSelected = len(u.resultsTable.SelectedIndexes())
	}
	s.CanLoadMore = u.lastTruncated && odbc.IsQueryStatement(u.lastQuerySQL)
	u.presenter.Update(s)
}

func outcomeOf(r odbc.StatementResult) *app.Outcome {
	return &app.Outcome{
		HasRows:      r.HasRows,
		Rows:         r.ResultSet.RowCount,
		RowsAffected: r.RowsAffected,
		Duration:     r.ExecutionTime,
		Truncated:    r.ResultSet.Truncated,
		TruncatedAt:  r.ResultSet.TruncatedAt,
		Err:          r.ErrorMessage,
		Canceled:     r.Canceled,
		TimedOut:     r.TimedOut,
		Statement:    r.Statement,
		At:           r.CompletedAt,
	}
}

func (u *applicationUI) disconnectActive() {
	active, err := u.manager.ActiveSession()
	if err != nil {
		return
	}
	if active.Tx != nil {
		if walk.MsgBox(u.mainWindow, "Disconnect", "A transaction is open on "+active.Name+".\nIt will be rolled back. Disconnect anyway?", walk.MsgBoxYesNo|walk.MsgBoxIconWarning) != walk.DlgCmdYes {
			return
		}
	}
	if err := u.manager.Disconnect(active.ID); err != nil {
		showErrorBox(u.mainWindow, "Disconnect", err)
		return
	}
	u.appendMessage(time.Now().Format("15:04:05") + "  Disconnected from " + active.Name + ".")
	u.refreshSessionSelect()
	u.refreshState()
}

func (u *applicationUI) openODBCAdmin(arch string) {
	path, openErr := odbc.OpenODBCAdmin(arch)
	if openErr != nil {
		showErrorBox(u.mainWindow, "ODBC Administrator", fmt.Errorf("%w (path: %s)", openErr, path))
	}
}

func (u *applicationUI) openUserGuide() {
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", userGuideURL).Start(); err != nil {
		showInfoBox(u.mainWindow, "User guide", "Open "+userGuideURL+" in your browser.")
	}
}
