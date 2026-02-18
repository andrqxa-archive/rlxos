package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"avyos.dev/pkg/graphics"
)

type imageState struct {
	srcPath string
	source  *graphics.Buffer
	opaque  *graphics.Buffer
	opaqueK string
	scaled  *graphics.Buffer
	scaledK string
}

func (e *Element) getImageState() *imageState {
	if st, ok := e.state.(*imageState); ok {
		return st
	}
	st := &imageState{}
	e.state = st
	return st
}

func drawImage(e *Element, buf *graphics.Buffer) {
	srcPath := strings.TrimSpace(e.Attr("src", ""))
	if srcPath == "" {
		return
	}

	st := e.getImageState()
	if st.source == nil || st.srcPath != srcPath {
		src, err := loadImageFile(srcPath)
		if err != nil {
			return
		}
		st.srcPath = srcPath
		st.source = src
		st.opaque = nil
		st.opaqueK = ""
		st.scaled = nil
		st.scaledK = ""
	}
	if st.source == nil {
		return
	}
	src := st.source
	opaque := false
	if _, ok := e.attrs["srcOpaque"]; ok {
		opaque = e.AttrBool("srcOpaque", false)
	} else {
		// Treat icons (src on non-Image widgets) as opaque by default.
		opaque = !strings.EqualFold(e.Tag(), "image")
	}
	if opaque {
		key := "auto"
		if _, ok := e.attrs["srcOpaque"]; ok {
			key = e.Attr("srcOpaque", "false")
		}
		if v, ok := e.attrs["srcOpaqueBg"]; ok {
			key += "|bg:" + v
		}
		if st.opaque == nil || st.opaqueK != key {
			bg := graphics.Color{}
			if _, ok := e.attrs["srcOpaqueBg"]; ok {
				bg = e.AttrColor("srcOpaqueBg", graphics.Color{})
			}
			st.opaque = makeOpaqueImage(st.source, bg)
			st.opaqueK = key
			st.scaled = nil
			st.scaledK = ""
		}
		src = st.opaque
	}

	bounds := e.contentArea()
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}
	srcW, srcH := src.Width, src.Height
	if srcW <= 0 || srcH <= 0 {
		return
	}

	mode := strings.ToLower(strings.TrimSpace(e.Attr("scaleMode", "contain")))
	fullSrc := graphics.Rect{X: 0, Y: 0, W: srcW, H: srcH}
	var srcVariant string
	if opaque {
		srcVariant = "opaque:" + st.opaqueK
	} else {
		srcVariant = "source"
	}

	getScaled := func(key string, srcRect graphics.Rect, w, h int) *graphics.Buffer {
		if w <= 0 || h <= 0 {
			return nil
		}
		if st.scaled != nil && st.scaledK == key && st.scaled.Width == w && st.scaled.Height == h {
			return st.scaled
		}
		out := scaleImageSmooth(src, srcRect, w, h)
		if out == nil {
			return nil
		}
		st.scaled = out
		st.scaledK = key
		return out
	}

	switch mode {
	case "none":
		x := bounds.X + (bounds.W-srcW)/2
		y := bounds.Y + (bounds.H-srcH)/2
		buf.Blit(src, x, y)
	case "cover":
		srcRect := coverSrcRect(srcW, srcH, bounds)
		key := fmt.Sprintf("%s|%s|cover|%d:%d:%d:%d|%d:%d",
			st.srcPath, srcVariant, srcRect.X, srcRect.Y, srcRect.W, srcRect.H, bounds.W, bounds.H)
		scaled := getScaled(key, srcRect, bounds.W, bounds.H)
		if scaled == nil {
			return
		}
		buf.Blit(scaled, bounds.X, bounds.Y)
	case "stretch":
		key := fmt.Sprintf("%s|%s|stretch|%d:%d", st.srcPath, srcVariant, bounds.W, bounds.H)
		scaled := getScaled(key, fullSrc, bounds.W, bounds.H)
		if scaled == nil {
			return
		}
		buf.Blit(scaled, bounds.X, bounds.Y)
	default: // contain
		dst := fitRect(srcW, srcH, bounds)
		key := fmt.Sprintf("%s|%s|contain|%d:%d", st.srcPath, srcVariant, dst.W, dst.H)
		scaled := getScaled(key, fullSrc, dst.W, dst.H)
		if scaled == nil {
			return
		}
		buf.Blit(scaled, dst.X, dst.Y)
	}
}

func makeOpaqueImage(src *graphics.Buffer, bg graphics.Color) *graphics.Buffer {
	if src == nil {
		return nil
	}
	out := graphics.NewBuffer(src.Width, src.Height)
	blendWithBG := bg.A > 0
	for y := 0; y < src.Height; y++ {
		for x := 0; x < src.Width; x++ {
			c := src.GetPixel(x, y)
			if c.A == 0 {
				out.SetPixel(x, y, graphics.ColorTransparent)
				continue
			}
			if blendWithBG {
				flat := c.Blend(bg)
				flat.A = 255
				out.SetPixel(x, y, flat)
				continue
			}
			// No explicit background: keep source RGB and force alpha to opaque.
			c.A = 255
			out.SetPixel(x, y, c)
		}
	}
	return out
}

func loadImageFile(path string) (*graphics.Buffer, error) {
	reqSize := 0
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		// Keep vector icons crisp when UI scales them up in controls.
		reqSize = 512
	}
	buf, err := graphics.DecodeImageToBuffer(path, reqSize)
	if err != nil {
		return nil, fmt.Errorf("load image: %w", err)
	}
	return buf, nil
}

func fitRect(srcW, srcH int, bounds graphics.Rect) graphics.Rect {
	scaleX := bounds.W * 1000 / srcW
	scaleY := bounds.H * 1000 / srcH
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	w := srcW * scale / 1000
	h := srcH * scale / 1000
	return graphics.Rect{
		X: bounds.X + (bounds.W-w)/2,
		Y: bounds.Y + (bounds.H-h)/2,
		W: w,
		H: h,
	}
}

func coverSrcRect(srcW, srcH int, bounds graphics.Rect) graphics.Rect {
	srcAspect := srcW * 1000 / srcH
	dstAspect := bounds.W * 1000 / bounds.H
	if srcAspect > dstAspect {
		newW := srcH * bounds.W / bounds.H
		return graphics.Rect{X: (srcW - newW) / 2, Y: 0, W: newW, H: srcH}
	}
	newH := srcW * bounds.H / bounds.W
	return graphics.Rect{X: 0, Y: (srcH - newH) / 2, W: srcW, H: newH}
}

func scaleImageSmooth(src *graphics.Buffer, srcRect graphics.Rect, w, h int) *graphics.Buffer {
	if src == nil || w <= 0 || h <= 0 || srcRect.W <= 0 || srcRect.H <= 0 {
		return nil
	}

	working := src
	workRect := srcRect
	if srcRect.X != 0 || srcRect.Y != 0 || srcRect.W != src.Width || srcRect.H != src.Height {
		working = src.SubBuffer(srcRect)
		workRect = graphics.Rect{X: 0, Y: 0, W: working.Width, H: working.Height}
	}

	// Progressive downscale reduces aliasing and makes icon edges smoother
	// than a single large minification step.
	for workRect.W > w*2 || workRect.H > h*2 {
		nextW := workRect.W / 2
		nextH := workRect.H / 2
		if nextW < w {
			nextW = w
		}
		if nextH < h {
			nextH = h
		}
		if nextW == workRect.W && nextH == workRect.H {
			break
		}

		next := graphics.NewBuffer(nextW, nextH)
		next.BlitScaled(working, workRect, graphics.Rect{X: 0, Y: 0, W: nextW, H: nextH})
		working = next
		workRect = graphics.Rect{X: 0, Y: 0, W: nextW, H: nextH}
	}

	out := graphics.NewBuffer(w, h)
	out.BlitScaled(working, workRect, graphics.Rect{X: 0, Y: 0, W: w, H: h})
	return out
}
