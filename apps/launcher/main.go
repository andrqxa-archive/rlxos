package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	display "avyos.dev/api/display"
	"avyos.dev/pkg/appcatalog"
	gapp "avyos.dev/pkg/graphics/app"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
	core "avyos.dev/pkg/graphics/pixmap"
	ui "avyos.dev/pkg/graphics/widget/engine"
	"avyos.dev/pkg/logger"
)

//go:embed ui/appmenu.ui
var appMenuUI string

const (
	launchpadTileSize = 116
	launchpadIconSize = 62
	appMenuID         = "dev.avyos.appmenu"
)

var log = logger.New(appMenuID)

type AppMenu struct {
	gapp.App

	home      string
	catalog   []appcatalog.Entry
	filtered  []appcatalog.Entry
	lastQuery string
}

func (a *AppMenu) e(id string) *ui.Element { return a.FindElement(id) }

func (a *AppMenu) SearchChanged(text string) {
	a.rebuildGrid(text)
}

func (a *AppMenu) SearchSubmitted(text string) {
	a.rebuildGrid(text)
	if len(a.filtered) == 0 {
		return
	}
	a.launchEntry(a.filtered[0])
}

func (a *AppMenu) DismissMenu() {
	a.Quit()
}

func (a *AppMenu) loadCatalog() {
	raw := appcatalog.Discover(appcatalog.DiscoverOptions{
		Home:          a.home,
		IncludeHidden: false,
	})
	out := make([]appcatalog.Entry, 0, len(raw))
	for _, entry := range raw {
		if !showInLaunchpad(entry) {
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(out[i].Name))
		lj := strings.ToLower(strings.TrimSpace(out[j].Name))
		if li == lj {
			return out[i].ID < out[j].ID
		}
		return li < lj
	})
	a.catalog = out
}

func showInLaunchpad(entry appcatalog.Entry) bool {
	if entry.Background || entry.Dock || entry.Hidden {
		return false
	}
	id := strings.ToLower(strings.TrimSpace(entry.ID))
	if id == "" || id == appMenuID {
		return false
	}
	if strings.TrimSpace(entry.ExecPath) == "" {
		return false
	}
	return true
}

func filterEntries(entries []appcatalog.Entry, query string) []appcatalog.Entry {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		out := make([]appcatalog.Entry, len(entries))
		copy(out, entries)
		return out
	}

	out := make([]appcatalog.Entry, 0, len(entries))
	for _, entry := range entries {
		name := strings.ToLower(strings.TrimSpace(entry.Name))
		id := strings.ToLower(strings.TrimSpace(entry.ID))
		desc := strings.ToLower(strings.TrimSpace(entry.Description))
		if strings.Contains(name, query) || strings.Contains(id, query) || strings.Contains(desc, query) {
			out = append(out, entry)
		}
	}
	return out
}

func (a *AppMenu) rebuildGrid(query string) {
	query = strings.TrimSpace(query)
	if query == a.lastQuery && a.filtered != nil {
		return
	}

	grid := a.e("AppGrid")
	if grid == nil {
		return
	}
	grid.ClearChildren()

	filtered := filterEntries(a.catalog, query)
	a.filtered = filtered
	grid.SetAttribute("scrollY", 0)
	a.lastQuery = query

	for _, entry := range filtered {
		entryCopy := entry
		grid.AddChild(buildLaunchpadTile(entryCopy, func() {
			a.launchEntry(entryCopy)
		}))
	}

	empty := a.e("EmptyState")
	if empty != nil {
		if len(filtered) == 0 {
			if query == "" {
				empty.SetAttribute("text", "No apps available")
			} else {
				empty.SetAttribute("text", fmt.Sprintf("No apps for %q", query))
			}
			empty.SetVisible(true)
		} else {
			empty.SetVisible(false)
		}
	}
	a.Redraw()
}

func buildLaunchpadTile(entry appcatalog.Entry, onClick func()) *ui.Element {
	btn := ui.NewElement("Button")
	btn.SetAttribute("text", "")
	btn.SetAttribute("padding", "8")
	btn.SetAttribute("minWidth", launchpadTileSize)
	btn.SetAttribute("maxWidth", launchpadTileSize)
	btn.SetAttribute("minHeight", launchpadTileSize)
	btn.SetAttribute("maxHeight", launchpadTileSize)
	btn.SetAttribute("borderRadius", 14)
	btn.SetAttribute("focusRing", false)
	btn.SetAttribute("background", "transparent")
	btn.SetAttribute("gradientTop", "transparent")
	btn.SetAttribute("gradientBottom", "transparent")
	btn.SetAttribute("borderColor", "transparent")
	btn.SetAttribute("focusedBorderColor", "transparent")
	btn.SetAttribute("hoverBackground", "transparent")
	btn.SetAttribute("pressedBackground", "transparent")
	btn.SetAttribute("shadow", true)
	btn.SetAttribute("shadowOnlyOnHover", true)
	btn.SetAttribute("allowShadowWithoutBackground", true)
	btn.SetAttribute("shadowColor", "theme.color.accent.subtle")
	btn.SetAttribute("shadowSpread", 22)
	btn.SetAttribute("shadowGap", 4)
	btn.SetAttribute("shadowOffsetY", 0)

	col := ui.NewElement("VBox")
	col.SetAttribute("direction", "column")
	col.SetAttribute("alignment", "center")
	col.SetAttribute("spacing", 8)
	col.SetAttribute("expand", true)
	col.SetAttribute("interactive", false)

	icon := ui.NewElement("Image")
	icon.SetAttribute("src", entry.IconPath)
	icon.SetAttribute("srcOpaque", false)
	icon.SetAttribute("scaleMode", "contain")
	icon.SetAttribute("minWidth", launchpadIconSize)
	icon.SetAttribute("maxWidth", launchpadIconSize)
	icon.SetAttribute("minHeight", launchpadIconSize)
	icon.SetAttribute("maxHeight", launchpadIconSize)
	icon.SetAttribute("interactive", false)

	label := ui.NewElement("Label")
	label.SetAttribute("text", ellipsisText(entry.Name, 92))
	label.SetAttribute("textAlign", "center")
	label.SetAttribute("minWidth", 96)
	label.SetAttribute("maxWidth", 96)
	label.SetAttribute("clipText", true)
	label.SetAttribute("textColor", "theme.color.text.primary")
	label.SetAttribute("interactive", false)

	col.AddChild(icon)
	col.AddChild(label)
	btn.AddChild(col)

	if onClick != nil {
		btn.BindSignal("clicked", onClick)
	}
	return btn
}

func (a *AppMenu) launchEntry(entry appcatalog.Entry) {
	if strings.TrimSpace(entry.ExecPath) == "" {
		return
	}
	if err := runCommand(entry.ExecPath); err != nil {
		log.Warn("failed to launch %s: %v", entry.ID, err)
		return
	}
	a.Quit()
}

func resolveBlurImage() string {
	candidates := []string{
		"/avyos/data/backgrounds/default_blur.png",
		"/data/backgrounds/default_blur.png",
		"data/backgrounds/default_blur.png",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
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

func ellipsisText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func main() {
	if err := logger.SetupSystemLog(); err != nil {
		log.Error("failed to setup system log: %v", err)
	}

	backend := displaybackend.New()
	backend.SetSize(1280, 760)
	backend.SetLayer(
		display.LayerOverlay,
		display.AnchorTop|display.AnchorBottom|display.AnchorLeft|display.AnchorRight,
		0,
	)

	home, _ := os.UserHomeDir()
	app := &AppMenu{home: home}
	app.SetOptions(gapp.Options{
		Title:      "App Menu",
		Backend:    backend,
		Input:      backend,
		Background: core.ColorTransparent,
	})
	if err := app.LoadString(appMenuUI, app); err != nil {
		log.Error("failed to load appmenu ui: %v", err)
		os.Exit(1)
	}

	if bg := app.e("BlurLayer"); bg != nil {
		bg.SetAttribute("src", resolveBlurImage())
	}

	app.loadCatalog()
	app.rebuildGrid("")

	app.Configure(func(core *gapp.App) {
		core.OnEscape = app.Quit
	})

	search := app.e("SearchInput")
	if search != nil {
		app.AddFocusable(search)
		app.Focus(search)
	}

	if err := app.Run(); err != nil {
		log.Error("appmenu error: %v", err)
		os.Exit(1)
	}
}
