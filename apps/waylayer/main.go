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
	_ "embed"
	"flag"
	"fmt"
	"log"

	gapp "avyos.dev/pkg/graphics/app"
	"avyos.dev/pkg/graphics/ui"
)

var (
	flagDaemon bool
)

//go:embed ui/waylayer.ui
var waylayerUI string

type waylayerInfoApp struct {
	ui.App
}

func (a *waylayerInfoApp) QuitWindow() {
	a.Quit()
}

func main() {
	flag.BoolVar(&flagDaemon, "daemon", false, "Run as Wayland translation daemon")
	flag.Parse()

	if flagDaemon {
		runDaemon()
		return
	}
	if err := runInfoWindow(); err != nil {
		log.Fatal(err)
	}
}

func runDaemon() {
	server := NewServer()
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}

func runInfoWindow() error {
	app := &waylayerInfoApp{}
	app.SetOptions(gapp.Options{
		Title:  "Waylayer",
		Width:  900,
		Height: 720,
	})
	if err := app.LoadString(waylayerUI, app); err != nil {
		return fmt.Errorf("load waylayer info UI: %w", err)
	}

	if summary := app.FindElement("SummaryText"); summary != nil {
		summary.SetAttribute("text",
			"Waylayer is a Wayland to display translation layer.\n"+
				"Use --daemon in user sessions (cmd/session) to run the socket bridge.\n"+
				"Default mode shows this protocol compatibility window.",
		)
	}

	setInfoList(app, "GlobalList", implementedGlobals())
	setInfoList(app, "ObjectList", implementedObjects())
	setInfoList(app, "EventList", implementedEvents())

	return app.Run()
}

func setInfoList(app *waylayerInfoApp, id string, items []string) {
	container := app.FindElement(id)
	if container == nil {
		return
	}
	container.ClearChildren()
	for _, item := range items {
		row := ui.NewElement("Label")
		row.SetAttribute("text", "• "+item)
		container.AddChild(row)
	}
}

func implementedGlobals() []string {
	return []string{
		fmt.Sprintf("%s v%d", ifaceWlCompositor, versionWlCompositor),
		fmt.Sprintf("%s v%d", ifaceWlSubcomp, versionWlSubcomp),
		fmt.Sprintf("%s v%d", ifaceWlDataDevMgr, versionWlDataDevMgr),
		fmt.Sprintf("%s v%d", ifaceWlOutput, versionWlOutput),
		fmt.Sprintf("%s v%d", ifaceWlShm, versionWlShm),
		fmt.Sprintf("%s v%d", ifaceWlSeat, versionWlSeat),
		fmt.Sprintf("%s v%d", ifaceXdgWmBase, versionXdgWmBase),
	}
}

func implementedObjects() []string {
	return []string{
		"wl_surface, wl_region, wl_subsurface",
		"wl_shm_pool and wl_buffer (ARGB8888, XRGB8888)",
		"xdg_surface and xdg_toplevel",
		"xdg_positioner and xdg_popup",
		"wl_seat pointer and keyboard",
		"wl_data_device_manager basic object lifecycle",
	}
}

func implementedEvents() []string {
	return []string{
		"wl_pointer: enter, leave, motion, button, frame",
		"wl_keyboard: keymap, enter, leave, key, modifiers",
		"xdg_toplevel: configure, close",
		"xdg_popup: configure, popup_done, repositioned",
		"xdg_surface: configure",
		"wl_output: geometry, mode, scale, name, description, done",
		"wl_buffer: release and wl_callback.done on frame callbacks",
	}
}
