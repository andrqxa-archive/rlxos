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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"avyos.dev/pkg/appcatalog"
	"avyos.dev/pkg/format"
	"avyos.dev/pkg/identity"
)

var flagApp string

func init() {
	flag.StringVar(&flagApp, "app", "", "Force app id instead of extension-based matching")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "open - Open files or directories with associated apps")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  open <path> [--app=<id>]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  (none)")
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/command error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

func main() {
	flag.Parse()
	if err := run(flag.Args()); err != nil {
		format.Error("%s", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: open <path> [--app=<id>]")
	}

	targetPath := resolvePath(args[0])
	if targetPath == "" {
		return fmt.Errorf("invalid path")
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return err
	}

	entries := appcatalog.Discover(appcatalog.DiscoverOptions{
		Home:          resolveHomeDir(),
		IncludeHidden: true,
	})
	if len(entries) == 0 {
		return fmt.Errorf("no applications discovered")
	}

	forced := strings.TrimSpace(flagApp)
	entry, err := resolveTargetApp(entries, targetPath, info.IsDir(), forced)
	if err != nil {
		return err
	}
	if strings.TrimSpace(entry.ExecPath) == "" {
		return fmt.Errorf("app %q is missing executable path", entry.ID)
	}

	cmd := exec.Command(entry.ExecPath, targetPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open with %s: %w", entry.Name, err)
	}
	return nil
}

func resolveTargetApp(entries []appcatalog.Entry, path string, isDir bool, forced string) (appcatalog.Entry, error) {
	if forced != "" {
		if entry, ok := appcatalog.FindByID(entries, forced); ok {
			return entry, nil
		}
		return appcatalog.Entry{}, fmt.Errorf("app not found: %s", forced)
	}

	if isDir {
		if entry, ok := appcatalog.FindByID(entries, "dev.avyos.filemanager"); ok {
			return entry, nil
		}
		return appcatalog.Entry{}, fmt.Errorf("file manager app not found")
	}

	if entry, ok := appcatalog.FindByFilePath(entries, path); ok {
		return entry, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return appcatalog.Entry{}, fmt.Errorf("no associated app for files without extension")
	}
	return appcatalog.Entry{}, fmt.Errorf("no app supports %s files", ext)
}

func resolvePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "~/") {
		home := resolveHomeDir()
		if home != "" {
			raw = filepath.Join(home, raw[2:])
		}
	} else if raw == "~" {
		home := resolveHomeDir()
		if home != "" {
			raw = home
		}
	}

	if !filepath.IsAbs(raw) {
		if cwd, err := os.Getwd(); err == nil {
			raw = filepath.Join(cwd, raw)
		}
	}
	return filepath.Clean(raw)
}

func resolveHomeDir() string {
	if id, err := identity.LookupByID(uint(os.Getuid())); err == nil {
		home := filepath.Clean(strings.TrimSpace(id.Home))
		if home != "" && home != "/users" {
			return home
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		home = filepath.Clean(strings.TrimSpace(home))
		if home != "" && home != "/users" {
			return home
		}
	}

	for _, key := range []string{"USER", "LOGNAME"} {
		name := strings.TrimSpace(os.Getenv(key))
		if name == "" {
			continue
		}
		candidate := filepath.Join("/users", name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}
