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

	"avyos.dev/api/dbg"
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
	shells    *shellSessionManager

	sessionMu sync.RWMutex
	sessions  map[string]*authSession
}

func NewHandler(maxOutput int, shells *shellSessionManager) *Handler {
	if maxOutput <= 0 {
		maxOutput = defaultMaxOutputBytes
	}
	return &Handler{
		maxOutput: maxOutput,
		shells:    shells,
		sessions:  make(map[string]*authSession),
	}
}

func (h *Handler) Authenticate(sender uint32, req dbg.AuthRequest) (dbg.SessionInfo, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = "admin"
	}

	id, err := identity.Authenticate(username, req.Password)
	if err != nil {
		return dbg.SessionInfo{}, fmt.Errorf("authentication failed: %w", err)
	}

	token, err := randomToken()
	if err != nil {
		return dbg.SessionInfo{}, err
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

	return dbg.SessionInfo{
		Token:    token,
		Username: id.Name,
		UID:      uint32(id.ID),
		GID:      uint32(id.ID),
		Home:     id.Home,
		Shell:    id.Shell,
	}, nil
}

func (h *Handler) Logout(sender uint32, req dbg.SessionToken) (dbg.Empty, error) {
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return dbg.Empty{}, nil
	}

	h.sessionMu.Lock()
	sess, ok := h.sessions[token]
	if ok {
		if sess.ClientID != sender {
			h.sessionMu.Unlock()
			return dbg.Empty{}, errors.New("session token does not belong to this client")
		}
		delete(h.sessions, token)
	}
	h.sessionMu.Unlock()

	if ok {
		if h.shells != nil {
			h.shells.CloseByToken(sender, token)
		}
		serviceLog.Info("session closed for user=%s client=%d", sess.Identity.Name, sender)
	}

	return dbg.Empty{}, nil
}

func (h *Handler) RunCommand(sender uint32, req dbg.ExecRequest) (dbg.ExecResult, error) {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return dbg.ExecResult{}, err
	}
	return h.execute(sess, req, false)
}

func (h *Handler) RunShell(sender uint32, req dbg.ExecRequest) (dbg.ExecResult, error) {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return dbg.ExecResult{}, err
	}
	return h.execute(sess, req, true)
}

func (h *Handler) ShellOpen(sender uint32, req dbg.ShellOpenRequest) (dbg.ShellSession, error) {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return dbg.ShellSession{}, err
	}
	if h.shells == nil {
		return dbg.ShellSession{}, errors.New("interactive shell is unavailable")
	}
	return h.shells.Open(sender, sess, req)
}

func (h *Handler) ShellInput(sender uint32, req dbg.ShellInputRequest) error {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return err
	}
	if h.shells == nil {
		return errors.New("interactive shell is unavailable")
	}
	return h.shells.Input(sender, sess, req)
}

func (h *Handler) ShellResize(sender uint32, req dbg.ShellResizeRequest) error {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return err
	}
	if h.shells == nil {
		return errors.New("interactive shell is unavailable")
	}
	return h.shells.Resize(sender, sess, req)
}

func (h *Handler) ShellClose(sender uint32, req dbg.ShellCloseRequest) error {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return err
	}
	if h.shells == nil {
		return nil
	}
	return h.shells.Close(sender, sess, req)
}

func (h *Handler) ReadFile(sender uint32, req dbg.ReadFileRequest) (dbg.FileChunk, error) {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return dbg.FileChunk{}, err
	}

	path := strings.TrimSpace(req.Path)
	if path == "" {
		return dbg.FileChunk{}, errors.New("read path is required")
	}
	path = filepath.Clean(path)

	size := int(req.Size)
	if size <= 0 || size > defaultFileChunkBytes {
		size = defaultFileChunkBytes
	}

	data, err := h.runHelper(sess, "read", path, req.Offset, uint32(size), false, 0, nil)
	if err != nil {
		return dbg.FileChunk{}, err
	}

	return dbg.FileChunk{
		Data: data,
		Eof:  len(data) < size,
	}, nil
}

func (h *Handler) WriteFile(sender uint32, req dbg.WriteFileRequest) (dbg.WriteFileResult, error) {
	sess, err := h.sessionFor(sender, req.Token)
	if err != nil {
		return dbg.WriteFileResult{}, err
	}

	path := strings.TrimSpace(req.Path)
	if path == "" {
		return dbg.WriteFileResult{}, errors.New("write path is required")
	}
	path = filepath.Clean(path)

	if len(req.Data) > defaultFileChunkBytes {
		return dbg.WriteFileResult{}, fmt.Errorf("write chunk exceeds %d bytes", defaultFileChunkBytes)
	}

	mode := req.Mode
	if mode == 0 {
		mode = 0644
	}

	stdout, err := h.runHelper(sess, "write", path, req.Offset, uint32(len(req.Data)), req.Truncate, mode, req.Data)
	if err != nil {
		return dbg.WriteFileResult{}, err
	}

	written := len(req.Data)
	if s := strings.TrimSpace(string(stdout)); s != "" {
		if n, parseErr := strconv.Atoi(s); parseErr == nil && n >= 0 {
			written = n
		}
	}

	return dbg.WriteFileResult{Written: uint32(written)}, nil
}

func (h *Handler) DropClientSessions(clientID uint32) {
	if h.shells != nil {
		h.shells.CloseByOwner(clientID)
	}

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

func (h *Handler) execute(sess *authSession, req dbg.ExecRequest, useShell bool) (dbg.ExecResult, error) {
	line := strings.TrimSpace(req.Command)
	if line == "" {
		return dbg.ExecResult{}, errors.New("command is required")
	}

	dir, err := resolveWorkDir(sess.Identity, req.Cwd)
	if err != nil {
		return dbg.ExecResult{}, fmt.Errorf("failed to resolve workdir %v", err)
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
			return dbg.ExecResult{}, splitErr
		}
		if len(parts) == 0 {
			return dbg.ExecResult{}, errors.New("command is required")
		}
		path, lookupErr := resolveExecutable(parts[0])
		if lookupErr != nil {
			return dbg.ExecResult{}, lookupErr
		}
		cmd = exec.CommandContext(ctx, path, parts[1:]...)
	}
	if err != nil {
		return dbg.ExecResult{}, err
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
	if cmd.Stdin == nil {
		cmd.Stdin, _ = os.OpenFile(fs.Resolve("device:null"), os.O_RDWR, 0)
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
				return dbg.ExecResult{}, runErr
			}
		}
	}

	return dbg.ExecResult{
		ExitCode:        exitCode,
		Stdout:          stdoutBuf.Bytes(),
		Stderr:          stderrBuf.Bytes(),
		StdoutTruncated: stdoutBuf.truncated,
		StderrTruncated: stderrBuf.truncated,
	}, nil
}

func (h *Handler) runHelper(sess *authSession, mode, path string, offset uint64, size uint32, truncate bool, perm uint32, input []byte) ([]byte, error) {
	exe, err := os.Readlink(fs.Resolve("process:self/exe"))
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
	var stdin bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = &stdin

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
	if home == "" || !fs.Exists(home) {
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
		shell = fs.Resolve("cmd:shell")
	}
	path := "/cmd:/avyos/cmd"

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
		shell = "/avyos/cmd/shell"
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

	for _, dir := range []string{"/cmd", "/avyos/cmd"} {
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
