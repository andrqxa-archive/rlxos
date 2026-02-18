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
	"fmt"
	"log"

	gapp "avyos.dev/pkg/graphics/app"
	"avyos.dev/pkg/graphics/ui"
)

//go:embed ui/demo.ui
var demoUI string

type DemoApp struct {
	ui.App
	clickCount   int
	optionCursor int
}

func (a *DemoApp) setStatus(msg string) {
	if el := a.FindElement("statusLabel"); el != nil {
		el.SetAttribute("text", msg)
	}
}

func (a *DemoApp) HandlePrimary() {
	a.clickCount++
	a.setStatus(fmt.Sprintf("Primary button clicked %d times", a.clickCount))
}

func (a *DemoApp) HandleTools() {
	a.setStatus("Tools menu clicked")
}

func (a *DemoApp) HandleQuit() {
	a.Quit()
}

func (a *DemoApp) HandleToggleChanged(checked bool) {
	if checked {
		a.setStatus("Quick toggle enabled")
		return
	}
	a.setStatus("Quick toggle disabled")
}

func (a *DemoApp) HandleCheckboxChange(checked bool) {
	a.setStatus(fmt.Sprintf("Checkbox: %v", checked))
}

func (a *DemoApp) HandleRadioA(checked bool) {
	if !checked {
		return
	}
	a.FindElement("radioOptionB").SetAttribute("checked", "false")
	a.FindElement("radioOptionC").SetAttribute("checked", "false")
	a.setStatus("Selected Option 1")
}

func (a *DemoApp) HandleRadioB(checked bool) {
	if !checked {
		return
	}
	a.FindElement("radioOptionA").SetAttribute("checked", "false")
	a.FindElement("radioOptionC").SetAttribute("checked", "false")
	a.setStatus("Selected Option 2")
}

func (a *DemoApp) HandleRadioC(checked bool) {
	if !checked {
		return
	}
	a.FindElement("radioOptionA").SetAttribute("checked", "false")
	a.FindElement("radioOptionB").SetAttribute("checked", "false")
	a.setStatus("Selected Option 3")
}

func (a *DemoApp) HandleSliderChange(val float64) {
	a.FindElement("progressBar").SetAttribute("value", fmt.Sprintf("%f", val/100.0))
	a.FindElement("progressBarAlt").SetAttribute("value", fmt.Sprintf("%f", val/100.0))
	a.setStatus(fmt.Sprintf("Progress: %.0f%%", val))
}

func (a *DemoApp) HandleInputChange(text string) {
	a.setStatus(fmt.Sprintf("Input: %s", text))
}

func (a *DemoApp) HandleInputSubmit(text string) {
	a.setStatus(fmt.Sprintf("Submitted: %s", text))
}

func (a *DemoApp) SelectListItem(idx int, label string) {
	a.setStatus(fmt.Sprintf("Selected %s (%d)", label, idx+1))
}

func (a *DemoApp) CycleOption() {
	opts := []string{"Option 1", "Option 2", "Option 3"}
	a.optionCursor = (a.optionCursor + 1) % len(opts)
	a.FindElement("selectButton").SetAttribute("text", opts[a.optionCursor])
	a.setStatus("Layout option changed")
}

func (a *DemoApp) DialogOpen() {
	a.setStatus(fmt.Sprintf("Open dialog: %s", a.FindElement("dialogTitleInput").Attr("text", "Dialog Title")))
}

func (a *DemoApp) DialogCancel() {
	a.setStatus("Dialog canceled")
}

func (a *DemoApp) DialogOK() {
	a.setStatus("Dialog confirmed")
}

func main() {
	
	app := &DemoApp{}
	app.SetOptions(gapp.Options{Title: "Demo App"})
	if err := app.LoadString(demoUI, app); err != nil {
		log.Fatalf("Failed to load UI: %v", err)
	}
	if inputEl := app.FindElement("textInput"); inputEl != nil {
		app.Focus(inputEl)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
	fmt.Println("Goodbye!")
}
