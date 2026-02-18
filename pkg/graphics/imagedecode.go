package graphics

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"sync"

	"avyos.dev/pkg/graphics/raster"
)

var decodedImageCache sync.Map // key: "<absPath>|<reqSize>" => *Buffer

// DecodeImageToBuffer decodes raster formats directly and rasterizes SVG files.
// reqSize is used for SVG rasterization. If <= 0, a size is inferred from SVG metadata.
func DecodeImageToBuffer(path string, reqSize int) (*Buffer, error) {
	cachePath := path
	if abs, err := filepath.Abs(path); err == nil {
		cachePath = abs
	}
	cacheKey := fmt.Sprintf("%s|%d", cachePath, reqSize)
	if v, ok := decodedImageCache.Load(cacheKey); ok {
		if b, ok := v.(*Buffer); ok && b != nil {
			return b, nil
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	var out *Buffer
	var err error
	if ext == ".svg" {
		out, err = decodeSVGPure(path, reqSize)
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
	img, err := raster.DecodeFile(path)
	if err != nil {
		return nil, err
	}
	return imageToBuffer(img), nil
}

func imageToBuffer(img image.Image) *Buffer {
	b := img.Bounds()
	out := NewBuffer(b.Dx(), b.Dy())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bb, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			out.SetPixel(x, y, NewColor(uint8(r>>8), uint8(g>>8), uint8(bb>>8), uint8(a>>8)))
		}
	}
	return out
}
