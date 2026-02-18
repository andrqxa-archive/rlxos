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
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"avyos.dev/pkg/fs"
)

type child struct {
	name string
	cmd  *exec.Cmd
}

func main() {
	log.SetPrefix("session: ")
	log.SetFlags(0)

	// Session components in start order.
	// Daemons run in the background; the last two (background, dock) are
	// the critical desktop processes — if either exits the session ends.
	type component struct {
		name     string
		args     []string
		daemon   bool // daemon = don't wait on exit
		optional bool // optional = warn on failure, continue
	}

	components := []component{
		{name: "settingsmanager", args: []string{"--daemon"}, daemon: true, optional: true},
		{name: "waylayer", daemon: true, optional: true},
		{name: "background", args: []string{"--mode", "layer"}},
		{name: "dock"},
	}

	// Set WAYLAND_DISPLAY for Wayland clients to find waylayer
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		os.Setenv("WAYLAND_DISPLAY", "wayland-0")
	}

	var children []*child
	var critical []*child

	for _, comp := range components {
		path := resolveApp(comp.name)
		cmd := exec.Command(path, comp.args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = os.Getenv("HOME")
		cmd.Env = os.Environ()
		cmd.SysProcAttr = &syscall.SysProcAttr{}

		if err := cmd.Start(); err != nil {
			if comp.optional {
				log.Printf("warning: failed to start %s: %v", comp.name, err)
				continue
			}
			// Critical component failed — tear down everything started so far
			log.Printf("failed to start %s: %v", comp.name, err)
			stopAll(children)
			os.Exit(1)
		}

		log.Printf("started %s pid=%d", comp.name, cmd.Process.Pid)
		c := &child{name: comp.name, cmd: cmd}
		children = append(children, c)

		if !comp.daemon {
			critical = append(critical, c)
		}
	}

	if len(critical) == 0 {
		log.Printf("no critical components running, exiting")
		stopAll(children)
		os.Exit(1)
	}

	// Wait for any critical component to exit, or a signal
	exits := make(chan *child, len(critical))
	for _, c := range critical {
		c := c
		go func() {
			c.cmd.Wait()
			exits <- c
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("received %v", sig)
	case c := <-exits:
		log.Printf("%s exited", c.name)
	}

	signal.Stop(sigCh)
	stopAll(children)
}

func resolveApp(name string) string {
	candidates := []string{
		filepath.Join("/apps", name, "exec"),
		filepath.Join(fs.AvyosPath, "apps", name, "exec"),
		filepath.Join("apps", name, "exec"),
		filepath.Join("/apps", name),
		filepath.Join(fs.AvyosPath, "apps", name),
		filepath.Join("apps", name),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return fmt.Sprintf("/avyos/apps/%s/exec", name)
}

func stopAll(children []*child) {
	// Stop in reverse order
	for i := len(children) - 1; i >= 0; i-- {
		stopChild(children[i])
	}
}

func stopChild(c *child) {
	if c.cmd == nil || c.cmd.Process == nil {
		return
	}
	_ = c.cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		c.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		log.Printf("forcing stop for %s pid=%d", c.name, c.cmd.Process.Pid)
		_ = c.cmd.Process.Signal(syscall.SIGKILL)
	}
}
