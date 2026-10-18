package platform

import "github.com/franklinjr12/GoQueryOne/internal/odbc"

// SystemODBCAdmin adapts odbc.OpenODBCAdmin to the ODBCAdmin interface.
// odbc.OpenODBCAdmin already has a real Windows implementation and a stub
// that returns an "unsupported" error on every other OS, so this adapter
// needs no build tag of its own.
type SystemODBCAdmin struct{}

func NewSystemODBCAdmin() SystemODBCAdmin {
	return SystemODBCAdmin{}
}

func (SystemODBCAdmin) Open(targetArch string) (string, error) {
	return odbc.OpenODBCAdmin(targetArch)
}
