package displayapi

import (
	"fmt"
	"log"
	"os"
	"sync"
	"syscall"
	"time"

	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/graphics"
)

const (
	LayerBackground uint32 = 0
	LayerBottom     uint32 = 1
	LayerTop        uint32 = 2
	LayerOverlay    uint32 = 3
)

const (
	AnchorTop              uint32 = 1
	AnchorBottom           uint32 = 2
	AnchorLeft             uint32 = 4
	AnchorRight            uint32 = 8
	AnchorHorizontalCenter uint32 = 16
	AnchorVerticalCenter   uint32 = 32
)

const (
	WindowFlagFocused   uint32 = 1 << 0
	WindowFlagMinimized uint32 = 1 << 1
	WindowFlagMaximized uint32 = 1 << 2
	WindowFlagVisible   uint32 = 1 << 3
)

const (
	WindowActionToggleMinimize uint32 = 0
	WindowActionMinimize       uint32 = 1
	WindowActionRestore        uint32 = 2
	WindowActionToggleMaximize uint32 = 3
	WindowActionFocus          uint32 = 4
	WindowActionClose          uint32 = 5
)

const (
	ShortcutScopeGlobal uint32 = 0
	ShortcutScopeClient uint32 = 1
)

type ClientEventType int

const (
	ClientEventWindowCreated ClientEventType = iota
	ClientEventConfigure
	ClientEventClose
	ClientEventPointerEnter
	ClientEventPointerLeave
	ClientEventPointerMotion
	ClientEventPointerButton
	ClientEventKey
	ClientEventFocus
	ClientEventMoved
	ClientEventWindowList
	ClientEventShortcut
)

type WindowInfo struct {
	ID        uint32
	Title     string
	Focused   bool
	Visible   bool
	Minimized bool
	Maximized bool
}

type Event struct {
	Type       ClientEventType
	WindowID   uint32
	ShortcutID uint32
	Scope      uint32
	X, Y       int
	Button     int
	Key        graphics.Key
	Rune       rune
	Modifiers  graphics.Modifiers
	Char       rune
	Pressed    bool
	Focused    bool
	Width      int
	Height     int
	Windows    []WindowInfo
}

type DisplayClient struct {
	rpc     *Client
	events  chan Event
	windows map[uint32]*ClientWindow
	mu      sync.Mutex
}

type ClientWindow struct {
	ID     uint32
	X, Y   int
	Width  int
	Height int

	client  *DisplayClient
	fd      int
	path    string
	data    []byte
	stride  int
	buf     *graphics.Buffer
	retired []sharedBuffer
}

func Dial() (*DisplayClient, error) {
	return DialDisplay(fs.Resolve("service:" + ServiceName))
}

func DialDisplay(socketPath string) (*DisplayClient, error) {
	rpc, err := NewClient(socketPath)
	if err != nil {
		return nil, err
	}
	cl := &DisplayClient{
		rpc:     rpc,
		events:  make(chan Event, 256),
		windows: make(map[uint32]*ClientWindow),
	}
	cl.hookEvents()
	return cl, nil
}

func (cl *DisplayClient) hookEvents() {
	cl.rpc.OnConfigure(func(_ uint32, ev ConfigureEvent) {
		cl.mu.Lock()
		if win, ok := cl.windows[ev.WindowID]; ok {
			win.Width = ev.Width
			win.Height = ev.Height
		}
		cl.mu.Unlock()
		cl.push(Event{Type: ClientEventConfigure, WindowID: ev.WindowID, Width: ev.Width, Height: ev.Height})
	})
	cl.rpc.OnClose(func(_ uint32, ev CloseEvent) {
		cl.push(Event{Type: ClientEventClose, WindowID: ev.WindowID})
	})
	cl.rpc.OnPointerEnter(func(_ uint32, ev PointerEnterEvent) {
		cl.push(Event{Type: ClientEventPointerEnter, WindowID: ev.WindowID, X: ev.X, Y: ev.Y})
	})
	cl.rpc.OnPointerLeave(func(_ uint32, ev PointerLeaveEvent) {
		cl.push(Event{Type: ClientEventPointerLeave, WindowID: ev.WindowID})
	})
	cl.rpc.OnPointerMotion(func(_ uint32, ev PointerMotionEvent) {
		cl.push(Event{Type: ClientEventPointerMotion, WindowID: ev.WindowID, X: ev.X, Y: ev.Y})
	})
	cl.rpc.OnPointerButton(func(_ uint32, ev PointerButtonEvent) {
		cl.push(Event{Type: ClientEventPointerButton, WindowID: ev.WindowID, Button: ev.Button, Pressed: ev.Pressed})
	})
	cl.rpc.OnKey(func(_ uint32, ev KeyEvent) {
		cl.push(Event{Type: ClientEventKey, WindowID: ev.WindowID, Key: ev.Key, Char: ev.Char, Pressed: ev.Pressed})
	})
	cl.rpc.OnFocus(func(_ uint32, ev FocusEvent) {
		cl.push(Event{Type: ClientEventFocus, WindowID: ev.WindowID, Focused: ev.Focused})
	})
	cl.rpc.OnMoved(func(_ uint32, ev MovedEvent) {
		cl.mu.Lock()
		if win, ok := cl.windows[ev.WindowID]; ok {
			win.X = ev.X
			win.Y = ev.Y
		}
		cl.mu.Unlock()
		cl.push(Event{Type: ClientEventMoved, WindowID: ev.WindowID, X: ev.X, Y: ev.Y})
	})
	cl.rpc.OnWindowList(func(_ uint32, ev WindowListEvent) {
		list := make([]WindowInfo, 0, len(ev.Windows))
		for _, w := range ev.Windows {
			list = append(list, decodeWindowInfo(w))
		}
		cl.push(Event{Type: ClientEventWindowList, Windows: list})
	})
	cl.rpc.OnShortcut(func(_ uint32, ev ShortcutEvent) {
		cl.push(Event{
			Type:       ClientEventShortcut,
			ShortcutID: ev.ShortcutID,
			WindowID:   ev.WindowID,
			Scope:      ev.Scope,
			Key:        ev.Key,
			Rune:       ev.Rune,
			Modifiers:  graphics.Modifiers(ev.Modifiers),
		})
	})
}

func decodeWindowInfo(item WindowListItem) WindowInfo {
	return WindowInfo{
		ID:        item.ID,
		Title:     item.Title,
		Focused:   item.Flags&WindowFlagFocused != 0,
		Visible:   item.Flags&WindowFlagVisible != 0,
		Minimized: item.Flags&WindowFlagMinimized != 0,
		Maximized: item.Flags&WindowFlagMaximized != 0,
	}
}

func (cl *DisplayClient) push(ev Event) {
	select {
	case cl.events <- ev:
	default:
		log.Printf("display client: event queue full, dropped event type %d", ev.Type)
	}
}

type sharedBuffer struct {
	fd     int
	path   string
	data   []byte
	stride int
}

func allocShared(width, height int) (*sharedBuffer, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid size %dx%d", width, height)
	}
	stride := width * 4
	size := stride * height

	dir := fs.Resolve("cache", "runtime/display-shm")
	if err := os.MkdirAll(dir, 0777); err != nil {
		return nil, err
	}
	_ = os.Chmod(dir, 0777)
	f, err := os.CreateTemp(dir, "win-*.shm")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	if err := f.Truncate(int64(size)); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	_ = f.Chmod(0666)
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}

	fd, err := syscall.Open(path, syscall.O_RDWR, 0)
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	data, err := syscall.Mmap(fd, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		_ = syscall.Close(fd)
		_ = os.Remove(path)
		return nil, err
	}
	return &sharedBuffer{fd: fd, path: path, data: data, stride: stride}, nil
}

func (cl *DisplayClient) registerWindow(id uint32, x, y, width, height int, shm *sharedBuffer) *ClientWindow {
	win := &ClientWindow{
		ID:     id,
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
		client: cl,
		fd:     shm.fd,
		path:   shm.path,
		data:   shm.data,
		stride: shm.stride,
		buf: &graphics.Buffer{
			Width:  width,
			Height: height,
			Stride: shm.stride,
			Format: graphics.PixelFormatBGRA,
			Data:   shm.data,
		},
	}
	cl.mu.Lock()
	cl.windows[id] = win
	cl.mu.Unlock()
	return win
}

func (cl *DisplayClient) CreateWindow(width, height int) (*ClientWindow, error) {
	shm, err := allocShared(width, height)
	if err != nil {
		return nil, err
	}
	resp, err := cl.rpc.CreateWindow(CreateWindowRequest{
		ShmKey: shm.path,
		Width:  width,
		Height: height,
		Stride: shm.stride,
	})
	if err != nil {
		cleanupShared(shm.fd, shm.path, shm.data)
		return nil, err
	}
	return cl.registerWindow(resp.WindowID, resp.X, resp.Y, width, height, shm), nil
}

func (cl *DisplayClient) CreateLayer(layer, anchor uint32, exclusive, width, height int, opaqueHint bool) (*ClientWindow, error) {
	if width <= 0 && height <= 0 {
		return nil, fmt.Errorf("layer must have at least one non-zero dimension")
	}
	allocW, allocH := width, height
	if allocW <= 0 {
		allocW = 1
	}
	if allocH <= 0 {
		allocH = 1
	}

	shm, err := allocShared(allocW, allocH)
	if err != nil {
		return nil, err
	}
	resp, err := cl.rpc.CreateLayer(CreateLayerRequest{
		Layer:      layer,
		Anchor:     anchor,
		Exclusive:  exclusive,
		OpaqueHint: opaqueHint,
		ShmKey:     shm.path,
		Width:      width,
		Height:     height,
		Stride:     shm.stride,
	})
	if err != nil {
		cleanupShared(shm.fd, shm.path, shm.data)
		return nil, err
	}
	win := cl.registerWindow(resp.WindowID, resp.X, resp.Y, allocW, allocH, shm)

	select {
	case ev := <-cl.events:
		if ev.Type == ClientEventConfigure && ev.WindowID == win.ID {
			_ = win.Resize(ev.Width, ev.Height)
		} else {
			cl.push(ev)
		}
	case <-time.After(100 * time.Millisecond):
	}

	return win, nil
}

func (cl *DisplayClient) CreatePopup(parent *ClientWindow, x, y, width, height int) (*ClientWindow, error) {
	if parent == nil {
		return nil, fmt.Errorf("popup requires a parent window")
	}
	shm, err := allocShared(width, height)
	if err != nil {
		return nil, err
	}
	resp, err := cl.rpc.CreatePopup(CreatePopupRequest{
		ParentID: parent.ID,
		X:        x,
		Y:        y,
		ShmKey:   shm.path,
		Width:    width,
		Height:   height,
		Stride:   shm.stride,
	})
	if err != nil {
		cleanupShared(shm.fd, shm.path, shm.data)
		return nil, err
	}
	return cl.registerWindow(resp.WindowID, resp.X, resp.Y, width, height, shm), nil
}

func (cl *DisplayClient) ListWindows() ([]WindowInfo, error) {
	resp, err := cl.rpc.ListWindows(Empty{})
	if err != nil {
		return nil, err
	}
	list := make([]WindowInfo, 0, len(resp.Windows))
	for _, w := range resp.Windows {
		list = append(list, decodeWindowInfo(w))
	}
	return list, nil
}

func (cl *DisplayClient) SetWindowState(windowID, action uint32) error {
	_, err := cl.rpc.SetWindowState(SetWindowStateRequest{WindowID: windowID, Action: action})
	return err
}

func (cl *DisplayClient) RegisterShortcut(shortcutID, windowID, scope uint32, key graphics.Key, modifiers graphics.Modifiers) error {
	return cl.RegisterShortcutEx(shortcutID, windowID, scope, key, 0, modifiers)
}

func (cl *DisplayClient) RegisterShortcutEx(shortcutID, windowID, scope uint32, key graphics.Key, ch rune, modifiers graphics.Modifiers) error {
	_, err := cl.rpc.RegisterShortcut(RegisterShortcutRequest{
		ShortcutID: shortcutID,
		WindowID:   windowID,
		Scope:      scope,
		Key:        key,
		Rune:       ch,
		Modifiers:  uint8(modifiers & (graphics.ModShift | graphics.ModCtrl | graphics.ModAlt)),
	})
	return err
}

func (cl *DisplayClient) UnregisterShortcut(shortcutID uint32) error {
	_, err := cl.rpc.UnregisterShortcut(UnregisterShortcutRequest{ShortcutID: shortcutID})
	return err
}

func (cl *DisplayClient) Poll() *Event {
	select {
	case ev := <-cl.events:
		return &ev
	default:
		return nil
	}
}

func (cl *DisplayClient) Wait() Event {
	return <-cl.events
}

func (cl *DisplayClient) Close() error {
	cl.mu.Lock()
	windows := make([]*ClientWindow, 0, len(cl.windows))
	for _, win := range cl.windows {
		windows = append(windows, win)
	}
	cl.mu.Unlock()

	// Best-effort: destroy server-side windows before disconnecting,
	// so compositor state stays consistent when clients exit.
	for _, win := range windows {
		_, _ = cl.rpc.DestroyWindow(DestroyWindowRequest{WindowID: win.ID})
	}

	cl.mu.Lock()
	for _, win := range windows {
		win.cleanup()
	}
	cl.windows = map[uint32]*ClientWindow{}
	cl.mu.Unlock()
	return cl.rpc.Close()
}

func (w *ClientWindow) Buffer() *graphics.Buffer {
	return w.buf
}

func (w *ClientWindow) Damage(r graphics.Rect) error {
	_, err := w.client.rpc.Damage(DamageRequest{WindowID: w.ID, X: r.X, Y: r.Y, Width: r.W, Height: r.H})
	return err
}

func (w *ClientWindow) DamageAll() error {
	return w.Damage(graphics.Rect{W: w.Width, H: w.Height})
}

func (w *ClientWindow) SetTitle(title string) error {
	_, err := w.client.rpc.SetTitle(SetTitleRequest{WindowID: w.ID, Title: title})
	return err
}

func (w *ClientWindow) Resize(width, height int) error {
	shm, err := allocShared(width, height)
	if err != nil {
		return err
	}
	_, err = w.client.rpc.Resize(ResizeRequest{
		WindowID: w.ID,
		ShmKey:   shm.path,
		Width:    width,
		Height:   height,
		Stride:   shm.stride,
	})
	if err != nil {
		cleanupShared(shm.fd, shm.path, shm.data)
		return err
	}

	// Clean up any previously retired buffers — by the time a new resize
	// arrives the earlier buffers are guaranteed unused by the render path.
	for _, old := range w.retired {
		cleanupShared(old.fd, old.path, old.data)
	}
	w.retired = w.retired[:0]

	// Defer cleanup of the current buffer until the next resize or destroy,
	// since rendering may still reference it briefly.
	if w.data != nil || w.fd >= 0 || w.path != "" {
		w.retired = append(w.retired, sharedBuffer{
			fd:     w.fd,
			path:   w.path,
			data:   w.data,
			stride: w.stride,
		})
	}
	w.fd = shm.fd
	w.path = shm.path
	w.data = shm.data
	w.stride = shm.stride
	w.Width = width
	w.Height = height
	w.buf = &graphics.Buffer{
		Width:  width,
		Height: height,
		Stride: shm.stride,
		Format: graphics.PixelFormatBGRA,
		Data:   shm.data,
	}
	return nil
}

func (w *ClientWindow) Destroy() error {
	_, err := w.client.rpc.DestroyWindow(DestroyWindowRequest{WindowID: w.ID})
	w.client.mu.Lock()
	delete(w.client.windows, w.ID)
	w.client.mu.Unlock()
	w.cleanup()
	return err
}

func cleanupShared(fd int, path string, data []byte) {
	if data != nil {
		_ = syscall.Munmap(data)
	}
	if fd >= 0 {
		_ = syscall.Close(fd)
	}
	if path != "" {
		_ = os.Remove(path)
	}
}

func (w *ClientWindow) cleanup() {
	for _, old := range w.retired {
		cleanupShared(old.fd, old.path, old.data)
	}
	w.retired = nil
	cleanupShared(w.fd, w.path, w.data)
	w.fd = -1
	w.path = ""
	w.data = nil
}
