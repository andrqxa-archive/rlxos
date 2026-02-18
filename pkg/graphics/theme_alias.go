package graphics

import gfxtheme "avyos.dev/pkg/graphics/theme"

type Theme = gfxtheme.Theme

var DefaultTheme = gfxtheme.AvyosLight

var AvyosLight = gfxtheme.AvyosLight
var AvyosDark = gfxtheme.AvyosDark

// Backwards-compatible aliases.
var MayurLight = AvyosLight
var MayurDark = AvyosDark
