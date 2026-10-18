package odbc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/franklinjr12/GoQueryOne/internal/core"
)

type queryExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type execExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (m *Manager) ExecuteStatement(sessionID, sqlText string, params []any, opts QueryOptions) (StatementResult, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return StatementResult{}, err
	}
	if session.DB == nil {
		return StatementResult{}, errors.New("session is not connected")
	}

	statement := strings.TrimSpace(sqlText)
	if statement == "" {
		return StatementResult{}, errors.New("sql statement is empty")
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	maxRows := opts.MaxRows
	if maxRows <= 0 {
		maxRows = 10000
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	start := time.Now()

	m.mu.Lock()
	session.State = SessionExecuting
	session.ExecCancel = cancel
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		session.ExecCancel = nil
		if session.State == SessionExecuting {
			session.State = SessionConnected
		}
		m.mu.Unlock()
	}()

	var q queryExecutor = session.DB
	var ex execExecutor = session.DB
	if session.Tx != nil {
		q = session.Tx
		ex = session.Tx
	}

	result := StatementResult{
		Statement:      statement,
		StartedAt:      start,
		ParameterCount: CountPositionalParams(statement),
		Kind:           kindName(core.ClassifyStatement(statement)),
	}

	// Classify once and execute exactly once. Historically a failing query
	// was retried through ExecContext, which executed DML twice on some
	// drivers and re-ran every failing statement. Never retry on the other path.
	var execErr error
	if core.ClassifyStatement(statement) == core.KindQuery {
		rows, qErr := q.QueryContext(ctx, statement, params...)
		if qErr != nil {
			execErr = qErr
		} else {
			rs, readErr := readResultSet(rows, maxRows)
			_ = rows.Close()
			if readErr != nil {
				execErr = readErr
			} else if len(rs.Columns) == 0 {
				result.HasRows = false
				result.Message = "Command executed"
			} else {
				result.HasRows = true
				result.ResultSet = rs
			}
		}
	} else {
		execResult, exErr := ex.ExecContext(ctx, statement, params...)
		if exErr != nil {
			execErr = exErr
		} else {
			affected, raErr := execResult.RowsAffected()
			if raErr != nil {
				affected = -1
			}
			result.RowsAffected = affected
			result.Message = "Command executed"
		}
	}

	if execErr != nil {
		end := time.Now()
		result.ExecutionTime = end.Sub(start)
		result.CompletedAt = end
		result.ErrorMessage = execErr.Error()
		diag := BuildDiagnostic("execute", statement, execErr)
		result.Diagnostics = diag.Records
		if errors.Is(ctx.Err(), context.Canceled) {
			result.Canceled = true
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.TimedOut = true
		}
		status := "error"
		if result.Canceled {
			status = "canceled"
		} else if result.TimedOut {
			status = "timeout"
		}
		m.mu.Lock()
		session.LastDiag = diag
		session.LastResult = &result
		session.LastQueryAt = time.Now()
		session.LastOutcome = status
		session.LastError = execErr.Error()
		m.addHistory(session, statement, status, result.ExecutionTime)
		m.mu.Unlock()
		cancel()
		return result, execErr
	}

	end := time.Now()
	result.ExecutionTime = end.Sub(start)
	result.CompletedAt = end
	if errors.Is(ctx.Err(), context.Canceled) {
		result.Canceled = true
		result.ErrorMessage = "canceled by user"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		result.ErrorMessage = "timed out"
	}
	cancel()

	status := "ok"
	if result.Canceled {
		status = "canceled"
	}
	if result.TimedOut {
		status = "timeout"
	}

	m.mu.Lock()
	session.LastResult = &result
	session.LastQueryAt = time.Now()
	session.LastOutcome = status
	session.LastError = result.ErrorMessage
	m.addHistory(session, statement, status, result.ExecutionTime)
	m.mu.Unlock()

	return result, nil
}

func (m *Manager) ExecuteScript(sessionID, script string, opts ScriptOptions) (ScriptResult, error) {
	statements := SplitSQLScript(script)
	if len(statements) == 0 {
		return ScriptResult{}, errors.New("script has no executable statements")
	}
	start := time.Now()
	if opts.Timeout <= 0 {
		opts.Timeout = 60 * time.Second
	}
	if opts.MaxRows <= 0 {
		opts.MaxRows = 10000
	}

	result := ScriptResult{
		Results: make([]StatementResult, 0, len(statements)),
	}
	for _, stmt := range statements {
		stmtResult, err := m.ExecuteStatement(sessionID, stmt, nil, QueryOptions{
			Timeout:  opts.Timeout,
			MaxRows:  opts.MaxRows,
			PageSize: opts.PageSize,
		})
		result.Results = append(result.Results, stmtResult)
		if err != nil {
			if opts.StopOnErr {
				result.StoppedOnErr = true
				result.ExecutionTime = time.Since(start)
				return result, err
			}
		}
	}
	result.ExecutionTime = time.Since(start)
	return result, nil
}

func kindName(k core.StatementKind) string {
	if k == core.KindQuery {
		return "query"
	}
	return "exec"
}

// IsQueryStatement reports whether sqlText is executed through the query
// path (returns rows). Load More only re-executes such statements.
func IsQueryStatement(sqlText string) bool {
	return core.ClassifyStatement(sqlText) == core.KindQuery
}

func readResultSet(rows *sql.Rows, maxRows int) (ResultSet, error) {
	columns, err := rows.Columns()
	if err != nil {
		return ResultSet{}, err
	}
	colTypes, _ := rows.ColumnTypes()
	result := ResultSet{
		Columns: make([]Column, 0, len(columns)),
		Rows:    make([][]string, 0, min(maxRows, 1024)),
		Nulls:   make([][]bool, 0, min(maxRows, 1024)),
	}
	for i, name := range columns {
		col := Column{Name: name, Type: "unknown", Nullable: true}
		if i < len(colTypes) {
			col.Type = colTypes[i].DatabaseTypeName()
			if length, ok := colTypes[i].Length(); ok {
				col.Size = length
			}
			if nullable, ok := colTypes[i].Nullable(); ok {
				col.Nullable = nullable
			}
		}
		result.Columns = append(result.Columns, col)
	}

	scanValues := make([]any, len(columns))
	scanArgs := make([]any, len(columns))
	for i := range scanValues {
		scanArgs[i] = &scanValues[i]
	}

	for rows.Next() {
		if len(result.Rows) >= maxRows {
			result.Truncated = true
			result.TruncatedAt = maxRows
			break
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return ResultSet{}, err
		}
		row := make([]string, len(columns))
		nulls := make([]bool, len(columns))
		for i, raw := range scanValues {
			switch v := raw.(type) {
			case nil:
				row[i] = ""
				nulls[i] = true
			case []byte:
				row[i] = string(v)
			default:
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		result.Rows = append(result.Rows, row)
		result.Nulls = append(result.Nulls, nulls)
	}
	if err := rows.Err(); err != nil {
		return ResultSet{}, err
	}
	result.RowCount = len(result.Rows)
	return result, nil
}
