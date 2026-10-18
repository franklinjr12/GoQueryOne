//go:build windows

package odbcapi

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	odbc32 = syscall.NewLazyDLL("odbc32.dll")

	procAllocHandle     = odbc32.NewProc("SQLAllocHandle")
	procFreeHandle      = odbc32.NewProc("SQLFreeHandle")
	procSetEnvAttr      = odbc32.NewProc("SQLSetEnvAttr")
	procSetConnectAttrW = odbc32.NewProc("SQLSetConnectAttrW")
	procDriverConnectW  = odbc32.NewProc("SQLDriverConnectW")
	procDisconnect      = odbc32.NewProc("SQLDisconnect")
	procGetInfoW        = odbc32.NewProc("SQLGetInfoW")
	procTablesW         = odbc32.NewProc("SQLTablesW")
	procColumnsW        = odbc32.NewProc("SQLColumnsW")
	procPrimaryKeysW    = odbc32.NewProc("SQLPrimaryKeysW")
	procForeignKeysW    = odbc32.NewProc("SQLForeignKeysW")
	procStatisticsW     = odbc32.NewProc("SQLStatisticsW")
	procFetch           = odbc32.NewProc("SQLFetch")
	procGetData         = odbc32.NewProc("SQLGetData")
	procGetDiagRecW     = odbc32.NewProc("SQLGetDiagRecW")
	procDataSourcesW    = odbc32.NewProc("SQLDataSourcesW")
	procDriversW        = odbc32.NewProc("SQLDriversW")
	procNumResultCols   = odbc32.NewProc("SQLNumResultCols")
)

const (
	sqlHandleEnv  = 1
	sqlHandleDbc  = 2
	sqlHandleStmt = 3

	sqlSuccess         = 0
	sqlSuccessWithInfo = 1
	sqlNoData          = 100
	sqlNullData        = -1
	sqlNoTotal         = -4
	sqlNTS             = -3

	sqlAttrODBCVersion  = 200
	sqlOVODBC3          = 3
	sqlAttrLoginTimeout = 103
	sqlDriverNoPrompt   = 0

	sqlFetchNext        = 1
	sqlFetchFirst       = 2
	sqlFetchFirstUser   = 31
	sqlFetchFirstSystem = 32

	sqlDriverName       = 6
	sqlDriverVer        = 7
	sqlDBMSName         = 17
	sqlDBMSVer          = 18
	sqlIdentifierQuote  = 29
	sqlIndexAll         = 1
	sqlQuickStatistics  = 0
	maxDiagMessageChars = 1024
)

type handle = uintptr

// Negative SQLSMALLINT arguments: only the low 16 bits are read by the callee.
var (
	ntsSmall  = uintptr(uint16(0xFFFD)) // SQL_NTS (-3)
	cWCharArg = uintptr(uint16(0xFFF8)) // SQL_C_WCHAR (-8)
)

func ok(ret uintptr) bool {
	r := int16(ret)
	return r == sqlSuccess || r == sqlSuccessWithInfo
}

// Error is an ODBC diagnostic record.
type Error struct {
	Func    string
	State   string
	Native  int32
	Message string
}

func (e *Error) Error() string {
	if e.State == "" {
		return e.Func + " failed"
	}
	return fmt.Sprintf("%s: [%s] %s", e.Func, e.State, e.Message)
}

func diag(fn string, handleType int16, h handle, ret uintptr) error {
	if int16(ret) == -2 { // SQL_INVALID_HANDLE
		return &Error{Func: fn, State: "HY000", Message: "invalid handle"}
	}
	var state [6]uint16
	var native int32
	var msg [maxDiagMessageChars]uint16
	var msgLen int16
	r, _, _ := procGetDiagRecW.Call(uintptr(handleType), h, 1,
		uintptr(unsafe.Pointer(&state[0])), uintptr(unsafe.Pointer(&native)),
		uintptr(unsafe.Pointer(&msg[0])), uintptr(len(msg)), uintptr(unsafe.Pointer(&msgLen)))
	if !ok(r) {
		return &Error{Func: fn}
	}
	return &Error{Func: fn, State: syscall.UTF16ToString(state[:5]), Native: native, Message: syscall.UTF16ToString(msg[:])}
}

func allocHandle(handleType int16, parent handle) (handle, error) {
	var h handle
	r, _, _ := procAllocHandle.Call(uintptr(handleType), parent, uintptr(unsafe.Pointer(&h)))
	if !ok(r) {
		if parent != 0 {
			parentType := int16(sqlHandleEnv)
			if handleType == sqlHandleStmt {
				parentType = sqlHandleDbc
			}
			return 0, diag("SQLAllocHandle", parentType, parent, r)
		}
		return 0, &Error{Func: "SQLAllocHandle", Message: "cannot allocate ODBC environment (is odbc32.dll available?)"}
	}
	return h, nil
}

func freeHandle(handleType int16, h handle) {
	if h != 0 {
		_, _, _ = procFreeHandle.Call(uintptr(handleType), h)
	}
}

func newEnv() (handle, error) {
	if err := odbc32.Load(); err != nil {
		return 0, err
	}
	env, err := allocHandle(sqlHandleEnv, 0)
	if err != nil {
		return 0, err
	}
	r, _, _ := procSetEnvAttr.Call(env, sqlAttrODBCVersion, sqlOVODBC3, 0)
	if !ok(r) {
		e := diag("SQLSetEnvAttr", sqlHandleEnv, env, r)
		freeHandle(sqlHandleEnv, env)
		return 0, e
	}
	return env, nil
}

// wstr returns a pointer/length pair for an optional search argument; an
// empty string becomes NULL (= "all").
func wstr(v string) (uintptr, uintptr) {
	if v == "" {
		return 0, 0
	}
	p, err := syscall.UTF16PtrFromString(v)
	if err != nil {
		return 0, 0
	}
	return uintptr(unsafe.Pointer(p)), ntsSmall
}

// stmtRS reads a statement's result set as strings through SQLGetData.
// Each row is read completely, in column order, when it is fetched:
// SQLGetData may only be called once per column and, for many drivers, only
// in ascending column order.
type stmtRS struct {
	stmt handle
	fn   string
	cols int
	buf  []uint16
	row  []*string
}

func (r *stmtRS) Next() (bool, error) {
	ret, _, _ := procFetch.Call(r.stmt)
	switch int16(ret) {
	case sqlSuccess, sqlSuccessWithInfo:
	case sqlNoData:
		return false, nil
	default:
		return false, diag("SQLFetch("+r.fn+")", sqlHandleStmt, r.stmt, ret)
	}
	r.row = r.row[:0]
	for i := 1; i <= r.cols; i++ {
		v, ok := r.getData(i)
		if ok {
			r.row = append(r.row, &v)
		} else {
			r.row = append(r.row, nil)
		}
	}
	return true, nil
}

func (r *stmtRS) Col(i int) (string, bool) {
	if i < 1 || i > len(r.row) || r.row[i-1] == nil {
		return "", false
	}
	return *r.row[i-1], true
}

func (r *stmtRS) getData(i int) (string, bool) {
	var out []uint16
	for {
		var ind int // SQLLEN: pointer-sized on both 386 and amd64
		ret, _, _ := procGetData.Call(r.stmt, uintptr(i), cWCharArg,
			uintptr(unsafe.Pointer(&r.buf[0])), uintptr(len(r.buf)*2), uintptr(unsafe.Pointer(&ind)))
		code := int16(ret)
		if code == sqlNoData {
			break
		}
		if code != sqlSuccess && code != sqlSuccessWithInfo {
			return "", false
		}
		if ind == sqlNullData {
			return "", false
		}
		room := len(r.buf) - 1 // characters that fit next to the terminator
		if ind != sqlNoTotal && ind >= 0 && ind/2 <= room {
			out = append(out, r.buf[:ind/2]...)
			break
		}
		// Truncated (01004): the buffer is full; read the next chunk.
		out = append(out, r.buf[:room]...)
		if code == sqlSuccess {
			break
		}
	}
	return syscall.UTF16ToString(append(out, 0)), true
}

// withStmt allocates a statement, runs call (one catalog function), decodes
// its result set and frees the statement.
func withStmt[T any](dbc handle, fn string, call func(stmt handle) uintptr, decode func(resultSet) (T, error)) (T, error) {
	var zero T
	stmt, err := allocHandle(sqlHandleStmt, dbc)
	if err != nil {
		return zero, err
	}
	defer freeHandle(sqlHandleStmt, stmt)
	if ret := call(stmt); !ok(ret) {
		return zero, diag(fn, sqlHandleStmt, stmt, ret)
	}
	var cols int16
	if ret, _, _ := procNumResultCols.Call(stmt, uintptr(unsafe.Pointer(&cols))); !ok(ret) {
		return zero, diag("SQLNumResultCols", sqlHandleStmt, stmt, ret)
	}
	return decode(&stmtRS{stmt: stmt, fn: fn, cols: int(cols), buf: make([]uint16, 2048)})
}
