package odbc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
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
		if session.State != SessionError {
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
	}

	rows, qErr := q.QueryContext(ctx, statement, params...)
	if qErr == nil {
		defer rows.Close()
		result.HasRows = true
		result.ResultSet, err = readResultSet(rows, maxRows)
		if err != nil {
			qErr = err
		}
	}

	if qErr != nil {
		execResult, exErr := ex.ExecContext(ctx, statement, params...)
		if exErr != nil {
			end := time.Now()
			result.ExecutionTime = end.Sub(start)
			result.CompletedAt = end
			result.ErrorMessage = exErr.Error()
			result.Diagnostics = BuildDiagnostic("execute", statement, exErr).Records
			if errors.Is(ctx.Err(), context.Canceled) {
				result.Canceled = true
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				result.TimedOut = true
			}
			m.mu.Lock()
			session.LastDiag = BuildDiagnostic("execute", statement, exErr)
			session.LastResult = &result
			session.LastQueryAt = time.Now()
			session.State = SessionError
			m.addHistory(session, statement, "error", result.ExecutionTime)
			m.mu.Unlock()
			cancel()
			return result, exErr
		}
		affected, _ := execResult.RowsAffected()
		result.HasRows = false
		result.RowsAffected = affected
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

func readResultSet(rows *sql.Rows, maxRows int) (ResultSet, error) {
	columns, err := rows.Columns()
	if err != nil {
		return ResultSet{}, err
	}
	colTypes, _ := rows.ColumnTypes()
	result := ResultSet{
		Columns: make([]Column, 0, len(columns)),
		Rows:    make([][]string, 0, maxRows),
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
		for i, raw := range scanValues {
			switch v := raw.(type) {
			case nil:
				row[i] = "NULL"
			case []byte:
				row[i] = string(v)
			default:
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return ResultSet{}, err
	}
	result.RowCount = len(result.Rows)
	return result, nil
}
