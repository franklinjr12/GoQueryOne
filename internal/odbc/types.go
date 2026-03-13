package odbc

import (
	"database/sql"
	"sync"
	"time"

	"github.com/franklinjr12/GoQueryOne/internal/config"
)

type SessionState string

const (
	SessionDisconnected SessionState = "DISCONNECTED"
	SessionConnecting   SessionState = "CONNECTING"
	SessionConnected    SessionState = "CONNECTED"
	SessionExecuting    SessionState = "EXECUTING"
	SessionError        SessionState = "ERROR"
)

type Session struct {
	ID            string
	Name          string
	Profile       config.ConnectionProfile
	DB            *sql.DB
	Tx            *sql.Tx
	State         SessionState
	CreatedAt     time.Time
	ConnectedAt   time.Time
	LastQueryAt   time.Time
	LastResult    *StatementResult
	LastDiag      Diagnostic
	History       []HistoryEntry
	SchemaCache   *SchemaCache
	ExecCancel    func()
	ConnectedInfo map[string]string
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	activeID string
}

type QueryOptions struct {
	Timeout   time.Duration
	MaxRows   int
	PageSize  int
	StopOnErr bool
}

type ScriptOptions struct {
	Timeout   time.Duration
	MaxRows   int
	PageSize  int
	StopOnErr bool
}

type ResultSet struct {
	Columns     []Column
	Rows        [][]string
	RowCount    int
	Truncated   bool
	TruncatedAt int
}

type StatementResult struct {
	Statement      string
	ResultSet      ResultSet
	HasRows        bool
	RowsAffected   int64
	ExecutionTime  time.Duration
	Diagnostics    []DiagRecord
	ErrorMessage   string
	Canceled       bool
	TimedOut       bool
	StartedAt      time.Time
	CompletedAt    time.Time
	ParameterCount int
}

type ScriptResult struct {
	Results       []StatementResult
	ExecutionTime time.Duration
	StoppedOnErr  bool
}

type Column struct {
	Name     string
	Type     string
	Size     int64
	Nullable bool
}

type DiagRecord struct {
	State       string
	NativeError int
	Message     string
}

type Diagnostic struct {
	Operation string
	Message   string
	SQL       string
	At        time.Time
	Records   []DiagRecord
}

type HistoryEntry struct {
	When     time.Time
	SQL      string
	Status   string
	Duration time.Duration
}

type DSNEntry struct {
	Name         string
	Driver       string
	Scope        string
	Architecture string
}

type DriverEntry struct {
	Name         string
	Architecture string
	Attributes   map[string]string
}

type SchemaSnapshot struct {
	Tables      []SchemaTable
	Columns     map[string][]SchemaColumn
	GeneratedAt time.Time
}

type SchemaTable struct {
	Catalog string
	Schema  string
	Name    string
	Type    string
}

type SchemaColumn struct {
	Catalog    string
	Schema     string
	Table      string
	Name       string
	Type       string
	Size       int64
	Nullable   bool
	DefaultVal string
}

type ForeignKey struct {
	Name      string
	Column    string
	RefTable  string
	RefColumn string
}

type IndexInfo struct {
	Name   string
	Column string
	Unique bool
}

type TableDetails struct {
	Catalog     string
	Schema      string
	Table       string
	Columns     []SchemaColumn
	PrimaryKeys []string
	ForeignKeys []ForeignKey
	Indexes     []IndexInfo
	Unsupported []string
}
