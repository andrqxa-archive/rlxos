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

package fs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	AvyosPath     = "/avyos"
	CommandsPath  = "/cmd"
	ServicesPath  = "/services"
	CachePath     = "/cache"
	KernelPath    = "/cache/kernel"
	DevicesPath   = "/cache/kernel/devices"
	ProcessesPath = "/cache/kernel/processes"
	SysfsPath     = "/cache/kernel/sysfs"
	RuntimePath   = "/cache/runtime"
	UserRunPath   = "/cache/runtime/user"
	ConfigPath    = "/config"
	UsersHomePath = "/users"
)

func Resolve(kind string, p ...string) string {
	if len(p) == 0 {
		if strings.HasPrefix(kind, "service:") {
			name := strings.TrimPrefix(kind, "service:")
			return filepath.Join(RuntimePath, name+".sock")
		}
		if strings.HasPrefix(kind, "user-service:") {
			name := strings.TrimPrefix(kind, "user-service:")
			return filepath.Join(UserRunPath, strconv.Itoa(os.Getuid()), name+".sock")
		}
		return kind
	}

	pathArg := p[0]

	switch kind {
	case "cmd":
		path := filepath.Join(CommandsPath, pathArg)
		if !Exists(path) {
			path = filepath.Join(AvyosPath, CommandsPath, pathArg)
		}
		return path
	case "service":
		path := filepath.Join(ServicesPath, pathArg)
		if !Exists(path) {
			path = filepath.Join(AvyosPath, ServicesPath, pathArg)
		}
		return path
	case "cache":
		return filepath.Join(CachePath, pathArg)
	case "kernel":
		return filepath.Join(KernelPath, pathArg)
	case "device":
		return filepath.Join(DevicesPath, pathArg)
	case "process":
		return filepath.Join(ProcessesPath, pathArg)
	case "sysfs":
		return filepath.Join(SysfsPath, pathArg)
	case "run":
		return filepath.Join(RuntimePath, pathArg)
	case "user-service":
		name := pathArg
		if filepath.Ext(name) == "" {
			name += ".sock"
		}
		return filepath.Join(UserRunPath, strconv.Itoa(os.Getuid()), name)
	case "config":
		path := filepath.Join(ConfigPath, pathArg)
		if !Exists(path) {
			path = filepath.Join(AvyosPath, ConfigPath, pathArg)
		}
		return path
	case "user":
		return filepath.Join(UsersHomePath, pathArg)
	}
	return pathArg
}
