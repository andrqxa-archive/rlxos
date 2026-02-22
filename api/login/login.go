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

package login

import (
	"time"

	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/sutra"
)

const (
	ServiceName = "dev.avyos.login"

	RequestLogout       uint16 = 0x0201
	RequestLock         uint16 = 0x0202
	RequestGetSession   uint16 = 0x0203
	RequestListSessions uint16 = 0x0204
	RequestSwitchUser   uint16 = 0x0205

	EventSessionStarted     uint16 = 0x0301
	EventSessionEnded       uint16 = 0x0302
	EventSessionListChanged uint16 = 0x0303
)

type SessionInfo struct {
	Username string
	UID      uint32
	PID      int
	Active   bool
}

func (s *SessionInfo) Encode() []byte {
	enc := sutra.NewEncoder(64)
	enc.PutString(s.Username)
	enc.PutUint32(s.UID)
	enc.PutInt(s.PID)
	enc.PutBool(s.Active)
	return enc.Bytes()
}

func DecodeSessionInfo(data []byte) SessionInfo {
	d := sutra.NewDecoder(data)
	return SessionInfo{
		Username: d.String(),
		UID:      d.Uint32(),
		PID:      d.Int(),
		Active:   d.Bool(),
	}
}

// Client provides a typed API to the login/session manager service.
type Client struct {
	conn *sutra.Client
}

func Connect() (*Client, error) {
	c, err := sutra.Connect(fs.Resolve("system:" + ServiceName))
	if err != nil {
		return nil, err
	}
	return &Client{conn: c}, nil
}

func (c *Client) Raw() *sutra.Client { return c.conn }

func (c *Client) Logout() error {
	_, err := c.conn.Call(sutra.IDService, RequestLogout, nil, 10*time.Second)
	return err
}

func (c *Client) Lock() error {
	_, err := c.conn.Call(sutra.IDService, RequestLock, nil, 5*time.Second)
	return err
}

func (c *Client) GetSession() (SessionInfo, error) {
	resp, err := c.conn.Call(sutra.IDService, RequestGetSession, nil, 5*time.Second)
	if err != nil {
		return SessionInfo{}, err
	}
	return DecodeSessionInfo(resp.Payload), nil
}

func (c *Client) OnSessionStarted(fn func(SessionInfo)) {
	c.conn.On(EventSessionStarted, func(t *sutra.Transaction) {
		fn(DecodeSessionInfo(t.Payload))
	})
}

func (c *Client) OnSessionEnded(fn func()) {
	c.conn.On(EventSessionEnded, func(t *sutra.Transaction) {
		fn()
	})
}

func (c *Client) ListSessions() ([]SessionInfo, error) {
	resp, err := c.conn.Call(sutra.IDService, RequestListSessions, nil, 5*time.Second)
	if err != nil {
		return nil, err
	}
	return DecodeSessionList(resp.Payload), nil
}

func (c *Client) SwitchUser() error {
	_, err := c.conn.Call(sutra.IDService, RequestSwitchUser, nil, 10*time.Second)
	return err
}

func (c *Client) OnSessionListChanged(fn func([]SessionInfo)) {
	c.conn.On(EventSessionListChanged, func(t *sutra.Transaction) {
		fn(DecodeSessionList(t.Payload))
	})
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func EncodeSessionList(sessions []SessionInfo) []byte {
	enc := sutra.NewEncoder(64)
	enc.PutUint32(uint32(len(sessions)))
	for i := range sessions {
		enc.PutString(sessions[i].Username)
		enc.PutUint32(sessions[i].UID)
		enc.PutInt(sessions[i].PID)
		enc.PutBool(sessions[i].Active)
	}
	return enc.Bytes()
}

func DecodeSessionList(data []byte) []SessionInfo {
	d := sutra.NewDecoder(data)
	n := int(d.Uint32())
	out := make([]SessionInfo, n)
	for i := range out {
		out[i].Username = d.String()
		out[i].UID = d.Uint32()
		out[i].PID = d.Int()
		out[i].Active = d.Bool()
	}
	return out
}
