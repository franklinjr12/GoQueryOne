package core

import (
	"fmt"
	"math"
	"time"
)

// FormatDuration renders d the way the UI shows it in messages, status and
// history: "850 µs", "1.1 ms", "12 ms", "2.4 s", "1 m 05 s".
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Millisecond:
		us := int64(math.Round(float64(d) / float64(time.Microsecond)))
		return fmt.Sprintf("%d µs", us)
	case d < time.Second:
		ms := float64(d) / float64(time.Millisecond)
		if ms < 10 {
			return fmt.Sprintf("%.1f ms", ms)
		}
		return fmt.Sprintf("%d ms", int64(math.Round(ms)))
	case d < time.Minute:
		s := float64(d) / float64(time.Second)
		return fmt.Sprintf("%.1f s", s)
	default:
		m := int64(d / time.Minute)
		s := int64(math.Round(float64(d%time.Minute) / float64(time.Second)))
		if s >= 60 {
			m++
			s -= 60
		}
		return fmt.Sprintf("%d m %02d s", m, s)
	}
}
