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
	ID          string
	Name        string
	Profile     config.ConnectionProfile
	DB          *sql.DB
	Tx          *sql.Tx
	State       SessionState
	CreatedAt   time.Time
	ConnectedAt time.Time
	LastQueryAt time.Time
	LastResult  *StatementResult
	LastDiag    Diagnostic
	// LastOutcome is the outcome of the last statement ("ok", "error",
	// "canceled", "timeout"); State only reflects connectivity.
	LastOutcome   string
	LastError     string
	History       []HistoryEntry
	SchemaCache   *SchemaCache
	ExecCancel    func()
	ConnectedInfo map[string]string

	// connStr is the full connection string (incl. password), kept in
	// memory only so the ODBC catalog connection can be opened lazily.
	connStr    string
	catalogMu  sync.Mutex
	catalog    Catalog
	catalogErr error
	dbInfo     *DBInfo
}

// Opener opens a database handle for a fully-built ODBC connection string.
// Production code wires it to sql.Open("odbc", ...); tests inject an Opener
// backed by a pure-Go driver (e.g. modernc.org/sqlite) so the engine can be
// exercised without CGO or a system ODBC driver manager.
type Opener func(connStr string) (*sql.DB, error)

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	activeID string
	opener   Opener
	// catalogOpener opens a native ODBC catalog connection (SQLTables,
	// SQLColumns, SQLGetInfo, …). nil means INFORMATION_SCHEMA/PRAGMA only.
	catalogOpener CatalogOpener
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
	Columns []Column
	// Rows holds cell text; SQL NULL cells hold "" and are flagged in Nulls.
	Rows [][]string
	// Nulls has the same shape as Rows; true marks a SQL NULL.
	Nulls       [][]bool
	RowCount    int
	Truncated   bool
	TruncatedAt int
}

// IsNull reports whether the cell at (row, col) is SQL NULL.
func (r ResultSet) IsNull(row, col int) bool {
	if row < 0 || row >= len(r.Nulls) || col < 0 || col >= len(r.Nulls[row]) {
		return false
	}
	return r.Nulls[row][col]
}

type StatementResult struct {
	Statement string
	// Kind is "query" or "exec" (from core.ClassifyStatement).
	Kind string
	// Message is a short outcome text for non-row results ("Command executed").
	Message        string
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
