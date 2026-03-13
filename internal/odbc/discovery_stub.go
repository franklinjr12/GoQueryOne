//go:build !windows

package odbc

import "errors"

func ListDSNs() ([]DSNEntry, error) {
	return nil, errors.New("ODBC discovery is supported on Windows only")
}

func ListDrivers() ([]DriverEntry, error) {
	return nil, errors.New("ODBC discovery is supported on Windows only")
}
