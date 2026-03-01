package svg

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
)

// DecodeFileOptions controls high-level SVG file decoding behavior.
type DecodeFileOptions struct {
	TextFace *TextFace
	// DecodeImage overrides nested <image href="..."> loading.
	// If nil, nested images are decoded recursively by extension.
	DecodeImage func(path string, reqSize int) (*core.Buffer, error)
}

var decodedSVGCache sync.Map // key: "<absPath>|<reqSize>|<textFace>" => *core.Buffer

// DecodeFile decodes an SVG file into a pixel buffer.
// Nested image references are resolved relative to the SVG's directory.
func DecodeFile(path string, reqSize int, opts ...DecodeFileOptions) (*core.Buffer, error) {
	cfg := DecodeFileOptions{}
	if len(opts) > 0 {
		cfg = opts[0]
	}
	return decodeImageToBuffer(path, reqSize, cfg, map[string]struct{}{})
}

func decodeImageToBuffer(path string, reqSize int, opts DecodeFileOptions, stack map[string]struct{}) (*core.Buffer, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("empty image path")
	}

	cachePath := path
	if abs, err := filepath.Abs(path); err == nil {
		cachePath = abs
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".svg" {
		return decodeRasterToBuffer(path)
	}

	cacheable := opts.DecodeImage == nil
	cacheKey := ""
	if cacheable {
		cacheKey = fmt.Sprintf("%s|%d|%s", cachePath, reqSize, textFaceCacheKey(opts.TextFace))
		if v, ok := decodedSVGCache.Load(cacheKey); ok {
			if b, ok := v.(*core.Buffer); ok && b != nil {
				return b, nil
			}
		}
	}

	if _, seen := stack[cachePath]; seen {
		return nil, fmt.Errorf("circular svg image reference: %q", cachePath)
	}
	stack[cachePath] = struct{}{}
	defer delete(stack, cachePath)

	baseDir := filepath.Dir(path)
	decodeNested := func(ref string, nestedReqSize int) (*core.Buffer, error) {
		refPath := resolveImageRef(baseDir, ref)
		if refPath == "" {
			return nil, fmt.Errorf("empty image href")
		}
		if opts.DecodeImage != nil {
			return opts.DecodeImage(refPath, nestedReqSize)
		}
		return decodeImageToBuffer(refPath, nestedReqSize, opts, stack)
	}

	buf, err := Decode(path, reqSize, Options{
		TextFace: opts.TextFace,
		DecodeImage: func(path string, reqSize int) (*core.Buffer, error) {
			return decodeNested(path, reqSize)
		},
	})
	if err != nil {
		return nil, err
	}
	if cacheable {
		decodedSVGCache.Store(cacheKey, buf)
	}
	return buf, nil
}

func decodeRasterToBuffer(path string) (*core.Buffer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image %q: %w", path, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image %q: %w", path, err)
	}

	b := img.Bounds()
	out := core.NewBuffer(b.Dx(), b.Dy())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			out.SetPixel(x, y, core.NewColor(uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)))
		}
	}
	return out, nil
}

func resolveImageRef(baseDir, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if filepath.IsAbs(href) {
		return href
	}
	if baseDir == "" {
		return href
	}
	return filepath.Clean(filepath.Join(baseDir, href))
}

func textFaceCacheKey(face *TextFace) string {
	if face == nil {
		return "nil"
	}
	return fmt.Sprintf("%dx%d:%d", face.Width, face.Height, len(face.Glyphs))
}
