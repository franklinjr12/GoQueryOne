#!/usr/bin/env bash
# Visual smoke test of the Windows build under Wine + Xvfb (Linux dev box only, optional).
#
# Builds dist/smoke/GoQueryOne-x64.exe against a locally patched copy of
# github.com/alexbrainman/odbc (Wine's odbc32 does not support SQL_ATTR_CP_MATCH,
# which upstream treats as fatal). The patch lives only in a temp dir and a temp
# -modfile; nothing is committed and go.mod/go.sum are not touched.
#
# Requirements: wine (wine64), Xvfb, xdotool, imagemagick (import), unixodbc,
# libsqliteodbc, sqlite3. A DSN named $DSN must exist in ~/.odbc.ini, e.g.
#   [DemoSQLite]
#   Driver=SQLite3
#   Database=/path/to/demo.db
#
# Usage: scripts/smoke-wine.sh [outdir]   (default: artifacts/screenshots)
# Env:   DISPLAY (default :99, started if missing), DSN (default DemoSQLite),
#        WINE64 (default /usr/lib/wine/wine64), SKIP_FLOWS=1 to only take the startup shot.
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT=$(pwd)
OUT=${1:-artifacts/screenshots}
DSN=${DSN:-DemoSQLite}
export DISPLAY=${DISPLAY:-:99}
export WINEDEBUG=${WINEDEBUG:--all}
WINE64=${WINE64:-/usr/lib/wine/wine64}
[ -x "$WINE64" ] || WINE64=$(command -v wine64 || command -v wine)
mkdir -p "$OUT" dist/smoke

if ! xdpyinfo -display "$DISPLAY" >/dev/null 2>&1; then
  Xvfb "$DISPLAY" -screen 0 1600x1000x24 >/dev/null 2>&1 &
  sleep 2
fi

# 1) patched driver copy + temp modfile
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
MODDIR=$(go list -m -f '{{.Dir}}' github.com/alexbrainman/odbc)
cp -r "$MODDIR" "$WORK/odbc" && chmod -R u+w "$WORK/odbc"
python3 - "$WORK/odbc/driver.go" <<'PY'
import re, sys
p = sys.argv[1]
s = open(p).read()
s2 = re.sub(r'ret = api\.SQLSetEnvUIntPtrAttr\(drv\.h, api\.SQL_ATTR_CP_MATCH,([^\n]*)\n\s*if IsError\(ret\) \{\n[^}]*\}',
            r'_ = api.SQLSetEnvUIntPtrAttr(drv.h, api.SQL_ATTR_CP_MATCH,\1 // wine smoke patch', s, count=1)
if s2 == s:
    sys.exit("smoke-wine: could not patch CP_MATCH in driver.go (upstream changed?)")
open(p, 'w').write(s2)
PY
cp go.mod "$WORK/go.mod" && cp go.sum "$WORK/go.sum"
go mod edit -modfile="$WORK/go.mod" -replace "github.com/alexbrainman/odbc=$WORK/odbc"
PKG=github.com/franklinjr12/GoQueryOne/internal/buildinfo
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -modfile="$WORK/go.mod" -trimpath \
  -ldflags "-H=windowsgui -X $PKG.Version=smoke -X $PKG.Commit=$(git rev-parse --short HEAD)" \
  -o dist/smoke/GoQueryOne-x64.exe ./cmd

# 2) portable run dir with a profile for the DSN
RUN=dist/smoke/run
mkdir -p "$RUN"
cp dist/smoke/GoQueryOne-x64.exe "$RUN/"
touch "$RUN/portable.flag"
cat > "$RUN/config.json" <<JSON
{
  "schemaVersion": 1,
  "app": {"window": {"width": 1260, "height": 760, "x": 40, "y": 40},
          "logging": {"level": "info", "file": "logs.txt", "maxMiB": 20, "enabled": true}},
  "query": {"defaultTimeoutMs": 60000, "defaultMaxRows": 10000, "fetchPageSize": 500, "stopOnErrorInScript": true},
  "connections": [{"id": "smoke-demo", "name": "Demo SQLite", "type": "dsn", "dsn": "$DSN",
                   "savePassword": false, "options": {"loginTimeoutMs": 30000, "limitSyntax": "LIMIT"}}]
}
JSON

shot() { sleep "${2:-1}"; import -window root "$OUT/$1.png"; echo "screenshot: $OUT/$1.png"; }
win() { xdotool search --sync --name "$1" | head -1; }

(cd "$RUN" && "$WINE64" GoQueryOne-x64.exe >"$ROOT/dist/smoke/wine.log" 2>&1 &)
MAIN=$(timeout 60 bash -c "until xdotool search --name '^GoQueryOne' >/dev/null 2>&1; do sleep 1; done; xdotool search --name '^GoQueryOne' | head -1")
shot 01-startup 3

if [ "${SKIP_FLOWS:-0}" != "1" ]; then
  # Without a window manager, X focus must be set explicitly (xdotool
  # windowfocus) before sending keys. Layout assumes 1600x1000 with the
  # window at 40,40 sized 1260x760. Best effort: a failing step keeps the
  # screenshots taken so far.
  set +e
  EDIT_X=${EDIT_X:-980} EDIT_Y=${EDIT_Y:-300}
  focus_main() { xdotool windowfocus "$MAIN" 2>/dev/null; sleep 0.3; }
  focus_editor() { focus_main; xdotool mousemove "$EDIT_X" "$EDIT_Y" click 1; sleep 0.5; }
  typeln() { xdotool type --delay 15 "$1"; xdotool key Return; }
  focus_main
  xdotool mousemove 700 600 click 1
  xdotool key ctrl+shift+c                                   # Tools > Connect
  DLG=$(timeout 10 bash -c "until xdotool search --name '^Connection$' >/dev/null 2>&1; do sleep 0.5; done; xdotool search --name '^Connection$' | head -1")
  shot 02-connect-dialog 2
  if [ -n "$DLG" ]; then xdotool windowfocus "$DLG"; sleep 0.3; xdotool key Return; fi   # Connect (default button)
  shot 03-connected 5
  # warm-up: after the modal dialog closes the first keystrokes can be lost
  focus_editor; sleep 1
  typeln "                    SELECT id, name, city, balance, NULL AS null_value, 'NULL' AS text_null FROM customers;"
  xdotool key Up F5                                          # caret on the SELECT line: runs only that statement
  shot 04-query-results 3
  focus_main
  xdotool mousemove 698 474 click 1; sleep 1                 # Results tab (an error clears the grid, so do this right after the query)
  focus_editor; sleep 0.5; xdotool key F9
  shot 07-text-view 2
  xdotool mousemove 545 103 click 1                          # back to the grid
  focus_editor
  xdotool key ctrl+End Return
  typeln "INSERT INTO orders (customer_id, total, created_at) VALUES (1, 10.5, '2026-10-18');"
  xdotool key F5                                             # caret after the INSERT: runs only the INSERT
  shot 05-insert-messages 3
  focus_editor
  xdotool key ctrl+End Return
  typeln "SELECT * FROM no_such_table;"
  xdotool key F5
  shot 06-error 3
  focus_main
  xdotool key alt+q                                          # Query menu
  shot 08-query-menu 2
  xdotool key Escape Escape
  focus_main
  xdotool mousemove 94 223 click --repeat 2 --delay 120 1    # double-click the first table: appends SELECT TOP/LIMIT
  sleep 1
  xdotool key F5
  shot 09-schema-quick-query 3
  set -e
fi

pkill -f GoQueryOne-x64.exe || true
echo "smoke-wine done: $OUT"
