//go:build windows

package ui

import (
	"log"
	"time"

	"github.com/lxn/walk"

	"github.com/franklinjr12/GoQueryOne/internal/config"
	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

// refreshConnectToMenu lists the saved profiles under Tools > Connect to.
func (u *applicationUI) refreshConnectToMenu() {
	if u.actions == nil || u.actions.connectToMenu == nil {
		return
	}
	list := u.actions.connectToMenu.Actions()
	_ = list.Clear()
	if len(u.cfg.Connections) == 0 {
		empty := walk.NewAction()
		_ = empty.SetText("(no saved connections)")
		_ = empty.SetEnabled(false)
		_ = list.Add(empty)
		return
	}
	for _, p := range u.cfg.Connections {
		id := p.ID
		act := walk.NewAction()
		_ = act.SetText(p.Name)
		act.Triggered().Attach(func() { u.quickConnect(id) })
		if err := list.Add(act); err != nil {
			log.Printf("connect-to menu: %v", err)
		}
	}
}

// quickConnect connects a saved profile directly when no password has to
// be typed (saved password, or no username, e.g. Windows login); otherwise
// it opens the connection dialog with that profile selected.
func (u *applicationUI) quickConnect(profileID string) {
	p, ok := u.cfg.ConnectionByID(profileID)
	if !ok {
		return
	}
	// Already connected: just make it the active connection.
	if s, err := u.manager.GetSession(p.ID); err == nil && s.State == odbc.SessionConnected {
		_ = u.manager.SetActiveSession(p.ID)
		u.refreshSessionSelect()
		u.refreshState()
		return
	}
	password := ""
	if p.SavePassword && p.CredentialRef != "" && u.secrets != nil {
		if pw, err := u.secrets.Load(p.CredentialRef); err == nil {
			password = pw
		}
	}
	if p.Username != "" && password == "" {
		u.preselectProfileID = p.ID
		u.showConnectionWindow()
		return
	}
	u.beginExecuting("Connecting to " + p.Name + "…")
	go func(p config.ConnectionProfile) {
		timeout := time.Duration(p.Options.LoginTimeoutMs) * time.Millisecond
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		_, err := u.manager.Connect(p, password, timeout)
		u.mainWindow.Synchronize(func() {
			u.updateStatus()
			if err != nil {
				showErrorBox(u.mainWindow, "Connect to "+p.Name, err)
				return
			}
			u.refreshSessionSelect()
			u.loadSchema(false)
			if tab := u.currentEditorTab(); tab != nil {
				_ = tab.Editor.SetFocus()
			}
			u.refreshState()
		})
	}(p)
}
