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
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"avyos.dev/pkg/fs"
)

const (
	distroWaylandRuntime = "/run/wayland"
	distroWaylandDisplay = "wayland-0"
)

// waylandBridge relays Wayland connections from inside the container to
// the session-level waylayer running at the user's XDG_RUNTIME_DIR.
type waylandBridge struct {
	rootfs         string
	socketHost     string
	upstreamSocket string

	listener *net.UnixListener

	closeOnce sync.Once
	closed    chan struct{}
}

// newWaylandBridge creates a bridge that relays container Wayland clients
// to the session waylayer socket at /cache/runtime/user/<uid>/wayland-0.
func newWaylandBridge(rootfs string, uid uint32) (*waylandBridge, error) {
	upstreamSocket := filepath.Join(fs.UserRunPath, strconv.FormatUint(uint64(uid), 10), distroWaylandDisplay)
	if _, err := os.Stat(upstreamSocket); err != nil {
		return nil, fmt.Errorf("session waylayer socket not found at %s: %w", upstreamSocket, err)
	}

	runtimeHost := filepath.Join(rootfs, filepath.FromSlash(distroWaylandRuntime))
	if err := os.MkdirAll(runtimeHost, 0777); err != nil {
		return nil, fmt.Errorf("create distro wayland runtime: %w", err)
	}
	_ = os.Chmod(runtimeHost, 0777)

	socketHost := filepath.Join(runtimeHost, distroWaylandDisplay)
	_ = os.Remove(socketHost)

	addr, err := net.ResolveUnixAddr("unix", socketHost)
	if err != nil {
		return nil, fmt.Errorf("resolve distro wayland socket: %w", err)
	}

	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("listen distro wayland socket: %w", err)
	}
	_ = os.Chmod(socketHost, 0666)

	b := &waylandBridge{
		rootfs:         rootfs,
		socketHost:     socketHost,
		upstreamSocket: upstreamSocket,
		listener:       ln,
		closed:         make(chan struct{}),
	}

	go b.acceptLoop()
	return b, nil
}

func (b *waylandBridge) Env() []string {
	return []string{
		"XDG_RUNTIME_DIR=" + distroWaylandRuntime,
		"WAYLAND_DISPLAY=" + distroWaylandDisplay,
	}
}

func (b *waylandBridge) Close() {
	b.closeOnce.Do(func() {
		close(b.closed)
		if b.listener != nil {
			_ = b.listener.Close()
		}
		if b.socketHost != "" {
			_ = os.Remove(b.socketHost)
		}
	})
}

func (b *waylandBridge) acceptLoop() {
	for {
		conn, err := b.listener.AcceptUnix()
		if err != nil {
			select {
			case <-b.closed:
				return
			default:
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go b.handleConn(conn)
	}
}

func (b *waylandBridge) handleConn(clientConn *net.UnixConn) {
	upstreamConn, err := b.connectUpstream()
	if err != nil {
		serviceLog.Debug("wayland bridge upstream connect failed: %v", err)
		_ = clientConn.Close()
		return
	}

	done := make(chan struct{}, 2)
	go relayWaylandStream(upstreamConn, clientConn, done)
	go relayWaylandStream(clientConn, upstreamConn, done)
	<-done
	_ = clientConn.Close()
	_ = upstreamConn.Close()
}

func relayWaylandStream(dst, src *net.UnixConn, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	srcRaw, err := src.SyscallConn()
	if err != nil {
		return
	}
	dstRaw, err := dst.SyscallConn()
	if err != nil {
		return
	}

	buf := make([]byte, 64*1024)
	oob := make([]byte, syscall.CmsgSpace(4*32))

	for {
		var n, oobn int
		var recvErr error
		err = srcRaw.Read(func(fd uintptr) bool {
			n, oobn, _, _, recvErr = syscall.Recvmsg(int(fd), buf, oob, 0)
			if recvErr == syscall.EINTR || recvErr == syscall.EAGAIN {
				return false
			}
			return true
		})
		if err != nil || recvErr != nil || n <= 0 {
			return
		}

		data := buf[:n]
		control := oob[:oobn]

		var sendErr error
		err = dstRaw.Write(func(fd uintptr) bool {
			sendErr = syscall.Sendmsg(int(fd), data, control, nil, 0)
			if sendErr == syscall.EINTR || sendErr == syscall.EAGAIN {
				return false
			}
			return true
		})
		if err != nil || sendErr != nil {
			return
		}
	}
}

func (b *waylandBridge) connectUpstream() (*net.UnixConn, error) {
	var lastErr error
	for i := 0; i < 20; i++ {
		addr, err := net.ResolveUnixAddr("unix", b.upstreamSocket)
		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		conn, err := net.DialUnix("unix", nil, addr)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timeout")
	}
	return nil, fmt.Errorf("connect session waylayer: %w", lastErr)
}
