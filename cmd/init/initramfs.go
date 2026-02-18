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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"avyos.dev/pkg/fs"
)

var (
	rootfs      string
	rootfsType  string = "btrfs"
	avyosfs     string
	avysofsType string = "squashfs"
	live        bool
)

func isInsideInitramfs() bool {
	return os.Args[0] == "/init"
}

func ensureRealRootfs() {
	if !isInsideInitramfs() {
		return
	}

	// Mount essential filesystems
	mountEssentialFilesystems()

	// Parser kernel cmdline args
	parseKernelFlags()

	// no rootfs specified
	if rootfs == "" {
		panic("no root device specified")
	}
	safeMount(rootfs, "/rootfs", rootfsType, "", 0)

	if avyosfs == "" {
		if !fs.Exists("/rootfs/avyos") {
			panic("avyos not found")
		}
	} else {
		safeMount(avyosfs, "/rootfs/avyos", avysofsType, "", syscall.MS_RDONLY)
	}

	for _, fs := range []string{
		fs.ProcessesPath,
		fs.SysfsPath,
		fs.DevicesPath,
		fs.RuntimePath,
	} {
		os.MkdirAll("/rootfs/"+fs, 0755)
		syscall.Mount("/"+fs, "/rootfs/"+fs, "", syscall.MS_MOVE, "")
	}

	syscall.Chdir("/rootfs")
	syscall.Chroot("/rootfs")

	if err := syscall.Exec(filepath.Join(fs.AvyosPath, "cmd", "init"), []string{filepath.Join(fs.AvyosPath, "cmd", "init")}, []string{}); err != nil {
		panic(err)
	}
}

func parseKernelFlags() error {
	data, err := os.ReadFile(fs.Resolve("process", "cmdline"))
	if err != nil {
		return fmt.Errorf("failed to read kernel cmdline flags %v", err)
	}

	for _, a := range strings.Fields(string(data)) {
		k, v := a, ""
		if i := strings.Index(k, "="); i != -1 {
			v = k[i+1:]
			k = k[:i]
		}

		switch k {
		case "root":
			rootfs = resolve(v)
		case "rootfstype":
			rootfsType = v
		case "avyos":
			avyosfs = resolve(v)
		case "avyosfstype":
			avysofsType = v
		case "live":
			live = true
		}
	}

	return nil
}

func resolve(s string) string {
	idx := strings.Index(s, ":")
	if idx == -1 {
		return s
	}
	kind := s[:idx]
	s = s[idx+1:]
	switch kind {
	case "device":
		return fs.Resolve(kind, s)
	}
	return s
}

func safeMount(source, target, fstype, options string, flags uintptr) {
	if _, err := os.Stat(source); err != nil {
		blocks, err := os.ReadDir(fs.Resolve("sysfs", "block"))
		if err != nil {
			panic("failed to read sysfs:block " + err.Error())
		}
		for i, block := range blocks {
			fmt.Println(i, block.Name())
		}
		panic("no source device present at " + source)
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		panic(err)
	}

	if err := syscall.Mount(source, target, fstype, flags, options); err != nil {
		panic(err)
	}
}

func mountEssentialFilesystems() {
	mounts := []struct {
		source string
		target string
		fstype string
		flags  uintptr
		data   string
	}{
		{"proc", fs.ProcessesPath, "proc", 0, ""},
		{"sysfs", fs.SysfsPath, "sysfs", 0, ""},
		{"devtmpfs", fs.DevicesPath, "devtmpfs", 0, ""},
		{"devpts", fs.DevicesPath + "/pts", "devpts", 0, "ptmxmode=0666,mode=0620"},
		{"tmpfs", fs.RuntimePath, "tmpfs", 0, ""},
	}

	for _, m := range mounts {
		_ = os.MkdirAll(m.target, 0755)

		if isMounted(m.target) {
			continue
		}

		err := syscall.Mount(m.source, m.target, m.fstype, m.flags, m.data)
		if err != nil {
			fmt.Printf("init: Failed to mount %s: %v\n", m.target, err)
		}
	}
}

func isMounted(path string) bool {
	file, err := os.Open(fs.Resolve("process", "mounts"))
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
	}
	return false
}
