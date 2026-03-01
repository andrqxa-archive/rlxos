package engine

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strings"

	gfxcanvas "avyos.dev/pkg/graphics/canvas"
	core "avyos.dev/pkg/graphics/pixmap"
)

type imageState struct {
	srcPath string
	source  *core.Buffer
	opaque  *core.Buffer
	opaqueK string
	scaled  *core.Buffer
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

func drawImage(e *Element, buf *core.Buffer) {
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
			bg := color.NRGBA{}
			if _, ok := e.attrs["srcOpaqueBg"]; ok {
				bg = e.AttrColor("srcOpaqueBg", color.NRGBA{})
			}
			st.opaque = makeOpaqueImage(st.source, bg)
			st.opaqueK = key
			st.scaled = nil
			st.scaledK = ""
		}
		src = st.opaque
	}

	bounds := e.contentArea()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	srcW, srcH := src.Width, src.Height
	if srcW <= 0 || srcH <= 0 {
		return
	}

	mode := strings.ToLower(strings.TrimSpace(e.Attr("scaleMode", "contain")))
	fullSrc := core.RectXYWH(0, 0, srcW, srcH)
	var srcVariant string
	if opaque {
		srcVariant = "opaque:" + st.opaqueK
	} else {
		srcVariant = "source"
	}

	getScaled := func(key string, srcRect image.Rectangle, w, h int) *core.Buffer {
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
		x := bounds.Min.X + (bounds.Dx()-srcW)/2
		y := bounds.Min.Y + (bounds.Dy()-srcH)/2
		buf.Blit(src, x, y)
	case "cover":
		srcRect := coverSrcRect(srcW, srcH, bounds)
		key := fmt.Sprintf("%s|%s|cover|%d:%d:%d:%d|%d:%d",
			st.srcPath, srcVariant, srcRect.Min.X, srcRect.Min.Y, srcRect.Dx(), srcRect.Dy(), bounds.Dx(), bounds.Dy())
		scaled := getScaled(key, srcRect, bounds.Dx(), bounds.Dy())
		if scaled == nil {
			return
		}
		buf.Blit(scaled, bounds.Min.X, bounds.Min.Y)
	case "stretch":
		key := fmt.Sprintf("%s|%s|stretch|%d:%d", st.srcPath, srcVariant, bounds.Dx(), bounds.Dy())
		scaled := getScaled(key, fullSrc, bounds.Dx(), bounds.Dy())
		if scaled == nil {
			return
		}
		buf.Blit(scaled, bounds.Min.X, bounds.Min.Y)
	default: // contain
		dst := fitRect(srcW, srcH, bounds)
		key := fmt.Sprintf("%s|%s|contain|%d:%d", st.srcPath, srcVariant, dst.Dx(), dst.Dy())
		scaled := getScaled(key, fullSrc, dst.Dx(), dst.Dy())
		if scaled == nil {
			return
		}
		buf.Blit(scaled, dst.Min.X, dst.Min.Y)
	}
}

func makeOpaqueImage(src *core.Buffer, bg color.NRGBA) *core.Buffer {
	if src == nil {
		return nil
	}
	out := core.NewBuffer(src.Width, src.Height)
	blendWithBG := bg.A > 0
	for y := 0; y < src.Height; y++ {
		for x := 0; x < src.Width; x++ {
			c := src.GetPixel(x, y)
			if c.A == 0 {
				out.SetPixel(x, y, core.ColorTransparent)
				continue
			}
			if blendWithBG {
				flat := core.Blend(c, bg)
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

func loadImageFile(path string) (*core.Buffer, error) {
	reqSize := 0
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		// Keep vector icons crisp when UI scales them up in controls.
		reqSize = 512
	}
	buf, err := gfxcanvas.DecodeImageToBuffer(path, reqSize)
	if err != nil {
		return nil, fmt.Errorf("load image: %w", err)
	}
	return buf, nil
}

func fitRect(srcW, srcH int, bounds image.Rectangle) image.Rectangle {
	scaleX := bounds.Dx() * 1000 / srcW
	scaleY := bounds.Dy() * 1000 / srcH
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	w := srcW * scale / 1000
	h := srcH * scale / 1000
	return core.RectXYWH(bounds.Min.X+(bounds.Dx()-w)/2, bounds.Min.Y+(bounds.Dy()-h)/2, w, h)
}

func coverSrcRect(srcW, srcH int, bounds image.Rectangle) image.Rectangle {
	srcAspect := srcW * 1000 / srcH
	dstAspect := bounds.Dx() * 1000 / bounds.Dy()
	if srcAspect > dstAspect {
		newW := srcH * bounds.Dx() / bounds.Dy()
		return core.RectXYWH((srcW-newW)/2, 0, newW, srcH)
	}
	newH := srcW * bounds.Dy() / bounds.Dx()
	return core.RectXYWH(0, (srcH-newH)/2, srcW, newH)
}

func scaleImageSmooth(src *core.Buffer, srcRect image.Rectangle, w, h int) *core.Buffer {
	if src == nil || w <= 0 || h <= 0 || srcRect.Dx() <= 0 || srcRect.Dy() <= 0 {
		return nil
	}

	working := src
	workRect := srcRect
	if srcRect.Min.X != 0 || srcRect.Min.Y != 0 || srcRect.Dx() != src.Width || srcRect.Dy() != src.Height {
		working = src.SubBuffer(srcRect)
		workRect = core.RectXYWH(0, 0, working.Width, working.Height)
	}

	// Progressive downscale reduces aliasing and makes icon edges smoother
	// than a single large minification step.
	for workRect.Dx() > w*2 || workRect.Dy() > h*2 {
		nextW := workRect.Dx() / 2
		nextH := workRect.Dy() / 2
		if nextW < w {
			nextW = w
		}
		if nextH < h {
			nextH = h
		}
		if nextW == workRect.Dx() && nextH == workRect.Dy() {
			break
		}

		next := core.NewBuffer(nextW, nextH)
		next.BlitScaled(working, workRect, core.RectXYWH(0, 0, nextW, nextH))
		working = next
		workRect = core.RectXYWH(0, 0, nextW, nextH)
	}

	out := core.NewBuffer(w, h)
	out.BlitScaled(working, workRect, core.RectXYWH(0, 0, w, h))
	return out
}
