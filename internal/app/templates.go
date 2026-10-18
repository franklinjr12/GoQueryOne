package app

// ConnTemplate is a starting point for a connection string; the user
// replaces the <placeholders> (core.UnfilledPlaceholders finds the ones
// left). UID/PWD come from the Username/Password fields.
type ConnTemplate struct {
	Name     string
	Template string
}

// ConnTemplates lists common ODBC connection-string templates.
var ConnTemplates = []ConnTemplate{
	{"SQL Server (ODBC Driver 18)", "Driver={ODBC Driver 18 for SQL Server};Server=<server>;Database=<database>;Encrypt=yes;TrustServerCertificate=no;"},
	{"SQL Server (ODBC Driver 17)", "Driver={ODBC Driver 17 for SQL Server};Server=<server>;Database=<database>;"},
	{"SQL Server - Windows login", "Driver={ODBC Driver 18 for SQL Server};Server=<server>;Database=<database>;Trusted_Connection=yes;Encrypt=yes;"},
	{"PostgreSQL (psqlODBC)", "Driver={PostgreSQL Unicode(x64)};Server=<server>;Port=5432;Database=<database>;"},
	{"MySQL (Connector/ODBC 8)", "Driver={MySQL ODBC 8.0 Unicode Driver};Server=<server>;Port=3306;Database=<database>;"},
	{"Oracle (Instant Client)", "Driver={Oracle in instantclient_21};DBQ=<host>:1521/<service>;"},
	{"IBM Db2", "Driver={IBM DB2 ODBC DRIVER};Hostname=<host>;Port=50000;Database=<database>;Protocol=TCPIP;"},
	{"Access (.accdb/.mdb)", "Driver={Microsoft Access Driver (*.mdb, *.accdb)};DBQ=<file path>;"},
	{"Excel (.xlsx)", "Driver={Microsoft Excel Driver (*.xls, *.xlsx, *.xlsm, *.xlsb)};DBQ=<file path>;ReadOnly=1;"},
	{"SQLite (SQLite3 ODBC)", "Driver={SQLite3 ODBC Driver};Database=<file path>;"},
}

// ConnTemplateNames returns the template names in display order, prefixed
// with an empty "choose" entry.
func ConnTemplateNames() []string {
	out := []string{"Insert template…"}
	for _, t := range ConnTemplates {
		out = append(out, t.Name)
	}
	return out
}
