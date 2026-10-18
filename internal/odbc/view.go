package odbc

// SessionView is a consistent copy of the session fields the UI shows,
// taken under the Manager lock (the executor goroutine updates them).
type SessionView struct {
	ID, Name      string
	State         SessionState
	HasDB         bool
	TxOpen        bool
	LastResult    *StatementResult
	History       []HistoryEntry
	ConnectedInfo map[string]string
}

// View returns a snapshot of the session for display.
func (m *Manager) View(sessionID string) (SessionView, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return SessionView{}, false
	}
	v := SessionView{ID: s.ID, Name: s.Name, State: s.State, HasDB: s.DB != nil, TxOpen: s.Tx != nil}
	if s.LastResult != nil {
		r := *s.LastResult
		v.LastResult = &r
	}
	v.History = append([]HistoryEntry(nil), s.History...)
	if s.ConnectedInfo != nil {
		v.ConnectedInfo = make(map[string]string, len(s.ConnectedInfo))
		for k, val := range s.ConnectedInfo {
			v.ConnectedInfo[k] = val
		}
	}
	return v, true
}

// ActiveView is View for the active session.
func (m *Manager) ActiveView() (SessionView, bool) {
	m.mu.RLock()
	id := m.activeID
	m.mu.RUnlock()
	if id == "" {
		return SessionView{}, false
	}
	return m.View(id)
}
