package app

import (
	"strings"
	"testing"

	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

func TestBuildSchemaTree(t *testing.T) {
	tables := []odbc.SchemaTable{
		{Name: "orders", Type: "TABLE"},
		{Name: "v_curitiba", Type: "VIEW"},
		{Schema: "dbo", Name: "Customers", Type: "BASE TABLE"},
		{Name: "sqlite_master", Type: "SYSTEM TABLE"},
		{Name: "syn", Type: "SYNONYM"},
		{Name: "weird", Type: "EXTERNAL"},
		{Name: "orders_v", Type: "MATERIALIZED VIEW"},
	}
	snap := odbc.SchemaSnapshot{Tables: tables}
	snap.SetColumns(tables[0], []odbc.SchemaColumn{
		{Name: "id", Type: "INTEGER", Size: 10},
		{Name: "note", Type: "VARCHAR", Size: 40, Nullable: true},
	})

	roots := BuildSchemaTree(snap, false)
	var labels []string
	for _, r := range roots {
		labels = append(labels, r.Label)
		if r.Kind != NodeGroup || r.Table != -1 || r.Parent != nil {
			t.Fatalf("bad group node %+v", r)
		}
	}
	want := "Tables (2)|Views (2)|Synonyms (1)|System tables (1)|Other (1)"
	if got := strings.Join(labels, "|"); got != want {
		t.Fatalf("groups = %q, want %q", got, want)
	}
	tablesGroup := roots[0]
	if tablesGroup.Children[0].Label != "dbo.Customers" || tablesGroup.Children[1].Label != "orders" {
		t.Fatalf("tables not sorted case-insensitively: %q, %q", tablesGroup.Children[0].Label, tablesGroup.Children[1].Label)
	}
	orders := tablesGroup.Children[1]
	if orders.Kind != NodeTable || orders.Table != 0 || orders.Parent != tablesGroup {
		t.Fatalf("bad table node %+v", orders)
	}
	if len(orders.Children) != 2 {
		t.Fatalf("orders has %d columns", len(orders.Children))
	}
	if c := orders.Children[0]; c.Kind != NodeColumn || c.Column != "id" || c.Label != "id  INTEGER, not null" || c.Parent != orders || c.Table != 0 {
		t.Fatalf("bad column node %+v", c)
	}
	if c := orders.Children[1]; c.Label != "note  VARCHAR(40)" {
		t.Fatalf("column label = %q", c.Label)
	}
	if len(BuildSchemaTree(odbc.SchemaSnapshot{}, true)) != 0 {
		t.Fatal("empty snapshot should give no groups")
	}
}

func TestColumnLabel(t *testing.T) {
	cases := []struct {
		c    odbc.SchemaColumn
		want string
	}{
		{odbc.SchemaColumn{Name: "a", Type: "NVARCHAR", Size: 50, Nullable: true}, "a  NVARCHAR(50)"},
		{odbc.SchemaColumn{Name: "b", Type: "TEXT", Size: 2147483647, Nullable: true}, "b  TEXT"},
		{odbc.SchemaColumn{Name: "c", Type: "DECIMAL", Size: 18}, "c  DECIMAL, not null"},
		{odbc.SchemaColumn{Name: "d", Nullable: true}, "d"},
	}
	for _, tc := range cases {
		if got := ColumnLabel(tc.c); got != tc.want {
			t.Errorf("ColumnLabel(%+v) = %q, want %q", tc.c, got, tc.want)
		}
	}
}

func TestDatabaseInfoNode(t *testing.T) {
	if n := DatabaseInfoNode(odbc.DBInfo{}); n != nil {
		t.Fatalf("empty info should give no node, got %+v", n)
	}
	n := DatabaseInfoNode(odbc.DBInfo{DBMSName: "SQLite", DBMSVersion: "3.46.1", DriverName: "sqlite3odbc.dll", DriverVersion: "0.9999", IdentifierQuote: `"`})
	if n == nil || n.Kind != NodeInfo || n.Table != -1 || len(n.Children) != 3 {
		t.Fatalf("node = %+v", n)
	}
	want := []string{"DBMS: SQLite 3.46.1", "Driver: sqlite3odbc.dll 0.9999", `Identifier quote: "`}
	for i, c := range n.Children {
		if c.Label != want[i] || c.Parent != n || c.Kind != NodeInfo || c.Table != -1 {
			t.Errorf("child %d = %+v, want %q", i, c, want[i])
		}
	}
	n = DatabaseInfoNode(odbc.DBInfo{DriverName: "x.dll"})
	if len(n.Children) != 2 || n.Children[1].Label != "Identifier quote: (none)" {
		t.Fatalf("no quote: %+v", n.Children)
	}
}
