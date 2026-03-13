//go:build windows

package odbc

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func OpenODBCAdmin(targetArch string) (string, error) {
	winDir := os.Getenv("WINDIR")
	if strings.TrimSpace(winDir) == "" {
		return "", errors.New("WINDIR is not set")
	}
	var exePath string
	switch strings.ToLower(strings.TrimSpace(targetArch)) {
	case "x86", "32", "32-bit":
		exePath = filepath.Join(winDir, "SysWOW64", "odbcad32.exe")
	default:
		exePath = filepath.Join(winDir, "System32", "odbcad32.exe")
	}
	cmd := exec.Command(exePath)
	if err := cmd.Start(); err != nil {
		return exePath, err
	}
	return exePath, nil
}
