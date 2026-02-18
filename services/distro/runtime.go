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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	distroapi "avyos.dev/api/distro"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/ini"
	avynet "avyos.dev/pkg/net"
)

const defaultShell = "/bin/sh"
const distroConfigName = "distro.ini"

var (
	linuxBase = "/linux"
)

// Distro represents a Linux distribution configuration.
type Distro struct {
	Name    string
	URL     string
	Version string
}

// DistroRegistry holds available distros.
type DistroRegistry struct {
	Distros []Distro
}

var archMapping = map[string]string{
	"arm64": "aarch64",
	"amd64": "x86_64",
}

var defaultDistros = DistroRegistry{
	Distros: []Distro{
		{
			Name:    "debian",
			URL:     "https://cdimage.debian.org/cdimage/cloud/bookworm/latest/debian-12-genericcloud-<goarch>.tar.gz",
			Version: "12",
		},
		{
			Name:    "ubuntu",
			URL:     "https://cdimage.ubuntu.com/ubuntu-base/releases/24.04/release/ubuntu-base-24.04.1-base-<goarch>.tar.gz",
			Version: "24.04",
		},
		{
			Name:    "arch",
			URL:     "https://archive.archlinux.org/iso/latest/archlinux-bootstrap-<arch>.tar.gz",
			Version: "rolling",
		},
		{
			Name:    "alpine",
			URL:     "https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/<arch>/alpine-minirootfs-3.20.3-<arch>.tar.gz",
			Version: "3.20",
		},
	},
}

func init() {
	if os.Geteuid() != 0 {
		linuxBase = filepath.Join(os.Getenv("HOME"), "Linux")
	}
}

func mappedArch() string {
	if arch, ok := archMapping[runtime.GOARCH]; ok {
		return arch
	}
	return runtime.GOARCH
}

func loadDistroRegistry() (DistroRegistry, error) {
	path := fs.Resolve("config", distroConfigName)
	if !fs.Exists(path) {
		return resolveRegistry(defaultDistros, nil), nil
	}

	cfg, err := ini.ParseFile(path)
	if err != nil {
		return DistroRegistry{}, fmt.Errorf("parse config:%s: %w", distroConfigName, err)
	}

	registry, err := parseDistroRegistry(cfg)
	if err != nil {
		return DistroRegistry{}, fmt.Errorf("parse config:%s: %w", distroConfigName, err)
	}

	return registry, nil
}

func parseDistroRegistry(cfg *ini.Config) (DistroRegistry, error) {
	registry := DistroRegistry{Distros: make([]Distro, 0)}
	seen := make(map[string]bool)

	for _, entry := range cfg.Entries {
		if entry.Type != ini.EntrySection {
			continue
		}
		section := strings.TrimSpace(entry.Section)
		if section == "" || seen[section] {
			continue
		}
		seen[section] = true

		if !isEnabled(cfg, section) {
			continue
		}

		url := strings.TrimSpace(firstNonEmpty(
			valueOrEmpty(cfg.Get(section, "url."+runtime.GOARCH)),
			valueOrEmpty(cfg.Get(section, "url")),
		))
		if url == "" {
			continue
		}

		version := strings.TrimSpace(valueOrEmpty(cfg.Get(section, "version")))
		url = expandArchPlaceholders(cfg, section, url)

		registry.Distros = append(registry.Distros, Distro{
			Name:    section,
			URL:     url,
			Version: version,
		})
	}

	if len(registry.Distros) == 0 {
		return DistroRegistry{}, fmt.Errorf("no distro entries found")
	}

	return registry, nil
}

func resolveRegistry(registry DistroRegistry, cfg *ini.Config) DistroRegistry {
	out := DistroRegistry{Distros: make([]Distro, 0, len(registry.Distros))}
	for _, distro := range registry.Distros {
		d := distro
		section := distro.Name
		if cfg == nil {
			d.URL = strings.ReplaceAll(d.URL, "<arch>", mappedArch())
			d.URL = strings.ReplaceAll(d.URL, "<goarch>", runtime.GOARCH)
		} else {
			d.URL = expandArchPlaceholders(cfg, section, d.URL)
		}
		out.Distros = append(out.Distros, d)
	}
	return out
}

func expandArchPlaceholders(cfg *ini.Config, section, url string) string {
	arch := mappedArch()
	if cfg != nil {
		if v, ok := cfg.Get(section, "arch."+runtime.GOARCH); ok {
			if value := strings.TrimSpace(v); value != "" {
				arch = value
			}
		} else if v, ok := cfg.Get(section, "arch"); ok {
			if value := strings.TrimSpace(v); value != "" {
				arch = value
			}
		}
	}
	url = strings.ReplaceAll(url, "<arch>", arch)
	url = strings.ReplaceAll(url, "<goarch>", runtime.GOARCH)
	return url
}

func isEnabled(cfg *ini.Config, section string) bool {
	v, ok := cfg.Get(section, "enabled")
	if !ok {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func valueOrEmpty(value string, ok bool) string {
	if !ok {
		return ""
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func listDistros(showAvailable bool) ([]distroapi.DistroInfo, error) {
	if showAvailable {
		registry, err := loadDistroRegistry()
		if err != nil {
			return nil, err
		}
		items := make([]distroapi.DistroInfo, 0, len(registry.Distros))
		for _, distro := range registry.Distros {
			items = append(items, distroapi.DistroInfo{
				Name:    distro.Name,
				Version: distro.Version,
				URL:     distro.URL,
			})
		}
		return items, nil
	}

	entries, err := os.ReadDir(linuxBase)
	if err != nil {
		if os.IsNotExist(err) {
			return []distroapi.DistroInfo{}, nil
		}
		return nil, err
	}

	items := make([]distroapi.DistroInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(linuxBase, entry.Name())
		items = append(items, distroapi.DistroInfo{
			Name: entry.Name(),
			Path: path,
			Size: getDirSize(path),
		})
	}

	return items, nil
}

func pullDistro(name, customURL string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("distro name required")
	}
	customURL = strings.TrimSpace(customURL)

	url := customURL
	if url == "" {
		registry, err := loadDistroRegistry()
		if err != nil {
			return err
		}
		for _, distro := range registry.Distros {
			if distro.Name == name {
				url = distro.URL
				break
			}
		}
		if url == "" {
			return fmt.Errorf("unknown distro: %s (check config:%s or use --url)", name, distroConfigName)
		}
	}

	targetDir := filepath.Join(linuxBase, name)
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("distro %s already exists at %s", name, targetDir)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := downloadAndExtract(url, targetDir); err != nil {
		_ = os.RemoveAll(targetDir)
		return fmt.Errorf("failed to download: %w", err)
	}

	for _, dir := range []string{"proc", "sys", "dev", "tmp", "root"} {
		_ = os.MkdirAll(filepath.Join(targetDir, dir), 0755)
	}

	return nil
}

func runContainer(req distroapi.RunRequest, uid uint32) (distroapi.RunResult, error) {
	req.Distro = strings.TrimSpace(req.Distro)
	if req.Distro == "" {
		return distroapi.RunResult{}, fmt.Errorf("distro name required")
	}

	rootfs := filepath.Join(linuxBase, req.Distro)
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return distroapi.RunResult{}, fmt.Errorf("distro %s not found (use 'distro pull %s' first)", req.Distro, req.Distro)
	}

	workdir := strings.TrimSpace(req.Workdir)
	if workdir == "" {
		workdir = "/"
	}

	waylandBridge, err := newWaylandBridge(rootfs, uid)
	if err != nil {
		return distroapi.RunResult{}, fmt.Errorf("setup wayland bridge: %w", err)
	}
	defer waylandBridge.Close()

	command := distroapi.DecodeCommand(req.Command)
	return execContainer(rootfs, command, workdir, req.Bind, req.Env, req.Input, waylandBridge.Env())
}

func removeDistro(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("distro name required")
	}

	targetDir := filepath.Join(linuxBase, name)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return fmt.Errorf("distro %s not found", name)
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("failed to remove: %w", err)
	}
	return nil
}

func runInit() error {
	rootfs := os.Getenv("DISTRO_ROOTFS")
	command := distroapi.DecodeCommand(os.Getenv("DISTRO_COMMAND"))
	workdir := os.Getenv("DISTRO_WORKDIR")
	bind := os.Getenv("DISTRO_BIND")

	if rootfs == "" || len(command) == 0 {
		return fmt.Errorf("invalid distro configuration")
	}
	if workdir == "" {
		workdir = "/"
	}

	if err := setupContainerFS(rootfs, bind); err != nil {
		return fmt.Errorf("failed to setup filesystem: %w", err)
	}

	if err := pivotRoot(rootfs); err != nil {
		return fmt.Errorf("failed to pivot root: %w", err)
	}

	if err := os.Chdir(workdir); err != nil {
		_ = os.Chdir("/")
	}

	path, err := exec.LookPath(command[0])
	if err != nil {
		path = command[0]
	}

	return syscall.Exec(path, command, os.Environ())
}

func execContainer(rootfs string, command []string, workdir, bind, envVar string, input []byte, extraEnv []string) (distroapi.RunResult, error) {
	exePath, err := os.Readlink(fs.Resolve("process", "self/exe"))
	if err != nil {
		return distroapi.RunResult{}, fmt.Errorf("resolve executable path: %w", err)
	}

	cmd := exec.Command(exePath, "init")
	cmd.Stdin = bytes.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Env = append(os.Environ(),
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		"USER=root",
		"LOGNAME=root",
		"TERM=xterm-256color",
		"LANG=C.UTF-8",
		"DISTRO_ROOTFS="+rootfs,
		"DISTRO_COMMAND="+distroapi.EncodeCommand(command),
		"DISTRO_WORKDIR="+workdir,
		"DISTRO_BIND="+bind,
	)
	if len(extraEnv) > 0 {
		cmd.Env = append(cmd.Env, extraEnv...)
	}
	if strings.TrimSpace(envVar) != "" {
		cmd.Env = append(cmd.Env, envVar)
	}

	attr := &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}
	if os.Geteuid() != 0 {
		attr.Cloneflags |= syscall.CLONE_NEWUSER
		attr.UidMappings = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Geteuid(), Size: 1},
		}
		attr.GidMappings = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getegid(), Size: 1},
		}
		attr.GidMappingsEnableSetgroups = false
	}
	cmd.SysProcAttr = attr

	result := distroapi.RunResult{}
	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return distroapi.RunResult{}, err
		}
	}

	if result.ExitCode == 0 {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	result.Stdout = append(result.Stdout[:0], stdout.Bytes()...)
	result.Stderr = append(result.Stderr[:0], stderr.Bytes()...)
	return result, nil
}

func setupContainerFS(rootfs, bind string) error {
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to do private mount: %w", err)
	}
	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("bind mount rootfs: %w", err)
	}

	processRoot := strings.TrimPrefix(fs.Resolve("process"), "/")
	procPath := filepath.Join(rootfs, processRoot)
	_ = os.MkdirAll(procPath, 0755)
	_ = syscall.Mount("proc", procPath, "proc", 0, "")

	sysPath := filepath.Join(rootfs, "sys")
	_ = os.MkdirAll(sysPath, 0755)
	_ = syscall.Mount("sysfs", sysPath, "sysfs", syscall.MS_RDONLY, "")

	tmpPath := filepath.Join(rootfs, "tmp")
	_ = os.MkdirAll(tmpPath, 0777)
	_ = syscall.Mount("tmpfs", tmpPath, "tmpfs", 0, "")

	devPath := filepath.Join(rootfs, "dev")
	_ = os.MkdirAll(devPath, 0755)
	_ = syscall.Mount("tmpfs", devPath, "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")

	for _, dev := range []string{"null", "zero", "random", "urandom", "tty"} {
		src := fs.Resolve("device", dev)
		dst := filepath.Join(devPath, dev)
		if _, err := os.Stat(src); err == nil {
			f, _ := os.Create(dst)
			if f != nil {
				_ = f.Close()
				_ = syscall.Mount(src, dst, "", syscall.MS_BIND, "")
			}
		}
	}

	ptsPath := filepath.Join(devPath, "pts")
	_ = os.MkdirAll(ptsPath, 0755)
	_ = syscall.Mount("devpts", ptsPath, "devpts", 0, "newinstance,ptmxmode=0666")
	ptmxPath := filepath.Join(devPath, "ptmx")
	_ = os.Remove(ptmxPath)
	_ = os.Symlink("pts/ptmx", ptmxPath)

	_ = symlinkIfMissing(fs.Resolve("process", "self/fd"), filepath.Join(devPath, "fd"))
	_ = symlinkIfMissing(fs.Resolve("process", "self/fd/0"), filepath.Join(devPath, "stdin"))
	_ = symlinkIfMissing(fs.Resolve("process", "self/fd/1"), filepath.Join(devPath, "stdout"))
	_ = symlinkIfMissing(fs.Resolve("process", "self/fd/2"), filepath.Join(devPath, "stderr"))

	if bind != "" {
		parts := strings.SplitN(bind, ":", 2)
		if len(parts) == 2 {
			src := strings.TrimSpace(parts[0])
			dst := filepath.Join(rootfs, strings.TrimSpace(parts[1]))
			if src != "" {
				_ = os.MkdirAll(dst, 0755)
				_ = syscall.Mount(src, dst, "", syscall.MS_BIND, "")
			}
		}
	}

	generateResolvConf(rootfs)

	return nil
}

func symlinkIfMissing(target, linkName string) error {
	if _, err := os.Lstat(linkName); err == nil {
		return nil
	}
	return os.Symlink(target, linkName)
}

func pivotRoot(rootfs string) error {
	oldRoot := filepath.Join(rootfs, ".old_root")
	if err := os.MkdirAll(oldRoot, 0700); err != nil {
		return err
	}

	if err := syscall.PivotRoot(rootfs, oldRoot); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return err
	}

	oldRootNew := "/.old_root"
	if err := syscall.Unmount(oldRootNew, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root: %w", err)
	}

	_ = os.RemoveAll(oldRootNew)
	return nil
}

func downloadAndExtract(url, targetDir string) error {
	client := avynet.NewClient()

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http error: %s", resp.Status)
	}

	if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		return extractTarGz(resp.Body, targetDir)
	}

	binPath := filepath.Join(targetDir, "bin")
	_ = os.MkdirAll(binPath, 0755)

	parts := strings.Split(url, "/")
	filename := parts[len(parts)-1]
	if filename == "" {
		filename = "binary"
	}

	outFile, err := os.Create(filepath.Join(binPath, filename))
	if err != nil {
		return err
	}
	defer outFile.Close()
	_ = outFile.Chmod(0755)
	_, err = io.Copy(outFile, resp.Body)
	return err
}

func extractTarGz(r io.Reader, targetDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	base := filepath.Clean(targetDir)
	prefix := base + string(os.PathSeparator)
	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(targetDir, header.Name)
		cleanTarget := filepath.Clean(target)
		if cleanTarget != base && !strings.HasPrefix(cleanTarget, prefix) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(cleanTarget, os.FileMode(header.Mode))
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(cleanTarget), 0755)
			f, err := os.Create(cleanTarget)
			if err != nil {
				continue
			}
			_, _ = io.Copy(f, tr)
			_ = f.Chmod(os.FileMode(header.Mode))
			_ = f.Close()
		case tar.TypeSymlink:
			_ = os.MkdirAll(filepath.Dir(cleanTarget), 0755)
			_ = os.Symlink(header.Linkname, cleanTarget)
		case tar.TypeLink:
			_ = os.MkdirAll(filepath.Dir(cleanTarget), 0755)
			linkTarget := filepath.Join(targetDir, header.Linkname)
			_ = os.Link(linkTarget, cleanTarget)
		}
	}

	return nil
}

func getDirSize(path string) int {
	total := 0
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += int(info.Size())
		}
		return nil
	})
	return total
}

// generateResolvConf writes /etc/resolv.conf inside the container rootfs
// using DNS servers from /config/net.conf.
func generateResolvConf(rootfs string) {
	netConf := fs.Resolve("config", "net.conf")
	cfg, err := ini.ParseFile(netConf)
	if err != nil {
		return
	}

	servers, ok := cfg.Get("dns", "servers")
	if !ok || strings.TrimSpace(servers) == "" {
		return
	}

	etcDir := filepath.Join(rootfs, "etc")
	_ = os.MkdirAll(etcDir, 0755)

	var buf strings.Builder
	buf.WriteString("# Generated by AvyOS distro service\n")
	for _, s := range strings.Split(servers, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			buf.WriteString("nameserver ")
			buf.WriteString(s)
			buf.WriteByte('\n')
		}
	}

	_ = os.WriteFile(filepath.Join(etcDir, "resolv.conf"), []byte(buf.String()), 0644)
}
