/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package framebuffer

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"avyos.dev/pkg/fs"
	graphics "avyos.dev/pkg/graphics/input"
	"avyos.dev/pkg/simd"
)

const (
	fbioGetVScreenInfo = 0x4600
	fbioGetFScreenInfo = 0x4602

	// VT/KD ioctls for switching to graphics mode
	kdSetMode  = 0x4B3A
	kdText     = 0x00
	kdGraphics = 0x01
)

// fbVarScreenInfo mirrors the Linux fb_var_screeninfo structure.
type fbVarScreenInfo struct {
	XRes         uint32
	YRes         uint32
	XResVirtual  uint32
	YResVirtual  uint32
	XOffset      uint32
	YOffset      uint32
	BitsPerPixel uint32
	Grayscale    uint32
	Red          fbBitField
	Green        fbBitField
	Blue         fbBitField
	Transp       fbBitField
	Nonstd       uint32
	Activate     uint32
	Height       uint32
	Width        uint32
	AccelFlags   uint32
	PixClock     uint32
	LeftMargin   uint32
	RightMargin  uint32
	UpperMargin  uint32
	LowerMargin  uint32
	HSyncLen     uint32
	VSyncLen     uint32
	Sync         uint32
	VMode        uint32
	Rotate       uint32
	Colorspace   uint32
	Reserved     [4]uint32
}

// fbBitField describes the bit layout for a color component.
type fbBitField struct {
	Offset   uint32
	Length   uint32
	MsbRight uint32
}

// fbFixScreenInfo mirrors the Linux fb_fix_screeninfo structure.
type fbFixScreenInfo struct {
	ID           [16]byte
	SMEMStart    uint64
	SMEMLen      uint32
	Type         uint32
	TypeAux      uint32
	Visual       uint32
	XPanStep     uint16
	YPanStep     uint16
	YWrapStep    uint16
	_            uint16
	LineLength   uint32
	MMIOStart    uint64
	MMIOLen      uint32
	Accel        uint32
	Capabilities uint16
	Reserved     [2]uint16
}

// Backend represents the Linux framebuffer backend.
type Backend struct {
	file       *os.File
	ttyFile    *os.File
	data       []byte
	varInfo    fbVarScreenInfo
	fixInfo    fbFixScreenInfo
	buffer     *graphics.Buffer
	backBuffer *graphics.Buffer
}

// New creates a new framebuffer backend.
func New() *Backend {
	return &Backend{}
}

// Open opens the framebuffer device.
func (b *Backend) Open() error {
	return b.OpenDevice(fs.Resolve("device:fb0"))
}

// OpenDevice opens a specific framebuffer device.
func (b *Backend) OpenDevice(device string) error {
	var err error
	b.file, err = os.OpenFile(device, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open framebuffer: %w", err)
	}

	// Get variable screen info
	if err := b.ioctl(fbioGetVScreenInfo, unsafe.Pointer(&b.varInfo)); err != nil {
		b.file.Close()
		return fmt.Errorf("failed to get variable screen info: %w", err)
	}

	// Get fixed screen info
	if err := b.ioctl(fbioGetFScreenInfo, unsafe.Pointer(&b.fixInfo)); err != nil {
		b.file.Close()
		return fmt.Errorf("failed to get fixed screen info: %w", err)
	}

	// Calculate size and mmap
	size := int(b.fixInfo.LineLength) * int(b.varInfo.YRes)
	b.data, err = syscall.Mmap(
		int(b.file.Fd()),
		0,
		size,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		b.file.Close()
		return fmt.Errorf("failed to mmap framebuffer: %w", err)
	}

	// Switch active TTY to graphics mode so kernel doesn't draw text over fb
	b.setGraphicsMode()

	// Create buffers
	b.buffer = b.createBuffer()
	b.backBuffer = graphics.NewBuffer(int(b.varInfo.XRes), int(b.varInfo.YRes))

	return nil
}

// Close closes the framebuffer device and restores text mode.
func (b *Backend) Close() error {
	b.restoreTextMode()
	if b.data != nil {
		syscall.Munmap(b.data)
		b.data = nil
	}
	if b.file != nil {
		return b.file.Close()
	}
	return nil
}

// Size returns the screen dimensions.
func (b *Backend) Size() (width, height int) {
	return int(b.varInfo.XRes), int(b.varInfo.YRes)
}

// Buffer returns the back buffer for drawing.
func (b *Backend) Buffer() *graphics.Buffer {
	return b.backBuffer
}

// Flush copies the entire back buffer to the screen.
func (b *Backend) Flush() error {
	w, h := int(b.varInfo.XRes), int(b.varInfo.YRes)
	return b.FlushRects([]graphics.Rect{{X: 0, Y: 0, W: w, H: h}})
}

// FlushRect copies only the given rectangle from the back buffer to the screen.
func (b *Backend) FlushRect(r graphics.Rect) error {
	return b.FlushRects([]graphics.Rect{r})
}

// FlushRects copies multiple rectangles from the back buffer to the screen.
func (b *Backend) FlushRects(rects []graphics.Rect) error {
	if b.data == nil || b.backBuffer == nil {
		return fmt.Errorf("framebuffer not initialized")
	}
	if len(rects) == 0 {
		return nil
	}

	width := int(b.varInfo.XRes)
	height := int(b.varInfo.YRes)
	bpp := int(b.varInfo.BitsPerPixel)
	stride := int(b.fixInfo.LineLength)

	for _, r := range rects {
		// Clamp rect to screen bounds
		x0 := r.X
		y0 := r.Y
		x1 := r.X + r.W
		y1 := r.Y + r.H
		if x0 < 0 {
			x0 = 0
		}
		if y0 < 0 {
			y0 = 0
		}
		if x1 > width {
			x1 = width
		}
		if y1 > height {
			y1 = height
		}
		if x0 >= x1 || y0 >= y1 {
			continue
		}

		switch bpp {
		case 32:
			b.flushRect32(x0, y0, x1, y1, stride)
		case 24:
			b.flushRect24(x0, y0, x1, y1, stride)
		case 16:
			b.flushRect16(x0, y0, x1, y1, stride)
		default:
			return fmt.Errorf("unsupported bits per pixel: %d", bpp)
		}
	}

	return nil
}

func (b *Backend) flushRect32(x0, y0, x1, y1, stride int) {
	// Fast path: if fb pixel layout matches backBuffer BGRA, bulk copy rows.
	// Strides may differ (fb may have alignment padding) — we use per-row offsets.
	if b.backBuffer.Format == graphics.PixelFormatBGRA &&
		b.varInfo.Blue.Offset == 0 && b.varInfo.Green.Offset == 8 &&
		b.varInfo.Red.Offset == 16 {
		// Row-by-row copy — pixel formats match, only strides may differ
		rowBytes := (x1 - x0) * 4
		for y := y0; y < y1; y++ {
			srcOff := y*b.backBuffer.Stride + x0*4
			dstOff := y*stride + x0*4
			simd.CopyBGRA(
				b.data[dstOff:dstOff+rowBytes],
				b.backBuffer.Data[srcOff:srcOff+rowBytes],
				x1-x0,
			)
		}
		return
	}

	// Slow path: pixel-by-pixel with color channel remapping
	rOff := int(b.varInfo.Red.Offset / 8)
	gOff := int(b.varInfo.Green.Offset / 8)
	bOff := int(b.varInfo.Blue.Offset / 8)
	hasAlpha := b.varInfo.Transp.Length > 0
	aOff := int(b.varInfo.Transp.Offset / 8)

	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := b.backBuffer.GetPixel(x, y)
			off := y*stride + x*4
			b.data[off+rOff] = c.R
			b.data[off+gOff] = c.G
			b.data[off+bOff] = c.B
			if hasAlpha {
				b.data[off+aOff] = c.A
			}
		}
	}
}

func (b *Backend) flushRect24(x0, y0, x1, y1, stride int) {
	rOff := int(b.varInfo.Red.Offset / 8)
	gOff := int(b.varInfo.Green.Offset / 8)
	bOff := int(b.varInfo.Blue.Offset / 8)

	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := b.backBuffer.GetPixel(x, y)
			off := y*stride + x*3
			b.data[off+rOff] = c.R
			b.data[off+gOff] = c.G
			b.data[off+bOff] = c.B
		}
	}
}

func (b *Backend) flushRect16(x0, y0, x1, y1, stride int) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := b.backBuffer.GetPixel(x, y)
			off := y*stride + x*2
			binary.LittleEndian.PutUint16(b.data[off:], c.RGB565())
		}
	}
}

func (b *Backend) createBuffer() *graphics.Buffer {
	width := int(b.varInfo.XRes)
	height := int(b.varInfo.YRes)
	stride := int(b.fixInfo.LineLength)

	buf := &graphics.Buffer{
		Width:  width,
		Height: height,
		Stride: stride,
		Data:   b.data,
	}

	// Determine format based on bit layout
	switch b.varInfo.BitsPerPixel {
	case 16:
		buf.Format = graphics.PixelFormatRGB565
	case 24, 32:
		if b.varInfo.Red.Offset > b.varInfo.Blue.Offset {
			buf.Format = graphics.PixelFormatRGBA
		} else {
			buf.Format = graphics.PixelFormatBGRA
		}
	}

	return buf
}

func (b *Backend) ioctl(request uint, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		b.file.Fd(),
		uintptr(request),
		uintptr(arg),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// PixelFormat returns the detected pixel format.
func (b *Backend) PixelFormat() graphics.PixelFormat {
	return b.buffer.Format
}

// HasSystemCursor returns false since the framebuffer has no system cursor.
func (b *Backend) HasSystemCursor() bool {
	return false
}

// Start is a no-op for the framebuffer backend.
func (b *Backend) Start() {}

// setGraphicsMode switches the active TTY to KD_GRAPHICS mode.
func (b *Backend) setGraphicsMode() {
	// Try /dev/tty0 first (requires root), then /dev/tty
	for _, path := range []string{fs.Resolve("device:tty0"), fs.Resolve("device:tty")} {
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			continue
		}
		mode := uintptr(kdGraphics)
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(kdSetMode), mode)
		if errno == 0 {
			b.ttyFile = f
			return
		}
		f.Close()
	}
}

// restoreTextMode restores the TTY to KD_TEXT mode.
func (b *Backend) restoreTextMode() {
	if b.ttyFile == nil {
		return
	}
	mode := uintptr(kdText)
	syscall.Syscall(syscall.SYS_IOCTL, b.ttyFile.Fd(), uintptr(kdSetMode), mode)
	b.ttyFile.Close()
	b.ttyFile = nil
}

// Info returns information about the framebuffer.
func (b *Backend) Info() string {
	return fmt.Sprintf(
		"Framebuffer: %dx%d, %d bpp, stride %d, format: R@%d G@%d B@%d",
		b.varInfo.XRes, b.varInfo.YRes,
		b.varInfo.BitsPerPixel,
		b.fixInfo.LineLength,
		b.varInfo.Red.Offset,
		b.varInfo.Green.Offset,
		b.varInfo.Blue.Offset,
	)
}
