package appcatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"avyos.dev/pkg/graphics"
)

type Manifest struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Description       string           `json:"description"`
	Icon              string           `json:"icon,omitempty"`
	Hidden            bool             `json:"hidden,omitempty"`
	Dock              bool             `json:"dock,omitempty"`
	Background        bool             `json:"background,omitempty"`
	Actions           []ManifestAction `json:"actions,omitempty"`
	SupportExtensions []string         `json:"support_extensions,omitempty"`
	Extensions        []string         `json:"extensions,omitempty"`
}

type ManifestAction struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name,omitempty"`
	Label   string   `json:"label,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

type Entry struct {
	ID                string
	Name              string
	Description       string
	ExecPath          string
	IconName          string
	IconPath          string
	RootDir           string
	DirName           string
	Hidden            bool
	Dock              bool
	Background        bool
	Actions           []ManifestAction
	SupportExtensions []string
}

type DiscoverOptions struct {
	Home          string
	IncludeHidden bool
}

type dockPinsFile struct {
	Pins []string `json:"pins"`
}

func Discover(opts DiscoverOptions) []Entry {
	seen := make(map[string]struct{})
	out := make([]Entry, 0, 24)

	for _, root := range discoverRoots(opts.Home) {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, de := range entries {
			if !de.IsDir() {
				continue
			}

			appDir := de.Name()
			mf, ok := loadManifest(filepath.Join(root, appDir, "manifest.json"))
			if !ok {
				continue
			}

			id := normalizeID(mf.ID)
			if id == "" {
				id = normalizeID(appDir)
			}
			if id == "" {
				continue
			}
			if !opts.IncludeHidden && mf.Hidden {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}

			execPath := resolveExecPath(root, appDir)
			if execPath == "" {
				continue
			}

			name := strings.TrimSpace(mf.Name)
			if name == "" {
				name = titleFromID(id)
			}
			iconName := strings.TrimSpace(mf.Icon)
			if iconName == "" {
				iconName = id
			}
			supportExtensions := normalizeExtensions(append(append([]string(nil), mf.SupportExtensions...), mf.Extensions...))

			entry := Entry{
				ID:                id,
				Name:              name,
				Description:       strings.TrimSpace(mf.Description),
				ExecPath:          execPath,
				IconName:          iconName,
				IconPath:          resolveLocalIcon(root, appDir, iconName, id),
				RootDir:           root,
				DirName:           appDir,
				Hidden:            mf.Hidden,
				Dock:              mf.Dock,
				Background:        mf.Background,
				Actions:           normalizeActions(mf.Actions),
				SupportExtensions: supportExtensions,
			}
			seen[id] = struct{}{}
			out = append(out, entry)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		ln := strings.ToLower(out[i].Name)
		rn := strings.ToLower(out[j].Name)
		if ln == rn {
			return out[i].ID < out[j].ID
		}
		return ln < rn
	})
	return out
}

func FindByID(entries []Entry, id string) (Entry, bool) {
	id = normalizeID(id)
	for _, entry := range entries {
		if normalizeID(entry.ID) == id {
			return entry, true
		}
	}
	return Entry{}, false
}

func FindByExtension(entries []Entry, ext string) (Entry, bool) {
	ext = normalizeExtension(ext)
	if ext == "" {
		return Entry{}, false
	}
	for _, entry := range entries {
		if SupportsExtension(entry, ext) {
			return entry, true
		}
	}
	return Entry{}, false
}

func FindByFilePath(entries []Entry, path string) (Entry, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Entry{}, false
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return Entry{}, false
	}
	return FindByExtension(entries, ext)
}

func SupportsExtension(entry Entry, ext string) bool {
	ext = normalizeExtension(ext)
	if ext == "" {
		return false
	}
	for _, supported := range entry.SupportExtensions {
		if supported == "*" || supported == ext {
			return true
		}
	}
	return false
}

func DefaultDockPins() []string {
	return []string{"appmenu", "filemanager", "terminal", "notepad", "power"}
}

func DockPinsPath(home string) string {
	home = strings.TrimSpace(home)
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	if home == "" {
		return filepath.Join(".config", "avyos", "dock_pins.json")
	}
	return filepath.Join(home, ".config", "avyos", "dock_pins.json")
}

func LoadDockPins(home string, defaults []string) []string {
	path := DockPinsPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		return normalizeIDs(defaults)
	}

	var file dockPinsFile
	if err := json.Unmarshal(data, &file); err == nil && len(file.Pins) > 0 {
		return normalizeIDs(file.Pins)
	}

	var flat []string
	if err := json.Unmarshal(data, &flat); err == nil && len(flat) > 0 {
		return normalizeIDs(flat)
	}

	return normalizeIDs(defaults)
}

func SaveDockPins(home string, pins []string) error {
	path := DockPinsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	payload := dockPinsFile{Pins: normalizeIDs(pins)}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func ToggleDockPin(home, id string, defaults []string) (bool, error) {
	id = normalizeID(id)
	if id == "" {
		return false, nil
	}

	pins := LoadDockPins(home, defaults)
	for i, pin := range pins {
		if normalizeID(pin) == id {
			pins = append(pins[:i], pins[i+1:]...)
			return false, SaveDockPins(home, pins)
		}
	}

	pins = append(pins, id)
	return true, SaveDockPins(home, pins)
}

func discoverRoots(home string) []string {
	roots := []string{"/apps", "/avyos/apps"}
	if h := strings.TrimSpace(home); h != "" {
		roots = append(roots, filepath.Join(h, "apps"))
	}
	roots = append(roots, "apps")
	return roots
}

func loadManifest(path string) (Manifest, bool) {
	var mf Manifest
	data, err := os.ReadFile(path)
	if err != nil {
		return mf, false
	}
	if err := json.Unmarshal(data, &mf); err != nil {
		return mf, false
	}
	if strings.TrimSpace(mf.ID) == "" && strings.TrimSpace(mf.Name) == "" {
		return mf, false
	}
	return mf, true
}

func resolveExecPath(root, appDir string) string {
	for _, rel := range []string{"exec", appDir} {
		p := filepath.Join(root, appDir, rel)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func resolveLocalIcon(root, appDir, iconName, id string) string {
	for _, rel := range []string{"icon.svg", "icon.png"} {
		p := filepath.Join(root, appDir, rel)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	if iconName != "" {
		if p := graphics.ResolveIconPath(iconName, 64); p != "" {
			return p
		}
	}
	if id != "" {
		if p := graphics.ResolveIconPath(id, 64); p != "" {
			return p
		}
	}
	return graphics.ResolveIconPath("help", 64)
}

func normalizeID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func normalizeActions(actions []ManifestAction) []ManifestAction {
	if len(actions) == 0 {
		return nil
	}
	out := make([]ManifestAction, 0, len(actions))
	for _, action := range actions {
		label := strings.TrimSpace(action.Label)
		if label == "" {
			label = strings.TrimSpace(action.Name)
		}
		if label == "" {
			continue
		}
		id := normalizeID(action.ID)
		if id == "" {
			id = normalizeID(label)
		}
		out = append(out, ManifestAction{
			ID:      id,
			Name:    label,
			Label:   label,
			Command: strings.TrimSpace(action.Command),
			Args:    append([]string(nil), action.Args...),
		})
	}
	return out
}

func normalizeExtension(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return ""
	}
	if ext == "*" {
		return ext
	}
	ext = strings.TrimPrefix(ext, "*")
	if ext == "" || ext == "." {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

func normalizeExtensions(exts []string) []string {
	if len(exts) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(exts))
	out := make([]string, 0, len(exts))
	for _, raw := range exts {
		ext := normalizeExtension(raw)
		if ext == "" {
			continue
		}
		if _, ok := seen[ext]; ok {
			continue
		}
		seen[ext] = struct{}{}
		if ext == "*" {
			return []string{"*"}
		}
		out = append(out, ext)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		n := normalizeID(id)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func titleFromID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "App"
	}
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	if len(parts) == 0 {
		return "App"
	}
	for i, part := range parts {
		if part == "" {
			continue
		}
		r := []rune(strings.ToLower(part))
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}
