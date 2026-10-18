// Package platform defines small seams between the pure application logic
// and OS-specific facilities (clipboard, file dialogs, the ODBC
// administrator applet) so that code depending on them can be unit tested
// with fakes instead of a real Windows UI. Production adapters are thin
// wrappers around the existing walk/odbc integrations; see adapter_windows.go.
package platform

// Clipboard reads and writes the system clipboard's text contents.
type Clipboard interface {
	SetText(text string) error
	Text() (string, error)
}

// OpenFileRequest describes a request to pick an existing file to open.
type OpenFileRequest struct {
	Title       string
	Filter      string
	FilterIndex int
	InitialPath string
}

// SaveFileRequest describes a request to pick a destination file to save.
type SaveFileRequest struct {
	Title       string
	Filter      string
	FilterIndex int
	InitialPath string
}

// FileDialogs shows native "open" and "save" file pickers. ok is false when
// the user cancels the dialog without choosing a path.
type FileDialogs interface {
	ShowOpen(req OpenFileRequest) (path string, ok bool, err error)
	ShowSave(req SaveFileRequest) (path string, ok bool, err error)
}

// ODBCAdmin launches the platform's ODBC Data Source Administrator applet
// for the given target architecture ("x64" or "x86"); the returned path is
// the executable that was launched (or attempted).
type ODBCAdmin interface {
	Open(targetArch string) (path string, err error)
}
