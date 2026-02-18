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
	"time"

	serviceapi "avyos.dev/api/service"
	"avyos.dev/pkg/sutra"
)

var managerService *sutra.Service

func setupServiceManager() {
	svc, err := sutra.NewService(serviceapi.ServiceName, "")
	if err != nil {
		log.Error("Failed to start service manager API: %v", err)
		return
	}

	svc.Handle(serviceapi.RequestStart, handleServiceStart)
	svc.Handle(serviceapi.RequestStop, handleServiceStop)
	svc.Handle(serviceapi.RequestRestart, handleServiceRestart)
	svc.Handle(serviceapi.RequestStatus, handleServiceStatus)
	svc.Handle(serviceapi.RequestList, handleServiceList)
	svc.Handle(serviceapi.RequestPoweroff, handleServicePoweroff)
	svc.Handle(serviceapi.RequestReboot, handleServiceReboot)

	managerService = svc
	log.Info("Service manager API ready at %s", serviceapi.ServiceName)
}

func handleServiceStart(t *sutra.Transaction) ([]byte, error) {
	name, err := serviceapi.DecodeServiceName(t.Payload)
	if err != nil {
		return nil, err
	}
	return nil, sv.StartService(name)
}

func handleServiceStop(t *sutra.Transaction) ([]byte, error) {
	name, err := serviceapi.DecodeServiceName(t.Payload)
	if err != nil {
		return nil, err
	}
	return nil, sv.StopService(name)
}

func handleServiceRestart(t *sutra.Transaction) ([]byte, error) {
	name, err := serviceapi.DecodeServiceName(t.Payload)
	if err != nil {
		return nil, err
	}
	return nil, sv.RestartService(name)
}

func handleServiceStatus(t *sutra.Transaction) ([]byte, error) {
	name, err := serviceapi.DecodeServiceName(t.Payload)
	if err != nil {
		return nil, err
	}

	status, err := sv.GetServiceStatus(name)
	if err != nil {
		return nil, err
	}

	return serviceapi.ServiceStatus{
		Name:        status.Name,
		Description: status.Description,
		Type:        status.Type,
		Restart:     status.Restart,
		Running:     status.Running,
		Started:     status.Started,
		Failed:      status.Failed,
		PID:         status.PID,
	}.Encode(), nil
}

func handleServiceList(t *sutra.Transaction) ([]byte, error) {
	_ = t
	items := sv.ListServices()
	out := make([]serviceapi.ServiceStatus, 0, len(items))
	for _, item := range items {
		out = append(out, serviceapi.ServiceStatus{
			Name:        item.Name,
			Description: item.Description,
			Type:        item.Type,
			Restart:     item.Restart,
			Running:     item.Running,
			Started:     item.Started,
			Failed:      item.Failed,
			PID:         item.PID,
		})
	}
	return serviceapi.EncodeServiceStatusList(out), nil
}

func handleServicePoweroff(t *sutra.Transaction) ([]byte, error) {
	_ = t
	go func() {
		time.Sleep(200 * time.Millisecond)
		shutdownPoweroff()
	}()
	return serviceapi.Empty{}.MarshalBinary(), nil
}

func handleServiceReboot(t *sutra.Transaction) ([]byte, error) {
	_ = t
	go func() {
		time.Sleep(200 * time.Millisecond)
		shutdownReboot()
	}()
	return serviceapi.Empty{}.MarshalBinary(), nil
}
