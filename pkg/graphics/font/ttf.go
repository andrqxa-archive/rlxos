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
	"errors"
	"image"
	"image/color"
	"image/draw"
	"os"
	"strings"
	"sync"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	defaultSize = 14.0
	defaultDPI  = 72.0
)

// Options controls TTF face creation.
type Options struct {
	Size    float64
	DPI     float64
	Hinting xfont.Hinting
	// HintingSet allows selecting HintingNone explicitly.
	HintingSet bool
}

// ShapedGlyph stores positioning data for a shaped glyph run.
type ShapedGlyph struct {
	Rune    rune
	X       fixed.Int26_6
	Advance fixed.Int26_6
}

// Face is a pure-Go anti-aliased TTF renderer.
type Face struct {
	mu          sync.Mutex
	face        xfont.Face
	lineHeight  int
	ascent      int
	descent     int
	replacement rune
}

// NewFromTTF parses a TTF payload and returns a renderable face.
func NewFromTTF(ttf []byte, opts *Options) (*Face, error) {
	if len(ttf) == 0 {
		return nil, errors.New("font: empty TTF payload")
	}

	cfg := normalizeOptions(opts)
	parsed, err := opentype.Parse(ttf)
	if err != nil {
		return nil, err
	}

	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    cfg.Size,
		DPI:     cfg.DPI,
		Hinting: cfg.Hinting,
	})
	if err != nil {
		return nil, err
	}

	metrics := face.Metrics()
	lineHeight := ceilFixed(metrics.Height)
	if lineHeight <= 0 {
		lineHeight = 1
	}

	return &Face{
		face:        face,
		lineHeight:  lineHeight,
		ascent:      ceilFixed(metrics.Ascent),
		descent:     ceilFixed(metrics.Descent),
		replacement: '?',
	}, nil
}

// NewFromFile loads and parses a TTF file from disk.
func NewFromFile(path string, opts *Options) (*Face, error) {
	if path == "" {
		return nil, errors.New("font: empty font path")
	}
	ttf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewFromTTF(ttf, opts)
}

// NewDefault returns a TTF renderer backed by gofont/goregular.
func NewDefault(opts *Options) (*Face, error) {
	return NewFromTTF(goregular.TTF, opts)
}

// Close releases face resources when applicable.
func (f *Face) Close() error {
	if f == nil || f.face == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.face.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}

// LineHeight returns the configured line height in pixels.
func (f *Face) LineHeight() int {
	if f == nil {
		return 0
	}
	return f.lineHeight
}

// Ascent returns the ascent in pixels.
func (f *Face) Ascent() int {
	if f == nil {
		return 0
	}
	return f.ascent
}

// Descent returns the descent in pixels.
func (f *Face) Descent() int {
	if f == nil {
		return 0
	}
	return f.descent
}

// ShapeLine applies kerning-aware shaping to a single text line.
func (f *Face) ShapeLine(line string) []ShapedGlyph {
	if f == nil || f.face == nil || line == "" {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	return f.shapeLineLocked(line)
}

// DrawText draws anti-aliased text at (x, y), where y is the top of the text box.
func (f *Face) DrawText(dst draw.Image, text string, x, y int, fg, bg color.Color) {
	if f == nil || f.face == nil || dst == nil || text == "" {
		return
	}
	if fg == nil {
		fg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}

	lines := strings.Split(text, "\n")
	fgSrc := image.NewUniform(fg)
	bgA := alpha8(bg)
	var bgSrc *image.Uniform
	if bgA > 0 {
		bgSrc = image.NewUniform(bg)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for i, line := range lines {
		top := y + i*f.lineHeight
		shaped := f.shapeLineLocked(line)
		if bgSrc != nil && len(shaped) > 0 {
			width := ceilFixed(runAdvance(shaped))
			if width > 0 {
				draw.Draw(dst, image.Rect(x, top, x+width, top+f.lineHeight), bgSrc, image.Point{}, draw.Over)
			}
		}

		baseY := fixed.I(top + f.ascent)
		baseX := fixed.I(x)
		for _, glyph := range shaped {
			dot := fixed.Point26_6{X: baseX + glyph.X, Y: baseY}
			dr, mask, maskp, _, ok := f.face.Glyph(dot, glyph.Rune)
			if !ok && glyph.Rune != f.replacement {
				dr, mask, maskp, _, ok = f.face.Glyph(dot, f.replacement)
			}
			if ok && !dr.Empty() {
				draw.DrawMask(dst, dr, fgSrc, image.Point{}, mask, maskp, draw.Over)
			}
		}
	}
}

// DrawGlyph draws one rune at (x, y), where y is the top of the glyph box.
func (f *Face) DrawGlyph(dst draw.Image, r rune, x, y int, fg, bg color.Color) {
	if r == '\n' {
		return
	}
	f.DrawText(dst, string(r), x, y, fg, bg)
}

// Measure returns text width and height in pixels.
func (f *Face) Measure(text string) (int, int) {
	if f == nil || f.face == nil {
		return 0, 0
	}

	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return 0, 0
	}

	maxWidth := 0
	f.mu.Lock()
	for _, line := range lines {
		width := ceilFixed(runAdvance(f.shapeLineLocked(line)))
		if width > maxWidth {
			maxWidth = width
		}
	}
	f.mu.Unlock()

	return maxWidth, len(lines) * f.lineHeight
}

func (f *Face) shapeLineLocked(line string) []ShapedGlyph {
	if line == "" {
		return nil
	}

	shaped := make([]ShapedGlyph, 0, len([]rune(line)))
	var pen fixed.Int26_6
	prev := rune(-1)
	for _, r := range line {
		if prev >= 0 {
			pen += f.face.Kern(prev, r)
		}
		advance := f.glyphAdvanceLocked(r)
		shaped = append(shaped, ShapedGlyph{
			Rune:    r,
			X:       pen,
			Advance: advance,
		})
		pen += advance
		prev = r
	}
	return shaped
}

func (f *Face) glyphAdvanceLocked(r rune) fixed.Int26_6 {
	advance, ok := f.face.GlyphAdvance(r)
	if ok {
		return advance
	}
	if r != f.replacement {
		if fallback, ok := f.face.GlyphAdvance(f.replacement); ok {
			return fallback
		}
	}
	return 0
}

func normalizeOptions(opts *Options) Options {
	cfg := Options{
		Size:    defaultSize,
		DPI:     defaultDPI,
		Hinting: xfont.HintingFull,
	}
	if opts == nil {
		return cfg
	}
	if opts.Size > 0 {
		cfg.Size = opts.Size
	}
	if opts.DPI > 0 {
		cfg.DPI = opts.DPI
	}
	if opts.HintingSet || opts.Hinting != 0 {
		cfg.Hinting = opts.Hinting
	}
	return cfg
}

func runAdvance(shaped []ShapedGlyph) fixed.Int26_6 {
	if len(shaped) == 0 {
		return 0
	}
	last := shaped[len(shaped)-1]
	return last.X + last.Advance
}

func ceilFixed(v fixed.Int26_6) int {
	if v <= 0 {
		return 0
	}
	return int((v + 63) >> 6)
}

func alpha8(c color.Color) uint8 {
	if c == nil {
		return 0
	}
	_, _, _, a := c.RGBA()
	return uint8(a >> 8)
}
