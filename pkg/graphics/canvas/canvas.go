package canvas

import (
	"image"
	"image/color"
	"image/draw"

	core "avyos.dev/pkg/graphics/pixmap"
)

// Canvas defines the drawing surface API used by renderers.
type Canvas interface {
	Buffer() *core.Buffer
	Image() draw.Image
	Size() (width, height int)

	SetClip(r image.Rectangle)
	ClearClip()
	Clip() (image.Rectangle, bool)

	Clear(fill color.Color)
	SetPixel(x, y int, c color.Color)
	GetPixel(x, y int) color.NRGBA

	FillRect(r image.Rectangle, fill color.Color)
	DrawRect(r image.Rectangle, stroke color.Color)
	FillRoundedRect(r image.Rectangle, radius int, fill color.Color)
	DrawRoundedRect(r image.Rectangle, radius int, stroke color.Color)
	DrawLine(x0, y0, x1, y1 int, stroke color.Color)

	DrawImage(src image.Image, dst image.Rectangle)
	DrawBuffer(src *core.Buffer, dst image.Rectangle)
}

// New returns the default software canvas implementation.
func New(width, height int) Canvas {
	return core.New(width, height)
}
