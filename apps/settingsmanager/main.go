package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/signal"
	osuser "os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	serviceapi "avyos.dev/api/service"
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
	defaultWallpaper       = "/avyos/data/backgrounds/default.png"
	defaultBackgroundScale = "cover"
	defaultDockPos         = "bottom"
	defaultBgColor         = ""

	settingValueColWidth = 420
	settingValueColMin   = 240
)

const (
	keyBackgroundSource = "/dev/rlxos/background/source"
	keyBackgroundScale  = "/dev/rlxos/background/scale"
	keyBackgroundColor  = "/dev/rlxos/background/color"
	keyDockPosition     = "/dev/rlxos/dock/position"
)

var (
	legacyKeyToCanonical = map[string]string{
		"background.wallpaper": keyBackgroundSource,
		"background.color":     keyBackgroundColor,
		"dock.position":        keyDockPosition,
	}
	canonicalToLegacy map[string][]string

	wallpaperDirs = []string{
		"/avyos/data/backgrounds",
		"/data/backgrounds",
	}

	backgroundColorChoices = []settingChoice{
		{Label: "Default", Value: ""},
		{Label: "Midnight", Value: "#11161e"},
		{Label: "Slate", Value: "#1f2937"},
		{Label: "Steel", Value: "#334155"},
		{Label: "Cloud", Value: "#f8fafc"},
		{Label: "Rose", Value: "#ef4444"},
		{Label: "Amber", Value: "#f59e0b"},
		{Label: "Emerald", Value: "#10b981"},
		{Label: "Ocean", Value: "#0284c7"},
	}
)

type fieldKind uint8

const (
	fieldText fieldKind = iota
	fieldToggle
	fieldChoice
)

type settingChoice struct {
	Label string
	Value string
}

type settingField struct {
	ID          string
	Key         string
	Label       string
	Hint        string
	Default     string
	Placeholder string
	Choices     []settingChoice
	Kind        fieldKind
	ReadOnly    bool
}

type settingSection struct {
	Title       string
	Description string
	Fields      []settingField
}

type settingPage struct {
	Name        string
	Title       string
	Description string
	Sections    []settingSection
}

func textSetting(id, key, label, hint, def, placeholder string) settingField {
	return settingField{
		ID:          id,
		Key:         key,
		Label:       label,
		Hint:        hint,
		Default:     def,
		Placeholder: placeholder,
		Kind:        fieldText,
	}
}

func toggleSetting(id, key, label, hint string, enabled bool) settingField {
	def := "false"
	if enabled {
		def = "true"
	}
	return settingField{
		ID:      id,
		Key:     key,
		Label:   label,
		Hint:    hint,
		Default: def,
		Kind:    fieldToggle,
	}
}

func choiceSetting(id, key, label, hint, def string, choices ...settingChoice) settingField {
	cp := make([]settingChoice, 0, len(choices))
	for _, choice := range choices {
		cp = append(cp, settingChoice{
			Label: strings.TrimSpace(choice.Label),
			Value: strings.TrimSpace(choice.Value),
		})
	}
	return settingField{
		ID:      id,
		Key:     key,
		Label:   label,
		Hint:    hint,
		Default: def,
		Choices: cp,
		Kind:    fieldChoice,
	}
}

func readOnlyTextSetting(id, key, label, hint, def string) settingField {
	field := textSetting(id, key, label, hint, def, def)
	field.ReadOnly = true
	return field
}

func supportedSettingsPages() []settingPage {
	return []settingPage{
		{
			Name:        "appearance",
			Title:       "Appearance",
			Description: "Configure wallpaper, fit mode, background color, and dock placement.",
			Sections: []settingSection{
				{
					Title:       "Desktop Layout",
					Description: "Core desktop behavior that is currently applied by running services.",
					Fields: []settingField{
						choiceSetting(
							"BackgroundScale",
							keyBackgroundScale,
							"Scale Mode",
							"How wallpaper should fit the screen.",
							defaultBackgroundScale,
							settingChoice{Label: "Cover", Value: "cover"},
							settingChoice{Label: "Contain", Value: "contain"},
							settingChoice{Label: "Stretch", Value: "stretch"},
							settingChoice{Label: "Original", Value: "none"},
						),
						choiceSetting(
							"DockPosition",
							keyDockPosition,
							"Dock Position",
							"Place the dock at the top or bottom edge.",
							defaultDockPos,
							settingChoice{Label: "Bottom", Value: "bottom"},
							settingChoice{Label: "Top", Value: "top"},
						),
					},
				},
			},
		},
		{
			Name:        "about-system",
			Title:       "About System",
			Description: "Runtime information detected from this system.",
			Sections: []settingSection{
				{
					Title:       "Identity",
					Description: "Product and release metadata.",
					Fields: []settingField{
						readOnlyTextSetting("SystemName", "/dev/rlxos/system/about/name", "System Name", "Distribution name.", "RlxOS"),
						readOnlyTextSetting("SystemVersion", "/dev/rlxos/system/about/version", "Version", "Operating system version.", "0.1.0"),
						readOnlyTextSetting("SystemBuild", "/dev/rlxos/system/about/build", "Build", "Build identifier.", "dev"),
					},
				},
				{
					Title:       "Runtime",
					Description: "Live environment and kernel details.",
					Fields: []settingField{
						readOnlyTextSetting("KernelVersion", "/dev/rlxos/system/about/kernel", "Kernel", "Kernel release string.", runtime.GOOS),
						readOnlyTextSetting("Architecture", "/dev/rlxos/system/about/architecture", "Architecture", "CPU architecture.", runtime.GOARCH),
						readOnlyTextSetting("SessionType", "/dev/rlxos/system/about/session", "Session", "Current session type.", "wayland"),
						readOnlyTextSetting("DefaultUser", "/dev/rlxos/accounts/default_user", "Default User", "Detected active user.", "user"),
					},
				},
			},
		},
	}
}

var settingsPages = supportedSettingsPages()

var (
	allSettingFields  []settingField
	settingFieldByKey map[string]settingField
)

func initSettingSchema() {
	allSettingFields = make([]settingField, 0, 256)
	settingFieldByKey = make(map[string]settingField, 256)
	for _, page := range settingsPages {
		for _, section := range page.Sections {
			for _, field := range section.Fields {
				if strings.TrimSpace(field.Key) == "" {
					continue
				}
				if _, exists := settingFieldByKey[field.Key]; exists {
					continue
				}
				allSettingFields = append(allSettingFields, field)
				settingFieldByKey[field.Key] = field
			}
		}
	}
	canonicalToLegacy = make(map[string][]string, len(legacyKeyToCanonical))
	for legacy, canonical := range legacyKeyToCanonical {
		canonicalToLegacy[canonical] = append(canonicalToLegacy[canonical], legacy)
	}
}

var log = logger.New("settings")
var flagDaemon bool
var flagList bool
var flagGet string
var flagSet string
var flagPrefix string

func init() {
	initSettingSchema()
	flag.BoolVar(&flagDaemon, "daemon", false, "Run as settings daemon service")
	flag.BoolVar(&flagList, "list", false, "List settings keys and values")
	flag.StringVar(&flagGet, "get", "", "Get setting value by key")
	flag.StringVar(&flagSet, "set", "", "Set a setting value (key=value)")
	flag.StringVar(&flagPrefix, "prefix", "", "Optional key prefix filter for --list")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "settingsmanager - Settings UI and daemon")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  settingsmanager [--daemon]")
		fmt.Fprintln(os.Stderr, "  settingsmanager --list [--prefix /dev/rlxos/...]")
		fmt.Fprintln(os.Stderr, "  settingsmanager --get <key>")
		fmt.Fprintln(os.Stderr, "  settingsmanager --set <key=value>")
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/app error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

func canonicalSettingKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if canonical, ok := legacyKeyToCanonical[trimmed]; ok {
		return canonical
	}
	return trimmed
}

func legacyKeysForCanonical(key string) []string {
	return canonicalToLegacy[strings.TrimSpace(key)]
}

func detectRuntimeDefaults() map[string]string {
	out := make(map[string]string, 64)

	if cfg, err := ini.ParseFile(fs.Resolve("config", "init.conf")); err == nil {
		if hostname, ok := cfg.Get("", "hostname"); ok {
			setIfNotEmpty(out, "/dev/rlxos/network/hostname", hostname)
		}
		if services, ok := cfg.Get("", "services"); ok {
			serviceSet := make(map[string]struct{}, 16)
			for _, name := range strings.Fields(services) {
				name = strings.ToLower(strings.TrimSpace(name))
				if name == "" {
					continue
				}
				serviceSet[name] = struct{}{}
			}
			if hasAnySetKey(serviceSet, "display", "login", "waylayer") {
				out["/dev/rlxos/services/startup/desktop"] = "true"
			}
			if hasAnySetKey(serviceSet, "network", "uevent") {
				out["/dev/rlxos/services/startup/network"] = "true"
			}
			if hasAnySetKey(serviceSet, "distro", "update", "updates") {
				out["/dev/rlxos/services/startup/updates"] = "true"
			}
		}

		if devices, ok := cfg.Get("network", "devices"); ok {
			mode := "auto"
			for _, dev := range strings.Fields(devices) {
				dev = strings.TrimSpace(dev)
				if dev == "" || dev == "lo" {
					continue
				}
				if strings.HasPrefix(strings.ToLower(dev), "wl") {
					out["/dev/rlxos/network/wifi/enabled"] = "true"
				}
				if ip, ok := cfg.Get("network."+dev, "ip"); ok && strings.TrimSpace(ip) != "" {
					mode = "static"
				}
			}
			setIfNotEmpty(out, "/dev/rlxos/network/mode", mode)
		}
	}

	if host, err := os.Hostname(); err == nil {
		setIfNotEmpty(out, "/dev/rlxos/network/hostname", host)
	}
	if dns := detectDNSServers(); dns != "" {
		out["/dev/rlxos/network/dns/servers"] = dns
	}

	if resolution := detectFramebufferResolution(); resolution != "" {
		out["/dev/rlxos/display/resolution"] = resolution
	}
	if brightness := detectBacklightBrightnessPercent(); brightness != "" {
		out["/dev/rlxos/display/brightness"] = brightness
	}

	hasBattery, batteryCapacity := detectBattery()
	if hasBattery {
		out["/dev/rlxos/power/battery_saver/enabled"] = "true"
		if batteryCapacity > 0 {
			threshold := 25
			if batteryCapacity < threshold {
				threshold = batteryCapacity
			}
			if threshold < 10 {
				threshold = 10
			}
			out["/dev/rlxos/power/battery_saver/threshold"] = strconv.Itoa(threshold)
		}
	} else {
		out["/dev/rlxos/power/battery_saver/enabled"] = "false"
	}

	if usr, err := osuser.Current(); err == nil {
		setIfNotEmpty(out, "/dev/rlxos/accounts/default_user", usr.Username)
		if home := strings.TrimSpace(usr.HomeDir); home != "" {
			avatar := filepath.Join(home, ".face")
			if _, err := os.Stat(avatar); err == nil {
				out["/dev/rlxos/accounts/avatar/path"] = avatar
			}
		}
	}

	if locale := normalizeLocale(os.Getenv("LANG")); locale != "" {
		out["/dev/rlxos/accounts/session/language"] = locale
	}
	if tz := strings.TrimSpace(os.Getenv("TZ")); tz != "" {
		out["/dev/rlxos/accounts/session/timezone"] = tz
	} else {
		out["/dev/rlxos/accounts/session/timezone"] = time.Now().Location().String()
	}

	if osRelease := readOSRelease(); len(osRelease) > 0 {
		if name := firstNonEmpty(osRelease["PRETTY_NAME"], osRelease["NAME"]); name != "" {
			out["/dev/rlxos/system/about/name"] = name
		}
		if version := firstNonEmpty(osRelease["VERSION_ID"], osRelease["VERSION"]); version != "" {
			out["/dev/rlxos/system/about/version"] = version
		}
		if build := firstNonEmpty(osRelease["BUILD_ID"], osRelease["IMAGE_ID"], osRelease["VERSION"]); build != "" {
			out["/dev/rlxos/system/about/build"] = build
		}
	}

	if kernel := readFirstNonEmpty(
		"/proc/sys/kernel/osrelease",
		fs.Resolve("process", "sys/kernel/osrelease"),
	); kernel != "" {
		out["/dev/rlxos/system/about/kernel"] = kernel
	}
	out["/dev/rlxos/system/about/architecture"] = runtime.GOARCH
	out["/dev/rlxos/system/about/session"] = detectSessionType()

	applyRuntimeServiceDefaults(out)
	return out
}

func applyRuntimeServiceDefaults(out map[string]string) {
	client, err := serviceapi.Connect()
	if err != nil {
		return
	}
	defer client.Close()

	items, err := client.List()
	if err != nil {
		return
	}
	active := make(map[string]bool, len(items))
	for _, item := range items {
		name := strings.ToLower(strings.TrimSpace(item.Name))
		if name == "" {
			continue
		}
		active[name] = item.Running || item.Started
	}

	setServiceToggle(out, active, "/dev/rlxos/services/startup/desktop", "display", "login", "waylayer", "background", "dock")
	setServiceToggle(out, active, "/dev/rlxos/services/startup/network", "network", "uevent")
	setServiceToggle(out, active, "/dev/rlxos/services/startup/updates", "distro", "updates", "update")
	setServiceToggle(out, active, "/dev/rlxos/services/ssh/enabled", "ssh", "sshd")
	setServiceToggle(out, active, "/dev/rlxos/services/remote_desktop/enabled", "rdp", "vnc", "remote-desktop", "remote_desktop")
	setServiceToggle(out, active, "/dev/rlxos/services/file_sharing/enabled", "samba", "smbd", "nfs")
	setServiceToggle(out, active, "/dev/rlxos/services/printing/enabled", "cups", "cupsd")
	setServiceToggle(out, active, "/dev/rlxos/services/telemetry/enabled", "telemetry")
}

func setServiceToggle(out map[string]string, active map[string]bool, key string, names ...string) {
	for _, name := range names {
		if active[strings.ToLower(strings.TrimSpace(name))] {
			out[key] = "true"
			return
		}
	}
}

func detectDNSServers() string {
	paths := []string{
		"/etc/resolv.conf",
		fs.Resolve("config", "resolv.conf"),
	}
	seen := make(map[string]struct{}, 8)
	servers := make([]string, 0, 4)
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 || strings.ToLower(fields[0]) != "nameserver" {
				continue
			}
			server := strings.TrimSpace(fields[1])
			if server == "" {
				continue
			}
			if _, ok := seen[server]; ok {
				continue
			}
			seen[server] = struct{}{}
			servers = append(servers, server)
		}
		_ = f.Close()
		if len(servers) > 0 {
			return strings.Join(servers, ", ")
		}
	}
	return ""
}

func detectFramebufferResolution() string {
	value := readFirstNonEmpty(
		"/sys/class/graphics/fb0/virtual_size",
		fs.Resolve("sysfs", "class/graphics/fb0/virtual_size"),
		fs.Resolve("process", "sys/class/graphics/fb0/virtual_size"),
	)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", "x")
	value = strings.ReplaceAll(value, " ", "")
	if strings.Count(value, "x") != 1 {
		return ""
	}
	parts := strings.Split(value, "x")
	if len(parts) != 2 {
		return ""
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return ""
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return ""
	}
	return value
}

func detectBacklightBrightnessPercent() string {
	roots := []string{
		"/sys/class/backlight",
		fs.Resolve("sysfs", "class/backlight"),
		fs.Resolve("process", "sys/class/backlight"),
	}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			cur, errCur := readIntFile(filepath.Join(dir, "brightness"))
			max, errMax := readIntFile(filepath.Join(dir, "max_brightness"))
			if errCur != nil || errMax != nil || max <= 0 {
				continue
			}
			pct := int((float64(cur) / float64(max) * 100) + 0.5)
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
			return strconv.Itoa(pct)
		}
	}
	return ""
}

func detectBattery() (bool, int) {
	roots := []string{
		"/sys/class/power_supply",
		fs.Resolve("sysfs", "class/power_supply"),
		fs.Resolve("process", "sys/class/power_supply"),
	}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(entry.Name()))
			dir := filepath.Join(root, entry.Name())
			kind := strings.ToLower(strings.TrimSpace(readFirstNonEmpty(filepath.Join(dir, "type"))))
			if kind != "battery" && !strings.HasPrefix(name, "bat") {
				continue
			}
			capacity, err := readIntFile(filepath.Join(dir, "capacity"))
			if err != nil {
				return true, -1
			}
			return true, capacity
		}
	}
	return false, -1
}

func readOSRelease() map[string]string {
	paths := []string{
		"/etc/os-release",
		"/usr/lib/os-release",
		fs.Resolve("config", "os-release"),
	}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		values := make(map[string]string, 16)
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if key == "" {
				continue
			}
			values[key] = value
		}
		_ = f.Close()
		if len(values) > 0 {
			return values
		}
	}
	return nil
}

func detectSessionType() string {
	if sessionType := strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE")); sessionType != "" {
		return strings.ToLower(sessionType)
	}
	if strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" || os.Getenv("AVYOS_SESSION_MODE") == "1" {
		return "wayland"
	}
	return "tty"
}

func normalizeLocale(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.IndexAny(value, ".@"); idx > 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func setIfNotEmpty(target map[string]string, key, value string) {
	if target == nil {
		return
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	target[key] = value
}

func hasAnySetKey(set map[string]struct{}, keys ...string) bool {
	for _, key := range keys {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if _, ok := set[key]; ok {
			return true
		}
	}
	return false
}

func readFirstNonEmpty(paths ...string) string {
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(data))
		if value != "" {
			return value
		}
	}
	return ""
}

func readIntFile(path string) (int, error) {
	value := readFirstNonEmpty(path)
	if value == "" {
		return 0, fmt.Errorf("empty value: %s", path)
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty value: %s", path)
	}
	return strconv.Atoi(parts[0])
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
	for legacy, canonical := range legacyKeyToCanonical {
		if _, ok := s.getLocked(canonical); ok {
			continue
		}
		if value, ok := s.getLocked(legacy); ok {
			s.setLocked(canonical, strings.TrimSpace(value))
		}
	}

	runtimeDefaults := detectRuntimeDefaults()
	for _, field := range allSettingFields {
		key := canonicalSettingKey(field.Key)
		if key == "" {
			continue
		}
		current, ok := s.getLocked(key)
		dynamic, hasDynamic := runtimeDefaults[key]
		if ok {
			// Upgrade only values that still match the schema default.
			if hasDynamic && strings.TrimSpace(dynamic) != "" && normalizeSettingValue(field, current) == normalizeSettingValue(field, field.Default) {
				s.setLocked(key, normalizeSettingValue(field, dynamic))
			}
			continue
		}
		if hasDynamic && strings.TrimSpace(dynamic) != "" {
			s.setLocked(key, normalizeSettingValue(field, dynamic))
			continue
		}
		s.setLocked(key, normalizeSettingValue(field, field.Default))
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

	key = strings.TrimSpace(key)
	canonical := canonicalSettingKey(key)
	if canonical != "" {
		if value, ok := s.getLocked(canonical); ok {
			return value, nil
		}
		for _, legacy := range legacyKeysForCanonical(canonical) {
			if value, ok := s.getLocked(legacy); ok {
				return value, nil
			}
		}
	}
	if key != "" && key != canonical {
		if value, ok := s.getLocked(key); ok {
			return value, nil
		}
	}
	return "", fmt.Errorf("setting not found: %s", key)
}

func (s *settingsStore) Set(key, value string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	canonical := canonicalSettingKey(key)
	if canonical == "" {
		return "", fmt.Errorf("setting key is required")
	}
	s.setLocked(canonical, value)
	if err := s.saveLocked(); err != nil {
		return "", err
	}
	return canonical, nil
}

func (s *settingsStore) List(prefix string) []settingsapi.Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix = strings.TrimSpace(prefix)
	out := make([]settingsapi.Entry, 0, 64)
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
	currentPage int
	values      map[string]string
	persisted   map[string]string
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

func (a *settingsApp) connect() (*settingsapi.Client, error) {
	client, err := settingsapi.Connect()
	if err != nil {
		return nil, fmt.Errorf("settings daemon unavailable: %w", err)
	}
	return client, nil
}

func (a *settingsApp) initializeValues() {
	a.values = make(map[string]string, len(allSettingFields))
	a.persisted = make(map[string]string, len(allSettingFields))
	for _, field := range allSettingFields {
		a.values[field.Key] = normalizeSettingValue(field, field.Default)
		a.persisted[field.Key] = normalizeSettingValue(field, field.Default)
	}
}

func (a *settingsApp) setPage(index int) {
	if index < 0 || index >= len(settingsPages) {
		return
	}
	a.currentPage = index
	if list := a.e("PageList"); list != nil {
		list.SetAttribute("selected", index)
	}
	a.renderPage()
}

func pageIndexByTitle(title string) int {
	title = strings.TrimSpace(strings.ToLower(title))
	for i, page := range settingsPages {
		if strings.ToLower(page.Title) == title {
			return i
		}
	}
	return -1
}

func pageListItems() string {
	titles := make([]string, 0, len(settingsPages))
	for _, page := range settingsPages {
		if strings.TrimSpace(page.Title) == "" {
			continue
		}
		titles = append(titles, page.Title)
	}
	return strings.Join(titles, "|")
}

func (a *settingsApp) SelectPage(index int, label string) {
	if index < 0 || index >= len(settingsPages) {
		index = pageIndexByTitle(label)
	}
	if index < 0 || index >= len(settingsPages) {
		return
	}
	a.setPage(index)
}

func (a *settingsApp) renderPage() {
	if a.currentPage < 0 || a.currentPage >= len(settingsPages) {
		a.currentPage = 0
	}
	page := settingsPages[a.currentPage]
	a.setText("PageTitle", page.Title)
	a.setText("PageDescription", page.Description)

	host := a.e("SectionsHost")
	if host == nil {
		return
	}
	host.ClearChildren()

	if page.Name == "appearance" {
		host.AddChild(a.buildWallpaperCard())
	}
	for _, section := range page.Sections {
		host.AddChild(a.buildSectionCard(section))
	}
}

func (a *settingsApp) buildSectionCard(section settingSection) *ui.Element {
	card, box := newSectionCard(section.Title, section.Description)
	for _, field := range section.Fields {
		fieldCopy := field
		box.AddChild(a.buildFieldRow(fieldCopy))
	}
	return card
}

func newSectionCard(titleText, description string) (*ui.Element, *ui.Element) {
	card := ui.NewElement("Card")
	card.SetAttribute("padding", "16")
	card.SetAttribute("expand", false)

	box := ui.NewElement("VBox")
	box.SetAttribute("direction", "column")
	box.SetAttribute("spacing", 8)
	box.SetAttribute("expand", false)

	title := ui.NewElement("Subheading")
	title.SetAttribute("text", titleText)
	box.AddChild(title)

	if strings.TrimSpace(description) != "" {
		desc := ui.NewElement("Paragraph")
		desc.SetAttribute("text", description)
		box.AddChild(desc)
	}

	card.AddChild(box)
	return card, box
}

func (a *settingsApp) buildFieldRow(field settingField) *ui.Element {
	row := ui.NewElement("HBox")
	row.SetAttribute("direction", "row")
	row.SetAttribute("spacing", 12)
	row.SetAttribute("expand", false)

	meta := ui.NewElement("VBox")
	meta.SetAttribute("direction", "column")
	meta.SetAttribute("spacing", 2)
	meta.SetAttribute("expand", true)
	meta.SetAttribute("shrink", true)
	meta.SetAttribute("shrinkMinWidth", 220)

	label := ui.NewElement("Label")
	label.SetAttribute("text", field.Label)
	label.SetAttribute("textRole", "subheading")
	meta.AddChild(label)

	if strings.TrimSpace(field.Hint) != "" {
		hint := ui.NewElement("Paragraph")
		hint.SetAttribute("text", field.Hint)
		hint.SetAttribute("maxWidth", 620)
		meta.AddChild(hint)
	}

	controlWrap := ui.NewElement("VBox")
	controlWrap.SetAttribute("direction", "column")
	controlWrap.SetAttribute("spacing", 0)
	controlWrap.SetAttribute("expand", false)
	controlWrap.SetAttribute("width", settingValueColWidth)
	controlWrap.SetAttribute("shrink", true)
	controlWrap.SetAttribute("shrinkMinWidth", settingValueColMin)

	switch field.Kind {
	case fieldToggle:
		controlWrap.AddChild(a.buildToggleField(field))
	case fieldChoice:
		controlWrap.AddChild(a.buildChoiceField(field))
	default:
		controlWrap.AddChild(a.buildTextField(field))
	}

	row.AddChild(meta)
	row.AddChild(controlWrap)
	return row
}

func (a *settingsApp) buildToggleField(field settingField) *ui.Element {
	align := ui.NewElement("HBox")
	align.SetAttribute("direction", "row")
	align.SetAttribute("spacing", 0)
	align.SetAttribute("expand", false)

	spacer := ui.NewElement("Spacer")
	spacer.SetAttribute("expand", true)
	align.AddChild(spacer)

	toggle := ui.NewElement("Toggle")
	toggle.SetAttribute("id", field.ID)
	toggle.SetAttribute("checked", parseBool(a.values[field.Key], parseBool(field.Default, false)))
	toggle.SetAttribute("text", "")
	toggle.SetAttribute("expand", false)
	if field.ReadOnly {
		toggle.SetAttribute("toggleable", false)
	}

	fieldCopy := field
	toggle.BindSignal("changed", func(checked bool) {
		if fieldCopy.ReadOnly {
			return
		}
		a.values[fieldCopy.Key] = boolString(checked)
		a.refreshPendingStatus()
	})

	align.AddChild(toggle)
	return align
}

func (a *settingsApp) buildTextField(field settingField) *ui.Element {
	input := ui.NewElement("TextInput")
	input.SetAttribute("id", field.ID)
	input.SetAttribute("text", a.values[field.Key])
	if field.Placeholder != "" {
		input.SetAttribute("placeholder", field.Placeholder)
	}
	if field.ReadOnly {
		input.SetAttribute("readOnly", true)
	}

	fieldCopy := field
	input.BindSignal("changed", func(text string) {
		if fieldCopy.ReadOnly {
			return
		}
		a.values[fieldCopy.Key] = text
		a.refreshPendingStatus()
	})
	input.BindSignal("submitted", func(text string) {
		if fieldCopy.ReadOnly {
			return
		}
		a.values[fieldCopy.Key] = text
		a.refreshPendingStatus()
	})
	return input
}

func (a *settingsApp) buildChoiceField(field settingField) *ui.Element {
	flow := ui.NewElement("Flow")
	flow.SetAttribute("layout", "flow")
	flow.SetAttribute("spacing", 8)
	flow.SetAttribute("colSpacing", 8)
	flow.SetAttribute("rowSpacing", 8)
	flow.SetAttribute("expand", false)

	current := strings.TrimSpace(a.values[field.Key])
	if current == "" {
		current = strings.TrimSpace(field.Default)
	}
	chips := make([]*ui.Element, 0, len(field.Choices))
	labels := make([]string, 0, len(field.Choices))
	values := make([]string, 0, len(field.Choices))
	updateChoiceLabels := func(selected string) {
		for i, chip := range chips {
			label := labels[i]
			if values[i] == selected {
				label += " (selected)"
			}
			chip.SetAttribute("text", label)
		}
	}
	for _, choice := range field.Choices {
		value := strings.TrimSpace(choice.Value)
		label := strings.TrimSpace(choice.Label)
		if label == "" {
			label = value
		}

		chip := ui.NewElement("Chip")
		chip.SetAttribute("text", label)
		if field.ReadOnly {
			chip.SetAttribute("interactive", false)
		}

		valueCopy := value
		chip.BindSignal("clicked", func() {
			if field.ReadOnly {
				return
			}
			a.values[field.Key] = valueCopy
			a.refreshPendingStatus()
			updateChoiceLabels(valueCopy)
		})

		chips = append(chips, chip)
		labels = append(labels, label)
		values = append(values, value)
		flow.AddChild(chip)
	}
	updateChoiceLabels(current)
	return flow
}

func (a *settingsApp) buildWallpaperCard() *ui.Element {
	card, box := newSectionCard(
		"Wallpaper",
		"Pick a background image from known folders or enter a custom path.",
	)
	box.SetAttribute("spacing", 12)

	selectedPath := strings.TrimSpace(a.values[keyBackgroundSource])
	if selectedPath == "" {
		selectedPath = defaultWallpaper
	}

	previewTitle := ui.NewElement("Subheading")
	previewTitle.SetAttribute("text", "Preview")
	box.AddChild(previewTitle)

	previewWrap := ui.NewElement("VBox")
	previewWrap.SetAttribute("direction", "column")
	previewWrap.SetAttribute("spacing", 6)
	previewWrap.SetAttribute("padding", "8")

	preview := ui.NewElement("Image")
	preview.SetAttribute("src", selectedPath)
	preview.SetAttribute("scaleMode", "cover")
	preview.SetAttribute("expand", true)
	preview.SetAttribute("minHeight", 180)
	preview.SetAttribute("maxHeight", 180)
	preview.SetAttribute("interactive", false)
	previewWrap.AddChild(preview)

	previewPath := ui.NewElement("Label")
	previewPath.SetAttribute("text", selectedPath)
	previewPath.SetAttribute("textAlign", "left")
	previewPath.SetAttribute("clipText", true)
	previewPath.SetAttribute("minHeight", 20)
	previewWrap.AddChild(previewPath)
	box.AddChild(previewWrap)

	pathTitle := ui.NewElement("Subheading")
	pathTitle.SetAttribute("text", "Image Path")
	box.AddChild(pathTitle)

	pathInput := ui.NewElement("TextInput")
	pathInput.SetAttribute("text", selectedPath)
	pathInput.SetAttribute("placeholder", defaultWallpaper)
	box.AddChild(pathInput)

	galleryTitle := ui.NewElement("Subheading")
	galleryTitle.SetAttribute("text", "Gallery")
	box.AddChild(galleryTitle)

	wallpapers := discoverWallpaperOptions(selectedPath)
	gallery := ui.NewElement("Flow")
	gallery.SetAttribute("layout", "flow")
	gallery.SetAttribute("spacing", 10)
	gallery.SetAttribute("colSpacing", 10)
	gallery.SetAttribute("rowSpacing", 10)
	gallery.SetAttribute("padding", "4")
	gallery.SetAttribute("overflow", "auto")
	gallery.SetAttribute("minHeight", 246)
	gallery.SetAttribute("maxHeight", 246)
	gallery.SetAttribute("expand", false)

	tiles := make([]*ui.Element, 0, len(wallpapers))
	tileLabels := make([]*ui.Element, 0, len(wallpapers))
	tilePaths := make([]string, 0, len(wallpapers))
	updateWallpaperSelection := func(path string, syncInput bool) {
		path = strings.TrimSpace(path)
		if path == "" {
			path = defaultWallpaper
		}
		a.values[keyBackgroundSource] = path
		a.refreshPendingStatus()
		if syncInput {
			pathInput.SetAttribute("text", path)
		}
		preview.SetAttribute("src", path)
		previewPath.SetAttribute("text", path)
		for i, label := range tileLabels {
			tileName := filepath.Base(tilePaths[i])
			if tilePaths[i] == path {
				label.SetAttribute("text", tileName+" (selected)")
			} else {
				label.SetAttribute("text", tileName)
			}
		}
		for i, tile := range tiles {
			if tilePaths[i] == path {
				tile.SetAttribute("text", "Selected")
			} else {
				tile.SetAttribute("text", "")
			}
		}
	}

	pathInput.BindSignal("changed", func(text string) {
		updateWallpaperSelection(text, false)
	})
	pathInput.BindSignal("submitted", func(text string) {
		updateWallpaperSelection(text, true)
	})

	for _, wallpaper := range wallpapers {
		tile := ui.NewElement("Button")
		tile.SetAttribute("text", "")
		tile.SetAttribute("padding", "6")
		tile.SetAttribute("minWidth", 148)
		tile.SetAttribute("maxWidth", 148)
		tile.SetAttribute("minHeight", 114)
		tile.SetAttribute("maxHeight", 114)

		thumb := ui.NewElement("Image")
		thumb.SetAttribute("src", wallpaper)
		thumb.SetAttribute("scaleMode", "cover")
		thumb.SetAttribute("minWidth", 134)
		thumb.SetAttribute("maxWidth", 134)
		thumb.SetAttribute("minHeight", 82)
		thumb.SetAttribute("maxHeight", 82)
		thumb.SetAttribute("interactive", false)

		name := ui.NewElement("Label")
		name.SetAttribute("text", filepath.Base(wallpaper))
		name.SetAttribute("textAlign", "center")
		name.SetAttribute("clipText", true)
		name.SetAttribute("minWidth", 134)
		name.SetAttribute("maxWidth", 134)
		name.SetAttribute("interactive", false)

		content := ui.NewElement("VBox")
		content.SetAttribute("direction", "column")
		content.SetAttribute("spacing", 6)
		content.SetAttribute("alignment", "center")
		content.SetAttribute("interactive", false)
		content.AddChild(thumb)
		content.AddChild(name)
		tile.AddChild(content)

		pathCopy := wallpaper
		tile.BindSignal("clicked", func() {
			updateWallpaperSelection(pathCopy, true)
		})

		tiles = append(tiles, tile)
		tileLabels = append(tileLabels, name)
		tilePaths = append(tilePaths, wallpaper)
		gallery.AddChild(tile)
	}

	if len(wallpapers) == 0 {
		empty := ui.NewElement("Paragraph")
		empty.SetAttribute("text", "No wallpaper images found in known folders.")
		box.AddChild(empty)
	} else {
		updateWallpaperSelection(selectedPath, false)
		box.AddChild(gallery)
	}

	colorTitle := ui.NewElement("Subheading")
	colorTitle.SetAttribute("text", "Fallback Color")
	box.AddChild(colorTitle)

	colorHint := ui.NewElement("Paragraph")
	colorHint.SetAttribute("text", "Fallback background color (used behind transparent or non-cover images).")
	box.AddChild(colorHint)

	colorFlow := ui.NewElement("Flow")
	colorFlow.SetAttribute("layout", "flow")
	colorFlow.SetAttribute("spacing", 8)
	colorFlow.SetAttribute("colSpacing", 8)
	colorFlow.SetAttribute("rowSpacing", 8)
	colorFlow.SetAttribute("expand", false)

	currentColor := normalizeColor(a.values[keyBackgroundColor])
	colorInput := ui.NewElement("TextInput")
	if currentColor != "" {
		colorInput.SetAttribute("text", currentColor)
	}
	colorInput.SetAttribute("placeholder", "#11161e")

	colorButtons := make([]*ui.Element, 0, len(backgroundColorChoices))
	colorLabels := make([]string, 0, len(backgroundColorChoices))
	colorValues := make([]string, 0, len(backgroundColorChoices))
	refreshColorLabels := func(selected string) {
		for i, chip := range colorButtons {
			label := colorLabels[i]
			if colorValues[i] != "" {
				label += " " + colorValues[i]
			}
			if colorValues[i] == selected {
				label += " (selected)"
			}
			chip.SetAttribute("text", label)
		}
	}
	updateColorSelection := func(value string, syncInput bool) {
		if value != "" {
			value = normalizeColor(value)
		}
		a.values[keyBackgroundColor] = value
		a.refreshPendingStatus()
		if syncInput {
			colorInput.SetAttribute("text", value)
		}
		refreshColorLabels(value)
	}

	for _, preset := range backgroundColorChoices {
		chip := ui.NewElement("Chip")
		chip.SetAttribute("text", preset.Label)

		valueCopy := preset.Value
		chip.BindSignal("clicked", func() {
			updateColorSelection(valueCopy, true)
		})

		colorButtons = append(colorButtons, chip)
		colorLabels = append(colorLabels, preset.Label)
		colorValues = append(colorValues, preset.Value)
		colorFlow.AddChild(chip)
	}

	colorInput.BindSignal("changed", func(text string) {
		typed := strings.TrimSpace(text)
		a.values[keyBackgroundColor] = typed
		a.refreshPendingStatus()
		current := normalizeColor(typed)
		refreshColorLabels(current)
	})
	colorInput.BindSignal("submitted", func(text string) {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			updateColorSelection("", true)
			return
		}
		normalized := normalizeColor(trimmed)
		if normalized == "" {
			a.setStatus("Use hex color format like #112233")
			return
		}
		updateColorSelection(normalized, true)
	})

	refreshColorLabels(currentColor)
	box.AddChild(colorFlow)
	box.AddChild(colorInput)
	return card
}

func discoverWallpaperOptions(selected string) []string {
	dirs := make([]string, 0, len(wallpaperDirs)+3)
	dirs = append(dirs, wallpaperDirs...)
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		dirs = append(dirs,
			filepath.Join(home, "Pictures"),
			filepath.Join(home, "Pictures", "Wallpapers"),
			filepath.Join(home, ".local", "share", "backgrounds"),
		)
	}

	seen := make(map[string]struct{}, 128)
	out := make([]string, 0, 128)
	addPath := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		if !isWallpaperImage(path) {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}

	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			addPath(filepath.Join(dir, entry.Name()))
		}
	}

	selected = strings.TrimSpace(selected)
	if selected != "" {
		addPath(selected)
	}
	if len(out) == 0 {
		addPath(defaultWallpaper)
	}

	sort.Slice(out, func(i, j int) bool {
		li := strings.ToLower(filepath.Base(out[i]))
		lj := strings.ToLower(filepath.Base(out[j]))
		if li == lj {
			return out[i] < out[j]
		}
		return li < lj
	})
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}

func isWallpaperImage(path string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".png", ".jpg", ".jpeg", ".bmp", ".gif", ".webp", ".svg":
		return true
	default:
		return false
	}
}

func (a *settingsApp) getSetting(client *settingsapi.Client, key string) (string, error) {
	value, err := client.Get(key)
	if err == nil {
		return value, nil
	}
	for _, legacy := range legacyKeysForCanonical(key) {
		if legacyValue, legacyErr := client.Get(legacy); legacyErr == nil {
			return legacyValue, nil
		}
	}
	return "", err
}

func (a *settingsApp) loadValues() error {
	client, err := a.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	for _, field := range allSettingFields {
		value, err := a.getSetting(client, field.Key)
		if err != nil {
			value = field.Default
		}
		value = normalizeSettingValue(field, value)
		a.values[field.Key] = value
		a.persisted[field.Key] = value
	}
	a.refreshPendingStatus()
	return nil
}

func (a *settingsApp) refreshPendingStatus() {
	pending := a.pendingCount()
	switch {
	case pending <= 0:
		a.setStatus("No pending changes")
	case pending == 1:
		a.setStatus("1 pending change")
	default:
		a.setStatus(fmt.Sprintf("%d pending changes", pending))
	}
}

func (a *settingsApp) pendingCount() int {
	count := 0
	for _, field := range allSettingFields {
		key := field.Key
		next := normalizeSettingValue(field, a.values[key])
		prev := normalizeSettingValue(field, a.persisted[key])
		if next != prev {
			count++
		}
	}
	return count
}

func (a *settingsApp) Apply() {
	client, err := a.connect()
	if err != nil {
		a.setStatus(err.Error())
		return
	}
	defer client.Close()

	changes := 0
	for _, field := range allSettingFields {
		key := field.Key
		next := normalizeSettingValue(field, a.values[key])
		prev := normalizeSettingValue(field, a.persisted[key])
		a.values[key] = next
		if next == prev {
			continue
		}
		if err := client.Set(key, next); err != nil {
			a.setStatus(fmt.Sprintf("failed to set %s: %v", key, err))
			return
		}
		a.persisted[key] = next
		changes++
	}

	if changes == 0 {
		a.setStatus("No changes to apply")
		return
	}
	if changes == 1 {
		a.setStatus("Applied 1 setting")
		return
	}
	a.setStatus(fmt.Sprintf("Applied %d settings", changes))
}

func (a *settingsApp) Reset() {
	if err := a.loadValues(); err != nil {
		a.setStatus(err.Error())
		return
	}
	a.renderPage()
	a.setStatus("Changes discarded")
}

func parseBool(value string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	case "0", "false", "no", "off", "disabled":
		return false
	default:
		return def
	}
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func normalizeSettingValue(field settingField, value string) string {
	if field.Kind == fieldToggle {
		return boolString(parseBool(value, parseBool(field.Default, false)))
	}

	value = strings.TrimSpace(value)
	if value == "" {
		value = field.Default
	}

	switch field.Key {
	case keyBackgroundScale:
		return normalizeBackgroundScale(value)
	case keyDockPosition:
		return normalizeDockPosition(value)
	case keyBackgroundColor:
		return normalizeColor(value)
	default:
		return value
	}
}

func normalizeBackgroundScale(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "contain":
		return "contain"
	case "cover":
		return "cover"
	case "stretch":
		return "stretch"
	case "none":
		return "none"
	default:
		return defaultBackgroundScale
	}
}

func normalizeDockPosition(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "top":
		return "top"
	case "bottom":
		return "bottom"
	default:
		return defaultDockPos
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
		canonical, err := store.Set(key, value)
		if err != nil {
			return nil, err
		}

		emitted := make(map[string]struct{}, 4)
		emitChanged := func(changedKey string) {
			changedKey = strings.TrimSpace(changedKey)
			if changedKey == "" {
				return
			}
			if _, exists := emitted[changedKey]; exists {
				return
			}
			emitted[changedKey] = struct{}{}
			_ = svc.Broadcast(settingsapi.EventChanged, settingsapi.EncodeChangedEvent(changedKey, value))
		}

		emitChanged(canonical)
		emitChanged(key)
		for _, legacy := range legacyKeysForCanonical(canonical) {
			emitChanged(legacy)
		}
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
	sessionMode := isSessionMode()
	if sessionMode {
		// In session mode, daemon lifetime is controlled by session manager.
		signal.Ignore(syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
		defer signal.Reset(syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
	}
	sigSet := []os.Signal{syscall.SIGTERM}
	if !sessionMode {
		sigSet = append(sigSet, syscall.SIGINT)
	}
	signal.Notify(sigCh, sigSet...)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		_ = svc.Close()
	}()

	log.Info("settings daemon running at %s", settingsapi.ServiceName)
	svc.Run()
	return nil
}

func isSessionMode() bool {
	if os.Getenv("AVYOS_SESSION_MODE") == "1" {
		return true
	}
	return os.Getenv("AVYOS_SESSION_ID") != ""
}

func hasCLIAction() bool {
	return flagList || strings.TrimSpace(flagGet) != "" || strings.TrimSpace(flagSet) != ""
}

func runCLI() error {
	actions := 0
	if flagList {
		actions++
	}
	if strings.TrimSpace(flagGet) != "" {
		actions++
	}
	if strings.TrimSpace(flagSet) != "" {
		actions++
	}
	if actions == 0 {
		return nil
	}
	if actions > 1 {
		return fmt.Errorf("choose only one of --list, --get, or --set")
	}
	if strings.TrimSpace(flagPrefix) != "" && !flagList {
		return fmt.Errorf("--prefix can only be used with --list")
	}

	if flagList {
		return runCLIList(flagPrefix)
	}
	if strings.TrimSpace(flagGet) != "" {
		return runCLIGet(flagGet)
	}
	return runCLISet(flagSet, flag.Args())
}

func runCLIList(prefix string) error {
	prefix = strings.TrimSpace(prefix)
	client, err := settingsapi.Connect()
	if err == nil {
		defer client.Close()
		items, err := client.List(prefix)
		if err != nil {
			return err
		}
		for _, item := range items {
			fmt.Printf("%s=%s\n", item.Key, item.Value)
		}
		return nil
	}

	store, storeErr := newSettingsStore(userSettingsPath())
	if storeErr != nil {
		return fmt.Errorf("settings daemon unavailable: %v (fallback store failed: %v)", err, storeErr)
	}
	for _, item := range store.List(prefix) {
		fmt.Printf("%s=%s\n", item.Key, item.Value)
	}
	return nil
}

func runCLIGet(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("--get requires a key")
	}
	client, err := settingsapi.Connect()
	if err == nil {
		defer client.Close()
		value, err := client.Get(key)
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	}

	store, storeErr := newSettingsStore(userSettingsPath())
	if storeErr != nil {
		return fmt.Errorf("settings daemon unavailable: %v (fallback store failed: %v)", err, storeErr)
	}
	value, err := store.Get(key)
	if err != nil {
		return err
	}
	fmt.Println(value)
	return nil
}

func runCLISet(spec string, args []string) error {
	key, value, err := parseSetSpec(spec, args)
	if err != nil {
		return err
	}

	client, err := settingsapi.Connect()
	if err == nil {
		defer client.Close()
		if err := client.Set(key, value); err != nil {
			return err
		}
		fmt.Printf("%s=%s\n", canonicalSettingKey(key), value)
		return nil
	}

	store, storeErr := newSettingsStore(userSettingsPath())
	if storeErr != nil {
		return fmt.Errorf("settings daemon unavailable: %v (fallback store failed: %v)", err, storeErr)
	}
	canonical, err := store.Set(key, value)
	if err != nil {
		return err
	}
	fmt.Printf("%s=%s\n", canonical, value)
	return nil
}

func parseSetSpec(spec string, args []string) (key, value string, err error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", fmt.Errorf("--set requires key=value")
	}
	if idx := strings.IndexByte(spec, '='); idx >= 0 {
		key = strings.TrimSpace(spec[:idx])
		value = spec[idx+1:]
		if key == "" {
			return "", "", fmt.Errorf("invalid --set key")
		}
		return key, value, nil
	}

	if len(args) == 0 {
		return "", "", fmt.Errorf("--set requires key=value")
	}
	key = spec
	value = strings.Join(args, " ")
	return strings.TrimSpace(key), value, nil
}

func runUI() error {
	app := &settingsApp{currentPage: 0}
	app.initializeValues()
	app.SetOptions(gapp.Options{Title: "Settings Manager"})
	if err := app.LoadString(settingsManagerUI, app); err != nil {
		return err
	}

	if list := app.e("PageList"); list != nil {
		list.SetAttribute("items", pageListItems())
		list.SetAttribute("selected", app.currentPage)
		selected := list.AttrInt("selected", 0)
		if selected >= 0 && selected < len(settingsPages) {
			app.currentPage = selected
		}
	}

	connected := false
	if err := app.loadValues(); err != nil {
		app.setStatus(err.Error())
	} else {
		connected = true
	}
	app.setPage(app.currentPage)
	if connected {
		app.refreshPendingStatus()
	}
	return app.Run()
}

func main() {
	flag.Parse()
	if flagDaemon && hasCLIAction() {
		fmt.Fprintln(os.Stderr, "settingsmanager: --daemon cannot be combined with --list/--get/--set")
		os.Exit(2)
	}

	if flagDaemon {
		if err := runDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "settingsmanager: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if hasCLIAction() {
		if err := runCLI(); err != nil {
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
