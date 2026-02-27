package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	gapp "avyos.dev/pkg/graphics/app"
	declapp "avyos.dev/pkg/graphics/app/decl"
	gfxcanvas "avyos.dev/pkg/graphics/canvas"
	ui "avyos.dev/pkg/graphics/widget/engine"
)

//go:embed ui/imageviewer.ui
var imageViewerUI string

type ImageViewerApp struct {
	declapp.App

	currentPath string
}

func (a *ImageViewerApp) e(id string) *ui.Element { return a.FindElement(id) }

func (a *ImageViewerApp) setText(id, text string) {
	if el := a.e(id); el != nil {
		el.SetAttribute("text", text)
	}
}

func (a *ImageViewerApp) setStatus(format string, args ...interface{}) {
	a.setText("Status", fmt.Sprintf(format, args...))
}

func (a *ImageViewerApp) pathInput() string {
	if in := a.e("PathInput"); in != nil {
		return strings.TrimSpace(in.Attr("text", ""))
	}
	return ""
}

func (a *ImageViewerApp) resolvePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			path = home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			path = filepath.Join(home, path[2:])
		}
	}
	if !filepath.IsAbs(path) {
		if cwd, err := os.Getwd(); err == nil && cwd != "" {
			path = filepath.Join(cwd, path)
		}
	}
	return filepath.Clean(path)
}

func (a *ImageViewerApp) openPath(path string) error {
	path = a.resolvePath(path)
	if path == "" {
		return fmt.Errorf("enter an image path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory")
	}

	buf, err := gfxcanvas.DecodeImageToBuffer(path, 0)
	if err != nil {
		return err
	}
	if buf == nil {
		return fmt.Errorf("unsupported image")
	}

	if img := a.e("ViewerImage"); img != nil {
		img.SetAttribute("src", path)
	}
	a.currentPath = path
	a.setText("PathInput", path)
	a.setStatus("Opened %s (%dx%d)", filepath.Base(path), buf.Width, buf.Height)
	return nil
}

func (a *ImageViewerApp) Open() {
	path := a.pathInput()
	if path == "" {
		path = a.currentPath
	}
	if err := a.openPath(path); err != nil {
		a.setStatus("Open failed: %v", err)
	}
}

func (a *ImageViewerApp) PathSubmitted(_ string) { a.Open() }

func startupImageArg() string {
	if len(os.Args) < 2 {
		return ""
	}
	return strings.TrimSpace(os.Args[1])
}

func main() {
	app := &ImageViewerApp{}
	app.SetOptions(gapp.Options{Title: "Image Viewer"})
	if err := app.LoadString(imageViewerUI, app); err != nil {
		log.Fatalf("Failed to load image viewer UI: %v", err)
	}

	app.AddFocusable(
		app.e("PathInput"),
		app.e("OpenBtn"),
	)
	if in := app.e("PathInput"); in != nil {
		app.Focus(in)
	}

	if img := app.e("ViewerImage"); img != nil {
		img.SetAttribute("scaleMode", "stretch")
	}
	app.setStatus("Ready")
	if arg := startupImageArg(); arg != "" {
		if err := app.openPath(arg); err != nil {
			app.setStatus("Startup open failed: %v", err)
		}
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Image viewer error: %v", err)
	}
}
