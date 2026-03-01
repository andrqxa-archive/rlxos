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

import (
	"image"
	"image/color"
)

// PixelFormat represents the pixel format of a buffer.
type PixelFormat int

const (
	PixelFormatBGRA PixelFormat = iota
	PixelFormatRGBA
	PixelFormatRGB565
)

// Buffer represents a pixel buffer for drawing.
// It implements draw.Image with Bounds/At/Set methods.
type Buffer struct {
	Width  int
	Height int
	Stride int
	Format PixelFormat
	Data   []byte
	clipOn bool
	clip   image.Rectangle
}

// NewBuffer creates a new buffer with the given dimensions (BGRA format).
func NewBuffer(width, height int) *Buffer {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	stride := width * 4
	return &Buffer{
		Width:  width,
		Height: height,
		Stride: stride,
		Format: PixelFormatBGRA,
		Data:   make([]byte, stride*height),
	}
}

// Resize resizes the buffer, reusing backing memory when capacity allows.
func (b *Buffer) Resize(width, height int) {
	if b == nil {
		return
	}
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	bytesPerPixel := 4
	if b.Format == PixelFormatRGB565 {
		bytesPerPixel = 2
	}
	stride := width * bytesPerPixel
	size := stride * height
	if size < 0 {
		size = 0
	}

	if size > cap(b.Data) {
		b.Data = make([]byte, size)
	} else {
		b.Data = b.Data[:size]
	}
	b.Width = width
	b.Height = height
	b.Stride = stride
	b.ClearClip()
}

func (b *Buffer) boundsRect() image.Rectangle {
	if b == nil {
		return image.Rectangle{}
	}
	return image.Rect(0, 0, b.Width, b.Height)
}

// SetClip restricts subsequent drawing operations to r intersected with bounds.
func (b *Buffer) SetClip(r image.Rectangle) {
	if b == nil {
		return
	}
	b.clip = r.Intersect(b.boundsRect())
	b.clipOn = true
}

// ClearClip removes any active drawing clip.
func (b *Buffer) ClearClip() {
	if b == nil {
		return
	}
	b.clipOn = false
	b.clip = image.Rectangle{}
}

// Clip returns the active clip rectangle and whether clipping is enabled.
func (b *Buffer) Clip() (image.Rectangle, bool) {
	if b == nil || !b.clipOn {
		return image.Rectangle{}, false
	}
	return b.clip, true
}

// ColorModel returns NRGBA model for image/draw interoperability.
func (b *Buffer) ColorModel() color.Model {
	return color.NRGBAModel
}

// Bounds returns buffer bounds for image/draw interoperability.
func (b *Buffer) Bounds() image.Rectangle {
	return b.boundsRect()
}

// At returns the pixel color at x,y.
func (b *Buffer) At(x, y int) color.Color {
	return b.GetPixel(x, y)
}

// Set writes a pixel using color.Color for image/draw interoperability.
func (b *Buffer) Set(x, y int, c color.Color) {
	b.SetPixel(x, y, c)
}

// SetPixel sets a pixel at x,y.
func (b *Buffer) SetPixel(x, y int, c color.Color) {
	if b == nil {
		return
	}
	if x < 0 || x >= b.Width || y < 0 || y >= b.Height {
		return
	}
	if b.clipOn && !RectContainsXY(b.clip, x, y) {
		return
	}

	n := ToNRGBA(c)
	switch b.Format {
	case PixelFormatBGRA:
		offset := y*b.Stride + x*4
		b.Data[offset] = n.B
		b.Data[offset+1] = n.G
		b.Data[offset+2] = n.R
		b.Data[offset+3] = n.A
	case PixelFormatRGBA:
		offset := y*b.Stride + x*4
		b.Data[offset] = n.R
		b.Data[offset+1] = n.G
		b.Data[offset+2] = n.B
		b.Data[offset+3] = n.A
	case PixelFormatRGB565:
		offset := y*b.Stride + x*2
		p := RGB565(n)
		b.Data[offset] = byte(p & 0xFF)
		b.Data[offset+1] = byte(p >> 8)
	}
}

// GetPixel returns pixel color at x,y.
func (b *Buffer) GetPixel(x, y int) color.NRGBA {
	if b == nil || x < 0 || x >= b.Width || y < 0 || y >= b.Height {
		return color.NRGBA{}
	}
	switch b.Format {
	case PixelFormatBGRA:
		offset := y*b.Stride + x*4
		return color.NRGBA{
			R: b.Data[offset+2],
			G: b.Data[offset+1],
			B: b.Data[offset],
			A: b.Data[offset+3],
		}
	case PixelFormatRGBA:
		offset := y*b.Stride + x*4
		return color.NRGBA{
			R: b.Data[offset],
			G: b.Data[offset+1],
			B: b.Data[offset+2],
			A: b.Data[offset+3],
		}
	case PixelFormatRGB565:
		offset := y*b.Stride + x*2
		p := uint16(b.Data[offset]) | uint16(b.Data[offset+1])<<8
		return color.NRGBA{
			R: uint8((p >> 11) << 3),
			G: uint8(((p >> 5) & 0x3F) << 2),
			B: uint8((p & 0x1F) << 3),
			A: 255,
		}
	default:
		return color.NRGBA{}
	}
}

// Clear fills entire buffer.
func (b *Buffer) Clear(c color.Color) {
	if b == nil || b.Width == 0 || b.Height == 0 {
		return
	}
	n := ToNRGBA(c)
	if b.Format == PixelFormatBGRA {
		rowBytes := b.Width * 4
		for x := 0; x < b.Width; x++ {
			off := x * 4
			b.Data[off] = n.B
			b.Data[off+1] = n.G
			b.Data[off+2] = n.R
			b.Data[off+3] = n.A
		}
		firstRow := b.Data[:rowBytes]
		for y := 1; y < b.Height; y++ {
			copy(b.Data[y*b.Stride:y*b.Stride+rowBytes], firstRow)
		}
		return
	}
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			b.SetPixel(x, y, n)
		}
	}
}

// FillRect fills r with c.
func (b *Buffer) FillRect(r image.Rectangle, c color.Color) {
	if b == nil {
		return
	}
	r = r.Intersect(b.boundsRect())
	if b.clipOn {
		r = r.Intersect(b.clip)
	}
	if r.Empty() {
		return
	}

	n := ToNRGBA(c)
	if b.Format == PixelFormatBGRA {
		rowBytes := r.Dx() * 4
		dstOff := r.Min.Y*b.Stride + r.Min.X*4
		for x := 0; x < r.Dx(); x++ {
			off := dstOff + x*4
			b.Data[off] = n.B
			b.Data[off+1] = n.G
			b.Data[off+2] = n.R
			b.Data[off+3] = n.A
		}
		firstRow := b.Data[dstOff : dstOff+rowBytes]
		for y := r.Min.Y + 1; y < r.Max.Y; y++ {
			rowOff := y*b.Stride + r.Min.X*4
			copy(b.Data[rowOff:rowOff+rowBytes], firstRow)
		}
		return
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			b.SetPixel(x, y, n)
		}
	}
}

// DrawRect draws a rectangle outline.
func (b *Buffer) DrawRect(r image.Rectangle, c color.Color) {
	if b == nil || r.Empty() {
		return
	}
	for x := r.Min.X; x < r.Max.X; x++ {
		b.SetPixel(x, r.Min.Y, c)
		b.SetPixel(x, r.Max.Y-1, c)
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		b.SetPixel(r.Min.X, y, c)
		b.SetPixel(r.Max.X-1, y, c)
	}
}

// DrawLine draws a line using Bresenham algorithm.
func (b *Buffer) DrawLine(x0, y0, x1, y1 int, c color.Color) {
	if b == nil {
		return
	}
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
