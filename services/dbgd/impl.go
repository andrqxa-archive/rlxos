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
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	dbgdapi "avyos.dev/api/dbgd"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/identity"
)

const (
	defaultMaxOutputBytes = 24 * 1024
	defaultFileChunkBytes = 32 * 1024
)

type authSession struct {
	Token     string
	ClientID  uint32
	Identity  *identity.Identity
	CreatedAt time.Time
}

type Handler struct {
	maxOutput int

	sessionMu sync.RWMutex
	sessions  map[string]*authSession
}

func NewHandler(maxOutput int) *Handler {
	if maxOutput <= 0 {
		maxOutput = defaultMaxOutputBytes
	}
	return &Handler{
		maxOutput: maxOutput,
		sessions:  make(map[string]*authSession),
	}
}

func (h *Handler) Authenticate(sender uint32, req dbgdapi.AuthRequest) (dbgdapi.SessionInfo, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = "admin"
	}

	id, err := identity.Authenticate(username, req.Password)
	if err != nil {
		return dbgdapi.SessionInfo{}, fmt.Errorf("authentication failed: %w", err)
	}

	token, err := randomToken()
	if err != nil {
		return dbgdapi.SessionInfo{}, err
	}

	sess := &authSession{
		Token:     token,
		ClientID:  sender,
		Identity:  id,
		CreatedAt: time.Now(),
	}

	h.sessionMu.Lock()
	h.sessions[token] = sess
	h.sessionMu.Unlock()

	serviceLog.Info("authenticated user=%s uid=%d client=%d", id.Name, id.ID, sender)

	return dbgdapi.SessionInfo{
		Token:    token,
		Username: id.Name,
		UID:      uint32(id.ID),
		GID:      uint32(id.ID),
		Home:     id.Home,
		Shell:    id.Shell,
	}, nil
}

func (h *Handler) Logout(sender uint32, req dbgdapi.SessionToken) (dbgdapi.Empty, error) {
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return dbgdapi.Empty{}, nil
	}

	h.sessionMu.Lock()
	sess, ok := h.sessions[token]
	if ok {
		if sess.ClientID != sender {
			h.sessionMu.Unlock()
			return dbgdapi.Empty{}, errors.New("session token does not belong to this client")
		}
		delete(h.sessions, token)
	}
	h.sessionMu.Unlock()

	if ok {
		serviceLog.Info("session closed for user=%s client=%d", sess.Identity.Name, sender)
	}

	return dbgdapi.Empty{}, nil
}

func (h *Handler) RunCommand(sender uint32, req dbgdapi.ExecRequest) (dbgdapi.ExecResult, error) {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return dbgdapi.ExecResult{}, err
	}
	return h.execute(sess, req, false)
}

func (h *Handler) RunShell(sender uint32, req dbgdapi.ExecRequest) (dbgdapi.ExecResult, error) {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return dbgdapi.ExecResult{}, err
	}
	return h.execute(sess, req, true)
}

func (h *Handler) ReadFile(sender uint32, req dbgdapi.ReadFileRequest) (dbgdapi.FileChunk, error) {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return dbgdapi.FileChunk{}, err
	}

	path := strings.TrimSpace(req.Path)
	if path == "" {
		return dbgdapi.FileChunk{}, errors.New("read path is required")
	}
	path = filepath.Clean(path)

	size := int(req.Size)
	if size <= 0 || size > defaultFileChunkBytes {
		size = defaultFileChunkBytes
	}

	data, err := h.runHelper(sess, "read", path, req.Offset, uint32(size), false, 0, nil)
	if err != nil {
		return dbgdapi.FileChunk{}, err
	}

	return dbgdapi.FileChunk{
		Data: data,
		Eof:  len(data) < size,
	}, nil
}

func (h *Handler) WriteFile(sender uint32, req dbgdapi.WriteFileRequest) (dbgdapi.WriteFileResult, error) {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return dbgdapi.WriteFileResult{}, err
	}

	path := strings.TrimSpace(req.Path)
	if path == "" {
		return dbgdapi.WriteFileResult{}, errors.New("write path is required")
	}
	path = filepath.Clean(path)

	if len(req.Data) > defaultFileChunkBytes {
		return dbgdapi.WriteFileResult{}, fmt.Errorf("write chunk exceeds %d bytes", defaultFileChunkBytes)
	}

	mode := req.Mode
	if mode == 0 {
		mode = 0644
	}

	stdout, err := h.runHelper(sess, "write", path, req.Offset, uint32(len(req.Data)), req.Truncate, mode, req.Data)
	if err != nil {
		return dbgdapi.WriteFileResult{}, err
	}

	written := len(req.Data)
	if s := strings.TrimSpace(string(stdout)); s != "" {
		if n, parseErr := strconv.Atoi(s); parseErr == nil && n >= 0 {
			written = n
		}
	}

	return dbgdapi.WriteFileResult{Written: uint32(written)}, nil
}

func (h *Handler) DropClientSessions(clientID uint32) {
	h.sessionMu.Lock()
	defer h.sessionMu.Unlock()

	for token, sess := range h.sessions {
		if sess.ClientID != clientID {
			continue
		}
		serviceLog.Info("dropping session user=%s client=%d", sess.Identity.Name, clientID)
		delete(h.sessions, token)
	}
}

func (h *Handler) sessionFor(sender uint32, token string) (*authSession, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("session token is required")
	}

	h.sessionMu.RLock()
	sess := h.sessions[token]
	h.sessionMu.RUnlock()
	if sess == nil {
		return nil, errors.New("invalid session token")
	}
	if sess.ClientID != sender {
		return nil, errors.New("session token does not belong to this client")
	}
	return sess, nil
}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (h *Handler) execute(sess *authSession, req dbgdapi.ExecRequest, useShell bool) (dbgdapi.ExecResult, error) {
	line := strings.TrimSpace(req.Command)
	if line == "" {
		return dbgdapi.ExecResult{}, errors.New("command is required")
	}

	dir, err := resolveWorkDir(sess.Identity, req.Cwd)
	if err != nil {
		return dbgdapi.ExecResult{}, err
	}

	timeout := normalizeTimeout(req.TimeoutSec)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if useShell {
		cmd, err = makeShellCommand(ctx, sess.Identity, line)
	} else {
		parts, splitErr := splitCommandLine(line)
		if splitErr != nil {
			return dbgdapi.ExecResult{}, splitErr
		}
		if len(parts) == 0 {
			return dbgdapi.ExecResult{}, errors.New("command is required")
		}
		path, lookupErr := resolveExecutable(parts[0])
		if lookupErr != nil {
			return dbgdapi.ExecResult{}, lookupErr
		}
		cmd = exec.CommandContext(ctx, path, parts[1:]...)
	}
	if err != nil {
		return dbgdapi.ExecResult{}, err
	}

	cmd.Dir = dir
	cmd.Env = buildUserEnv(sess.Identity)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: buildCredential(sess.Identity)}

	stdoutBuf := &limitedBuffer{limit: h.maxOutput}
	stderrBuf := &limitedBuffer{limit: h.maxOutput}
	if cmd.Stdout == nil {
		cmd.Stdout = stdoutBuf
	}
	if cmd.Stderr == nil {
		cmd.Stderr = stderrBuf
	}

	runErr := cmd.Run()
	exitCode := 0

	if runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			exitCode = -1
			stderrBuf.Write([]byte("command timed out"))
		} else {
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				return dbgdapi.ExecResult{}, runErr
			}
		}
	}

	return dbgdapi.ExecResult{
		ExitCode:        exitCode,
		Stdout:          stdoutBuf.Bytes(),
		Stderr:          stderrBuf.Bytes(),
		StdoutTruncated: stdoutBuf.truncated,
		StderrTruncated: stderrBuf.truncated,
	}, nil
}

func (h *Handler) runHelper(sess *authSession, mode, path string, offset uint64, size uint32, truncate bool, perm uint32, input []byte) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	args := []string{
		"-dbg-helper", mode,
		"-dbg-path", path,
		"-dbg-offset", strconv.FormatUint(offset, 10),
		"-dbg-size", strconv.FormatUint(uint64(size), 10),
		"-dbg-mode", strconv.FormatUint(uint64(perm), 10),
	}
	if truncate {
		args = append(args, "-dbg-truncate")
	}

	cmd := exec.Command(exe, args...)
	cmd.Env = buildUserEnv(sess.Identity)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: buildCredential(sess.Identity)}
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("%w: %s", err, detail)
		}
		return nil, err
	}

	return stdout.Bytes(), nil
}

func normalizeTimeout(v int32) time.Duration {
	if v <= 0 {
		return 30 * time.Second
	}
	if v > 300 {
		v = 300
	}
	return time.Duration(v) * time.Second
}

func resolveWorkDir(id *identity.Identity, requested string) (string, error) {
	home := strings.TrimSpace(id.Home)
	if home == "" {
		home = "/"
	}

	requested = strings.TrimSpace(requested)
	if requested == "" {
		requested = home
	} else if !filepath.IsAbs(requested) {
		requested = filepath.Join(home, requested)
	}

	requested = filepath.Clean(requested)
	info, err := os.Stat(requested)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", requested)
	}
	return requested, nil
}

func buildUserEnv(id *identity.Identity) []string {
	home := strings.TrimSpace(id.Home)
	if home == "" {
		home = "/"
	}
	shell := strings.TrimSpace(id.Shell)
	if shell == "" {
		shell = fs.Resolve("cmd", "shell")
	}
	path := "/cmd:/avyos/cmd:/bin:/usr/bin"

	out := make([]string, 0, 8)
	seen := map[string]struct{}{}
	for _, kv := range []string{
		"HOME=" + home,
		"USER=" + id.Name,
		"LOGNAME=" + id.Name,
		"SHELL=" + shell,
		"PATH=" + path,
		"TERM=xterm",
	} {
		out = append(out, kv)
		if i := strings.IndexByte(kv, '='); i > 0 {
			seen[kv[:i]] = struct{}{}
		}
	}

	for _, kv := range os.Environ() {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		key := kv[:i]
		if _, ok := seen[key]; ok {
			continue
		}
		out = append(out, kv)
	}

	return out
}

func buildCredential(id *identity.Identity) *syscall.Credential {
	groupsRaw := id.GetGroupIDs()
	groups := make([]uint32, 0, len(groupsRaw))
	for _, g := range groupsRaw {
		groups = append(groups, uint32(g))
	}

	gid := uint32(id.ID)
	uid := uint32(id.ID)
	return &syscall.Credential{
		Uid:    uid,
		Gid:    gid,
		Groups: groups,
	}
}

func makeShellCommand(ctx context.Context, id *identity.Identity, line string) (*exec.Cmd, error) {
	shell := strings.TrimSpace(id.Shell)
	if shell == "" {
		shell = "/bin/sh"
	}

	if path, err := resolveExecutable(shell); err == nil {
		shell = path
	}

	if supportsDashC(shell) {
		return exec.CommandContext(ctx, shell, "-lc", line), nil
	}

	cmd := exec.CommandContext(ctx, shell)
	cmd.Stdin = strings.NewReader(line + "\nexit\n")
	return cmd, nil
}

func supportsDashC(shell string) bool {
	base := strings.ToLower(filepath.Base(shell))
	switch base {
	case "sh", "bash", "dash", "ash", "zsh", "ksh":
		return true
	default:
		return false
	}
}

func resolveExecutable(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("empty command")
	}

	if strings.Contains(name, "/") {
		if isExecutableFile(name) {
			return name, nil
		}
		return "", fmt.Errorf("command not executable: %s", name)
	}

	for _, dir := range []string{"/cmd", "/avyos/cmd", "/bin", "/usr/bin"} {
		candidate := filepath.Join(dir, name)
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}

	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("command not found: %s", name)
	}
	return path, nil
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0111 != 0
}

func splitCommandLine(line string) ([]string, error) {
	var out []string
	var current strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		out = append(out, current.String())
		current.Reset()
	}

	for _, r := range line {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			current.WriteRune(r)
		}
	}

	if escaped {
		return nil, errors.New("invalid command line: trailing escape")
	}
	if quote != 0 {
		return nil, errors.New("invalid command line: unterminated quote")
	}
	flush()
	return out, nil
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		b.truncated = true
		return len(p), nil
	}

	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	_, err := b.buf.Write(p)
	return len(p), err
}

func (b *limitedBuffer) Bytes() []byte {
	out := b.buf.Bytes()
	copyOut := make([]byte, len(out))
	copy(copyOut, out)
	return copyOut
}
