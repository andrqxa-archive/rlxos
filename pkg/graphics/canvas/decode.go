package canvas

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"avyos.dev/pkg/graphics/core"
	gfxsvg "avyos.dev/pkg/graphics/svg"
)

type Buffer = core.Buffer

// TextFace provides bitmap glyph metrics used by SVG text fallback.
type TextFace struct {
	Width  int
	Height int
	Glyphs map[rune][]byte
}

// DecodeOptions controls image decode behavior.
type DecodeOptions struct {
	TextFace *TextFace
}

var decodedImageCache sync.Map // key: "<absPath>|<reqSize>|<textFace>" => *Buffer

// DecodeFile decodes a raster image file into an image.Image.
func DecodeFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image %q: %w", path, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image %q: %w", path, err)
	}
	return img, nil
}

// DecodeImageToBuffer decodes raster formats directly and rasterizes SVG files.
// reqSize is used for SVG rasterization. If <= 0, a size is inferred from SVG metadata.
func DecodeImageToBuffer(path string, reqSize int, opts ...DecodeOptions) (*Buffer, error) {
	cfg := DecodeOptions{}
	if len(opts) > 0 {
		cfg = opts[0]
	}
	return decodeImageToBuffer(path, reqSize, cfg)
}

// DecodeSVGToBuffer rasterizes an SVG file using optional text-face settings.
func DecodeSVGToBuffer(path string, reqSize int, opts ...DecodeOptions) (*Buffer, error) {
	cfg := DecodeOptions{}
	if len(opts) > 0 {
		cfg = opts[0]
	}
	return decodeSVGPure(path, reqSize, cfg)
}

func decodeImageToBuffer(path string, reqSize int, opts DecodeOptions) (*Buffer, error) {
	cachePath := path
	if abs, err := filepath.Abs(path); err == nil {
		cachePath = abs
	}
	cacheKey := fmt.Sprintf("%s|%d|%s", cachePath, reqSize, textFaceCacheKey(opts.TextFace))
	if v, ok := decodedImageCache.Load(cacheKey); ok {
		if b, ok := v.(*Buffer); ok && b != nil {
			return b, nil
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	var out *Buffer
	var err error
	if ext == ".svg" {
		out, err = decodeSVGPure(path, reqSize, opts)
	} else {
		out, err = decodeRasterToBuffer(path)
	}
	if err != nil {
		return nil, err
	}
	decodedImageCache.Store(cacheKey, out)
	return out, nil
}

func decodeRasterToBuffer(path string) (*Buffer, error) {
	img, err := DecodeFile(path)
	if err != nil {
		return nil, err
	}
	return imageToBuffer(img), nil
}

func imageToBuffer(img image.Image) *Buffer {
	b := img.Bounds()
	out := core.NewBuffer(b.Dx(), b.Dy())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bb, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			out.SetPixel(x, y, core.NewColor(uint8(r>>8), uint8(g>>8), uint8(bb>>8), uint8(a>>8)))
		}
	}
	return out
}

func decodeSVGPure(path string, reqSize int, opts DecodeOptions) (*Buffer, error) {
	var tf *gfxsvg.TextFace
	if opts.TextFace != nil {
		tf = &gfxsvg.TextFace{
			Width:  opts.TextFace.Width,
			Height: opts.TextFace.Height,
			Glyphs: opts.TextFace.Glyphs,
		}
	}
	return gfxsvg.Decode(path, reqSize, gfxsvg.Options{
		TextFace: tf,
		DecodeImage: func(path string, reqSize int) (*Buffer, error) {
			return decodeImageToBuffer(path, reqSize, opts)
		},
	})
}

func textFaceCacheKey(face *TextFace) string {
	if face == nil {
		return "nil"
	}
	return fmt.Sprintf("%dx%d:%d", face.Width, face.Height, len(face.Glyphs))
}
