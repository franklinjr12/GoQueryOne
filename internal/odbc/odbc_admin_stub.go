//go:build !windows

package odbc

import "errors"

func OpenODBCAdmin(targetArch string) (string, error) {
	return "", errors.New("ODBC admin launcher is supported on Windows only")
}
