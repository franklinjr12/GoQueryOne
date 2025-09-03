package odbc

import (
	"database/sql"
	"errors"
	"log"
	"time"
)

// ExecuteQuery executes a SQL query and returns the result
func (c *Connection) ExecuteQuery(query string) (*QueryResult, error) {
	if !c.connected {
		log.Printf("Error: not connected to %s", c.DSN)
		return nil, errors.New("not connected to " + c.DSN)
	}

	startTime := time.Now()
	log.Printf("Executing query: %s", query)

	// Execute the query using database/sql
	rows, err := c.Handle.Query(query)
	if err != nil {
		log.Printf("Error: failed to execute query: %v", err)
		return nil, errors.New("failed to execute query: " + err.Error())
	}
	defer rows.Close()

	// Parse the result
	result, err := c.parseResult(rows)
	if err != nil {
		log.Printf("Error: failed to parse query result: %v", err)
		return nil, errors.New("failed to parse query result: " + err.Error())
	}

	result.ExecutionTime = time.Since(startTime)
	c.LastQueryTime = time.Now()

	log.Printf("Query executed successfully in %v, returned %d rows", result.ExecutionTime, result.RowCount)
	return result, nil
}

// ExecuteScript executes a multi-statement SQL script
func (c *Connection) ExecuteScript(script string) ([]*QueryResult, error) {
	if !c.connected {
		log.Printf("Error: not connected to %s", c.DSN)
		return nil, errors.New("not connected to " + c.DSN)
	}

	log.Printf("Executing script with %d characters", len(script))

	// For now, we'll treat the script as a single query
	// In a more advanced implementation, we could parse and split the script
	result, err := c.ExecuteQuery(script)
	if err != nil {
		return nil, err
	}

	return []*QueryResult{result}, nil
}

// parseResult parses the database/sql result rows into our QueryResult structure
func (c *Connection) parseResult(rows *sql.Rows) (*QueryResult, error) {
	result := &QueryResult{
		Rows:    make([][]interface{}, 0),
		Columns: make([]Column, 0),
	}

	// Get column information
	columns, err := rows.Columns()
	if err != nil {
		log.Printf("Error: failed to get column information: %v", err)
		return nil, errors.New("failed to get column information: " + err.Error())
	}

	// Parse column metadata
	for _, colName := range columns {
		column := Column{
			Name:     colName,
			Type:     "unknown", // Database/sql doesn't provide type info easily
			Size:     0,
			Nullable: true,
		}
		result.Columns = append(result.Columns, column)
	}

	// Prepare slice for row values
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// Iterate through rows
	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			log.Printf("Warning: failed to scan row: %v", err)
			continue
		}

		// Convert row values to interface{} slice
		row := make([]interface{}, len(columns))
		for i, v := range values {
			row[i] = v
		}
		result.Rows = append(result.Rows, row)
	}

	result.RowCount = len(result.Rows)
	return result, nil
}

// DisplayResult displays the query result in a formatted way
func DisplayResult(result *QueryResult) {
	if result == nil {
		log.Println("No result to display")
		return
	}

	if result.Error != nil {
		log.Printf("Error: %v", result.Error)
		return
	}

	log.Printf("Query executed in %v", result.ExecutionTime)
	log.Printf("Returned %d rows with %d columns", result.RowCount, len(result.Columns))

	if result.RowCount == 0 {
		log.Println("No rows returned")
		return
	}

	// Display column headers
	headerLine := ""
	for i, col := range result.Columns {
		if i > 0 {
			headerLine += ","
		}
		headerLine += col.Name
	}
	log.Println(headerLine)

	// Display rows
	for _, row := range result.Rows {
		rowLine := ""
		for i, val := range row {
			if i > 0 {
				rowLine += ","
			}
			if val == nil {
				rowLine += "NULL"
			} else {
				// check if val is []byte if so convert to string
				if _, ok := val.([]byte); ok {
					val = string(val.([]byte))
				}
				rowLine += val.(string)
			}
		}
		log.Println(rowLine)
	}

	log.Printf("Total: %d rows", result.RowCount)
}
