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
	"strings"
	"syscall"
	"time"
	"unsafe"

	"avyos.dev/pkg/avyos"
	"avyos.dev/pkg/fs"
)

var ASCII_LOGO string

type InfoRow struct {
	Label string
	Value string
}

func main() {
	info := collectInfo()
	logo := asciiLogo()

	printLayout(logo, info)
}

func collectInfo() []InfoRow {
	return []InfoRow{
		{"OS", "Avyos"},
		{"Kernel", getKernel()},
		{"Hostname", getHostname()},
		{"Uptime", getUptime()},
		{"CPU", getCPU()},
		{"Memory", getMemory()},
	}
}

func getKernel() string {
	var u syscall.Utsname
	if _, _, err := syscall.Syscall(syscall.SYS_UNAME, uintptr(unsafe.Pointer(&u)), 0, 0); err != 0 {
		return "unknown"
	}
	var b [65]byte
	for i, ch := range u.Release {
		b[i] = byte(ch)
	}
	return string(b[:])
}

func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}

func getUptime() string {
	data, err := os.ReadFile(fs.Resolve("process", "uptime"))
	if err != nil {
		return "unknown"
	}
	secs := int(parseFloat(strings.Fields(string(data))[0]))
	return formatDuration(time.Duration(secs) * time.Second)
}

func getCPU() string {
	file, err := os.Open(fs.Resolve("process", "cpuinfo"))
	if err != nil {
		return "unknown"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			return strings.TrimSpace(parts[1])
		}
	}
	return "unknown"
}

func getMemory() string {
	memTotal := readKeyValue(fs.Resolve("process", "meminfo"), "MemTotal")
	memFree := readKeyValue(fs.Resolve("process", "meminfo"), "MemAvailable")

	if memTotal == "" || memFree == "" {
		return "unknown"
	}

	return fmt.Sprintf("%s / %s", memFree, memTotal)
}

func readKeyValue(path, key string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, key) {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(parts[1], "\"")
			}
			parts = strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

func maxLineWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		if len(l) > w {
			w = len(l)
		}
	}
	return w
}

func printLayout(logo []string, info []InfoRow) {
	logoWidth := maxLineWidth(logo)
	lines := max(len(logo), len(info))

	for i := 0; i < lines; i++ {
		left := ""
		right := ""

		if i < len(logo) {
			left = logo[i]
		}
		if i < len(info) {
			right = fmt.Sprintf("%-12s : %s", info[i].Label, info[i].Value)
		}

		fmt.Printf("%-*s  %s\n", logoWidth, left, right)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func asciiLogo() []string {
	return strings.Split(avyos.LogoAscii, "\n")
}
