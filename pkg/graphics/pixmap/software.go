package pixmap

import (
	"image"
	"image/draw"
)

// New creates a software canvas with the given dimensions.
func New(width, height int) *Buffer {
	return NewBuffer(width, height)
}

// Wrap treats an existing buffer as a software canvas.
func Wrap(buf *Buffer) *Buffer {
	return buf
}

// Buffer returns the backing buffer.
func (b *Buffer) Buffer() *Buffer {
	return b
}

// Image returns the backing draw.Image.
func (b *Buffer) Image() draw.Image {
	if b == nil {
		return nil
	}
	return b
}

// Size returns canvas dimensions.
func (b *Buffer) Size() (width, height int) {
	if b == nil {
		return 0, 0
	}
	return b.Width, b.Height
}

// DrawImage scales and draws any image.Image into dst.
func (b *Buffer) DrawImage(src image.Image, dst image.Rectangle) {
	if b == nil || src == nil {
		return
	}
	if dst.Empty() {
		return
	}
	var srcBuf *Buffer
	switch s := src.(type) {
	case *Buffer:
		srcBuf = s
	default:
		srcBuf = BufferFromImage(src)
	}
	if srcBuf == nil || srcBuf.Width <= 0 || srcBuf.Height <= 0 {
		return
	}
	srcRect := image.Rect(0, 0, srcBuf.Width, srcBuf.Height)
	b.BlitScaled(srcBuf, srcRect, dst)
}

// DrawBuffer scales and draws a source buffer into dst.
func (b *Buffer) DrawBuffer(src *Buffer, dst image.Rectangle) {
	if src == nil {
		return
	}
	b.DrawImage(src, dst)
}
