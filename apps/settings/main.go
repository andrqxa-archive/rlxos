package main

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/binary"
	"flag"
	"fmt"
	"net"
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

	distroapi "avyos.dev/api/distro"
	serviceapi "avyos.dev/api/service"
	settingsapi "avyos.dev/api/settings"
	ueventapi "avyos.dev/api/uevent"
	"avyos.dev/pkg/fs"
	gapp "avyos.dev/pkg/graphics/app"
	uidecl "avyos.dev/pkg/graphics/widget/decl"
	ui "avyos.dev/pkg/graphics/widget/engine"
	"avyos.dev/pkg/identity"
	"avyos.dev/pkg/ini"
	"avyos.dev/pkg/logger"
	"avyos.dev/pkg/sutra"
)

//go:embed ui/settingsmanager.ui
var settingsManagerUI string

//go:embed ui/pages/*.ui
var settingsPageLayoutsFS embed.FS

var settingsPageLayoutFiles = map[string]string{
	"network":       "ui/pages/network.ui",
	"appearance":    "ui/pages/appearance.ui",
	"display":       "ui/pages/display.ui",
	"sound":         "ui/pages/sound.ui",
	"notifications": "ui/pages/notifications.ui",
	"security":      "ui/pages/security.ui",
	"user":          "ui/pages/user.ui",
	"services":      "ui/pages/services.ui",
	"distro":        "ui/pages/distro.ui",
	"devices":       "ui/pages/devices.ui",
	"configs":       "ui/pages/configs.ui",
	"about":         "ui/pages/about.ui",
}

const (
	defaultWallpaper       = "/avyos/data/backgrounds/default.png"
	defaultBackgroundScale = "cover"
	defaultDockPos         = "bottom"
	defaultBgColor         = ""
	defaultRoundedRadius   = "12"
	defaultBrightness      = "70"

	settingValueColWidth = 420
	settingValueColMin   = 240
)

const (
	keyBackgroundSource      = "/dev/rlxos/background/source"
	keyBackgroundScale       = "/dev/rlxos/background/scale"
	keyBackgroundColor       = "/dev/rlxos/background/color"
	keyDockPosition          = "/dev/rlxos/dock/position"
	keyRoundedCornersEnabled = "/dev/rlxos/ui/rounded_corners/enabled"
	keyRoundedCornersRadius  = "/dev/rlxos/ui/rounded_corners/radius"
	keyDisplayBrightness     = "/dev/rlxos/display/brightness"
	keySystemGoVersion       = "/dev/rlxos/system/about/golang"
)

var (
	legacyKeyToCanonical = map[string]string{
		"background.wallpaper": keyBackgroundSource,
		"background.color":     keyBackgroundColor,
		"dock.position":        keyDockPosition,
		"display.brightness":   keyDisplayBrightness,
		"desktop.rounded":      keyRoundedCornersEnabled,
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
	fieldSlider
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
	Min         float64
	Max         float64
	Step        float64
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

func sliderSetting(id, key, label, hint, def string, min, max, step float64) settingField {
	if max < min {
		max = min
	}
	if step < 0 {
		step = 0
	}
	return settingField{
		ID:      id,
		Key:     key,
		Label:   label,
		Hint:    hint,
		Default: def,
		Kind:    fieldSlider,
		Min:     min,
		Max:     max,
		Step:    step,
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
			Name:        "network",
			Title:       "Network",
			Description: "Wired network status, DNS servers, addresses, and the active default route.",
		},
		{
			Name:        "appearance",
			Title:       "Appearance",
			Description: "Background image controls, rounded corners, and dock placement.",
			Sections: []settingSection{
				{
					Title:       "Desktop Style",
					Description: "General desktop appearance preferences.",
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
						toggleSetting(
							"RoundedCornersEnabled",
							keyRoundedCornersEnabled,
							"Rounded Corners",
							"Enable rounded corners across desktop surfaces.",
							true,
						),
						choiceSetting(
							"RoundedCornersRadius",
							keyRoundedCornersRadius,
							"Corner Radius",
							"Corner radius used when rounded corners are enabled.",
							defaultRoundedRadius,
							settingChoice{Label: "Subtle (8px)", Value: "8"},
							settingChoice{Label: "Balanced (12px)", Value: "12"},
							settingChoice{Label: "Large (16px)", Value: "16"},
							settingChoice{Label: "Extra (20px)", Value: "20"},
						),
					},
				},
			},
		},
		{
			Name:        "display",
			Title:       "Display",
			Description: "Current display information and brightness control.",
			Sections: []settingSection{
				{
					Title:       "Brightness",
					Description: "Adjust preferred display brightness percentage.",
					Fields: []settingField{
						sliderSetting(
							"DisplayBrightness",
							keyDisplayBrightness,
							"Brightness",
							"Preferred screen brightness level.",
							defaultBrightness,
							1,
							100,
							1,
						),
					},
				},
			},
		},
		{
			Name:        "sound",
			Title:       "Sound",
			Description: "Volume, output, and input controls.",
		},
		{
			Name:        "notifications",
			Title:       "Notifications",
			Description: "Banner behavior and per-app notification controls.",
		},
		{
			Name:        "security",
			Title:       "Security",
			Description: "Authentication, policies, and system hardening.",
		},
		{
			Name:        "user",
			Title:       "User",
			Description: "Current user identity and password management.",
		},
		{
			Name:        "services",
			Title:       "Services",
			Description: "Inspect and control running system services.",
		},
		{
			Name:        "distro",
			Title:       "Distro",
			Description: "Manage installed and available distributions.",
		},
		{
			Name:        "devices",
			Title:       "Devices",
			Description: "Manage detected hardware devices from uevent.",
		},
		{
			Name:        "configs",
			Title:       "Configs",
			Description: "List and manage raw settings keys and values.",
		},
		{
			Name:        "about",
			Title:       "About",
			Description: "System build and runtime metadata.",
			Sections: []settingSection{
				{
					Title:       "System Defaults",
					Description: "Metadata persisted in settings for other services.",
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
						readOnlyTextSetting("GoVersion", keySystemGoVersion, "Go Runtime", "Go runtime version.", runtime.Version()),
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

	if cfg, err := ini.ParseFile(fs.Resolve("config:init.conf")); err == nil {
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
		out[keyDisplayBrightness] = brightness
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
		fs.Resolve("process:sys/kernel/osrelease"),
	); kernel != "" {
		out["/dev/rlxos/system/about/kernel"] = kernel
	}
	out["/dev/rlxos/system/about/architecture"] = runtime.GOARCH
	out["/dev/rlxos/system/about/session"] = detectSessionType()
	out[keySystemGoVersion] = runtime.Version()

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
		fs.Resolve("config:resolv.conf"),
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
		fs.Resolve("sysfs:class/graphics/fb0/virtual_size"),
		fs.Resolve("process:sys/class/graphics/fb0/virtual_size"),
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
		fs.Resolve("sysfs:class/backlight"),
		fs.Resolve("process:sys/class/backlight"),
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
		fs.Resolve("sysfs:class/power_supply"),
		fs.Resolve("process:sys/class/power_supply"),
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
		fs.Resolve("config:release.conf"),
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
	gapp.App
	currentPage int
	values      map[string]string
	persisted   map[string]string

	serviceTarget      string
	distroInstallURL   string
	deviceTriggerScope string
	configFilterPrefix string
	configEditKey      string
	configEditValue    string

	passwordCurrent string
	passwordNext    string
	passwordConfirm string
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

func (a *settingsApp) RefreshPage() {
	a.renderPage()
	a.setStatus("Page refreshed")
}

func (a *settingsApp) loadPageLayout(pageName string) (*ui.Element, *ui.Element, error) {
	path := strings.TrimSpace(settingsPageLayoutFiles[pageName])
	if path == "" {
		return nil, nil, nil
	}
	data, err := settingsPageLayoutsFS.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	root, err := uidecl.LoadString(string(data), a)
	if err != nil {
		return nil, nil, err
	}
	return root, uidecl.FindElement(root, "PageSectionsHost"), nil
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

	sectionsHost := host
	if pageRoot, dynamicHost, err := a.loadPageLayout(page.Name); err != nil {
		log.Warn("settings: failed to load page layout %q: %v", page.Name, err)
	} else if pageRoot != nil {
		host.AddChild(pageRoot)
		if dynamicHost != nil {
			sectionsHost = dynamicHost
		} else {
			sectionsHost = nil
		}
	}
	if sectionsHost == nil {
		return
	}
	sectionsHost.ClearChildren()

	switch page.Name {
	case "network":
		sectionsHost.AddChild(a.buildNetworkWiredCard())
	case "appearance":
		sectionsHost.AddChild(a.buildWallpaperCard())
		for _, section := range page.Sections {
			sectionsHost.AddChild(a.buildSectionCard(section))
		}
	case "display":
		sectionsHost.AddChild(a.buildDisplayInfoCard())
		for _, section := range page.Sections {
			sectionsHost.AddChild(a.buildSectionCard(section))
		}
	case "sound":
		sectionsHost.AddChild(a.buildNotImplementedCard(
			"Sound",
			"Output and input settings are not yet implemented.",
		))
	case "notifications":
		sectionsHost.AddChild(a.buildNotImplementedCard(
			"Notifications",
			"Notification settings are not yet implemented.",
		))
	case "security":
		sectionsHost.AddChild(a.buildNotImplementedCard(
			"Security",
			"Security settings are not yet implemented.",
		))
	case "user":
		sectionsHost.AddChild(a.buildUserInfoCard())
		sectionsHost.AddChild(a.buildUserPasswordCard())
	case "services":
		sectionsHost.AddChild(a.buildServicesListCard())
		sectionsHost.AddChild(a.buildServicesActionCard())
	case "distro":
		sectionsHost.AddChild(a.buildDistroListCard())
		sectionsHost.AddChild(a.buildDistroManageCard())
	case "devices":
		sectionsHost.AddChild(a.buildDevicesListCard())
		sectionsHost.AddChild(a.buildDevicesManageCard())
	case "configs":
		sectionsHost.AddChild(a.buildConfigsListCard())
		sectionsHost.AddChild(a.buildConfigsManageCard())
	case "about":
		sectionsHost.AddChild(a.buildAboutSystemCard())
		sectionsHost.AddChild(a.buildAboutUpdatesCard())
	default:
		for _, section := range page.Sections {
			sectionsHost.AddChild(a.buildSectionCard(section))
		}
	}
}

func newSettingsElement(tag string) *ui.Element {
	e := ui.NewElement(tag)
	applySettingsWidgetDefaults(e, tag)
	return e
}

func applySettingsWidgetDefaults(e *ui.Element, tag string) {
	switch tag {
	case "HBox":
		e.SetAttribute("direction", "row")
		e.SetAttribute("spacing", 0)
		e.SetAttribute("padding", "0")
		e.SetAttribute("alignment", "stretch")
	case "VBox":
		e.SetAttribute("direction", "column")
		e.SetAttribute("spacing", 0)
		e.SetAttribute("padding", "0")
		e.SetAttribute("alignment", "stretch")
	case "Flow":
		e.SetAttribute("layout", "flow")
		e.SetAttribute("spacing", 12)
		e.SetAttribute("colSpacing", 12)
		e.SetAttribute("rowSpacing", 12)
	case "Spacer":
		e.SetAttribute("expand", true)
	case "Label", "Paragraph":
		e.SetAttribute("textColor", "theme.color.text.primary")
		e.SetAttribute("textAlign", "left")
		e.SetAttribute("textRole", "paragraph")
	case "Subheading":
		e.SetAttribute("textColor", "theme.color.text.primary")
		e.SetAttribute("textAlign", "left")
		e.SetAttribute("textRole", "subheading")
	case "StatusBadge":
		e.SetAttribute("expand", false)
		e.SetAttribute("minHeight", 28)
		e.SetAttribute("padding", "6 12")
		e.SetAttribute("borderRadius", 999)
		e.SetAttribute("background", "theme.color.accent.subtle")
		e.SetAttribute("borderColor", "theme.color.stroke.hairline")
		e.SetAttribute("textColor", "theme.color.accent")
		e.SetAttribute("textRole", "subheading")
		e.SetAttribute("textAlign", "center")
	case "Card":
		e.SetAttribute("background", "theme.color.surface.card")
		e.SetAttribute("gradientTop", "transparent")
		e.SetAttribute("gradientBottom", "transparent")
		e.SetAttribute("borderColor", "theme.color.stroke.hairline")
		e.SetAttribute("borderRadius", 20)
		e.SetAttribute("padding", "18")
		e.SetAttribute("shadow", true)
		e.SetAttribute("shadowColor", "#101A2B17")
		e.SetAttribute("shadowOffsetY", 8)
		e.SetAttribute("shadowSpread", 24)
	case "TextInput":
		e.SetAttribute("editable", true)
		e.SetAttribute("focusable", true)
		e.SetAttribute("minHeight", 40)
		e.SetAttribute("background", "theme.color.surface.card")
		e.SetAttribute("gradientTop", "transparent")
		e.SetAttribute("gradientBottom", "transparent")
		e.SetAttribute("hoverBackground", "theme.color.control.hover")
		e.SetAttribute("textColor", "theme.color.text.primary")
		e.SetAttribute("placeholderColor", "theme.color.text.muted")
		e.SetAttribute("borderColor", "theme.color.stroke.hairline")
		e.SetAttribute("focusedBorderColor", "theme.color.stroke.hairline")
		e.SetAttribute("focusRing", true)
		e.SetAttribute("focusRingColor", "theme.color.stroke.focus")
		e.SetAttribute("focusRingWidth", 3)
		e.SetAttribute("focusRingOffset", 1)
		e.SetAttribute("borderRadius", 14)
		e.SetAttribute("padding", "10 12")
		e.SetAttribute("shadow", false)
	case "Toggle":
		e.SetAttribute("toggleable", true)
		e.SetAttribute("focusable", true)
		e.SetAttribute("minHeight", 40)
		e.SetAttribute("background", "transparent")
		e.SetAttribute("onTrackColor", "theme.color.accent")
		e.SetAttribute("offTrackColor", "theme.color.surface.card")
		e.SetAttribute("thumbColor", "theme.color.surface.card")
		e.SetAttribute("borderColor", "transparent")
		e.SetAttribute("focusedBorderColor", "transparent")
		e.SetAttribute("trackBorderColor", "theme.color.stroke.hairline")
		e.SetAttribute("focusedTrackBorderColor", "theme.color.stroke.focus")
		e.SetAttribute("textColor", "theme.color.text.primary")
		e.SetAttribute("shadow", true)
		e.SetAttribute("shadowColor", "#101A2B17")
		e.SetAttribute("shadowOffsetY", 2)
		e.SetAttribute("shadowSpread", 6)
	case "Slider":
		e.SetAttribute("slidable", true)
		e.SetAttribute("focusable", true)
		e.SetAttribute("borderColor", "theme.color.stroke.hairline")
		e.SetAttribute("focusedBorderColor", "theme.color.stroke.hairline")
		e.SetAttribute("focusRing", true)
		e.SetAttribute("focusRingColor", "theme.color.stroke.focus")
		e.SetAttribute("focusRingWidth", 2)
		e.SetAttribute("focusRingOffset", 1)
		e.SetAttribute("trackColor", "theme.color.control.fill")
		e.SetAttribute("thumbColor", "theme.color.accent")
		e.SetAttribute("thumbHoverColor", "theme.color.accent.hover")
		e.SetAttribute("thumbActiveColor", "theme.primaryactive")
		e.SetAttribute("shadow", true)
		e.SetAttribute("shadowColor", "#101A2B17")
		e.SetAttribute("shadowOffsetY", 2)
		e.SetAttribute("shadowSpread", 4)
	case "Chip":
		e.SetAttribute("interactive", true)
		e.SetAttribute("focusable", true)
		e.SetAttribute("expand", false)
		e.SetAttribute("minHeight", 30)
		e.SetAttribute("padding", "6 12")
		e.SetAttribute("borderRadius", 999)
		e.SetAttribute("background", "theme.color.surface.glass")
		e.SetAttribute("hoverBackground", "theme.color.control.hover")
		e.SetAttribute("pressedBackground", "theme.color.control.pressed")
		e.SetAttribute("borderColor", "theme.color.stroke.hairline")
		e.SetAttribute("focusRing", true)
		e.SetAttribute("focusRingColor", "theme.color.stroke.focus")
		e.SetAttribute("focusRingWidth", 3)
		e.SetAttribute("focusRingOffset", 1)
		e.SetAttribute("textColor", "theme.color.text.primary")
	case "Image":
		e.SetAttribute("scaleMode", "contain")
		e.SetAttribute("background", "transparent")
		e.SetAttribute("intrinsicSize", false)
		e.SetAttribute("expand", false)
	case "Button":
		e.SetAttribute("interactive", true)
		e.SetAttribute("focusable", true)
		e.SetAttribute("background", "theme.color.surface.glass")
		e.SetAttribute("gradientTop", "transparent")
		e.SetAttribute("gradientBottom", "transparent")
		e.SetAttribute("hoverBackground", "theme.color.control.hover")
		e.SetAttribute("pressedBackground", "theme.color.control.pressed")
		e.SetAttribute("borderColor", "theme.color.stroke.hairline")
		e.SetAttribute("focusedBorderColor", "theme.color.stroke.hairline")
		e.SetAttribute("focusRing", true)
		e.SetAttribute("focusRingColor", "theme.color.stroke.focus")
		e.SetAttribute("focusRingWidth", 3)
		e.SetAttribute("focusRingOffset", 1)
		e.SetAttribute("borderRadius", 14)
		e.SetAttribute("minHeight", 40)
		e.SetAttribute("padding", "10 16")
		e.SetAttribute("textColor", "theme.color.text.primary")
		e.SetAttribute("textAlign", "center")
		e.SetAttribute("shadow", true)
		e.SetAttribute("shadowColor", "#101A2B1A")
		e.SetAttribute("shadowOffsetY", 2)
		e.SetAttribute("shadowSpread", 8)
	case "PrimaryButton":
		e.SetAttribute("interactive", true)
		e.SetAttribute("focusable", true)
		e.SetAttribute("gradientTop", "theme.color.accent")
		e.SetAttribute("gradientBottom", "theme.color.accent.alt")
		e.SetAttribute("hoverBackground", "theme.color.accent.hover")
		e.SetAttribute("pressedBackground", "theme.primaryactive")
		e.SetAttribute("borderColor", "theme.color.accent")
		e.SetAttribute("textColor", "#FFFFFF")
		e.SetAttribute("borderRadius", 14)
		e.SetAttribute("minHeight", 40)
		e.SetAttribute("padding", "10 16")
		e.SetAttribute("textAlign", "center")
		e.SetAttribute("shadow", true)
		e.SetAttribute("shadowColor", "#0D63F34D")
		e.SetAttribute("shadowOffsetY", 4)
		e.SetAttribute("shadowSpread", 12)
	case "DangerButton":
		e.SetAttribute("interactive", true)
		e.SetAttribute("focusable", true)
		e.SetAttribute("minHeight", 40)
		e.SetAttribute("padding", "10 16")
		e.SetAttribute("background", "theme.color.semantic.danger")
		e.SetAttribute("hoverBackground", "theme.color.semantic.danger")
		e.SetAttribute("pressedBackground", "theme.color.semantic.danger")
		e.SetAttribute("borderColor", "theme.color.semantic.danger")
		e.SetAttribute("focusedBorderColor", "theme.color.stroke.focus")
		e.SetAttribute("borderRadius", 14)
		e.SetAttribute("textColor", "#FFFFFF")
		e.SetAttribute("shadow", true)
		e.SetAttribute("shadowColor", "#101A2B22")
		e.SetAttribute("shadowOffsetY", 2)
		e.SetAttribute("shadowSpread", 8)
	case "TableView":
		e.SetAttribute("editable", true)
		e.SetAttribute("multiline", true)
		e.SetAttribute("focusable", true)
		e.SetAttribute("readOnly", true)
		e.SetAttribute("lineNumbers", false)
		e.SetAttribute("table", true)
		e.SetAttribute("background", "theme.color.surface.card")
		e.SetAttribute("gradientTop", "transparent")
		e.SetAttribute("gradientBottom", "transparent")
		e.SetAttribute("textColor", "theme.color.text.secondary")
		e.SetAttribute("borderColor", "theme.color.stroke.hairline")
		e.SetAttribute("focusedBorderColor", "theme.color.stroke.hairline")
		e.SetAttribute("focusRing", true)
		e.SetAttribute("focusRingColor", "theme.color.stroke.focus")
		e.SetAttribute("focusRingWidth", 3)
		e.SetAttribute("focusRingOffset", 1)
		e.SetAttribute("headerBackground", "theme.color.surface.glass")
		e.SetAttribute("dividerColor", "theme.color.stroke.divider")
		e.SetAttribute("headerTextColor", "theme.color.text.primary")
		e.SetAttribute("borderRadius", 14)
		e.SetAttribute("padding", "10 12")
		e.SetAttribute("shadow", true)
		e.SetAttribute("shadowColor", "#101A2B17")
		e.SetAttribute("shadowOffsetY", 2)
		e.SetAttribute("shadowSpread", 8)
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
	card := newSettingsElement("Card")
	card.SetAttribute("padding", "16")
	card.SetAttribute("expand", false)

	box := newSettingsElement("VBox")
	box.SetAttribute("direction", "column")
	box.SetAttribute("spacing", 8)
	box.SetAttribute("expand", false)

	title := newSettingsElement("Subheading")
	title.SetAttribute("text", titleText)
	box.AddChild(title)

	if strings.TrimSpace(description) != "" {
		desc := newSettingsElement("Paragraph")
		desc.SetAttribute("text", description)
		box.AddChild(desc)
	}

	card.AddChild(box)
	return card, box
}

func (a *settingsApp) buildFieldRow(field settingField) *ui.Element {
	row := newSettingsElement("HBox")
	row.SetAttribute("direction", "row")
	row.SetAttribute("spacing", 12)
	row.SetAttribute("expand", false)

	meta := newSettingsElement("VBox")
	meta.SetAttribute("direction", "column")
	meta.SetAttribute("spacing", 2)
	meta.SetAttribute("expand", true)
	meta.SetAttribute("shrink", true)
	meta.SetAttribute("shrinkMinWidth", 220)

	label := newSettingsElement("Label")
	label.SetAttribute("text", field.Label)
	label.SetAttribute("textRole", "subheading")
	meta.AddChild(label)

	if strings.TrimSpace(field.Hint) != "" {
		hint := newSettingsElement("Paragraph")
		hint.SetAttribute("text", field.Hint)
		hint.SetAttribute("maxWidth", 620)
		meta.AddChild(hint)
	}

	controlWrap := newSettingsElement("VBox")
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
	case fieldSlider:
		controlWrap.AddChild(a.buildSliderField(field))
	default:
		controlWrap.AddChild(a.buildTextField(field))
	}

	row.AddChild(meta)
	row.AddChild(controlWrap)
	return row
}

func (a *settingsApp) buildToggleField(field settingField) *ui.Element {
	align := newSettingsElement("HBox")
	align.SetAttribute("direction", "row")
	align.SetAttribute("spacing", 0)
	align.SetAttribute("expand", false)

	spacer := newSettingsElement("Spacer")
	spacer.SetAttribute("expand", true)
	align.AddChild(spacer)

	toggle := newSettingsElement("Toggle")
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
	input := newSettingsElement("TextInput")
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
	flow := newSettingsElement("Flow")
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

		chip := newSettingsElement("Chip")
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

func (a *settingsApp) buildSliderField(field settingField) *ui.Element {
	container := newSettingsElement("VBox")
	container.SetAttribute("direction", "column")
	container.SetAttribute("spacing", 6)
	container.SetAttribute("expand", false)

	current, err := strconv.ParseFloat(strings.TrimSpace(a.values[field.Key]), 64)
	if err != nil {
		current, err = strconv.ParseFloat(strings.TrimSpace(field.Default), 64)
		if err != nil {
			current = field.Min
		}
	}
	if current < field.Min {
		current = field.Min
	}
	if current > field.Max {
		current = field.Max
	}

	slider := newSettingsElement("Slider")
	slider.SetAttribute("id", field.ID)
	slider.SetAttribute("min", field.Min)
	slider.SetAttribute("max", field.Max)
	slider.SetAttribute("step", field.Step)
	slider.SetAttribute("value", current)
	if field.ReadOnly {
		slider.SetAttribute("slidable", false)
	}

	valueInput := newSettingsElement("TextInput")
	valueInput.SetAttribute("text", strconv.Itoa(int(current+0.5)))
	valueInput.SetAttribute("placeholder", strconv.Itoa(int(field.Min)))
	valueInput.SetAttribute("maxWidth", 96)
	valueInput.SetAttribute("textAlign", "center")
	if field.ReadOnly {
		valueInput.SetAttribute("readOnly", true)
	}

	suffix := newSettingsElement("Label")
	suffix.SetAttribute("text", "%")
	suffix.SetAttribute("textAlign", "left")
	suffix.SetAttribute("minWidth", 24)

	inputRow := newSettingsElement("HBox")
	inputRow.SetAttribute("direction", "row")
	inputRow.SetAttribute("spacing", 6)
	inputRow.SetAttribute("expand", false)
	inputRow.AddChild(valueInput)
	inputRow.AddChild(suffix)

	normalize := func(v float64) float64 {
		if v < field.Min {
			v = field.Min
		}
		if v > field.Max {
			v = field.Max
		}
		if field.Step > 0 {
			steps := (v - field.Min) / field.Step
			rounded := float64(int(steps + 0.5))
			v = field.Min + rounded*field.Step
			if v < field.Min {
				v = field.Min
			}
			if v > field.Max {
				v = field.Max
			}
		}
		return v
	}
	updateValue := func(v float64, syncSlider bool) {
		v = normalize(v)
		text := strconv.Itoa(int(v + 0.5))
		a.values[field.Key] = text
		valueInput.SetAttribute("text", text)
		if syncSlider {
			slider.SetAttribute("value", v)
		}
		a.refreshPendingStatus()
	}

	fieldCopy := field
	slider.BindSignal("changed", func(v float64) {
		if fieldCopy.ReadOnly {
			return
		}
		updateValue(v, false)
	})
	valueInput.BindSignal("changed", func(text string) {
		if fieldCopy.ReadOnly {
			return
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return
		}
		value, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return
		}
		updateValue(value, true)
	})
	valueInput.BindSignal("submitted", func(text string) {
		if fieldCopy.ReadOnly {
			return
		}
		trimmed := strings.TrimSpace(text)
		value, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			valueInput.SetAttribute("text", a.values[fieldCopy.Key])
			return
		}
		updateValue(value, true)
	})

	container.AddChild(slider)
	container.AddChild(inputRow)
	return container
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

	previewTitle := newSettingsElement("Subheading")
	previewTitle.SetAttribute("text", "Preview")
	box.AddChild(previewTitle)

	previewWrap := newSettingsElement("VBox")
	previewWrap.SetAttribute("direction", "column")
	previewWrap.SetAttribute("spacing", 6)
	previewWrap.SetAttribute("padding", "8")

	preview := newSettingsElement("Image")
	preview.SetAttribute("src", selectedPath)
	preview.SetAttribute("scaleMode", "cover")
	preview.SetAttribute("expand", true)
	preview.SetAttribute("minHeight", 180)
	preview.SetAttribute("maxHeight", 180)
	preview.SetAttribute("interactive", false)
	previewWrap.AddChild(preview)

	previewPath := newSettingsElement("Label")
	previewPath.SetAttribute("text", selectedPath)
	previewPath.SetAttribute("textAlign", "left")
	previewPath.SetAttribute("clipText", true)
	previewPath.SetAttribute("minHeight", 20)
	previewWrap.AddChild(previewPath)
	box.AddChild(previewWrap)

	pathTitle := newSettingsElement("Subheading")
	pathTitle.SetAttribute("text", "Image Path")
	box.AddChild(pathTitle)

	pathInput := newSettingsElement("TextInput")
	pathInput.SetAttribute("text", selectedPath)
	pathInput.SetAttribute("placeholder", defaultWallpaper)
	box.AddChild(pathInput)

	galleryTitle := newSettingsElement("Subheading")
	galleryTitle.SetAttribute("text", "Gallery")
	box.AddChild(galleryTitle)

	wallpapers := discoverWallpaperOptions(selectedPath)
	gallery := newSettingsElement("Flow")
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
		tile := newSettingsElement("Button")
		tile.SetAttribute("text", "")
		tile.SetAttribute("padding", "6")
		tile.SetAttribute("minWidth", 148)
		tile.SetAttribute("maxWidth", 148)
		tile.SetAttribute("minHeight", 114)
		tile.SetAttribute("maxHeight", 114)

		thumb := newSettingsElement("Image")
		thumb.SetAttribute("src", wallpaper)
		thumb.SetAttribute("scaleMode", "cover")
		thumb.SetAttribute("minWidth", 134)
		thumb.SetAttribute("maxWidth", 134)
		thumb.SetAttribute("minHeight", 82)
		thumb.SetAttribute("maxHeight", 82)
		thumb.SetAttribute("interactive", false)

		name := newSettingsElement("Label")
		name.SetAttribute("text", filepath.Base(wallpaper))
		name.SetAttribute("textAlign", "center")
		name.SetAttribute("clipText", true)
		name.SetAttribute("minWidth", 134)
		name.SetAttribute("maxWidth", 134)
		name.SetAttribute("interactive", false)

		content := newSettingsElement("VBox")
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
		empty := newSettingsElement("Paragraph")
		empty.SetAttribute("text", "No wallpaper images found in known folders.")
		box.AddChild(empty)
	} else {
		updateWallpaperSelection(selectedPath, false)
		box.AddChild(gallery)
	}

	colorTitle := newSettingsElement("Subheading")
	colorTitle.SetAttribute("text", "Fallback Color")
	box.AddChild(colorTitle)

	colorHint := newSettingsElement("Paragraph")
	colorHint.SetAttribute("text", "Fallback background color (used behind transparent or non-cover images).")
	box.AddChild(colorHint)

	colorFlow := newSettingsElement("Flow")
	colorFlow.SetAttribute("layout", "flow")
	colorFlow.SetAttribute("spacing", 8)
	colorFlow.SetAttribute("colSpacing", 8)
	colorFlow.SetAttribute("rowSpacing", 8)
	colorFlow.SetAttribute("expand", false)

	currentColor := normalizeColor(a.values[keyBackgroundColor])
	colorInput := newSettingsElement("TextInput")
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
		chip := newSettingsElement("Chip")
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

type infoRow struct {
	Label string
	Value string
}

type wiredNetworkSnapshot struct {
	Interface string
	DNS       string
	IP        string
	Route     string
}

func (a *settingsApp) buildNotImplementedCard(title, description string) *ui.Element {
	card, box := newSectionCard(title, description)
	msg := newSettingsElement("Paragraph")
	msg.SetAttribute("text", "This section is planned and currently not yet implemented.")
	box.AddChild(msg)
	return card
}

func (a *settingsApp) buildNetworkWiredCard() *ui.Element {
	card, box := newSectionCard(
		"Wired",
		"Current wired network interface details based on system state.",
	)

	snap := detectWiredNetworkSnapshot()
	addInfoRows(box, []infoRow{
		{Label: "Interface", Value: snap.Interface},
		{Label: "DNS", Value: snap.DNS},
		{Label: "IP", Value: snap.IP},
		{Label: "Route", Value: snap.Route},
	})

	actions := newSettingsElement("HBox")
	actions.SetAttribute("direction", "row")
	actions.SetAttribute("spacing", 8)
	actions.SetAttribute("expand", false)

	refresh := newSettingsElement("Button")
	refresh.SetAttribute("text", "Refresh Network")
	refresh.BindSignal("clicked", func() {
		a.renderPage()
		a.setStatus("Network section refreshed")
	})
	actions.AddChild(refresh)
	box.AddChild(actions)
	return card
}

func detectWiredNetworkSnapshot() wiredNetworkSnapshot {
	snap := wiredNetworkSnapshot{
		Interface: "Unavailable",
		DNS:       fallbackText(detectDNSServers(), "Unavailable"),
		IP:        "Unavailable",
		Route:     "Unavailable",
	}

	iface, ok := detectPrimaryWiredInterface()
	if ok {
		snap.Interface = iface.Name
		snap.IP = fallbackText(interfaceAddressSummary(iface), "No address")
	}

	routeIface, gateway := detectDefaultRoute()
	if gateway != "" {
		route := "default via " + gateway
		if routeIface != "" {
			route += " dev " + routeIface
		}
		snap.Route = route
	}
	return snap
}

func detectPrimaryWiredInterface() (net.Interface, bool) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return net.Interface{}, false
	}

	pick := -1
	for i := range ifaces {
		iface := ifaces[i]
		name := strings.ToLower(strings.TrimSpace(iface.Name))
		if iface.Flags&net.FlagLoopback != 0 || !looksLikeWiredInterface(name) {
			continue
		}
		if pick < 0 {
			pick = i
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if hasInterfaceAddress(iface) {
			return iface, true
		}
		if pick >= 0 && ifaces[pick].Flags&net.FlagUp == 0 {
			pick = i
		}
	}

	if pick < 0 {
		return net.Interface{}, false
	}
	return ifaces[pick], true
}

func looksLikeWiredInterface(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "wl") || strings.HasPrefix(name, "wlan") {
		return false
	}
	switch {
	case strings.HasPrefix(name, "eth"):
		return true
	case strings.HasPrefix(name, "en"):
		return true
	default:
		return false
	}
}

func hasInterfaceAddress(iface net.Interface) bool {
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP == nil {
			continue
		}
		if ipnet.IP.IsGlobalUnicast() {
			return true
		}
	}
	return false
}

func interfaceAddressSummary(iface net.Interface) string {
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	values := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		switch v := addr.(type) {
		case *net.IPNet:
			ip := v.IP
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ones, _ := v.Mask.Size()
			values = append(values, fmt.Sprintf("%s/%d", ip.String(), ones))
		default:
			text := strings.TrimSpace(addr.String())
			if text != "" {
				values = append(values, text)
			}
		}
	}
	sort.Strings(values)
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ", ")
}

func detectDefaultRoute() (ifaceName, gateway string) {
	paths := []string{
		"/proc/net/route",
		fs.Resolve("process:net/route"),
	}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			fields := strings.Fields(line)
			if len(fields) < 3 || strings.EqualFold(fields[0], "Iface") {
				continue
			}
			destination := strings.TrimSpace(fields[1])
			if destination != "00000000" {
				continue
			}
			gw := decodeRouteGateway(fields[2])
			if gw == "" {
				continue
			}
			_ = f.Close()
			return fields[0], gw
		}
		_ = f.Close()
	}
	return "", ""
}

func decodeRouteGateway(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return ""
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(parsed))
	ip := net.IPv4(buf[0], buf[1], buf[2], buf[3])
	if ip == nil || ip.IsUnspecified() {
		return ""
	}
	return ip.String()
}

func (a *settingsApp) buildDisplayInfoCard() *ui.Element {
	card, box := newSectionCard(
		"Current Display",
		"Detected display/session state from live runtime information.",
	)

	resolution := fallbackText(detectFramebufferResolution(), "Unavailable")
	brightness := fallbackText(detectBacklightBrightnessPercent(), "Unavailable")
	if brightness != "Unavailable" {
		brightness += "%"
	}
	preferredBrightness := normalizeDisplayBrightness(a.values[keyDisplayBrightness])

	addInfoRows(box, []infoRow{
		{Label: "Resolution", Value: resolution},
		{Label: "Session", Value: detectSessionType()},
		{Label: "Architecture", Value: runtime.GOARCH},
		{Label: "Brightness (Live)", Value: brightness},
		{Label: "Brightness (Preferred)", Value: preferredBrightness + "%"},
	})
	return card
}

func (a *settingsApp) buildUserInfoCard() *ui.Element {
	card, box := newSectionCard(
		"User Info",
		"Current session identity from pkg/identity.",
	)

	username := a.currentUsername()
	rows := []infoRow{
		{Label: "Username", Value: username},
	}

	if id, err := identity.LookupByName(username); err == nil && id != nil {
		rows = append(rows,
			infoRow{Label: "User ID", Value: strconv.FormatUint(uint64(id.ID), 10)},
			infoRow{Label: "Home", Value: fallbackText(id.Home, "-")},
			infoRow{Label: "Shell", Value: fallbackText(id.Shell, "-")},
		)
		if len(id.Capabilities) > 0 {
			rows = append(rows, infoRow{
				Label: "Capabilities",
				Value: strings.Join(id.Capabilities, ", "),
			})
		}
	} else if err != nil {
		rows = append(rows, infoRow{
			Label: "Identity",
			Value: "Unavailable (" + err.Error() + ")",
		})
	}

	if authType, err := identity.GetAuthType(username); err == nil {
		rows = append(rows, infoRow{Label: "Auth Type", Value: authType})
	}

	addInfoRows(box, rows)
	return card
}

func (a *settingsApp) buildUserPasswordCard() *ui.Element {
	card, box := newSectionCard(
		"Change Password",
		"Update current account password using pkg/identity.",
	)

	currentInput := newSettingsElement("TextInput")
	currentInput.SetAttribute("text", a.passwordCurrent)
	currentInput.SetAttribute("placeholder", "Current password")
	currentInput.SetAttribute("password", true)
	currentInput.BindSignal("changed", func(text string) {
		a.passwordCurrent = text
	})

	newInput := newSettingsElement("TextInput")
	newInput.SetAttribute("text", a.passwordNext)
	newInput.SetAttribute("placeholder", "New password")
	newInput.SetAttribute("password", true)
	newInput.BindSignal("changed", func(text string) {
		a.passwordNext = text
	})

	confirmInput := newSettingsElement("TextInput")
	confirmInput.SetAttribute("text", a.passwordConfirm)
	confirmInput.SetAttribute("placeholder", "Confirm new password")
	confirmInput.SetAttribute("password", true)
	confirmInput.BindSignal("changed", func(text string) {
		a.passwordConfirm = text
	})

	buttons := newSettingsElement("HBox")
	buttons.SetAttribute("direction", "row")
	buttons.SetAttribute("spacing", 8)
	buttons.SetAttribute("expand", false)

	updateBtn := newSettingsElement("PrimaryButton")
	updateBtn.SetAttribute("text", "Update Password")
	updateBtn.BindSignal("clicked", func() {
		username := a.currentUsername()
		if strings.TrimSpace(username) == "" {
			a.setStatus("No active user detected")
			return
		}
		if a.passwordNext == "" {
			a.setStatus("New password is required")
			return
		}
		if a.passwordNext != a.passwordConfirm {
			a.setStatus("New password and confirmation do not match")
			return
		}
		if err := identity.UpdatePassword(username, a.passwordCurrent, a.passwordNext); err != nil {
			a.setStatus(fmt.Sprintf("Password update failed: %v", err))
			return
		}

		a.passwordCurrent = ""
		a.passwordNext = ""
		a.passwordConfirm = ""
		a.renderPage()
		a.setStatus("Password updated")
	})
	buttons.AddChild(updateBtn)

	clearBtn := newSettingsElement("Button")
	clearBtn.SetAttribute("text", "Clear")
	clearBtn.BindSignal("clicked", func() {
		a.passwordCurrent = ""
		a.passwordNext = ""
		a.passwordConfirm = ""
		a.renderPage()
		a.setStatus("Password form cleared")
	})
	buttons.AddChild(clearBtn)

	box.AddChild(currentInput)
	box.AddChild(newInput)
	box.AddChild(confirmInput)
	box.AddChild(buttons)
	return card
}

func (a *settingsApp) currentUsername() string {
	if user, err := osuser.Current(); err == nil {
		name := strings.TrimSpace(user.Username)
		if name != "" {
			return name
		}
	}
	if fallback := strings.TrimSpace(a.values["/dev/rlxos/accounts/default_user"]); fallback != "" {
		return fallback
	}
	return "user"
}

func (a *settingsApp) buildServicesListCard() *ui.Element {
	card, box := newSectionCard(
		"Service List",
		"Table view for services from api/service.",
	)

	items, err := listServices()
	if err != nil {
		errMsg := newSettingsElement("Paragraph")
		errMsg.SetAttribute("text", "Unable to list services: "+err.Error())
		box.AddChild(errMsg)
		return card
	}

	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.Name,
			serviceStateLabel(item),
			strconv.Itoa(item.PID),
			item.Type,
			item.Restart,
		})
	}

	table := buildTableView(
		[]string{"Name", "State", "PID", "Type", "Restart"},
		rows,
		240,
	)
	box.AddChild(table)
	return card
}

func (a *settingsApp) buildServicesActionCard() *ui.Element {
	card, box := newSectionCard(
		"Service Actions",
		"Start, stop, restart, and refresh services.",
	)

	target := newSettingsElement("TextInput")
	target.SetAttribute("text", a.serviceTarget)
	target.SetAttribute("placeholder", "service name")
	target.BindSignal("changed", func(text string) {
		a.serviceTarget = strings.TrimSpace(text)
	})
	box.AddChild(target)

	actions := newSettingsElement("HBox")
	actions.SetAttribute("direction", "row")
	actions.SetAttribute("spacing", 8)
	actions.SetAttribute("expand", false)

	startBtn := newSettingsElement("PrimaryButton")
	startBtn.SetAttribute("text", "Start")
	startBtn.BindSignal("clicked", func() { a.runServiceAction("start") })
	actions.AddChild(startBtn)

	stopBtn := newSettingsElement("Button")
	stopBtn.SetAttribute("text", "Stop")
	stopBtn.BindSignal("clicked", func() { a.runServiceAction("stop") })
	actions.AddChild(stopBtn)

	restartBtn := newSettingsElement("Button")
	restartBtn.SetAttribute("text", "Restart")
	restartBtn.BindSignal("clicked", func() { a.runServiceAction("restart") })
	actions.AddChild(restartBtn)

	refreshBtn := newSettingsElement("Button")
	refreshBtn.SetAttribute("text", "Refresh")
	refreshBtn.BindSignal("clicked", func() { a.RefreshPage() })
	actions.AddChild(refreshBtn)

	box.AddChild(actions)
	return card
}

func (a *settingsApp) runServiceAction(action string) {
	name := strings.TrimSpace(a.serviceTarget)
	if name == "" {
		a.setStatus("Select a service name first")
		return
	}

	client, err := serviceapi.Connect()
	if err != nil {
		a.setStatus("Service API unavailable: " + err.Error())
		return
	}
	defer client.Close()

	switch action {
	case "start":
		err = client.Start(name)
	case "stop":
		err = client.Stop(name)
	case "restart":
		err = client.Restart(name)
	default:
		err = fmt.Errorf("unknown action %q", action)
	}
	if err != nil {
		a.setStatus(fmt.Sprintf("Service %s failed: %v", action, err))
		return
	}
	a.renderPage()
	a.setStatus(fmt.Sprintf("Service %s: %s", action, name))
}

func listServices() ([]serviceapi.ServiceStatus, error) {
	client, err := serviceapi.Connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	items, err := client.List()
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, nil
}

func serviceStateLabel(item serviceapi.ServiceStatus) string {
	switch {
	case item.Running:
		return "running"
	case item.Failed:
		return "failed"
	case item.Started:
		return "started"
	default:
		return "stopped"
	}
}

func (a *settingsApp) buildDistroListCard() *ui.Element {
	card, box := newSectionCard(
		"Distro Status",
		"Linux distribution environment status.",
	)

	client, err := distroapi.Connect()
	if err != nil {
		msg := newSettingsElement("Paragraph")
		msg.SetAttribute("text", "Distro API unavailable: "+err.Error())
		box.AddChild(msg)
		return card
	}
	defer client.Close()

	status, err := client.GetStatus()
	if err != nil {
		msg := newSettingsElement("Paragraph")
		msg.SetAttribute("text", "Failed to get distro status: "+err.Error())
		box.AddChild(msg)
		return card
	}

	if status.Installed {
		box.AddChild(buildTableView(
			[]string{"Property", "Value"},
			[][]string{
				{"Installed", "Yes"},
				{"Path", status.Path},
				{"Size", humanSize(status.Size)},
			},
			180,
		))
	} else {
		msg := newSettingsElement("Paragraph")
		msg.SetAttribute("text", "Distro is not installed. Use the Install button below to set up the environment.")
		box.AddChild(msg)
	}
	return card
}

func humanSize(size int) string {
	if size <= 0 {
		return "-"
	}
	value := float64(size)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	idx := 0
	for value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}
	if value >= 10 || idx == 0 {
		return fmt.Sprintf("%.0f%s", value, units[idx])
	}
	return fmt.Sprintf("%.1f%s", value, units[idx])
}

func (a *settingsApp) buildDistroManageCard() *ui.Element {
	card, box := newSectionCard(
		"Manage Distro",
		"Install or remove the Linux distro environment.",
	)

	urlInput := newSettingsElement("TextInput")
	urlInput.SetAttribute("text", a.distroInstallURL)
	urlInput.SetAttribute("placeholder", "Optional source URL (leave empty for default)")
	urlInput.BindSignal("changed", func(text string) {
		a.distroInstallURL = strings.TrimSpace(text)
	})
	box.AddChild(urlInput)

	buttons := newSettingsElement("HBox")
	buttons.SetAttribute("direction", "row")
	buttons.SetAttribute("spacing", 8)
	buttons.SetAttribute("expand", false)

	installBtn := newSettingsElement("PrimaryButton")
	installBtn.SetAttribute("text", "Install")
	installBtn.BindSignal("clicked", func() {
		client, err := distroapi.Connect()
		if err != nil {
			a.setStatus("Distro API unavailable: " + err.Error())
			return
		}
		defer client.Close()
		if err := client.InstallDistro(a.distroInstallURL); err != nil {
			a.setStatus("Install failed: " + err.Error())
			return
		}
		a.renderPage()
		a.setStatus("Distro installed successfully")
	})
	buttons.AddChild(installBtn)

	removeBtn := newSettingsElement("Button")
	removeBtn.SetAttribute("text", "Remove")
	removeBtn.BindSignal("clicked", func() {
		client, err := distroapi.Connect()
		if err != nil {
			a.setStatus("Distro API unavailable: " + err.Error())
			return
		}
		defer client.Close()
		if err := client.Uninstall(); err != nil {
			a.setStatus("Remove failed: " + err.Error())
			return
		}
		a.renderPage()
		a.setStatus("Distro removed")
	})
	buttons.AddChild(removeBtn)

	refreshBtn := newSettingsElement("Button")
	refreshBtn.SetAttribute("text", "Refresh")
	refreshBtn.BindSignal("clicked", func() { a.RefreshPage() })
	buttons.AddChild(refreshBtn)

	box.AddChild(buttons)
	return card
}

func (a *settingsApp) buildDevicesListCard() *ui.Element {
	card, box := newSectionCard(
		"Device List",
		"Device inventory from api/uevent.",
	)

	devices, err := listDevices()
	if err != nil {
		msg := newSettingsElement("Paragraph")
		msg.SetAttribute("text", "Failed to list devices: "+err.Error())
		box.AddChild(msg)
		return card
	}

	rows := make([][]string, 0, len(devices))
	for i, dev := range devices {
		if i >= 80 {
			break
		}
		rows = append(rows, []string{
			fallbackText(dev.DevName, "-"),
			fallbackText(dev.Subsystem, "-"),
			fallbackText(dev.DevType, "-"),
			fallbackText(dev.Driver, "-"),
			fallbackText(dev.DevPath, "-"),
		})
	}

	box.AddChild(buildTableView(
		[]string{"Name", "Subsystem", "Type", "Driver", "Path"},
		rows,
		260,
	))
	return card
}

func (a *settingsApp) buildDevicesManageCard() *ui.Element {
	card, box := newSectionCard(
		"Device Actions",
		"Trigger device scan/events using api/uevent.",
	)

	triggerInput := newSettingsElement("TextInput")
	triggerInput.SetAttribute("text", a.deviceTriggerScope)
	triggerInput.SetAttribute("placeholder", "Subsystem (example: net)")
	triggerInput.BindSignal("changed", func(text string) {
		a.deviceTriggerScope = strings.TrimSpace(text)
	})
	box.AddChild(triggerInput)

	buttons := newSettingsElement("HBox")
	buttons.SetAttribute("direction", "row")
	buttons.SetAttribute("spacing", 8)
	buttons.SetAttribute("expand", false)

	triggerBtn := newSettingsElement("PrimaryButton")
	triggerBtn.SetAttribute("text", "Trigger")
	triggerBtn.BindSignal("clicked", func() {
		client, err := ueventapi.Connect()
		if err != nil {
			a.setStatus("uevent API unavailable: " + err.Error())
			return
		}
		defer client.Close()

		if err := client.Trigger(a.deviceTriggerScope); err != nil {
			a.setStatus("Trigger failed: " + err.Error())
			return
		}
		a.renderPage()
		if strings.TrimSpace(a.deviceTriggerScope) == "" {
			a.setStatus("Triggered uevent scan")
			return
		}
		a.setStatus("Triggered uevent for " + a.deviceTriggerScope)
	})
	buttons.AddChild(triggerBtn)

	refreshBtn := newSettingsElement("Button")
	refreshBtn.SetAttribute("text", "Refresh")
	refreshBtn.BindSignal("clicked", func() { a.RefreshPage() })
	buttons.AddChild(refreshBtn)

	box.AddChild(buttons)
	return card
}

func listDevices() ([]ueventapi.DeviceInfo, error) {
	client, err := ueventapi.Connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	devices, err := client.ListDevices()
	if err != nil {
		return nil, err
	}

	sort.Slice(devices, func(i, j int) bool {
		keyI := strings.ToLower(devices[i].Subsystem + "|" + devices[i].DevName + "|" + devices[i].DevPath)
		keyJ := strings.ToLower(devices[j].Subsystem + "|" + devices[j].DevName + "|" + devices[j].DevPath)
		return keyI < keyJ
	})
	return devices, nil
}

func (a *settingsApp) buildConfigsListCard() *ui.Element {
	card, box := newSectionCard(
		"Config Entries",
		"Raw settings list from api/settings.",
	)

	filterInput := newSettingsElement("TextInput")
	filterInput.SetAttribute("text", a.configFilterPrefix)
	filterInput.SetAttribute("placeholder", "Optional prefix (for example /dev/rlxos/display)")
	filterInput.BindSignal("changed", func(text string) {
		a.configFilterPrefix = strings.TrimSpace(text)
	})
	box.AddChild(filterInput)

	filterButtons := newSettingsElement("HBox")
	filterButtons.SetAttribute("direction", "row")
	filterButtons.SetAttribute("spacing", 8)
	filterButtons.SetAttribute("expand", false)

	applyFilter := newSettingsElement("PrimaryButton")
	applyFilter.SetAttribute("text", "Apply Filter")
	applyFilter.BindSignal("clicked", func() {
		a.renderPage()
		a.setStatus("Filter applied")
	})
	filterButtons.AddChild(applyFilter)

	clearFilter := newSettingsElement("Button")
	clearFilter.SetAttribute("text", "Clear")
	clearFilter.BindSignal("clicked", func() {
		a.configFilterPrefix = ""
		a.renderPage()
		a.setStatus("Filter cleared")
	})
	filterButtons.AddChild(clearFilter)
	box.AddChild(filterButtons)

	client, err := settingsapi.Connect()
	if err != nil {
		msg := newSettingsElement("Paragraph")
		msg.SetAttribute("text", "Settings API unavailable: "+err.Error())
		box.AddChild(msg)
		return card
	}
	defer client.Close()

	items, err := client.List(a.configFilterPrefix)
	if err != nil {
		msg := newSettingsElement("Paragraph")
		msg.SetAttribute("text", "Failed to list settings: "+err.Error())
		box.AddChild(msg)
		return card
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})

	rows := make([][]string, 0, len(items))
	for i, item := range items {
		if i >= 200 {
			break
		}
		rows = append(rows, []string{item.Key, item.Value})
	}

	box.AddChild(buildTableView([]string{"Key", "Value"}, rows, 280))
	return card
}

func (a *settingsApp) buildConfigsManageCard() *ui.Element {
	card, box := newSectionCard(
		"Manage Config",
		"Set a raw setting key/value pair through api/settings.",
	)

	keyInput := newSettingsElement("TextInput")
	keyInput.SetAttribute("text", a.configEditKey)
	keyInput.SetAttribute("placeholder", "/dev/rlxos/example/key")
	keyInput.BindSignal("changed", func(text string) {
		a.configEditKey = text
	})
	box.AddChild(keyInput)

	valueInput := newSettingsElement("TextInput")
	valueInput.SetAttribute("text", a.configEditValue)
	valueInput.SetAttribute("placeholder", "value")
	valueInput.BindSignal("changed", func(text string) {
		a.configEditValue = text
	})
	box.AddChild(valueInput)

	buttons := newSettingsElement("HBox")
	buttons.SetAttribute("direction", "row")
	buttons.SetAttribute("spacing", 8)
	buttons.SetAttribute("expand", false)

	saveBtn := newSettingsElement("PrimaryButton")
	saveBtn.SetAttribute("text", "Save")
	saveBtn.BindSignal("clicked", func() {
		key := strings.TrimSpace(a.configEditKey)
		if key == "" {
			a.setStatus("Config key is required")
			return
		}

		client, err := settingsapi.Connect()
		if err != nil {
			a.setStatus("Settings API unavailable: " + err.Error())
			return
		}
		defer client.Close()

		if err := client.Set(key, a.configEditValue); err != nil {
			a.setStatus("Save failed: " + err.Error())
			return
		}

		canonical := canonicalSettingKey(key)
		if field, ok := settingFieldByKey[canonical]; ok {
			value := normalizeSettingValue(field, a.configEditValue)
			a.values[field.Key] = value
			a.persisted[field.Key] = value
		}
		a.renderPage()
		a.setStatus("Saved " + canonical)
	})
	buttons.AddChild(saveBtn)

	refreshBtn := newSettingsElement("Button")
	refreshBtn.SetAttribute("text", "Refresh")
	refreshBtn.BindSignal("clicked", func() { a.RefreshPage() })
	buttons.AddChild(refreshBtn)
	box.AddChild(buttons)
	return card
}

func (a *settingsApp) buildAboutSystemCard() *ui.Element {
	card, box := newSectionCard(
		"System Info",
		"Core OS/runtime information.",
	)

	defaults := detectRuntimeDefaults()
	addInfoRows(box, []infoRow{
		{Label: "Name", Value: fallbackText(defaults["/dev/rlxos/system/about/name"], "RlxOS")},
		{Label: "Version", Value: fallbackText(defaults["/dev/rlxos/system/about/version"], "0.1.0")},
		{Label: "Kernel", Value: fallbackText(defaults["/dev/rlxos/system/about/kernel"], runtime.GOOS)},
		{Label: "Go Version", Value: fallbackText(defaults[keySystemGoVersion], runtime.Version())},
		{Label: "Architecture", Value: fallbackText(defaults["/dev/rlxos/system/about/architecture"], runtime.GOARCH)},
	})
	return card
}

func (a *settingsApp) buildAboutUpdatesCard() *ui.Element {
	card, box := newSectionCard(
		"Updates",
		"System update controls will be added in a later release.",
	)

	msg := newSettingsElement("Paragraph")
	msg.SetAttribute("text", "Automatic update checks are currently disabled for this build.")
	box.AddChild(msg)

	checkBtn := newSettingsElement("PrimaryButton")
	checkBtn.SetAttribute("text", "Check for Updates")
	checkBtn.SetAttribute("interactive", false)
	box.AddChild(checkBtn)
	return card
}

func buildTableView(headers []string, rows [][]string, minHeight int) *ui.Element {
	table := newSettingsElement("TableView")
	table.SetAttribute("text", buildTableText(headers, rows))
	table.SetAttribute("readOnly", true)
	table.SetAttribute("minHeight", minHeight)
	return table
}

func buildTableText(headers []string, rows [][]string) string {
	cleanHeaders := make([]string, 0, len(headers))
	for _, header := range headers {
		cleanHeaders = append(cleanHeaders, sanitizeTableCell(header))
	}

	lines := make([]string, 0, len(rows)+1)
	if len(cleanHeaders) > 0 {
		lines = append(lines, strings.Join(cleanHeaders, "\t"))
	}
	for _, row := range rows {
		cols := make([]string, 0, len(cleanHeaders))
		maxCols := len(cleanHeaders)
		if maxCols == 0 {
			maxCols = len(row)
		}
		for i := 0; i < maxCols; i++ {
			value := "-"
			if i < len(row) {
				value = sanitizeTableCell(row[i])
			}
			cols = append(cols, value)
		}
		lines = append(lines, strings.Join(cols, "\t"))
	}
	if len(lines) == 0 {
		return "Key\tValue\n-\t-"
	}
	return strings.Join(lines, "\n")
}

func sanitizeTableCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "\t", " ")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func addInfoRows(host *ui.Element, rows []infoRow) {
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []string{
			fallbackText(row.Label, "-"),
			fallbackText(row.Value, "-"),
		})
	}
	minHeight := 120
	if len(tableRows) > 0 {
		minHeight = 56 + len(tableRows)*34
	}
	if minHeight > 320 {
		minHeight = 320
	}
	host.AddChild(buildTableView([]string{"Key", "Value"}, tableRows, minHeight))
}

func fallbackText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
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
	case keyRoundedCornersRadius:
		return normalizeRoundedRadius(value)
	case keyDisplayBrightness:
		return normalizeDisplayBrightness(value)
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

func normalizeRoundedRadius(value string) string {
	switch strings.TrimSpace(value) {
	case "8":
		return "8"
	case "12":
		return "12"
	case "16":
		return "16"
	case "20":
		return "20"
	default:
		return defaultRoundedRadius
	}
}

func normalizeDisplayBrightness(value string) string {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return defaultBrightness
	}
	if parsed < 1 {
		parsed = 1
	}
	if parsed > 100 {
		parsed = 100
	}
	return strconv.Itoa(parsed)
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

	svc, err := sutra.NewService(settingsapi.ServiceName, fs.Resolve("user:%s", settingsapi.ServiceName))
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
