package theme

import (
	"image/color"

	core "avyos.dev/pkg/graphics/pixmap"
)

// Theme holds the colors used for rendering widgets.
type Theme struct {
	Background      color.NRGBA
	Foreground      color.NRGBA
	Primary         color.NRGBA
	AccentAlt       color.NRGBA
	PrimaryHover    color.NRGBA
	PrimaryActive   color.NRGBA
	Secondary       color.NRGBA
	Border          color.NRGBA
	BorderFocused   color.NRGBA
	InputBackground color.NRGBA
	InputForeground color.NRGBA
	ButtonText      color.NRGBA
	SurfaceGlass    color.NRGBA
	SurfaceRaised   color.NRGBA
	SurfaceSidebar  color.NRGBA
	StrokeDivider   color.NRGBA
	TextSecondary   color.NRGBA
	TextMuted       color.NRGBA
	TextDisabled    color.NRGBA
	ControlHover    color.NRGBA
	ControlPressed  color.NRGBA
	AccentSubtle    color.NRGBA
	Success         color.NRGBA
	Warning         color.NRGBA
	Danger          color.NRGBA
	BorderRadius    int
}

// DefaultTheme is the default color theme.
var DefaultTheme = AvyosLight

var AvyosLight = Theme{
	Background:      core.NewColorHex(0xF4F8FF),
	Foreground:      core.NewColorHex(0x101A2B),
	Primary:         core.NewColorHex(0x0D63F3),
	AccentAlt:       core.NewColorHex(0x11B6E8),
	PrimaryHover:    core.NewColorHex(0x0B57D6),
	PrimaryActive:   core.NewColorHex(0x0848B2),
	Secondary:       core.NewColorHex(0xFFFFFF9E),
	Border:          core.NewColorHex(0x101A2B24),
	BorderFocused:   core.NewColorHex(0x0D63F352),
	InputBackground: core.NewColorHex(0xFFFFFFC7),
	InputForeground: core.NewColorHex(0x101A2B),
	ButtonText:      core.NewColorHex(0xFFFFFF),
	SurfaceGlass:    core.NewColorHex(0xFFFFFF9E),
	SurfaceRaised:   core.NewColorHex(0xFFFFFFC7),
	SurfaceSidebar:  core.NewColorHex(0xFFFFFFB8),
	StrokeDivider:   core.NewColorHex(0x101A2B1A),
	TextSecondary:   core.NewColorHex(0x434D64),
	TextMuted:       core.NewColorHex(0x58627A),
	TextDisabled:    core.NewColorHex(0x101A2B66),
	ControlHover:    core.NewColorHex(0x0D63F31F),
	ControlPressed:  core.NewColorHex(0x0D63F333),
	AccentSubtle:    core.NewColorHex(0x0D63F31F),
	Success:         core.NewColorHex(0x25C46A),
	Warning:         core.NewColorHex(0xFF8F24),
	Danger:          core.NewColorHex(0xFF5F3A),
	BorderRadius:    14,
}

var AvyosDark = Theme{
	Background:      core.NewColorHex(0x070D19),
	Foreground:      core.NewColorHex(0xE8EFFF),
	Primary:         core.NewColorHex(0x0D63F3),
	AccentAlt:       core.NewColorHex(0x11B6E8),
	PrimaryHover:    core.NewColorHex(0x149ED9),
	PrimaryActive:   core.NewColorHex(0x0A4EBF),
	Secondary:       core.NewColorHex(0x0F1627D1),
	Border:          core.NewColorHex(0xE8EFFF29),
	BorderFocused:   core.NewColorHex(0x11B6E83D),
	InputBackground: core.NewColorHex(0x0F1627E0),
	InputForeground: core.NewColorHex(0xE8EFFF),
	ButtonText:      core.NewColorHex(0xFFFFFF),
	SurfaceGlass:    core.NewColorHex(0x0D15269E),
	SurfaceRaised:   core.NewColorHex(0x0D1526C7),
	SurfaceSidebar:  core.NewColorHex(0x0F1627C2),
	StrokeDivider:   core.NewColorHex(0xE8EFFF1A),
	TextSecondary:   core.NewColorHex(0xC4D1ED),
	TextMuted:       core.NewColorHex(0xADBAD8),
	TextDisabled:    core.NewColorHex(0xE8EFFF66),
	ControlHover:    core.NewColorHex(0x1A2947D9),
	ControlPressed:  core.NewColorHex(0x223763E5),
	AccentSubtle:    core.NewColorHex(0x11B6E83D),
	Success:         core.NewColorHex(0x25C46A),
	Warning:         core.NewColorHex(0xFF8F24),
	Danger:          core.NewColorHex(0xFF5F3A),
	BorderRadius:    14,
}
