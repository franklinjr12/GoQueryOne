package odbc

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type fakeCatalog struct {
	opens, closes int
	tables        []SchemaTable
	tablesErr     error
	cols          []SchemaColumn
	wildcardErr   error
	pks           []string
	fks           []ForeignKey
	idx           []IndexInfo
	detailErr     error
	columnCalls   []string
}

func (f *fakeCatalog) Info() (DBInfo, error) {
	return DBInfo{DBMSName: "FakeDB", DBMSVersion: "1.2", DriverName: "fake.dll", DriverVersion: "03.80", IdentifierQuote: `"`}, nil
}
func (f *fakeCatalog) Tables() ([]SchemaTable, error) { return f.tables, f.tablesErr }
func (f *fakeCatalog) Columns(t SchemaTable) ([]SchemaColumn, error) {
	f.columnCalls = append(f.columnCalls, t.Name)
	if t.Name == "" {
		if f.wildcardErr != nil {
			return nil, f.wildcardErr
		}
		return f.cols, nil
	}
	var out []SchemaColumn
	for _, c := range f.cols {
		if c.Table == t.Name {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeCatalog) PrimaryKeys(SchemaTable) ([]string, error)     { return f.pks, f.detailErr }
func (f *fakeCatalog) ForeignKeys(SchemaTable) ([]ForeignKey, error) { return f.fks, f.detailErr }
func (f *fakeCatalog) Indexes(SchemaTable) ([]IndexInfo, error)      { return f.idx, f.detailErr }
func (f *fakeCatalog) Close() error                                  { f.closes++; return nil }

func catalogManager(t *testing.T, fc *fakeCatalog, openErr error) (*Manager, *Session) {
	t.Helper()
	m, s := newSQLiteManager(t)
	m.SetCatalogOpener(func(connStr string) (Catalog, error) {
		fc.opens++
		if connStr == "" {
			t.Fatal("empty connection string")
		}
		if openErr != nil {
			return nil, openErr
		}
		return fc, nil
	})
	return m, s
}

func sampleCatalog() *fakeCatalog {
	return &fakeCatalog{
		tables: []SchemaTable{{Schema: "dbo", Name: "orders", Type: "TABLE"}, {Schema: "dbo", Name: "v", Type: "VIEW"}},
		cols: []SchemaColumn{
			{Schema: "dbo", Table: "orders", Name: "id", Type: "int"},
			{Schema: "dbo", Table: "orders", Name: "total", Type: "decimal", Nullable: true},
			{Schema: "dbo", Table: "v", Name: "x", Type: "int"},
		},
		pks: []string{"id"},
		fks: []ForeignKey{{Name: "fk", Column: "customer_id", RefTable: "customers", RefColumn: "id"}},
		idx: []IndexInfo{{Name: "ix", Column: "total"}},
	}
}

func TestLoadSchemaUsesNativeCatalog(t *testing.T) {
	fc := sampleCatalog()
	m, s := catalogManager(t, fc, nil)
	snap, err := m.LoadSchema(s.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tables) != 2 || snap.Tables[0].Name != "orders" {
		t.Fatalf("tables = %+v", snap.Tables)
	}
	if got := snap.ColumnsOf(snap.Tables[0]); len(got) != 2 {
		t.Fatalf("orders columns = %+v", got)
	}
	if strings.Join(fc.columnCalls, ",") != "" {
		t.Fatalf("expected one wildcard SQLColumns call, got %q", fc.columnCalls)
	}
	// Cached: no reopen, no new calls.
	if _, err := m.LoadSchema(s.ID, "tot"); err != nil || fc.opens != 1 || len(fc.columnCalls) != 1 {
		t.Fatalf("second load: err=%v opens=%d calls=%d", err, fc.opens, len(fc.columnCalls))
	}
}

func TestLoadSchemaCatalogPerTableColumnsWhenWildcardFails(t *testing.T) {
	fc := sampleCatalog()
	fc.wildcardErr = errors.New("HY090 invalid string length")
	m, s := catalogManager(t, fc, nil)
	snap, err := m.LoadSchema(s.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(fc.columnCalls, ","); got != ",orders,v" {
		t.Fatalf("column calls = %q", got)
	}
	if len(snap.ColumnsOf(snap.Tables[1])) != 1 {
		t.Fatalf("v columns missing: %+v", snap.Columns)
	}
}

func TestLoadSchemaFallsBackWhenCatalogFails(t *testing.T) {
	fc := sampleCatalog()
	m, s := catalogManager(t, fc, errors.New("IM002 data source not found"))
	mustExec(t, m, s.ID, "CREATE TABLE real_table (a INTEGER)")
	snap, err := m.LoadSchema(s.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tables) != 1 || snap.Tables[0].Name != "real_table" {
		t.Fatalf("fallback tables = %+v", snap.Tables)
	}
	// The failed open is remembered.
	_, _ = m.TableDetails(s.ID, "", "", "real_table")
	if fc.opens != 1 {
		t.Fatalf("catalog opened %d times, want 1", fc.opens)
	}
}

func TestLoadSchemaFallsBackWhenCatalogHasNoTables(t *testing.T) {
	fc := sampleCatalog()
	fc.tablesErr = errors.New("not supported")
	m, s := catalogManager(t, fc, nil)
	mustExec(t, m, s.ID, "CREATE TABLE t2 (a INTEGER)")
	snap, err := m.LoadSchema(s.ID, "")
	if err != nil || len(snap.Tables) != 1 || snap.Tables[0].Name != "t2" {
		t.Fatalf("snap=%+v err=%v", snap.Tables, err)
	}
}

func TestTableDetailsUsesCatalogAndFallsBackPerPiece(t *testing.T) {
	fc := sampleCatalog()
	m, s := catalogManager(t, fc, nil)
	d, err := m.TableDetails(s.ID, "", "dbo", "orders")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Columns) != 2 || len(d.PrimaryKeys) != 1 || len(d.ForeignKeys) != 1 || len(d.Indexes) != 1 || len(d.Unsupported) != 0 {
		t.Fatalf("details = %+v", d)
	}

	fc2 := sampleCatalog()
	fc2.detailErr = errors.New("IM001 driver does not support this function")
	m2, s2 := catalogManager(t, fc2, nil)
	mustExec(t, m2, s2.ID, "CREATE TABLE orders (id INTEGER PRIMARY KEY, total REAL)")
	d2, err := m2.TableDetails(s2.ID, "", "", "orders")
	if err != nil {
		t.Fatal(err)
	}
	if len(d2.PrimaryKeys) != 1 || d2.PrimaryKeys[0] != "id" {
		t.Fatalf("PK fallback = %+v (unsupported %v)", d2.PrimaryKeys, d2.Unsupported)
	}
}

func TestDBInfoCachedAndRecorded(t *testing.T) {
	fc := sampleCatalog()
	m, s := catalogManager(t, fc, nil)
	info, err := m.DBInfo(s.ID)
	if err != nil || info.DBMSName != "FakeDB" {
		t.Fatalf("info=%+v err=%v", info, err)
	}
	if _, err := m.DBInfo(s.ID); err != nil || fc.opens != 1 {
		t.Fatalf("second DBInfo: err=%v opens=%d", err, fc.opens)
	}
	if s.ConnectedInfo["dbms"] != "FakeDB 1.2" || s.ConnectedInfo["odbcDriver"] != "fake.dll 03.80" {
		t.Fatalf("ConnectedInfo = %+v", s.ConnectedInfo)
	}
	if err := m.Disconnect(s.ID); err != nil {
		t.Fatal(err)
	}
	if fc.closes != 1 {
		t.Fatalf("catalog closed %d times", fc.closes)
	}
	if _, err := m.DBInfo("nope"); err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestDBInfoWithoutCatalog(t *testing.T) {
	m, s := newSQLiteManager(t)
	if _, err := m.DBInfo(s.ID); !errors.Is(err, errNoCatalog) {
		t.Fatalf("err = %v", err)
	}
}

func TestTableDetailsEmptyCatalogPKUsesFallback(t *testing.T) {
	fc := sampleCatalog()
	fc.pks = nil
	m, s := catalogManager(t, fc, nil)
	mustExec(t, m, s.ID, "CREATE TABLE orders (id INTEGER PRIMARY KEY, total REAL)")
	d, err := m.TableDetails(s.ID, "", "", "orders")
	if err != nil || len(d.PrimaryKeys) != 1 || d.PrimaryKeys[0] != "id" {
		t.Fatalf("PKs = %v err %v unsupported %v", d.PrimaryKeys, err, d.Unsupported)
	}
	// No key anywhere: reported as none, not as unsupported.
	fc2 := sampleCatalog()
	fc2.pks = nil
	m2, s2 := catalogManager(t, fc2, nil)
	mustExec(t, m2, s2.ID, "CREATE TABLE nokey (a INTEGER)")
	d2, err := m2.TableDetails(s2.ID, "", "", "nokey")
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range d2.Unsupported {
		if strings.Contains(u, "SQLPrimaryKeys") {
			t.Fatalf("unexpected unsupported note: %v", d2.Unsupported)
		}
	}
}

func TestLoadSchemaLargeSchemaSkipsPerTableColumns(t *testing.T) {
	fc := sampleCatalog()
	fc.wildcardErr = errors.New("HY090 invalid string length")
	for i := 0; i <= MaxEagerColumnTables; i++ {
		fc.tables = append(fc.tables, SchemaTable{Name: fmt.Sprintf("t%d", i), Type: "TABLE"})
	}
	m, s := catalogManager(t, fc, nil)
	snap, err := m.LoadSchema(s.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(fc.columnCalls) != 1 {
		t.Fatalf("expected only the wildcard call, got %d calls", len(fc.columnCalls))
	}
	if len(snap.Tables) != len(fc.tables) {
		t.Fatalf("tables = %d", len(snap.Tables))
	}
}
