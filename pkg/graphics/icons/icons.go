package icons

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"avyos.dev/pkg/graphics/canvas"
	core "avyos.dev/pkg/graphics/pixmap"
)

var iconCache sync.Map // key: "<size>:<name>" => *core.Buffer
var pathCache sync.Map // key: "<name>:<size>" => string

const fallbackIconName = "help"

// ResolvePath returns the best icon file path for name+size.
// Falls back to help.{svg,png} if the requested icon is missing.
func ResolvePath(name string, size int) string {
	if p := resolvePathByName(name, size); p != "" {
		return p
	}
	return resolvePathByName(fallbackIconName, size)
}

// ResolveIconPath returns the best icon file path for name+size.
func ResolveIconPath(name string, size int) string {
	return ResolvePath(name, size)
}

// Load loads an icon by name and size as a Buffer.
func Load(name string, size int) (*core.Buffer, error) {
	key := fmt.Sprintf("%d:%s", size, name)
	if v, ok := iconCache.Load(key); ok {
		if buf, ok := v.(*core.Buffer); ok {
			return buf, nil
		}
	}

	path := ResolvePath(name, size)
	if path == "" {
		return nil, fmt.Errorf("icon not found: %s (%d)", name, size)
	}

	buf, err := canvas.DecodeImageToBuffer(path, size)
	if err != nil {
		return nil, err
	}
	iconCache.Store(key, buf)
	return buf, nil
}

// LoadIcon loads an icon by name and size as a Buffer.
func LoadIcon(name string, size int) (*core.Buffer, error) {
	return Load(name, size)
}

func resolvePathByName(name string, size int) string {
	if name == "" {
		return ""
	}

	key := fmt.Sprintf("%s:%d", name, size)
	if v, ok := pathCache.Load(key); ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}

	for _, root := range []string{
		"/data/icons/default",
		"/avyos/data/icons/default",
		"data/icons/default",
	} {
		for _, rel := range []string{name + ".svg", name + ".png"} {
			p := filepath.Join(root, rel)
			if fileExists(p) {
				pathCache.Store(key, p)
				return p
			}
		}
	}

	pathCache.Store(key, "")
	return ""
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
