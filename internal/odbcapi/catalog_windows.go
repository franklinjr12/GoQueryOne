//go:build windows

package odbcapi

import (
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

// LoginTimeoutSeconds is applied to the catalog connection.
var LoginTimeoutSeconds = 30

// Catalog is a native ODBC metadata connection (own environment and
// connection handles, separate from the database/sql pool).
type Catalog struct {
	env, dbc handle
}

var _ odbc.Catalog = (*Catalog)(nil)

// Open connects with SQLDriverConnect (no prompt) for catalog calls.
func Open(connStr string) (odbc.Catalog, error) {
	env, err := newEnv()
	if err != nil {
		return nil, err
	}
	dbc, err := allocHandle(sqlHandleDbc, env)
	if err != nil {
		freeHandle(sqlHandleEnv, env)
		return nil, err
	}
	if LoginTimeoutSeconds > 0 {
		_, _, _ = procSetConnectAttrW.Call(dbc, sqlAttrLoginTimeout, uintptr(LoginTimeoutSeconds), 0)
	}
	cs, err := syscall.UTF16PtrFromString(connStr)
	if err != nil {
		freeHandle(sqlHandleDbc, dbc)
		freeHandle(sqlHandleEnv, env)
		return nil, err
	}
	var outLen int16
	ret, _, _ := procDriverConnectW.Call(dbc, 0, uintptr(unsafe.Pointer(cs)), ntsSmall, 0, 0, uintptr(unsafe.Pointer(&outLen)), sqlDriverNoPrompt)
	if !ok(ret) {
		e := diag("SQLDriverConnect", sqlHandleDbc, dbc, ret)
		freeHandle(sqlHandleDbc, dbc)
		freeHandle(sqlHandleEnv, env)
		return nil, e
	}
	return &Catalog{env: env, dbc: dbc}, nil
}

// Close disconnects and frees the handles.
func (c *Catalog) Close() error {
	if c.dbc != 0 {
		_, _, _ = procDisconnect.Call(c.dbc)
		freeHandle(sqlHandleDbc, c.dbc)
		c.dbc = 0
	}
	freeHandle(sqlHandleEnv, c.env)
	c.env = 0
	return nil
}

func (c *Catalog) infoString(infoType uint16) (string, error) {
	var buf [512]uint16
	var n int16
	ret, _, _ := procGetInfoW.Call(c.dbc, uintptr(infoType), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2), uintptr(unsafe.Pointer(&n)))
	if !ok(ret) {
		return "", diag("SQLGetInfo", sqlHandleDbc, c.dbc, ret)
	}
	return syscall.UTF16ToString(buf[:]), nil
}

// Info returns DBMS/driver names and versions (SQLGetInfo).
func (c *Catalog) Info() (odbc.DBInfo, error) {
	name, err := c.infoString(sqlDBMSName)
	if err != nil {
		return odbc.DBInfo{}, err
	}
	info := odbc.DBInfo{DBMSName: name}
	info.DBMSVersion, _ = c.infoString(sqlDBMSVer)
	info.DriverName, _ = c.infoString(sqlDriverName)
	info.DriverVersion, _ = c.infoString(sqlDriverVer)
	q, _ := c.infoString(sqlIdentifierQuote)
	if strings.TrimSpace(q) != "" {
		info.IdentifierQuote = q
	}
	return info, nil
}

// Tables lists all tables, views, synonyms and system tables.
func (c *Catalog) Tables() ([]odbc.SchemaTable, error) {
	return withStmt(c.dbc, "SQLTables", func(st handle) uintptr {
		ret, _, _ := procTablesW.Call(st, 0, 0, 0, 0, 0, 0, 0, 0)
		return ret
	}, readTables)
}

// Columns lists the columns of t, or of every table when t.Name is empty.
func (c *Catalog) Columns(t odbc.SchemaTable) ([]odbc.SchemaColumn, error) {
	cat, catLen := wstr(t.Catalog)
	sch, schLen := wstr(t.Schema)
	tbl, tblLen := wstr(t.Name)
	return withStmt(c.dbc, "SQLColumns", func(st handle) uintptr {
		ret, _, _ := procColumnsW.Call(st, uintptr(unsafe.Pointer(cat)), catLen, uintptr(unsafe.Pointer(sch)), schLen, uintptr(unsafe.Pointer(tbl)), tblLen, 0, 0)
		return ret
	}, readColumns)
}

// PrimaryKeys lists the primary-key columns of t in key order.
func (c *Catalog) PrimaryKeys(t odbc.SchemaTable) ([]string, error) {
	cat, catLen := wstr(t.Catalog)
	sch, schLen := wstr(t.Schema)
	tbl, tblLen := wstr(t.Name)
	return withStmt(c.dbc, "SQLPrimaryKeys", func(st handle) uintptr {
		ret, _, _ := procPrimaryKeysW.Call(st, uintptr(unsafe.Pointer(cat)), catLen, uintptr(unsafe.Pointer(sch)), schLen, uintptr(unsafe.Pointer(tbl)), tblLen)
		return ret
	}, readPrimaryKeys)
}

// ForeignKeys lists the foreign keys defined on t.
func (c *Catalog) ForeignKeys(t odbc.SchemaTable) ([]odbc.ForeignKey, error) {
	cat, catLen := wstr(t.Catalog)
	sch, schLen := wstr(t.Schema)
	tbl, tblLen := wstr(t.Name)
	return withStmt(c.dbc, "SQLForeignKeys", func(st handle) uintptr {
		ret, _, _ := procForeignKeysW.Call(st, 0, 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(cat)), catLen, uintptr(unsafe.Pointer(sch)), schLen, uintptr(unsafe.Pointer(tbl)), tblLen)
		return ret
	}, readForeignKeys)
}

// Indexes lists the indexes of t (SQLStatistics, quick mode).
func (c *Catalog) Indexes(t odbc.SchemaTable) ([]odbc.IndexInfo, error) {
	cat, catLen := wstr(t.Catalog)
	sch, schLen := wstr(t.Schema)
	tbl, tblLen := wstr(t.Name)
	return withStmt(c.dbc, "SQLStatistics", func(st handle) uintptr {
		ret, _, _ := procStatisticsW.Call(st, uintptr(unsafe.Pointer(cat)), catLen, uintptr(unsafe.Pointer(sch)), schLen, uintptr(unsafe.Pointer(tbl)), tblLen, sqlIndexAll, sqlQuickStatistics)
		return ret
	}, readIndexes)
}

// DataSources lists the DSNs visible to this process (its bitness), user
// DSNs first, via SQLDataSources.
func DataSources() ([]DataSource, error) {
	env, err := newEnv()
	if err != nil {
		return nil, err
	}
	defer freeHandle(sqlHandleEnv, env)
	out, err := listDataSources(env, sqlFetchFirstUser, false)
	if err == nil {
		system, sysErr := listDataSources(env, sqlFetchFirstSystem, true)
		if sysErr == nil {
			return append(out, system...), nil
		}
	}
	// Driver managers without SQL_FETCH_FIRST_USER/SYSTEM (e.g. Wine):
	// list everything without the user/system distinction.
	return listDataSources(env, sqlFetchFirst, false)
}

func listDataSources(env handle, first uintptr, system bool) ([]DataSource, error) {
	var out []DataSource
	dir := first
	for {
		var name [256]uint16
		var desc [512]uint16
		var nameLen, descLen int16
		ret, _, _ := procDataSourcesW.Call(env, dir, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)), uintptr(unsafe.Pointer(&nameLen)),
			uintptr(unsafe.Pointer(&desc[0])), uintptr(len(desc)), uintptr(unsafe.Pointer(&descLen)))
		if int16(ret) == sqlNoData {
			break
		}
		if !ok(ret) {
			return out, diag("SQLDataSources", sqlHandleEnv, env, ret)
		}
		out = append(out, DataSource{Name: syscall.UTF16ToString(name[:]), Description: syscall.UTF16ToString(desc[:]), System: system})
		dir = sqlFetchNext
	}
	return out, nil
}

// Drivers lists the installed ODBC drivers for this process's bitness.
func Drivers() ([]Driver, error) {
	env, err := newEnv()
	if err != nil {
		return nil, err
	}
	defer freeHandle(sqlHandleEnv, env)
	var out []Driver
	dir := uintptr(sqlFetchFirst)
	for {
		var desc [512]uint16
		var attrs [2048]uint16
		var descLen, attrLen int16
		ret, _, _ := procDriversW.Call(env, dir, uintptr(unsafe.Pointer(&desc[0])), uintptr(len(desc)), uintptr(unsafe.Pointer(&descLen)),
			uintptr(unsafe.Pointer(&attrs[0])), uintptr(len(attrs)), uintptr(unsafe.Pointer(&attrLen)))
		if int16(ret) == sqlNoData {
			break
		}
		if !ok(ret) {
			return out, diag("SQLDrivers", sqlHandleEnv, env, ret)
		}
		n := int(attrLen)
		if n < 0 || n > len(attrs) {
			n = len(attrs)
		}
		out = append(out, Driver{Name: syscall.UTF16ToString(desc[:]), Attributes: parseDriverAttributes(string(utf16.Decode(attrs[:n])))})
		dir = sqlFetchNext
	}
	return out, nil
}
