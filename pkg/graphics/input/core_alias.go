package input

import gfxcore "avyos.dev/pkg/graphics/core"

type (
	Color       = gfxcore.Color
	Point       = gfxcore.Point
	Rect        = gfxcore.Rect
	PixelFormat = gfxcore.PixelFormat
	Buffer      = gfxcore.Buffer
)

const (
	PixelFormatBGRA   = gfxcore.PixelFormatBGRA
	PixelFormatRGBA   = gfxcore.PixelFormatRGBA
	PixelFormatRGB565 = gfxcore.PixelFormatRGB565
)

var (
	ColorBlack       = gfxcore.ColorBlack
	ColorWhite       = gfxcore.ColorWhite
	ColorRed         = gfxcore.ColorRed
	ColorGreen       = gfxcore.ColorGreen
	ColorBlue        = gfxcore.ColorBlue
	ColorGray        = gfxcore.ColorGray
	ColorLightGray   = gfxcore.ColorLightGray
	ColorDarkGray    = gfxcore.ColorDarkGray
	ColorTransparent = gfxcore.ColorTransparent

	NewColor    = gfxcore.NewColor
	NewColorRGB = gfxcore.NewColorRGB
	NewColorHex = gfxcore.NewColorHex
	NewRect     = gfxcore.NewRect
	NewBuffer   = gfxcore.NewBuffer
)
