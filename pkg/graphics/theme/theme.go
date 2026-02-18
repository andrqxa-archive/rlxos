package theme

import "avyos.dev/pkg/graphics/core"

// Theme holds the colors used for rendering widgets.
type Theme struct {
	Background      core.Color
	Foreground      core.Color
	Primary         core.Color
	PrimaryHover    core.Color
	PrimaryActive   core.Color
	Secondary       core.Color
	Border          core.Color
	BorderFocused   core.Color
	InputBackground core.Color
	InputForeground core.Color
	ButtonText      core.Color
	SurfaceGlass    core.Color
	SurfaceRaised   core.Color
	SurfaceSidebar  core.Color
	StrokeDivider   core.Color
	TextSecondary   core.Color
	TextMuted       core.Color
	TextDisabled    core.Color
	ControlHover    core.Color
	ControlPressed  core.Color
	AccentSubtle    core.Color
	Success         core.Color
	Warning         core.Color
	Danger          core.Color
	BorderRadius    int
}

// DefaultTheme is the default color theme.
var DefaultTheme = AvyosLight

var AvyosLight = Theme{
	Background:      core.NewColorHex(0xEEF2FA),
	Foreground:      core.NewColorHex(0x121A2B),
	Primary:         core.NewColorHex(0x2F6BFF),
	PrimaryHover:    core.NewColorHex(0x2A5FE7),
	PrimaryActive:   core.NewColorHex(0x2455D0),
	Secondary:       core.NewColorHex(0xFFFFFFB2),
	Border:          core.NewColorHex(0x1B2A4A24),
	BorderFocused:   core.NewColorHex(0x2F6BFF59),
	InputBackground: core.NewColorHex(0xFFFFFFB2),
	InputForeground: core.NewColorHex(0x121A2B),
	ButtonText:      core.NewColorHex(0xFFFFFFF2),
	SurfaceGlass:    core.NewColorHex(0xF7F9FFB8),
	SurfaceRaised:   core.NewColorHex(0xFFFFFFD1),
	SurfaceSidebar:  core.NewColorHex(0xF3F6FFC2),
	StrokeDivider:   core.NewColorHex(0x1B2A4A1A),
	TextSecondary:   core.NewColorHex(0x3A4963),
	TextMuted:       core.NewColorHex(0x63708A),
	TextDisabled:    core.NewColorHex(0x121A2B59),
	ControlHover:    core.NewColorHex(0xFFFFFFD1),
	ControlPressed:  core.NewColorHex(0xDCE6FFB2),
	AccentSubtle:    core.NewColorHex(0x2F6BFF24),
	Success:         core.NewColorHex(0x2FBF6D),
	Warning:         core.NewColorHex(0xFF8A3D),
	Danger:          core.NewColorHex(0xFF4D5E),
	BorderRadius:    10,
}

var AvyosDark = Theme{
	Background:      core.NewColorHex(0x0B1220),
	Foreground:      core.NewColorHex(0xEAF0FF),
	Primary:         core.NewColorHex(0x3B7CFF),
	PrimaryHover:    core.NewColorHex(0x3871E3),
	PrimaryActive:   core.NewColorHex(0x2F67D1),
	Secondary:       core.NewColorHex(0x1A2744B8),
	Border:          core.NewColorHex(0xEAF0FF1F),
	BorderFocused:   core.NewColorHex(0x3B7CFF73),
	InputBackground: core.NewColorHex(0x1A2744B8),
	InputForeground: core.NewColorHex(0xEAF0FF),
	ButtonText:      core.NewColorHex(0xFFFFFFF2),
	SurfaceGlass:    core.NewColorHex(0x121B2FB8),
	SurfaceRaised:   core.NewColorHex(0x18233DD1),
	SurfaceSidebar:  core.NewColorHex(0x0F1830C2),
	StrokeDivider:   core.NewColorHex(0xEAF0FF14),
	TextSecondary:   core.NewColorHex(0xB7C2DA),
	TextMuted:       core.NewColorHex(0x8F9AB4),
	TextDisabled:    core.NewColorHex(0xEAF0FF66),
	ControlHover:    core.NewColorHex(0x223255C7),
	ControlPressed:  core.NewColorHex(0x2A3D66D1),
	AccentSubtle:    core.NewColorHex(0x3B7CFF2E),
	Success:         core.NewColorHex(0x2FBF6D),
	Warning:         core.NewColorHex(0xFF8A3D),
	Danger:          core.NewColorHex(0xFF4D5E),
	BorderRadius:    10,
}

// Backwards-compatible aliases.
var MayurLight = AvyosLight
var MayurDark = AvyosDark
