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

package service

import (
	"fmt"
	"strings"

	"avyos.dev/pkg/sutra"
)

const (
	RequestStart    = RequestStartService
	RequestStop     = RequestStopService
	RequestRestart  = RequestRestartService
	RequestStatus   = RequestGetStatus
	RequestList     = RequestListServices
	RequestPoweroff = RequestSystemPoweroff
	RequestReboot   = RequestSystemReboot
)

func Connect() (*Client, error) {
	return NewClient("")
}

func (c *Client) Raw() *sutra.Client {
	return c.client
}

func (c *Client) Start(name string) error {
	req, err := newServiceNameRequest(name)
	if err != nil {
		return err
	}
	_, err = c.StartService(req)
	return err
}

func (c *Client) Stop(name string) error {
	req, err := newServiceNameRequest(name)
	if err != nil {
		return err
	}
	_, err = c.StopService(req)
	return err
}

func (c *Client) Restart(name string) error {
	req, err := newServiceNameRequest(name)
	if err != nil {
		return err
	}
	_, err = c.RestartService(req)
	return err
}

func (c *Client) Status(name string) (ServiceStatus, error) {
	req, err := newServiceNameRequest(name)
	if err != nil {
		return ServiceStatus{}, err
	}
	return c.GetStatus(req)
}

func (c *Client) List() ([]ServiceStatus, error) {
	resp, err := c.ListServices(Empty{})
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) Poweroff() error {
	_, err := c.SystemPoweroff(Empty{})
	return err
}

func (c *Client) Reboot() error {
	_, err := c.SystemReboot(Empty{})
	return err
}

func (s ServiceStatus) Encode() []byte {
	return s.MarshalBinary()
}

func DecodeServiceStatus(data []byte) (ServiceStatus, error) {
	var status ServiceStatus
	return status, status.UnmarshalBinary(data)
}

func EncodeServiceStatusList(items []ServiceStatus) []byte {
	return ServiceStatusList{Items: items}.MarshalBinary()
}

func DecodeServiceStatusList(data []byte) ([]ServiceStatus, error) {
	var list ServiceStatusList
	if err := list.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func EncodeServiceName(name string) []byte {
	return ServiceNameRequest{Name: strings.TrimSpace(name)}.MarshalBinary()
}

func DecodeServiceName(data []byte) (string, error) {
	var req ServiceNameRequest
	if err := req.UnmarshalBinary(data); err != nil {
		return "", err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", fmt.Errorf("service name is required")
	}
	return name, nil
}

func newServiceNameRequest(name string) (ServiceNameRequest, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ServiceNameRequest{}, fmt.Errorf("service name is required")
	}
	return ServiceNameRequest{Name: name}, nil
}
