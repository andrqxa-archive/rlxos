package core

import (
	"testing"
	"unsafe"
)

func TestBufferResizeReusesBackingStore(t *testing.T) {
	buf := NewBuffer(8, 8)
	if len(buf.Data) == 0 {
		t.Fatal("expected non-empty backing store")
	}
	ptrBefore := uintptr(unsafe.Pointer(&buf.Data[0]))
	capBefore := cap(buf.Data)

	buf.SetClip(Rect{X: 1, Y: 1, W: 2, H: 2})
	buf.Resize(4, 4)

	if buf.Width != 4 || buf.Height != 4 || buf.Stride != 16 {
		t.Fatalf("unexpected size after resize: %dx%d stride=%d", buf.Width, buf.Height, buf.Stride)
	}
	if cap(buf.Data) != capBefore {
		t.Fatalf("expected capacity reuse, got cap=%d want=%d", cap(buf.Data), capBefore)
	}
	ptrAfter := uintptr(unsafe.Pointer(&buf.Data[0]))
	if ptrAfter != ptrBefore {
		t.Fatal("expected resize to reuse existing backing store")
	}
	if _, clipped := buf.Clip(); clipped {
		t.Fatal("expected clip to be cleared by resize")
	}
}

func TestBufferResizeGrowAllocates(t *testing.T) {
	buf := NewBuffer(2, 2)
	capBefore := cap(buf.Data)

	buf.Resize(64, 64)

	if buf.Width != 64 || buf.Height != 64 || buf.Stride != 256 {
		t.Fatalf("unexpected size after grow: %dx%d stride=%d", buf.Width, buf.Height, buf.Stride)
	}
	if len(buf.Data) != 64*64*4 {
		t.Fatalf("unexpected buffer length: %d", len(buf.Data))
	}
	if cap(buf.Data) <= capBefore {
		t.Fatalf("expected larger capacity after grow, got cap=%d before=%d", cap(buf.Data), capBefore)
	}
}
