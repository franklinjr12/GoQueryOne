package odbc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/alexbrainman/odbc"
	"github.com/franklinjr12/GoQueryOne/internal/config"
)

const maxHistoryEntries = 50

func NewManager() *Manager {
	return &Manager{
		sessions: map[string]*Session{},
	}
}

func (m *Manager) SessionList() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

func (m *Manager) ActiveSession() (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.activeID == "" {
		return nil, errors.New("no active session")
	}
	s, ok := m.sessions[m.activeID]
	if !ok {
		return nil, errors.New("active session not found")
	}
	return s, nil
}

func (m *Manager) SetActiveSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[id]; !ok {
		return errors.New("session not found")
	}
	m.activeID = id
	return nil
}

func (m *Manager) GetSession(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, errors.New("session not found")
	}
	return s, nil
}

func (m *Manager) Connect(profile config.ConnectionProfile, password string, timeout time.Duration) (*Session, error) {
	if strings.TrimSpace(profile.ID) == "" {
		return nil, errors.New("profile id is required")
	}
	if strings.TrimSpace(profile.Name) == "" {
		return nil, errors.New("profile name is required")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	m.mu.Lock()
	if _, exists := m.sessions[profile.ID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("session %s is already open", profile.Name)
	}
	session := &Session{
		ID:          profile.ID,
		Name:        profile.Name,
		Profile:     profile,
		State:       SessionConnecting,
		CreatedAt:   time.Now(),
		SchemaCache: NewSchemaCache(),
	}
	m.sessions[session.ID] = session
	if m.activeID == "" {
		m.activeID = session.ID
	}
	m.mu.Unlock()

	dsn, err := BuildConnectionString(profile, password)
	if err != nil {
		m.setSessionError(session.ID, "connect", "", err)
		_ = m.Disconnect(session.ID)
		return nil, err
	}

	db, err := sql.Open("odbc", dsn)
	if err != nil {
		m.setSessionError(session.ID, "connect", "", err)
		_ = m.Disconnect(session.ID)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if pingErr := db.PingContext(ctx); pingErr != nil {
		_ = db.Close()
		m.setSessionError(session.ID, "connect", "", pingErr)
		_ = m.Disconnect(session.ID)
		return nil, pingErr
	}

	m.mu.Lock()
	session.DB = db
	session.State = SessionConnected
	session.ConnectedAt = time.Now()
	session.ConnectedInfo = map[string]string{
		"profileType": profile.Type,
		"dsn":         profile.DSN,
		"driver":      profile.Driver,
	}
	m.mu.Unlock()

	return session, nil
}

func (m *Manager) TestConnection(profile config.ConnectionProfile, password string, timeout time.Duration) (time.Duration, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dsn, err := BuildConnectionString(profile, password)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	db, err := sql.Open("odbc", dsn)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

func (m *Manager) Disconnect(sessionID string) error {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return errors.New("session not found")
	}
	delete(m.sessions, sessionID)
	if m.activeID == sessionID {
		m.activeID = ""
		for id := range m.sessions {
			m.activeID = id
			break
		}
	}
	m.mu.Unlock()

	if session.ExecCancel != nil {
		session.ExecCancel()
	}
	if session.Tx != nil {
		_ = session.Tx.Rollback()
	}
	if session.DB != nil {
		return session.DB.Close()
	}
	return nil
}

func (m *Manager) DisconnectAll() {
	sessions := m.SessionList()
	for _, s := range sessions {
		_ = m.Disconnect(s.ID)
	}
}

func BuildConnectionString(profile config.ConnectionProfile, password string) (string, error) {
	base := strings.TrimSpace(profile.ConnectionString)
	switch strings.ToLower(strings.TrimSpace(profile.Type)) {
	case "dsn":
		if strings.TrimSpace(profile.DSN) == "" {
			return "", errors.New("dsn is required")
		}
		base = "DSN=" + profile.DSN + ";"
	case "connection_string":
		if base == "" {
			return "", errors.New("connection string is required")
		}
	case "file_dsn":
		if strings.TrimSpace(profile.FilePath) == "" {
			return "", errors.New("file dsn path is required")
		}
		base = "FILEDSN=" + profile.FilePath + ";"
	case "driver":
		if strings.TrimSpace(profile.Driver) == "" {
			return "", errors.New("driver is required")
		}
		base = "Driver={" + profile.Driver + "};"
	default:
		return "", fmt.Errorf("unsupported connection profile type %q", profile.Type)
	}
	if profile.Username != "" && !containsConnKey(base, "uid") && !containsConnKey(base, "user id") {
		base += "UID=" + profile.Username + ";"
	}
	if password != "" && !containsConnKey(base, "pwd") && !containsConnKey(base, "password") {
		base += "PWD=" + password + ";"
	}
	return base, nil
}

func containsConnKey(connStr, key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(connStr, " ", ""))
	key = strings.ToLower(strings.ReplaceAll(key, " ", ""))
	return strings.Contains(normalized, key+"=")
}

func (m *Manager) setSessionError(sessionID, op, sqlText string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[sessionID]
	if !ok {
		return
	}
	session.State = SessionError
	session.LastDiag = BuildDiagnostic(op, sqlText, err)
}

func (m *Manager) BeginTx(sessionID string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}
	if session.DB == nil {
		return errors.New("session has no active database handle")
	}
	if session.Tx != nil {
		return errors.New("transaction already active")
	}
	tx, err := session.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	m.mu.Lock()
	session.Tx = tx
	m.mu.Unlock()
	return nil
}

func (m *Manager) CommitTx(sessionID string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}
	if session.Tx == nil {
		return errors.New("no active transaction")
	}
	err = session.Tx.Commit()
	m.mu.Lock()
	session.Tx = nil
	m.mu.Unlock()
	return err
}

func (m *Manager) RollbackTx(sessionID string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}
	if session.Tx == nil {
		return errors.New("no active transaction")
	}
	err = session.Tx.Rollback()
	m.mu.Lock()
	session.Tx = nil
	m.mu.Unlock()
	return err
}

func (m *Manager) addHistory(session *Session, sqlText, status string, duration time.Duration) {
	entry := HistoryEntry{
		When:     time.Now(),
		SQL:      MaskSecrets(sqlText),
		Status:   status,
		Duration: duration,
	}
	history := append(session.History, entry)
	if len(history) > maxHistoryEntries {
		history = history[len(history)-maxHistoryEntries:]
	}
	session.History = history
}

func (m *Manager) CancelExecution(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[sessionID]
	if !ok {
		return errors.New("session not found")
	}
	if session.ExecCancel == nil {
		return errors.New("no running execution")
	}
	session.ExecCancel()
	return nil
}
