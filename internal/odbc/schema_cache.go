package odbc

import (
	"fmt"
	"sync"
)

type SchemaCache struct {
	mu      sync.RWMutex
	schema  *SchemaSnapshot
	details map[string]TableDetails
}

func NewSchemaCache() *SchemaCache {
	return &SchemaCache{
		details: map[string]TableDetails{},
	}
}

func (s *SchemaCache) GetSchema() (*SchemaSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.schema == nil {
		return nil, false
	}
	return s.schema, true
}

func (s *SchemaCache) PutSchema(snapshot *SchemaSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schema = snapshot
}

func (s *SchemaCache) GetTableDetails(catalog, schema, table string) (TableDetails, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.details[detailsKey(catalog, schema, table)]
	return value, ok
}

func (s *SchemaCache) PutTableDetails(details TableDetails) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.details[detailsKey(details.Catalog, details.Schema, details.Table)] = details
}

func (s *SchemaCache) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schema = nil
	s.details = map[string]TableDetails{}
}

func detailsKey(catalog, schema, table string) string {
	return fmt.Sprintf("%s|%s|%s", catalog, schema, table)
}
