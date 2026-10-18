package odbc

import (
	"errors"
	"strings"
)

// DBInfo describes the connected database and driver (SQLGetInfo).
type DBInfo struct {
	DBMSName        string
	DBMSVersion     string
	DriverName      string
	DriverVersion   string
	IdentifierQuote string // "" when the driver does not support quoting
}

// Catalog is a native ODBC metadata connection: SQLTables, SQLColumns,
// SQLPrimaryKeys, SQLForeignKeys, SQLStatistics and SQLGetInfo. It works
// for any driver, unlike INFORMATION_SCHEMA queries. Implementations must be
// safe for use by one goroutine at a time; the Manager serialises calls.
type Catalog interface {
	Info() (DBInfo, error)
	// Tables lists tables, views, synonyms and system tables.
	Tables() ([]SchemaTable, error)
	// Columns lists the columns of one table; an empty table name lists
	// the columns of all tables in one call.
	Columns(t SchemaTable) ([]SchemaColumn, error)
	PrimaryKeys(t SchemaTable) ([]string, error)
	ForeignKeys(t SchemaTable) ([]ForeignKey, error)
	Indexes(t SchemaTable) ([]IndexInfo, error)
	Close() error
}

// CatalogOpener opens a Catalog for a full ODBC connection string.
type CatalogOpener func(connStr string) (Catalog, error)

// SetCatalogOpener enables native ODBC catalog functions (wired in cmd on
// Windows). Without it, schema metadata comes from INFORMATION_SCHEMA or
// SQLite PRAGMAs only.
func (m *Manager) SetCatalogOpener(o CatalogOpener) {
	m.mu.Lock()
	m.catalogOpener = o
	m.mu.Unlock()
}

var errNoCatalog = errors.New("native ODBC catalog not available")

// withCatalog runs fn with the session's catalog connection, opening it on
// first use. A failed open is remembered so it is not retried on every call.
func (m *Manager) withCatalog(s *Session, fn func(Catalog) error) error {
	m.mu.RLock()
	opener := m.catalogOpener
	m.mu.RUnlock()
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()
	if s.catalog == nil {
		if opener == nil || s.connStr == "" {
			return errNoCatalog
		}
		if s.catalogErr != nil {
			return s.catalogErr
		}
		c, err := opener(s.connStr)
		if err != nil {
			s.catalogErr = err
			return err
		}
		s.catalog = c
	}
	return fn(s.catalog)
}

func (s *Session) closeCatalog() {
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()
	if s.catalog != nil {
		_ = s.catalog.Close()
		s.catalog = nil
	}
}

// DBInfo returns DBMS and driver names/versions for the session, from
// SQLGetInfo when the native catalog is available. The result is cached.
func (m *Manager) DBInfo(sessionID string) (DBInfo, error) {
	s, err := m.GetSession(sessionID)
	if err != nil {
		return DBInfo{}, err
	}
	s.catalogMu.Lock()
	cached := s.dbInfo
	s.catalogMu.Unlock()
	if cached != nil {
		return *cached, nil
	}
	var info DBInfo
	err = m.withCatalog(s, func(c Catalog) error {
		var e error
		info, e = c.Info()
		return e
	})
	if err != nil {
		return DBInfo{}, err
	}
	s.catalogMu.Lock()
	s.dbInfo = &info
	s.catalogMu.Unlock()
	m.mu.Lock()
	if s.ConnectedInfo == nil {
		s.ConnectedInfo = map[string]string{}
	}
	s.ConnectedInfo["dbms"] = strings.TrimSpace(info.DBMSName + " " + info.DBMSVersion)
	s.ConnectedInfo["odbcDriver"] = strings.TrimSpace(info.DriverName + " " + info.DriverVersion)
	m.mu.Unlock()
	return info, nil
}

// catalogSchema lists tables through the native catalog and, in the same
// pass, all columns (one SQLColumns call instead of one query per table).
// ok is false when the catalog is unavailable or failed, so the caller falls
// back to the SQL-based loaders.
func (m *Manager) catalogSchema(s *Session) (tables []SchemaTable, columns map[string][]SchemaColumn, ok bool) {
	err := m.withCatalog(s, func(c Catalog) error {
		var e error
		tables, e = c.Tables()
		if e != nil {
			return e
		}
		columns = map[string][]SchemaColumn{}
		all, colErr := c.Columns(SchemaTable{})
		if colErr == nil && len(all) > 0 {
			for _, col := range all {
				k := detailsKey(col.Catalog, col.Schema, col.Table)
				columns[k] = append(columns[k], col)
			}
			return nil
		}
		// Some drivers reject a wildcard table name: ask per table.
		for _, t := range tables {
			if cols, e := c.Columns(t); e == nil {
				columns[detailsKey(t.Catalog, t.Schema, t.Name)] = cols
			}
		}
		return nil
	})
	if err != nil || len(tables) == 0 {
		return nil, nil, false
	}
	return tables, columns, true
}
