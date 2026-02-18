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

package core

// Color represents an RGBA color.
type Color struct {
	R, G, B, A uint8
}

// Common colors
var (
	ColorBlack       = Color{0, 0, 0, 255}
	ColorWhite       = Color{255, 255, 255, 255}
	ColorRed         = Color{255, 0, 0, 255}
	ColorGreen       = Color{0, 255, 0, 255}
	ColorBlue        = Color{0, 0, 255, 255}
	ColorGray        = Color{128, 128, 128, 255}
	ColorLightGray   = Color{200, 200, 200, 255}
	ColorDarkGray    = Color{64, 64, 64, 255}
	ColorTransparent = Color{0, 0, 0, 0}
)

// RGBA returns the color as 32-bit RGBA value.
func (c Color) RGBA() uint32 {
	return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
}

// BGRA returns the color as 32-bit BGRA value (common framebuffer format).
func (c Color) BGRA() uint32 {
	return uint32(c.B)<<24 | uint32(c.G)<<16 | uint32(c.R)<<8 | uint32(c.A)
}

// RGB565 returns the color as 16-bit RGB565 value.
func (c Color) RGB565() uint16 {
	r := uint16(c.R) >> 3
	g := uint16(c.G) >> 2
	b := uint16(c.B) >> 3
	return (r << 11) | (g << 5) | b
}

// Blend blends this color over a background color using alpha compositing.
func (c Color) Blend(bg Color) Color {
	if c.A == 0 {
		return bg
	}
	if c.A == 255 {
		return c
	}

	sa := uint32(c.A)
	da := uint32(bg.A)
	invSa := uint32(255 - c.A)
	outA := sa + (da*invSa+127)/255
	if outA == 0 {
		return ColorTransparent
	}

	sr := uint32(c.R) * sa
	sg := uint32(c.G) * sa
	sb := uint32(c.B) * sa

	dr := (uint32(bg.R) * da * invSa) / 255
	dg := (uint32(bg.G) * da * invSa) / 255
	db := (uint32(bg.B) * da * invSa) / 255

	half := outA / 2
	return Color{
		R: uint8((sr + dr + half) / outA),
		G: uint8((sg + dg + half) / outA),
		B: uint8((sb + db + half) / outA),
		A: uint8(outA),
	}
}

// NewColor creates a new color from RGBA values.
func NewColor(r, g, b, a uint8) Color {
	return Color{R: r, G: g, B: b, A: a}
}

// NewColorRGB creates a new opaque color from RGB values.
func NewColorRGB(r, g, b uint8) Color {
	return Color{R: r, G: g, B: b, A: 255}
}

// NewColorHex creates a color from a hex value (0xRRGGBB or 0xRRGGBBAA).
func NewColorHex(hex uint32) Color {
	if hex <= 0xFFFFFF {
		return Color{
			R: uint8((hex >> 16) & 0xFF),
			G: uint8((hex >> 8) & 0xFF),
			B: uint8(hex & 0xFF),
			A: 255,
		}
	}
	return Color{
		R: uint8((hex >> 24) & 0xFF),
		G: uint8((hex >> 16) & 0xFF),
		B: uint8((hex >> 8) & 0xFF),
		A: uint8(hex & 0xFF),
	}
}
