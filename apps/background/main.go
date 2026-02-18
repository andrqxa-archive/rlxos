package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	display "avyos.dev/api/display"
	settingsapi "avyos.dev/api/settings"
	"avyos.dev/pkg/graphics"
	gapp "avyos.dev/pkg/graphics/app"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
	"avyos.dev/pkg/graphics/ui"
	"avyos.dev/pkg/logger"
)

//go:embed ui/background.ui
var backgroundUI string

var log = logger.New("background")

type options struct {
	mode      string
	imagePath string
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
	ui.App
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
	app.SetOptions(gapp.Options{Title: "Background", Backend: backend, Input: backend, Background: opts.color})
	if err := app.LoadString(backgroundUI, app); err != nil {
		return err
	}
	if root := app.FindElement("Root"); root != nil {
		root.SetAttribute("background", colorToHex(opts.color))
	}
	if wallpaper := app.FindElement("Wallpaper"); wallpaper != nil {
		wallpaper.SetAttribute("src", opts.imagePath)
		wallpaper.SetAttribute("scaleMode", "cover")
	}

	go watchSettings(app)

	if err := app.Run(); err != nil {
		log.Error("background error: %v", err)
		os.Exit(1)
	}
	return nil
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
			case "background.wallpaper":
				if wallpaper := app.FindElement("Wallpaper"); wallpaper != nil {
					wallpaper.SetAttribute("src", strings.TrimSpace(ev.Value))
				}
			case "background.color":
				value := strings.TrimSpace(ev.Value)
				if value == "" {
					setAppBackground(app, graphics.DefaultTheme.Background)
					if root := app.FindElement("Root"); root != nil {
						root.SetAttribute("background", colorToHex(graphics.DefaultTheme.Background))
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
		color:     graphics.DefaultTheme.Background,
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
		if configured := loadSetting("background.wallpaper"); configured != "" {
			opts.imagePath = configured
		}
	}
	if opts.imagePath == "" {
		opts.imagePath = "/avyos/data/backgrounds/default.png"
	}

	colorText := strings.TrimSpace(flagColor)
	colorFromFlag := flagProvided("color")
	if !colorFromFlag {
		colorText = loadSetting("background.color")
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

func loadSetting(key string) string {
	for range 6 {
		client, err := settingsapi.Connect()
		if err == nil {
			value, err := client.Get(key)
			_ = client.Close()
			if err == nil {
				return strings.TrimSpace(value)
			}
			return ""
		}
		time.Sleep(100 * time.Millisecond)
	}
	return ""
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
