//go:build windows

package odbcapi

import (
	"os"
	"strings"
	"testing"

	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

// Integration test against a real DSN. Run with GOQUERYONE_TEST_DSN set to a
// DSN whose database has the demo schema (customers, orders, v_curitiba),
// e.g. under Wine on the dev box (see docs/DEVELOPER.md).
func TestCatalogAgainstDSN(t *testing.T) {
	dsn := os.Getenv("GOQUERYONE_TEST_DSN")
	if dsn == "" {
		t.Skip("GOQUERYONE_TEST_DSN not set")
	}
	cat, err := Open("DSN=" + dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer cat.Close()

	info, err := cat.Info()
	if err != nil || info.DBMSName == "" {
		t.Fatalf("Info = %+v, %v", info, err)
	}
	t.Logf("DBMS %q %q, driver %q %q, quote %q", info.DBMSName, info.DBMSVersion, info.DriverName, info.DriverVersion, info.IdentifierQuote)

	tables, err := cat.Tables()
	if err != nil {
		t.Fatalf("Tables: %v", err)
	}
	byName := map[string]odbc.SchemaTable{}
	for _, tb := range tables {
		byName[strings.ToLower(tb.Name)] = tb
	}
	for _, want := range []string{"customers", "orders", "v_curitiba"} {
		if _, ok := byName[want]; !ok {
			t.Fatalf("table %s missing from %+v", want, tables)
		}
	}
	t.Logf("%d tables; v_curitiba type %q", len(tables), byName["v_curitiba"].Type)

	cols, err := cat.Columns(byName["customers"])
	if err != nil || len(cols) < 3 || cols[0].Name != "id" {
		t.Fatalf("Columns(customers) = %+v, %v", cols, err)
	}
	all, err := cat.Columns(odbc.SchemaTable{})
	t.Logf("wildcard SQLColumns: %d columns, err %v", len(all), err)

	t.Logf("customers ref %+v", byName["customers"])
	if p2, e2 := cat.PrimaryKeys(odbc.SchemaTable{Name: "customers"}); true {
		t.Logf("PrimaryKeys(name only) = %v, %v", p2, e2)
	}
	pks, err := cat.PrimaryKeys(byName["customers"])
	t.Logf("PrimaryKeys(customers) = %v, %v", pks, err)
	// sqlite3odbc reports no key for an INTEGER PRIMARY KEY (rowid alias);
	// the Manager then falls back to PRAGMA table_info. Other drivers must
	// return "id".
	if err == nil && len(pks) > 0 && pks[0] != "id" {
		t.Fatalf("PrimaryKeys = %v", pks)
	}
	fks, err := cat.ForeignKeys(byName["orders"])
	t.Logf("ForeignKeys(orders) = %+v, %v", fks, err)
	idx, err := cat.Indexes(byName["orders"])
	t.Logf("Indexes(orders) = %+v, %v", idx, err)

	dsns, err := DataSources()
	if err != nil {
		t.Fatalf("DataSources: %v", err)
	}
	found := false
	for _, d := range dsns {
		if strings.EqualFold(d.Name, dsn) {
			found = true
		}
	}
	t.Logf("DataSources = %+v (found %v)", dsns, found)
	drivers, err := Drivers()
	t.Logf("Drivers = %d, %v", len(drivers), err)
}

func TestOpenBadDSN(t *testing.T) {
	if os.Getenv("GOQUERYONE_TEST_DSN") == "" {
		t.Skip("GOQUERYONE_TEST_DSN not set")
	}
	_, err := Open("DSN=__no_such_dsn__")
	if err == nil {
		t.Fatal("expected an error")
	}
	t.Logf("bad DSN error: %v", err)
}
