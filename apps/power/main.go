package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"

	display "avyos.dev/api/display"
	"avyos.dev/pkg/fs"
	gapp "avyos.dev/pkg/graphics/app"
	declapp "avyos.dev/pkg/graphics/app/decl"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
	gfxicons "avyos.dev/pkg/graphics/icons"
	graphics "avyos.dev/pkg/graphics/input"
	ui "avyos.dev/pkg/graphics/widget/engine"
)

//go:embed ui/power.ui
var powerUI string

const iconSize = 64

type powerApp struct {
	declapp.App
}

func (a *powerApp) e(id string) *ui.Element { return a.FindElement(id) }

func (a *powerApp) setStatus(text string) {
	if el := a.e("Status"); el != nil {
		el.SetAttribute("text", text)
	}
}

func (a *powerApp) Dismiss() { a.Quit() }

func (a *powerApp) Lock() {
	cmd := exec.Command("/avyos/services/login")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Start()
	a.Quit()
}

func (a *powerApp) Logout() {
	if err := runAction("power", "logout"); err != nil {
		a.setStatus(fmt.Sprintf("Logout failed: %v", err))
		return
	}
	a.Quit()
}

func (a *powerApp) Reboot() {
	if err := runAction("power", "reboot"); err != nil {
		a.setStatus(fmt.Sprintf("Reboot failed: %v", err))
		return
	}
	a.Quit()
}

func (a *powerApp) Shutdown() {
	if err := runAction("power", "off"); err != nil {
		a.setStatus(fmt.Sprintf("Shutdown failed: %v", err))
		return
	}
	a.Quit()
}

func runAction(name string, args ...string) error {
	command := name
	if name == "power" {
		if resolved := fs.Resolve("cmd:power"); resolved != "" {
			if _, err := os.Stat(resolved); err == nil {
				command = resolved
			}
		}
	}
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Start()
}

func main() {
	backend := displaybackend.New()
	backend.SetLayer(display.LayerOverlay, display.AnchorTop|display.AnchorBottom|display.AnchorLeft|display.AnchorRight, 0)

	app := &powerApp{}
	app.SetOptions(gapp.Options{
		Title:      "Power",
		Backend:    backend,
		Input:      backend,
		Background: graphics.ColorTransparent,
	})
	if err := app.LoadString(powerUI, app); err != nil {
		log.Fatalf("Failed to load power UI: %v", err)
	}
	app.Configure(func(g *gapp.App) {
		g.OnEscape = app.Dismiss
	})

	if btn := app.e("LogoutBtn"); btn != nil {
		btn.SetAttribute("src", gfxicons.ResolvePath("lock_closed", iconSize))
		btn.SetAttribute("srcOpaque", false)
	}
	if btn := app.e("RebootBtn"); btn != nil {
		btn.SetAttribute("src", gfxicons.ResolvePath("reboot", iconSize))
		btn.SetAttribute("srcOpaque", false)
	}
	if btn := app.e("ShutdownBtn"); btn != nil {
		btn.SetAttribute("src", gfxicons.ResolvePath("power", iconSize))
		btn.SetAttribute("srcOpaque", false)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Power app error: %v", err)
	}
}
