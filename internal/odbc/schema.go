package odbc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (m *Manager) LoadSchema(sessionID, search string) (SchemaSnapshot, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return SchemaSnapshot{}, err
	}
	if session.DB == nil {
		return SchemaSnapshot{}, errors.New("session is not connected")
	}
	if snapshot, ok := session.SchemaCache.GetSchema(); ok {
		if search == "" {
			return *snapshot, nil
		}
		filtered := filterSnapshot(*snapshot, search)
		return filtered, nil
	}

	tables, err := loadTables(session)
	if err != nil {
		diag := BuildDiagnostic("schema.load", "", err)
		m.mu.Lock()
		session.LastDiag = diag
		session.State = SessionError
		m.mu.Unlock()
		return SchemaSnapshot{}, err
	}

	columnsByTable := map[string][]SchemaColumn{}
	for _, table := range tables {
		cols, colErr := loadColumns(session, table)
		if colErr != nil {
			continue
		}
		columnsByTable[detailsKey(table.Catalog, table.Schema, table.Name)] = cols
	}

	snapshot := &SchemaSnapshot{
		Tables:      tables,
		Columns:     columnsByTable,
		GeneratedAt: time.Now(),
	}
	session.SchemaCache.PutSchema(snapshot)

	if search == "" {
		return *snapshot, nil
	}
	return filterSnapshot(*snapshot, search), nil
}

func (m *Manager) RefreshSchema(sessionID, search string) (SchemaSnapshot, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return SchemaSnapshot{}, err
	}
	session.SchemaCache.Invalidate()
	return m.LoadSchema(sessionID, search)
}

func filterSnapshot(snapshot SchemaSnapshot, search string) SchemaSnapshot {
	if strings.TrimSpace(search) == "" {
		return snapshot
	}
	search = strings.ToLower(strings.TrimSpace(search))
	filteredTables := make([]SchemaTable, 0)
	filteredColumns := map[string][]SchemaColumn{}
	for _, t := range snapshot.Tables {
		key := detailsKey(t.Catalog, t.Schema, t.Name)
		cols := snapshot.Columns[key]
		if strings.Contains(strings.ToLower(t.Name), search) || strings.Contains(strings.ToLower(t.Schema), search) {
			filteredTables = append(filteredTables, t)
			filteredColumns[key] = cols
			continue
		}
		matchedCols := make([]SchemaColumn, 0)
		for _, c := range cols {
			if strings.Contains(strings.ToLower(c.Name), search) {
				matchedCols = append(matchedCols, c)
			}
		}
		if len(matchedCols) > 0 {
			filteredTables = append(filteredTables, t)
			filteredColumns[key] = matchedCols
		}
	}
	snapshot.Tables = filteredTables
	snapshot.Columns = filteredColumns
	return snapshot
}

func (m *Manager) TableDetails(sessionID, catalog, schema, table string) (TableDetails, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return TableDetails{}, err
	}
	if cached, ok := session.SchemaCache.GetTableDetails(catalog, schema, table); ok {
		return cached, nil
	}
	if session.DB == nil {
		return TableDetails{}, errors.New("session is not connected")
	}

	details := TableDetails{
		Catalog: catalog,
		Schema:  schema,
		Table:   table,
	}
	cols, colErr := loadColumns(session, SchemaTable{Catalog: catalog, Schema: schema, Name: table})
	if colErr != nil {
		details.Unsupported = append(details.Unsupported, "this driver does not support SQLColumns-compatible metadata for columns")
	} else {
		details.Columns = cols
	}

	pks, pkErr := loadPrimaryKeys(session, catalog, schema, table)
	if pkErr != nil {
		details.Unsupported = append(details.Unsupported, "this driver does not support SQLPrimaryKeys-compatible metadata")
	} else {
		details.PrimaryKeys = pks
	}

	fks, fkErr := loadForeignKeys(session, catalog, schema, table)
	if fkErr != nil {
		details.Unsupported = append(details.Unsupported, "this driver does not support SQLForeignKeys-compatible metadata")
	} else {
		details.ForeignKeys = fks
	}

	indexes, idxErr := loadIndexes(session, catalog, schema, table)
	if idxErr != nil {
		details.Unsupported = append(details.Unsupported, "this driver does not support SQLStatistics-compatible metadata")
	} else {
		details.Indexes = indexes
	}

	session.SchemaCache.PutTableDetails(details)
	return details, nil
}

func loadTables(session *Session) ([]SchemaTable, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	queries := []string{
		`SELECT TABLE_CATALOG, TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM INFORMATION_SCHEMA.TABLES ORDER BY TABLE_SCHEMA, TABLE_NAME`,
		`SELECT '' as TABLE_CATALOG, '' as TABLE_SCHEMA, name as TABLE_NAME, upper(type) as TABLE_TYPE FROM sqlite_master WHERE type in ('table','view') ORDER BY name`,
	}

	for _, q := range queries {
		rows, err := session.DB.QueryContext(ctx, q)
		if err != nil {
			continue
		}
		tables := make([]SchemaTable, 0)
		for rows.Next() {
			var t SchemaTable
			if scanErr := rows.Scan(&t.Catalog, &t.Schema, &t.Name, &t.Type); scanErr == nil {
				tables = append(tables, t)
			}
		}
		_ = rows.Close()
		if len(tables) > 0 {
			return tables, nil
		}
	}
	return nil, errors.New("failed to load schema tables")
}

func loadColumns(session *Session, table SchemaTable) ([]SchemaColumn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	queries := []struct {
		sql  string
		args []any
	}{
		{
			sql: `SELECT TABLE_CATALOG, TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, DATA_TYPE, COALESCE(CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, 0), CASE WHEN IS_NULLABLE IN ('YES','Y',1,'1',true) THEN 1 ELSE 0 END, COALESCE(COLUMN_DEFAULT, '') 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_NAME = ? AND (? = '' OR TABLE_SCHEMA = ?) 
ORDER BY ORDINAL_POSITION`,
			args: []any{table.Name, table.Schema, table.Schema},
		},
		{
			sql:  fmt.Sprintf("PRAGMA table_info('%s')", escapeLiteral(table.Name)),
			args: nil,
		},
	}

	for i, q := range queries {
		rows, err := session.DB.QueryContext(ctx, q.sql, q.args...)
		if err != nil {
			continue
		}
		cols := make([]SchemaColumn, 0)
		if i == 0 {
			for rows.Next() {
				var c SchemaColumn
				var nullableInt int
				if scanErr := rows.Scan(&c.Catalog, &c.Schema, &c.Table, &c.Name, &c.Type, &c.Size, &nullableInt, &c.DefaultVal); scanErr == nil {
					c.Nullable = nullableInt == 1
					cols = append(cols, c)
				}
			}
		} else {
			for rows.Next() {
				var cid int
				var name, typ string
				var notNull int
				var defaultVal any
				var pk int
				if scanErr := rows.Scan(&cid, &name, &typ, &notNull, &defaultVal, &pk); scanErr == nil {
					def := ""
					if defaultVal != nil {
						def = fmt.Sprintf("%v", defaultVal)
					}
					cols = append(cols, SchemaColumn{
						Catalog:    table.Catalog,
						Schema:     table.Schema,
						Table:      table.Name,
						Name:       name,
						Type:       typ,
						Nullable:   notNull == 0,
						DefaultVal: def,
					})
				}
			}
		}
		_ = rows.Close()
		if len(cols) > 0 {
			return cols, nil
		}
	}
	return nil, errors.New("failed to load columns")
}

func loadPrimaryKeys(session *Session, catalog, schema, table string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	queries := []struct {
		sql  string
		args []any
	}{
		{
			sql: `SELECT kcu.COLUMN_NAME
FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu ON tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
WHERE tc.TABLE_NAME = ? AND (? = '' OR tc.TABLE_SCHEMA = ?) AND tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
ORDER BY kcu.ORDINAL_POSITION`,
			args: []any{table, schema, schema},
		},
		{
			sql:  fmt.Sprintf("PRAGMA table_info('%s')", escapeLiteral(table)),
			args: nil,
		},
	}

	for i, q := range queries {
		rows, err := session.DB.QueryContext(ctx, q.sql, q.args...)
		if err != nil {
			continue
		}
		keys := []string{}
		if i == 0 {
			for rows.Next() {
				var col string
				if scanErr := rows.Scan(&col); scanErr == nil {
					keys = append(keys, col)
				}
			}
		} else {
			for rows.Next() {
				var cid int
				var name, typ string
				var notNull int
				var defaultVal any
				var pk int
				if scanErr := rows.Scan(&cid, &name, &typ, &notNull, &defaultVal, &pk); scanErr == nil && pk > 0 {
					keys = append(keys, name)
				}
			}
		}
		_ = rows.Close()
		if len(keys) > 0 {
			return keys, nil
		}
	}
	return nil, errors.New("failed to load primary keys")
}

func loadForeignKeys(session *Session, catalog, schema, table string) ([]ForeignKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	queries := []struct {
		sql  string
		args []any
	}{
		{
			sql: `SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
WHERE TABLE_NAME = ? AND (? = '' OR TABLE_SCHEMA = ?) AND REFERENCED_TABLE_NAME IS NOT NULL`,
			args: []any{table, schema, schema},
		},
		{
			sql:  fmt.Sprintf("PRAGMA foreign_key_list('%s')", escapeLiteral(table)),
			args: nil,
		},
	}

	for i, q := range queries {
		rows, err := session.DB.QueryContext(ctx, q.sql, q.args...)
		if err != nil {
			continue
		}
		fks := []ForeignKey{}
		if i == 0 {
			for rows.Next() {
				var fk ForeignKey
				if scanErr := rows.Scan(&fk.Name, &fk.Column, &fk.RefTable, &fk.RefColumn); scanErr == nil {
					fks = append(fks, fk)
				}
			}
		} else {
			for rows.Next() {
				var id, seq int
				var tableName, from, to, onUpdate, onDelete, match string
				if scanErr := rows.Scan(&id, &seq, &tableName, &from, &to, &onUpdate, &onDelete, &match); scanErr == nil {
					fks = append(fks, ForeignKey{
						Name:      fmt.Sprintf("fk_%d", id),
						Column:    from,
						RefTable:  tableName,
						RefColumn: to,
					})
				}
			}
		}
		_ = rows.Close()
		if len(fks) > 0 {
			return fks, nil
		}
	}
	return nil, errors.New("failed to load foreign keys")
}

func loadIndexes(session *Session, catalog, schema, table string) ([]IndexInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	queries := []struct {
		sql  string
		args []any
	}{
		{
			sql: `SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE
FROM INFORMATION_SCHEMA.STATISTICS
WHERE TABLE_NAME = ? AND (? = '' OR TABLE_SCHEMA = ?)`,
			args: []any{table, schema, schema},
		},
		{
			sql: `SELECT i.name, c.name, i.is_unique
FROM sys.indexes i
JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
JOIN sys.tables t ON i.object_id = t.object_id
WHERE t.name = ?`,
			args: []any{table},
		},
		{
			sql:  fmt.Sprintf("PRAGMA index_list('%s')", escapeLiteral(table)),
			args: nil,
		},
	}

	for i, q := range queries {
		rows, err := session.DB.QueryContext(ctx, q.sql, q.args...)
		if err != nil {
			continue
		}
		indexes := []IndexInfo{}
		if i == 0 {
			for rows.Next() {
				var name, col string
				var nonUnique int
				if scanErr := rows.Scan(&name, &col, &nonUnique); scanErr == nil {
					indexes = append(indexes, IndexInfo{Name: name, Column: col, Unique: nonUnique == 0})
				}
			}
		} else if i == 1 {
			for rows.Next() {
				var name, col string
				var unique bool
				if scanErr := rows.Scan(&name, &col, &unique); scanErr == nil {
					indexes = append(indexes, IndexInfo{Name: name, Column: col, Unique: unique})
				}
			}
		} else {
			indexNames := []string{}
			for rows.Next() {
				var seq int
				var name string
				var unique int
				var origin, partial string
				if scanErr := rows.Scan(&seq, &name, &unique, &origin, &partial); scanErr == nil {
					indexes = append(indexes, IndexInfo{Name: name, Unique: unique == 1})
					indexNames = append(indexNames, name)
				}
			}
			_ = rows.Close()
			for _, idxName := range indexNames {
				infoRows, infoErr := session.DB.QueryContext(ctx, fmt.Sprintf("PRAGMA index_info('%s')", escapeLiteral(idxName)))
				if infoErr != nil {
					continue
				}
				for infoRows.Next() {
					var seqno, cid int
					var cname string
					if scanErr := infoRows.Scan(&seqno, &cid, &cname); scanErr == nil {
						for j := range indexes {
							if indexes[j].Name == idxName {
								indexes[j].Column = cname
							}
						}
					}
				}
				_ = infoRows.Close()
			}
			if len(indexes) > 0 {
				return indexes, nil
			}
			continue
		}
		_ = rows.Close()
		if len(indexes) > 0 {
			return indexes, nil
		}
	}
	return nil, errors.New("failed to load indexes")
}

func escapeLiteral(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
