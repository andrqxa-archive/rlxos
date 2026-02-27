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

package distro

import (
	"fmt"
	"strings"
	"time"

	"avyos.dev/pkg/sutra"
)

func Connect() (*Client, error) {
	return NewClient("")
}

func (c *Client) Raw() *sutra.Client {
	return c.client
}

func (c *Client) GetStatus() (StatusResponse, error) {
	return c.Status(Empty{})
}

func (c *Client) InstallDistro(url string) error {
	prev := c.timeout
	if c.timeout < installTimeout {
		c.timeout = installTimeout
	}
	defer func() {
		c.timeout = prev
	}()

	_, err := c.Install(InstallRequest{URL: strings.TrimSpace(url)})
	return err
}

func (c *Client) RunDistro(req RunRequest) (RunResult, error) {
	if strings.TrimSpace(req.Workdir) == "" {
		req.Workdir = "/"
	}
	if strings.TrimSpace(req.Command) == "" {
		req.Command = defaultCommand
	}

	prev := c.timeout
	if c.timeout < runTimeout {
		c.timeout = runTimeout
	}
	defer func() {
		c.timeout = prev
	}()

	return c.Run(req)
}

func (c *Client) Uninstall() error {
	_, err := c.Remove(Empty{})
	return err
}

func (c *Client) OpenShell(req ShellOpenRequest) (uint32, error) {
	if strings.TrimSpace(req.Workdir) == "" {
		req.Workdir = "/"
	}
	if req.Rows <= 0 {
		req.Rows = 24
	}
	if req.Cols <= 0 {
		req.Cols = 80
	}
	resp, err := c.ShellOpen(req)
	if err != nil {
		return 0, err
	}
	return resp.SessionID, nil
}

func (c *Client) SendShellInput(sessionID uint32, data []byte) error {
	if sessionID == 0 {
		return fmt.Errorf("invalid shell session")
	}
	return c.ShellInput(ShellInputRequest{
		SessionID: sessionID,
		Data:      data,
	})
}

func (c *Client) ResizeShell(sessionID uint32, rows, cols int) error {
	if sessionID == 0 {
		return fmt.Errorf("invalid shell session")
	}
	if rows <= 0 || cols <= 0 {
		return nil
	}
	return c.ShellResize(ShellResizeRequest{
		SessionID: sessionID,
		Rows:      rows,
		Cols:      cols,
	})
}

func (c *Client) CloseShell(sessionID uint32) error {
	if sessionID == 0 {
		return nil
	}
	return c.ShellClose(ShellCloseRequest{SessionID: sessionID})
}

func (c *Client) OnShellOutputEvent(fn func(ShellOutputEvent)) {
	c.OnShellOutput(func(_ uint32, ev ShellOutputEvent) {
		fn(ev)
	})
}

func (c *Client) OnShellExitEvent(fn func(ShellExitEvent)) {
	c.OnShellExit(func(_ uint32, ev ShellExitEvent) {
		fn(ev)
	})
}

func EncodeCommand(args []string) string {
	if len(args) == 0 {
		return defaultCommand
	}
	return strings.Join(args, commandSep)
}

func DecodeCommand(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{defaultShell}
	}
	parts := strings.Split(value, commandSep)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return []string{defaultShell}
	}
	return out
}

const (
	defaultShell   = "/bin/sh"
	commandSep     = "\x00"
	defaultCommand = "/bin/sh"
	installTimeout = 30 * time.Minute
	runTimeout     = 30 * time.Minute
)
