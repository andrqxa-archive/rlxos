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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/identity"
	"avyos.dev/pkg/sutra"
)

// session manages the lifecycle of a user session: spawning cmd/session
// as the user and waiting for it to exit naturally.
type session struct {
	id      *identity.Identity
	service *sutra.Service
	cmd     *exec.Cmd
	mu      sync.Mutex
}

func newSession(id *identity.Identity, svc *sutra.Service) *session {
	return &session{
		id:      id,
		service: svc,
	}
}

// pid returns the PID of the session process, or 0 if not running.
func (s *session) pid() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Pid
	}
	return 0
}

// running returns true if the session process is still alive.
func (s *session) running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	// Check if process is still alive
	err := s.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// run starts cmd/session as the user and blocks until it exits naturally.
func (s *session) run() {
	uid := uint32(s.id.ID)
	gid := uint32(s.id.ID)

	// Ensure home directory exists and is owned by user
	if err := os.MkdirAll(s.id.Home, 0750); err != nil {
		serviceLog.Error("failed to create home %s: %v", s.id.Home, err)
	}
	_ = os.Chown(s.id.Home, int(s.id.ID), int(s.id.ID))

	if err := ensureUserRuntimeDir(uid, gid); err != nil {
		serviceLog.Warn("failed to setup user runtime dir: %v", err)
	}

	// Build per-session environment (don't pollute global env)
	env := s.sessionEnv()

	sessionCmd, err := s.startCmd("session", uid, gid, env)
	if err != nil {
		serviceLog.Error("failed to start session: %v", err)
		return
	}

	s.mu.Lock()
	s.cmd = sessionCmd
	s.mu.Unlock()

	// Block until cmd/session exits (all critical desktop apps closed)
	if err := sessionCmd.Wait(); err != nil {
		serviceLog.Error("session process exited: %v", err)
	} else {
		serviceLog.Info("session process exited cleanly")
	}

	s.mu.Lock()
	s.cmd = nil
	s.mu.Unlock()
}

// stop terminates the full session process group and waits for exit.
func (s *session) stop(timeout time.Duration) error {
	pid := s.pid()
	if pid == 0 {
		return nil
	}

	if err := signalProcessGroup(pid, syscall.SIGTERM); err != nil {
		serviceLog.Warn("failed to signal session process group pid=%d: %v", pid, err)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !s.running() {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := signalProcessGroup(pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("force-kill session process group pid=%d: %w", pid, err)
	}

	killDeadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(killDeadline) {
		if !s.running() {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("session pid=%d did not stop", pid)
}

// sessionEnv builds the environment for this user's session.
func (s *session) sessionEnv() []string {
	// Start from a clean base, preserving only essential system vars
	env := []string{
		"HOME=" + s.id.Home,
		"USER=" + s.id.Name,
		"LOGNAME=" + s.id.Name,
		"SHELL=" + s.id.Shell,
		"AVYOS_SESSION_PID=" + strconv.Itoa(os.Getpid()),
		"AVYOS_SESSION_ID=" + strconv.FormatUint(uint64(s.id.ID), 10),
	}
	// Carry over system-level environment
	for _, e := range os.Environ() {
		key := e
		if idx := indexOf(e, '='); idx >= 0 {
			key = e[:idx]
		}
		switch key {
		case "HOME", "USER", "LOGNAME", "SHELL", "AVYOS_SESSION_PID", "AVYOS_SESSION_ID":
			continue // already set above
		default:
			env = append(env, e)
		}
	}
	return env
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// startCmd spawns a cmd/ binary as the given user with the given environment.
func (s *session) startCmd(name string, uid, gid uint32, env []string, args ...string) (*exec.Cmd, error) {
	path := resolveCmd(name)
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = s.id.Home
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uid, Gid: gid},
		Setpgid:    true,
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s (%s): %w", name, path, err)
	}

	serviceLog.Info("started %s pid=%d uid=%d", name, cmd.Process.Pid, uid)
	return cmd, nil
}

func resolveCmd(name string) string {
	candidates := []string{
		filepath.Join(fs.AvyosPath, "cmd", name),
		filepath.Join("/cmd", name),
		filepath.Join("cmd", name),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(fs.AvyosPath, "cmd", name)
}

func ensureUserRuntimeDir(uid, gid uint32) error {
	base := filepath.Join(fs.RuntimePath, "user")
	if err := os.MkdirAll(base, 0755); err != nil {
		return err
	}

	userDir := filepath.Join(base, strconv.FormatUint(uint64(uid), 10))
	if err := os.MkdirAll(userDir, 0700); err != nil {
		return err
	}
	if err := os.Chown(userDir, int(uid), int(gid)); err != nil {
		return err
	}
	if err := os.Chmod(userDir, 0700); err != nil {
		return err
	}
	return nil
}

func signalProcessGroup(pid int, sig syscall.Signal) error {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return err
	}
	if err := syscall.Kill(-pgid, sig); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}
