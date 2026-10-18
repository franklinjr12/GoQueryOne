package app

import "testing"

func TestPlaceWindow(t *testing.T) {
	work := Rect{0, 0, 1920, 1040}
	cases := []struct {
		name                 string
		saved, current, want Rect
		work                 Rect
	}{
		{"saved inside is kept", Rect{100, 50, 1260, 760}, Rect{}, Rect{100, 50, 1260, 760}, work},
		{"first run keeps the default", Rect{}, Rect{10, 10, 1260, 760}, Rect{10, 10, 1260, 760}, work},
		{"nothing known: centered 3/4", Rect{}, Rect{}, Rect{240, 130, 1440, 780}, work},
		{"monitor gone (far right) moves back", Rect{3000, 100, 1260, 760}, Rect{}, Rect{660, 100, 1260, 760}, work},
		{"negative position (left monitor gone)", Rect{-1500, -20, 1260, 760}, Rect{}, Rect{0, 0, 1260, 760}, work},
		{"too big shrinks to the work area", Rect{0, 0, 2560, 1440}, Rect{}, Rect{0, 0, 1920, 1040}, work},
		{"below the taskbar", Rect{100, 900, 800, 600}, Rect{}, Rect{100, 440, 800, 600}, work},
		{"secondary monitor work area", Rect{2000, 100, 800, 600}, Rect{}, Rect{2000, 100, 800, 600}, Rect{1920, 0, 1280, 1000}},
		{"unknown work area returns saved", Rect{5, 5, 10, 10}, Rect{}, Rect{5, 5, 10, 10}, Rect{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlaceWindow(tc.saved, tc.current, tc.work); got != tc.want {
				t.Fatalf("PlaceWindow = %+v, want %+v", got, tc.want)
			}
		})
	}
}
