package core

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0 µs"},
		{"microseconds", 850 * time.Microsecond, "850 µs"},
		{"just under a millisecond", 999 * time.Microsecond, "999 µs"},
		{"sub-10ms with decimal", 1132400 * time.Nanosecond, "1.1 ms"},
		{"ten ms rounds to int", 10 * time.Millisecond, "10 ms"},
		{"twelve ms", 12 * time.Millisecond, "12 ms"},
		{"just under a second", 999 * time.Millisecond, "999 ms"},
		{"seconds with decimal", 2400 * time.Millisecond, "2.4 s"},
		{"just under a minute", 59900 * time.Millisecond, "59.9 s"},
		{"one minute five seconds", 65 * time.Second, "1 m 05 s"},
		{"exact minute", 60 * time.Second, "1 m 00 s"},
		{"multiple minutes", 125 * time.Second, "2 m 05 s"},
		{"negative duration formatted as positive", -850 * time.Microsecond, "850 µs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatDuration(tc.d); got != tc.want {
				t.Fatalf("FormatDuration(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}
