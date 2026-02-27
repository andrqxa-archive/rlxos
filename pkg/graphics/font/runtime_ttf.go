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

package font

import (
	"image"
	"image/color"
)

// LoadTTFFont creates a gfxfont.Font from raw TTF bytes.
func LoadTTFFont(ttf []byte, opts *Options) (*Font, error) {
	face, err := NewFromTTF(ttf, opts)
	if err != nil {
		return nil, err
	}
	return NewTTFFont(face), nil
}

// LoadTTFFontFile creates a gfxfont.Font from a TTF file path.
func LoadTTFFontFile(path string, opts *Options) (*Font, error) {
	face, err := NewFromFile(path, opts)
	if err != nil {
		return nil, err
	}
	return NewTTFFont(face), nil
}

// NewDefaultTTFFont creates a gfxfont.Font using gofont/goregular.
func NewDefaultTTFFont(opts *Options) (*Font, error) {
	face, err := NewDefault(opts)
	if err != nil {
		return nil, err
	}
	return NewTTFFont(face), nil
}

// NewTTFFont wraps a shaped TTF face for the graphics renderer.
func NewTTFFont(face *Face) *Font {
	if face == nil {
		return nil
	}

	width, _ := face.Measure("M")
	if width <= 0 {
		width, _ = face.Measure(" ")
	}
	if width <= 0 {
		width = 8
	}

	height := face.LineHeight()
	if height <= 0 {
		height = 16
	}

	return &Font{
		Width:    width,
		Height:   height,
		FirstRun: 32,
		LastRun:  126,
		renderer: &ttfRenderer{face: face},
	}
}

// Close releases font resources when supported by the backing renderer.
func (f *Font) Close() error {
	if f == nil || f.renderer == nil {
		return nil
	}
	if c, ok := f.renderer.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}

type ttfRenderer struct {
	face *Face
}

func (r *ttfRenderer) DrawText(buf *Buffer, text string, x, y int, fg, bg Color) {
	if r == nil || r.face == nil || buf == nil {
		return
	}
	dst := &bufferDrawImage{buf: buf}
	var bgColor color.Color
	if bg.A > 0 {
		bgColor = color.NRGBA{R: bg.R, G: bg.G, B: bg.B, A: bg.A}
	}
	r.face.DrawText(dst, text, x, y, color.NRGBA{R: fg.R, G: fg.G, B: fg.B, A: fg.A}, bgColor)
}

func (r *ttfRenderer) DrawGlyph(buf *Buffer, rn rune, x, y int, fg, bg Color) {
	if r == nil || r.face == nil || buf == nil {
		return
	}
	dst := &bufferDrawImage{buf: buf}
	var bgColor color.Color
	if bg.A > 0 {
		bgColor = color.NRGBA{R: bg.R, G: bg.G, B: bg.B, A: bg.A}
	}
	r.face.DrawGlyph(dst, rn, x, y, color.NRGBA{R: fg.R, G: fg.G, B: fg.B, A: fg.A}, bgColor)
}

func (r *ttfRenderer) TextWidth(text string) int {
	if r == nil || r.face == nil {
		return 0
	}
	w, _ := r.face.Measure(text)
	return w
}

func (r *ttfRenderer) TextHeight(text string) int {
	if r == nil || r.face == nil {
		return 0
	}
	_, h := r.face.Measure(text)
	return h
}

func (r *ttfRenderer) Close() error {
	if r == nil || r.face == nil {
		return nil
	}
	return r.face.Close()
}

type bufferDrawImage struct {
	buf *Buffer
}

func (i *bufferDrawImage) ColorModel() color.Model {
	return color.NRGBAModel
}

func (i *bufferDrawImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, i.buf.Width, i.buf.Height)
}

func (i *bufferDrawImage) At(x, y int) color.Color {
	c := i.buf.GetPixel(x, y)
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func (i *bufferDrawImage) Set(x, y int, c color.Color) {
	if i.buf == nil {
		return
	}
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	i.buf.SetPixel(x, y, Color{R: n.R, G: n.G, B: n.B, A: n.A})
}
