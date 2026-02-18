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

package distroapi

import (
	"fmt"
	"strings"

	"avyos.dev/pkg/sutra"
)

func Connect() (*Client, error) {
	return NewClient("")
}

func (c *Client) Raw() *sutra.Client {
	return c.client
}

func (c *Client) ListDistros(available bool) ([]DistroInfo, error) {
	resp, err := c.List(ListRequest{Available: available})
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) PullDistro(name, url string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("distro name required")
	}
	_, err := c.Pull(PullRequest{Name: name, URL: strings.TrimSpace(url)})
	return err
}

func (c *Client) RunDistro(req RunRequest) (RunResult, error) {
	req.Distro = strings.TrimSpace(req.Distro)
	if req.Distro == "" {
		return RunResult{}, fmt.Errorf("distro name required")
	}
	if strings.TrimSpace(req.Workdir) == "" {
		req.Workdir = "/"
	}
	if strings.TrimSpace(req.Command) == "" {
		req.Command = defaultCommand
	}
	return c.Run(req)
}

func (c *Client) RemoveDistro(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("distro name required")
	}
	_, err := c.Remove(RemoveRequest{Name: name})
	return err
}

func (c *Client) OpenShell(req ShellOpenRequest) (uint32, error) {
	req.Distro = strings.TrimSpace(req.Distro)
	if req.Distro == "" {
		return 0, fmt.Errorf("distro name required")
	}
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
)
