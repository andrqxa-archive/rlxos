package pixmap

import (
	"image"
	"image/color"
	"math"

	"avyos.dev/pkg/simd"
)

// Blit copies src onto b at x,y with alpha blending.
func (b *Buffer) Blit(src *Buffer, x, y int) {
	if b == nil || src == nil {
		return
	}
	for sy := 0; sy < src.Height; sy++ {
		for sx := 0; sx < src.Width; sx++ {
			c := src.GetPixel(sx, sy)
			if c.A == 0 {
				continue
			}
			if c.A == 255 {
				b.SetPixel(x+sx, y+sy, c)
				continue
			}
			bg := b.GetPixel(x+sx, y+sy)
			b.SetPixel(x+sx, y+sy, Blend(c, bg))
		}
	}
}

// BlitOpaque copies src onto b and forces source alpha to 255.
func (b *Buffer) BlitOpaque(src *Buffer, x, y int) {
	if b == nil || src == nil {
		return
	}
	if b.Format == src.Format && b.Format == PixelFormatBGRA {
		for sy := 0; sy < src.Height; sy++ {
			dy := y + sy
			if dy < 0 || dy >= b.Height {
				continue
			}
			srcOff := sy * src.Stride
			dstOff := dy*b.Stride + x*4
			sx0 := 0
			if x < 0 {
				sx0 = -x
			}
			sx1 := src.Width
			if x+sx1 > b.Width {
				sx1 = b.Width - x
			}
			if sx0 >= sx1 {
				continue
			}
			simd.CopyBGRAOpaque(
				b.Data[dstOff+sx0*4:dstOff+sx1*4],
				src.Data[srcOff+sx0*4:srcOff+sx1*4],
				sx1-sx0,
			)
		}
		return
	}

	for sy := 0; sy < src.Height; sy++ {
		for sx := 0; sx < src.Width; sx++ {
			c := src.GetPixel(sx, sy)
			c.A = 255
			b.SetPixel(x+sx, y+sy, c)
		}
	}
}

// BlitRect copies srcRect from src onto b at x,y with alpha blending.
func (b *Buffer) BlitRect(src *Buffer, srcRect image.Rectangle, x, y int) {
	if b == nil || src == nil {
		return
	}
	for sy := srcRect.Min.Y; sy < srcRect.Max.Y; sy++ {
		for sx := srcRect.Min.X; sx < srcRect.Max.X; sx++ {
			c := src.GetPixel(sx, sy)
			if c.A == 0 {
				continue
			}
			dx := x + (sx - srcRect.Min.X)
			dy := y + (sy - srcRect.Min.Y)
			if c.A == 255 {
				b.SetPixel(dx, dy, c)
				continue
			}
			bg := b.GetPixel(dx, dy)
			b.SetPixel(dx, dy, Blend(c, bg))
		}
	}
}

// BlitOpaqueRect copies srcRect from src onto b at x,y and forces alpha to 255.
func (b *Buffer) BlitOpaqueRect(src *Buffer, srcRect image.Rectangle, x, y int) {
	if b == nil || src == nil || srcRect.Empty() {
		return
	}
	sx0, sy0 := srcRect.Min.X, srcRect.Min.Y
	dx0, dy0 := x, y
	w, h := srcRect.Dx(), srcRect.Dy()

	if sx0 < 0 {
		dx0 -= sx0
		w += sx0
		sx0 = 0
	}
	if sy0 < 0 {
		dy0 -= sy0
		h += sy0
		sy0 = 0
	}
	if sx0+w > src.Width {
		w = src.Width - sx0
	}
	if sy0+h > src.Height {
		h = src.Height - sy0
	}

	if dx0 < 0 {
		sx0 -= dx0
		w += dx0
		dx0 = 0
	}
	if dy0 < 0 {
		sy0 -= dy0
		h += dy0
		dy0 = 0
	}
	if dx0+w > b.Width {
		w = b.Width - dx0
	}
	if dy0+h > b.Height {
		h = b.Height - dy0
	}
	if w <= 0 || h <= 0 {
		return
	}

	if b.Format == src.Format && b.Format == PixelFormatBGRA {
		rowBytes := w * 4
		for row := 0; row < h; row++ {
			srcOff := (sy0+row)*src.Stride + sx0*4
			dstOff := (dy0+row)*b.Stride + dx0*4
			simd.CopyBGRAOpaque(
				b.Data[dstOff:dstOff+rowBytes],
				src.Data[srcOff:srcOff+rowBytes],
				w,
			)
		}
		return
	}

	for row := 0; row < h; row++ {
		for col := 0; col < w; col++ {
			c := src.GetPixel(sx0+col, sy0+row)
			c.A = 255
			b.SetPixel(dx0+col, dy0+row, c)
		}
	}
}

// SubBuffer copies r region into a new buffer.
func (b *Buffer) SubBuffer(r image.Rectangle) *Buffer {
	if b == nil || r.Empty() {
		return NewBuffer(0, 0)
	}
	out := NewBuffer(r.Dx(), r.Dy())
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			out.SetPixel(x, y, b.GetPixel(r.Min.X+x, r.Min.Y+y))
		}
	}
	return out
}

// BlitScaled copies srcRect from src to dstRect on b using bilinear sampling.
func (b *Buffer) BlitScaled(src *Buffer, srcRect, dstRect image.Rectangle) {
	if b == nil || src == nil || dstRect.Empty() || srcRect.Empty() {
		return
	}
	srcMinX := srcRect.Min.X
	srcMinY := srcRect.Min.Y
	srcMaxX := srcRect.Max.X - 1
	srcMaxY := srcRect.Max.Y - 1

	scaleX := float64(srcRect.Dx()) / float64(dstRect.Dx())
	scaleY := float64(srcRect.Dy()) / float64(dstRect.Dy())

	for dy := 0; dy < dstRect.Dy(); dy++ {
		syF := float64(srcRect.Min.Y) + (float64(dy)+0.5)*scaleY - 0.5
		y0 := int(math.Floor(syF))
		ty := syF - float64(y0)
		if y0 < srcMinY {
			y0 = srcMinY
			ty = 0
		} else if y0 >= srcMaxY {
			y0 = srcMaxY
			ty = 0
		}
		y1 := y0 + 1
		if y1 > srcMaxY {
			y1 = srcMaxY
		}

		for dx := 0; dx < dstRect.Dx(); dx++ {
			sxF := float64(srcRect.Min.X) + (float64(dx)+0.5)*scaleX - 0.5
			x0 := int(math.Floor(sxF))
			tx := sxF - float64(x0)
			if x0 < srcMinX {
				x0 = srcMinX
				tx = 0
			} else if x0 >= srcMaxX {
				x0 = srcMaxX
				tx = 0
			}
			x1 := x0 + 1
			if x1 > srcMaxX {
				x1 = srcMaxX
			}

			c00 := src.GetPixel(x0, y0)
			c10 := src.GetPixel(x1, y0)
			c01 := src.GetPixel(x0, y1)
			c11 := src.GetPixel(x1, y1)
			c := lerpColor2D(c00, c10, c01, c11, tx, ty)
			if c.A == 0 {
				continue
			}
			px, py := dstRect.Min.X+dx, dstRect.Min.Y+dy
			if c.A == 255 {
				b.SetPixel(px, py, c)
			} else {
				bg := b.GetPixel(px, py)
				b.SetPixel(px, py, Blend(c, bg))
			}
		}
	}
}

func lerpColor2D(c00, c10, c01, c11 color.NRGBA, tx, ty float64) color.NRGBA {
	w00 := (1 - tx) * (1 - ty)
	w10 := tx * (1 - ty)
	w01 := (1 - tx) * ty
	w11 := tx * ty

	a := float64(c00.A)*w00 + float64(c10.A)*w10 + float64(c01.A)*w01 + float64(c11.A)*w11
	if a <= 0.5 {
		return color.NRGBA{}
	}

	rPM := (float64(c00.R)*float64(c00.A))*w00 +
		(float64(c10.R)*float64(c10.A))*w10 +
		(float64(c01.R)*float64(c01.A))*w01 +
		(float64(c11.R)*float64(c11.A))*w11
	gPM := (float64(c00.G)*float64(c00.A))*w00 +
		(float64(c10.G)*float64(c10.A))*w10 +
		(float64(c01.G)*float64(c01.A))*w01 +
		(float64(c11.G)*float64(c11.A))*w11
	bPM := (float64(c00.B)*float64(c00.A))*w00 +
		(float64(c10.B)*float64(c10.A))*w10 +
		(float64(c01.B)*float64(c01.A))*w01 +
		(float64(c11.B)*float64(c11.A))*w11

	invA := 1.0 / a
	return color.NRGBA{
		R: uint8(rPM*invA + 0.5),
		G: uint8(gPM*invA + 0.5),
		B: uint8(bPM*invA + 0.5),
		A: uint8(a + 0.5),
	}
}
