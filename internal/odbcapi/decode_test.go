package odbcapi

import (
	"errors"
	"reflect"
	"testing"

	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

// fakeRS rows: each row is a slice of *string (nil = NULL), 1-based access.
type fakeRS struct {
	rows [][]*string
	i    int
	err  error // returned by Next after the rows
}

func s(v string) *string { return &v }

func (f *fakeRS) Next() (bool, error) {
	if f.i < len(f.rows) {
		f.i++
		return true, nil
	}
	return false, f.err
}

func (f *fakeRS) Col(i int) (string, bool) {
	row := f.rows[f.i-1]
	if i < 1 || i > len(row) || row[i-1] == nil {
		return "", false
	}
	return *row[i-1], true
}

func TestReadTables(t *testing.T) {
	rs := &fakeRS{rows: [][]*string{
		{nil, s("dbo"), s("orders"), s("TABLE"), nil},
		{s("db"), nil, s("v"), s("VIEW"), s("a view")},
	}}
	got, err := readTables(rs)
	if err != nil {
		t.Fatal(err)
	}
	want := []odbc.SchemaTable{{Schema: "dbo", Name: "orders", Type: "TABLE"}, {Catalog: "db", Name: "v", Type: "VIEW"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if _, err := readTables(&fakeRS{err: errors.New("boom")}); err == nil {
		t.Fatal("expected error")
	}
}

func row18(table, name, dataType, typeName, size, nullable, def, pos string) []*string {
	r := make([]*string, 18)
	r[2], r[3] = s(table), s(name)
	if dataType != "" {
		r[4] = s(dataType)
	}
	if typeName != "" {
		r[5] = s(typeName)
	}
	if size != "" {
		r[6] = s(size)
	}
	if nullable != "" {
		r[10] = s(nullable)
	}
	if def != "" {
		r[12] = s(def)
	}
	if pos != "" {
		r[16] = s(pos)
	}
	return r
}

func TestReadColumns(t *testing.T) {
	rs := &fakeRS{rows: [][]*string{
		row18("t", "b", "12", "varchar", "40", "1", "'x'", "2"),
		row18("t", "a", "4", "int", "10", "0", "", "1"),
		row18("u", "z", "93", "", " 23 ", "2", "", ""),
		{nil, nil, s("old"), s("c"), s("4"), s("INTEGER"), s("10")}, // ODBC 2 driver: short row
	}}
	got, err := readColumns(rs)
	if err != nil {
		t.Fatal(err)
	}
	want := []odbc.SchemaColumn{
		{Table: "t", Name: "a", Type: "int", Size: 10, Nullable: false},
		{Table: "t", Name: "b", Type: "varchar", Size: 40, Nullable: true, DefaultVal: "'x'"},
		{Table: "u", Name: "z", Type: "SQL type 93", Size: 23, Nullable: true},
		{Table: "old", Name: "c", Type: "INTEGER", Size: 10, Nullable: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if _, err := readColumns(&fakeRS{err: errors.New("boom")}); err == nil {
		t.Fatal("expected error")
	}
}

func TestReadPrimaryKeys(t *testing.T) {
	rs := &fakeRS{rows: [][]*string{
		{nil, nil, s("t"), s("b"), s("2"), s("pk")},
		{nil, nil, s("t"), s("a"), s("1"), s("pk")},
	}}
	got, err := readPrimaryKeys(rs)
	if err != nil || !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("got %v err %v", got, err)
	}
	if _, err := readPrimaryKeys(&fakeRS{err: errors.New("boom")}); err == nil {
		t.Fatal("expected error")
	}
}

func TestReadForeignKeys(t *testing.T) {
	rs := &fakeRS{rows: [][]*string{
		{nil, s("dbo"), s("customers"), s("id"), nil, s("dbo"), s("orders"), s("customer_id"), s("1"), nil, nil, s("fk_orders_customers")},
		{nil, nil, s("x"), s("xid"), nil, nil, s("orders"), s("x_id"), s("1")},
	}}
	got, err := readForeignKeys(rs)
	if err != nil {
		t.Fatal(err)
	}
	want := []odbc.ForeignKey{
		{Name: "fk_orders_customers", Column: "customer_id", RefTable: "dbo.customers", RefColumn: "id"},
		{Name: "fk_x_id", Column: "x_id", RefTable: "x", RefColumn: "xid"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestReadIndexes(t *testing.T) {
	rs := &fakeRS{rows: [][]*string{
		{nil, nil, s("t"), nil, nil, nil, s("0"), nil, nil}, // SQL_TABLE_STAT
		{nil, nil, s("t"), s("0"), nil, s("pk_t"), s("1"), s("1"), s("id")},
		{nil, nil, s("t"), s("1"), nil, s("ix_t_name"), s("3"), s("1"), s("name")},
		{nil, nil, s("t"), s("1"), nil, nil, s("3"), s("1"), s("q")}, // no name: skipped
	}}
	got, err := readIndexes(rs)
	if err != nil {
		t.Fatal(err)
	}
	want := []odbc.IndexInfo{{Name: "pk_t", Column: "id", Unique: true}, {Name: "ix_t_name", Column: "name"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseDriverAttributes(t *testing.T) {
	got := parseDriverAttributes("UsageCount=1\x00Driver=C:\\x.dll\x00Flag\x00\x00")
	want := map[string]string{"UsageCount": "1", "Driver": `C:\x.dll`, "Flag": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}
