package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	gfxfont "avyos.dev/pkg/graphics/fonts"
)

var (
	elementFontCache sync.Map // key -> *gfxfont.Font
	elementFontMiss  sync.Map // key -> struct{}
)

func resolveTextRole(e *Element, fallback string) string {
	if e == nil {
		return fallback
	}
	if role := e.Attr("textRole", ""); role != "" {
		return role
	}
	if role := e.Attr("fontRole", ""); role != "" {
		return role
	}
	return fallback
}

func resolveTitleRole(e *Element) string {
	if e == nil {
		return gfxfont.UIFontTitle
	}
	if role := e.Attr("titleTextRole", ""); role != "" {
		return role
	}
	return gfxfont.UIFontTitle
}

func elementFont(e *Element, fallbackRole string) *gfxfont.Font {
	if f := elementCustomFont(e, fallbackRole); f != nil {
		return f
	}
	return gfxfont.UIFont(resolveTextRole(e, fallbackRole))
}

func elementTitleFont(e *Element) *gfxfont.Font {
	return gfxfont.UIFont(resolveTitleRole(e))
}

func elementCustomFont(e *Element, fallbackRole string) *gfxfont.Font {
	if e == nil {
		return nil
	}

	path := resolveElementFontPath(e)
	if path == "" {
		return nil
	}

	size := e.AttrFloat("fontSize", 0)
	if size <= 0 {
		if base := gfxfont.UIFont(resolveTextRole(e, fallbackRole)); base != nil && base.Height > 0 {
			size = float64(base.Height)
		}
	}
	if size <= 0 {
		size = 14
	}

	key := fmt.Sprintf("%s|%.2f", path, size)
	if _, miss := elementFontMiss.Load(key); miss {
		return nil
	}
	if cached, ok := elementFontCache.Load(key); ok {
		if f, ok := cached.(*gfxfont.Font); ok && f != nil {
			return f
		}
	}

	font, err := gfxfont.LoadTTFFontFile(path, &gfxfont.Options{Size: size})
	if err != nil || font == nil {
		elementFontMiss.Store(key, struct{}{})
		return nil
	}

	elementFontCache.Store(key, font)
	return font
}

func resolveElementFontPath(e *Element) string {
	if e == nil {
		return ""
	}

	if raw := strings.TrimSpace(e.Attr("fontPath", "")); raw != "" {
		if elementFontFileExists(raw) {
			return raw
		}
	}

	family := normalizeFontFamily(e.Attr("fontFamily", ""))
	if family == "" {
		return ""
	}

	candidates := []string{
		filepath.Join("/data/fonts", family, family+".ttf"),
		filepath.Join("/avyos/data/fonts", family, family+".ttf"),
		filepath.Join("data/fonts", family, family+".ttf"),
	}
	for _, candidate := range candidates {
		if elementFontFileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func normalizeFontFamily(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "-", "_", "-", "\\", "-", "/", "-")
	return replacer.Replace(s)
}

func elementFontFileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
