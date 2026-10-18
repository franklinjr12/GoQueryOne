# GoQueryOne user guide

GoQueryOne is a portable SQL client for ODBC data sources on Windows. This
guide covers connecting, running SQL, working with results and fixing the
most common ODBC problems.

## 1. Pick the right exe

| Your ODBC driver / DSN is… | Use |
|---|---|
| 64-bit (configured in *ODBC Data Sources (64-bit)*) | `GoQueryOne-x64.exe` |
| 32-bit (configured in *ODBC Data Sources (32-bit)*, e.g. old Access/Excel Jet drivers) | `GoQueryOne-x86.exe` |

The About box (Help → About) shows the version and the architecture of the
running exe. **Tools → ODBC Administrator** opens the administrator that
matches the running exe (Ctrl+T); the x64 / x86 variants are also listed.

## 2. Connect

Open **Tools → Connect…** (Ctrl+Shift+C or the green toolbar button).

| Mode | What to enter |
|---|---|
| DSN | a system or user DSN name |
| Driver | the exact driver name, e.g. `ODBC Driver 18 for SQL Server`, plus server/database in the connection string |
| File DSN | the path of a `.dsn` file |
| Connection string | the full ODBC string, e.g. `Driver={SQLite3 ODBC Driver};Database=C:\data\demo.db` |

- **Username / Password** are added to the connection string as `UID` / `PWD`.
  Values that contain `;`, `{`, `}` or `=` are escaped automatically.
- **Save password securely** stores the password encrypted with Windows DPAPI
  (only your Windows user can decrypt it). Without it the password is never
  written to disk.
- **Test** opens and closes a connection; **Save profile** stores the
  connection for next time; **Connect** opens it.
- Saved profiles are listed in the *Profile* box; the first one is
  preselected.

The active connection is shown in the toolbar box and in the status bar.
**Disconnect** closes it.

## 3. Run SQL

| Key | Action |
|---|---|
| F5 or Ctrl+Enter | Run the statement under the caret. The statement is highlighted so you can see exactly what ran. If text is selected, only the selection runs. |
| F6 | Run the whole script (statements separated by `;`) |
| Esc | Cancel the running statement |
| F8 | Refresh the schema |
| Ctrl+F / F3 | Find text in the editor (case-insensitive) / find the next match, wrapping at the end |
| F9 | Toggle the results between grid and text view |
| F1 | Open this guide |
| Ctrl+N / Ctrl+O / Ctrl+S / Ctrl+W | New tab / open file / save / close tab |
| Ctrl+Q | Exit |

Every statement runs **exactly once**. Statements that return rows (`SELECT`,
`WITH`, `SHOW`, `EXPLAIN`, `DESCRIBE`, `PRAGMA`, `VALUES`, `EXEC`, `CALL`) go
to the Results tab; other statements (`INSERT`, `UPDATE`, `CREATE`, …) report
the number of rows affected on the Messages tab. Errors go to the Errors tab
with the ODBC SQLSTATE and native error code.

`;` inside quotes, `[brackets]`, `"identifiers"` and comments (`--`, `/* */`)
does not split statements.

### Transactions

By default every statement is committed immediately (*Autocommit* in the
status bar). **Query → Begin transaction** switches to a manual transaction;
use **Commit** or **Rollback** to finish it. The status bar shows
*Transaction* while one is open.

### Large results

Results are capped at the configured maximum number of rows. **Query → Load
more rows** re-runs the query (only for row-returning statements) and fetches
the next page.

## 4. Work with results

- `(null)` in grey is a database NULL. The text `'NULL'` is shown as `NULL`.
- Select rows with click, Shift+click, Ctrl+click or Ctrl+A; **Ctrl+C**
  copies them as tab-separated text that pastes into Excel.
- Click a column header to sort the results (numeric columns sort as numbers,
  `(null)` first); click it again for descending order and click `#` to get
  the original order back. Copy and export use the order on screen.
- **Edit** menu: copy selected rows (with or without headers), copy all as
  TSV or CSV.
- **File → Export results…** writes CSV, TSV, JSON, Markdown or plain text.
  NULL becomes an empty field in CSV/TSV and `null` in JSON.

## 5. Files and settings

SQL tabs can be saved as `.sql` files. Closing a modified tab or exiting asks
whether to save. **File → Recent files** lists the last 10 files you opened or
saved; choosing one that is already open switches to its tab. Settings and saved profiles live in `config.json`:

- next to the exe if `portable.flag` exists there or the folder is writable
  (portable mode, e.g. on a USB stick);
- otherwise in `%APPDATA%\GoQueryOne\`;
- or wherever the `GOQUERYONE_CONFIG` environment variable points.

## 6. Troubleshooting

| Message | Cause and fix |
|---|---|
| `IM014` *architecture mismatch between the Driver and Application* | The DSN's driver has the other bitness. Use the other exe (x86 vs x64) or recreate the DSN in the matching ODBC Administrator. |
| `IM002` *Data source name not found and no default driver specified* | The DSN does not exist for this bitness or this user. Check *ODBC Data Sources (32-bit/64-bit)*, user vs system DSN. |
| `28000` / login failed | Wrong username or password, or the server does not allow that authentication mode. |
| Garbled accents | Use the driver's Unicode variant (e.g. *PostgreSQL Unicode*, *MySQL ODBC 8.x Unicode Driver*) instead of the ANSI one. |
| Windows SmartScreen warns on first start | The exe is not code-signed. Choose *More info → Run anyway* if you got it from a trusted source. |
| Nothing happens on F5 | The editor must have the focus; click into the SQL text first. |
| "The settings file could not be read" at start | `config.json` was damaged (e.g. edited by hand). It was kept as `config.json.broken-<date-time>`; copy your connections back from it. |
| GoQueryOne closed with "unexpected error" | Details are in `crash.log` next to `config.json`; please attach it (and `logs.txt`) to a bug report. |

## Query history

The History tab lists the last 50 statements of the session (passwords are masked).
To keep history between sessions, enable **Tools > Settings > Save query history between
sessions**. Entries are then stored in `history.jsonl` next to `config.json` (file mode
private to your user, capped at 1000 entries) and reloaded when you connect with the same
profile. Delete that file to clear it.

## Connection profiles

- **Tools > Connect to** lists saved profiles and connects in one step (the dialog opens only
  when a password must be typed).
- **New / Duplicate / Delete** next to the profile picker manage saved profiles. A duplicate
  never shares the saved password; deleting a profile also removes its saved password.
- **Driver** mode takes extra attributes (for example `Server=db1;Database=sales;`).
- **Connection string** mode has an *Insert template…* list (SQL Server, PostgreSQL, MySQL,
  Oracle, Db2, Access, Excel, SQLite). Replace every `<placeholder>`; Test/Connect refuse a
  string that still contains one. Username and password come from their own fields.
- **File DSN** mode has a Browse… button.
- The dialog title shows whether the exe uses 32-bit or 64-bit ODBC drivers and DSNs.
- Test shows its result under the buttons instead of in a message box.

## Settings and what is remembered

**Tools > Settings…** sets the statement timeout, maximum rows fetched, rows added by *Load
more*, editor font size, whether system tables appear in the schema tree, stop-on-error for
scripts and opt-in history saving.

On exit GoQueryOne remembers, in `config.json`: the window position and size (maximized
too), the splitter positions, and the SQL files open in tabs (reopened at startup; untitled
tabs are not kept, so save them first).
