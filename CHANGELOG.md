# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Edit > Find (Ctrl+F) and Find next (F3) in the SQL editor.
- Click a results column header to sort (numbers numerically, NULLs first;
  click again for descending, click `#` for the original order). Copy and
  export follow the displayed order.
- File > Recent files (last 10 opened or saved files; a missing file is
  dropped from the list; "Clear list").
- Connect and Test failures carry a one-line hint for common SQLSTATEs
  (IM014 bitness mismatch, IM002 unknown DSN/driver, IM003, 28000 login,
  08001 unreachable, HYT00 timeout, driver manager init failure).

### Fixed
- Rename tab (and other one-line prompts) ignored the typed text.
- A Test failure with several diagnostic records showed only the first line.
- Each statement is executed **exactly once**. Previously a statement that
  failed on the query path was retried as a command, so an `INSERT` could run
  twice.
- Session state reflects connectivity only; a failing statement no longer
  marks the connection as broken.
- `NULL` is shown as a grey `(null)` and is distinct from the string `'NULL'`
  in the grid, copies and exports.
- The connection dialog fills in the preselected saved profile.
- Clicking Connect twice (or pressing Enter twice) no longer opens the session
  twice and reports "already open".
- The x86 (32-bit) build works: per-architecture Windows resources.
- Saving a new profile twice no longer creates two copies.
- Quick queries from the schema browser are appended to the editor instead of
  replacing what you typed.
- Window size no longer grows on every start on high-DPI screens; the window is
  kept on a visible monitor.
- An unreadable `config.json` is kept as `config.json.broken-<time>` (with a
  notice) instead of being overwritten with defaults; config writes are atomic.

### Added
- Menu bar (File, Edit, Query, View, Tools, Help) and an icon toolbar with
  tooltips; actions are enabled or disabled according to the app state.
- F5 / Ctrl+Enter runs the statement under the caret and selects it; F6 runs
  the whole script; Esc cancels.
- Status bar with connection, transaction mode, activity, last result and
  caret position. The window title shows the connection and the tab (`*` when
  unsaved).
- Results grid: row numbers, right-aligned numbers, automatic column widths,
  multi-row selection, Ctrl+C / Ctrl+A, copy with headers.
- F9 switches the results between the grid and an aligned text view.
- Export to CSV, TSV, JSON, Markdown and plain text.
- Save / Save as for SQL tabs and a prompt for unsaved changes on close and
  exit.
- Monospace editor font with a fixed-pitch fallback.
- Version, commit and build date embedded in the exe (shown in About), new
  application icon, version information resource.
- `make check` local build and test script, `scripts/release.sh` for local
  release zips with SHA-256 sums, `scripts/smoke-wine.sh` visual smoke test.
- README, user guide, developer guide, MIT license, this changelog.
- Schema browser tree (Tables, Views, Synonyms, System tables) with columns,
  context menu, double-click for `SELECT TOP` / insert column name, filter,
  automatic load after connecting and F8 refresh.
- Catalog through the ODBC API (`SQLTables`, `SQLColumns`, `SQLPrimaryKeys`,
  `SQLForeignKeys`, `SQLStatistics`, `SQLGetInfo`), with the old SQL queries as
  fallback. Row-limit syntax (TOP / LIMIT / FETCH FIRST / FIRST) and identifier
  quoting follow the connected DBMS.
- The DSN and Driver fields list what is installed for the exe's bitness
  (`SQLDataSources` / `SQLDrivers`); the dialog title shows 32-bit or 64-bit.
- Connection profiles: New, Duplicate, Delete; connection-string templates for
  common drivers with a check for unfilled `<placeholders>`; extra attributes in
  Driver mode; Browse for File DSN; Test result shown inline.
- Tools > Settings: timeout, maximum rows, Load-more page size, editor font
  size, show system tables, stop on error, opt-in saved query history
  (`history.jsonl`, passwords masked).
- Splitter positions and the SQL files open at exit are restored.
- `crash.log` and an error dialog for unexpected errors; startup problems are
  shown in a message box instead of exiting silently.

### Changed
- Minimum Go version 1.24. Unit tests run with `CGO_ENABLED=0` and need no
  system packages (the ODBC driver is only linked on Windows).
- The Dockerfile is optional; native cross-compilation is the default build.

## [0.2] and earlier

See the git history (tags `v0.1`, `v0.2`).
