package app

// Rect is a window or monitor rectangle in physical pixels.
type Rect struct {
	X, Y, Width, Height int
}

// Empty reports whether the rectangle has no area.
func (r Rect) Empty() bool { return r.Width <= 0 || r.Height <= 0 }

// PlaceWindow decides where the main window opens. saved is the rectangle
// stored in config.json (may be empty on first run, or point at a monitor
// that is gone), current is what the window got by default, and work is the
// work area of the monitor nearest to saved (taskbar excluded).
//
// The result always lies fully inside work: oversized windows shrink, and
// windows that would be partly or fully off-screen are moved back, so the
// title bar can always be grabbed.
func PlaceWindow(saved, current, work Rect) Rect {
	r := saved
	if r.Empty() {
		r = current
	}
	if work.Empty() {
		return r
	}
	if r.Empty() {
		r = Rect{Width: work.Width * 3 / 4, Height: work.Height * 3 / 4}
		r.X = work.X + (work.Width-r.Width)/2
		r.Y = work.Y + (work.Height-r.Height)/2
		return r
	}
	if r.Width > work.Width {
		r.Width = work.Width
	}
	if r.Height > work.Height {
		r.Height = work.Height
	}
	if r.X < work.X {
		r.X = work.X
	}
	if r.Y < work.Y {
		r.Y = work.Y
	}
	if r.X+r.Width > work.X+work.Width {
		r.X = work.X + work.Width - r.Width
	}
	if r.Y+r.Height > work.Y+work.Height {
		r.Y = work.Y + work.Height - r.Height
	}
	return r
}
