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

package uevent

import (
	"time"

	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/sutra"
)

const (
	ServiceName = "dev.avyos.uevent"

	RequestListDevices uint16 = 0x0201
	RequestGetDevice   uint16 = 0x0202
	RequestTrigger     uint16 = 0x0203

	EventDeviceAdded   uint16 = 0x0301
	EventDeviceRemoved uint16 = 0x0302
	EventDeviceChanged uint16 = 0x0303
)

type DeviceEvent struct {
	Action    string
	Subsystem string
	DevPath   string
	DevName   string
	DevType   string
	Major     int
	Minor     int
}

type DeviceInfo struct {
	DevPath   string
	DevName   string
	Subsystem string
	DevType   string
	Driver    string
	Major     int
	Minor     int
}

func (e *DeviceEvent) Encode() []byte {
	enc := sutra.NewEncoder(128)
	enc.PutString(e.Action)
	enc.PutString(e.Subsystem)
	enc.PutString(e.DevPath)
	enc.PutString(e.DevName)
	enc.PutString(e.DevType)
	enc.PutInt(e.Major)
	enc.PutInt(e.Minor)
	return enc.Bytes()
}

func DecodeDeviceEvent(data []byte) DeviceEvent {
	d := sutra.NewDecoder(data)
	return DeviceEvent{
		Action:    d.String(),
		Subsystem: d.String(),
		DevPath:   d.String(),
		DevName:   d.String(),
		DevType:   d.String(),
		Major:     d.Int(),
		Minor:     d.Int(),
	}
}

func (i *DeviceInfo) Encode() []byte {
	enc := sutra.NewEncoder(128)
	enc.PutString(i.DevPath)
	enc.PutString(i.DevName)
	enc.PutString(i.Subsystem)
	enc.PutString(i.DevType)
	enc.PutString(i.Driver)
	enc.PutInt(i.Major)
	enc.PutInt(i.Minor)
	return enc.Bytes()
}

func DecodeDeviceInfo(data []byte) DeviceInfo {
	d := sutra.NewDecoder(data)
	return DeviceInfo{
		DevPath:   d.String(),
		DevName:   d.String(),
		Subsystem: d.String(),
		DevType:   d.String(),
		Driver:    d.String(),
		Major:     d.Int(),
		Minor:     d.Int(),
	}
}

func EncodeDeviceList(devices []DeviceInfo) []byte {
	enc := sutra.NewEncoder(256)
	enc.PutUint32(uint32(len(devices)))
	for _, dev := range devices {
		enc.PutString(dev.DevPath)
		enc.PutString(dev.DevName)
		enc.PutString(dev.Subsystem)
		enc.PutString(dev.DevType)
		enc.PutString(dev.Driver)
		enc.PutInt(dev.Major)
		enc.PutInt(dev.Minor)
	}
	return enc.Bytes()
}

func DecodeDeviceList(data []byte) []DeviceInfo {
	d := sutra.NewDecoder(data)
	count := int(d.Uint32())
	devices := make([]DeviceInfo, 0, count)
	for i := 0; i < count; i++ {
		devices = append(devices, DeviceInfo{
			DevPath:   d.String(),
			DevName:   d.String(),
			Subsystem: d.String(),
			DevType:   d.String(),
			Driver:    d.String(),
			Major:     d.Int(),
			Minor:     d.Int(),
		})
	}
	return devices
}

// Client provides a typed API to the uevent service.
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

func (c *Client) ListDevices() ([]DeviceInfo, error) {
	resp, err := c.conn.Call(sutra.IDService, RequestListDevices, nil, 5*time.Second)
	if err != nil {
		return nil, err
	}
	return DecodeDeviceList(resp.Payload), nil
}

func (c *Client) GetDevice(devPath string) (DeviceInfo, error) {
	req := &DeviceInfo{DevPath: devPath}
	resp, err := c.conn.Call(sutra.IDService, RequestGetDevice, req.Encode(), 5*time.Second)
	if err != nil {
		return DeviceInfo{}, err
	}
	return DecodeDeviceInfo(resp.Payload), nil
}

func (c *Client) Trigger(subsystem string) error {
	enc := sutra.NewEncoder(32)
	enc.PutString(subsystem)
	_, err := c.conn.Call(sutra.IDService, RequestTrigger, enc.Bytes(), 10*time.Second)
	return err
}

func (c *Client) OnDeviceAdded(fn func(DeviceEvent)) {
	c.conn.On(EventDeviceAdded, func(t *sutra.Transaction) {
		fn(DecodeDeviceEvent(t.Payload))
	})
}

func (c *Client) OnDeviceRemoved(fn func(DeviceEvent)) {
	c.conn.On(EventDeviceRemoved, func(t *sutra.Transaction) {
		fn(DecodeDeviceEvent(t.Payload))
	})
}

func (c *Client) OnDeviceChanged(fn func(DeviceEvent)) {
	c.conn.On(EventDeviceChanged, func(t *sutra.Transaction) {
		fn(DecodeDeviceEvent(t.Payload))
	})
}

func (c *Client) Close() error {
	return c.conn.Close()
}
