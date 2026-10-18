package odbcapi

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

// resultSet is a forward-only ODBC result set. Columns are 1-based as in
// ODBC; ok is false for NULL (and for columns past the end, so optional
// ODBC 3 columns can be read from ODBC 2 drivers).
type resultSet interface {
	Next() (bool, error)
	Col(i int) (value string, ok bool)
}

func colInt(rs resultSet, i int) (int64, bool) {
	v, ok := rs.Col(i)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func col(rs resultSet, i int) string {
	v, _ := rs.Col(i)
	return v
}

// readTables decodes SQLTables: TABLE_CAT, TABLE_SCHEM, TABLE_NAME, TABLE_TYPE, REMARKS.
func readTables(rs resultSet) ([]odbc.SchemaTable, error) {
	var out []odbc.SchemaTable
	for {
		more, err := rs.Next()
		if err != nil {
			return out, err
		}
		if !more {
			return out, nil
		}
		out = append(out, odbc.SchemaTable{Catalog: col(rs, 1), Schema: col(rs, 2), Name: col(rs, 3), Type: col(rs, 4)})
	}
}

// readColumns decodes SQLColumns: 1 TABLE_CAT, 2 TABLE_SCHEM, 3 TABLE_NAME,
// 4 COLUMN_NAME, 5 DATA_TYPE, 6 TYPE_NAME, 7 COLUMN_SIZE, … 11 NULLABLE,
// 12 REMARKS, 13 COLUMN_DEF (ODBC 3), … 17 ORDINAL_POSITION (ODBC 3).
func readColumns(rs resultSet) ([]odbc.SchemaColumn, error) {
	type ordered struct {
		c   odbc.SchemaColumn
		pos int64
		seq int
	}
	var rows []ordered
	for {
		more, err := rs.Next()
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
		c := odbc.SchemaColumn{Catalog: col(rs, 1), Schema: col(rs, 2), Table: col(rs, 3), Name: col(rs, 4), Type: col(rs, 6)}
		if c.Type == "" {
			if dt, ok := colInt(rs, 5); ok {
				c.Type = fmt.Sprintf("SQL type %d", dt)
			}
		}
		if size, ok := colInt(rs, 7); ok {
			c.Size = size
		}
		// SQL_NO_NULLS = 0, SQL_NULLABLE = 1, SQL_NULLABLE_UNKNOWN = 2.
		nullable, ok := colInt(rs, 11)
		c.Nullable = !ok || nullable != 0
		if def, ok := rs.Col(13); ok {
			c.DefaultVal = def
		}
		pos, _ := colInt(rs, 17)
		rows = append(rows, ordered{c: c, pos: pos, seq: len(rows)})
	}
	// The spec orders by catalog, schema, table, ordinal; keep the driver's
	// table order but make sure columns follow ORDINAL_POSITION.
	sort.SliceStable(rows, func(a, b int) bool {
		ra, rb := rows[a], rows[b]
		if ra.c.Catalog != rb.c.Catalog || ra.c.Schema != rb.c.Schema || ra.c.Table != rb.c.Table {
			return ra.seq < rb.seq
		}
		if ra.pos != rb.pos && ra.pos > 0 && rb.pos > 0 {
			return ra.pos < rb.pos
		}
		return ra.seq < rb.seq
	})
	out := make([]odbc.SchemaColumn, len(rows))
	for i, r := range rows {
		out[i] = r.c
	}
	return out, nil
}

// readPrimaryKeys decodes SQLPrimaryKeys: 4 COLUMN_NAME, 5 KEY_SEQ, 6 PK_NAME.
func readPrimaryKeys(rs resultSet) ([]string, error) {
	type key struct {
		name string
		seq  int64
	}
	var keys []key
	for {
		more, err := rs.Next()
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
		seq, _ := colInt(rs, 5)
		keys = append(keys, key{col(rs, 4), seq})
	}
	sort.SliceStable(keys, func(a, b int) bool { return keys[a].seq < keys[b].seq })
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.name
	}
	return out, nil
}

// readForeignKeys decodes SQLForeignKeys (with the table as FK table):
// 2 PKTABLE_SCHEM, 3 PKTABLE_NAME, 4 PKCOLUMN_NAME, 8 FKCOLUMN_NAME,
// 9 KEY_SEQ, 12 FK_NAME.
func readForeignKeys(rs resultSet) ([]odbc.ForeignKey, error) {
	var out []odbc.ForeignKey
	for {
		more, err := rs.Next()
		if err != nil {
			return out, err
		}
		if !more {
			return out, nil
		}
		ref := col(rs, 3)
		if s := col(rs, 2); s != "" {
			ref = s + "." + ref
		}
		name := col(rs, 12)
		if name == "" {
			name = "fk_" + col(rs, 8)
		}
		out = append(out, odbc.ForeignKey{Name: name, Column: col(rs, 8), RefTable: ref, RefColumn: col(rs, 4)})
	}
}

// readIndexes decodes SQLStatistics: 4 NON_UNIQUE, 6 INDEX_NAME, 7 TYPE
// (0 = SQL_TABLE_STAT, skipped), 8 ORDINAL_POSITION, 9 COLUMN_NAME.
func readIndexes(rs resultSet) ([]odbc.IndexInfo, error) {
	var out []odbc.IndexInfo
	for {
		more, err := rs.Next()
		if err != nil {
			return out, err
		}
		if !more {
			return out, nil
		}
		if typ, ok := colInt(rs, 7); ok && typ == 0 {
			continue
		}
		name := col(rs, 6)
		if name == "" {
			continue
		}
		nonUnique, ok := colInt(rs, 4)
		out = append(out, odbc.IndexInfo{Name: name, Column: col(rs, 9), Unique: ok && nonUnique == 0})
	}
}

// DataSource is one entry from SQLDataSources.
type DataSource struct {
	Name        string
	Description string // the driver name
	System      bool
}

// Driver is one entry from SQLDrivers.
type Driver struct {
	Name       string
	Attributes map[string]string
}

// parseDriverAttributes splits SQLDrivers' attribute list
// ("Key1=Value1\x00Key2=Value2\x00\x00").
func parseDriverAttributes(raw string) map[string]string {
	attrs := map[string]string{}
	for _, kv := range strings.Split(raw, "\x00") {
		if kv == "" {
			continue
		}
		k, v, _ := strings.Cut(kv, "=")
		attrs[k] = v
	}
	return attrs
}
