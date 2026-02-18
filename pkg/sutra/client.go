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
	"io"
	"net"
	"sync"
	"time"
)

// DefaultSocketPath is the legacy default Sutra socket path.
const DefaultSocketPath = "/cache/runtime/sutra.sock"

// Client represents a connection to a service endpoint.
type Client struct {
	ID           uint32
	conn         net.Conn
	handlers     map[uint16]EventHandler
	pending      map[pendingKey][]pendingCall
	mu           sync.RWMutex
	writeMu      sync.Mutex
	pendingMu    sync.Mutex
	closed       bool
	closedMu     sync.Mutex
	onDisconnect func()
}

type pendingKey struct {
	sender uint32
	event  uint16
}

type pendingCall struct {
	ch chan *Transaction
}

// EventHandler handles incoming events.
type EventHandler func(t *Transaction)

// Connect connects to a service endpoint socket.
func Connect(socketPath string) (*Client, error) {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}

	c := &Client{
		conn:     conn,
		handlers: make(map[uint16]EventHandler),
		pending:  make(map[pendingKey][]pendingCall),
	}

	// Wait for connection acknowledgment with assigned client ID.
	t, err := ReadTransaction(conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if t.Event != EventConnect {
		_ = conn.Close()
		return nil, errors.New("unexpected response from service")
	}

	c.ID = t.PayloadUint32()

	go c.receiveLoop()
	return c, nil
}

func (c *Client) isClosed() bool {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	return c.closed
}

func (c *Client) receiveLoop() {
	for !c.isClosed() {
		t, err := ReadTransaction(c.conn)
		if err != nil {
			if err != io.EOF && !c.isClosed() {
				// Connection error.
			}
			break
		}
		c.handleTransaction(t)
	}

	c.closedMu.Lock()
	c.closed = true
	c.closedMu.Unlock()
	if c.onDisconnect != nil {
		c.onDisconnect()
	}
}

func (c *Client) handleTransaction(t *Transaction) {
	c.pendingMu.Lock()
	key := pendingKey{sender: t.Sender, event: t.Event}
	if list, exists := c.pending[key]; exists && len(list) > 0 {
		pending := list[0]
		if len(list) == 1 {
			delete(c.pending, key)
		} else {
			c.pending[key] = list[1:]
		}
		c.pendingMu.Unlock()
		pending.ch <- t
		return
	}
	if t.Event == EventError {
		for k, pending := range c.pending {
			if k.sender == t.Sender && len(pending) > 0 {
				first := pending[0]
				if len(pending) == 1 {
					delete(c.pending, k)
				} else {
					c.pending[k] = pending[1:]
				}
				c.pendingMu.Unlock()
				first.ch <- t
				return
			}
		}
	}
	c.pendingMu.Unlock()

	c.mu.RLock()
	handler, exists := c.handlers[t.Event]
	c.mu.RUnlock()
	if exists {
		handler(t)
	}
}

// OnDisconnect sets the disconnection callback.
func (c *Client) OnDisconnect(fn func()) {
	c.onDisconnect = fn
}

// On registers an event handler.
func (c *Client) On(event uint16, handler EventHandler) {
	c.mu.Lock()
	c.handlers[event] = handler
	c.mu.Unlock()
}

// Off removes an event handler.
func (c *Client) Off(event uint16) {
	c.mu.Lock()
	delete(c.handlers, event)
	c.mu.Unlock()
}

// Send sends a transaction without waiting for response.
func (c *Client) Send(dest uint32, event uint16, payload []byte) error {
	if c.isClosed() {
		return errors.New("client closed")
	}

	t := NewTransaction(c.ID, dest, event, payload)
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := t.WriteTo(c.conn)
	return err
}

// Call sends a transaction and waits for a response.
func (c *Client) Call(dest uint32, event uint16, payload []byte, timeout time.Duration) (*Transaction, error) {
	if c.isClosed() {
		return nil, errors.New("client closed")
	}

	t := NewTransaction(c.ID, dest, event, payload)

	ch := make(chan *Transaction, 1)
	key := pendingKey{sender: dest, event: event}
	c.pendingMu.Lock()
	c.pending[key] = append(c.pending[key], pendingCall{ch: ch})
	c.pendingMu.Unlock()

	c.writeMu.Lock()
	_, err := t.WriteTo(c.conn)
	c.writeMu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		c.removePendingLocked(key, ch)
		c.pendingMu.Unlock()
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Event == EventError {
			return nil, errors.New(resp.PayloadString())
		}
		return resp, nil
	case <-time.After(timeout):
		c.pendingMu.Lock()
		c.removePendingLocked(key, ch)
		c.pendingMu.Unlock()
		return nil, errors.New("timeout waiting for response")
	}
}

func (c *Client) removePendingLocked(key pendingKey, ch chan *Transaction) {
	list := c.pending[key]
	if len(list) == 0 {
		return
	}
	for i := range list {
		if list[i].ch == ch {
			list = append(list[:i], list[i+1:]...)
			if len(list) == 0 {
				delete(c.pending, key)
			} else {
				c.pending[key] = list
			}
			return
		}
	}
}

// Lookup is unsupported in decentralized mode.
func (c *Client) Lookup(name string) (uint32, error) {
	_ = name
	return 0, errors.New("lookup is not supported in decentralized mode")
}

// Ping is unsupported in decentralized mode.
func (c *Client) Ping() error {
	return errors.New("ping is not supported in decentralized mode")
}

// Broadcast sends a broadcast transaction handled by the target service.
func (c *Client) Broadcast(event uint16, payload []byte) error {
	return c.Send(IDBroadcast, event, payload)
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.closedMu.Lock()
	if c.closed {
		c.closedMu.Unlock()
		return nil
	}
	c.closed = true
	c.closedMu.Unlock()
	return c.conn.Close()
}

// IsConnected returns true if the client is connected.
func (c *Client) IsConnected() bool {
	return !c.isClosed()
}
