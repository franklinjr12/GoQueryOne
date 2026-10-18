//go:build integration && !windows

package odbcdriver

import (
	_ "github.com/alexbrainman/odbc" // needs cgo + unixODBC headers
)

// DriverName is the database/sql driver name registered by this package.
const DriverName = "odbc"
