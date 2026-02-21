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

	distroapi "avyos.dev/api/distro"
	"avyos.dev/pkg/sutra"
)

type Handler struct {
	service *sutra.Service
	shells  *shellSessionManager
}

func (h *Handler) List(sender uint32, req distroapi.ListRequest) (distroapi.DistroList, error) {
	_ = sender
	items, err := listDistros(req.Available)
	if err != nil {
		return distroapi.DistroList{}, err
	}
	return distroapi.DistroList{Items: items}, nil
}

func (h *Handler) Pull(sender uint32, req distroapi.PullRequest) (distroapi.Empty, error) {
	_ = sender
	if err := pullDistro(req.Name, req.URL); err != nil {
		return distroapi.Empty{}, err
	}
	return distroapi.Empty{}, nil
}

func (h *Handler) Run(sender uint32, req distroapi.RunRequest) (distroapi.RunResult, error) {
	uid, err := h.callerUID(sender)
	if err != nil {
		return distroapi.RunResult{}, err
	}
	return runContainer(req, uid)
}

func (h *Handler) Remove(sender uint32, req distroapi.RemoveRequest) (distroapi.Empty, error) {
	_ = sender
	if err := removeDistro(req.Name); err != nil {
		return distroapi.Empty{}, err
	}
	return distroapi.Empty{}, nil
}

func (h *Handler) ShellOpen(sender uint32, req distroapi.ShellOpenRequest) (distroapi.ShellSession, error) {
	uid, err := h.callerUID(sender)
	if err != nil {
		return distroapi.ShellSession{}, err
	}
	return h.shells.Open(sender, uid, req)
}

func (h *Handler) ShellInput(sender uint32, req distroapi.ShellInputRequest) error {
	return h.shells.Input(sender, req)
}

func (h *Handler) ShellResize(sender uint32, req distroapi.ShellResizeRequest) error {
	return h.shells.Resize(sender, req)
}

func (h *Handler) ShellClose(sender uint32, req distroapi.ShellCloseRequest) error {
	return h.shells.Close(sender, req)
}

func (h *Handler) callerUID(sender uint32) (uint32, error) {
	uid, ok := h.service.GetClientUID(sender)
	if !ok {
		return 0, fmt.Errorf("unable to resolve caller uid")
	}
	return uid, nil
}
