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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	dbgapi "avyos.dev/api/dbgd"
)

const defaultChunkSize = 32 * 1024

type globalOptions struct {
	host     string
	port     int
	user     string
	password string
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opt, rest, code := parseGlobalFlags(args)
	if code != 0 {
		return code
	}
	if len(rest) == 0 {
		printUsage()
		return 2
	}

	if strings.TrimSpace(opt.password) == "" {
		opt.password = strings.TrimSpace(os.Getenv("DBG_PASSWORD"))
	}
	if strings.TrimSpace(opt.password) == "" {
		password, err := promptPassword(opt.user)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dbg: %v\n", err)
			return 1
		}
		opt.password = password
	}

	client, err := dbgapi.NewHostClient(opt.host, opt.port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dbg: connect failed: %v\n", err)
		return 1
	}
	defer client.Close()

	session, err := client.Authenticate(dbgapi.AuthRequest{
		Username: opt.user,
		Password: opt.password,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "dbg: authentication failed: %v\n", err)
		return 1
	}
	defer client.Logout(dbgapi.SessionToken{Token: session.Token})

	sub := rest[0]
	subArgs := rest[1:]

	switch sub {
	case "cmd":
		return runExecCommand(client, session.Token, false, subArgs)
	case "shell":
		return runExecCommand(client, session.Token, true, subArgs)
	case "pull":
		return runPull(client, session.Token, subArgs)
	case "push":
		return runPush(client, session.Token, subArgs)
	case "whoami":
		fmt.Printf("%s uid=%d gid=%d home=%s shell=%s\n", session.Username, session.UID, session.GID, session.Home, session.Shell)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "dbg: unknown subcommand: %s\n", sub)
		printUsage()
		return 2
	}
}

func parseGlobalFlags(args []string) (globalOptions, []string, int) {
	fs := flag.NewFlagSet("dbg", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opt := globalOptions{}
	fs.StringVar(&opt.host, "host", "127.0.0.1", "dbgd host")
	fs.IntVar(&opt.port, "port", dbgapi.DefaultTCPPort, "dbgd TCP port")
	fs.StringVar(&opt.user, "user", "admin", "identity username")
	fs.StringVar(&opt.password, "password", "", "identity password (or use DBG_PASSWORD)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "dbg: %v\n", err)
		printUsage()
		return globalOptions{}, nil, 2
	}
	if opt.port <= 0 || opt.port > 65535 {
		fmt.Fprintf(os.Stderr, "dbg: invalid port: %d\n", opt.port)
		return globalOptions{}, nil, 2
	}

	return opt, fs.Args(), 0
}

func runExecCommand(client *dbgapi.Client, token string, useShell bool, args []string) int {
	name := "cmd"
	if useShell {
		name = "shell"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cwd := fs.String("cwd", "", "remote working directory")
	timeout := fs.Int("timeout", 30, "command timeout in seconds")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "dbg %s: %v\n", name, err)
		return 2
	}
	if len(fs.Args()) == 0 {
		fmt.Fprintf(os.Stderr, "dbg %s: command is required\n", name)
		return 2
	}
	if *timeout <= 0 {
		*timeout = 30
	}

	req := dbgapi.ExecRequest{
		Token:      token,
		Command:    strings.Join(fs.Args(), " "),
		Cwd:        strings.TrimSpace(*cwd),
		TimeoutSec: int32(*timeout),
	}

	var (
		resp dbgapi.ExecResult
		err  error
	)
	if useShell {
		resp, err = client.RunShell(req)
	} else {
		resp, err = client.RunCommand(req)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "dbg %s: %v\n", name, err)
		return 1
	}

	if len(resp.Stdout) > 0 {
		_, _ = os.Stdout.Write(resp.Stdout)
	}
	if len(resp.Stderr) > 0 {
		_, _ = os.Stderr.Write(resp.Stderr)
	}
	if resp.StdoutTruncated || resp.StderrTruncated {
		fmt.Fprintln(os.Stderr, "dbg: output truncated by daemon capture limit")
	}

	if resp.ExitCode == 0 {
		return 0
	}
	if resp.ExitCode < 0 {
		return 1
	}
	if resp.ExitCode > 255 {
		return 255
	}
	return resp.ExitCode
}

func runPull(client *dbgapi.Client, token string, args []string) int {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	chunk := fs.Int("chunk", defaultChunkSize, "chunk size in bytes (max 32768)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "dbg pull: %v\n", err)
		return 2
	}
	argv := fs.Args()
	if len(argv) < 1 || len(argv) > 2 {
		fmt.Fprintln(os.Stderr, "usage: dbg pull [-chunk=N] <remote> [local]")
		return 2
	}

	if *chunk <= 0 || *chunk > defaultChunkSize {
		*chunk = defaultChunkSize
	}

	remotePath := strings.TrimSpace(argv[0])
	if remotePath == "" {
		fmt.Fprintln(os.Stderr, "dbg pull: remote path is required")
		return 2
	}

	localPath := ""
	if len(argv) == 2 {
		localPath = strings.TrimSpace(argv[1])
	} else {
		localPath = filepath.Base(remotePath)
	}
	if localPath == "" || localPath == "." || localPath == string(filepath.Separator) {
		fmt.Fprintln(os.Stderr, "dbg pull: invalid local path")
		return 2
	}

	out, err := os.Create(localPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dbg pull: %v\n", err)
		return 1
	}
	defer out.Close()

	offset := uint64(0)
	for {
		resp, err := client.ReadFile(dbgapi.ReadFileRequest{
			Token:  token,
			Path:   remotePath,
			Offset: offset,
			Size:   uint32(*chunk),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "dbg pull: %v\n", err)
			return 1
		}
		if len(resp.Data) > 0 {
			if _, err := out.Write(resp.Data); err != nil {
				fmt.Fprintf(os.Stderr, "dbg pull: %v\n", err)
				return 1
			}
			offset += uint64(len(resp.Data))
		}
		if resp.Eof || len(resp.Data) == 0 {
			break
		}
	}

	fmt.Printf("pulled %s -> %s (%d bytes)\n", remotePath, localPath, offset)
	return 0
}

func runPush(client *dbgapi.Client, token string, args []string) int {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	chunk := fs.Int("chunk", defaultChunkSize, "chunk size in bytes (max 32768)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "dbg push: %v\n", err)
		return 2
	}
	argv := fs.Args()
	if len(argv) != 2 {
		fmt.Fprintln(os.Stderr, "usage: dbg push [-chunk=N] <local> <remote>")
		return 2
	}

	if *chunk <= 0 || *chunk > defaultChunkSize {
		*chunk = defaultChunkSize
	}

	localPath := strings.TrimSpace(argv[0])
	remotePath := strings.TrimSpace(argv[1])
	if localPath == "" || remotePath == "" {
		fmt.Fprintln(os.Stderr, "dbg push: local and remote path are required")
		return 2
	}

	in, err := os.Open(localPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dbg push: %v\n", err)
		return 1
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dbg push: %v\n", err)
		return 1
	}

	mode := uint32(info.Mode().Perm())
	buf := make([]byte, *chunk)
	offset := uint64(0)
	truncate := true
	bytesSent := uint64(0)

	for {
		n, readErr := in.Read(buf)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			fmt.Fprintf(os.Stderr, "dbg push: %v\n", readErr)
			return 1
		}

		if n == 0 {
			if offset == 0 {
				if _, err := client.WriteFile(dbgapi.WriteFileRequest{
					Token:    token,
					Path:     remotePath,
					Offset:   0,
					Data:     nil,
					Truncate: true,
					Mode:     mode,
				}); err != nil {
					fmt.Fprintf(os.Stderr, "dbg push: %v\n", err)
					return 1
				}
			}
			break
		}

		chunkData := make([]byte, n)
		copy(chunkData, buf[:n])
		resp, err := client.WriteFile(dbgapi.WriteFileRequest{
			Token:    token,
			Path:     remotePath,
			Offset:   offset,
			Data:     chunkData,
			Truncate: truncate,
			Mode:     mode,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "dbg push: %v\n", err)
			return 1
		}
		if int(resp.Written) != len(chunkData) {
			fmt.Fprintf(os.Stderr, "dbg push: short write (%d/%d)\n", resp.Written, len(chunkData))
			return 1
		}

		truncate = false
		offset += uint64(n)
		bytesSent += uint64(n)

		if errors.Is(readErr, io.EOF) {
			break
		}
	}

	fmt.Printf("pushed %s -> %s (%d bytes)\n", localPath, remotePath, bytesSent)
	return 0
}

func promptPassword(user string) (string, error) {
	if strings.TrimSpace(user) == "" {
		user = "admin"
	}
	fmt.Fprintf(os.Stderr, "Password for %s: ", user)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	fmt.Fprintln(os.Stderr)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", fmt.Errorf("empty password")
	}
	return line, nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "dbg - AvyOS remote debug client")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  dbg [--host=127.0.0.1] [--port=5037] [--user=admin] [--password=...] <subcommand> [args]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Subcommands:")
	fmt.Fprintln(os.Stderr, "  cmd     Run a command without shell parsing")
	fmt.Fprintln(os.Stderr, "  shell   Run a command through shell semantics")
	fmt.Fprintln(os.Stderr, "  pull    Download file from remote host")
	fmt.Fprintln(os.Stderr, "  push    Upload file to remote host")
	fmt.Fprintln(os.Stderr, "  whoami  Show authenticated identity")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  dbg --host=10.0.2.15 --user=admin cmd list /config")
	fmt.Fprintln(os.Stderr, "  dbg shell \"list /config | read pattern services\"")
	fmt.Fprintln(os.Stderr, "  dbg pull /config/init.conf ./init.conf")
	fmt.Fprintln(os.Stderr, "  dbg push ./init.conf /config/init.conf")
}
