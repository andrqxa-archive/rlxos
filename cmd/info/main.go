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
	"path/filepath"
	"time"

	"avyos.dev/pkg/format"
	"avyos.dev/pkg/fs"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "info - Show filesystem metadata for a path")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  info <path>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  (none)")
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
		return fmt.Errorf("usage: info <path>")
	}

	path := args[0]
	info, err := fs.Info(path)
	if err != nil {
		return err
	}

	absPath, _ := filepath.Abs(path)

	fmt.Printf("Name:        %s\n", info.Name)
	fmt.Printf("Path:        %s\n", absPath)
	fmt.Printf("Type:        %s\n", fileType(info))
	fmt.Printf("Size:        %s (%d bytes)\n", format.Size(info.Size), info.Size)
	fmt.Printf("Permissions: %s\n", fs.PermString(info.Mode))
	fmt.Printf("Modified:    %s\n", time.Unix(info.ModTime, 0).Format("2006-01-02 15:04:05"))

	if info.IsLink && info.Target != "" {
		fmt.Printf("Target:      %s\n", info.Target)
	}

	return nil
}

func fileType(info *fs.FileInfo) string {
	if info.IsLink {
		return "symbolic link"
	}
	if info.IsDir {
		return "directory"
	}
	return "regular file"
}
