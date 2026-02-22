package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	display "avyos.dev/api/display"
	settingsapi "avyos.dev/api/settings"
	"avyos.dev/pkg/appcatalog"
	"avyos.dev/pkg/graphics"
	gapp "avyos.dev/pkg/graphics/app"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
	"avyos.dev/pkg/graphics/ui"
	"avyos.dev/pkg/logger"
)

//go:embed ui/dock.ui
var taskbarUI string

const (
	taskbarHeight    = 54
	taskbarExclusive = taskbarHeight + 2
	defaultDockPos   = "bottom"
	keyDockPosition  = "/dev/rlxos/dock/position"
	legacyDockKey    = "dock.position"
	keyDockPinned    = "/dev/rlxos/dock/pinned"
	legacyPinnedKey  = "dock.pinned"

	taskIconSize = 48

	dockOuterPadX = 1
	dockInnerPadX = 0
	dockItemGap   = 0
	dockMinWidth  = 10
)

var (
	log = logger.New("dock")
	db  *displaybackend.Backend

	flagPosition string
)

func init() {
	flag.StringVar(&flagPosition, "position", "", "Dock position: top or bottom")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "dock - Desktop dock app")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  dock [--position top|bottom]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  (none)")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/app error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

type taskGroup struct {
	Key       string
	AppID     string
	Title     string
	IconPath  string
	WindowIDs []uint32
	Focused   bool
	Pinned    bool
	HasEntry  bool
	Entry     appcatalog.Entry
}

type menuItem struct {
	Label       string
	Destructive bool
	Run         func()
}

type TaskbarApp struct {
	ui.App

	home       string
	catalog    []appcatalog.Entry
	pinnedIDs  []string
	pinsLoaded bool
	windows    []display.WindowInfo
	groups     []taskGroup

	catalogSig string
	pinSig     string
	windowSig  string
	groupSig   string

	taskPopup    *displaybackend.Popup
	taskPopupKey string
	dockWidth    int
}

func (a *TaskbarApp) e(id string) *ui.Element { return a.FindElement(id) }

func (a *TaskbarApp) OpenLaunchpad() {
	a.closeTaskMenu()
	if err := runCommand("appmenu"); err != nil {
		log.Warn("failed to open appmenu: %v", err)
	}
}

func (a *TaskbarApp) closeTaskMenu() {
	p := a.taskPopup
	a.taskPopup = nil
	a.taskPopupKey = ""
	if p != nil {
		db.ClosePopup(p)
	}
}

func (a *TaskbarApp) syncCatalogAndPins(force bool) {
	catalog := appcatalog.Discover(appcatalog.DiscoverOptions{
		Home:          a.home,
		IncludeHidden: true,
	})
	catSig := catalogSignature(catalog)

	if !a.pinsLoaded {
		pins, found := loadDockPinsFromSettings()
		if !found {
			pins = appcatalog.DefaultDockPins()
			if err := saveDockPinsToSettings(pins); err != nil {
				log.Warn("failed to bootstrap dock pins setting: %v", err)
			}
		}
		a.pinnedIDs = filterPins(pins, catalog)
		a.pinsLoaded = true
	}
	pins := filterPins(a.pinnedIDs, catalog)
	pinSig := strings.Join(pins, ",")

	if !force && catSig == a.catalogSig && pinSig == a.pinSig {
		return
	}

	a.catalog = catalog
	a.pinnedIDs = pins
	a.catalogSig = catSig
	a.pinSig = pinSig
	a.rebuildTaskGroups(true)
}

func (a *TaskbarApp) applyPinnedIDs(ids []string) {
	pins := filterPins(ids, a.catalog)
	if sameIDs(a.pinnedIDs, pins) {
		return
	}
	a.pinnedIDs = pins
	a.pinSig = strings.Join(pins, ",")
	a.rebuildTaskGroups(true)
	a.refreshWindows()
}

func (a *TaskbarApp) setPinned(appID string, pinned bool) {
	appID = strings.ToLower(strings.TrimSpace(appID))
	if appID == "" {
		return
	}

	next := append([]string(nil), a.pinnedIDs...)
	index := -1
	for i, id := range next {
		if strings.ToLower(strings.TrimSpace(id)) == appID {
			index = i
			break
		}
	}

	if pinned {
		if index < 0 {
			next = append(next, appID)
		}
	} else if index >= 0 {
		next = append(next[:index], next[index+1:]...)
	}

	next = filterPins(next, a.catalog)
	if sameIDs(next, a.pinnedIDs) {
		return
	}

	if err := saveDockPinsToSettings(next); err != nil {
		log.Warn("failed to persist dock pins setting: %v", err)
	}
	a.applyPinnedIDs(next)
}

func (a *TaskbarApp) refreshWindows() {
	wins, err := db.ListWindows()
	if err != nil {
		return
	}
	sig := windowSignature(wins)
	if sig == a.windowSig {
		return
	}
	a.windowSig = sig
	a.windows = append(a.windows[:0], wins...)
	a.rebuildTaskGroups(false)
}

func (a *TaskbarApp) rebuildTaskGroups(force bool) {
	next := a.collectTaskGroups()
	nextSig := groupsSignature(next)
	if !force && nextSig == a.groupSig {
		return
	}
	a.groups = next
	a.groupSig = nextSig
	a.rebuildTaskList()
}

func (a *TaskbarApp) collectTaskGroups() []taskGroup {
	byKey := make(map[string]*taskGroup)

	for _, win := range a.windows {
		entry, appID, ok := a.matchWindowToEntry(win)
		if !ok {
			continue
		}
		key := "app:" + appID
		g, exists := byKey[key]
		if !exists {
			title := entry.Name
			if strings.TrimSpace(title) == "" {
				title = strings.TrimSpace(entry.ID)
			}
			icon := entry.IconPath
			if strings.TrimSpace(icon) == "" {
				icon = graphics.ResolveIconPath("help", 64)
			}
			g = &taskGroup{
				Key:      key,
				AppID:    appID,
				Title:    title,
				IconPath: icon,
				HasEntry: true,
				Entry:    entry,
			}
			byKey[key] = g
		}
		g.WindowIDs = append(g.WindowIDs, win.ID)
		if win.Focused {
			g.Focused = true
		}
		if title := strings.TrimSpace(win.Title); title != "" {
			g.Title = title
		}
	}

	ordered := make([]taskGroup, 0, len(byKey)+len(a.pinnedIDs))
	seen := make(map[string]struct{}, len(byKey)+len(a.pinnedIDs))

	for _, id := range a.pinnedIDs {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		entry, ok := appcatalog.FindByID(a.catalog, id)
		if !ok {
			continue
		}
		key := "app:" + id
		if g, exists := byKey[key]; exists {
			g.Pinned = true
			ordered = append(ordered, *g)
			seen[key] = struct{}{}
			continue
		}

		pinnedTitle := entry.Name
		if strings.TrimSpace(pinnedTitle) == "" {
			pinnedTitle = strings.TrimSpace(entry.ID)
		}
		icon := entry.IconPath
		if strings.TrimSpace(icon) == "" {
			icon = graphics.ResolveIconPath("help", 64)
		}
		ordered = append(ordered, taskGroup{
			Key:      key,
			AppID:    id,
			Title:    pinnedTitle,
			IconPath: icon,
			Pinned:   true,
			HasEntry: true,
			Entry:    entry,
		})
		seen[key] = struct{}{}
	}

	rest := make([]taskGroup, 0, len(byKey))
	for key, g := range byKey {
		if _, ok := seen[key]; ok {
			continue
		}
		rest = append(rest, *g)
	}
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].Focused != rest[j].Focused {
			return rest[i].Focused
		}
		li := strings.ToLower(strings.TrimSpace(rest[i].Title))
		lj := strings.ToLower(strings.TrimSpace(rest[j].Title))
		if li == lj {
			return rest[i].Key < rest[j].Key
		}
		return li < lj
	})
	ordered = append(ordered, rest...)
	return ordered
}

func (a *TaskbarApp) rebuildTaskList() {
	strip := a.e("TaskList")
	if strip == nil {
		return
	}
	strip.ClearChildren()

	for _, g := range a.groups {
		btn := ui.NewElement("Button")
		btn.SetAttribute("text", "")
		btn.SetAttribute("padding", "0")
		btn.SetAttribute("minWidth", taskIconSize+8)
		btn.SetAttribute("maxWidth", taskIconSize+8)
		btn.SetAttribute("minHeight", taskIconSize+8)
		btn.SetAttribute("maxHeight", taskIconSize+8)
		btn.SetAttribute("borderRadius", 11)
		btn.SetAttribute("focusRing", false)
		btn.SetAttribute("background", "transparent")
		btn.SetAttribute("gradientTop", "transparent")
		btn.SetAttribute("gradientBottom", "transparent")
		btn.SetAttribute("borderColor", "transparent")
		btn.SetAttribute("focusedBorderColor", "transparent")
		btn.SetAttribute("hoverBackground", "transparent")
		btn.SetAttribute("pressedBackground", "transparent")
		btn.SetAttribute("shadow", false)

		col := ui.NewElement("VBox")
		col.SetAttribute("direction", "column")
		col.SetAttribute("alignment", "center")
		col.SetAttribute("spacing", 0)
		col.SetAttribute("expand", true)
		col.SetAttribute("interactive", false)

		dotRow := ui.NewElement("HBox")
		dotRow.SetAttribute("spacing", 2)
		dotRow.SetAttribute("alignment", "center")
		dotRow.SetAttribute("minHeight", 4)
		dotRow.SetAttribute("maxHeight", 4)
		dotRow.SetAttribute("interactive", false)

		dotCount := len(g.WindowIDs)
		if dotCount > 3 {
			dotCount = 3
		}
		dotColor := "theme.color.stroke.hairline"
		if g.Focused {
			dotColor = "theme.color.accent"
		}
		for i := 0; i < dotCount; i++ {
			dot := ui.NewElement("Element")
			dot.SetAttribute("minWidth", 4)
			dot.SetAttribute("maxWidth", 4)
			dot.SetAttribute("minHeight", 4)
			dot.SetAttribute("maxHeight", 4)
			dot.SetAttribute("borderRadius", 2)
			dot.SetAttribute("background", dotColor)
			dot.SetAttribute("gradientTop", "transparent")
			dot.SetAttribute("gradientBottom", "transparent")
			dot.SetAttribute("borderColor", "transparent")
			dot.SetAttribute("interactive", false)
			dotRow.AddChild(dot)
		}
		col.AddChild(dotRow)

		icon := ui.NewElement("Image")
		icon.SetAttribute("src", g.IconPath)
		icon.SetAttribute("srcOpaque", false)
		icon.SetAttribute("scaleMode", "contain")
		icon.SetAttribute("minWidth", taskIconSize)
		icon.SetAttribute("maxWidth", taskIconSize)
		icon.SetAttribute("minHeight", taskIconSize)
		icon.SetAttribute("maxHeight", taskIconSize)
		icon.SetAttribute("interactive", false)
		col.AddChild(icon)

		bottomSpacer := ui.NewElement("Element")
		bottomSpacer.SetAttribute("minHeight", 4)
		bottomSpacer.SetAttribute("maxHeight", 4)
		bottomSpacer.SetAttribute("interactive", false)
		col.AddChild(bottomSpacer)

		btn.AddChild(col)

		groupKey := g.Key
		btn.BindSignal("clicked", func() { a.activateGroup(groupKey) })
		btn.BindSignal("secondaryClicked", func() { a.openGroupMenu(groupKey, btn) })
		strip.AddChild(btn)
	}

	if a.taskPopup != nil {
		if _, ok := a.findGroup(a.taskPopupKey); !ok {
			a.closeTaskMenu()
		}
	}

	a.resizeDockToContent()
	a.Redraw()
}

func (a *TaskbarApp) resizeDockToContent() {
	n := len(a.groups)
	if n < 1 {
		n = 1
	}
	itemW := taskIconSize + 8
	width := dockOuterPadX*2 + dockInnerPadX*2 + n*itemW
	if n > 1 {
		width += (n - 1) * dockItemGap
	}
	if width < dockMinWidth {
		width = dockMinWidth
	}
	if width == a.dockWidth {
		return
	}
	if err := db.Resize(width, taskbarHeight); err != nil {
		log.Warn("failed to resize dock: %v", err)
		return
	}
	a.dockWidth = width
}

func (a *TaskbarApp) activateGroup(key string) {
	g, ok := a.findGroup(key)
	if !ok {
		return
	}
	a.closeTaskMenu()

	if len(g.WindowIDs) > 0 {
		target := g.WindowIDs[0]
		if g.Focused && len(g.WindowIDs) > 1 {
			for i, wid := range g.WindowIDs {
				if a.windowFocused(wid) {
					target = g.WindowIDs[(i+1)%len(g.WindowIDs)]
					break
				}
			}
		}
		_ = db.SetWindowState(target, display.WindowActionFocus)
		return
	}

	if g.HasEntry && strings.TrimSpace(g.Entry.ExecPath) != "" {
		if err := runCommand(g.Entry.ExecPath); err != nil {
			log.Warn("launch failed for %s: %v", g.Entry.ID, err)
		}
	}
}

func (a *TaskbarApp) openGroupMenu(key string, anchor *ui.Element) {
	if anchor == nil {
		return
	}
	if a.taskPopup != nil && a.taskPopupKey == key {
		a.closeTaskMenu()
		return
	}

	g, ok := a.findGroup(key)
	if !ok {
		return
	}

	items := a.buildGroupMenuItems(g)
	if len(items) == 0 {
		return
	}

	a.closeTaskMenu()

	const (
		menuW      = 236
		rowHeight  = 34
		rowSpacing = 3
		menuPad    = 7
		menuOffset = 8
	)
	menuH := menuPad*2 + len(items)*rowHeight + (len(items)-1)*rowSpacing
	content := buildContextMenu(menuW, menuH, items)

	b := anchor.Bounds()
	x := b.X + (b.W-menuW)/2
	y := b.Y - menuH - menuOffset

	var popup *displaybackend.Popup
	popup = db.OpenPopup(x, y, menuW, menuH, content, func() {
		if a.taskPopup == popup {
			a.taskPopup = nil
			a.taskPopupKey = ""
		}
	})
	if popup == nil {
		return
	}
	a.taskPopup = popup
	a.taskPopupKey = key
}

func buildContextMenu(menuW, menuH int, items []menuItem) *ui.Element {
	root := ui.NewElement("VBox")
	root.SetAttribute("direction", "column")
	root.SetAttribute("spacing", 3)
	root.SetAttribute("padding", "7")
	root.SetAttribute("minWidth", menuW)
	root.SetAttribute("maxWidth", menuW)
	root.SetAttribute("minHeight", menuH)
	root.SetAttribute("maxHeight", menuH)
	root.SetAttribute("background", "theme.color.surface.card")
	root.SetAttribute("gradientTop", "transparent")
	root.SetAttribute("gradientBottom", "transparent")
	root.SetAttribute("borderColor", "theme.color.stroke.hairline")
	root.SetAttribute("borderRadius", 14)
	root.SetAttribute("shadow", true)
	root.SetAttribute("shadowColor", "theme.color.stroke.divider")
	root.SetAttribute("shadowSpread", 14)
	root.SetAttribute("shadowGap", 1)
	root.SetAttribute("shadowOffsetY", 4)

	for i := range items {
		item := items[i]
		btn := ui.NewElement("Button")
		btn.SetID("CtxMenuItem" + strconv.Itoa(i))
		btn.SetAttribute("text", item.Label)
		btn.SetAttribute("textAlign", "start")
		btn.SetAttribute("padding", "0 10")
		btn.SetAttribute("minHeight", 34)
		btn.SetAttribute("maxHeight", 34)
		btn.SetAttribute("expand", false)
		btn.SetAttribute("srcOpaque", false)
		btn.SetAttribute("focusRing", false)
		btn.SetAttribute("borderRadius", 8)
		btn.SetAttribute("background", "transparent")
		btn.SetAttribute("borderColor", "transparent")
		btn.SetAttribute("focusedBorderColor", "transparent")
		btn.SetAttribute("hoverBackground", "theme.color.control.hover")
		btn.SetAttribute("pressedBackground", "theme.color.control.pressed")
		if item.Destructive {
			btn.SetAttribute("textColor", "theme.color.semantic.danger")
		}
		if item.Run != nil {
			btn.BindSignal("clicked", item.Run)
		}
		root.AddChild(btn)
	}
	return root
}

func (a *TaskbarApp) buildGroupMenuItems(g taskGroup) []menuItem {
	items := make([]menuItem, 0, 4+len(g.Entry.Actions))

	if len(g.WindowIDs) > 0 {
		ids := append([]uint32(nil), g.WindowIDs...)
		items = append(items, menuItem{
			Label:       "Close",
			Destructive: true,
			Run: func() {
				a.closeTaskMenu()
				for _, id := range ids {
					_ = db.SetWindowState(id, display.WindowActionClose)
				}
			},
		})
	}

	if g.HasEntry && strings.TrimSpace(g.Entry.ExecPath) != "" {
		entry := g.Entry
		items = append(items, menuItem{
			Label: "New Window",
			Run: func() {
				a.closeTaskMenu()
				if err := runCommand(entry.ExecPath); err != nil {
					log.Warn("new window failed for %s: %v", entry.ID, err)
				}
			},
		})
	}

	if g.HasEntry && strings.TrimSpace(g.AppID) != "" {
		id := g.AppID
		if g.Pinned {
			items = append(items, menuItem{
				Label: "Unpin",
				Run: func() {
					a.closeTaskMenu()
					a.setPinned(id, false)
				},
			})
		} else {
			items = append(items, menuItem{
				Label: "Pin",
				Run: func() {
					a.closeTaskMenu()
					a.setPinned(id, true)
				},
			})
		}
	}

	if g.HasEntry {
		entry := g.Entry
		for _, rawAction := range entry.Actions {
			action := rawAction
			label := strings.TrimSpace(action.Label)
			if label == "" {
				label = strings.TrimSpace(action.Name)
			}
			if label == "" {
				continue
			}
			labelCopy := label
			items = append(items, menuItem{
				Label: labelCopy,
				Run: func() {
					a.closeTaskMenu()
					if err := runManifestAction(entry, action); err != nil {
						log.Warn("action failed for %s (%s): %v", entry.ID, labelCopy, err)
					}
				},
			})
		}
	}

	return items
}

func (a *TaskbarApp) findGroup(key string) (taskGroup, bool) {
	for _, g := range a.groups {
		if g.Key == key {
			return g, true
		}
	}
	return taskGroup{}, false
}

func (a *TaskbarApp) windowFocused(windowID uint32) bool {
	for _, w := range a.windows {
		if w.ID == windowID {
			return w.Focused
		}
	}
	return false
}

func (a *TaskbarApp) matchWindowToEntry(win display.WindowInfo) (appcatalog.Entry, string, bool) {
	for _, entry := range a.catalog {
		id := strings.ToLower(strings.TrimSpace(entry.ID))
		if id == "" {
			continue
		}
		if windowMatchesLookupKeys(win, entryLookupKeys(entry)) {
			return entry, id, true
		}
	}
	return appcatalog.Entry{}, "", false
}

func runManifestAction(entry appcatalog.Entry, action appcatalog.ManifestAction) error {
	command := strings.TrimSpace(action.Command)
	args := append([]string(nil), action.Args...)

	if command == "" {
		if strings.TrimSpace(entry.ExecPath) == "" {
			return fmt.Errorf("missing app executable")
		}
		return runCommand(entry.ExecPath, args...)
	}

	if len(args) == 0 {
		parts := strings.Fields(command)
		if len(parts) == 0 {
			return fmt.Errorf("empty action command")
		}
		command = parts[0]
		args = parts[1:]
	}
	return runCommand(command, args...)
}

func runCommand(path string, args ...string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty command")
	}
	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Start()
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Error("dock error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	if err := logger.SetupSystemLog(); err != nil {
		log.Error("failed to setup system log: %v", err)
	}
	graphics.DefaultTheme.BorderRadius = 8

	position := resolveDockPosition(strings.TrimSpace(flagPosition))
	anchor := layerAnchorForPosition(position)

	db = displaybackend.New()
	db.SetSize(dockMinWidth, taskbarHeight)
	db.SetLayer(display.LayerTop, anchor, taskbarExclusive)

	home, _ := os.UserHomeDir()
	a := &TaskbarApp{home: home}
	a.SetOptions(gapp.Options{
		Title:      "Dock",
		Height:     taskbarHeight,
		Backend:    db,
		Input:      db,
		Background: graphics.ColorTransparent,
	})
	if err := a.LoadString(taskbarUI, a); err != nil {
		log.Error("failed to load dock ui: %v", err)
		os.Exit(1)
	}
	a.Configure(func(app *gapp.App) {
		app.OnEscape = a.Quit
	})

	a.syncCatalogAndPins(true)
	a.refreshWindows()
	a.resizeDockToContent()
	go watchDockSettings(a)
	go func() {
		refreshTicker := time.NewTicker(150 * time.Millisecond)
		defer refreshTicker.Stop()
		reloadEvery := 0
		for {
			select {
			case <-refreshTicker.C:
				reloadEvery++
				if reloadEvery >= 4 {
					reloadEvery = 0
					a.syncCatalogAndPins(false)
				}
				a.refreshWindows()
			}
		}
	}()

	if err := a.Run(); err != nil {
		log.Error("dock error: %v", err)
		os.Exit(1)
	}
	return nil
}

func resolveDockPosition(flagValue string) string {
	position := strings.ToLower(strings.TrimSpace(flagValue))
	if !flagProvided("position") {
		if configured, ok := loadSettingCompat(keyDockPosition, legacyDockKey); ok {
			position = strings.ToLower(strings.TrimSpace(configured))
		}
	}

	switch position {
	case "top":
		return "top"
	case "bottom":
		return "bottom"
	default:
		return defaultDockPos
	}
}

func layerAnchorForPosition(position string) uint32 {
	if strings.ToLower(strings.TrimSpace(position)) == "top" {
		return uint32(display.AnchorTop | display.AnchorHorizontalCenter)
	}
	return uint32(display.AnchorBottom | display.AnchorHorizontalCenter)
}

func applyDockPositionSetting(key, value string) {
	position := strings.ToLower(strings.TrimSpace(value))
	if position != "top" && position != "bottom" {
		log.Warn("ignoring invalid %s setting %q", key, value)
		return
	}
	if err := db.ReconfigureLayer(layerAnchorForPosition(position), taskbarExclusive); err != nil {
		log.Warn("failed to apply %s %q: %v", key, position, err)
	}
}

func watchDockSettings(app *TaskbarApp) {
	for {
		client, err := settingsapi.Connect()
		if err != nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		disconnected := make(chan struct{}, 1)
		client.OnDisconnect(func() {
			select {
			case disconnected <- struct{}{}:
			default:
			}
		})

		if value, ok := getSetting(client, keyDockPosition); ok {
			applyDockPositionSetting(keyDockPosition, value)
		} else if value, ok := getSetting(client, legacyDockKey); ok {
			applyDockPositionSetting(legacyDockKey, value)
		}
		if app != nil {
			if value, ok := getSetting(client, keyDockPinned); ok {
				app.applyPinnedIDs(parsePinnedIDs(value))
			} else if value, ok := getSetting(client, legacyPinnedKey); ok {
				app.applyPinnedIDs(parsePinnedIDs(value))
			}
		}

		client.OnChanged(func(ev settingsapi.ChangedEvent) {
			switch ev.Key {
			case keyDockPosition, legacyDockKey:
				applyDockPositionSetting(ev.Key, ev.Value)
			case keyDockPinned, legacyPinnedKey:
				if app == nil {
					return
				}
				app.applyPinnedIDs(parsePinnedIDs(ev.Value))
			}
		})

		<-disconnected
		_ = client.Close()
		time.Sleep(200 * time.Millisecond)
	}
}

func loadSettingCompat(primary string, fallback ...string) (string, bool) {
	for range 6 {
		client, err := settingsapi.Connect()
		if err == nil {
			if value, ok := getSetting(client, primary); ok {
				_ = client.Close()
				return strings.TrimSpace(value), true
			}
			for _, key := range fallback {
				if value, ok := getSetting(client, key); ok {
					_ = client.Close()
					return strings.TrimSpace(value), true
				}
			}
			_ = client.Close()
			return "", false
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", false
}

func getSetting(client *settingsapi.Client, key string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}
	value, err := client.Get(key)
	if err != nil {
		return "", false
	}
	return value, true
}

func setSetting(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("setting key is required")
	}
	for range 4 {
		client, err := settingsapi.Connect()
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		err = client.Set(key, value)
		_ = client.Close()
		return err
	}
	return fmt.Errorf("settings service unavailable")
}

func loadDockPinsFromSettings() ([]string, bool) {
	value, ok := loadSettingCompat(keyDockPinned, legacyPinnedKey)
	if !ok {
		return nil, false
	}
	return parsePinnedIDs(value), true
}

func saveDockPinsToSettings(pins []string) error {
	value := strings.Join(normalizePins(pins), ",")
	return setSetting(keyDockPinned, value)
}

func parsePinnedIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	})
	return normalizePins(parts)
}

func normalizePins(pins []string) []string {
	out := make([]string, 0, len(pins))
	seen := make(map[string]struct{}, len(pins))
	for _, id := range pins {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func flagProvided(name string) bool {
	long := "--" + name
	short := "-" + name
	for _, arg := range os.Args[1:] {
		if arg == long || arg == short {
			return true
		}
		if strings.HasPrefix(arg, long+"=") || strings.HasPrefix(arg, short+"=") {
			return true
		}
	}
	return false
}

func filterPins(pins []string, catalog []appcatalog.Entry) []string {
	cat := make(map[string]struct{}, len(catalog))
	for _, entry := range catalog {
		cat[strings.ToLower(strings.TrimSpace(entry.ID))] = struct{}{}
	}

	seen := make(map[string]struct{}, len(pins))
	out := make([]string, 0, len(pins))
	for _, id := range pins {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		if _, ok := cat[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func sameIDs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if strings.ToLower(strings.TrimSpace(left[i])) != strings.ToLower(strings.TrimSpace(right[i])) {
			return false
		}
	}
	return true
}

func catalogSignature(entries []appcatalog.Entry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for i, entry := range entries {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(entry.ID)
		b.WriteByte('|')
		b.WriteString(entry.ExecPath)
		b.WriteByte('|')
		b.WriteString(entry.IconPath)
		for _, action := range entry.Actions {
			b.WriteByte('|')
			b.WriteString(action.Label)
			b.WriteByte(':')
			b.WriteString(action.Command)
			if len(action.Args) > 0 {
				b.WriteByte(':')
				b.WriteString(strings.Join(action.Args, ","))
			}
		}
	}
	return b.String()
}

func windowSignature(wins []display.WindowInfo) string {
	if len(wins) == 0 {
		return ""
	}
	var b strings.Builder
	for i, w := range wins {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%d:%t:%t:%t:%s", w.ID, w.Focused, w.Visible, w.Minimized, strings.ToLower(strings.TrimSpace(w.Title))))
	}
	return b.String()
}

func groupsSignature(groups []taskGroup) string {
	if len(groups) == 0 {
		return ""
	}
	var b strings.Builder
	for i, g := range groups {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(g.Key)
		b.WriteByte('|')
		b.WriteString(g.IconPath)
		b.WriteByte('|')
		b.WriteString(g.Title)
		b.WriteByte('|')
		b.WriteString(strconv.FormatBool(g.Focused))
		b.WriteByte('|')
		b.WriteString(strconv.FormatBool(g.Pinned))
		b.WriteByte('|')
		b.WriteString(strconv.Itoa(len(g.WindowIDs)))
	}
	return b.String()
}

func entryLookupKeys(entry appcatalog.Entry) map[string]struct{} {
	keys := make(map[string]struct{}, 8)
	addAppLookupKey(keys, entry.ID)
	addAppLookupKey(keys, entry.Name)
	addAppLookupKey(keys, entry.IconName)
	addAppLookupKey(keys, entry.DirName)
	return keys
}

func windowMatchesLookupKeys(win display.WindowInfo, keys map[string]struct{}) bool {
	for _, key := range taskTitleLookupKeys(win.Title) {
		if _, ok := keys[key]; ok {
			return true
		}
	}
	return false
}

func addAppLookupKey(dst map[string]struct{}, raw string) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return
	}
	dst[raw] = struct{}{}
	if compact := compactAlphaNumKey(raw); compact != "" {
		dst[compact] = struct{}{}
	}
}

func compactAlphaNumKey(raw string) string {
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func taskTitleLookupKeys(title string) []string {
	trimmed := strings.TrimSpace(strings.ToLower(title))
	if trimmed == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	add := func(v string) {
		v = strings.TrimSpace(strings.ToLower(v))
		if v == "" {
			return
		}
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
		if compact := compactAlphaNumKey(v); compact != "" {
			if _, ok := seen[compact]; !ok {
				seen[compact] = struct{}{}
				out = append(out, compact)
			}
		}
	}

	add(trimmed)
	for _, sep := range []string{" - ", " — ", " | ", ":"} {
		if idx := strings.Index(trimmed, sep); idx > 0 {
			add(trimmed[:idx])
		}
	}
	if idx := strings.Index(trimmed, " ("); idx > 0 {
		add(trimmed[:idx])
	}
	parts := strings.Fields(trimmed)
	if len(parts) > 0 {
		add(parts[0])
	}
	return out
}
