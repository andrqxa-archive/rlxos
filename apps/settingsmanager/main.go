package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	settingsapi "avyos.dev/api/settings"
	"avyos.dev/pkg/fs"
	gapp "avyos.dev/pkg/graphics/app"
	"avyos.dev/pkg/graphics/ui"
	"avyos.dev/pkg/ini"
	"avyos.dev/pkg/logger"
	"avyos.dev/pkg/sutra"
)

//go:embed ui/settingsmanager.ui
var settingsManagerUI string

const (
	defaultWallpaper = "/avyos/data/backgrounds/default.png"
	defaultDockPos   = "bottom"
	defaultBgColor   = ""
)

var (
	wallpaperDirs = []string{
		"/data/backgrounds",
		"/avyos/data/backgrounds",
	}
	colorPresets = []string{
		"",
		"#11161e",
		"#1f2937",
		"#334155",
		"#64748b",
		"#f8fafc",
		"#ef4444",
		"#f97316",
		"#eab308",
		"#22c55e",
		"#06b6d4",
		"#3b82f6",
		"#a855f7",
	}
)

var log = logger.New("settings")
var flagDaemon bool

func init() {
	flag.BoolVar(&flagDaemon, "daemon", false, "Run as settings daemon service")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "settingsmanager - Settings UI and daemon")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  settingsmanager [--daemon]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  (none; use --daemon mode)")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/app error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

type settingsStore struct {
	path string
	cfg  *ini.Config
	mu   sync.RWMutex
}

func newSettingsStore(path string) (*settingsStore, error) {
	store := &settingsStore{
		path: path,
		cfg:  ini.NewConfig(),
	}

	if data, err := os.ReadFile(path); err == nil {
		cfg, perr := ini.Parse(bytes.NewReader(data))
		if perr != nil {
			return nil, perr
		}
		store.cfg = cfg
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	store.ensureDefaults()
	if err := store.saveLocked(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *settingsStore) ensureDefaults() {
	if _, ok := s.getLocked("background.wallpaper"); !ok {
		s.setLocked("background.wallpaper", defaultWallpaper)
	}
	if _, ok := s.getLocked("dock.position"); !ok {
		s.setLocked("dock.position", defaultDockPos)
	}
	if _, ok := s.getLocked("background.color"); !ok {
		s.setLocked("background.color", defaultBgColor)
	}
}

func (s *settingsStore) getLocked(key string) (string, bool) {
	section, name := splitConfigKey(key)
	return s.cfg.Get(section, name)
}

func (s *settingsStore) setLocked(key, value string) {
	section, name := splitConfigKey(key)
	s.cfg.Set(section, name, value)
}

func (s *settingsStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	var out bytes.Buffer
	if err := s.cfg.Write(&out); err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, out.Bytes(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *settingsStore) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.getLocked(key)
	if !ok {
		return "", fmt.Errorf("setting not found: %s", key)
	}
	return value, nil
}

func (s *settingsStore) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.setLocked(key, value)
	return s.saveLocked()
}

func (s *settingsStore) List(prefix string) []settingsapi.Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix = strings.TrimSpace(prefix)
	out := make([]settingsapi.Entry, 0, 16)
	for section, sec := range s.cfg.Sections {
		if sec == nil {
			continue
		}
		for _, entry := range sec.Entries {
			if entry == nil || entry.Type != ini.EntryKeyValue {
				continue
			}
			key := entry.Key
			if section != "" {
				key = section + "." + entry.Key
			}
			if prefix != "" && !strings.HasPrefix(key, prefix) {
				continue
			}
			out = append(out, settingsapi.Entry{Key: key, Value: entry.Value})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out
}

type settingsApp struct {
	ui.App
	pendingWallpaper string
	pendingColor     string
	wallpapers       []string
}

func (a *settingsApp) e(id string) *ui.Element { return a.FindElement(id) }

func (a *settingsApp) setStatus(text string) {
	if el := a.e("Status"); el != nil {
		el.SetAttribute("text", text)
	}
}

func (a *settingsApp) setText(id, value string) {
	if el := a.e(id); el != nil {
		el.SetAttribute("text", value)
	}
}

func (a *settingsApp) valueOf(id string) string {
	if el := a.e(id); el != nil {
		return strings.TrimSpace(el.Attr("text", ""))
	}
	return ""
}

func (a *settingsApp) connect() (*settingsapi.Client, error) {
	client, err := settingsapi.Connect()
	if err != nil {
		return nil, fmt.Errorf("settings daemon unavailable: %w", err)
	}
	return client, nil
}

func (a *settingsApp) loadValues() error {
	client, err := a.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	wallpaper, err := client.Get("background.wallpaper")
	if err != nil {
		return err
	}
	dockPos, err := client.Get("dock.position")
	if err != nil {
		return err
	}
	color, _ := client.Get("background.color")

	a.pendingWallpaper = strings.TrimSpace(wallpaper)
	a.pendingColor = normalizeColor(strings.TrimSpace(color))
	a.setText("DockPosition", strings.ToLower(strings.TrimSpace(dockPos)))
	a.refreshWallpaperSelectionLabel()
	a.refreshColorSelectionLabel()
	a.setText("CustomColorInput", a.pendingColor)
	return nil
}

func (a *settingsApp) refreshWallpaperSelectionLabel() {
	value := strings.TrimSpace(a.pendingWallpaper)
	if value == "" {
		a.setText("SelectedWallpaper", "(none)")
		return
	}
	a.setText("SelectedWallpaper", value)
}

func (a *settingsApp) refreshColorSelectionLabel() {
	if a.pendingColor == "" {
		a.setText("SelectedColor", "Default theme")
		return
	}
	a.setText("SelectedColor", strings.ToLower(a.pendingColor))
}

func (a *settingsApp) selectWallpaper(path string) {
	a.pendingWallpaper = strings.TrimSpace(path)
	a.refreshWallpaperSelectionLabel()
	a.populateWallpaperPicker()
	a.setStatus("Pending changes")
}

func (a *settingsApp) discoverWallpapers() []string {
	seen := make(map[string]struct{}, 32)
	out := make([]string, 0, 32)

	for _, dir := range wallpaperDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			switch ext {
			case ".png", ".jpg", ".jpeg", ".webp", ".bmp":
			default:
				continue
			}

			full := filepath.Join(dir, entry.Name())
			key := strings.ToLower(entry.Name())
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, full)
		}
	}

	sort.Strings(out)
	return out
}

func (a *settingsApp) populateWallpaperPicker() {
	flow := a.e("WallpaperFlow")
	if flow == nil {
		return
	}
	flow.ClearChildren()
	a.wallpapers = a.discoverWallpapers()

	if len(a.wallpapers) == 0 {
		empty := ui.NewElement("Label")
		empty.SetAttribute("text", "No wallpapers found in /data/backgrounds or /avyos/data/backgrounds")
		empty.SetAttribute("textColor", "theme.color.text.secondary")
		flow.AddChild(empty)
		return
	}

	selected := strings.TrimSpace(a.pendingWallpaper)
	for _, path := range a.wallpapers {
		btn := ui.NewElement("Button")
		btn.SetAttribute("text", "")
		btn.SetAttribute("padding", "6")
		btn.SetAttribute("minWidth", 132)
		btn.SetAttribute("maxWidth", 132)
		btn.SetAttribute("minHeight", 112)
		btn.SetAttribute("maxHeight", 112)
		btn.SetAttribute("textAlign", "left")

		if selected == path {
			btn.SetAttribute("background", "theme.color.accent.subtle")
			btn.SetAttribute("borderColor", "theme.color.accent")
			btn.SetAttribute("focusedBorderColor", "theme.color.accent")
		} else {
			btn.SetAttribute("background", "theme.color.control.fill")
		}

		box := ui.NewElement("VBox")
		box.SetAttribute("direction", "column")
		box.SetAttribute("spacing", 4)
		box.SetAttribute("expand", true)
		box.SetAttribute("interactive", false)

		img := ui.NewElement("Image")
		img.SetAttribute("src", path)
		img.SetAttribute("srcOpaque", false)
		img.SetAttribute("scaleMode", "cover")
		img.SetAttribute("minWidth", 116)
		img.SetAttribute("maxWidth", 116)
		img.SetAttribute("minHeight", 72)
		img.SetAttribute("maxHeight", 72)
		img.SetAttribute("interactive", false)

		label := ui.NewElement("Label")
		label.SetAttribute("text", filepath.Base(path))
		label.SetAttribute("clipText", true)
		label.SetAttribute("textAlign", "left")
		label.SetAttribute("minWidth", 116)
		label.SetAttribute("maxWidth", 116)
		label.SetAttribute("interactive", false)

		box.AddChild(img)
		box.AddChild(label)
		btn.AddChild(box)

		pathCopy := path
		btn.BindSignal("clicked", func() { a.selectWallpaper(pathCopy) })
		flow.AddChild(btn)
	}
}

func normalizeColor(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "#") {
		value = "#" + value
	}
	value = strings.ToLower(value)
	if !isHexColor(value) {
		return ""
	}
	return value
}

func (a *settingsApp) selectColor(value string) {
	a.pendingColor = normalizeColor(value)
	a.setText("CustomColorInput", a.pendingColor)
	a.refreshColorSelectionLabel()
	a.populateColorPicker()
	a.setStatus("Pending changes")
}

func (a *settingsApp) SelectCustomColor() {
	value := normalizeColor(a.valueOf("CustomColorInput"))
	if value == "" {
		a.pendingColor = ""
		a.setText("CustomColorInput", "")
	} else {
		a.pendingColor = value
		a.setText("CustomColorInput", value)
	}
	a.refreshColorSelectionLabel()
	a.populateColorPicker()
	a.setStatus("Pending changes")
}

func (a *settingsApp) CustomColorSubmitted(_ string) {
	a.SelectCustomColor()
}

func (a *settingsApp) populateColorPicker() {
	flow := a.e("ColorFlow")
	if flow == nil {
		return
	}
	flow.ClearChildren()

	selected := normalizeColor(a.pendingColor)
	for _, preset := range colorPresets {
		value := normalizeColor(preset)

		btn := ui.NewElement("Button")
		btn.SetAttribute("text", "")
		btn.SetAttribute("minWidth", 34)
		btn.SetAttribute("maxWidth", 34)
		btn.SetAttribute("minHeight", 34)
		btn.SetAttribute("maxHeight", 34)
		btn.SetAttribute("padding", "0")
		btn.SetAttribute("shadow", false)

		if value == "" {
			btn.SetAttribute("text", "D")
			btn.SetAttribute("textRole", "subheading")
			btn.SetAttribute("background", "#FFFFFF")
			btn.SetAttribute("textColor", "#11161e")
		} else {
			btn.SetAttribute("background", value)
		}

		if selected == value {
			btn.SetAttribute("borderColor", "theme.color.accent")
			btn.SetAttribute("focusedBorderColor", "theme.color.accent")
		} else {
			btn.SetAttribute("borderColor", "theme.color.stroke.hairline")
			btn.SetAttribute("focusedBorderColor", "theme.color.stroke.hairline")
		}

		valueCopy := value
		btn.BindSignal("clicked", func() { a.selectColor(valueCopy) })
		flow.AddChild(btn)
	}
}

func (a *settingsApp) ShowDesktop() {
	if card := a.e("DesktopCard"); card != nil {
		card.SetVisible(true)
	}
	if card := a.e("DockCard"); card != nil {
		card.SetVisible(false)
	}
}

func (a *settingsApp) ShowDock() {
	if card := a.e("DesktopCard"); card != nil {
		card.SetVisible(false)
	}
	if card := a.e("DockCard"); card != nil {
		card.SetVisible(true)
	}
}

func (a *settingsApp) setSetting(key, value string) error {
	client, err := a.connect()
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Set(key, value)
}

func (a *settingsApp) setDockPositionField(position string) {
	position = strings.ToLower(strings.TrimSpace(position))
	if position != "top" && position != "bottom" {
		a.setStatus("Dock position must be top or bottom")
		return
	}
	a.setText("DockPosition", position)
	a.setStatus("Pending changes")
}

func (a *settingsApp) SetDockTop() {
	a.setDockPositionField("top")
}

func (a *settingsApp) SetDockBottom() {
	a.setDockPositionField("bottom")
}

func (a *settingsApp) Apply() {
	wallpaper := strings.TrimSpace(a.pendingWallpaper)
	if wallpaper == "" {
		a.setStatus("Select a wallpaper")
		return
	}
	if _, err := os.Stat(wallpaper); err != nil {
		a.setStatus(fmt.Sprintf("Wallpaper path invalid: %v", err))
		return
	}

	dockPos := strings.ToLower(strings.TrimSpace(a.valueOf("DockPosition")))
	if dockPos != "top" && dockPos != "bottom" {
		a.setStatus("Dock position must be top or bottom")
		return
	}

	bgColor := normalizeColor(a.pendingColor)

	if err := a.setSetting("background.wallpaper", wallpaper); err != nil {
		a.setStatus(err.Error())
		return
	}
	if err := a.setSetting("dock.position", dockPos); err != nil {
		a.setStatus(err.Error())
		return
	}
	if err := a.setSetting("background.color", bgColor); err != nil {
		a.setStatus(err.Error())
		return
	}

	a.setStatus("Settings applied")
}

func (a *settingsApp) Reset() {
	if err := a.loadValues(); err != nil {
		a.setStatus(err.Error())
		return
	}
	a.populateWallpaperPicker()
	a.populateColorPicker()
	a.setStatus("Changes discarded")
}

func (a *settingsApp) WallpaperSubmitted(_ string) {
	// No-op: wallpaper uses picker buttons.
}

func (a *settingsApp) BackgroundColorSubmitted(_ string) {
	a.SelectCustomColor()
}

func isHexColor(value string) bool {
	v := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(v) != 6 && len(v) != 8 {
		return false
	}
	for _, ch := range v {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return false
		}
	}
	return true
}

func splitConfigKey(key string) (section, name string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", ""
	}
	idx := strings.IndexByte(key, '.')
	if idx <= 0 || idx == len(key)-1 {
		return "", key
	}
	return key[:idx], key[idx+1:]
}

func userSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "/users/default/.config/avyos/settings.ini"
	}
	return filepath.Join(home, ".config", "avyos", "settings.ini")
}

func runDaemon() error {
	if err := logger.SetupSystemLog(); err != nil {
		log.Error("failed to setup system log: %v", err)
	}

	store, err := newSettingsStore(userSettingsPath())
	if err != nil {
		return fmt.Errorf("load settings store: %w", err)
	}

	svc, err := sutra.NewService(settingsapi.ServiceName, fs.Resolve("user-service:"+settingsapi.ServiceName))
	if err != nil {
		return fmt.Errorf("start settings service: %w", err)
	}
	defer svc.Close()

	svc.Handle(settingsapi.RequestGet, func(t *sutra.Transaction) ([]byte, error) {
		key, err := settingsapi.DecodeKey(t.Payload)
		if err != nil {
			return nil, err
		}
		value, err := store.Get(key)
		if err != nil {
			return nil, err
		}
		return settingsapi.EncodeGetResponse(value), nil
	})

	svc.Handle(settingsapi.RequestSet, func(t *sutra.Transaction) ([]byte, error) {
		key, value, err := settingsapi.DecodeSetRequest(t.Payload)
		if err != nil {
			return nil, err
		}
		if err := store.Set(key, value); err != nil {
			return nil, err
		}
		_ = svc.Broadcast(settingsapi.EventChanged, settingsapi.EncodeChangedEvent(key, value))
		return nil, nil
	})

	svc.Handle(settingsapi.RequestList, func(t *sutra.Transaction) ([]byte, error) {
		d := sutra.NewDecoder(t.Payload)
		prefix := d.String()
		if err := d.Err(); err != nil {
			return nil, err
		}
		return settingsapi.EncodeEntryList(store.List(prefix)), nil
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = svc.Close()
	}()

	log.Info("settings daemon running at %s", settingsapi.ServiceName)
	svc.Run()
	return nil
}

func runUI() error {
	app := &settingsApp{}
	app.SetOptions(gapp.Options{Title: "Settings Manager"})
	if err := app.LoadString(settingsManagerUI, app); err != nil {
		return err
	}

	app.ShowDesktop()
	if err := app.loadValues(); err != nil {
		app.pendingWallpaper = defaultWallpaper
		app.pendingColor = normalizeColor(defaultBgColor)
		app.refreshWallpaperSelectionLabel()
		app.refreshColorSelectionLabel()
		app.setStatus(err.Error())
	} else {
		app.setStatus("Connected to settings daemon")
	}
	app.populateWallpaperPicker()
	app.populateColorPicker()

	return app.Run()
}

func main() {
	flag.Parse()
	if flagDaemon {
		if err := runDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "settingsmanager: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := runUI(); err != nil {
		fmt.Fprintf(os.Stderr, "settingsmanager: %v\n", err)
		os.Exit(1)
	}
}
