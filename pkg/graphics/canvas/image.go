package canvas

import (
	"image"

	core "avyos.dev/pkg/graphics/pixmap"
)

// BufferFromImage copies any image.Image into a core buffer.
func BufferFromImage(img image.Image) *core.Buffer {
	if img == nil {
		return nil
	}
	b := img.Bounds()
	out := core.NewBuffer(b.Dx(), b.Dy())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			out.SetPixel(x, y, core.NewColor(uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)))
		}
	}
	return out
}
