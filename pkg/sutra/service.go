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

package sutra

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"syscall"
	"unsafe"

	"avyos.dev/pkg/fs"
)

// Service represents a decentralized named service endpoint.
type Service struct {
	Name         string
	ID           uint32
	socketPath   string
	listener     net.Listener
	methods      map[uint16]MethodHandler
	pending      map[pendingKey][]pendingCall
	clients      map[uint32]*serviceConn
	mu           sync.RWMutex
	writeMu      sync.Mutex
	pendingMu    sync.Mutex
	closed       bool
	closedMu     sync.Mutex
	done         chan struct{}
	onDisconnect func()
	nextClientID atomic.Uint32
}

type serviceConn struct {
	id      uint32
	uid     uint32 // Unix UID of the connecting process (via SO_PEERCRED)
	conn    net.Conn
	writeMu sync.Mutex
}

// MethodHandler handles a method call and returns a response.
type MethodHandler func(t *Transaction) ([]byte, error)

// NewService creates and starts a service endpoint.
func NewService(name, socketPath string) (*Service, error) {
	if name == "" {
		return nil, errors.New("service name is required")
	}
	if socketPath == "" {
		socketPath = fs.Resolve("system:%s", name)
	}

	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		return nil, err
	}
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(socketPath, 0777)

	return newServiceWithListener(name, socketPath, listener), nil
}

// NewServiceTCP creates and starts a service endpoint on a TCP address.
func NewServiceTCP(name, address string) (*Service, error) {
	if name == "" {
		return nil, errors.New("service name is required")
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("tcp address is required")
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	return newServiceWithListener(name, "", listener), nil
}

func newServiceWithListener(name, socketPath string, listener net.Listener) *Service {
	s := &Service{
		Name:       name,
		ID:         IDService,
		socketPath: socketPath,
		listener:   listener,
		methods:    make(map[uint16]MethodHandler),
		pending:    make(map[pendingKey][]pendingCall),
		clients:    make(map[uint32]*serviceConn),
		done:       make(chan struct{}),
	}
	s.nextClientID.Store(IDClient)

	go s.acceptLoop()
	return s
}

func (s *Service) isClosed() bool {
	s.closedMu.Lock()
	defer s.closedMu.Unlock()
	return s.closed
}

func (s *Service) acceptLoop() {
	for !s.isClosed() {
		netConn, err := s.listener.Accept()
		if err != nil {
			if s.isClosed() {
				break
			}
			continue
		}
		go s.handleConnection(netConn)
	}

	s.closedMu.Lock()
	closed := s.closed
	s.closedMu.Unlock()
	if !closed {
		s.closedMu.Lock()
		s.closed = true
		s.closedMu.Unlock()
	}

	select {
	case <-s.done:
	default:
		close(s.done)
	}
	if s.onDisconnect != nil {
		s.onDisconnect()
	}
}

func (s *Service) handleConnection(netConn net.Conn) {
	id := s.nextClientID.Add(1)
	c := &serviceConn{id: id, conn: netConn}

	// Extract peer credentials (UID) from Unix socket.
	if uc, ok := netConn.(*net.UnixConn); ok {
		if raw, err := uc.SyscallConn(); err == nil {
			_ = raw.Control(func(fd uintptr) {
				if cred, err := getPeerCred(int(fd)); err == nil {
					c.uid = cred
				}
			})
		}
	}

	s.mu.Lock()
	s.clients[id] = c
	s.mu.Unlock()

	ack := NewTransaction(IDService, id, EventConnect, EncodeUint32(id))
	if err := c.send(ack); err != nil {
		s.removeClient(id)
		_ = netConn.Close()
		return
	}

	for !s.isClosed() {
		t, err := ReadTransaction(netConn)
		if err != nil {
			if err != io.EOF && !s.isClosed() {
				// Connection error.
			}
			break
		}
		t.Sender = id
		s.handleTransaction(t)
	}

	s.removeClient(id)
	_ = netConn.Close()

	// Dispatch local disconnect event so services can cleanup by client ID.
	s.dispatchDisconnect(id)
}

func (s *Service) handleTransaction(t *Transaction) {
	s.pendingMu.Lock()
	key := pendingKey{sender: t.Sender, event: t.Event}
	if list, exists := s.pending[key]; exists && len(list) > 0 {
		pending := list[0]
		if len(list) == 1 {
			delete(s.pending, key)
		} else {
			s.pending[key] = list[1:]
		}
		s.pendingMu.Unlock()
		pending.ch <- t
		return
	}
	if t.Event == EventError {
		for k, pending := range s.pending {
			if k.sender == t.Sender && len(pending) > 0 {
				first := pending[0]
				if len(pending) == 1 {
					delete(s.pending, k)
				} else {
					s.pending[k] = pending[1:]
				}
				s.pendingMu.Unlock()
				first.ch <- t
				return
			}
		}
	}
	s.pendingMu.Unlock()

	s.mu.RLock()
	handler, exists := s.methods[t.Event]
	s.mu.RUnlock()
	if !exists {
		reply := t.Error("unknown method")
		_ = s.sendReply(reply)
		return
	}

	result, err := handler(t)
	if err != nil {
		reply := t.Error(err.Error())
		_ = s.sendReply(reply)
		return
	}

	reply := t.Reply(t.Event, result)
	_ = s.sendReply(reply)
}

func (s *Service) sendReply(t *Transaction) error {
	if s.isClosed() {
		return errors.New("service closed")
	}
	s.mu.RLock()
	client := s.clients[t.Destination]
	s.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("destination %d not connected", t.Destination)
	}
	return client.send(t)
}

func (c *serviceConn) send(t *Transaction) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := t.WriteTo(c.conn)
	return err
}

func (s *Service) removeClient(id uint32) {
	s.mu.Lock()
	delete(s.clients, id)
	s.mu.Unlock()
}

func (s *Service) dispatchDisconnect(id uint32) {
	s.mu.RLock()
	handler, exists := s.methods[EventDisconnect]
	s.mu.RUnlock()
	if !exists {
		return
	}
	t := NewTransaction(IDService, IDService, EventDisconnect, EncodeUint32(id))
	_, _ = handler(t)
}

// OnDisconnect sets the disconnection callback.
func (s *Service) OnDisconnect(fn func()) {
	s.onDisconnect = fn
}

// Handle registers a method handler for an event.
func (s *Service) Handle(event uint16, handler MethodHandler) {
	s.mu.Lock()
	s.methods[event] = handler
	s.mu.Unlock()
}

// HandleFunc registers a simple handler that returns a fixed response.
func (s *Service) HandleFunc(event uint16, fn func(payload []byte) ([]byte, error)) {
	s.Handle(event, func(t *Transaction) ([]byte, error) {
		return fn(t.Payload)
	})
}

// Send sends a message to a destination client.
func (s *Service) Send(dest uint32, event uint16, payload []byte) error {
	if s.isClosed() {
		return errors.New("service closed")
	}

	if dest == IDBroadcast {
		s.mu.RLock()
		clients := make([]*serviceConn, 0, len(s.clients))
		for _, c := range s.clients {
			clients = append(clients, c)
		}
		s.mu.RUnlock()

		var firstErr error
		for _, c := range clients {
			t := NewTransaction(IDService, c.id, event, payload)
			if err := c.send(t); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	t := NewTransaction(IDService, dest, event, payload)
	return s.sendReply(t)
}

// Call makes a call to a specific client and waits for response.
func (s *Service) Call(dest uint32, event uint16, payload []byte, timeout time.Duration) (*Transaction, error) {
	if s.isClosed() {
		return nil, errors.New("service closed")
	}

	t := NewTransaction(IDService, dest, event, payload)

	ch := make(chan *Transaction, 1)
	key := pendingKey{sender: dest, event: event}
	s.pendingMu.Lock()
	s.pending[key] = append(s.pending[key], pendingCall{ch: ch})
	s.pendingMu.Unlock()

	if err := s.sendReply(t); err != nil {
		s.pendingMu.Lock()
		s.removePendingLocked(key, ch)
		s.pendingMu.Unlock()
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Event == EventError {
			return nil, errors.New(resp.PayloadString())
		}
		return resp, nil
	case <-time.After(timeout):
		s.pendingMu.Lock()
		s.removePendingLocked(key, ch)
		s.pendingMu.Unlock()
		return nil, errors.New("timeout waiting for response")
	}
}

// Broadcast sends an event to all connected clients.
func (s *Service) Broadcast(event uint16, payload []byte) error {
	return s.Send(IDBroadcast, event, payload)
}

// Close stops the service listener and all client connections.
func (s *Service) Close() error {
	s.closedMu.Lock()
	if s.closed {
		s.closedMu.Unlock()
		return nil
	}
	s.closed = true
	s.closedMu.Unlock()

	if s.listener != nil {
		_ = s.listener.Close()
	}

	s.mu.Lock()
	for _, c := range s.clients {
		_ = c.conn.Close()
	}
	s.clients = make(map[uint32]*serviceConn)
	s.mu.Unlock()

	if s.socketPath != "" {
		_ = os.Remove(s.socketPath)
	}

	select {
	case <-s.done:
	default:
		close(s.done)
	}
	return nil
}

func (s *Service) removePendingLocked(key pendingKey, ch chan *Transaction) {
	list := s.pending[key]
	if len(list) == 0 {
		return
	}
	for i := range list {
		if list[i].ch == ch {
			list = append(list[:i], list[i+1:]...)
			if len(list) == 0 {
				delete(s.pending, key)
			} else {
				s.pending[key] = list
			}
			return
		}
	}
}

// IsRunning returns true if the service is running.
func (s *Service) IsRunning() bool {
	return !s.isClosed()
}

// Run blocks until the service is closed.
func (s *Service) Run() {
	<-s.done
}

// GetClientUID returns the Unix UID of the process that connected with the given client ID.
func (s *Service) GetClientUID(clientID uint32) (uint32, bool) {
	s.mu.RLock()
	c, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		return 0, false
	}
	return c.uid, true
}

// getPeerCred extracts the UID from a Unix socket using SO_PEERCRED.
func getPeerCred(fd int) (uint32, error) {
	var cred syscall.Ucred
	credLen := uint32(unsafe.Sizeof(cred))
	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(fd),
		syscall.SOL_SOCKET,
		syscall.SO_PEERCRED,
		uintptr(unsafe.Pointer(&cred)),
		uintptr(unsafe.Pointer(&credLen)),
		0,
	)
	if errno != 0 {
		return 0, errno
	}
	return cred.Uid, nil
}
