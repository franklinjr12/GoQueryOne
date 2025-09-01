# GoQueryOne

A Go application that connects to ODBC data sources and executes SQL queries using the alexbrainman/odbc package.

## Features

- Connect to ODBC data sources using DSN (Data Source Name)
- Execute SQL queries and display results
- Support for configuration files
- Comprehensive error handling and logging
- Connection testing capabilities
- CSV-style result output

## Installation

1. Ensure you have Go 1.22.3 or later installed
2. Clone the repository
3. Install dependencies:
   ```bash
   go mod tidy
   ```

## Usage

### Configuration

The application uses constants defined in `cmd/main.go` for configuration:

```go
const (
    dsn            = "your_odbc_dsn_here"  // Set your ODBC DSN here
    query          = "SELECT 1"            // Set your SQL query here
    configFile     = ""                    // Set config file path if using one, or leave empty
    testConnection = false                 // Set to true to test connection only
)
```

### Running the Application

```bash
# Run the application
go run cmd/main.go

# Build and run
go build -o GoQueryOne cmd/main.go
./GoQueryOne
```

### Configuration File

Create a JSON configuration file (e.g., `config.json`):

```json
{
  "database": {
    "dsn": "your_dsn_name",
    "username": "your_username",
    "password": "your_password",
    "timeout": "30s"
  },
  "app": {
    "log_level": "info",
    "query_timeout": "60s",
    "max_rows": 1000
  }
}
```

## Output Format

Results are displayed in CSV format:
- Column headers separated by commas
- Each row on a new line
- Values separated by commas
- NULL values displayed as "NULL"

## Dependencies

- `github.com/alexbrainman/odbc` - ODBC driver for database/sql
- Standard library packages: `database/sql`, `flag`, `log`, `encoding/json`, `time`

## Building

```bash
go build -o GoQueryOne cmd/main.go
```

## Testing

To test the connection, set `testConnection = true` in the constants in `cmd/main.go`:

```go
const (
    dsn            = "your_odbc_dsn_here"
    query          = "SELECT 1"
    configFile     = ""
    testConnection = true  // Set to true to test connection only
)
```

## Error Handling

All errors and output are logged to the console/log file. The application provides clear error messages for:
- Connection failures
- Invalid DSN
- Query execution errors
- Configuration issues

## Future Enhancements

- UI framework integration (Fyne)
- Advanced query features
- Schema browsing
- Connection pooling
- Result export functionality
- Support for multiple database types

## License

This project is part of the GoQueryOne application suite.
