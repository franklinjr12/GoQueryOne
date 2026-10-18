// Package app holds the GUI-independent presentation logic of GoQueryOne:
// which actions are enabled, what the status bar and window title say, and
// which output tab a statement result should bring to front. The walk view
// (internal/ui, Windows only) renders this state; everything here is pure Go
// and unit-tested on any OS.
package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/franklinjr12/GoQueryOne/internal/core"
)

// State is a snapshot of everything the presenter needs to derive the UI.
type State struct {
	// Connection
	Connected      bool   // an active session exists and is connected
	ConnectionName string // active session/profile name
	DBMS           string // DBMS name/version when known (optional)
	SessionCount   int
	TxOpen         bool
	Executing      bool
	ExecutingSince time.Time

	// Editor
	TabCount     int
	TabTitle     string
	TabDirty     bool
	TabHasPath   bool
	EditorHasSQL bool
	Line, Col    int // 1-based caret position; 0 = unknown

	// Results
	HasResultRows bool // a result grid with columns is loaded
	RowsSelected  int
	CanLoadMore   bool // last result truncated and was a query

	// Last statement outcome
	Last *Outcome
}

// Outcome summarises the last executed statement.
type Outcome struct {
	HasRows      bool
	Rows         int
	RowsAffected int64 // -1 = unknown
	Duration     time.Duration
	Truncated    bool
	TruncatedAt  int
	Err          string
	Canceled     bool
	TimedOut     bool
	Statement    string
	At           time.Time
}

// Actions is the enablement of every user command.
type Actions struct {
	Connect, Disconnect                   bool
	NewTab, Open, Save, SaveAs, CloseTab  bool
	Run, RunScript, Cancel, LoadMore      bool
	Begin, Commit, Rollback               bool
	RefreshSchema                         bool
	CopyRows, CopyAll, Export, ToggleText bool
	ODBCAdmin, About, Help                bool
}

// ComputeActions applies the enablement rules (plan §5.3): Run/Script/Refresh/
// Disconnect need a connection; Cancel only while executing; Commit/Rollback
// only with an open transaction; Copy/Export only with results. Nothing that
// talks to the database is enabled while a statement is executing.
func ComputeActions(s State) Actions {
	idle := !s.Executing
	ready := s.Connected && idle
	return Actions{
		Connect:       idle,
		Disconnect:    s.Connected && idle,
		NewTab:        true,
		Open:          true,
		Save:          s.TabCount > 0,
		SaveAs:        s.TabCount > 0,
		CloseTab:      s.TabCount > 0,
		Run:           ready && s.EditorHasSQL,
		RunScript:     ready && s.EditorHasSQL,
		Cancel:        s.Connected && s.Executing,
		LoadMore:      ready && s.CanLoadMore,
		Begin:         ready && !s.TxOpen,
		Commit:        ready && s.TxOpen,
		Rollback:      ready && s.TxOpen,
		RefreshSchema: ready,
		CopyRows:      s.HasResultRows,
		CopyAll:       s.HasResultRows,
		Export:        s.HasResultRows && idle,
		ToggleText:    s.HasResultRows,
		ODBCAdmin:     true,
		About:         true,
		Help:          true,
	}
}

// Status bar section indexes.
const (
	SectionConnection = iota
	SectionTransaction
	SectionActivity
	SectionResult
	SectionCaret
	sectionCount
)

// StatusSections returns the texts of the status bar sections.
func StatusSections(s State, now time.Time) []string {
	out := make([]string, sectionCount)
	switch {
	case !s.Connected:
		out[SectionConnection] = "Not connected"
	case s.DBMS != "":
		out[SectionConnection] = s.ConnectionName + " · " + s.DBMS
	default:
		out[SectionConnection] = s.ConnectionName
	}
	if s.Connected && s.SessionCount > 1 {
		out[SectionConnection] += fmt.Sprintf(" (+%d)", s.SessionCount-1)
	}
	if s.TxOpen {
		out[SectionTransaction] = "TX open"
	} else if s.Connected {
		out[SectionTransaction] = "Autocommit"
	}
	if s.Executing {
		out[SectionActivity] = "Executing"
		if !s.ExecutingSince.IsZero() {
			out[SectionActivity] += " " + core.FormatDuration(now.Sub(s.ExecutingSince).Truncate(100*time.Millisecond))
		}
	} else {
		out[SectionActivity] = "Ready"
	}
	out[SectionResult] = OutcomeSummary(s.Last)
	if s.Line > 0 {
		out[SectionCaret] = fmt.Sprintf("Ln %d, Col %d", s.Line, s.Col)
	}
	return out
}

// OutcomeSummary is a one-line description of the last statement outcome,
// e.g. "42 rows · 12 ms (truncated at 10,000)" or "3 rows affected · 4 ms".
func OutcomeSummary(o *Outcome) string {
	if o == nil {
		return ""
	}
	d := core.FormatDuration(o.Duration)
	switch {
	case o.Canceled:
		return "Canceled after " + d
	case o.TimedOut:
		return "Timed out after " + d
	case o.Err != "":
		return "Error after " + d
	case o.HasRows:
		s := fmt.Sprintf("%s %s · %s", FormatCount(int64(o.Rows)), plural(int64(o.Rows), "row", "rows"), d)
		if o.Truncated {
			s += fmt.Sprintf(" (truncated at %s)", FormatCount(int64(o.TruncatedAt)))
		}
		return s
	case o.RowsAffected >= 0:
		return fmt.Sprintf("%s %s affected · %s", FormatCount(o.RowsAffected), plural(o.RowsAffected, "row", "rows"), d)
	default:
		return "Command executed · " + d
	}
}

// WindowTitle returns "GoQueryOne — <connection> — <tab>[*]".
func WindowTitle(s State) string {
	parts := []string{"GoQueryOne"}
	if s.Connected && s.ConnectionName != "" {
		parts = append(parts, s.ConnectionName)
	}
	if s.TabTitle != "" {
		t := s.TabTitle
		if s.TabDirty {
			t += " *"
		}
		parts = append(parts, t)
	}
	return strings.Join(parts, " — ")
}

// ResultTab identifies the output tab to bring to front after execution.
type ResultTab int

const (
	TabResults ResultTab = iota
	TabMessages
	TabErrors
	TabHistory
)

// ResultTabFor picks Results for row sets, Messages for commands and Errors
// for failures (plan §5.11).
func ResultTabFor(o Outcome) ResultTab {
	switch {
	case o.Err != "" || o.Canceled || o.TimedOut:
		return TabErrors
	case o.HasRows:
		return TabResults
	default:
		return TabMessages
	}
}

// MessageFor returns the timestamped Messages-tab line for an outcome. A
// failure never produces a "completed" line.
func MessageFor(o Outcome) string {
	ts := o.At
	if ts.IsZero() {
		ts = time.Now()
	}
	prefix := ts.Format("15:04:05") + "  "
	d := core.FormatDuration(o.Duration)
	switch {
	case o.Canceled:
		return prefix + "Canceled by user after " + d + "."
	case o.TimedOut:
		return prefix + "Timed out after " + d + "."
	case o.Err != "":
		return prefix + "Error after " + d + ": " + firstLine(o.Err)
	case o.HasRows:
		s := fmt.Sprintf("%s%s %s retrieved in %s.", prefix, FormatCount(int64(o.Rows)), plural(int64(o.Rows), "row", "rows"), d)
		if o.Truncated {
			s += fmt.Sprintf(" Truncated at %s rows (use Load More).", FormatCount(int64(o.TruncatedAt)))
		}
		return s
	case o.RowsAffected >= 0:
		return fmt.Sprintf("%sCommand executed in %s. %s %s affected.", prefix, d, FormatCount(o.RowsAffected), plural(o.RowsAffected, "row", "rows"))
	default:
		return prefix + "Command executed in " + d + "."
	}
}

// FormatCount formats an integer with thousands separators ("10,000").
func FormatCount(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func plural(n int64, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// LineCol returns the 1-based line and column of a byte offset in text,
// counting columns in runes. CRLF and LF both end a line.
func LineCol(text string, byteOffset int) (line, col int) {
	if byteOffset < 0 {
		byteOffset = 0
	}
	if byteOffset > len(text) {
		byteOffset = len(text)
	}
	line, col = 1, 1
	for _, r := range text[:byteOffset] {
		switch r {
		case '\n':
			line++
			col = 1
		case '\r':
			// part of CRLF; not a column
		default:
			col++
		}
	}
	return line, col
}

// View is the rendering surface the walk UI implements.
type View interface {
	SetActionsEnabled(Actions)
	SetStatus(sections []string)
	SetTitle(title string)
}

// Presenter pushes derived state to a View.
type Presenter struct {
	view View
	now  func() time.Time
	last State
}

// NewPresenter creates a presenter bound to view.
func NewPresenter(view View) *Presenter {
	return &Presenter{view: view, now: time.Now}
}

// Update renders state s to the view.
func (p *Presenter) Update(s State) {
	p.last = s
	if p.view == nil {
		return
	}
	p.view.SetActionsEnabled(ComputeActions(s))
	p.view.SetStatus(StatusSections(s, p.now()))
	p.view.SetTitle(WindowTitle(s))
}

// State returns the last rendered state.
func (p *Presenter) State() State { return p.last }
