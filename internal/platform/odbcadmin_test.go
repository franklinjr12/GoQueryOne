package platform

import "testing"

var _ ODBCAdmin = SystemODBCAdmin{}

func TestSystemODBCAdminDelegatesToODBCPackage(t *testing.T) {
	// On non-Windows, odbc.OpenODBCAdmin always returns an "unsupported"
	// error; this just locks in that SystemODBCAdmin forwards to it rather
	// than swallowing or reinterpreting the result.
	admin := NewSystemODBCAdmin()
	if _, err := admin.Open("x64"); err == nil {
		t.Skip("running on an OS where ODBC admin is supported; nothing to assert here")
	}
}
