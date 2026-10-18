//go:build windows

package ui

import (
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"

	"github.com/franklinjr12/GoQueryOne/internal/app"
)

// placeMainWindow restores the saved window rectangle (physical pixels) and
// keeps it inside the work area of the nearest monitor, so a window saved on
// a monitor that is no longer attached does not open off-screen.
func (u *applicationUI) placeMainWindow() {
	w := u.cfg.App.Window
	saved := app.Rect{X: int(w.X), Y: int(w.Y), Width: int(w.Width), Height: int(w.Height)}
	cur := u.mainWindow.BoundsPixels()
	current := app.Rect{X: cur.X, Y: cur.Y, Width: cur.Width, Height: cur.Height}
	if !saved.Empty() {
		// Move first so MonitorFromWindow picks the monitor nearest to the
		// saved position.
		_ = u.mainWindow.SetBoundsPixels(walk.Rectangle{X: saved.X, Y: saved.Y, Width: saved.Width, Height: saved.Height})
	}
	work := monitorWorkArea(u.mainWindow.Handle())
	r := app.PlaceWindow(saved, current, work)
	_ = u.mainWindow.SetBoundsPixels(walk.Rectangle{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height})
}

// showMainWindow shows the window, maximized if it was maximized on exit.
func (u *applicationUI) showMainWindow() {
	u.mainWindow.Show()
	if u.cfg.App.Window.Maximized {
		win.ShowWindow(u.mainWindow.Handle(), win.SW_MAXIMIZE)
	}
}

// saveMainWindowPlacement stores the restored (non-maximized) rectangle and
// the maximized flag in the config (saved on exit).
func (u *applicationUI) saveMainWindowPlacement() {
	hwnd := u.mainWindow.Handle()
	var r walk.Rectangle
	maximized := win.IsZoomed(hwnd)
	var wp win.WINDOWPLACEMENT
	wp.Length = uint32(unsafe.Sizeof(wp))
	if (maximized || win.IsIconic(hwnd)) && win.GetWindowPlacement(hwnd, &wp) {
		n := wp.RcNormalPosition
		r = walk.Rectangle{X: int(n.Left), Y: int(n.Top), Width: int(n.Right - n.Left), Height: int(n.Bottom - n.Top)}
	} else {
		r = u.mainWindow.BoundsPixels()
	}
	if r.Width <= 0 || r.Height <= 0 {
		return
	}
	u.cfg.App.Window.X = float32(r.X)
	u.cfg.App.Window.Y = float32(r.Y)
	u.cfg.App.Window.Width = float32(r.Width)
	u.cfg.App.Window.Height = float32(r.Height)
	u.cfg.App.Window.Maximized = maximized
}

func monitorWorkArea(hwnd win.HWND) app.Rect {
	mon := win.MonitorFromWindow(hwnd, win.MONITOR_DEFAULTTONEAREST)
	if mon == 0 {
		return app.Rect{}
	}
	var mi win.MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if !win.GetMonitorInfo(mon, &mi) {
		return app.Rect{}
	}
	w := mi.RcWork
	return app.Rect{X: int(w.Left), Y: int(w.Top), Width: int(w.Right - w.Left), Height: int(w.Bottom - w.Top)}
}
