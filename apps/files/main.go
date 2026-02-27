package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"avyos.dev/pkg/fs"
	gapp "avyos.dev/pkg/graphics/app"
	declapp "avyos.dev/pkg/graphics/app/decl"
	gfxfont "avyos.dev/pkg/graphics/font"
	gfxicons "avyos.dev/pkg/graphics/icons"
	ui "avyos.dev/pkg/graphics/widget/engine"
	"avyos.dev/pkg/identity"
)

//go:embed ui/filemanager.ui
var fileManagerUI string

const (
	driveSlotSize = 6
	iconTileWidth = 112
	iconTileHgt   = 112
	iconLabelW    = 96
)

type sortMode int

const (
	sortByName sortMode = iota
	sortBySize
	sortByModified
	sortByType
)

func (m sortMode) label() string {
	switch m {
	case sortBySize:
		return "Size"
	case sortByModified:
		return "Modified"
	case sortByType:
		return "Type"
	default:
		return "Name"
	}
}

func (m sortMode) next() sortMode {
	switch m {
	case sortByName:
		return sortBySize
	case sortBySize:
		return sortByModified
	case sortByModified:
		return sortByType
	default:
		return sortByName
	}
}

type fileEntry struct {
	name       string
	path       string
	isDir      bool
	isSymlink  bool
	linkTarget string
	size       int64
	modTime    time.Time
	mode       os.FileMode
}

type clipboardItem struct {
	path string
	cut  bool
}

type driveEntry struct {
	name string
	path string
}

type fileManager struct {
	declapp.App

	cwd        string
	home       string
	history    []string
	historyPos int

	entries  []fileEntry
	visible  []fileEntry
	selected string

	filter     string
	showHidden bool
	sortMode   sortMode
	sortDesc   bool

	clipboard *clipboardItem
	drives    []driveEntry

	iconView         bool
	lastIconSelect   string
	lastIconSelectAt time.Time
}

func (fm *fileManager) e(id string) *ui.Element { return fm.FindElement(id) }

func (fm *fileManager) setAttr(id, name string, value interface{}) bool {
	el := fm.e(id)
	if el == nil {
		return false
	}
	el.SetAttribute(name, value)
	return true
}

func (fm *fileManager) setText(id, text string) bool {
	return fm.setAttr(id, "text", text)
}

func (fm *fileManager) setVisible(id string, visible bool) bool {
	el := fm.e(id)
	if el == nil {
		return false
	}
	el.SetVisible(visible)
	return true
}

func (fm *fileManager) textOf(id string) string {
	el := fm.e(id)
	if el == nil {
		return ""
	}
	return el.Attr("text", "")
}

func main() {
	startDir, err := os.Getwd()
	if err != nil {
		startDir = "/"
	}
	startDir = filepath.Clean(startDir)

	home := resolveHomeDir(startDir)

	initialSelect := ""
	startupStatus := "Ready"
	if len(os.Args) > 1 {
		target := resolveInputPath(os.Args[1], startDir, home)
		if target != "" {
			info, statErr := os.Stat(target)
			if statErr != nil {
				startupStatus = fmt.Sprintf("Open target failed: %v", statErr)
			} else if info.IsDir() {
				startDir = target
			} else {
				startDir = filepath.Dir(target)
				initialSelect = target
			}
		}
	}

	ensureHomeLibraryDirs(home)

	fm := &fileManager{
		cwd:        startDir,
		home:       home,
		history:    []string{startDir},
		historyPos: 0,
		sortMode:   sortByName,
	}

	fm.SetOptions(gapp.Options{Title: "File Manager"})
	if err := fm.LoadString(fileManagerUI, fm); err != nil {
		log.Fatalf("Failed to load UI: %v", err)
	}

	fm.setText("Address", fm.cwd)
	fm.setText("FilterInput", "")
	fm.setAttr("HiddenToggle", "checked", false)
	fm.initSidebarIcons()
	fm.styleSidebarButtons()
	fm.setViewMode(true)
	fm.updateSortButtons()
	fm.updateClipboardLabel()
	fm.refreshDrives()
	fm.reload(initialSelect)
	fm.setStatus(startupStatus)
	if list := fm.e("FileList"); list != nil {
		list.BindSignal("secondaryClicked", func() { fm.openListContext() })
	}
	if list := fm.e("FileList"); list != nil {
		fm.Focus(list)
	}

	if err := fm.Run(); err != nil {
		log.Fatalf("File manager error: %v", err)
	}
}

func resolveHomeDir(fallback string) string {
	if id, err := identity.LookupByID(uint(os.Getuid())); err == nil {
		home := filepath.Clean(strings.TrimSpace(id.Home))
		if home != "" && !samePath(home, "/users") {
			return home
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		home = filepath.Clean(strings.TrimSpace(home))
		if home != "" && !samePath(home, "/users") {
			return home
		}
	}

	for _, key := range []string{"USER", "LOGNAME"} {
		name := strings.TrimSpace(os.Getenv(key))
		if name == "" {
			continue
		}
		candidate := filepath.Join("/users", name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	if fallback == "" {
		fallback = "/"
	}
	return filepath.Clean(fallback)
}

func resolveInputPath(raw, base, home string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if raw == "~" {
		return home
	}
	if strings.HasPrefix(raw, "~/") {
		raw = filepath.Join(home, raw[2:])
	}
	if !filepath.IsAbs(raw) {
		raw = filepath.Join(base, raw)
	}
	return filepath.Clean(raw)
}

func ensureHomeLibraryDirs(home string) {
	if home == "" {
		return
	}
	for _, name := range []string{"Documents", "Downloads", "Pictures", "Videos", "Desktop", "Movies", "Public"} {
		path := filepath.Join(home, name)
		info, err := os.Stat(path)
		if err == nil {
			if !info.IsDir() {
				log.Printf("filemanager: %s exists but is not a directory", path)
			}
			continue
		}
		if !os.IsNotExist(err) {
			log.Printf("filemanager: failed to check %s: %v", path, err)
			continue
		}
		if mkErr := os.MkdirAll(path, 0755); mkErr != nil {
			log.Printf("filemanager: failed to create %s: %v", path, mkErr)
		}
	}
}

func (fm *fileManager) GoBack() {
	if fm.historyPos <= 0 {
		fm.setStatus("No earlier history item")
		return
	}
	targetPos := fm.historyPos - 1
	if fm.navigateTo(fm.history[targetPos], false) {
		fm.historyPos = targetPos
		fm.setStatus("Moved back")
	}
}

func (fm *fileManager) GoForward() {
	if fm.historyPos >= len(fm.history)-1 {
		fm.setStatus("No newer history item")
		return
	}
	targetPos := fm.historyPos + 1
	if fm.navigateTo(fm.history[targetPos], false) {
		fm.historyPos = targetPos
		fm.setStatus("Moved forward")
	}
}

func (fm *fileManager) GoUp() {
	parent := filepath.Dir(fm.cwd)
	if samePath(parent, fm.cwd) {
		fm.setStatus("Already at filesystem root")
		return
	}
	fm.navigateTo(parent, true)
}

func (fm *fileManager) GoHome() {
	fm.navigateTo(fm.home, true)
}

func (fm *fileManager) Navigate(path string) {
	fm.navigateTo(path, true)
}

func (fm *fileManager) GoAddress() {
	fm.navigateTo(fm.textOf("Address"), true)
}

func (fm *fileManager) Refresh() {
	fm.refreshDrives()
	fm.reload(fm.selected)
	fm.setStatus("Refreshed")
}

func (fm *fileManager) RefreshDrives() {
	fm.refreshDrives()
	fm.setStatusf("Discovered %d drive(s)", len(fm.drives))
}

func (fm *fileManager) ToggleViewMode() {
	fm.setViewMode(!fm.iconView)
	fm.renderIconView(fm.selected)
	fm.updateSummary()
}

func (fm *fileManager) IconPrevPage() {
	fm.setStatus("Icon grid uses continuous scrolling")
}

func (fm *fileManager) IconNextPage() {
	fm.setStatus("Icon grid uses continuous scrolling")
}

func (fm *fileManager) FilterChanged(text string) {
	fm.filter = strings.TrimSpace(text)
	fm.applyView(fm.selected)
}

func (fm *fileManager) ToggleHidden(checked bool) {
	fm.showHidden = checked
	fm.applyView(fm.selected)
}

func (fm *fileManager) CycleSort() {
	fm.sortMode = fm.sortMode.next()
	fm.updateSortButtons()
	fm.applyView(fm.selected)
}

func (fm *fileManager) ToggleSortOrder() {
	fm.sortDesc = !fm.sortDesc
	fm.updateSortButtons()
	fm.applyView(fm.selected)
}

func (fm *fileManager) SelectEntry(idx int, _ string) {
	if idx < 0 || idx >= len(fm.visible) {
		return
	}
	entry := fm.visible[idx]
	now := time.Now()
	doubleClick := samePath(entry.path, fm.lastIconSelect) && now.Sub(fm.lastIconSelectAt) <= 500*time.Millisecond

	fm.selected = entry.path
	fm.lastIconSelect = entry.path
	fm.lastIconSelectAt = now
	fm.hideContextMenu()

	fm.updateSelectionView()
	fm.renderIconView(fm.selected)
	if doubleClick {
		fm.OpenSelected()
	}
}

func (fm *fileManager) OpenSelected() {
	fm.hideContextMenu()
	entry, ok := fm.selectedEntry()
	if !ok {
		fm.setStatus("No file or folder selected")
		return
	}
	if entry.isDir {
		fm.navigateTo(entry.path, true)
		return
	}
	if err := fm.openWithDefaultApp(entry.path); err != nil {
		fm.setStatusf("Open failed: %v", err)
		return
	}
	fm.setStatusf("Opened %s", entry.name)
}

func (fm *fileManager) CreateFolder() {
	fm.hideContextMenu()
	name := "New Folder"

	target := fm.resolvePath(name, fm.cwd)
	if target == "" {
		fm.setStatus("Folder name is required")
		return
	}
	target = fm.uniqueDestination(target)
	if err := os.Mkdir(target, 0755); err != nil {
		fm.setStatusf("Create folder failed: %v", err)
		return
	}

	fm.reload(target)
	fm.setStatusf("Created folder: %s", filepath.Base(target))
}

func (fm *fileManager) CreateFile() {
	fm.hideContextMenu()
	name := "New File.txt"

	target := fm.resolvePath(name, fm.cwd)
	if target == "" {
		fm.setStatus("File name is required")
		return
	}
	target = fm.uniqueDestination(target)

	f, err := os.OpenFile(target, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		fm.setStatusf("Create file failed: %v", err)
		return
	}
	_ = f.Close()

	fm.reload(target)
	fm.setStatusf("Created file: %s", filepath.Base(target))
}

func (fm *fileManager) RenameSelected() {
	fm.hideContextMenu()
	entry, ok := fm.selectedEntry()
	if !ok {
		fm.setStatus("No file or folder selected")
		return
	}
	raw := strings.TrimSpace(fm.textOf("ActionInput"))
	target := ""
	if raw == "" {
		target = fm.uniqueDestination(fm.defaultRenameTarget(entry.path))
	} else {
		target = fm.resolveRenameTarget(raw, entry.path)
	}
	if target == "" {
		fm.setStatus("Invalid rename target")
		return
	}
	if samePath(target, entry.path) {
		fm.setStatus("Rename target is unchanged")
		return
	}
	if exists(target) {
		fm.setStatus("Target already exists")
		return
	}

	if err := movePath(entry.path, target); err != nil {
		fm.setStatusf("Rename failed: %v", err)
		return
	}

	nextSel := ""
	if samePath(filepath.Dir(target), fm.cwd) {
		nextSel = target
	}
	fm.reload(nextSel)
	fm.setStatusf("Renamed to %s", filepath.Base(target))
}

func (fm *fileManager) DeleteSelected() {
	fm.hideContextMenu()
	entry, ok := fm.selectedEntry()
	if !ok {
		fm.setStatus("No file or folder selected")
		return
	}

	if err := removePath(entry.path); err != nil {
		fm.setStatusf("Delete failed: %v", err)
		return
	}
	fm.reload("")
	fm.setStatusf("Deleted %s", entry.name)
}

func (fm *fileManager) CopySelected() {
	entry, ok := fm.selectedEntry()
	if !ok {
		fm.setStatus("No file or folder selected")
		return
	}
	fm.clipboard = &clipboardItem{path: entry.path, cut: false}
	fm.updateClipboardLabel()
	fm.setStatusf("Copied %s to clipboard", entry.name)
}

func (fm *fileManager) CutSelected() {
	entry, ok := fm.selectedEntry()
	if !ok {
		fm.setStatus("No file or folder selected")
		return
	}
	fm.clipboard = &clipboardItem{path: entry.path, cut: true}
	fm.updateClipboardLabel()
	fm.setStatusf("Cut %s to clipboard", entry.name)
}

func (fm *fileManager) PasteClipboard() {
	if fm.clipboard == nil {
		fm.setStatus("Clipboard is empty")
		return
	}
	src := fm.clipboard.path
	if !exists(src) {
		fm.clipboard = nil
		fm.updateClipboardLabel()
		fm.setStatus("Clipboard source no longer exists")
		return
	}

	base := filepath.Base(src)
	dst := filepath.Join(fm.cwd, base)

	if fm.clipboard.cut {
		if samePath(src, dst) {
			fm.setStatus("Source is already in this directory")
			return
		}
		if exists(dst) {
			dst = fm.uniqueDestination(dst)
		}
		if err := movePath(src, dst); err != nil {
			fm.setStatusf("Move failed: %v", err)
			return
		}
		fm.clipboard = nil
		fm.updateClipboardLabel()
		fm.reload(dst)
		fm.setStatusf("Moved to %s", filepath.Base(dst))
		return
	}

	if samePath(src, dst) || exists(dst) {
		dst = fm.uniqueDestination(dst)
	}
	if err := copyPath(src, dst); err != nil {
		fm.setStatusf("Copy failed: %v", err)
		return
	}
	fm.reload(dst)
	fm.setStatusf("Copied to %s", filepath.Base(dst))
}

func (fm *fileManager) OpenPlaceHome() {
	fm.openPlace("Home", fm.home)
}

func (fm *fileManager) OpenPlaceDocuments() {
	fm.openPlace("Documents", filepath.Join(fm.home, "Documents"))
}

func (fm *fileManager) OpenPlaceDownloads() {
	fm.openPlace("Downloads", filepath.Join(fm.home, "Downloads"))
}

func (fm *fileManager) OpenPlacePictures() {
	fm.openPlace("Pictures", filepath.Join(fm.home, "Pictures"))
}

func (fm *fileManager) OpenPlaceVideos() {
	fm.openPlace("Videos", filepath.Join(fm.home, "Videos"))
}

func (fm *fileManager) OpenDrive0() { fm.openDriveSlot(0) }
func (fm *fileManager) OpenDrive1() { fm.openDriveSlot(1) }
func (fm *fileManager) OpenDrive2() { fm.openDriveSlot(2) }
func (fm *fileManager) OpenDrive3() { fm.openDriveSlot(3) }
func (fm *fileManager) OpenDrive4() { fm.openDriveSlot(4) }
func (fm *fileManager) OpenDrive5() { fm.openDriveSlot(5) }

func (fm *fileManager) openPlace(label, path string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(path, 0755); mkErr == nil {
				info, err = os.Stat(path)
			} else {
				fm.setStatusf("Cannot create %s folder: %v", label, mkErr)
				return
			}
		}
	}
	if err != nil || info == nil || !info.IsDir() {
		fm.setStatusf("%s folder not available: %s", label, path)
		return
	}
	fm.navigateTo(path, true)
}

func (fm *fileManager) openDriveSlot(slot int) {
	if slot < 0 || slot >= len(fm.drives) {
		fm.setStatus("Drive slot is empty")
		return
	}
	d := fm.drives[slot]
	fm.navigateTo(d.path, true)
}

func (fm *fileManager) openWithDefaultApp(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	path = filepath.Clean(path)

	command := fs.Resolve("cmd:open")
	if _, err := os.Stat(command); err != nil {
		command = "open"
	}

	cmd := exec.Command(command, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch open command: %w", err)
	}
	return nil
}

func (fm *fileManager) openListContext() {
	list := fm.e("FileList")
	if list != nil {
		idx := list.AttrInt("selected", -1)
		if idx >= 0 && idx < len(fm.visible) {
			fm.selected = fm.visible[idx].path
		}
	}
	fm.updateSelectionView()
	fm.renderIconView(fm.selected)
	fm.showContextMenu()
}

func (fm *fileManager) showContextMenu() {
	label := fmt.Sprintf("Actions: %s", shortPath(fm.cwd, 42))
	if entry, ok := fm.selectedEntry(); ok {
		label = fmt.Sprintf("Actions: %s", shortPath(entry.name, 42))
	}
	fm.setText("ContextLabel", label)
	fm.setVisible("ContextMenu", true)
	fm.Redraw()
}

func (fm *fileManager) hideContextMenu() {
	fm.setVisible("ContextMenu", false)
	fm.Redraw()
}

func (fm *fileManager) ContextRename() {
	fm.RenameSelected()
}

func (fm *fileManager) ContextNewFolder() {
	fm.CreateFolder()
}

func (fm *fileManager) ContextNewFile() {
	fm.CreateFile()
}

func (fm *fileManager) ContextRefresh() {
	fm.Refresh()
}

func (fm *fileManager) ContextDelete() {
	fm.DeleteSelected()
}

func (fm *fileManager) navigateTo(path string, push bool) bool {
	fm.hideContextMenu()
	if path == "" {
		fm.setStatus("Path cannot be empty")
		return false
	}

	clean := fm.resolvePath(path, fm.cwd)
	if clean == "" {
		fm.setStatus("Invalid path")
		return false
	}

	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		fm.setStatusf("Invalid directory: %s", clean)
		return false
	}

	if clean == fm.cwd {
		fm.setText("Address", clean)
		fm.reload(fm.selected)
		return true
	}

	fm.cwd = clean
	fm.setText("Address", clean)
	if push {
		fm.pushHistory(clean)
	}
	fm.reload("")
	fm.setStatusf("Opened %s", clean)
	return true
}

func (fm *fileManager) pushHistory(path string) {
	if fm.historyPos >= 0 && fm.historyPos < len(fm.history) && samePath(fm.history[fm.historyPos], path) {
		return
	}
	if fm.historyPos < len(fm.history)-1 {
		fm.history = fm.history[:fm.historyPos+1]
	}
	fm.history = append(fm.history, path)
	fm.historyPos = len(fm.history) - 1
}

func (fm *fileManager) reload(selectPath string) {
	fm.hideContextMenu()
	fm.setText("PathLabel", fm.cwd)
	entries, err := listDir(fm.cwd)
	if err != nil {
		fm.entries = nil
		fm.visible = nil
		fm.selected = ""
		fm.setAttr("FileList", "items", "")
		fm.setAttr("FileList", "selected", -1)
		fm.renderIconView("")
		fm.setStatusf("Error: %v", err)
		return
	}
	fm.entries = entries
	fm.applyView(selectPath)
}

func (fm *fileManager) applyView(selectPath string) {
	fm.visible = fm.filteredAndSorted()
	fm.renderList(selectPath)
	fm.renderIconView(selectPath)
	fm.updateSummary()
}

func (fm *fileManager) filteredAndSorted() []fileEntry {
	filter := strings.ToLower(strings.TrimSpace(fm.filter))
	out := make([]fileEntry, 0, len(fm.entries))
	for _, entry := range fm.entries {
		if !fm.showHidden && strings.HasPrefix(entry.name, ".") {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(entry.name), filter) {
			continue
		}
		out = append(out, entry)
	}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.isDir != b.isDir {
			return a.isDir
		}

		cmp := 0
		switch fm.sortMode {
		case sortBySize:
			if a.size < b.size {
				cmp = -1
			} else if a.size > b.size {
				cmp = 1
			}
		case sortByModified:
			if a.modTime.Before(b.modTime) {
				cmp = -1
			} else if a.modTime.After(b.modTime) {
				cmp = 1
			}
		case sortByType:
			atype := strings.ToLower(filepath.Ext(a.name))
			btype := strings.ToLower(filepath.Ext(b.name))
			if a.isDir {
				atype = "dir"
			}
			if b.isDir {
				btype = "dir"
			}
			switch {
			case atype < btype:
				cmp = -1
			case atype > btype:
				cmp = 1
			}
		default:
		}

		if cmp == 0 {
			an := strings.ToLower(a.name)
			bn := strings.ToLower(b.name)
			switch {
			case an < bn:
				cmp = -1
			case an > bn:
				cmp = 1
			}
		}

		if fm.sortDesc {
			return cmp > 0
		}
		return cmp < 0
	})

	return out
}

func (fm *fileManager) renderList(selectPath string) {
	if selectPath == "" {
		selectPath = fm.selected
	}

	list := fm.e("FileList")
	if list == nil {
		fm.selected = ""
		fm.updateSelectionView()
		return
	}
	if len(fm.visible) == 0 {
		list.SetAttribute("items", "")
		list.SetAttribute("selected", -1)
		fm.selected = ""
		fm.updateSelectionView()
		return
	}

	items := make([]string, 0, len(fm.visible))
	selectedIndex := -1
	for i, entry := range fm.visible {
		items = append(items, fm.formatListItem(entry))
		if selectPath != "" && samePath(entry.path, selectPath) {
			selectedIndex = i
		}
	}
	if selectedIndex < 0 {
		selectedIndex = 0
	}

	list.SetAttribute("items", strings.Join(items, "|"))
	list.SetAttribute("selected", selectedIndex)
	fm.selected = fm.visible[selectedIndex].path
	fm.updateSelectionView()
}

func (fm *fileManager) renderIconView(selectPath string) {
	if selectPath == "" {
		selectPath = fm.selected
	}

	grid := fm.e("IconGrid")
	if grid == nil {
		return
	}
	grid.ClearChildren()

	if len(fm.visible) == 0 {
		fm.setText("IconPageLabel", "0 item(s)")
		return
	}

	for _, entry := range fm.visible {
		btn := fm.buildIconTile(entry)
		if btn == nil {
			continue
		}
		fm.styleIconTile(btn, selectPath != "" && samePath(entry.path, selectPath))
		grid.AddChild(btn)
	}

	fm.setText("IconPageLabel", fmt.Sprintf("%d item(s)", len(fm.visible)))
}

func (fm *fileManager) buildIconTile(entry fileEntry) *ui.Element {
	btn := ui.NewElement("Button")
	btn.SetAttribute("minWidth", iconTileWidth)
	btn.SetAttribute("maxWidth", iconTileWidth)
	btn.SetAttribute("minHeight", iconTileHgt)
	btn.SetAttribute("maxHeight", iconTileHgt)
	btn.SetAttribute("padding", "6 8 8 8")
	btn.SetAttribute("expand", false)
	btn.SetAttribute("textAlign", "center")
	btn.SetAttribute("borderRadius", 10)
	path := entry.path
	btn.BindSignal("clicked", func() { fm.iconTileClicked(path) })
	btn.BindSignal("secondaryClicked", func() { fm.iconTileSecondaryClicked(path) })

	box := ui.NewElement("VBox")
	box.SetAttribute("direction", "column")
	box.SetAttribute("spacing", 0)
	box.SetAttribute("alignment", "center")
	box.SetAttribute("expand", true)

	topGap := ui.NewElement("Spacer")
	topGap.SetAttribute("expand", true)
	topGap.SetAttribute("flex", 2)
	topGap.SetAttribute("minHeight", 1)

	img := ui.NewElement("Image")
	img.SetAttribute("src", fm.iconSrcForEntry(entry, 80))
	img.SetAttribute("minWidth", 56)
	img.SetAttribute("minHeight", 56)
	img.SetAttribute("maxWidth", 56)
	img.SetAttribute("maxHeight", 56)
	img.SetAttribute("expand", false)

	bottomGap := ui.NewElement("Spacer")
	bottomGap.SetAttribute("expand", true)
	bottomGap.SetAttribute("flex", 1)
	bottomGap.SetAttribute("minHeight", 1)

	lbl := ui.NewElement("Label")
	lbl.SetAttribute("text", ellipsisText(entry.name, iconLabelW))
	lbl.SetAttribute("minWidth", iconLabelW)
	lbl.SetAttribute("maxWidth", iconLabelW)
	lbl.SetAttribute("textAlign", "center")
	lbl.SetAttribute("expand", false)

	box.AddChild(topGap)
	box.AddChild(img)
	box.AddChild(bottomGap)
	box.AddChild(lbl)
	btn.AddChild(box)
	return btn
}

func (fm *fileManager) iconTileClicked(path string) {
	entry, ok := fm.entryByPath(path)
	if !ok {
		return
	}

	now := time.Now()
	doubleClick := samePath(path, fm.lastIconSelect) && now.Sub(fm.lastIconSelectAt) <= 500*time.Millisecond

	fm.selected = path
	fm.lastIconSelect = path
	fm.lastIconSelectAt = now
	fm.hideContextMenu()

	fm.updateSelectionView()
	fm.renderList(path)
	fm.renderIconView(path)

	if doubleClick {
		fm.OpenSelected()
		return
	}
	fm.setStatusf("Selected %s", entry.name)
}

func (fm *fileManager) iconTileSecondaryClicked(path string) {
	entry, ok := fm.entryByPath(path)
	if !ok {
		return
	}
	fm.selected = path
	fm.lastIconSelect = path
	fm.lastIconSelectAt = time.Now()
	fm.updateSelectionView()
	fm.renderList(path)
	fm.renderIconView(path)
	fm.setStatusf("Selected %s", entry.name)
	fm.showContextMenu()
}

func (fm *fileManager) entryByPath(path string) (fileEntry, bool) {
	for _, entry := range fm.visible {
		if samePath(entry.path, path) {
			return entry, true
		}
	}
	return fileEntry{}, false
}

func (fm *fileManager) styleIconTile(btn *ui.Element, selected bool) {
	if btn == nil {
		return
	}
	if selected {
		btn.SetAttribute("background", "theme.color.accent.subtle")
		btn.SetAttribute("hoverBackground", "theme.color.accent.subtle")
		btn.SetAttribute("pressedBackground", "theme.color.accent.subtle")
		btn.SetAttribute("borderColor", "theme.color.accent")
		btn.SetAttribute("focusedBorderColor", "theme.color.accent")
		btn.SetAttribute("shadow", "true")
		btn.SetAttribute("shadowOnlyOnHover", "false")
		btn.SetAttribute("shadowColor", "theme.color.accent.subtle")
		btn.SetAttribute("shadowSpread", "10")
		return
	}
	btn.SetAttribute("background", "theme.color.surface.glass")
	btn.SetAttribute("hoverBackground", "theme.color.control.hover")
	btn.SetAttribute("pressedBackground", "theme.color.control.pressed")
	btn.SetAttribute("borderColor", "theme.color.stroke.hairline")
	btn.SetAttribute("focusedBorderColor", "theme.color.stroke.hairline")
	btn.SetAttribute("shadow", "true")
	btn.SetAttribute("shadowOnlyOnHover", "true")
	btn.SetAttribute("shadowColor", "theme.color.stroke.divider")
	btn.SetAttribute("shadowSpread", "8")
	btn.SetAttribute("shadowOffsetY", "3")
}

func (fm *fileManager) setViewMode(icon bool) {
	fm.iconView = icon
	active := 0
	mode := "Icon Grid"
	if icon {
		active = 1
		mode = "List View"
	}
	fm.setAttr("BrowserStack", "active", active)
	fm.setText("ViewModeBtn", mode)
	fm.setVisible("IconPrevBtn", false)
	fm.setVisible("IconNextBtn", false)
	fm.setVisible("IconPageLabel", icon)
	fm.setVisible("HeaderBar", !icon)
}

func (fm *fileManager) selectedEntry() (fileEntry, bool) {
	if fm.selected == "" {
		return fileEntry{}, false
	}
	for _, entry := range fm.visible {
		if samePath(entry.path, fm.selected) {
			return entry, true
		}
	}
	return fileEntry{}, false
}

func (fm *fileManager) updateSelectionView() {
	entry, ok := fm.selectedEntry()
	if !ok {
		fm.setText("SelectedName", "(no selection)")
		fm.setText("SelectedMeta", "-")
		fm.setText("SelectedPerm", "-")
		fm.setText("Preview", "Select a file or folder to preview.")
		return
	}

	name := entry.name
	kind := "File"
	size := formatSize(entry.size)
	if entry.isDir {
		kind = "Directory"
		size = "--"
		name += "/"
	}
	if entry.isSymlink {
		kind = "Symlink"
	}

	mod := entry.modTime.Format("2006-01-02 15:04:05")
	meta := fmt.Sprintf("%s | size: %s | modified: %s", kind, size, mod)
	perm := fmt.Sprintf("%s | %s", entry.mode.String(), shortPath(entry.path, 68))
	if entry.isSymlink && entry.linkTarget != "" {
		perm = fmt.Sprintf("%s -> %s", perm, shortPath(entry.linkTarget, 30))
	}

	fm.setText("SelectedName", name)
	fm.setText("SelectedMeta", meta)
	fm.setText("SelectedPerm", perm)
	if fm.e("Preview") != nil {
		fm.setText("Preview", fm.buildPreview(entry))
	}
}

func (fm *fileManager) buildPreview(entry fileEntry) string {
	if entry.isDir {
		children, err := os.ReadDir(entry.path)
		if err != nil {
			return fmt.Sprintf("Directory preview failed: %v", err)
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("Directory: %s\n", entry.path))
		b.WriteString(fmt.Sprintf("Items: %d\n\n", len(children)))
		if len(children) == 0 {
			b.WriteString("(empty)")
			return b.String()
		}

		limit := len(children)
		if limit > 20 {
			limit = 20
		}
		for i := 0; i < limit; i++ {
			name := children[i].Name()
			if children[i].IsDir() {
				name += "/"
			}
			b.WriteString(name)
			b.WriteByte('\n')
		}
		if len(children) > limit {
			b.WriteString(fmt.Sprintf("\n... and %d more", len(children)-limit))
		}
		return b.String()
	}

	f, err := os.Open(entry.path)
	if err != nil {
		return fmt.Sprintf("File preview failed: %v", err)
	}
	defer f.Close()

	const previewBytes = 8192
	buf := make([]byte, previewBytes)
	n, readErr := f.Read(buf)
	if readErr != nil && readErr != io.EOF {
		return fmt.Sprintf("File preview failed: %v", readErr)
	}
	data := buf[:n]
	if len(data) == 0 {
		return "(empty file)"
	}

	if !utf8.Valid(data) || looksBinary(data) {
		return hexPreview(data, entry.size)
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if entry.size > int64(len(data)) {
		text += fmt.Sprintf("\n\n... truncated at %d bytes of %d bytes", len(data), entry.size)
	}
	return text
}

func (fm *fileManager) updateSortButtons() {
	fm.setText("SortBtn", "Sort")
	order := "Asc"
	if fm.sortDesc {
		order = "Desc"
	}
	fm.setText("OrderBtn", "Order: "+order)
}

func (fm *fileManager) updateSummary() {
	total := len(fm.entries)
	shown := len(fm.visible)
	summary := fmt.Sprintf("%d item(s)", total)
	if shown != total {
		summary = fmt.Sprintf("%d shown of %d item(s)", shown, total)
	}
	fm.setText("Status", summary)
}

func (fm *fileManager) updateClipboardLabel() {
	if fm.clipboard == nil {
		fm.setText("ClipboardLabel", "Clipboard: empty")
		return
	}
	action := "Copy"
	if fm.clipboard.cut {
		action = "Cut"
	}
	fm.setText("ClipboardLabel", fmt.Sprintf("Clipboard: %s %s", action, shortPath(fm.clipboard.path, 44)))
}

func (fm *fileManager) initSidebarIcons() {
	fm.setAttr("PlaceHomeIcon", "src", fm.iconPath("folder", 48))
	fm.setAttr("PlaceDocumentsIcon", "src", fm.iconPath("folder", 48))
	fm.setAttr("PlaceDownloadsIcon", "src", fm.iconPath("folder", 48))
	fm.setAttr("PlacePicturesIcon", "src", fm.iconPath("folder", 48))
	fm.setAttr("PlaceVideosIcon", "src", fm.iconPath("folder", 48))
}

func (fm *fileManager) styleSidebarButtons() {
	buttonIDs := []string{
		"PlaceHomeBtn",
		"PlaceDocumentsBtn",
		"PlaceDownloadsBtn",
		"PlacePicturesBtn",
		"PlaceVideosBtn",
		"DriveBtn0",
		"DriveBtn1",
		"DriveBtn2",
		"DriveBtn3",
		"DriveBtn4",
		"DriveBtn5",
		"RefreshDrivesBtn",
	}
	for _, id := range buttonIDs {
		fm.setAttr(id, "background", "transparent")
		fm.setAttr(id, "gradientTop", "transparent")
		fm.setAttr(id, "gradientBottom", "transparent")
		fm.setAttr(id, "hoverBackground", "transparent")
		fm.setAttr(id, "pressedBackground", "transparent")
		fm.setAttr(id, "borderColor", "transparent")
		fm.setAttr(id, "focusedBorderColor", "transparent")
		fm.setAttr(id, "focusRing", false)
		fm.setAttr(id, "shadow", false)
		fm.setAttr(id, "textAlign", "left")
	}
}

func (fm *fileManager) defaultRenameTarget(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem = base
		ext = ""
	}
	return filepath.Join(dir, stem+"-renamed"+ext)
}

func (fm *fileManager) refreshDrives() {
	fm.drives = discoverDrives(fm.home)
	for i := 0; i < driveSlotSize; i++ {
		btn := fm.e(fmt.Sprintf("DriveBtn%d", i))
		img := fm.e(fmt.Sprintf("DriveIcon%d", i))
		lbl := fm.e(fmt.Sprintf("DriveLabel%d", i))
		if btn == nil || img == nil || lbl == nil {
			continue
		}

		if i < len(fm.drives) {
			d := fm.drives[i]
			btn.SetVisible(true)
			lbl.SetAttribute("text", shortPath(d.name, 24))
			iconName := "folder"
			if samePath(d.path, fm.home) {
				iconName = "home"
			}
			img.SetAttribute("src", fm.iconPath(iconName, 48))
		} else {
			btn.SetVisible(false)
		}
	}
}

func (fm *fileManager) iconNameForEntry(entry fileEntry) string {
	if entry.isDir {
		return "folder"
	}

	ext := strings.ToLower(filepath.Ext(entry.name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".svg", ".tif", ".tiff":
		return "image"
	case ".mp4", ".mkv", ".avi", ".mov", ".webm", ".mpg", ".mpeg", ".flv":
		return "video"
	case ".mp3", ".wav", ".flac", ".ogg", ".aac", ".m4a":
		return "audio"
	case ".zip", ".tar", ".gz", ".xz", ".bz2", ".7z", ".rar":
		return "archive"
	default:
		return "file"
	}
}

func (fm *fileManager) iconSrcForEntry(entry fileEntry, size int) string {
	if entry.isDir && fm.isAppCatalogDir(fm.cwd) {
		customIcon := filepath.Join(entry.path, "icon.png")
		if info, err := os.Stat(customIcon); err == nil && !info.IsDir() {
			return customIcon
		}
	}
	return fm.iconPath(fm.iconNameForEntry(entry), size)
}

func (fm *fileManager) isAppCatalogDir(path string) bool {
	roots := []string{
		"/apps",
		"/avyos/apps",
		filepath.Join(fm.home, "apps"),
	}
	for _, root := range roots {
		if samePath(path, root) {
			return true
		}
	}
	return false
}

func (fm *fileManager) iconPath(name string, size int) string {
	return gfxicons.ResolvePath(name, size)
}

func (fm *fileManager) setStatus(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	fm.setText("Status", text)
}

func (fm *fileManager) setStatusf(format string, args ...interface{}) {
	fm.setStatus(fmt.Sprintf(format, args...))
}

func (fm *fileManager) resolvePath(raw, base string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if raw == "~" {
		return fm.home
	}
	if strings.HasPrefix(raw, "~/") {
		return filepath.Clean(filepath.Join(fm.home, raw[2:]))
	}
	if !filepath.IsAbs(raw) {
		raw = filepath.Join(base, raw)
	}
	return filepath.Clean(raw)
}

func (fm *fileManager) resolveRenameTarget(raw, selectedPath string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "~") || filepath.IsAbs(raw) {
		return fm.resolvePath(raw, fm.cwd)
	}
	if strings.Contains(raw, "/") {
		return filepath.Clean(filepath.Join(fm.cwd, raw))
	}
	return filepath.Clean(filepath.Join(filepath.Dir(selectedPath), raw))
}

func (fm *fileManager) uniqueDestination(path string) string {
	if !exists(path) {
		return path
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	name, ext := splitNameExt(base)
	for i := 1; i < 10000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
		if !exists(candidate) {
			return candidate
		}
	}
	return filepath.Join(dir, name+"-copy"+ext)
}

func (fm *fileManager) formatListItem(entry fileEntry) string {
	name := entry.name
	if entry.isDir {
		name += "/"
	}
	name = shortPath(sanitizeListText(name), 34)

	size := "--"
	if !entry.isDir {
		size = formatSize(entry.size)
	}

	kind := shortPath(entryTypeLabel(entry), 12)
	modified := humanModified(entry.modTime)
	return fmt.Sprintf("%-34s %-9s %-12s %s", name, size, kind, modified)
}

func listDir(path string) ([]fileEntry, error) {
	des, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	out := make([]fileEntry, 0, len(des))
	for _, de := range des {
		fullPath := filepath.Join(path, de.Name())
		linfo, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}

		entry := fileEntry{
			name:      de.Name(),
			path:      fullPath,
			isSymlink: linfo.Mode()&os.ModeSymlink != 0,
			mode:      linfo.Mode(),
			modTime:   linfo.ModTime(),
		}

		targetInfo := os.FileInfo(linfo)
		if entry.isSymlink {
			if target, err := os.Readlink(fullPath); err == nil {
				entry.linkTarget = target
			}
			if tinfo, err := os.Stat(fullPath); err == nil {
				targetInfo = tinfo
			}
		}

		entry.isDir = targetInfo.IsDir()
		if !entry.isDir {
			entry.size = targetInfo.Size()
		}
		out = append(out, entry)
	}
	return out, nil
}

func discoverDrives(home string) []driveEntry {
	out := make([]driveEntry, 0, 1)
	if info, err := os.Stat("/"); err == nil && info.IsDir() {
		out = append(out, driveEntry{name: "Root (/)", path: "/"})
	}
	return out
}

func entryLabel(entry fileEntry) string {
	if entry.isDir {
		return entry.name + "/"
	}
	return entry.name
}

func formatSize(size int64) string {
	switch {
	case size >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(1<<30))
	case size >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func entryTypeLabel(entry fileEntry) string {
	if entry.isDir {
		return "Folder"
	}
	if entry.isSymlink {
		return "Symlink"
	}

	ext := strings.ToLower(filepath.Ext(entry.name))
	switch ext {
	case ".txt", ".md", ".rtf", ".doc", ".docx", ".pdf", ".odt":
		return "Document"
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".svg", ".tif", ".tiff":
		return "Image"
	case ".mp4", ".mkv", ".avi", ".mov", ".webm", ".mpg", ".mpeg", ".flv":
		return "Video"
	case ".mp3", ".wav", ".flac", ".ogg", ".aac", ".m4a":
		return "Audio"
	case ".zip", ".tar", ".gz", ".xz", ".bz2", ".7z", ".rar":
		return "Archive"
	default:
		return "File"
	}
}

func humanModified(t time.Time) string {
	now := time.Now()
	if t.After(now) {
		return t.Format("2006-01-02")
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "Just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "Yesterday"
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

func shortPath(text string, max int) string {
	if max < 4 {
		return text
	}
	r := []rune(text)
	if len(r) <= max {
		return text
	}
	return string(r[:max-3]) + "..."
}

func ellipsisText(text string, maxPx int) string {
	text = strings.TrimSpace(text)
	if text == "" || maxPx <= 0 {
		return ""
	}
	font := gfxfont.DefaultFont
	if font == nil {
		return shortPath(text, 12)
	}
	if font.TextWidth(text) <= maxPx {
		return text
	}

	const dots = "..."
	dotsW := font.TextWidth(dots)
	if dotsW >= maxPx {
		return dots
	}

	r := []rune(text)
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if font.TextWidth(string(r[:mid]))+dotsW <= maxPx {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo <= 0 {
		return dots
	}
	return string(r[:lo]) + dots
}

func sanitizeListText(text string) string {
	text = strings.ReplaceAll(text, "|", "/")
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	return text
}

func splitNameExt(base string) (string, string) {
	ext := filepath.Ext(base)
	if ext == base {
		ext = ""
	}
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		name = base
	}
	return name, ext
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if samePath(src, dst) {
		return fmt.Errorf("source and destination are the same")
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			srcChild := filepath.Join(src, entry.Name())
			dstChild := filepath.Join(dst, entry.Name())
			if err := copyPath(srcChild, dstChild); err != nil {
				return err
			}
		}
		return nil
	}

	return copyFile(src, dst, info.Mode().Perm())
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func movePath(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyPath(src, dst); err != nil {
		return err
	}
	return removePath(src)
}

func removePath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

func looksBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}
	control := 0
	for _, b := range data {
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			control++
		}
	}
	return float64(control)/float64(len(data)) > 0.25
}

func hexPreview(data []byte, totalSize int64) string {
	const maxDump = 256
	limit := len(data)
	if limit > maxDump {
		limit = maxDump
	}

	var b strings.Builder
	b.WriteString("Binary file preview\n")
	b.WriteString(fmt.Sprintf("Showing first %d bytes of %d bytes\n\n", limit, totalSize))

	for i := 0; i < limit; i += 16 {
		end := i + 16
		if end > limit {
			end = limit
		}
		line := data[i:end]
		b.WriteString(fmt.Sprintf("%08x  ", i))
		for j := 0; j < 16; j++ {
			if j < len(line) {
				b.WriteString(fmt.Sprintf("%02x ", line[j]))
			} else {
				b.WriteString("   ")
			}
		}
		b.WriteString(" ")
		for _, ch := range line {
			if ch >= 32 && ch <= 126 {
				b.WriteByte(ch)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteByte('\n')
	}

	if len(data) > limit {
		b.WriteString("\n... truncated ...")
	}
	return b.String()
}
