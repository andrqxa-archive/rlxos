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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"avyos.dev/pkg/format"
	"avyos.dev/pkg/fs"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "system - System information and configuration tools")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  system <subcommand> [options] [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  info      Show general system information")
		fmt.Fprintln(os.Stderr, "  memory    Show memory and swap usage")
		fmt.Fprintln(os.Stderr, "  disk      Show disk usage by mountpoint")
		fmt.Fprintln(os.Stderr, "  uptime    Show uptime and boot time")
		fmt.Fprintln(os.Stderr, "  hostname  Get or set hostname")
		fmt.Fprintln(os.Stderr, "  env       Get or set environment variables")
		fmt.Fprintln(os.Stderr, "  date      Show formatted date/time")
		fmt.Fprintln(os.Stderr, "  config    Read/write kernel parameters (sysctl-style)")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/command error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

func main() {
	flag.Parse()
	args := flag.Args()

	commands := map[string]func(args []string) error{
		"info":     cmdInfo,
		"memory":   cmdMemory,
		"disk":     cmdDisk,
		"uptime":   cmdUptime,
		"hostname": cmdHostname,
		"env":      cmdEnv,
		"date":     cmdDate,
		"config":   cmdConfig,
	}

	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	cmd, ok := commands[args[0]]
	if !ok {
		format.Error("unknown subcommand: %s", args[0])
		os.Exit(1)
	}

	if err := cmd(args[1:]); err != nil {
		format.Error("%s", err)
		os.Exit(1)
	}
}

func cmdInfo(args []string) error {
	// Hostname
	hostname, _ := os.Hostname()

	// Kernel version
	var uname syscall.Utsname
	syscall.Uname(&uname)
	kernelRelease := int8ToString(uname.Release[:])
	kernelVersion := int8ToString(uname.Version[:])
	machine := int8ToString(uname.Machine[:])

	// OS info
	osRelease := readOSRelease()

	// CPU info
	cpuInfo := readCPUInfo()

	// Memory
	memTotal, memFree, _ := getMemoryInfo()

	// Uptime
	uptime := getUptime()

	fmt.Printf("Hostname:     %s\n", hostname)
	if name, ok := osRelease["PRETTY_NAME"]; ok {
		fmt.Printf("OS:           %s\n", name)
	}
	fmt.Printf("Kernel:       %s %s\n", kernelRelease, machine)
	fmt.Printf("Kernel Build: %s\n", kernelVersion)
	if model, ok := cpuInfo["model name"]; ok {
		fmt.Printf("CPU:          %s\n", model)
	}
	fmt.Printf("CPU Cores:    %d\n", runtime.NumCPU())
	fmt.Printf("Memory:       %s / %s\n", format.Size(memFree*1024), format.Size(memTotal*1024))
	fmt.Printf("Uptime:       %s\n", formatDuration(uptime))
	fmt.Printf("Go Version:   %s\n", runtime.Version())

	return nil
}

func cmdMemory(args []string) error {
	memTotal, memFree, memAvailable := getMemoryInfo()
	memUsed := memTotal - memFree
	swapTotal, swapFree := getSwapInfo()
	swapUsed := swapTotal - swapFree

	buffers, cached := getBuffersAndCache()

	table := format.NewTable("Type", "Total", "Used", "Free", "Usage")

	// RAM
	ramUsage := format.Percent(float64(memUsed), float64(memTotal))
	table.AddRow("RAM",
		format.Size(memTotal*1024),
		format.Size(memUsed*1024),
		format.Size(memFree*1024),
		ramUsage)

	// Available (accounting for buffers/cache)
	if memAvailable > 0 {
		actualUsed := memTotal - memAvailable
		table.AddRow("Available",
			format.Size(memTotal*1024),
			format.Size(actualUsed*1024),
			format.Size(memAvailable*1024),
			format.Percent(float64(actualUsed), float64(memTotal)))
	}

	// Swap
	if swapTotal > 0 {
		swapUsage := format.Percent(float64(swapUsed), float64(swapTotal))
		table.AddRow("Swap",
			format.Size(swapTotal*1024),
			format.Size(swapUsed*1024),
			format.Size(swapFree*1024),
			swapUsage)
	}

	table.Print()

	fmt.Printf("\nBuffers: %s, Cached: %s\n",
		format.Size(buffers*1024),
		format.Size(cached*1024))

	return nil
}

func cmdDisk(args []string) error {
	// Parse /proc/mounts for filesystems
	mounts, err := parseMounts()
	if err != nil {
		return err
	}

	// Filter to specific path if provided
	if len(args) > 0 {
		path := args[0]
		for _, m := range mounts {
			if m.mountPoint == path {
				printDiskUsage(m)
				return nil
			}
		}
		return fmt.Errorf("mount point not found: %s", path)
	}

	table := format.NewTable("Filesystem", "Size", "Used", "Free", "Usage", "Mounted on")

	for _, m := range mounts {
		// Skip pseudo filesystems
		if strings.HasPrefix(m.fsType, "devtmpfs") ||
			strings.HasPrefix(m.fsType, "tmpfs") ||
			strings.HasPrefix(m.fsType, "proc") ||
			strings.HasPrefix(m.fsType, "sysfs") ||
			strings.HasPrefix(m.fsType, "cgroup") {
			continue
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(m.mountPoint, &stat); err != nil {
			continue
		}

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bfree * uint64(stat.Bsize)
		used := total - free

		if total == 0 {
			continue
		}

		usage := format.Percent(float64(used), float64(total))
		table.AddRow(
			m.device,
			format.Size(int64(total)),
			format.Size(int64(used)),
			format.Size(int64(free)),
			usage,
			m.mountPoint,
		)
	}

	table.Print()
	return nil
}

func cmdUptime(args []string) error {
	uptime := getUptime()
	bootTime := time.Now().Add(-uptime)

	fmt.Printf("Uptime:    %s\n", formatDuration(uptime))
	fmt.Printf("Boot time: %s\n", bootTime.Format("2006-01-02 15:04:05"))

	// Load averages
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			fmt.Printf("Load avg:  %s %s %s\n", parts[0], parts[1], parts[2])
		}
	}

	return nil
}

func cmdHostname(args []string) error {
	if len(args) > 0 {
		// Set hostname
		newName := args[0]
		if err := syscall.Sethostname([]byte(newName)); err != nil {
			return fmt.Errorf("failed to set hostname: %w", err)
		}
		format.Success("Hostname set to: %s", newName)
		return nil
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	fmt.Println(hostname)
	return nil
}

func cmdEnv(args []string) error {
	if len(args) == 0 {
		// List all environment variables
		for _, env := range os.Environ() {
			fmt.Println(env)
		}
		return nil
	}

	arg := args[0]
	if strings.Contains(arg, "=") {
		// Set variable
		parts := strings.SplitN(arg, "=", 2)
		if err := os.Setenv(parts[0], parts[1]); err != nil {
			return err
		}
		format.Success("Set %s=%s", parts[0], parts[1])
	} else {
		// Get variable
		value := os.Getenv(arg)
		if value == "" {
			fmt.Printf("%s is not set\n", arg)
		} else {
			fmt.Printf("%s=%s\n", arg, value)
		}
	}

	return nil
}

func cmdDate(args []string) error {
	fs := flag.NewFlagSet("date", flag.ContinueOnError)
	formatStr := fs.String("format", "", "Output format")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	now := time.Now()

	if *formatStr == "" {
		fmt.Println(now.Format("Mon Jan 2 15:04:05 MST 2006"))
	} else {
		// Convert common format strings
		f := strings.ReplaceAll(*formatStr, "%Y", "2006")
		f = strings.ReplaceAll(f, "%m", "01")
		f = strings.ReplaceAll(f, "%d", "02")
		f = strings.ReplaceAll(f, "%H", "15")
		f = strings.ReplaceAll(f, "%M", "04")
		f = strings.ReplaceAll(f, "%S", "05")
		f = strings.ReplaceAll(f, "%Z", "MST")
		fmt.Println(now.Format(f))
	}

	return nil
}

func cmdConfig(args []string) error {
	flagSet := flag.NewFlagSet("config", flag.ContinueOnError)
	showAll := flagSet.Bool("all", false, "Show all parameters")
	write := flagSet.Bool("write", false, "Write value (use with key=value)")
	filter := flagSet.String("filter", "", "Filter parameters by pattern (grep-like)")
	flagSet.SetOutput(os.Stderr)
	if err := flagSet.Parse(args); err != nil {
		return err
	}
	args = flagSet.Args()

	sysctlBase := fs.Resolve("process", "sys")

	if *filter != "" {
		return listSysctlParams(sysctlBase, "", *filter)
	}

	if *showAll || len(args) == 0 {
		return listSysctlParams(sysctlBase, "", "")
	}

	arg := args[0]

	// Check if setting a value (key=value)
	if strings.Contains(arg, "=") {
		if !*write {
			return fmt.Errorf("use --write flag to modify kernel parameters")
		}
		parts := strings.SplitN(arg, "=", 2)
		key, value := parts[0], parts[1]
		return setSysctlParam(sysctlBase, key, value)
	}

	// Get a specific parameter
	return getSysctlParam(sysctlBase, arg)
}

func sysctlKeyToPath(base, key string) string {
	// Convert dot notation to path: kernel.hostname -> /proc/sys/kernel/hostname
	return filepath.Join(base, strings.ReplaceAll(key, ".", string(filepath.Separator)))
}

func pathToSysctlKey(base, path string) string {
	// Convert path to dot notation: /proc/sys/kernel/hostname -> kernel.hostname
	rel, _ := filepath.Rel(base, path)
	return strings.ReplaceAll(rel, string(filepath.Separator), ".")
}

func getSysctlParam(base, key string) error {
	path := sysctlKeyToPath(base, key)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("parameter not found: %s", key)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: %s", key)
		}
		return err
	}

	value := strings.TrimSpace(string(data))
	fmt.Printf("%s = %s\n", key, value)
	return nil
}

func setSysctlParam(base, key, value string) error {
	path := sysctlKeyToPath(base, key)

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("parameter not found: %s", key)
	}

	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied (requires root): %s", key)
		}
		return err
	}

	format.Success("%s = %s", key, value)
	return nil
}

func listSysctlParams(base, prefix, filter string) error {
	searchPath := base
	if prefix != "" {
		searchPath = sysctlKeyToPath(base, prefix)
	}

	return filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible files
		}

		if info.IsDir() {
			return nil
		}

		key := pathToSysctlKey(base, path)

		// Apply filter if specified
		if filter != "" && !strings.Contains(key, filter) {
			return nil
		}

		// Try to read the value
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		value := strings.TrimSpace(string(data))
		// Truncate long values
		if len(value) > 60 {
			value = value[:57] + "..."
		}
		// Replace newlines with spaces for display
		value = strings.ReplaceAll(value, "\n", " ")

		fmt.Printf("%s = %s\n", key, value)
		return nil
	})
}

// Helper functions

func int8ToString(arr []int8) string {
	b := make([]byte, 0, len(arr))
	for _, v := range arr {
		if v == 0 {
			break
		}
		b = append(b, byte(v))
	}
	return string(b)
}

func readOSRelease() map[string]string {
	result := make(map[string]string)

	file, err := os.Open("/config/release.conf")
	if err != nil {
		return result
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "="); idx != -1 {
			key := line[:idx]
			value := strings.Trim(line[idx+1:], "\"")
			result[key] = value
		}
	}

	return result
}

func readCPUInfo() map[string]string {
	result := make(map[string]string)

	file, err := os.Open("/cache/kernel/processes/cpuinfo")
	if err != nil {
		return result
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, ":"); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			result[key] = value
		}
	}

	return result
}

func getMemoryInfo() (total, free, available int64) {
	file, err := os.Open("/cache/kernel/processes/meminfo")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, _ := strconv.ParseInt(fields[1], 10, 64)

		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = value
		case strings.HasPrefix(line, "MemFree:"):
			free = value
		case strings.HasPrefix(line, "MemAvailable:"):
			available = value
		}
	}

	return
}

func getSwapInfo() (total, free int64) {
	file, err := os.Open("/cache/kernel/processes/meminfo")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, _ := strconv.ParseInt(fields[1], 10, 64)

		switch {
		case strings.HasPrefix(line, "SwapTotal:"):
			total = value
		case strings.HasPrefix(line, "SwapFree:"):
			free = value
		}
	}

	return
}

func getBuffersAndCache() (buffers, cached int64) {
	file, err := os.Open("/cache/kernel/processes/meminfo")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, _ := strconv.ParseInt(fields[1], 10, 64)

		switch {
		case strings.HasPrefix(line, "Buffers:"):
			buffers = value
		case strings.HasPrefix(line, "Cached:"):
			cached = value
		}
	}

	return
}

func getUptime() time.Duration {
	data, err := os.ReadFile("/cache/kernel/processes/uptime")
	if err != nil {
		return 0
	}

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0
	}

	seconds, _ := strconv.ParseFloat(fields[0], 64)
	return time.Duration(seconds) * time.Second
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours, %d minutes", hours, minutes)
	}
	return fmt.Sprintf("%d minutes", minutes)
}

type mountInfo struct {
	device     string
	mountPoint string
	fsType     string
}

func parseMounts() ([]mountInfo, error) {
	file, err := os.Open("/cache/kernel/processes/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var mounts []mountInfo
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		mounts = append(mounts, mountInfo{
			device:     fields[0],
			mountPoint: fields[1],
			fsType:     fields[2],
		})
	}

	return mounts, scanner.Err()
}

func printDiskUsage(m mountInfo) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(m.mountPoint, &stat); err != nil {
		fmt.Printf("Cannot stat %s\n", m.mountPoint)
		return
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free
	usage := format.Percent(float64(used), float64(total))

	fmt.Printf("Mount point: %s\n", m.mountPoint)
	fmt.Printf("Device:      %s\n", m.device)
	fmt.Printf("Type:        %s\n", m.fsType)
	fmt.Printf("Total:       %s\n", format.Size(int64(total)))
	fmt.Printf("Used:        %s (%s)\n", format.Size(int64(used)), usage)
	fmt.Printf("Free:        %s\n", format.Size(int64(free)))
}
