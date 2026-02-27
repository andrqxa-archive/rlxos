/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package app

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"avyos.dev/pkg/fs"
	gfxfont "avyos.dev/pkg/graphics/font"
	"avyos.dev/pkg/ini"
	xfont "golang.org/x/image/font"
)

const fontConfigFile = "fonts.ini"

type configuredFont struct {
	Name      string
	BaseOpts  gfxfont.Options
	RoleSizes map[string]float64
}

// ApplyConfiguredDefaultFont loads config/fonts.ini and sets Default/UI fonts.
// It always applies bitmap fallback first so rendering remains functional.
func ApplyConfiguredDefaultFont() error {
	applyBitmapFallbackFonts()

	fonts, err := loadConfiguredFonts()
	if err != nil {
		return err
	}
	if len(fonts) == 0 {
		return nil
	}

	if paragraph := fonts[gfxfont.UIFontParagraph]; paragraph != nil {
		gfxfont.DefaultFont = paragraph
	}
	gfxfont.SetUIFonts(fonts)
	return nil
}

func (a *App) loadConfiguredDefaultFont() {
	if err := ApplyConfiguredDefaultFont(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load configured font: %v (using internal bitmap font)\n", err)
	}
}

func applyBitmapFallbackFonts() {
	if gfxfont.BitmapFont == nil {
		return
	}
	gfxfont.DefaultFont = gfxfont.BitmapFont
	gfxfont.SetUIFonts(map[string]*gfxfont.Font{
		gfxfont.UIFontParagraph:  gfxfont.BitmapFont,
		gfxfont.UIFontSubheading: gfxfont.BitmapFont,
		gfxfont.UIFontHeading:    gfxfont.BitmapFont,
		gfxfont.UIFontTitle:      gfxfont.BitmapFont,
	})
}

func loadConfiguredFonts() (map[string]*gfxfont.Font, error) {
	cfgPath := fs.Resolve("config:%s", fontConfigFile)
	cfg, err := ini.ParseFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("parse %s: %w", cfgPath, err)
	}

	spec, ok := pickConfiguredFont(cfg)
	if !ok {
		return nil, nil
	}

	resolvedPath, err := resolveConfiguredFontPath(spec.Name)
	if err != nil {
		return nil, err
	}

	loaded := make(map[string]*gfxfont.Font, len(spec.RoleSizes))
	loadOrder := []string{
		gfxfont.UIFontParagraph,
		gfxfont.UIFontSubheading,
		gfxfont.UIFontHeading,
		gfxfont.UIFontTitle,
	}
	for _, role := range loadOrder {
		size, ok := spec.RoleSizes[role]
		if !ok || size <= 0 {
			continue
		}
		opts := spec.BaseOpts
		opts.Size = size
		font, err := gfxfont.LoadTTFFontFile(resolvedPath, &opts)
		if err != nil {
			return nil, fmt.Errorf("load %s (%s size %.1f): %w", resolvedPath, role, size, err)
		}
		loaded[role] = font
	}

	if loaded[gfxfont.UIFontParagraph] == nil {
		return nil, fmt.Errorf("paragraph font size is not configured in %s", cfgPath)
	}
	return loaded, nil
}

func pickConfiguredFont(cfg *ini.Config) (configuredFont, bool) {
	if cfg == nil {
		return configuredFont{}, false
	}

	sections := prioritizedSections(cfg)
	for _, section := range sections {
		if !isSectionEnabled(cfg, section) {
			continue
		}

		name := normalizeFontToken(firstNonEmpty(cfg, section, "name"))
		if name == "" {
			continue
		}

		opts := gfxfont.Options{Hinting: xfont.HintingFull}
		if v := firstNonEmpty(cfg, section, "dpi"); v != "" {
			if dpi, err := strconv.ParseFloat(v, 64); err == nil && dpi > 0 {
				opts.DPI = dpi
			}
		}
		if v := firstNonEmpty(cfg, section, "hinting", "hint"); v != "" {
			if hinting, ok := parseHinting(v); ok {
				opts.Hinting = hinting
				opts.HintingSet = true
			}
		}

		return configuredFont{
			Name:      name,
			BaseOpts:  opts,
			RoleSizes: roleSizes(cfg, section),
		}, true
	}

	return configuredFont{}, false
}

func roleSizes(cfg *ini.Config, section string) map[string]float64 {
	base := parsePositiveFloat(firstNonEmpty(cfg, section, "size"), 15)

	paragraph := parsePositiveFloat(firstNonEmpty(cfg, section,
		"paragraph_size", "paragraphSize", "parag_size", "paragSize", "body_size", "bodySize"), base)
	if paragraph <= 0 {
		paragraph = base
	}

	subheading := parsePositiveFloat(firstNonEmpty(cfg, section,
		"subheading_size", "subheadingSize", "subtitle_size", "subtitleSize"), paragraph+2)
	heading := parsePositiveFloat(firstNonEmpty(cfg, section,
		"heading_size", "headingSize", "header_size", "headerSize"), paragraph+4)
	title := parsePositiveFloat(firstNonEmpty(cfg, section,
		"title_size", "titleSize"), paragraph+8)

	return map[string]float64{
		gfxfont.UIFontParagraph:  paragraph,
		gfxfont.UIFontSubheading: subheading,
		gfxfont.UIFontHeading:    heading,
		gfxfont.UIFontTitle:      title,
	}
}

func parsePositiveFloat(v string, fallback float64) float64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		return fallback
	}
	return f
}

func prioritizedSections(cfg *ini.Config) []string {
	ordered := make([]string, 0, len(cfg.Sections))
	seen := make(map[string]struct{}, len(cfg.Sections))
	add := func(name string) {
		if _, ok := cfg.Sections[name]; !ok {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		ordered = append(ordered, name)
	}

	for _, name := range []string{"ui", "default", "font", "fonts", ""} {
		add(name)
	}

	others := make([]string, 0, len(cfg.Sections))
	for name := range cfg.Sections {
		if _, ok := seen[name]; ok {
			continue
		}
		others = append(others, name)
	}
	sort.Strings(others)
	ordered = append(ordered, others...)

	return ordered
}

func firstNonEmpty(cfg *ini.Config, section string, keys ...string) string {
	for _, key := range keys {
		if v, ok := cfg.Get(section, key); ok {
			v = strings.TrimSpace(v)
			if v != "" {
				return v
			}
		}
	}
	if section != "" {
		for _, key := range keys {
			if v, ok := cfg.Get("", key); ok {
				v = strings.TrimSpace(v)
				if v != "" {
					return v
				}
			}
		}
	}
	return ""
}

func isSectionEnabled(cfg *ini.Config, section string) bool {
	value := firstNonEmpty(cfg, section, "enabled", "load", "use")
	if value == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enable", "enabled":
		return true
	case "0", "false", "no", "off", "disable", "disabled":
		return false
	default:
		return true
	}
}

func parseHinting(s string) (xfont.Hinting, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "none":
		return xfont.HintingNone, true
	case "vertical":
		return xfont.HintingVertical, true
	case "full":
		return xfont.HintingFull, true
	default:
		return xfont.HintingFull, false
	}
}

func normalizeFontToken(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	repl := strings.NewReplacer(" ", "-", "_", "-", ".", "-", "/", "-", "\\", "-")
	return repl.Replace(s)
}

func resolveConfiguredFontPath(name string) (string, error) {
	name = normalizeFontToken(name)
	if name == "" {
		return "", fmt.Errorf("empty font name in %s", fs.Resolve("config:%s", fontConfigFile))
	}

	path := fs.Resolve("data:fonts/%s/%s.ttf", name, name)
	if !fs.Exists(path) {
		path = fs.Resolve("data:fonts/%s.ttf", name)
	}

	if !fs.Exists(path) {
		return "", fmt.Errorf("font %v not found", name)
	}
	return path, nil
}
