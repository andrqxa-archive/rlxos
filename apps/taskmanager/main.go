package main

import (
	_ "embed"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"avyos.dev/pkg/appcatalog"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/graphics"
	gapp "avyos.dev/pkg/graphics/app"
	"avyos.dev/pkg/graphics/ui"
)

//go:embed ui/taskmanager.ui
var taskManagerUI string

type procInfo struct {
	pid        int
	ppid       int
	name       string
	command    string
	state      string
	rssKB      uint64
	cpuTicks   uint64
	cpuPercent float64

	isApp   bool
	appID   string
	appName string
	appIcon string
}

const (
	taskIconColWidth = 24
	taskPIDColWidth  = 52
	taskCPUColWidth  = 56
	taskMemColWidth  = 72
	taskPIDColMin    = 34
	taskCPUColMin    = 40
	taskMemColMin    = 52
	taskNameColMin   = 96
)

func (p procInfo) displayName() string {
	if p.isApp && strings.TrimSpace(p.appName) != "" {
		return p.appName
	}
	if strings.TrimSpace(p.name) != "" {
		return p.name
	}
	return "unknown"
}

type TaskManagerApp struct {
	ui.App
	mu sync.Mutex

	home string
	apps []appcatalog.Entry

	stopCh   chan struct{}
	stopOnce sync.Once

	prevTotalTicks uint64
	prevIdleTicks  uint64
	prevProcTicks  map[int]uint64

	appProcs   []procInfo
	otherProcs []procInfo
	procByPID  map[int]procInfo

	selectedPID int
	contextPID  int

	taskPIDWidth  int
	taskCPUWidth  int
	taskMemWidth  int
	taskNameWidth int
}

func (a *TaskManagerApp) e(id string) *ui.Element { return a.FindElement(id) }

func (a *TaskManagerApp) setStatus(text string) {
	if strings.TrimSpace(text) == "" {
		text = " "
	}
	if status := a.e("Status"); status != nil {
		status.SetAttribute("text", text)
	}
}

func (a *TaskManagerApp) Refresh() {
	a.mu.Lock()
	defer a.mu.Unlock()

	root := fs.Resolve("process:")
	procs := listProcesses(root)
	totalTicks, idleTicks := readCPUTicks(root)
	memUsed, memTotal, memPercent := readMemoryUsage(root)
	stUsed, stTotal, stPercent := readStorageUsage("/")

	cpuPercent := 0.0
	totalDelta := uint64(0)
	if a.prevTotalTicks > 0 && totalTicks >= a.prevTotalTicks {
		totalDelta = totalTicks - a.prevTotalTicks
	}
	if totalDelta > 0 && idleTicks >= a.prevIdleTicks {
		idleDelta := idleTicks - a.prevIdleTicks
		if idleDelta > totalDelta {
			idleDelta = totalDelta
		}
		cpuPercent = float64(totalDelta-idleDelta) * 100.0 / float64(totalDelta)
	}

	nextProcTicks := make(map[int]uint64, len(procs))
	for i := range procs {
		p := &procs[i]
		nextProcTicks[p.pid] = p.cpuTicks
		if totalDelta == 0 {
			continue
		}
		prev := a.prevProcTicks[p.pid]
		if prev == 0 || p.cpuTicks < prev {
			continue
		}
		delta := p.cpuTicks - prev
		p.cpuPercent = float64(delta) * 100.0 / float64(totalDelta)
	}

	a.prevTotalTicks = totalTicks
	a.prevIdleTicks = idleTicks
	a.prevProcTicks = nextProcTicks

	a.categorizeProcesses(procs)
	a.updateMetricBars(cpuPercent, memPercent, stPercent, memUsed, memTotal, stUsed, stTotal)
	a.syncSelection()
	a.renderTaskTable()
	a.syncContextMenu()

	a.setStatus(fmt.Sprintf("%d app(s), %d process(es)  CPU %.1f%%  MEM %.1f%%  STO %.1f%%",
		len(a.appProcs), len(a.otherProcs), cpuPercent, memPercent, stPercent))
}

func (a *TaskManagerApp) updateMetricBars(cpuPercent, memPercent, stPercent float64, memUsed, memTotal, stUsed, stTotal uint64) {
	if v := a.e("CPUValue"); v != nil {
		v.SetAttribute("text", fmt.Sprintf("%.1f%%", cpuPercent))
	}
	if b := a.e("CPUBar"); b != nil {
		b.SetAttribute("value", clamp01(cpuPercent/100))
	}

	if v := a.e("MemoryValue"); v != nil {
		v.SetAttribute("text", fmt.Sprintf("%s/%s", formatBytes(memUsed), formatBytes(memTotal)))
	}
	if b := a.e("MemoryBar"); b != nil {
		b.SetAttribute("value", clamp01(memPercent/100))
	}

	if v := a.e("StorageValue"); v != nil {
		v.SetAttribute("text", fmt.Sprintf("%s/%s", formatBytes(stUsed), formatBytes(stTotal)))
	}
	if b := a.e("StorageBar"); b != nil {
		b.SetAttribute("value", clamp01(stPercent/100))
	}
}

func (a *TaskManagerApp) categorizeProcesses(all []procInfo) {
	apps := make([]procInfo, 0, len(all))
	other := make([]procInfo, 0, len(all))
	byPID := make(map[int]procInfo, len(all))

	for i := range all {
		p := all[i]
		if entry, ok := a.matchApp(p); ok {
			p.isApp = true
			p.appID = entry.ID
			p.appName = entry.Name
			p.appIcon = entry.IconPath
			if strings.TrimSpace(p.appIcon) == "" {
				p.appIcon = graphics.ResolveIconPath("help", 64)
			}
			apps = append(apps, p)
		} else {
			other = append(other, p)
		}
		byPID[p.pid] = p
	}

	sort.SliceStable(apps, func(i, j int) bool {
		li := strings.ToLower(apps[i].displayName())
		lj := strings.ToLower(apps[j].displayName())
		if li == lj {
			return apps[i].pid < apps[j].pid
		}
		return li < lj
	})

	sort.SliceStable(other, func(i, j int) bool {
		if math.Abs(other[i].cpuPercent-other[j].cpuPercent) > 0.01 {
			return other[i].cpuPercent > other[j].cpuPercent
		}
		return other[i].pid < other[j].pid
	})

	a.appProcs = apps
	a.otherProcs = other
	a.procByPID = byPID
}

func (a *TaskManagerApp) renderTaskTable() {
	table := a.e("TaskTable")
	if table == nil {
		return
	}
	a.updateTaskColumnLayout(table.Bounds().W)
	table.ClearChildren()

	if len(a.appProcs) == 0 && len(a.otherProcs) == 0 {
		empty := ui.NewElement("StatusLabel")
		empty.SetAttribute("text", "No tasks available")
		empty.SetAttribute("expand", false)
		table.AddChild(empty)
		return
	}

	if len(a.appProcs) > 0 {
		table.AddChild(sectionLabel("Applications"))
		table.AddChild(dividerLine())
		for i := range a.appProcs {
			p := a.appProcs[i]
			table.AddChild(a.buildTaskRow(p))
			if i < len(a.appProcs)-1 {
				table.AddChild(dividerLine())
			}
		}
	}

	if len(a.otherProcs) > 0 {
		if len(a.appProcs) > 0 {
			table.AddChild(gapLine(6))
		}
		table.AddChild(sectionLabel("Processes"))
		table.AddChild(dividerLine())
		for i := range a.otherProcs {
			p := a.otherProcs[i]
			table.AddChild(a.buildTaskRow(p))
			if i < len(a.otherProcs)-1 {
				table.AddChild(dividerLine())
			}
		}
	}
}

func (a *TaskManagerApp) updateTaskColumnLayout(totalWidth int) {
	pid := taskPIDColMin
	cpu := taskCPUColMin
	mem := taskMemColMin

	// Row has 5 columns with spacing 4 and horizontal padding 4+4.
	rowFixedBase := taskIconColWidth + (4 * 4) + 8
	maxFixedForMetrics := totalWidth - taskNameColMin - rowFixedBase
	if maxFixedForMetrics < pid+cpu+mem {
		maxFixedForMetrics = pid + cpu + mem
	}

	add := maxFixedForMetrics - (pid + cpu + mem)
	if add < 0 {
		add = 0
	}

	memHeadroom := taskMemColWidth - mem
	if memHeadroom > 0 && add > 0 {
		step := memHeadroom
		if step > add {
			step = add
		}
		mem += step
		add -= step
	}

	cpuHeadroom := taskCPUColWidth - cpu
	if cpuHeadroom > 0 && add > 0 {
		step := cpuHeadroom
		if step > add {
			step = add
		}
		cpu += step
		add -= step
	}

	pidHeadroom := taskPIDColWidth - pid
	if pidHeadroom > 0 && add > 0 {
		step := pidHeadroom
		if step > add {
			step = add
		}
		pid += step
	}

	a.taskPIDWidth = pid
	a.taskCPUWidth = cpu
	a.taskMemWidth = mem
	nameW := totalWidth - rowFixedBase - pid - cpu - mem
	if nameW < 1 {
		nameW = 1
	}
	a.taskNameWidth = nameW

	if h := a.e("TaskPIDHeader"); h != nil {
		h.SetAttribute("minWidth", pid)
		h.SetAttribute("maxWidth", pid)
	}
	if h := a.e("TaskCPUHeader"); h != nil {
		h.SetAttribute("minWidth", cpu)
		h.SetAttribute("maxWidth", cpu)
	}
	if h := a.e("TaskMemoryHeader"); h != nil {
		h.SetAttribute("minWidth", mem)
		h.SetAttribute("maxWidth", mem)
	}
}

func sectionLabel(text string) *ui.Element {
	el := ui.NewElement("StatusLabel")
	el.SetAttribute("text", text)
	el.SetAttribute("textColor", "theme.color.text.muted")
	el.SetAttribute("expand", false)
	el.SetAttribute("minHeight", 20)
	return el
}

func dividerLine() *ui.Element {
	el := ui.NewElement("Label")
	el.SetAttribute("text", "")
	el.SetAttribute("expand", false)
	el.SetAttribute("minHeight", 1)
	el.SetAttribute("background", "theme.color.stroke.divider")
	return el
}

func gapLine(h int) *ui.Element {
	if h < 1 {
		h = 1
	}
	el := ui.NewElement("Label")
	el.SetAttribute("text", "")
	el.SetAttribute("expand", false)
	el.SetAttribute("minHeight", h)
	el.SetAttribute("background", "transparent")
	return el
}

func (a *TaskManagerApp) buildTaskRow(p procInfo) *ui.Element {
	row := ui.NewElement("Button")
	row.SetAttribute("expand", false)
	row.SetAttribute("minHeight", 30)
	row.SetAttribute("maxHeight", 30)
	row.SetAttribute("padding", "2 4")
	row.SetAttribute("borderRadius", 6)
	row.SetAttribute("focusRing", false)
	row.SetAttribute("shadow", false)
	row.SetAttribute("textAlign", "left")
	row.SetAttribute("overflow", "hidden")

	selected := a.selectedPID == p.pid
	if selected {
		row.SetAttribute("background", "theme.color.accent.subtle")
		row.SetAttribute("borderColor", "theme.color.accent")
		row.SetAttribute("focusedBorderColor", "theme.color.accent")
	} else {
		row.SetAttribute("background", "transparent")
		row.SetAttribute("gradientTop", "transparent")
		row.SetAttribute("gradientBottom", "transparent")
		row.SetAttribute("hoverBackground", "theme.color.accent.subtle")
		row.SetAttribute("pressedBackground", "theme.color.control.pressed")
		row.SetAttribute("borderColor", "transparent")
		row.SetAttribute("focusedBorderColor", "transparent")
	}

	pid := p.pid
	row.BindSignal("clicked", func() { a.selectTask(pid) })
	row.BindSignal("secondaryClicked", func() { a.openTaskContext(pid) })

	content := ui.NewElement("HBox")
	content.SetAttribute("direction", "row")
	content.SetAttribute("spacing", 4)
	content.SetAttribute("expand", true)
	content.SetAttribute("overflow", "hidden")

	iconCol := ui.NewElement("HBox")
	iconCol.SetAttribute("direction", "row")
	iconCol.SetAttribute("expand", false)
	iconCol.SetAttribute("minWidth", taskIconColWidth)
	iconCol.SetAttribute("maxWidth", taskIconColWidth)
	iconCol.SetAttribute("alignment", "center")
	iconPath := p.appIcon
	if strings.TrimSpace(iconPath) == "" {
		iconPath = graphics.ResolveIconPath("help", 64)
	}
	icon := ui.NewElement("Image")
	icon.SetAttribute("src", iconPath)
	icon.SetAttribute("minWidth", 16)
	icon.SetAttribute("minHeight", 16)
	icon.SetAttribute("maxWidth", 16)
	icon.SetAttribute("maxHeight", 16)
	icon.SetAttribute("expand", false)
	icon.SetAttribute("srcOpaque", false)
	iconCol.AddChild(icon)

	nameCol := ui.NewElement("Label")
	nameCol.SetAttribute("expand", true)
	nameW := a.taskNameWidth
	if nameW <= 0 {
		nameW = taskNameColMin
	}
	nameCol.SetAttribute("minWidth", 1)
	nameCol.SetAttribute("maxWidth", nameW)
	nameCol.SetAttribute("text", p.displayName())
	nameCol.SetAttribute("textAlign", "left")
	nameCol.SetAttribute("clipText", true)

	pidCol := ui.NewElement("Label")
	pidCol.SetAttribute("text", fmt.Sprintf("%d", p.pid))
	pidCol.SetAttribute("expand", false)
	pidW := a.taskPIDWidth
	if pidW <= 0 {
		pidW = taskPIDColWidth
	}
	pidCol.SetAttribute("minWidth", pidW)
	pidCol.SetAttribute("maxWidth", pidW)
	pidCol.SetAttribute("textAlign", "right")
	pidCol.SetAttribute("fontFamily", "jetbrains-mono")
	pidCol.SetAttribute("fontSize", 13)
	pidCol.SetAttribute("clipText", true)

	cpuCol := ui.NewElement("Label")
	cpuCol.SetAttribute("text", fmt.Sprintf("%.1f%%", p.cpuPercent))
	cpuCol.SetAttribute("expand", false)
	cpuW := a.taskCPUWidth
	if cpuW <= 0 {
		cpuW = taskCPUColWidth
	}
	cpuCol.SetAttribute("minWidth", cpuW)
	cpuCol.SetAttribute("maxWidth", cpuW)
	cpuCol.SetAttribute("textAlign", "right")
	cpuCol.SetAttribute("fontFamily", "jetbrains-mono")
	cpuCol.SetAttribute("fontSize", 13)
	cpuCol.SetAttribute("clipText", true)

	memCol := ui.NewElement("Label")
	memCol.SetAttribute("text", formatBytes(p.rssKB*1024))
	memCol.SetAttribute("expand", false)
	memW := a.taskMemWidth
	if memW <= 0 {
		memW = taskMemColWidth
	}
	memCol.SetAttribute("minWidth", memW)
	memCol.SetAttribute("maxWidth", memW)
	memCol.SetAttribute("textAlign", "right")
	memCol.SetAttribute("fontFamily", "jetbrains-mono")
	memCol.SetAttribute("fontSize", 13)
	memCol.SetAttribute("clipText", true)

	content.AddChild(iconCol)
	content.AddChild(nameCol)
	content.AddChild(pidCol)
	content.AddChild(cpuCol)
	content.AddChild(memCol)
	row.AddChild(content)
	return row
}

func (a *TaskManagerApp) selectTask(pid int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.selectedPID = pid
	a.contextPID = pid
	a.showContextMenu()
	a.renderTaskTable()
	a.Redraw()
}

func (a *TaskManagerApp) openTaskContext(pid int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.selectedPID = pid
	a.contextPID = pid
	a.showContextMenu()
	a.renderTaskTable()
	a.Redraw()
}

func (a *TaskManagerApp) showContextMenu() {
	p, ok := a.procByPID[a.contextPID]
	if !ok {
		a.hideContextMenu()
		return
	}
	if menu := a.e("ContextMenu"); menu != nil {
		menu.SetVisible(true)
	}
	if info := a.e("ContextInfo"); info != nil {
		info.SetAttribute("text", fmt.Sprintf("Task: %s (PID %d)", p.displayName(), p.pid))
	}
}

func (a *TaskManagerApp) hideContextMenu() {
	a.contextPID = 0
	if menu := a.e("ContextMenu"); menu != nil {
		menu.SetVisible(false)
	}
	if info := a.e("ContextInfo"); info != nil {
		info.SetAttribute("text", "")
	}
}

func (a *TaskManagerApp) syncSelection() {
	if a.selectedPID > 0 {
		if _, ok := a.procByPID[a.selectedPID]; !ok {
			a.selectedPID = 0
		}
	}
}

func (a *TaskManagerApp) syncContextMenu() {
	if a.contextPID <= 0 {
		return
	}
	if _, ok := a.procByPID[a.contextPID]; !ok {
		a.hideContextMenu()
		return
	}
	a.showContextMenu()
}

func (a *TaskManagerApp) CloseContextMenu() {
	a.hideContextMenu()
	a.Redraw()
}

func (a *TaskManagerApp) ContextTerminate() {
	a.signalCurrentContext(syscall.SIGTERM, "SIGTERM")
}

func (a *TaskManagerApp) ContextKill() {
	a.signalCurrentContext(syscall.SIGKILL, "SIGKILL")
}

func (a *TaskManagerApp) signalCurrentContext(sig syscall.Signal, sigName string) {
	pid := a.contextPID
	if pid <= 0 {
		pid = a.selectedPID
	}
	if pid <= 0 {
		a.setStatus("Select a task first")
		return
	}
	if pid == os.Getpid() {
		a.setStatus("Refusing to terminate Task Manager process")
		return
	}
	if err := syscall.Kill(pid, sig); err != nil {
		a.setStatus(fmt.Sprintf("%s failed: %v", sigName, err))
		return
	}
	a.hideContextMenu()
	a.setStatus(fmt.Sprintf("Sent %s to PID %d", sigName, pid))
	a.Refresh()
	a.Redraw()
}

func (a *TaskManagerApp) shutdown() {
	a.stopOnce.Do(func() { close(a.stopCh) })
}

func (a *TaskManagerApp) matchApp(p procInfo) (appcatalog.Entry, bool) {
	name := strings.ToLower(strings.TrimSpace(p.name))
	command := strings.ToLower(strings.TrimSpace(p.command))

	for _, entry := range a.apps {
		id := strings.ToLower(strings.TrimSpace(entry.ID))
		dir := strings.ToLower(strings.TrimSpace(entry.DirName))
		if name != "" && (name == id || name == dir) {
			return entry, true
		}
	}

	if command == "" {
		return appcatalog.Entry{}, false
	}

	for _, entry := range a.apps {
		execPath := strings.ToLower(filepath.Clean(strings.TrimSpace(entry.ExecPath)))
		if execPath != "" && strings.Contains(command, execPath) {
			return entry, true
		}
		dir := strings.ToLower(strings.TrimSpace(entry.DirName))
		if dir != "" {
			token := "/" + dir + "/"
			if strings.Contains(command, token) || strings.Contains(command, " "+dir+" ") {
				return entry, true
			}
		}
	}
	return appcatalog.Entry{}, false
}

func main() {
	home, _ := os.UserHomeDir()
	app := &TaskManagerApp{
		home:          home,
		stopCh:        make(chan struct{}),
		prevProcTicks: make(map[int]uint64),
		procByPID:     make(map[int]procInfo),
	}

	app.SetOptions(gapp.Options{Title: "Task Manager"})
	if err := app.LoadString(taskManagerUI, app); err != nil {
		log.Fatalf("Failed to load UI: %v", err)
	}

	app.apps = appcatalog.Discover(appcatalog.DiscoverOptions{
		Home:          app.home,
		IncludeHidden: false,
	})

	app.Configure(func(core *gapp.App) {
		core.OnQuit = app.shutdown
		go func() {
			ticker := time.NewTicker(1200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-app.stopCh:
					return
				case <-ticker.C:
					app.Refresh()
				}
			}
		}()
	})

	app.AddFocusable(
		app.e("ContextTerminateBtn"),
		app.e("ContextKillBtn"),
	)

	app.Refresh()
	if err := app.Run(); err != nil {
		log.Fatalf("Task manager error: %v", err)
	}
}

func listProcesses(root string) []procInfo {
	des, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	out := make([]procInfo, 0, len(des))
	for _, de := range des {
		if !de.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(de.Name())
		if err != nil || pid <= 0 {
			continue
		}
		p, err := readProcess(root, pid)
		if err != nil {
			continue
		}
		if isKernelProcess(p) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].pid < out[j].pid })
	return out
}

func isKernelProcess(p procInfo) bool {
	name := strings.TrimSpace(p.name)
	cmd := strings.TrimSpace(p.command)
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]") {
		return true
	}
	// Linux kernel threads are parented by kthreadd (PID 2) and have no user cmdline.
	if p.ppid == 2 && (cmd == "" || cmd == name || strings.HasPrefix(cmd, "[")) {
		return true
	}
	return false
}

func readProcess(root string, pid int) (procInfo, error) {
	statPath := filepath.Join(root, strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(statPath)
	if err != nil {
		return procInfo{}, err
	}

	name, state, ppid, cpuTicks, rssPages, err := parseProcessStat(string(data))
	if err != nil {
		return procInfo{}, err
	}
	if name == "" {
		name = readProcessName(root, pid)
	}
	cmdline := readProcessCmdline(root, pid)
	if cmdline == "" {
		cmdline = name
	}

	pageSize := uint64(os.Getpagesize())
	rssKB := uint64(0)
	if rssPages > 0 {
		rssKB = uint64(rssPages) * pageSize / 1024
	}

	return procInfo{
		pid:      pid,
		ppid:     ppid,
		name:     strings.TrimSpace(name),
		command:  strings.TrimSpace(cmdline),
		state:    state,
		rssKB:    rssKB,
		cpuTicks: cpuTicks,
	}, nil
}

func parseProcessStat(s string) (name, state string, ppid int, procTicks uint64, rssPages int64, err error) {
	l := strings.IndexByte(s, '(')
	r := strings.LastIndexByte(s, ')')
	if l < 0 || r <= l || r+2 > len(s) {
		err = fmt.Errorf("invalid stat format")
		return
	}
	name = s[l+1 : r]
	fields := strings.Fields(s[r+2:])
	if len(fields) < 22 {
		err = fmt.Errorf("invalid stat field count")
		return
	}
	state = fields[0]
	ppid, _ = strconv.Atoi(fields[1])
	utime, _ := strconv.ParseUint(fields[11], 10, 64)
	stime, _ := strconv.ParseUint(fields[12], 10, 64)
	procTicks = utime + stime
	rssPages, _ = strconv.ParseInt(fields[21], 10, 64)
	return
}

func readProcessName(root string, pid int) string {
	path := filepath.Join(root, strconv.Itoa(pid), "comm")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readProcessCmdline(root string, pid int) string {
	path := filepath.Join(root, strconv.Itoa(pid), "cmdline")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	cmd := strings.ReplaceAll(string(data), "\x00", " ")
	return strings.TrimSpace(cmd)
}

func readCPUTicks(root string) (total, idle uint64) {
	paths := []string{
		filepath.Join(root, "stat"),
		"/proc/stat",
	}
	seen := map[string]struct{}{}
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) == 0 {
			continue
		}
		line := strings.TrimSpace(lines[0])
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		var sum uint64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			sum += v
			if i == 4 || i == 5 {
				idle += v
			}
		}
		return sum, idle
	}
	return 0, 0
}

func readMemoryUsage(root string) (usedBytes, totalBytes uint64, percent float64) {
	paths := []string{
		filepath.Join(root, "meminfo"),
		"/proc/meminfo",
	}
	seen := map[string]struct{}{}
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var memTotal uint64
		var memAvailable uint64
		var memFree uint64
		var cached uint64
		var buffers uint64
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			key := strings.TrimSuffix(fields[0], ":")
			val, _ := strconv.ParseUint(fields[1], 10, 64)
			switch key {
			case "MemTotal":
				memTotal = val * 1024
			case "MemAvailable":
				memAvailable = val * 1024
			case "MemFree":
				memFree = val * 1024
			case "Cached":
				cached = val * 1024
			case "Buffers":
				buffers = val * 1024
			}
		}
		if memTotal == 0 {
			continue
		}
		if memAvailable == 0 {
			memAvailable = memFree + cached + buffers
		}
		if memAvailable > memTotal {
			memAvailable = memTotal
		}
		used := memTotal - memAvailable
		pct := 0.0
		if memTotal > 0 {
			pct = float64(used) * 100.0 / float64(memTotal)
		}
		return used, memTotal, pct
	}
	return 0, 0, 0
}

func readStorageUsage(path string) (usedBytes, totalBytes uint64, percent float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	avail := stat.Bavail * uint64(stat.Bsize)
	if avail > total {
		avail = total
	}
	used := total - avail
	pct := 0.0
	if total > 0 {
		pct = float64(used) * 100.0 / float64(total)
	}
	return used, total, pct
}

func formatBytes(v uint64) string {
	switch {
	case v >= 1<<30:
		return fmt.Sprintf("%.1fGB", float64(v)/float64(1<<30))
	case v >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(v)/float64(1<<20))
	case v >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(v)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", v)
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	if n <= 3 {
		return string(r[:n])
	}
	return string(r[:n-3]) + "..."
}
