# Developer guide

## Layout

| Package | Role | Platform |
|---|---|---|
| `cmd` | `main`, Windows resources (`rsrc_windows_*.syso`) | windows |
| `internal/buildinfo` | version / commit / date set by `-ldflags -X` | any |
| `internal/core` | statement classifier, splitter with byte ranges, `StatementAt`, UTF-16 offsets, connection-string escaping, durations | any |
| `internal/odbc` | `Manager`, sessions, executor (exactly-once), transactions, schema queries; the driver is injected as an `Opener` | any |
| `internal/odbcdriver` | registers `github.com/alexbrainman/odbc` | windows, or `integration` tag |
| `internal/export` | CSV / TSV / JSON / Markdown / text writers | any |
| `internal/app` | presenter / view-model: action enablement, status bar, title, messages | any |
| `internal/platform` | clipboard, file dialogs, ODBC administrator behind interfaces + fakes | any (adapters: windows) |
| `internal/securestore` | DPAPI-protected credentials behind a `Protector` | any (DPAPI: windows) |
| `internal/config` | `config.json` load/save, portable mode | any |
| `internal/ui` | the `lxn/walk` view; keep logic out of it and put it in `app`/`core` | windows |

The rule: anything testable lives outside `internal/ui`, so `go test ./...`
runs on Linux with `CGO_ENABLED=0` and no system packages.

## Build and test

```bash
make check                 # gofmt, vet (host, windows/amd64, windows/386), tests, both builds
make test                  # CGO_ENABLED=0 go test ./...
make test-race             # needs cgo
make test-integration      # unixODBC + an ODBC driver (e.g. libsqliteodbc), -tags integration
make winres                # regenerate cmd/rsrc_windows_{amd64,386}.syso from winres/winres.json
make release VERSION=1.0.0 # dist/GoQueryOne-1.0.0-windows-{x64,x86}.zip + SHA256SUMS.txt
```

`scripts/check.sh` is the implementation; it works without `make`. There is
deliberately **no CI workflow**: run `make check` before pushing.

Version info is injected with
`-X github.com/franklinjr12/GoQueryOne/internal/buildinfo.{Version,Commit,Date}`.

Tests against a real database use `modernc.org/sqlite` (pure Go, test-only)
through the injected `odbc.Opener`, so the executor is exercised without ODBC.

## Visual smoke test under Wine (Linux box)

```bash
sudo apt-get install -y wine wine64 xdotool imagemagick unixodbc libsqliteodbc sqlite3 xvfb
Xvfb :99 -screen 0 1600x1000x24 &
DSN=DemoSQLite scripts/smoke-wine.sh /tmp/shots
```

The script builds the exe with a temporary copy of `alexbrainman/odbc`
patched to tolerate Wine's missing `SQL_ATTR_CP_MATCH` (via `-modfile`; the
patch is never committed), starts it under `wine64`, drives connect / query /
insert / error / text view with `xdotool`, and saves screenshots. Without a
window manager, keyboard focus has to be set with `xdotool windowfocus`; the
script handles that.

Known walk/Wine quirks:
- Shortcuts are dispatched by walk only when the keystroke reaches a walk
  window (editor, main window); while the results tab header has focus, F9
  does nothing. Use the toolbar button.
- Consolas does not exist under Wine; `monoFont` falls back to the first
  installed fixed-pitch family.

## Releases

Never automated. `make release` builds zips locally; Franklin tags and
publishes manually after review.
