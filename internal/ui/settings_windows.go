//go:build windows

package ui

import (
	"github.com/lxn/walk"

	"github.com/franklinjr12/GoQueryOne/internal/app"
	"github.com/franklinjr12/GoQueryOne/internal/config"
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
	_ = dlg.SetMinMaxSize(walk.Size{Width: 420, Height: 220}, walk.Size{})
	_ = dlg.SetSize(walk.Size{Width: 420, Height: 220})
	layout := walk.NewGridLayout()
	_ = layout.SetMargins(walk.Margins{HNear: 10, VNear: 10, HFar: 10, VFar: 10})
	_ = layout.SetSpacing(8)
	_ = dlg.SetLayout(layout)

	cur := app.SettingsFromConfig(u.cfg.Query)
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
	stop, _ := walk.NewCheckBox(dlg)
	_ = stop.SetText("Stop a script at the first error")
	stop.SetChecked(cur.StopOnError)
	_ = layout.SetRange(stop, walk.Rectangle{X: 0, Y: row, Width: 2, Height: 1})
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
		}
		if err := s.Apply(&u.cfg.Query); err != nil {
			showErrorBox(dlg, "Settings", err)
			return
		}
		u.lastMaxRows = u.cfg.Query.DefaultMaxRows
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
