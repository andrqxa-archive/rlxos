package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	gapp "avyos.dev/pkg/graphics/app"
	ui "avyos.dev/pkg/graphics/widget/engine"
)

//go:embed ui/notepad.ui
var notepadUI string

type NotepadApp struct {
	gapp.App

	currentPath string
	savedText   string
	dirty       bool
	syncingText bool
}

func (a *NotepadApp) e(id string) *ui.Element { return a.FindElement(id) }

func (a *NotepadApp) setText(id, text string) {
	if el := a.e(id); el != nil {
		el.SetAttribute("text", text)
	}
}

func (a *NotepadApp) pathInput() string {
	in := a.e("PathInput")
	if in == nil {
		return ""
	}
	return strings.TrimSpace(in.Attr("text", ""))
}

func (a *NotepadApp) editorText() string {
	editor := a.e("Editor")
	if editor == nil {
		return ""
	}
	return editor.Attr("text", "")
}

func (a *NotepadApp) setEditorText(text string) {
	editor := a.e("Editor")
	if editor == nil {
		return
	}
	a.syncingText = true
	editor.SetAttribute("text", text)
	a.syncingText = false
}

func (a *NotepadApp) resolvePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
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

func (a *NotepadApp) documentPath() string {
	if p := a.pathInput(); p != "" {
		return a.resolvePath(p)
	}
	if a.currentPath != "" {
		return a.currentPath
	}
	return ""
}

func (a *NotepadApp) shortPath(path string, max int) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "(untitled)"
	}
	r := []rune(path)
	if len(r) <= max || max < 4 {
		return path
	}
	return "..." + string(r[len(r)-max+3:])
}

func countLines(text string) int {
	if text == "" {
		return 1
	}
	return strings.Count(text, "\n") + 1
}

func (a *NotepadApp) refreshStatus(prefix string) {
	text := a.editorText()
	chars := utf8.RuneCountInString(text)
	lines := countLines(text)
	state := "saved"
	if a.dirty {
		state = "modified"
	}
	doc := a.shortPath(a.currentPath, 48)
	status := fmt.Sprintf("%s | %d line(s), %d char(s) | %s", doc, lines, chars, state)
	if strings.TrimSpace(prefix) != "" {
		status = prefix + " | " + status
	}
	a.setText("Status", status)
}

func (a *NotepadApp) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(data)
	if !utf8.Valid(data) {
		text = string(bytes.ToValidUTF8(data, []byte("?")))
	}

	a.setEditorText(text)
	a.currentPath = path
	a.savedText = text
	a.dirty = false
	a.setText("PathInput", path)
	a.refreshStatus(fmt.Sprintf("Opened: %s", a.shortPath(path, 42)))
	return nil
}

func (a *NotepadApp) saveFile(path string) error {
	path = a.resolvePath(path)
	if path == "" {
		return fmt.Errorf("enter a file path")
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	text := a.editorText()
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		return err
	}

	a.currentPath = path
	a.savedText = text
	a.dirty = false
	a.setText("PathInput", path)
	a.refreshStatus(fmt.Sprintf("Saved: %s", a.shortPath(path, 42)))
	return nil
}

func (a *NotepadApp) Open() {
	path := a.documentPath()
	if path == "" {
		a.refreshStatus("Enter a file path to open")
		return
	}
	if err := a.loadFile(path); err != nil {
		a.refreshStatus(fmt.Sprintf("Open failed: %v", err))
		return
	}
}

func (a *NotepadApp) Save() {
	path := a.documentPath()
	if path == "" {
		a.refreshStatus("Set a path then Save")
		return
	}
	if err := a.saveFile(path); err != nil {
		a.refreshStatus(fmt.Sprintf("Save failed: %v", err))
		return
	}
}

func (a *NotepadApp) SaveAs() {
	path := a.pathInput()
	if path == "" {
		a.refreshStatus("Enter a destination path for Save As")
		return
	}
	if err := a.saveFile(path); err != nil {
		a.refreshStatus(fmt.Sprintf("Save As failed: %v", err))
		return
	}
}

func (a *NotepadApp) Reload() {
	path := a.documentPath()
	if path == "" {
		a.refreshStatus("No file to reload")
		return
	}
	if err := a.loadFile(path); err != nil {
		a.refreshStatus(fmt.Sprintf("Reload failed: %v", err))
		return
	}
	a.refreshStatus(fmt.Sprintf("Reloaded: %s", a.shortPath(path, 42)))
}

func (a *NotepadApp) Clear() {
	a.currentPath = ""
	a.savedText = ""
	a.dirty = false
	a.setText("PathInput", "")
	a.setEditorText("")
	a.refreshStatus("New document")
}

func (a *NotepadApp) PathSubmitted(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	a.Open()
}

func (a *NotepadApp) EditorChanged(text string) {
	if a.syncingText {
		return
	}
	a.dirty = text != a.savedText
	a.refreshStatus("")
}

func startupPathArg() string {
	if len(os.Args) < 2 {
		return ""
	}
	return strings.TrimSpace(os.Args[1])
}

func (a *NotepadApp) openStartupPath(path string) {
	path = a.resolvePath(path)
	if path == "" {
		return
	}

	if info, err := os.Stat(path); err == nil && info.IsDir() {
		a.refreshStatus("Startup path is a directory")
		return
	}

	if err := a.loadFile(path); err != nil {
		if os.IsNotExist(err) {
			a.currentPath = path
			a.savedText = a.editorText()
			a.dirty = false
			a.setText("PathInput", path)
			a.refreshStatus(fmt.Sprintf("New file: %s", a.shortPath(path, 42)))
			return
		}
		a.refreshStatus(fmt.Sprintf("Startup open failed: %v", err))
	}
}

func main() {
	app := &NotepadApp{}
	app.SetOptions(gapp.Options{Title: "Notepad"})
	if err := app.LoadString(notepadUI, app); err != nil {
		log.Fatalf("Failed to load UI: %v", err)
	}

	app.AddFocusable(
		app.e("PathInput"),
		app.e("OpenBtn"),
		app.e("SaveBtn"),
		app.e("SaveAsBtn"),
		app.e("ReloadBtn"),
		app.e("ClearBtn"),
		app.e("Editor"),
	)
	app.Focus(app.e("Editor"))
	app.refreshStatus("Ready")
	if path := startupPathArg(); path != "" {
		app.openStartupPath(path)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Notepad error: %v", err)
	}
}
