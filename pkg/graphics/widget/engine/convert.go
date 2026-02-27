package engine

import (
	"fmt"
	"strconv"
	"strings"

	graphics "avyos.dev/pkg/graphics/input"
	gfxtheme "avyos.dev/pkg/graphics/theme"
)

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case int8:
		return strconv.FormatInt(int64(val), 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case uint16:
		return strconv.FormatUint(uint64(val), 10)
	case uint8:
		return strconv.FormatUint(uint64(val), 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	}
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	}
	return 0
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1"
	}
	return false
}

func toColor(v interface{}) graphics.Color {
	switch val := v.(type) {
	case graphics.Color:
		return val
	case string:
		return parseColorStr(val)
	}
	return graphics.Color{}
}

func parseColorStr(s string) graphics.Color {
	s = strings.TrimSpace(s)

	// Theme colors
	if strings.HasPrefix(s, "theme.") {
		name := strings.ToLower(s[6:])
		switch name {
		case "background":
			return gfxtheme.DefaultTheme.Background
		case "color.bg":
			return gfxtheme.DefaultTheme.Background
		case "color.bg.alt":
			return gfxtheme.DefaultTheme.Secondary
		case "foreground":
			return gfxtheme.DefaultTheme.Foreground
		case "color.text.primary":
			return gfxtheme.DefaultTheme.Foreground
		case "primary":
			return gfxtheme.DefaultTheme.Primary
		case "color.accent":
			return gfxtheme.DefaultTheme.Primary
		case "color.accent.alt", "color.accent.secondary", "color.accent2":
			return gfxtheme.DefaultTheme.AccentAlt
		case "primaryhover":
			return gfxtheme.DefaultTheme.PrimaryHover
		case "color.accent.hover":
			return gfxtheme.DefaultTheme.PrimaryHover
		case "primaryactive":
			return gfxtheme.DefaultTheme.PrimaryActive
		case "secondary":
			return gfxtheme.DefaultTheme.Secondary
		case "color.control.fill":
			return gfxtheme.DefaultTheme.Secondary
		case "border":
			return gfxtheme.DefaultTheme.Border
		case "color.stroke.hairline":
			return gfxtheme.DefaultTheme.Border
		case "borderfocused":
			return gfxtheme.DefaultTheme.BorderFocused
		case "color.stroke.focus":
			return gfxtheme.DefaultTheme.BorderFocused
		case "inputbackground":
			return gfxtheme.DefaultTheme.InputBackground
		case "inputforeground":
			return gfxtheme.DefaultTheme.InputForeground
		case "buttontext":
			return gfxtheme.DefaultTheme.ButtonText
		case "color.surface.glass":
			return gfxtheme.DefaultTheme.SurfaceGlass
		case "color.surface.card":
			return gfxtheme.DefaultTheme.SurfaceRaised
		case "color.surface.glassraised":
			return gfxtheme.DefaultTheme.SurfaceRaised
		case "color.surface.sidebar":
			return gfxtheme.DefaultTheme.SurfaceSidebar
		case "color.stroke.divider":
			return gfxtheme.DefaultTheme.StrokeDivider
		case "color.text.secondary":
			return gfxtheme.DefaultTheme.TextSecondary
		case "color.text.muted":
			return gfxtheme.DefaultTheme.TextMuted
		case "color.text.disabled":
			return gfxtheme.DefaultTheme.TextDisabled
		case "color.control.hover":
			return gfxtheme.DefaultTheme.ControlHover
		case "color.control.pressed":
			return gfxtheme.DefaultTheme.ControlPressed
		case "color.accent.subtle":
			return gfxtheme.DefaultTheme.AccentSubtle
		case "color.semantic.success":
			return gfxtheme.DefaultTheme.Success
		case "color.semantic.warning":
			return gfxtheme.DefaultTheme.Warning
		case "color.semantic.danger":
			return gfxtheme.DefaultTheme.Danger
		}
	}

	// Named colors
	switch strings.ToLower(s) {
	case "black":
		return graphics.ColorBlack
	case "white":
		return graphics.ColorWhite
	case "red":
		return graphics.ColorRed
	case "green":
		return graphics.ColorGreen
	case "blue":
		return graphics.ColorBlue
	case "gray", "grey":
		return graphics.ColorGray
	case "transparent":
		return graphics.ColorTransparent
	}

	// Hex colors
	if strings.HasPrefix(s, "#") {
		hex := s[1:]
		switch len(hex) {
		case 3:
			r, _ := strconv.ParseUint(string(hex[0])+string(hex[0]), 16, 8)
			g, _ := strconv.ParseUint(string(hex[1])+string(hex[1]), 16, 8)
			bl, _ := strconv.ParseUint(string(hex[2])+string(hex[2]), 16, 8)
			return graphics.NewColorRGB(uint8(r), uint8(g), uint8(bl))
		case 6:
			v, _ := strconv.ParseUint(hex, 16, 32)
			return graphics.NewColorHex(uint32(v))
		case 8:
			v, _ := strconv.ParseUint(hex, 16, 32)
			return graphics.NewColorHex(uint32(v))
		}
	}

	return graphics.Color{}
}

// parsePaddingStr parses CSS-like padding: "10", "10 20", "10 20 10 20"
func parsePaddingStr(s string) (top, right, bottom, left int) {
	parts := strings.Fields(s)
	switch len(parts) {
	case 1:
		v, _ := strconv.Atoi(parts[0])
		return v, v, v, v
	case 2:
		tb, _ := strconv.Atoi(parts[0])
		lr, _ := strconv.Atoi(parts[1])
		return tb, lr, tb, lr
	case 4:
		t, _ := strconv.Atoi(parts[0])
		r, _ := strconv.Atoi(parts[1])
		b, _ := strconv.Atoi(parts[2])
		l, _ := strconv.Atoi(parts[3])
		return t, r, b, l
	}
	return 0, 0, 0, 0
}
