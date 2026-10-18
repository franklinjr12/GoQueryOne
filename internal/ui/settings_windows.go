//go:build windows

package ui

import (
	"path/filepath"
	"time"

	"github.com/lxn/walk"

	"github.com/franklinjr12/GoQueryOne/internal/app"
	"github.com/franklinjr12/GoQueryOne/internal/config"
	"github.com/franklinjr12/GoQueryOne/internal/history"
	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

// showSettings edits the query settings (timeout, row limits, script
// behaviour) and saves them to config.json.
func (u *applicationUI) showSettings() {
	dlg, err := walk.NewDialog(u.mainWindow)
	if err != nil {
		showErrorBox(u.mainWindow, "Settings", err)
		return
	}
	_ = dlg.SetTitle("Settings")
	_ = dlg.SetMinMaxSize(walk.Size{Width: 440, Height: 310}, walk.Size{})
	_ = dlg.SetSize(walk.Size{Width: 440, Height: 310})
	layout := walk.NewGridLayout()
	_ = layout.SetMargins(walk.Margins{HNear: 10, VNear: 10, HFar: 10, VFar: 10})
	_ = layout.SetSpacing(8)
	_ = dlg.SetLayout(layout)

	cur := app.SettingsFromConfig(*u.cfg)
	row := 0
	number := func(label string, value, max int, suffix string) *walk.NumberEdit {
		l, _ := walk.NewLabel(dlg)
		_ = l.SetText(label)
		_ = layout.SetRange(l, walk.Rectangle{X: 0, Y: row, Width: 1, Height: 1})
		ne, _ := walk.NewNumberEdit(dlg)
		_ = ne.SetDecimals(0)
		_ = ne.SetMinMaxSize(walk.Size{Width: 140}, walk.Size{})
		_ = ne.SetRange(1, float64(max))
		_ = ne.SetValue(float64(value))
		if suffix != "" {
			_ = ne.SetSuffix(" " + suffix)
		}
		_ = layout.SetRange(ne, walk.Rectangle{X: 1, Y: row, Width: 1, Height: 1})
		row++
		return ne
	}
	timeout := number("Statement timeout", cur.TimeoutSeconds, app.MaxTimeoutSeconds, "s")
	maxRows := number("Maximum rows fetched", cur.MaxRows, app.MaxRowsLimit, "")
	pageSize := number("Rows added by Load more", cur.PageSize, app.MaxRowsLimit, "")
	fontSize := number("Editor font size", cur.EditorFontSize, app.MaxFontSize, "pt")
	_ = fontSize.SetRange(app.MinFontSize, app.MaxFontSize)
	showSys, _ := walk.NewCheckBox(dlg)
	_ = showSys.SetText("Show system tables in the schema tree")
	showSys.SetChecked(cur.ShowSystemTabs)
	_ = layout.SetRange(showSys, walk.Rectangle{X: 0, Y: row, Width: 2, Height: 1})
	row++
	stop, _ := walk.NewCheckBox(dlg)
	_ = stop.SetText("Stop a script at the first error")
	stop.SetChecked(cur.StopOnError)
	_ = layout.SetRange(stop, walk.Rectangle{X: 0, Y: row, Width: 2, Height: 1})
	row++
	saveHist, _ := walk.NewCheckBox(dlg)
	_ = saveHist.SetText("Save query history between sessions (passwords masked)")
	saveHist.SetChecked(cur.SaveHistory)
	_ = layout.SetRange(saveHist, walk.Rectangle{X: 0, Y: row, Width: 2, Height: 1})
	row++

	buttons, _ := walk.NewComposite(dlg)
	bl := walk.NewHBoxLayout()
	_ = bl.SetMargins(walk.Margins{})
	_ = buttons.SetLayout(bl)
	_ = layout.SetRange(buttons, walk.Rectangle{X: 0, Y: row, Width: 2, Height: 1})
	_, _ = walk.NewHSpacer(buttons)
	_ = layout.SetColumnStretchFactor(1, 1)
	okBtn, _ := walk.NewPushButton(buttons)
	_ = okBtn.SetText("OK")
	cancelBtn, _ := walk.NewPushButton(buttons)
	_ = cancelBtn.SetText("Cancel")

	okBtn.Clicked().Attach(func() {
		s := app.Settings{
			TimeoutSeconds: int(timeout.Value()),
			MaxRows:        int(maxRows.Value()),
			PageSize:       int(pageSize.Value()),
			StopOnError:    stop.Checked(),
			SaveHistory:    saveHist.Checked(),
			EditorFontSize: int(fontSize.Value()),
			ShowSystemTabs: showSys.Checked(),
		}
		if err := s.Apply(u.cfg); err != nil {
			showErrorBox(dlg, "Settings", err)
			return
		}
		u.lastMaxRows = u.cfg.Query.DefaultMaxRows
		u.applyHistorySetting()
		u.applyEditorFont()
		u.setSchemaSnapshot(u.schemaSnapshot) // re-apply the system-tables filter
		if err := config.SaveConfig(u.cfg, u.configPath); err != nil {
			showErrorBox(dlg, "Settings", err)
			return
		}
		dlg.Accept()
	})
	cancelBtn.Clicked().Attach(func() { dlg.Cancel() })
	_ = dlg.SetDefaultButton(okBtn)
	_ = dlg.SetCancelButton(cancelBtn)
	dlg.Run()
}

// applyHistorySetting enables or disables persisted history according to
// the config (history.jsonl next to config.json).
func (u *applicationUI) applyHistorySetting() {
	if !u.cfg.App.SaveHistory {
		u.manager.SetHistoryHooks(nil, nil)
		return
	}
	store := history.New(filepath.Join(filepath.Dir(u.configPath), "history.jsonl"), 1000)
	u.manager.SetHistoryHooks(func(profileID string, e odbc.HistoryEntry) {
		_ = store.Append(history.Record{ProfileID: profileID, When: e.When, SQL: e.SQL, Status: e.Status, DurationMs: e.Duration.Milliseconds()})
	}, func(profileID string) []odbc.HistoryEntry {
		recs, err := store.Recent(profileID, 50)
		if err != nil {
			return nil
		}
		out := make([]odbc.HistoryEntry, 0, len(recs))
		for _, r := range recs {
			out = append(out, odbc.HistoryEntry{When: r.When, SQL: r.SQL, Status: r.Status, Duration: time.Duration(r.DurationMs) * time.Millisecond})
		}
		return out
	})
}

// applyEditorFont sets the configured font size on every open editor tab.
func (u *applicationUI) applyEditorFont() {
	font := monoFont(app.EditorFontSize(*u.cfg))
	if font == nil {
		return
	}
	for _, tab := range u.editorTabIndex {
		tab.Editor.SetFont(font)
	}
}
