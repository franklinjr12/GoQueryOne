package odbc

import "testing"

func TestSnapshotColumnsRoundTrip(t *testing.T) {
	var s SchemaSnapshot
	tbl := SchemaTable{Catalog: "c", Schema: "s", Name: "t"}
	if s.ColumnsOf(tbl) != nil {
		t.Fatal("expected no columns")
	}
	s.SetColumns(tbl, []SchemaColumn{{Name: "a"}})
	if got := s.ColumnsOf(tbl); len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("ColumnsOf = %+v", got)
	}
	if s.ColumnsOf(SchemaTable{Name: "t"}) != nil {
		t.Fatal("catalog/schema must be part of the key")
	}
}
