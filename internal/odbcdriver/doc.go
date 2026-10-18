// Package odbcdriver registers the ODBC database/sql driver on Windows.
// On other platforms it is intentionally empty; ODBC integration tests on
// Linux use the `integration` build tag (see driver_integration.go).
package odbcdriver
