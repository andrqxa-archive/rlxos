/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	displayapi "avyos.dev/api/display"
	loginapi "avyos.dev/api/login"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/graphics"
	gapp "avyos.dev/pkg/graphics/app"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
	"avyos.dev/pkg/graphics/ui"
	"avyos.dev/pkg/identity"
	"avyos.dev/pkg/logger"
	"avyos.dev/pkg/sutra"
)

//go:embed ui/login.ui
var loginUI string

var serviceLog = logger.New("login")

// sessionManager tracks multiple concurrent user sessions.
type sessionManager struct {
	sessions map[uint32]*session // uid -> session
	active   uint32              // active session UID
	svc      *sutra.Service
	display  *displayapi.Client
	showCh   chan struct{} // signal to show login screen
	mu       sync.Mutex
}

func newSessionManager(svc *sutra.Service) *sessionManager {
	return &sessionManager{
		sessions: make(map[uint32]*session),
		svc:      svc,
		showCh:   make(chan struct{}, 1),
	}
}

func (m *sessionManager) connectDisplay() error {
	c, err := displayapi.NewClient("")
	if err != nil {
		return fmt.Errorf("connect display: %w", err)
	}
	m.display = c
	return nil
}

// sessionList returns info for all active sessions.
func (m *sessionManager) sessionList() []loginapi.SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]loginapi.SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, loginapi.SessionInfo{
			Username: s.id.Name,
			UID:      uint32(s.id.ID),
			PID:      s.pid(),
			Active:   uint32(s.id.ID) == m.active,
		})
	}
	return out
}

func (m *sessionManager) broadcastSessionList() {
	list := m.sessionList()
	_ = m.svc.Broadcast(loginapi.EventSessionListChanged, loginapi.EncodeSessionList(list))
}

// handleLogin processes a successful authentication.
func (m *sessionManager) handleLogin(id *identity.Identity) {
	uid := uint32(id.ID)

	m.mu.Lock()
	existing, exists := m.sessions[uid]
	m.mu.Unlock()

	if exists && existing.running() {
		// Re-attach to existing session
		serviceLog.Info("switching to existing session for %s (uid=%d)", id.Name, id.ID)
		m.switchTo(uid)
		return
	}

	// Create new session
	serviceLog.Info("creating new session for %s (uid=%d)", id.Name, id.ID)

	// Register with display server
	if m.display != nil {
		_, _ = m.display.RegisterSession(displayapi.RegisterSessionRequest{
			SessionID: uid,
			UID:       uid,
		})
	}

	sess := newSession(id, m.svc)
	m.mu.Lock()
	m.sessions[uid] = sess
	m.mu.Unlock()

	// Start session in background goroutine
	go func() {
		sess.run()

		// Session ended naturally (all apps exited)
		serviceLog.Info("session ended naturally for %s (uid=%d)", id.Name, id.ID)
		m.mu.Lock()
		delete(m.sessions, uid)
		wasActive := m.active == uid
		m.mu.Unlock()

		_ = m.svc.Broadcast(loginapi.EventSessionEnded, nil)
		m.broadcastSessionList()

		if wasActive {
			// Show login screen
			select {
			case m.showCh <- struct{}{}:
			default:
			}
		}
	}()

	m.switchTo(uid)

	info := loginapi.SessionInfo{
		Username: id.Name,
		UID:      uid,
		PID:      sess.pid(),
		Active:   true,
	}
	_ = m.svc.Broadcast(loginapi.EventSessionStarted, info.Encode())
	m.broadcastSessionList()
}

func (m *sessionManager) switchTo(uid uint32) {
	m.mu.Lock()
	m.active = uid
	m.mu.Unlock()

	if m.display != nil {
		_, _ = m.display.SetActiveSession(displayapi.SetActiveSessionRequest{
			SessionID: uid,
		})
	}
}

// requestShowLogin signals the main loop to show the login screen.
func (m *sessionManager) requestShowLogin() {
	select {
	case m.showCh <- struct{}{}:
	default:
	}
}

func (m *sessionManager) logoutActiveSession() error {
	m.mu.Lock()
	activeUID := m.active
	sess := m.sessions[activeUID]
	m.mu.Unlock()

	if sess == nil {
		return nil
	}

	serviceLog.Info("terminating active session uid=%d pid=%d", activeUID, sess.pid())
	if err := sess.stop(3 * time.Second); err != nil {
		return err
	}
	m.switchTo(0)
	return nil
}

func (m *sessionManager) registerHandlers() {
	m.svc.Handle(loginapi.RequestLogout, func(t *sutra.Transaction) ([]byte, error) {
		serviceLog.Info("logout requested via API")
		if err := m.logoutActiveSession(); err != nil {
			serviceLog.Warn("logout session termination failed: %v", err)
		}
		m.requestShowLogin()
		return nil, nil
	})

	m.svc.Handle(loginapi.RequestLock, func(t *sutra.Transaction) ([]byte, error) {
		serviceLog.Info("lock requested")
		m.requestShowLogin()
		return nil, nil
	})

	m.svc.Handle(loginapi.RequestGetSession, func(t *sutra.Transaction) ([]byte, error) {
		m.mu.Lock()
		activeUID := m.active
		sess := m.sessions[activeUID]
		m.mu.Unlock()
		if sess == nil {
			return nil, fmt.Errorf("no active session")
		}
		info := loginapi.SessionInfo{
			Username: sess.id.Name,
			UID:      uint32(sess.id.ID),
			PID:      sess.pid(),
			Active:   true,
		}
		return info.Encode(), nil
	})

	m.svc.Handle(loginapi.RequestListSessions, func(t *sutra.Transaction) ([]byte, error) {
		return loginapi.EncodeSessionList(m.sessionList()), nil
	})

	m.svc.Handle(loginapi.RequestSwitchUser, func(t *sutra.Transaction) ([]byte, error) {
		serviceLog.Info("switch user requested via API")
		m.requestShowLogin()
		return nil, nil
	})
}

func main() {
	if err := logger.SetupSystemLog(); err != nil {
		serviceLog.Error("failed to setup system log: %v", err)
	}

	startOOBE()

	// Create sutra service for session control API
	svc, err := sutra.NewService(loginapi.ServiceName, "")
	if err != nil {
		serviceLog.Error("failed to create service: %v", err)
		os.Exit(1)
	}
	defer svc.Close()

	mgr := newSessionManager(svc)

	// Connect to display service for session management
	if err := mgr.connectDisplay(); err != nil {
		serviceLog.Warn("could not connect to display service: %v", err)
	}

	mgr.registerHandlers()

	serviceLog.Info("starting login service")

	// Main loop: show login → handle authentication → wait for logout signal → repeat
	for {
		id, err := showLogin()
		if err != nil {
			serviceLog.Error("login UI error: %v", err)
			continue
		}
		if id == nil {
			continue
		}

		mgr.handleLogin(id)

		// Wait for logout/lock/switch signal
		<-mgr.showCh
	}
}

// showLogin displays the login UI and returns the authenticated identity.
func showLogin() (*identity.Identity, error) {
	backend := displaybackend.New()
	backend.SetLayer(
		displayapi.LayerOverlay,
		displayapi.AnchorBottom|displayapi.AnchorLeft|displayapi.AnchorRight|displayapi.AnchorTop,
		0,
	)

	app := &loginApp{}
	app.SetOptions(gapp.Options{
		Title:      "Login",
		Backend:    backend,
		Input:      backend,
		Background: graphics.DefaultTheme.Background,
	})

	if err := app.LoadString(loginUI, app); err != nil {
		return nil, fmt.Errorf("failed to load UI: %w", err)
	}

	if bg := app.e("BackgroundImage"); bg != nil {
		bg.SetAttribute("src", "/avyos/data/backgrounds/default_blur.png")
	}

	app.Configure(func(g *gapp.App) {
		g.OnEscape = app.Cancel
	})
	app.AddFocusable(app.e("UsernameInput"), app.e("PasswordInput"), app.e("SignInBtn"))
	app.Focus(app.e("UsernameInput"))

	if err := app.Run(); err != nil {
		return nil, err
	}

	return app.result, nil
}

type loginApp struct {
	ui.App
	result *identity.Identity
}

func (a *loginApp) e(id string) *ui.Element { return a.FindElement(id) }

func (a *loginApp) SignIn() {
	user := a.e("UsernameInput").Attr("text", "")
	pass := a.e("PasswordInput").Attr("text", "")

	id, err := identity.Authenticate(user, pass)
	if err != nil {
		a.e("StatusLabel").SetAttribute("text", fmt.Sprintf("Sign in failed: %v", err))
		a.e("StatusLabel").SetAttribute("textColor", "#ff6b6b")
		return
	}

	a.e("StatusLabel").SetAttribute("text", "Signing in...")
	a.e("StatusLabel").SetAttribute("textColor", "#3f5678")

	a.result = id
	a.Quit()
}

func (a *loginApp) SubmitFromEnter(_ string) { a.SignIn() }
func (a *loginApp) Cancel()                  { a.Quit() }

func startOOBE() {
	if fs.Exists("/config/.oobe-done") {
		return
	}
	cmd := exec.Command("/avyos/apps/oobe/exec")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}
