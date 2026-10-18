# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed
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

### Changed
- Minimum Go version 1.24. Unit tests run with `CGO_ENABLED=0` and need no
  system packages (the ODBC driver is only linked on Windows).
- The Dockerfile is optional; native cross-compilation is the default build.

## [0.2] and earlier

See the git history (tags `v0.1`, `v0.2`).
