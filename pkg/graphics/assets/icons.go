package assets

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const fallbackIconName = "help"

var pathCache sync.Map // key: "<size>:<name>" => string

// ResolveIconPath returns the best icon file path for name+size.
// Falls back to help.{svg,png} if the requested icon is missing.
func ResolveIconPath(name string, size int) string {
	if p := resolvePathByName(name, size); p != "" {
		return p
	}
	return resolvePathByName(fallbackIconName, size)
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
