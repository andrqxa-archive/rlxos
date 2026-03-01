package main

import (
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	display "avyos.dev/api/display"
	gapp "avyos.dev/pkg/graphics/app"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
	core "avyos.dev/pkg/graphics/pixmap"
	ui "avyos.dev/pkg/graphics/widget/engine"
	"avyos.dev/pkg/identity"
)

//go:embed ui/oobe.ui
var oobeUI string

type OobeApp struct{ gapp.App }

func (a *OobeApp) e(id string) *ui.Element { return a.FindElement(id) }

func (a *OobeApp) showStage(stage int) {
	active := stage - 1
	if active < 0 {
		active = 0
	}
	if active > 2 {
		active = 2
	}

	if stack := a.e("StageStack"); stack != nil {
		stack.SetAttribute("active", active)
	}

	// Keep only the active stack page visible so min-size calculation does not
	// include inactive pages before the first layout pass.
	if stageWelcome := a.e("StageWelcome"); stageWelcome != nil {
		stageWelcome.SetVisible(active == 0)
	}
	if stageCreateUser := a.e("StageCreateUser"); stageCreateUser != nil {
		stageCreateUser.SetVisible(active == 1)
	}
	if stageInfo := a.e("StageInfo"); stageInfo != nil {
		stageInfo.SetVisible(active == 2)
	}

	switch stage {
	case 1:
		if btn := a.e("WelcomeContinueBtn"); btn != nil {
			a.Focus(btn)
		}
	case 2:
		if input := a.e("NameInput"); input != nil {
			a.Focus(input)
		}
	case 3:
		if btn := a.e("FinishBtn"); btn != nil {
			a.Focus(btn)
		}
	}
	a.relayoutRoot()
	a.Redraw()
}

func (a *OobeApp) relayoutRoot() {
	root := a.UIRoot()
	if root == nil {
		return
	}
	w, h := a.App.Size()
	if w < 2 || h < 1 {
		return
	}
	root.SetBounds(core.RectXYWH(0, 0, w-1, h))
	root.SetBounds(core.RectXYWH(0, 0, w, h))
}

func (a *OobeApp) setCreateStatus(msg string) {
	if lbl := a.e("CreateStatus"); lbl != nil {
		lbl.SetAttribute("text", msg)
	}
}

func (a *OobeApp) text(id string) string {
	el := a.e(id)
	if el == nil {
		return ""
	}
	return strings.TrimSpace(el.Attr("text", ""))
}

func (a *OobeApp) WelcomeContinue() { a.showStage(2) }

func (a *OobeApp) BackToWelcome() { a.showStage(1) }

func (a *OobeApp) CreateUser() {
	if err := ensureSecurityBootstrapFiles(); err != nil {
		a.setCreateStatus(fmt.Sprintf("Failed to prepare security config: %v", err))
		return
	}

	name := a.text("NameInput")
	pass := a.text("PasswordInput")
	repeat := a.text("RepeatPasswordInput")

	if name == "" {
		a.setCreateStatus("Name is required")
		return
	}
	if pass == "" {
		a.setCreateStatus("Password is required")
		return
	}
	if pass != repeat {
		a.setCreateStatus("Passwords do not match")
		return
	}
	if _, err := identity.LookupByName(name); err == nil {
		a.setCreateStatus("User already exists")
		return
	}

	id := identity.Identity{
		Name:  name,
		Home:  filepath.Join("/users", name),
		Shell: "/avyos/cmd/shell",
	}

	if err := identity.AddIdentity(id, "user"); err != nil {
		a.setCreateStatus(fmt.Sprintf("Failed to create user: %v", err))
		return
	}
	if err := identity.UpdatePassword(name, "", pass); err != nil {
		a.setCreateStatus(fmt.Sprintf("User created, password setup failed: %v", err))
		return
	}
	if err := os.MkdirAll(id.Home, 0755); err != nil {
		a.setCreateStatus(fmt.Sprintf("Failed to create home directory: %v", err))
		return
	}
	if err := os.Chown(id.Home, int(id.ID), int(id.ID)); err != nil {
		a.setCreateStatus(fmt.Sprintf("Failed to set permissions to home directory: %v", err))
		return
	}

	a.setCreateStatus("")
	a.showStage(3)
}

func (a *OobeApp) Finish() {
	os.WriteFile("/config/.oobe-done", []byte(``), 0644)
	a.Quit()
}

func ensureSecurityBootstrapFiles() error {
	if err := os.MkdirAll("/config/security", 0755); err != nil {
		return err
	}
	if err := copyFileIfMissing("/config/security/identity.conf", "/avyos/config/security/identity.conf", 0644); err != nil {
		return err
	}
	if err := copyFileIfMissing("/config/security/auth.conf", "/avyos/config/security/auth.conf", 0600); err != nil {
		return err
	}
	return nil
}

func copyFileIfMissing(dst, src string, mode os.FileMode) error {
	if st, err := os.Stat(dst); err == nil && !st.IsDir() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func main() {
	backend := displaybackend.New()
	backend.SetLayer(display.LayerOverlay, display.AnchorBottom|display.AnchorLeft|display.AnchorRight|display.AnchorTop, 0)

	app := &OobeApp{}
	app.SetOptions(gapp.Options{Title: "Welcome", Backend: backend, Input: backend})
	if err := app.LoadString(oobeUI, app); err != nil {
		log.Fatalf("Failed to load UI: %v", err)
	}
	if bg := app.FindElement("BackgroundImage"); bg != nil {
		bg.SetAttribute("src", "/avyos/data/backgrounds/default_blur.png")
	}
	if logo := app.e("LogoImage"); logo != nil {
		logo.SetAttribute("src", "/avyos/data/icons/logo/logo.png")
	}

	focusables := []*ui.Element{
		app.e("WelcomeContinueBtn"),
		app.e("NameInput"),
		app.e("PasswordInput"),
		app.e("RepeatPasswordInput"),
		app.e("CreateUserBtn"),
		app.e("BackBtn"),
		app.e("FinishBtn"),
	}
	for _, f := range focusables {
		if f != nil {
			app.AddFocusable(f)
		}
	}
	app.showStage(1)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
