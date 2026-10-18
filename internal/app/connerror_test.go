package app

import (
	"strings"
	"testing"
)

func TestConnectErrorHint(t *testing.T) {
	cases := []struct {
		msg, arch, want string
	}{
		{"SQLDriverConnect: {IM014} [Microsoft][ODBC Driver Manager] The specified DSN contains an architecture mismatch", "x64", "GoQueryOne-x86.exe"},
		{"SQLDriverConnect: {IM014} mismatch", "x86", "GoQueryOne-x64.exe"},
		{"SQLDriverConnect: {IM002} [Microsoft][ODBC Driver Manager] Data source name not found", "x64", "64-bit ODBC"},
		{"{IM003} Specified driver could not be loaded", "x86", "32-bit driver"},
		{"{28000} Login failed for user 'sa'", "x64", "username and password"},
		{"{08001} TCP Provider: No such host is known", "x64", "not reachable"},
		{"{HYT00} Login timeout expired", "x64", "timed out"},
		{"SQLSetEnvUIntPtrAttr: {HYC00} Optional feature not implemented", "x64", "Driver Manager (odbc32.dll)"},
		{"{IM006} SQLSetConnectAttr failed", "x64", "failed to initialise"},
		{"SQLDriverConnect: ", "x64", "No details"},
	}
	for _, c := range cases {
		if got := ConnectErrorHint(c.msg, c.arch); !strings.Contains(got, c.want) {
			t.Errorf("ConnectErrorHint(%q, %s) = %q, want it to contain %q", c.msg, c.arch, got, c.want)
		}
	}
	if got := ConnectErrorHint("{42S02} no such table: x", "x64"); got != "" {
		t.Errorf("unexpected hint %q", got)
	}
	if got := ConnectErrorHint("XIM0021 not a state", "x64"); got != "" {
		t.Errorf("word boundary: %q", got)
	}
}

func TestWithConnectHint(t *testing.T) {
	if got := WithConnectHint("plain", "x64"); got != "plain" {
		t.Fatalf("got %q", got)
	}
	got := WithConnectHint("{28000} denied", "x64")
	if !strings.HasPrefix(got, "{28000} denied\r\n\r\n") || !strings.Contains(got, "password") {
		t.Fatalf("got %q", got)
	}
}

func TestOneLine(t *testing.T) {
	if got := OneLine("SQLDriverConnect: {01000} a\r\n{IM002} b\n\n"); got != "SQLDriverConnect: {01000} a | {IM002} b" {
		t.Fatalf("got %q", got)
	}
}
