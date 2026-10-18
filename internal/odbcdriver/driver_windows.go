//go:build windows

// Package odbcdriver registers the real ODBC database/sql driver ("odbc").
// It is imported only by the Windows entry point so that every other package
// builds and tests on any OS with CGO disabled and no unixODBC installed.
package odbcdriver

import (
	_ "github.com/alexbrainman/odbc" // registers the "odbc" driver
)

// DriverName is the database/sql driver name registered by this package.
const DriverName = "odbc"
