package odbc

import "testing"

func TestSchemaCacheInvalidate(t *testing.T) {
	cache := NewSchemaCache()
	cache.PutSchema(&SchemaSnapshot{
		Tables: []SchemaTable{{Schema: "dbo", Name: "users"}},
	})
	cache.PutTableDetails(TableDetails{
		Schema: "dbo",
		Table:  "users",
	})

	if _, ok := cache.GetSchema(); !ok {
		t.Fatalf("expected schema cache to exist")
	}
	if _, ok := cache.GetTableDetails("", "dbo", "users"); !ok {
		t.Fatalf("expected table details cache to exist")
	}

	cache.Invalidate()

	if _, ok := cache.GetSchema(); ok {
		t.Fatalf("expected schema cache to be cleared")
	}
	if _, ok := cache.GetTableDetails("", "dbo", "users"); ok {
		t.Fatalf("expected table details cache to be cleared")
	}
}
