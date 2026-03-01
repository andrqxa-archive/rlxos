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

	core "avyos.dev/pkg/graphics/pixmap"
	gfxsvg "avyos.dev/pkg/graphics/svg"
)

// DecodeOptions controls image decode behavior.
type DecodeOptions struct {
	SVG gfxsvg.DecodeFileOptions
}

var decodedImageCache sync.Map // key: "<absPath>|<reqSize>|<svgOpts>" => *core.Buffer

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

// DecodeImageToBuffer decodes raster formats and delegates SVG decoding to pkg/graphics/svg.
func DecodeImageToBuffer(path string, reqSize int, opts ...DecodeOptions) (*core.Buffer, error) {
	cfg := DecodeOptions{}
	if len(opts) > 0 {
		cfg = opts[0]
	}
	return decodeImageToBuffer(path, reqSize, cfg)
}

func decodeImageToBuffer(path string, reqSize int, opts DecodeOptions) (*core.Buffer, error) {
	cachePath := path
	if abs, err := filepath.Abs(path); err == nil {
		cachePath = abs
	}
	cacheKey := fmt.Sprintf("%s|%d|%s", cachePath, reqSize, svgDecodeCacheKey(opts))
	if v, ok := decodedImageCache.Load(cacheKey); ok {
		if b, ok := v.(*core.Buffer); ok && b != nil {
			return b, nil
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	var out *core.Buffer
	var err error
	if ext == ".svg" {
		out, err = gfxsvg.DecodeFile(path, reqSize, opts.SVG)
	} else {
		out, err = decodeRasterToBuffer(path)
	}
	if err != nil {
		return nil, err
	}
	decodedImageCache.Store(cacheKey, out)
	return out, nil
}

func decodeRasterToBuffer(path string) (*core.Buffer, error) {
	img, err := DecodeFile(path)
	if err != nil {
		return nil, err
	}
	return BufferFromImage(img), nil
}

func svgDecodeCacheKey(opts DecodeOptions) string {
	if opts.SVG.TextFace == nil {
		return "nil"
	}
	face := opts.SVG.TextFace
	return fmt.Sprintf("%dx%d:%d", face.Width, face.Height, len(face.Glyphs))
}
