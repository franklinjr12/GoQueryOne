package odbc

import (
	"database/sql"
	"time"
)

// Connection represents an ODBC database connection
type Connection struct {
	DSN           string
	Handle        *sql.DB
	connected     bool
	ConnectTime   time.Time
	LastQueryTime time.Time
}

// QueryResult represents the result of a SQL query execution
type QueryResult struct {
	Columns       []Column
	Rows          [][]interface{}
	RowCount      int
	ExecutionTime time.Duration
	Error         error
}

// Column represents metadata about a database column
type Column struct {
	Name     string
	Type     string
	Size     int
	Nullable bool
}

// QueryOptions represents options for query execution
type QueryOptions struct {
	Timeout   time.Duration
	MaxRows   int
	FetchSize int
}

// ConnectionConfig represents configuration for database connections
type ConnectionConfig struct {
	DSN      string
	Username string
	Password string
	Timeout  time.Duration
}
