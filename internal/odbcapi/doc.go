// Package odbcapi talks to the ODBC driver manager directly for metadata the
// database/sql driver does not expose: SQLTables, SQLColumns,
// SQLPrimaryKeys, SQLForeignKeys, SQLStatistics, SQLGetInfo, SQLDataSources
// and SQLDrivers. The Windows implementation calls odbc32.dll through
// syscalls (no cgo); result-set decoding is platform independent and tested
// on any OS with fake result sets.
package odbcapi
