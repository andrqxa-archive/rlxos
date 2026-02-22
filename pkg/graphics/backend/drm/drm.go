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

package drm

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/graphics"
	"avyos.dev/pkg/simd"
)

const (
	// TTY mode switching
	kdSetMode  = 0x4B3A
	kdText     = 0x00
	kdGraphics = 0x01
	vtActivate = 0x5606
	vtWaitAct  = 0x5607

	// DRM connector status
	drmConnected    = 1
	drmModeTypePref = 1 << 3

	// Page-flip flags/events
	drmModePageFlipEvent = 0x01
	drmEventVBlank       = 0x01
	drmEventFlipComplete = 0x02
)

// ioctl number encoding: _IOWR('d', nr, size) = (3<<30)|(size<<16)|(0x64<<8)|nr
func ioctlWR(nr, size uintptr) uintptr {
	return (3 << 30) | (size << 16) | (0x64 << 8) | nr
}

// DRM ioctl numbers (computed from struct sizes)
var (
	ioctlGetResources = ioctlWR(0xA0, unsafe.Sizeof(drmModeCardRes{}))
	ioctlGetCrtc      = ioctlWR(0xA1, unsafe.Sizeof(drmModeCrtc{}))
	ioctlSetCrtc      = ioctlWR(0xA2, unsafe.Sizeof(drmModeCrtc{}))
	ioctlGetEncoder   = ioctlWR(0xA6, unsafe.Sizeof(drmModeGetEncoder{}))
	ioctlGetConnector = ioctlWR(0xA7, unsafe.Sizeof(drmModeGetConnector{}))
	ioctlAddFb        = ioctlWR(0xAE, unsafe.Sizeof(drmModeFbCmd{}))
	ioctlRmFb         = ioctlWR(0xAF, unsafe.Sizeof(uint32(0)))
	ioctlPageFlip     = ioctlWR(0xB0, unsafe.Sizeof(drmModeCrtcPageFlip{}))
	ioctlDirtyFb      = ioctlWR(0xB1, unsafe.Sizeof(drmModeFbDirtyCmd{}))
	ioctlCreateDumb   = ioctlWR(0xB2, unsafe.Sizeof(drmModeCreateDumb{}))
	ioctlMapDumb      = ioctlWR(0xB3, unsafe.Sizeof(drmModeMapDumb{}))
	ioctlDestroyDumb  = ioctlWR(0xB4, unsafe.Sizeof(drmModeDestroyDumb{}))
)

// --- DRM structs (Linux UAPI) ---

// drmModeModeInfo: display mode descriptor (68 bytes).
type drmModeModeInfo struct {
	Clock      uint32
	Hdisplay   uint16
	HsyncStart uint16
	HsyncEnd   uint16
	Htotal     uint16
	Hskew      uint16
	Vdisplay   uint16
	VsyncStart uint16
	VsyncEnd   uint16
	Vtotal     uint16
	Vscan      uint16
	Vrefresh   uint32
	Flags      uint32
	Type       uint32
	Name       [32]byte
}

// drmModeCardRes: getresources request/response.
type drmModeCardRes struct {
	FbIDPtr        uint64
	CrtcIDPtr      uint64
	ConnectorIDPtr uint64
	EncoderIDPtr   uint64
	CountFbs       uint32
	CountCrtcs     uint32
	CountConns     uint32
	CountEncoders  uint32
	MinWidth       uint32
	MaxWidth       uint32
	MinHeight      uint32
	MaxHeight      uint32
}

// drmModeGetConnector: connector query.
type drmModeGetConnector struct {
	EncoderIDPtr    uint64
	ModesPtr        uint64
	PropsPtr        uint64
	PropValuesPtr   uint64
	CountModes      uint32
	CountProps      uint32
	CountEncoders   uint32
	EncoderID       uint32
	ConnectorID     uint32
	ConnectorType   uint32
	ConnectorTypeID uint32
	Connection      uint32
	MmWidth         uint32
	MmHeight        uint32
	SubPixel        uint32
	_               uint32
}

// drmModeGetEncoder: encoder query.
type drmModeGetEncoder struct {
	EncoderID      uint32
	EncoderType    uint32
	CrtcID         uint32
	PossibleCrtcs  uint32
	PossibleClones uint32
}

// drmModeCreateDumb: create a dumb scanout buffer.
type drmModeCreateDumb struct {
	Height uint32
	Width  uint32
	Bpp    uint32
	Flags  uint32
	Handle uint32
	Pitch  uint32
	Size   uint64
}

// drmModeFbCmd: register a framebuffer.
type drmModeFbCmd struct {
	FbID   uint32
	Width  uint32
	Height uint32
	Pitch  uint32
	Bpp    uint32
	Depth  uint32
	Handle uint32
}

// drmModeMapDumb: get mmap offset for a dumb buffer.
type drmModeMapDumb struct {
	Handle uint32
	_      uint32
	Offset uint64
}

// drmModeCrtcPageFlip: schedule framebuffer page flip on vblank.
type drmModeCrtcPageFlip struct {
	CrtcID   uint32
	FbID     uint32
	Flags    uint32
	Reserved uint32
	UserData uint64
}

// drmModeClipRect: dirtyfb clip rectangle.
type drmModeClipRect struct {
	X1 uint16
	Y1 uint16
	X2 uint16
	Y2 uint16
}

// drmModeFbDirtyCmd: notify updated framebuffer regions.
type drmModeFbDirtyCmd struct {
	FbID     uint32
	Flags    uint32
	Color    uint32
	NumClips uint32
	ClipsPtr uint64
}

// drmModeCrtc: CRTC state.
type drmModeCrtc struct {
	SetConnectorsPtr uint64
	CountConnectors  uint32
	CrtcID           uint32
	FbID             uint32
	X                uint32
	Y                uint32
	GammaSize        uint32
	ModeValid        uint32
	Mode             drmModeModeInfo
}

// drmModeDestroyDumb: destroy a dumb buffer.
type drmModeDestroyDumb struct {
	Handle uint32
}

// --- Backend ---

// Backend is a DRM/KMS display backend.
type Backend struct {
	file       *os.File
	devPath    string
	ttyFile    *os.File
	ttyFD      uintptr
	ttyFDSet   bool
	width      int
	height     int
	pitch      uint32
	handle     uint32
	fbID       uint32
	data       []byte
	pitchB     uint32
	handleB    uint32
	fbIDB      uint32
	dataB      []byte
	frontIsB   bool
	pageFlip   int8 // 0 unknown, 1 supported, -1 unsupported
	backBuffer *graphics.Buffer
	crtcID     uint32
	connID     uint32
	savedCrtc  drmModeCrtc
	mode       drmModeModeInfo
	dirtyState int8 // 0 unknown, 1 supported, -1 unsupported
	// True after both scanouts have been seeded with a full frame.
	// Needed so first page-flip cannot expose uninitialized regions.
	scanoutsPrimed bool
}

type connectorCandidate struct {
	conn     drmModeGetConnector
	modes    []drmModeModeInfo
	encoders []uint32
	encID    uint32
	crtcID   uint32
	score    int
}

// New creates a new DRM/KMS backend.
func New() *Backend {
	return &Backend{}
}

// Open opens the DRM device, finds a connected display, and sets the mode.
func (b *Backend) Open() error {
	// Try multiple DRM node locations and card indices.
	var err error
	var lastErr error
	for i := 0; i < 8; i++ {
		candidates := []string{fs.Resolve("device:dri/card%d", i)}
		var selected bool
		for _, path := range candidates {
			if path == "" {
				continue
			}
			b.file, err = os.OpenFile(path, os.O_RDWR, 0)
			if err != nil {
				lastErr = err
				continue
			}
			// Skip nodes that cannot provide KMS resources.
			var probe drmModeCardRes
			if probeErr := b.ioctl(ioctlGetResources, unsafe.Pointer(&probe)); probeErr != nil || probe.CountConns == 0 || probe.CountCrtcs == 0 {
				_ = b.file.Close()
				b.file = nil
				if probeErr != nil {
					lastErr = probeErr
				}
				continue
			}

			b.devPath = path
			selected = true
			break
		}
		if selected {
			break
		}
	}
	if b.file == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("no card nodes found")
		}
		return fmt.Errorf("drm: no device found: %w", lastErr)
	}

	// Get resources (two-pass)
	var res drmModeCardRes
	if err := b.ioctl(ioctlGetResources, unsafe.Pointer(&res)); err != nil {
		b.file.Close()
		return fmt.Errorf("drm: getresources: %w", err)
	}

	if res.CountConns == 0 || res.CountCrtcs == 0 {
		b.file.Close()
		return fmt.Errorf("drm: no connectors or CRTCs")
	}

	connIDs := make([]uint32, res.CountConns)
	crtcIDs := make([]uint32, res.CountCrtcs)

	res.ConnectorIDPtr = uint64(uintptr(unsafe.Pointer(&connIDs[0])))
	res.CrtcIDPtr = uint64(uintptr(unsafe.Pointer(&crtcIDs[0])))
	if res.CountEncoders > 0 {
		encIDs := make([]uint32, res.CountEncoders)
		res.EncoderIDPtr = uint64(uintptr(unsafe.Pointer(&encIDs[0])))
	}
	if res.CountFbs > 0 {
		fbIDs := make([]uint32, res.CountFbs)
		res.FbIDPtr = uint64(uintptr(unsafe.Pointer(&fbIDs[0])))
	}

	if err := b.ioctl(ioctlGetResources, unsafe.Pointer(&res)); err != nil {
		b.file.Close()
		return fmt.Errorf("drm: getresources fill: %w", err)
	}

	// Snapshot CRTC state for connector scoring (prefer active CRTC routes).
	crtcState := make(map[uint32]drmModeCrtc, len(crtcIDs))
	for _, id := range crtcIDs {
		var crtc drmModeCrtc
		crtc.CrtcID = id
		if err := b.ioctl(ioctlGetCrtc, unsafe.Pointer(&crtc)); err == nil {
			crtcState[id] = crtc
		}
	}

	// Find connected connector with modes and usable encoder/CRTC routing.
	var selected connectorCandidate
	found := false
	bestScore := -1

	for _, cid := range connIDs {
		var conn drmModeGetConnector
		conn.ConnectorID = cid
		if err := b.ioctl(ioctlGetConnector, unsafe.Pointer(&conn)); err != nil {
			continue
		}
		if conn.Connection != drmConnected || conn.CountModes == 0 {
			continue
		}
		// Second pass: fill modes and encoders
		modes := make([]drmModeModeInfo, conn.CountModes)
		conn.ModesPtr = uint64(uintptr(unsafe.Pointer(&modes[0])))
		var encs []uint32
		if conn.CountEncoders > 0 {
			encs = make([]uint32, conn.CountEncoders)
			conn.EncoderIDPtr = uint64(uintptr(unsafe.Pointer(&encs[0])))
		}
		if conn.CountProps > 0 {
			props := make([]uint32, conn.CountProps)
			propVals := make([]uint64, conn.CountProps)
			conn.PropsPtr = uint64(uintptr(unsafe.Pointer(&props[0])))
			conn.PropValuesPtr = uint64(uintptr(unsafe.Pointer(&propVals[0])))
		}
		if err := b.ioctl(ioctlGetConnector, unsafe.Pointer(&conn)); err != nil {
			continue
		}
		encID, crtcID, ok := b.pickCRTC(conn, encs, crtcIDs)
		if !ok {
			continue
		}
		score := 0
		if crtc, ok := crtcState[crtcID]; ok && crtc.ModeValid != 0 {
			score += 10
		}
		if conn.EncoderID == encID && encID != 0 {
			score += 2
		}
		for _, m := range modes {
			if m.Type&drmModeTypePref != 0 {
				score++
				break
			}
		}
		if !found || score > bestScore {
			selected = connectorCandidate{
				conn:     conn,
				modes:    modes,
				encoders: encs,
				encID:    encID,
				crtcID:   crtcID,
				score:    score,
			}
			bestScore = score
			found = true
		}
	}
	if !found {
		b.file.Close()
		return fmt.Errorf("drm: no connected display found")
	}
	b.connID = selected.conn.ConnectorID

	// Select preferred mode or first available
	b.mode = selected.modes[0]
	for _, m := range selected.modes {
		if m.Type&drmModeTypePref != 0 {
			b.mode = m
			break
		}
	}
	b.width = int(b.mode.Hdisplay)
	b.height = int(b.mode.Vdisplay)

	encID, crtcID := selected.encID, selected.crtcID
	b.crtcID = crtcID

	// Save current CRTC state for restore
	b.savedCrtc.CrtcID = b.crtcID
	b.ioctl(ioctlGetCrtc, unsafe.Pointer(&b.savedCrtc))

	// Create dumb buffer
	scanoutA, err := b.createScanout()
	if err != nil {
		b.file.Close()
		return fmt.Errorf("drm: create scanout A: %w", err)
	}
	b.handle, b.fbID, b.pitch, b.data = scanoutA.handle, scanoutA.fbID, scanoutA.pitch, scanoutA.data

	// Best-effort second scanout for page-flip/vblank synced presentation.
	if scanoutB, err := b.createScanout(); err == nil {
		b.handleB, b.fbIDB, b.pitchB, b.dataB = scanoutB.handle, scanoutB.fbID, scanoutB.pitch, scanoutB.data
	}

	// Set CRTC: activate display
	var setCrtc drmModeCrtc
	setCrtc.CrtcID = b.crtcID
	setCrtc.FbID = b.fbID
	setCrtc.SetConnectorsPtr = uint64(uintptr(unsafe.Pointer(&b.connID)))
	setCrtc.CountConnectors = 1
	setCrtc.ModeValid = 1
	setCrtc.Mode = b.mode
	if err := b.ioctl(ioctlSetCrtc, unsafe.Pointer(&setCrtc)); err != nil {
		b.unmapScanouts()
		b.removeFb()
		b.destroyDumb()
		b.file.Close()
		return fmt.Errorf("drm: setcrtc: %w", err)
	}
	_ = encID

	// Switch TTY to graphics mode
	b.setGraphicsMode()

	// Create software back buffer
	b.backBuffer = graphics.NewBuffer(b.width, b.height)

	return nil
}

func (b *Backend) pickCRTC(conn drmModeGetConnector, connectorEncoders, crtcIDs []uint32) (uint32, uint32, bool) {
	candidates := make([]uint32, 0, 1+len(connectorEncoders))
	if conn.EncoderID != 0 {
		candidates = append(candidates, conn.EncoderID)
	}
	for _, eid := range connectorEncoders {
		if eid == 0 {
			continue
		}
		dup := false
		for _, existing := range candidates {
			if existing == eid {
				dup = true
				break
			}
		}
		if !dup {
			candidates = append(candidates, eid)
		}
	}

	for _, eid := range candidates {
		var enc drmModeGetEncoder
		enc.EncoderID = eid
		if err := b.ioctl(ioctlGetEncoder, unsafe.Pointer(&enc)); err != nil {
			continue
		}
		if enc.CrtcID != 0 {
			for _, id := range crtcIDs {
				if id == enc.CrtcID {
					return eid, enc.CrtcID, true
				}
			}
		}
		for i, id := range crtcIDs {
			if enc.PossibleCrtcs&(1<<uint(i)) != 0 {
				return eid, id, true
			}
		}
	}

	return 0, 0, false
}

// Close restores the display and cleans up.
func (b *Backend) Close() error {
	b.restoreTextMode()

	// Restore saved CRTC
	if b.savedCrtc.ModeValid != 0 && b.file != nil {
		b.savedCrtc.SetConnectorsPtr = uint64(uintptr(unsafe.Pointer(&b.connID)))
		b.savedCrtc.CountConnectors = 1
		b.ioctl(ioctlSetCrtc, unsafe.Pointer(&b.savedCrtc))
	}

	if b.data != nil || b.dataB != nil {
		b.unmapScanouts()
	}

	b.removeFb()
	b.destroyDumb()

	if b.file != nil {
		err := b.file.Close()
		b.file = nil
		return err
	}
	return nil
}

// Size returns the display dimensions.
func (b *Backend) Size() (int, int) {
	return b.width, b.height
}

// Buffer returns the back buffer for drawing.
func (b *Backend) Buffer() *graphics.Buffer {
	return b.backBuffer
}

// Flush copies the entire back buffer to the display.
func (b *Backend) Flush() error {
	return b.FlushRects([]graphics.Rect{{W: b.width, H: b.height}})
}

// FlushRect copies a region from the back buffer to the display.
func (b *Backend) FlushRect(r graphics.Rect) error {
	return b.FlushRects([]graphics.Rect{r})
}

// FlushRects copies multiple regions from the back buffer to the display.
// This reduces per-rect overhead and allows one hardware dirty notification.
func (b *Backend) FlushRects(rects []graphics.Rect) error {
	if b.backBuffer == nil {
		return nil
	}
	if len(rects) == 0 {
		return nil
	}
	srcStride := b.backBuffer.Stride

	// Preferred: render into back scanout and page-flip on vblank.
	if b.fbIDB != 0 && len(b.dataB) > 0 && b.pageFlip >= 0 {
		updateRects := rects
		if !b.scanoutsPrimed {
			updateRects = []graphics.Rect{{X: 0, Y: 0, W: b.width, H: b.height}}
		}
		targetData, targetPitch := b.backScanout()
		if len(targetData) != 0 && targetPitch > 0 {
			_, haveMerged := b.copyRectsToScanout(targetData, int(targetPitch), srcStride, updateRects)
			if haveMerged {
				// Keep front and back scanouts coherent for partial damage updates.
				// Without this, alternating flips would expose stale/blank regions.
				frontData, frontPitch, _ := b.frontScanout()
				if len(frontData) != 0 && frontPitch > 0 {
					_, _ = b.copyRectsToScanout(frontData, int(frontPitch), srcStride, updateRects)
				}
				if err := b.flipToBackScanout(); err == nil {
					b.scanoutsPrimed = true
					return nil
				}
				b.pageFlip = -1
				b.scanoutsPrimed = false
			}
		}
	}

	// Fallback: write directly to front scanout and notify dirty.
	frontData, frontPitch, frontFBID := b.frontScanout()
	if len(frontData) == 0 || frontPitch == 0 {
		return nil
	}
	merged, haveMerged := b.copyRectsToScanout(frontData, int(frontPitch), srcStride, rects)
	if haveMerged {
		b.notifyDirty(frontFBID, merged.X, merged.Y, merged.X+merged.W, merged.Y+merged.H)
	}
	return nil
}

// Info returns backend information.
func (b *Backend) Info() string {
	name := ""
	for i, c := range b.mode.Name {
		if c == 0 {
			name = string(b.mode.Name[:i])
			break
		}
	}
	return fmt.Sprintf("DRM/KMS: %dx%d@%dHz (%s), XRGB8888, pitch %d, %s",
		b.width, b.height, b.mode.Vrefresh, name, b.pitch, b.devPath)
}

// HasSystemCursor returns false.
func (b *Backend) HasSystemCursor() bool {
	return false
}

// --- helpers ---

func (b *Backend) ioctl(request uintptr, arg unsafe.Pointer) error {
	for {
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, b.file.Fd(), request, uintptr(arg))
		if errno == 0 {
			return nil
		}
		if errno == syscall.EINTR || errno == syscall.EAGAIN {
			continue
		}
		return errno
	}
}

func (b *Backend) destroyDumb() {
	if b.file == nil {
		return
	}
	for _, h := range []*uint32{&b.handleB, &b.handle} {
		if *h == 0 {
			continue
		}
		d := drmModeDestroyDumb{Handle: *h}
		_ = b.ioctl(ioctlDestroyDumb, unsafe.Pointer(&d))
		*h = 0
	}
}

func (b *Backend) removeFb() {
	if b.file == nil {
		return
	}
	for _, idPtr := range []*uint32{&b.fbIDB, &b.fbID} {
		if *idPtr == 0 {
			continue
		}
		id := *idPtr
		_ = b.ioctl(ioctlRmFb, unsafe.Pointer(&id))
		*idPtr = 0
	}
}

func (b *Backend) notifyDirty(fbID uint32, x0, y0, x1, y1 int) {
	if b.file == nil || fbID == 0 || b.dirtyState < 0 {
		return
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > b.width {
		x1 = b.width
	}
	if y1 > b.height {
		y1 = b.height
	}
	if x0 >= x1 || y0 >= y1 || x1 > 0xFFFF || y1 > 0xFFFF {
		return
	}
	clip := drmModeClipRect{
		X1: uint16(x0),
		Y1: uint16(y0),
		X2: uint16(x1),
		Y2: uint16(y1),
	}
	dirty := drmModeFbDirtyCmd{
		FbID:     fbID,
		NumClips: 1,
		ClipsPtr: uint64(uintptr(unsafe.Pointer(&clip))),
	}
	if err := b.ioctl(ioctlDirtyFb, unsafe.Pointer(&dirty)); err != nil {
		if err == syscall.EINVAL || err == syscall.ENOSYS || err == syscall.EOPNOTSUPP {
			b.dirtyState = -1
		}
		return
	}
	b.dirtyState = 1
}

func (b *Backend) createScanout() (*scanoutBuffer, error) {
	out := &scanoutBuffer{}

	var create drmModeCreateDumb
	create.Width = uint32(b.width)
	create.Height = uint32(b.height)
	create.Bpp = 32
	if err := b.ioctl(ioctlCreateDumb, unsafe.Pointer(&create)); err != nil {
		return nil, err
	}
	out.handle = create.Handle
	out.pitch = create.Pitch

	var fb drmModeFbCmd
	fb.Width = uint32(b.width)
	fb.Height = uint32(b.height)
	fb.Pitch = out.pitch
	fb.Bpp = 32
	fb.Depth = 24
	fb.Handle = out.handle
	if err := b.ioctl(ioctlAddFb, unsafe.Pointer(&fb)); err != nil {
		d := drmModeDestroyDumb{Handle: out.handle}
		_ = b.ioctl(ioctlDestroyDumb, unsafe.Pointer(&d))
		return nil, err
	}
	out.fbID = fb.FbID

	var mapReq drmModeMapDumb
	mapReq.Handle = out.handle
	if err := b.ioctl(ioctlMapDumb, unsafe.Pointer(&mapReq)); err != nil {
		id := out.fbID
		_ = b.ioctl(ioctlRmFb, unsafe.Pointer(&id))
		d := drmModeDestroyDumb{Handle: out.handle}
		_ = b.ioctl(ioctlDestroyDumb, unsafe.Pointer(&d))
		return nil, err
	}

	data, err := syscall.Mmap(
		int(b.file.Fd()),
		int64(mapReq.Offset),
		int(create.Size),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		id := out.fbID
		_ = b.ioctl(ioctlRmFb, unsafe.Pointer(&id))
		d := drmModeDestroyDumb{Handle: out.handle}
		_ = b.ioctl(ioctlDestroyDumb, unsafe.Pointer(&d))
		return nil, err
	}
	out.data = data
	return out, nil
}

type scanoutBuffer struct {
	handle uint32
	fbID   uint32
	pitch  uint32
	data   []byte
}

func (b *Backend) unmapScanouts() {
	if b.dataB != nil {
		_ = syscall.Munmap(b.dataB)
		b.dataB = nil
	}
	if b.data != nil {
		_ = syscall.Munmap(b.data)
		b.data = nil
	}
}

func (b *Backend) frontScanout() ([]byte, uint32, uint32) {
	if b.frontIsB && b.fbIDB != 0 && len(b.dataB) > 0 {
		return b.dataB, b.pitchB, b.fbIDB
	}
	return b.data, b.pitch, b.fbID
}

func (b *Backend) backScanout() ([]byte, uint32) {
	if b.frontIsB {
		return b.data, b.pitch
	}
	return b.dataB, b.pitchB
}

func (b *Backend) backScanoutFBID() uint32 {
	if b.frontIsB {
		return b.fbID
	}
	return b.fbIDB
}

func (b *Backend) copyRectsToScanout(dst []byte, dstStride int, srcStride int, rects []graphics.Rect) (graphics.Rect, bool) {
	var merged graphics.Rect
	haveMerged := false
	for _, r := range rects {
		x0, y0 := r.X, r.Y
		x1, y1 := r.X+r.W, r.Y+r.H
		if x0 < 0 {
			x0 = 0
		}
		if y0 < 0 {
			y0 = 0
		}
		if x1 > b.width {
			x1 = b.width
		}
		if y1 > b.height {
			y1 = b.height
		}
		if x0 >= x1 || y0 >= y1 {
			continue
		}
		rowBytes := (x1 - x0) * 4
		for y := y0; y < y1; y++ {
			srcOff := y*srcStride + x0*4
			dstOff := y*dstStride + x0*4
			simd.CopyBGRA(
				dst[dstOff:dstOff+rowBytes],
				b.backBuffer.Data[srcOff:srcOff+rowBytes],
				x1-x0,
			)
		}
		cr := graphics.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
		if !haveMerged {
			merged = cr
			haveMerged = true
		} else {
			merged = merged.Union(cr)
		}
	}
	return merged, haveMerged
}

func (b *Backend) flipToBackScanout() error {
	fbID := b.backScanoutFBID()
	if fbID == 0 {
		return fmt.Errorf("invalid back scanout")
	}
	req := drmModeCrtcPageFlip{
		CrtcID: b.crtcID,
		FbID:   fbID,
		Flags:  drmModePageFlipEvent,
	}
	if err := b.ioctl(ioctlPageFlip, unsafe.Pointer(&req)); err != nil {
		return err
	}
	if err := b.waitForFlipEvent(200 * time.Millisecond); err != nil {
		return err
	}
	b.frontIsB = !b.frontIsB
	b.pageFlip = 1
	return nil
}

func (b *Backend) waitForFlipEvent(timeout time.Duration) error {
	if b.file == nil {
		return fmt.Errorf("drm device not open")
	}
	fd := int(b.file.Fd())
	deadline := time.Now().Add(timeout)
	var eventBuf [256]byte

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("page flip timeout")
		}
		var readfds syscall.FdSet
		fdSet(fd, &readfds)
		tv := syscall.NsecToTimeval(remaining.Nanoseconds())
		n, err := syscall.Select(fd+1, &readfds, nil, nil, &tv)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("page flip timeout")
		}

		nread, err := syscall.Read(fd, eventBuf[:])
		if err == syscall.EINTR || err == syscall.EAGAIN {
			continue
		}
		if err != nil {
			return err
		}
		if nread < 8 {
			continue
		}
		for off := 0; off+8 <= nread; {
			typ := binary.LittleEndian.Uint32(eventBuf[off : off+4])
			l := int(binary.LittleEndian.Uint32(eventBuf[off+4 : off+8]))
			if l < 8 || off+l > nread {
				break
			}
			if typ == drmEventFlipComplete || typ == drmEventVBlank {
				return nil
			}
			off += l
		}
	}
}

func fdSet(fd int, set *syscall.FdSet) {
	idx := fd / 64
	bit := uint(fd % 64)
	set.Bits[idx] |= int64(1) << bit
}

func (b *Backend) setGraphicsMode() {
	// First try the already-attached controlling TTY via stdin (fd 0).
	stdinPath := fs.Resolve("process:self/fd/0")
	activateVTBestEffort(stdinPath, 0)
	mode := uintptr(kdGraphics)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(0), uintptr(kdSetMode), mode)
	if errno == 0 {
		b.ttyFD = 0
		b.ttyFDSet = true
		return
	}

	// Fall back to opening tty devices directly.
	for _, path := range []string{
		fs.Resolve("device:tty"),
		fs.Resolve("device:tty0"),
	} {
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			continue
		}
		activateVTBestEffort(path, f.Fd())
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(kdSetMode), mode)
		if errno == 0 {
			b.ttyFile = f
			return
		}
		f.Close()
	}
}

func (b *Backend) restoreTextMode() {
	if b.ttyFDSet {
		mode := uintptr(kdText)
		syscall.Syscall(syscall.SYS_IOCTL, b.ttyFD, uintptr(kdSetMode), mode)
		b.ttyFDSet = false
	}
	if b.ttyFile == nil {
		return
	}
	mode := uintptr(kdText)
	syscall.Syscall(syscall.SYS_IOCTL, b.ttyFile.Fd(), uintptr(kdSetMode), mode)
	b.ttyFile.Close()
	b.ttyFile = nil
}

func activateVTBestEffort(path string, fd uintptr) {
	real := path
	if strings.Contains(path, "/fd/") {
		if resolved, err := os.Readlink(path); err == nil && resolved != "" {
			real = resolved
		}
	}
	base := filepath.Base(real)
	if !strings.HasPrefix(base, "tty") {
		return
	}
	n, err := strconv.Atoi(strings.TrimPrefix(base, "tty"))
	if err != nil || n <= 0 {
		return
	}
	arg := uintptr(n)
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(vtActivate), arg)
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(vtWaitAct), arg)
}
