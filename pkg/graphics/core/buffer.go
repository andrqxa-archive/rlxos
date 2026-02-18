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

// PixelFormat represents the pixel format of a buffer.
type PixelFormat int

const (
	PixelFormatBGRA PixelFormat = iota
	PixelFormatRGBA
	PixelFormatRGB565
)

// Buffer represents a pixel buffer for drawing.
type Buffer struct {
	Width  int
	Height int
	Stride int
	Format PixelFormat
	Data   []byte
	clipOn bool
	clip   Rect
}

// NewBuffer creates a new buffer with the given dimensions (BGRA format).
func NewBuffer(width, height int) *Buffer {
	stride := width * 4
	return &Buffer{
		Width:  width,
		Height: height,
		Stride: stride,
		Format: PixelFormatBGRA,
		Data:   make([]byte, stride*height),
	}
}

// SetClip restricts subsequent drawing operations to the given rectangle.
// The clip is intersected with the buffer bounds.
func (b *Buffer) SetClip(r Rect) {
	r = r.Intersection(Rect{W: b.Width, H: b.Height})
	b.clip = r
	b.clipOn = true
}

// ClearClip removes any active drawing clip.
func (b *Buffer) ClearClip() {
	b.clipOn = false
	b.clip = Rect{}
}

// Clip returns the active clip rectangle and whether clipping is enabled.
func (b *Buffer) Clip() (Rect, bool) {
	if !b.clipOn {
		return Rect{}, false
	}
	return b.clip, true
}

func (b *Buffer) clipRect() (Rect, bool) {
	if !b.clipOn {
		return Rect{}, false
	}
	return b.clip, true
}

// SetPixel sets a pixel at the given coordinates.
func (b *Buffer) SetPixel(x, y int, c Color) {
	if x < 0 || x >= b.Width || y < 0 || y >= b.Height {
		return
	}
	if b.clipOn && !b.clip.ContainsXY(x, y) {
		return
	}

	switch b.Format {
	case PixelFormatBGRA:
		offset := y*b.Stride + x*4
		b.Data[offset] = c.B
		b.Data[offset+1] = c.G
		b.Data[offset+2] = c.R
		b.Data[offset+3] = c.A
	case PixelFormatRGBA:
		offset := y*b.Stride + x*4
		b.Data[offset] = c.R
		b.Data[offset+1] = c.G
		b.Data[offset+2] = c.B
		b.Data[offset+3] = c.A
	case PixelFormatRGB565:
		offset := y*b.Stride + x*2
		pixel := c.RGB565()
		b.Data[offset] = byte(pixel & 0xFF)
		b.Data[offset+1] = byte(pixel >> 8)
	}
}

// GetPixel returns the color at the given coordinates.
func (b *Buffer) GetPixel(x, y int) Color {
	if x < 0 || x >= b.Width || y < 0 || y >= b.Height {
		return ColorTransparent
	}

	switch b.Format {
	case PixelFormatBGRA:
		offset := y*b.Stride + x*4
		return Color{B: b.Data[offset], G: b.Data[offset+1], R: b.Data[offset+2], A: b.Data[offset+3]}
	case PixelFormatRGBA:
		offset := y*b.Stride + x*4
		return Color{R: b.Data[offset], G: b.Data[offset+1], B: b.Data[offset+2], A: b.Data[offset+3]}
	case PixelFormatRGB565:
		offset := y*b.Stride + x*2
		pixel := uint16(b.Data[offset]) | uint16(b.Data[offset+1])<<8
		return Color{R: uint8((pixel >> 11) << 3), G: uint8(((pixel >> 5) & 0x3F) << 2), B: uint8((pixel & 0x1F) << 3), A: 255}
	}
	return ColorTransparent
}

// Clear fills the entire buffer with a color.
func (b *Buffer) Clear(c Color) {
	if b.Format == PixelFormatBGRA {
		if b.Height == 0 || b.Width == 0 {
			return
		}
		rowBytes := b.Width * 4
		for x := 0; x < b.Width; x++ {
			off := x * 4
			b.Data[off] = c.B
			b.Data[off+1] = c.G
			b.Data[off+2] = c.R
			b.Data[off+3] = c.A
		}
		firstRow := b.Data[:rowBytes]
		for y := 1; y < b.Height; y++ {
			copy(b.Data[y*b.Stride:y*b.Stride+rowBytes], firstRow)
		}
		return
	}
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			b.SetPixel(x, y, c)
		}
	}
}

// FillRect fills a rectangle with a color.
func (b *Buffer) FillRect(r Rect, c Color) {
	if b.Format == PixelFormatBGRA && r.W > 0 && r.H > 0 {
		x0, y0 := r.X, r.Y
		x1, y1 := r.X+r.W, r.Y+r.H
		if x0 < 0 {
			x0 = 0
		}
		if y0 < 0 {
			y0 = 0
		}
		if x1 > b.Width {
			x1 = b.Width
		}
		if y1 > b.Height {
			y1 = b.Height
		}
		if b.clipOn {
			if x0 < b.clip.X {
				x0 = b.clip.X
			}
			if y0 < b.clip.Y {
				y0 = b.clip.Y
			}
			clipX1 := b.clip.X + b.clip.W
			clipY1 := b.clip.Y + b.clip.H
			if x1 > clipX1 {
				x1 = clipX1
			}
			if y1 > clipY1 {
				y1 = clipY1
			}
		}
		if x0 >= x1 || y0 >= y1 {
			return
		}
		rowBytes := (x1 - x0) * 4
		dstOff := y0*b.Stride + x0*4
		for x := 0; x < x1-x0; x++ {
			off := dstOff + x*4
			b.Data[off] = c.B
			b.Data[off+1] = c.G
			b.Data[off+2] = c.R
			b.Data[off+3] = c.A
		}
		firstRow := b.Data[dstOff : dstOff+rowBytes]
		for y := y0 + 1; y < y1; y++ {
			copy(b.Data[y*b.Stride+x0*4:y*b.Stride+x0*4+rowBytes], firstRow)
		}
		return
	}
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			b.SetPixel(x, y, c)
		}
	}
}

// DrawRect draws a rectangle outline with a color.
func (b *Buffer) DrawRect(r Rect, c Color) {
	for x := r.X; x < r.X+r.W; x++ {
		b.SetPixel(x, r.Y, c)
		b.SetPixel(x, r.Y+r.H-1, c)
	}
	for y := r.Y; y < r.Y+r.H; y++ {
		b.SetPixel(r.X, y, c)
		b.SetPixel(r.X+r.W-1, y, c)
	}
}

// DrawLine draws a line between two points using Bresenham's algorithm.
func (b *Buffer) DrawLine(x0, y0, x1, y1 int, c Color) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy

	for {
		b.SetPixel(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
