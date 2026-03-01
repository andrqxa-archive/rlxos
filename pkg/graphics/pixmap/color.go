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

package pixmap

import "image/color"

var (
	ColorBlack       = color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	ColorWhite       = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	ColorRed         = color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	ColorGreen       = color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	ColorBlue        = color.NRGBA{R: 0, G: 0, B: 255, A: 255}
	ColorGray        = color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	ColorLightGray   = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
	ColorDarkGray    = color.NRGBA{R: 64, G: 64, B: 64, A: 255}
	ColorTransparent = color.NRGBA{}
)

func NewColor(r, g, b, a uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: a}
}

func NewColorRGB(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 255}
}

func NewColorHex(hex uint32) color.NRGBA {
	if hex <= 0xFFFFFF {
		return color.NRGBA{
			R: uint8((hex >> 16) & 0xFF),
			G: uint8((hex >> 8) & 0xFF),
			B: uint8(hex & 0xFF),
			A: 255,
		}
	}
	return color.NRGBA{
		R: uint8((hex >> 24) & 0xFF),
		G: uint8((hex >> 16) & 0xFF),
		B: uint8((hex >> 8) & 0xFF),
		A: uint8(hex & 0xFF),
	}
}

func ToNRGBA(c color.Color) color.NRGBA {
	if c == nil {
		return color.NRGBA{}
	}
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

func RGB565(c color.Color) uint16 {
	n := ToNRGBA(c)
	r := uint16(n.R) >> 3
	g := uint16(n.G) >> 2
	b := uint16(n.B) >> 3
	return (r << 11) | (g << 5) | b
}

func BGRA32(c color.Color) uint32 {
	n := ToNRGBA(c)
	return uint32(n.B)<<24 | uint32(n.G)<<16 | uint32(n.R)<<8 | uint32(n.A)
}

func RGBA32(c color.Color) uint32 {
	n := ToNRGBA(c)
	return uint32(n.R)<<24 | uint32(n.G)<<16 | uint32(n.B)<<8 | uint32(n.A)
}

// Blend composites fg over bg in non-premultiplied NRGBA space.
func Blend(fg, bg color.NRGBA) color.NRGBA {
	if fg.A == 0 {
		return bg
	}
	if fg.A == 255 {
		return fg
	}

	sa := uint32(fg.A)
	da := uint32(bg.A)
	invSa := uint32(255 - fg.A)
	outA := sa + (da*invSa+127)/255
	if outA == 0 {
		return color.NRGBA{}
	}

	sr := uint32(fg.R) * sa
	sg := uint32(fg.G) * sa
	sb := uint32(fg.B) * sa

	dr := (uint32(bg.R) * da * invSa) / 255
	dg := (uint32(bg.G) * da * invSa) / 255
	db := (uint32(bg.B) * da * invSa) / 255

	half := outA / 2
	return color.NRGBA{
		R: uint8((sr + dr + half) / outA),
		G: uint8((sg + dg + half) / outA),
		B: uint8((sb + db + half) / outA),
		A: uint8(outA),
	}
}
