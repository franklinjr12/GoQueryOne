# **Backlog – Go SQL Client(GoQueryOne)**

---

## **Epic 1 – Project Setup**

1. **Create project structure**

   * Initialize Go modules (`go mod init`).
   * Define folder structure (`/cmd`, `/internal`, `/ui`, `/config`).
   * `cmd` will be the entry point of the application.
   * `internal` will contain the all the application logic and packages used only for the application.
   * `ui` will contain the UI of the application.
   * `config` will contain the configuration of the application which will be plain json format.
2. **Dependency management**

   * Will use Fyne for UI and backend will be in Go.
   * Using alexbrainman/odbc for ODBC driver.
3. **Basic build system**

   * Setup `Makefile` or scripts for build/run/test.
   * Ensure Windows compilation (cross-compile if needed).
4. **Versioning & repository**

   * Setup Git repo.
   * Setup semantic versioning.
   * Add LICENSE + README.

---

## **Epic 2 – Database Connectivity (ODBC only, MVP)**

1. **ODBC driver integration**

   * Using alexbrainman/odbc for ODBC driver.
   * Will add support for different drivers later.
   * Test connecting to a known database.
2. **Connection manager**

   * Input for DSN, username, password.
   * Test “Connect” button.
   * Store connection session in memory.
3. **Connection error handling**

   * Show meaningful errors when connection fails.
   * Graceful disconnection handling.
4. **Basic disconnection**

   * Implement “Disconnect” button.
   * Free resources on disconnect.

---

## **Epic 3 – Query Execution**

1. **SQL editor**

   * Text box for writing SQL queries.
   * Support multi-line input.
   * Basic syntax highlighting (optional).
2. **Run query**

   * Button “Run SQL”.
   * Send query to ODBC connection.
   * Retrieve results.
3. **Run script**

   * Allow running multi-statement scripts.
4. **Error handling**

   * Display SQL errors in output window.

---

## **Epic 4 – Results Display**

1. **Result table grid**

   * Display results in a table format.
   * Support scrollable grid.
2. **Basic formatting**

   * Show column names + row data.
   * Handle large result sets (pagination or virtualized table).
3. **Clipboard export**

   * Copy selected rows.
   * Copy all rows.
4. **Row count + execution time**

   * Show footer: “Query executed successfully, N rows retrieved in X ms”.

---

## **Epic 5 – Schema Browser (simplified)**

1. **Schema tree view**

   * Display database tables (retrieved from ODBC).
   * Expand table to show columns.
2. **Refresh schema**

   * Button to reload schema.
3. **Table inspection**

   * Double click table name → auto-generate `SELECT * FROM table`.

---

## **Epic 6 – User Interface (Simplified Desktop UI)**

1. **Main layout**

   * Split screen: left (schema tree), right (query editor + results).
   * Top menu (File, Edit, Help).
2. **Toolbar**

   * Buttons: Connect, Disconnect, Run SQL, Run Script.
3. **Tabs support (optional MVP feature)**

   * Multiple query result tabs.
   * Close tab button.
4. **Window settings**

   * Remember last window size & position (save to config file).

---

## **Epic 7 – Configuration & Persistence**

1. **Config file**

   * Store saved connections (DSN, user, etc).
   * Store app preferences (theme, window size).
2. **Connection history**

   * List of previously used DSNs.
3. **Basic security**

   * Optionally don’t save passwords, or encrypt locally.

---

## **Epic 8 – Quality & Testing**

1. **Unit tests**

   * Connection handling.
   * Query execution.
2. **Integration tests**

   * Test with local SQLite ODBC / MySQL ODBC.
3. **Manual QA checklist**

   * Connect/disconnect, run queries, schema refresh, error handling.
4. **Basic logging**

   * Write logs to file (errors, queries executed).

---

## **Epic 9 – Packaging & Distribution**

1. **Windows executable build**

   * Compile `.exe` with resources (icon).
2. **Installer (optional for v1.0)**

   * Simple installer using NSIS/InnoSetup.
3. **Portable mode**

   * Single `.exe` with config in same folder.
4. **Version info**

   * Embed version + build date.

---

## **Epic 10 – Documentation**

1. **User Guide**

   * How to connect.
   * How to run queries.
   * Export results.
2. **Developer Guide**

   * How to build from source.
   * Contribution guidelines.
3. **FAQ / Troubleshooting**

   * Common ODBC issues.

---

### **Future (post-MVP) Backlog Ideas**

* Cross-platform support (Linux, macOS).
* Non-ODBC connections (Postgres, MySQL native).
* Dark mode / theming.
* Syntax highlighting & autocomplete in SQL editor.
* Query history.
* Export to CSV/XLSX.
* Multiple concurrent connections.
* Result filtering & sorting.
