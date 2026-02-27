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
	"os"
	"strings"

	gapp "avyos.dev/pkg/graphics/app"
	"avyos.dev/pkg/graphics/backend/drm"
	"avyos.dev/pkg/graphics/backend/framebuffer"
	graphics "avyos.dev/pkg/graphics/input"
	"avyos.dev/pkg/graphics/input/evdev"
	"avyos.dev/pkg/logger"
)

var serviceLog = logger.New("display")

func main() {
	if err := logger.SetupSystemLog(); err != nil {
		serviceLog.Error("failed to setup system log: %v", err)
	}
	serviceLog.SetLevel(logger.INFO)
	if err := gapp.ApplyConfiguredDefaultFont(); err != nil {
		serviceLog.Error("failed to load configured fonts: %v", err)
	}

	fb, err := selectBackend()
	if err != nil {
		serviceLog.Error("failed to select graphics backend: %v", err)
		os.Exit(1)
	}

	input := evdev.NewHandler()

	srv := NewServer(fb, input)

	serviceLog.Info("starting display service")

	if err := srv.Run(); err != nil {
		serviceLog.Error("display service error: %v", err)
		os.Exit(1)
	}

	serviceLog.Info("display service stopped")
}

func selectBackend() (graphics.Backend, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("AVYOS_DISPLAY_BACKEND")))
	serviceLog.Info("graphics backend requested: %s", modeOrAuto(mode))
	switch mode {
	case "", "auto":
		return &fallbackBackend{
			order: []graphics.Backend{
				drm.New(),
				framebuffer.New(),
			},
		}, nil
	case "drm", "drmkms":
		return drm.New(), nil
	case "framebuffer", "fb":
		return framebuffer.New(), nil
	default:
		return nil, fmt.Errorf("unknown backend %q (supported: auto, drm, framebuffer)", mode)
	}
}

func modeOrAuto(mode string) string {
	if mode == "" {
		return "auto"
	}
	return mode
}

type fallbackBackend struct {
	order  []graphics.Backend
	active graphics.Backend
}

func (b *fallbackBackend) Open() error {
	if b.active != nil {
		return nil
	}
	var errs []string
	for _, candidate := range b.order {
		if err := candidate.Open(); err != nil {
			errs = append(errs, fmt.Sprintf("%T: %v", candidate, err))
			serviceLog.Warn("graphics backend %T unavailable: %v", candidate, err)
			continue
		}
		b.active = candidate
		serviceLog.Info("graphics backend active: %s", b.active.Info())
		return nil
	}
	return fmt.Errorf("no graphics backend available: %s", strings.Join(errs, "; "))
}

func (b *fallbackBackend) Close() error {
	if b.active == nil {
		return nil
	}
	err := b.active.Close()
	b.active = nil
	return err
}

func (b *fallbackBackend) Size() (int, int) {
	if b.active == nil {
		return 0, 0
	}
	return b.active.Size()
}

func (b *fallbackBackend) Buffer() *graphics.Buffer {
	if b.active == nil {
		return nil
	}
	return b.active.Buffer()
}

func (b *fallbackBackend) Flush() error {
	if b.active == nil {
		return fmt.Errorf("backend not open")
	}
	return b.active.Flush()
}

func (b *fallbackBackend) FlushRect(r graphics.Rect) error {
	if b.active == nil {
		return fmt.Errorf("backend not open")
	}
	return b.active.FlushRect(r)
}

func (b *fallbackBackend) Info() string {
	if b.active == nil {
		return "auto: not open"
	}
	return "auto -> " + b.active.Info()
}

func (b *fallbackBackend) HasSystemCursor() bool {
	if b.active == nil {
		return false
	}
	return b.active.HasSystemCursor()
}
