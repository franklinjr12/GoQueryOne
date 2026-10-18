package odbc

import "testing"

func TestLoadSchemaSQLiteIncludesColumns(t *testing.T) {
	m, s := newSQLiteManager(t)
	mustExec(t, m, s.ID, "CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT NOT NULL, city TEXT)")
	mustExec(t, m, s.ID, "CREATE VIEW v_c AS SELECT name FROM customers")

	snap, err := m.LoadSchema(s.ID, "")
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	var customers *SchemaTable
	for i := range snap.Tables {
		if snap.Tables[i].Name == "customers" {
			customers = &snap.Tables[i]
		}
	}
	if customers == nil {
		t.Fatalf("customers not in %+v", snap.Tables)
	}
	cols := snap.ColumnsOf(*customers)
	if len(cols) != 3 || cols[0].Name != "id" || cols[1].Nullable || !cols[2].Nullable {
		t.Fatalf("columns = %+v", cols)
	}

	filtered, err := m.LoadSchema(s.ID, "city")
	if err != nil {
		t.Fatalf("LoadSchema(filter): %v", err)
	}
	if len(filtered.Tables) != 1 || len(filtered.ColumnsOf(filtered.Tables[0])) != 1 {
		t.Fatalf("filter by column: tables=%+v columns=%+v", filtered.Tables, filtered.Columns)
	}
}
