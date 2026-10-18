package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestStringContainsFields(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, Date
	defer func() { Version, Commit, Date = oldV, oldC, oldD }()
	Version, Commit, Date = "1.2.3", "abc1234", "2026-10-18T00:00:00Z"
	s := String()
	for _, want := range []string{"GoQueryOne 1.2.3", "abc1234", "2026-10-18T00:00:00Z", Arch()} {
		if !strings.Contains(s, want) {
			t.Fatalf("String() = %q, missing %q", s, want)
		}
	}
}

func TestArch(t *testing.T) {
	got := Arch()
	switch runtime.GOARCH {
	case "amd64":
		if got != "x64" {
			t.Fatalf("got %q", got)
		}
	case "386":
		if got != "x86" {
			t.Fatalf("got %q", got)
		}
	default:
		if got != runtime.GOARCH {
			t.Fatalf("got %q", got)
		}
	}
}
