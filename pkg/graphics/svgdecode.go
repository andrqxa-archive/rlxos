package graphics

import gfxsvg "avyos.dev/pkg/graphics/svg"

func decodeSVGPure(path string, reqSize int) (*Buffer, error) {
	var tf *gfxsvg.TextFace
	if DefaultFont != nil {
		tf = &gfxsvg.TextFace{
			Width:  DefaultFont.Width,
			Height: DefaultFont.Height,
			Glyphs: DefaultFont.Glyphs,
		}
	}
	return gfxsvg.Decode(path, reqSize, gfxsvg.Options{
		TextFace:    tf,
		DecodeImage: DecodeImageToBuffer,
	})
}
