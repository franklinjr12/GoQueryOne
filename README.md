# GoQueryOne

A Go application that connects to ODBC data sources and executes SQL queries using the alexbrainman/odbc package.

## Features

- Connect to ODBC data sources using DSN (Data Source Name)
- Execute SQL queries and display results
- Support for configuration files
- Comprehensive error handling and logging
- Connection testing capabilities
- CSV-style result output

## Installation and building
- Use the Dockerfile
```bash
docker buildx build --output type=local,dest=out .
```
- The output includes `GoQueryOne.exe`. The Windows manifest for Walk/Common Controls 6 and DPI behavior is embedded in the executable from `cmd/GoQueryOne.syso`.
- If `cmd/GoQueryOne.exe.manifest` changes, regenerate the embedded resource with `go run github.com/akavel/rsrc@latest -manifest cmd\GoQueryOne.exe.manifest -o cmd\GoQueryOne.syso`.

## Usage

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

## Error Handling

All errors and output are logged to the console/log file. The application provides clear error messages for:
- Connection failures
- Invalid DSN
- Query execution errors
- Configuration issues

## Future Enhancements

- Schema browsing
- Advanced query features
- Connection pooling
- Result export functionality
- Support for multiple database types

## License

This project is part of the GoQueryOne application suite.
