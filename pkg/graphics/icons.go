package graphics

import (
	"fmt"
	"sync"

	"avyos.dev/pkg/graphics/assets"
)


var iconCache sync.Map // key: "<size>:<name>" => *Buffer

// ResolveIconPath returns the best icon file path for name+size.
// Falls back to help.{svg,png} if the requested icon is missing.
func ResolveIconPath(name string, size int) string {
	return assets.ResolveIconPath(name, size)
}

// LoadIcon loads an icon by name and size as a graphics.Buffer.
func LoadIcon(name string, size int) (*Buffer, error) {
	key := fmt.Sprintf("%d:%s", size, name)
	if v, ok := iconCache.Load(key); ok {
		if buf, ok2 := v.(*Buffer); ok2 {
			return buf, nil
		}
	}

	path := ResolveIconPath(name, size)
	if path == "" {
		return nil, fmt.Errorf("icon not found: %s (%d)", name, size)
	}

	buf, err := DecodeImageToBuffer(path, size)
	if err != nil {
		return nil, err
	}
	iconCache.Store(key, buf)
	return buf, nil
}
