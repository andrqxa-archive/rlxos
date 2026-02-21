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

package dbg

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"avyos.dev/pkg/sutra"
)

const DefaultTCPPort = 5037

func NewTCPClient(address string) (*Client, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(DefaultTCPPort))
	}

	raw, err := sutra.ConnectTCP(address)
	if err != nil {
		return nil, err
	}
	return &Client{
		client:  raw,
		timeout: 15 * time.Second,
	}, nil
}

func NewHostClient(host string, port int) (*Client, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", port)
	}
	return NewTCPClient(net.JoinHostPort(host, strconv.Itoa(port)))
}

func (c *Client) Raw() *sutra.Client {
	return c.client
}
