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
	"fmt"
	"strconv"
	"sync"
	"syscall"

	ueventapi "avyos.dev/api/uevent"
	"avyos.dev/pkg/sutra"
)

// Server is the uevent daemon. It listens on a netlink socket for kernel
// device events, applies rules from uevent.conf, creates device nodes,
// and exposes a sutra service for clients.
type Server struct {
	service  *sutra.Service
	nlFD     int
	rules    []Rule
	devices  map[string]ueventapi.DeviceInfo // devpath -> info
	deviceMu sync.RWMutex
	done     chan struct{}
}

func NewServer() (*Server, error) {
	svc, err := sutra.NewService(ueventapi.ServiceName, "")
	if err != nil {
		return nil, fmt.Errorf("sutra service: %w", err)
	}

	nlFD, err := openNetlinkSocket()
	if err != nil {
		svc.Close()
		return nil, fmt.Errorf("netlink socket: %w", err)
	}

	rules, err := loadRules()
	if err != nil {
		serviceLog.Warn("failed to load rules: %v (using defaults)", err)
		rules = defaultRules()
	}
	serviceLog.Info("loaded %d rules", len(rules))

	srv := &Server{
		service: svc,
		nlFD:    nlFD,
		rules:   rules,
		devices: make(map[string]ueventapi.DeviceInfo),
		done:    make(chan struct{}),
	}

	srv.registerHandlers()
	return srv, nil
}

func (s *Server) registerHandlers() {
	s.service.Handle(ueventapi.RequestListDevices, s.handleListDevices)
	s.service.Handle(ueventapi.RequestGetDevice, s.handleGetDevice)
	s.service.Handle(ueventapi.RequestTrigger, s.handleTrigger)
}

func (s *Server) handleListDevices(t *sutra.Transaction) ([]byte, error) {
	s.deviceMu.RLock()
	devices := make([]ueventapi.DeviceInfo, 0, len(s.devices))
	for _, dev := range s.devices {
		devices = append(devices, dev)
	}
	s.deviceMu.RUnlock()
	return ueventapi.EncodeDeviceList(devices), nil
}

func (s *Server) handleGetDevice(t *sutra.Transaction) ([]byte, error) {
	req := ueventapi.DecodeDeviceInfo(t.Payload)
	s.deviceMu.RLock()
	dev, ok := s.devices[req.DevPath]
	s.deviceMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("device not found: %s", req.DevPath)
	}
	return dev.Encode(), nil
}

func (s *Server) handleTrigger(t *sutra.Transaction) ([]byte, error) {
	d := sutra.NewDecoder(t.Payload)
	subsystem := d.String()

	go func() {
		if subsystem == "" || subsystem == "*" {
			serviceLog.Info("triggering all devices")
			if err := triggerAll(); err != nil {
				serviceLog.Error("trigger all: %v", err)
			}
		} else {
			serviceLog.Info("triggering subsystem: %s", subsystem)
			if err := triggerSubsystem(subsystem); err != nil {
				serviceLog.Error("trigger %s: %v", subsystem, err)
			}
		}
	}()

	return nil, nil
}

func (s *Server) Run() error {
	// Coldplug: trigger existing devices
	go func() {
		serviceLog.Info("starting coldplug scan")
		if err := triggerAll(); err != nil {
			serviceLog.Warn("coldplug: %v", err)
		}
		serviceLog.Info("coldplug scan complete")
	}()

	// Main event loop: read from netlink
	for {
		ev, err := recvUEvent(s.nlFD)
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.done:
				return nil
			default:
			}
			serviceLog.Error("netlink recv: %v", err)
			continue
		}

		s.processEvent(ev)
	}
}

func (s *Server) processEvent(ev *UEvent) {
	serviceLog.Debug("%s %s [%s] dev=%s", ev.Action, ev.DevPath, ev.Subsystem, ev.DevName)

	// Track device in our registry
	s.updateDeviceRegistry(ev)

	// Find matching rule (first match wins, then fall through to defaults)
	matched := false
	for i := range s.rules {
		if matchRule(&s.rules[i], ev) {
			applyRule(&s.rules[i], ev)
			matched = true
			break
		}
	}

	// Apply default device node creation if no rule matched
	if !matched {
		defaultRule := Rule{Mode: 0660}
		applyRule(&defaultRule, ev)
	}

	// Broadcast event to sutra clients
	s.broadcastEvent(ev)
}

func (s *Server) updateDeviceRegistry(ev *UEvent) {
	major, _ := strconv.Atoi(ev.Major)
	minor, _ := strconv.Atoi(ev.Minor)

	switch ev.Action {
	case "add", "change":
		info := ueventapi.DeviceInfo{
			DevPath:   ev.DevPath,
			DevName:   ev.DevName,
			Subsystem: ev.Subsystem,
			DevType:   ev.DevType,
			Driver:    ev.Driver,
			Major:     major,
			Minor:     minor,
		}
		s.deviceMu.Lock()
		s.devices[ev.DevPath] = info
		s.deviceMu.Unlock()

	case "remove":
		s.deviceMu.Lock()
		delete(s.devices, ev.DevPath)
		s.deviceMu.Unlock()
	}
}

func (s *Server) broadcastEvent(ev *UEvent) {
	major, _ := strconv.Atoi(ev.Major)
	minor, _ := strconv.Atoi(ev.Minor)

	devEv := ueventapi.DeviceEvent{
		Action:    ev.Action,
		Subsystem: ev.Subsystem,
		DevPath:   ev.DevPath,
		DevName:   ev.DevName,
		DevType:   ev.DevType,
		Major:     major,
		Minor:     minor,
	}

	var eventID uint16
	switch ev.Action {
	case "add":
		eventID = ueventapi.EventDeviceAdded
	case "remove":
		eventID = ueventapi.EventDeviceRemoved
	case "change":
		eventID = ueventapi.EventDeviceChanged
	default:
		return
	}

	_ = s.service.Broadcast(eventID, devEv.Encode())
}

func (s *Server) Close() error {
	close(s.done)
	syscall.Close(s.nlFD)
	return s.service.Close()
}

// defaultRules returns minimal fallback rules when no config is found.
func defaultRules() []Rule {
	return []Rule{
		{Name: "null", Subsystem: "mem", DevName: "null", Mode: 0666},
		{Name: "zero", Subsystem: "mem", DevName: "zero", Mode: 0666},
		{Name: "full", Subsystem: "mem", DevName: "full", Mode: 0666},
		{Name: "random", Subsystem: "mem", DevName: "random", Mode: 0666},
		{Name: "urandom", Subsystem: "mem", DevName: "urandom", Mode: 0666},
		{Name: "tty-default", Subsystem: "tty", Mode: 0660, Group: 5},
		{Name: "block-default", Subsystem: "block", Mode: 0660, Group: 6},
		{Name: "input-default", Subsystem: "input", Mode: 0660, Group: 13},
		{Name: "video-default", Subsystem: "video4linux", Mode: 0660, Group: 12},
		{Name: "sound-default", Subsystem: "sound", Mode: 0660, Group: 11},
		{Name: "drm-default", Subsystem: "drm", Mode: 0660, Group: 12},
	}
}
