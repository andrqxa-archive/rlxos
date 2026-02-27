package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	display "avyos.dev/api/display"
	settingsapi "avyos.dev/api/settings"
	gapp "avyos.dev/pkg/graphics/app"
	declapp "avyos.dev/pkg/graphics/app/decl"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
	graphics "avyos.dev/pkg/graphics/input"
	gfxtheme "avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/logger"
)

//go:embed ui/background.ui
var backgroundUI string

var log = logger.New("background")

const (
	shortcutDesktopMenu uint32 = 1
	shortcutLaunchpad   uint32 = 2
	shortcutTerminal    uint32 = 3

	defaultWallpaperPath = "/avyos/data/backgrounds/default.png"

	keyBackgroundSource = "/dev/rlxos/background/source"
	keyBackgroundScale  = "/dev/rlxos/background/scale"
	keyBackgroundColor  = "/dev/rlxos/background/color"

	legacyWallpaperKey = "background.wallpaper"
	legacyColorKey     = "background.color"
)

type options struct {
	mode      string
	imagePath string
	scaleMode string
	color     graphics.Color
}

var (
	flagMode  string
	flagImage string
	flagColor string
)

func init() {
	flag.StringVar(&flagMode, "mode", "", "Display mode: layer or window")
	flag.StringVar(&flagImage, "image", "", "Background image path")
	flag.StringVar(&flagColor, "color", "", "Background color (hex, e.g. #11161e or 11161e)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "background - Desktop background app")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  background [--mode layer|window] [--image <path>] [--color <hex>]")
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

type BackgroundApp struct {
	declapp.App
	mouseX int
	mouseY int
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "background: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if err := logger.SetupSystemLog(); err != nil {
		log.Error("failed to setup system log: %v", err)
	}
	opts, err := parseOptions()
	if err != nil {
		return err
	}

	backend := displaybackend.New()
	if opts.mode == "layer" {
		backend.SetLayer(display.LayerBackground, display.AnchorTop|display.AnchorBottom|display.AnchorLeft|display.AnchorRight, 0)
		backend.SetLayerOpaqueHint(true)
	}

	app := &BackgroundApp{}
	app.mouseX = -1
	app.mouseY = -1
	app.SetOptions(gapp.Options{Title: "Background", Backend: backend, Input: backend, Background: opts.color})
	if err := app.LoadString(backgroundUI, app); err != nil {
		return err
	}
	app.Configure(func(core *gapp.App) {
		core.OnEvent = app.handleCoreEvent
	})
	if err := app.RegisterClientShortcut(shortcutDesktopMenu, 0, graphics.KeyF10, 0, graphics.ModShift, func(graphics.Event) {
		app.openDesktopMenuAtPointer()
	}); err != nil {
		return err
	}
	if err := app.RegisterGlobalShortcut(shortcutLaunchpad, graphics.KeySpace, 0, graphics.ModCtrl, func(graphics.Event) {
		app.OpenLaunchpad()
	}); err != nil {
		return err
	}
	if err := app.RegisterGlobalShortcut(shortcutTerminal, graphics.KeyNone, 't', graphics.ModCtrl|graphics.ModAlt, func(graphics.Event) {
		app.OpenTerminal()
	}); err != nil {
		return err
	}
	if root := app.FindElement("Root"); root != nil {
		root.SetAttribute("background", colorToHex(opts.color))
	}
	if wallpaper := app.FindElement("Wallpaper"); wallpaper != nil {
		wallpaper.SetAttribute("src", opts.imagePath)
		wallpaper.SetAttribute("scaleMode", opts.scaleMode)
	}

	go watchSettings(app)

	if err := app.Run(); err != nil {
		log.Error("background error: %v", err)
		os.Exit(1)
	}
	return nil
}

func (a *BackgroundApp) handleCoreEvent(ev graphics.Event) bool {
	switch ev.Type {
	case graphics.EventMouseMove:
		a.mouseX = ev.X
		a.mouseY = ev.Y
	case graphics.EventMouseButtonRelease:
		if ev.MouseButton == graphics.MouseButtonRight {
			a.mouseX = ev.X
			a.mouseY = ev.Y
			return a.OpenMenu("DesktopMenu", ev.X, ev.Y)
		}
	case graphics.EventMouseButtonPress:
		// Dismiss open desktop menu quickly when user clicks the wallpaper.
		if ev.MouseButton == graphics.MouseButtonLeft {
			a.CloseMenu()
		}
	}
	return false
}

func (a *BackgroundApp) openDesktopMenuAtPointer() {
	x, y := a.mouseX, a.mouseY
	if x < 0 || y < 0 {
		if root := a.FindElement("Root"); root != nil {
			b := root.Bounds()
			x = b.W / 2
			y = b.H / 2
		} else {
			x, y = 20, 20
		}
	}
	_ = a.OpenMenu("DesktopMenu", x, y)
}

func (a *BackgroundApp) OpenLaunchpad() {
	a.CloseMenu()
	if err := runCommand("appmenu"); err != nil {
		log.Warn("failed to launch app menu: %v", err)
	}
}

func (a *BackgroundApp) OpenTerminal() {
	a.CloseMenu()
	if err := runCommand("terminal"); err != nil {
		log.Warn("failed to launch terminal: %v", err)
	}
}

func (a *BackgroundApp) OpenFileManager() {
	a.CloseMenu()
	if err := runCommand("filemanager"); err != nil {
		log.Warn("failed to launch file manager: %v", err)
	}
}

func (a *BackgroundApp) OpenTaskManager() {
	a.CloseMenu()
	if err := runCommand("taskmanager"); err != nil {
		log.Warn("failed to launch task manager: %v", err)
	}
}

func (a *BackgroundApp) RefreshDesktop() {
	a.CloseMenu()
	a.RequestFrame()
}

func watchSettings(app *BackgroundApp) {
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

		client.OnChanged(func(ev settingsapi.ChangedEvent) {
			switch ev.Key {
			case keyBackgroundSource, legacyWallpaperKey:
				if wallpaper := app.FindElement("Wallpaper"); wallpaper != nil {
					wallpaper.SetAttribute("src", strings.TrimSpace(ev.Value))
				}
			case keyBackgroundScale:
				if wallpaper := app.FindElement("Wallpaper"); wallpaper != nil {
					wallpaper.SetAttribute("scaleMode", normalizeBackgroundScale(ev.Value))
				}
			case keyBackgroundColor, legacyColorKey:
				value := strings.TrimSpace(ev.Value)
				if value == "" {
					setAppBackground(app, gfxtheme.DefaultTheme.Background)
					if root := app.FindElement("Root"); root != nil {
						root.SetAttribute("background", colorToHex(gfxtheme.DefaultTheme.Background))
					}
					return
				}
				color, err := parseHexColor(value)
				if err != nil {
					log.Warn("ignoring invalid background.color setting %q: %v", value, err)
					return
				}
				setAppBackground(app, color)
				if root := app.FindElement("Root"); root != nil {
					root.SetAttribute("background", colorToHex(color))
				}
			}
		})

		<-disconnected
		_ = client.Close()
		time.Sleep(200 * time.Millisecond)
	}
}

func setAppBackground(app *BackgroundApp, color graphics.Color) {
	internal := app.InternalApp()
	if internal != nil {
		internal.SetBackground(color)
	}
}

func parseOptions() (options, error) {
	opts := options{
		mode:      strings.ToLower(strings.TrimSpace(flagMode)),
		imagePath: strings.TrimSpace(flagImage),
		scaleMode: "cover",
		color:     gfxtheme.DefaultTheme.Background,
	}

	if !flagProvided("mode") {
		opts.mode = "layer"
	}
	if opts.mode == "" {
		opts.mode = "layer"
	}
	if opts.mode != "layer" && opts.mode != "window" {
		return opts, fmt.Errorf("invalid --mode %q (use layer or window)", opts.mode)
	}

	if !flagProvided("image") {
		if configured, ok := loadSettingCompat(keyBackgroundSource, legacyWallpaperKey); ok {
			opts.imagePath = configured
		}
	}
	if opts.imagePath == "" {
		opts.imagePath = defaultWallpaperPath
	}

	if configured, ok := loadSettingCompat(keyBackgroundScale); ok {
		opts.scaleMode = normalizeBackgroundScale(configured)
	}

	colorText := strings.TrimSpace(flagColor)
	colorFromFlag := flagProvided("color")
	if !colorFromFlag {
		if configured, ok := loadSettingCompat(keyBackgroundColor, legacyColorKey); ok {
			colorText = configured
		}
	}
	if colorText != "" {
		color, err := parseHexColor(colorText)
		if err != nil {
			if colorFromFlag {
				return opts, err
			}
			log.Warn("ignoring invalid background.color setting %q: %v", colorText, err)
			return opts, nil
		}
		opts.color = color
	}
	return opts, nil
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
		return "cover"
	}
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

func parseHexColor(value string) (graphics.Color, error) {
	v := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(v) != 6 && len(v) != 8 {
		return graphics.Color{}, fmt.Errorf("invalid --color %q: expected RRGGBB or RRGGBBAA", value)
	}
	var parsed uint32
	for _, ch := range v {
		parsed <<= 4
		switch {
		case ch >= '0' && ch <= '9':
			parsed += uint32(ch - '0')
		case ch >= 'a' && ch <= 'f':
			parsed += uint32(ch-'a') + 10
		case ch >= 'A' && ch <= 'F':
			parsed += uint32(ch-'A') + 10
		default:
			return graphics.Color{}, fmt.Errorf("invalid --color %q: bad hex digit %q", value, string(ch))
		}
	}
	return graphics.NewColorHex(parsed), nil
}

func colorToHex(c graphics.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
