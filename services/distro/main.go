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
	"os"

	distroapi "avyos.dev/api/distro"
	"avyos.dev/pkg/logger"
	"avyos.dev/pkg/sutra"
)

var serviceLog = logger.New("distro")

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInit(); err != nil {
			serviceLog.Error("distro init failed: %v", err)
			os.Exit(1)
		}
		return
	}

	if err := logger.SetupSystemLog(); err != nil {
		serviceLog.Error("failed to setup system log: %v", err)
	}

	svc, err := sutra.NewService(distroapi.ServiceName, "")
	if err != nil {
		serviceLog.Error("failed to create service: %v", err)
		os.Exit(1)
	}
	defer svc.Close()

	handler := &Handler{
		service: svc,
		shells:  newShellSessionManager(svc),
	}
	RegisterHandlers(svc, handler)
	svc.Handle(sutra.EventDisconnect, func(t *sutra.Transaction) ([]byte, error) {
		handler.shells.CloseByOwner(t.PayloadUint32())
		return nil, nil
	})

	serviceLog.Info("distro service ready")
	select {}
}
