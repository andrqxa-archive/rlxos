package icons

import (
	"fmt"
	"sync"

	"avyos.dev/pkg/graphics/assets"
	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/core"
)

type Buffer = core.Buffer

var iconCache sync.Map // key: "<size>:<name>" => *Buffer

// ResolvePath returns the best icon file path for name+size.
// Falls back to help.{svg,png} if the requested icon is missing.
func ResolvePath(name string, size int) string {
	return assets.ResolveIconPath(name, size)
}

// ResolveIconPath returns the best icon file path for name+size.
func ResolveIconPath(name string, size int) string {
	return ResolvePath(name, size)
}

// Load loads an icon by name and size as a Buffer.
func Load(name string, size int) (*Buffer, error) {
	key := fmt.Sprintf("%d:%s", size, name)
	if v, ok := iconCache.Load(key); ok {
		if buf, ok := v.(*Buffer); ok {
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
func LoadIcon(name string, size int) (*Buffer, error) {
	return Load(name, size)
}
