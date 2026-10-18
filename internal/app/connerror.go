package app

import (
	"regexp"
	"strings"
)

var noDetailsRe = regexp.MustCompile(`^\s*SQL\w+:\s*$`)

var sqlStateRe = regexp.MustCompile(`\b(IM0\d\d|HYT00|HYT01|08001|08S01|08004|28000)\b`)

// ConnectErrorHint returns a one-line, actionable explanation for common
// ODBC connect failures (by SQLSTATE or driver-manager message), or "" when
// there is nothing useful to add. arch is the running build: "x64" or "x86".
func ConnectErrorHint(msg, arch string) string {
	other := "x86"
	otherBits, bits := "32", "64"
	if arch == "x86" {
		other, otherBits, bits = "x64", "64", "32"
	}
	if strings.Contains(msg, "SQLSetEnvUIntPtrAttr") || (strings.Contains(msg, "SQLAllocHandle") && strings.Contains(msg, "ENV")) {
		return "The ODBC Driver Manager (odbc32.dll) could not be initialised. Repair the Windows ODBC components."
	}
	if noDetailsRe.MatchString(msg) {
		return "No details from the driver manager. Check that the DSN/driver exists in ODBC Administrator (" + arch + ")."
	}
	switch sqlStateRe.FindString(msg) {
	case "IM014":
		return "Bitness mismatch: the DSN uses a " + otherBits + "-bit driver. Use GoQueryOne-" + other + ".exe or a " + bits + "-bit DSN."
	case "IM002":
		return "DSN/driver not found for " + bits + "-bit ODBC. Check ODBC Administrator (" + arch + "), or use GoQueryOne-" + other + ".exe."
	case "IM003":
		return "The driver could not be loaded (missing DLL or wrong bitness). Reinstall the " + bits + "-bit driver."
	case "IM006", "IM004":
		return "The driver failed to initialise; it may be damaged or not " + bits + "-bit."
	case "28000":
		return "Login rejected. Check the username and password (or whether Windows authentication is expected)."
	case "08001", "08S01", "08004":
		return "Server not reachable. Check the host, port, network/VPN and that the server is running."
	case "HYT00", "HYT01":
		return "Login timed out. Check the server address and network, or raise the login timeout."
	}
	return ""
}

// OneLine joins a multi-line driver message (one line per diagnostic
// record) for single-line labels.
func OneLine(msg string) string {
	msg = strings.ReplaceAll(msg, "\r\n", "\n")
	parts := strings.Split(msg, "\n")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " | ")
}

// WithConnectHint appends the ConnectErrorHint (if any) to msg, separated by
// a blank line, for message boxes.
func WithConnectHint(msg, arch string) string {
	if h := ConnectErrorHint(msg, arch); h != "" {
		return msg + "\r\n\r\n" + h
	}
	return msg
}
