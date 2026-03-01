package main

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	gapp "avyos.dev/pkg/graphics/app"
	gfxfont "avyos.dev/pkg/graphics/fonts"
	gfxinput "avyos.dev/pkg/graphics/input"
	core "avyos.dev/pkg/graphics/pixmap"
	gfxtheme "avyos.dev/pkg/graphics/themes"
	ui "avyos.dev/pkg/graphics/widget/engine"
	"avyos.dev/pkg/pty"
)

//go:embed ui/terminal.ui
var terminalUI string

const (
	defaultRows = 24
	defaultCols = 80
	minRows     = 6
	minCols     = 20
)

type TerminalApp struct {
	gapp.App

	mu       sync.Mutex
	term     *pty.Terminal
	rows     int
	cols     int
	status   string
	stopCh   chan struct{}
	stopOnce sync.Once

	fontKey      string
	font         *gfxfont.Font
	lastFrameSig uint64
	hasFrame     bool
}

func (a *TerminalApp) e(id string) *ui.Element { return a.FindElement(id) }

func (a *TerminalApp) sessionSize() (rows, cols int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	rows = a.rows
	cols = a.cols
	if rows < minRows {
		rows = defaultRows
	}
	if cols < minCols {
		cols = defaultCols
	}
	return rows, cols
}

func (a *TerminalApp) setSessionSize(rows, cols int) {
	if rows < minRows {
		rows = minRows
	}
	if cols < minCols {
		cols = minCols
	}
	a.mu.Lock()
	a.rows = rows
	a.cols = cols
	a.mu.Unlock()
}

func (a *TerminalApp) currentTerminal() *pty.Terminal {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.term
}

func (a *TerminalApp) swapTerminal(next *pty.Terminal, rows, cols int) (prev *pty.Terminal) {
	a.mu.Lock()
	prev = a.term
	a.term = next
	a.rows = rows
	a.cols = cols
	a.lastFrameSig = 0
	a.hasFrame = false
	a.mu.Unlock()
	return prev
}

func (a *TerminalApp) setStatus(text string) {
	if strings.TrimSpace(text) == "" {
		text = " "
	}
	if a.status == text {
		return
	}
	a.status = text
	if status := a.e("Status"); status != nil {
		status.SetAttribute("text", text)
	}
	a.Redraw()
}

func (a *TerminalApp) startSession(rows, cols int) error {
	if rows < minRows {
		rows = defaultRows
	}
	if cols < minCols {
		cols = defaultCols
	}

	t, err := pty.NewTerminal(rows, cols)
	if err != nil {
		return err
	}
	prev := a.swapTerminal(t, rows, cols)
	if prev != nil {
		_ = prev.Close()
	}
	a.setStatus("shell running")
	return nil
}

func (a *TerminalApp) restartSession() {
	rows, cols := a.sessionSize()
	if err := a.startSession(rows, cols); err != nil {
		a.setStatus(fmt.Sprintf("failed to start shell: %v", err))
	}
}

func (a *TerminalApp) shutdown() {
	a.stopOnce.Do(func() {
		close(a.stopCh)
	})
	prev := a.swapTerminal(nil, a.rows, a.cols)
	if prev != nil {
		_ = prev.Close()
	}
}

func (a *TerminalApp) writeToShell(data []byte) error {
	term := a.currentTerminal()
	if term == nil || !term.IsRunning() {
		a.restartSession()
		term = a.currentTerminal()
		if term == nil {
			return fmt.Errorf("shell unavailable")
		}
	}
	return term.Write(data)
}

func (a *TerminalApp) SubmitCommand(text string) {
	if input := a.e("CommandInput"); input != nil {
		input.SetAttribute("text", "")
	}
	if err := a.writeToShell([]byte(text + "\r")); err != nil {
		a.setStatus(fmt.Sprintf("write failed: %v", err))
	}
}

func (a *TerminalApp) SubmitCommandClick() {
	input := a.e("CommandInput")
	if input == nil {
		return
	}
	a.SubmitCommand(input.Attr("text", ""))
}

func (a *TerminalApp) SendCtrlC() {
	if err := a.writeToShell([]byte{3}); err != nil {
		a.setStatus(fmt.Sprintf("ctrl-c failed: %v", err))
	}
}

func (a *TerminalApp) SendCtrlD() {
	if err := a.writeToShell([]byte{4}); err != nil {
		a.setStatus(fmt.Sprintf("ctrl-d failed: %v", err))
	}
}

func (a *TerminalApp) ClearTerminal() {
	if err := a.writeToShell([]byte("\x1b[2J\x1b[H")); err != nil {
		a.setStatus(fmt.Sprintf("clear failed: %v", err))
	}
}

func (a *TerminalApp) RestartSession() {
	a.restartSession()
}

func (a *TerminalApp) HandleEscape() {
	if err := a.writeToShell([]byte{0x1b}); err != nil {
		a.setStatus(fmt.Sprintf("escape failed: %v", err))
	}
}

func encodeTTYKey(ev gfxinput.Event) ([]byte, bool) {
	switch ev.Key {
	case gfxinput.KeyEnter:
		return []byte{'\r'}, true
	case gfxinput.KeyTab:
		if ev.IsShift() {
			return []byte("\x1b[Z"), true
		}
		return []byte{'\t'}, true
	case gfxinput.KeyBackspace:
		return []byte{0x7f}, true
	case gfxinput.KeyEscape:
		return []byte{0x1b}, true
	case gfxinput.KeyUp:
		return []byte("\x1b[A"), true
	case gfxinput.KeyDown:
		return []byte("\x1b[B"), true
	case gfxinput.KeyRight:
		return []byte("\x1b[C"), true
	case gfxinput.KeyLeft:
		return []byte("\x1b[D"), true
	case gfxinput.KeyHome:
		return []byte("\x1b[H"), true
	case gfxinput.KeyEnd:
		return []byte("\x1b[F"), true
	case gfxinput.KeyPageUp:
		return []byte("\x1b[5~"), true
	case gfxinput.KeyPageDown:
		return []byte("\x1b[6~"), true
	case gfxinput.KeyInsert:
		return []byte("\x1b[2~"), true
	case gfxinput.KeyDelete:
		return []byte("\x1b[3~"), true
	case gfxinput.KeyF1:
		return []byte("\x1bOP"), true
	case gfxinput.KeyF2:
		return []byte("\x1bOQ"), true
	case gfxinput.KeyF3:
		return []byte("\x1bOR"), true
	case gfxinput.KeyF4:
		return []byte("\x1bOS"), true
	case gfxinput.KeyF5:
		return []byte("\x1b[15~"), true
	case gfxinput.KeyF6:
		return []byte("\x1b[17~"), true
	case gfxinput.KeyF7:
		return []byte("\x1b[18~"), true
	case gfxinput.KeyF8:
		return []byte("\x1b[19~"), true
	case gfxinput.KeyF9:
		return []byte("\x1b[20~"), true
	case gfxinput.KeyF10:
		return []byte("\x1b[21~"), true
	case gfxinput.KeyF11:
		return []byte("\x1b[23~"), true
	case gfxinput.KeyF12:
		return []byte("\x1b[24~"), true
	}

	if ev.IsCtrl() {
		r := unicode.ToLower(ev.Rune)
		switch {
		case r >= 'a' && r <= 'z':
			return []byte{byte(r-'a') + 1}, true
		case r == ' ' || r == '@':
			return []byte{0x00}, true
		case r == '[':
			return []byte{0x1b}, true
		case r == '\\':
			return []byte{0x1c}, true
		case r == ']':
			return []byte{0x1d}, true
		case r == '^':
			return []byte{0x1e}, true
		case r == '_':
			return []byte{0x1f}, true
		case r == '?':
			return []byte{0x7f}, true
		}
	}

	if ev.Rune != 0 && ev.Rune != '\n' && ev.Rune != '\r' {
		out := []byte(string(ev.Rune))
		if ev.IsAlt() {
			out = append([]byte{0x1b}, out...)
		}
		return out, true
	}

	return nil, false
}

func (a *TerminalApp) TerminalKey(ev gfxinput.Event) {
	if ev.Type != gfxinput.EventKeyPress {
		return
	}
	data, ok := encodeTTYKey(ev)
	if !ok || len(data) == 0 {
		return
	}
	if err := a.writeToShell(data); err != nil {
		a.setStatus(fmt.Sprintf("input failed: %v", err))
	}
}

func terminalCellSize(font *gfxfont.Font) (charW, charH int) {
	if font == nil {
		font = gfxfont.UIFont(gfxfont.UIFontParagraph)
	}
	charW = 8
	charH = 16
	if font != nil {
		if w := font.TextWidth("W"); w > 0 {
			charW = w
		}
		if h := font.Height; h > 0 {
			charH = h
		} else if h := font.TextHeight("W"); h > 0 {
			charH = h
		}
	}
	if charW < 1 {
		charW = 8
	}
	if charH < 1 {
		charH = 16
	}
	return charW, charH
}

func estimateGrid(bounds image.Rectangle, font *gfxfont.Font) (rows, cols int) {
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return defaultRows, defaultCols
	}

	charW, charH := terminalCellSize(font)

	cols = bounds.Dx() / charW
	rows = bounds.Dy() / charH
	if cols < minCols {
		cols = minCols
	}
	if rows < minRows {
		rows = minRows
	}
	return rows, cols
}

func normalizeFontFamilyName(family string) string {
	family = strings.TrimSpace(strings.ToLower(family))
	if family == "" {
		return ""
	}
	repl := strings.NewReplacer(" ", "-", "_", "-", "\\", "-", "/", "-")
	return repl.Replace(family)
}

func resolveElementFontPath(path, family string) string {
	if path = strings.TrimSpace(path); path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}

	family = normalizeFontFamilyName(family)
	if family == "" {
		return ""
	}

	candidates := []string{
		filepath.Join("/data/fonts", family, family+".ttf"),
		filepath.Join("/avyos/data/fonts", family, family+".ttf"),
		filepath.Join("data/fonts", family, family+".ttf"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func (a *TerminalApp) outputFont(output *ui.Element) *gfxfont.Font {
	if output == nil {
		return gfxfont.UIFont(gfxfont.UIFontParagraph)
	}

	path := output.Attr("fontPath", "")
	family := output.Attr("fontFamily", "")
	size := output.AttrFloat("fontSize", 0)
	if size <= 0 {
		size = 14
	}
	key := fmt.Sprintf("%s|%s|%.2f", path, family, size)

	a.mu.Lock()
	if a.fontKey == key && a.font != nil {
		font := a.font
		a.mu.Unlock()
		return font
	}
	a.mu.Unlock()

	resolved := resolveElementFontPath(path, family)
	if resolved == "" {
		return gfxfont.UIFont(gfxfont.UIFontParagraph)
	}
	font, err := gfxfont.LoadTTFFontFile(resolved, &gfxfont.Options{Size: size})
	if err != nil || font == nil {
		return gfxfont.UIFont(gfxfont.UIFontParagraph)
	}

	a.mu.Lock()
	a.fontKey = key
	a.font = font
	a.mu.Unlock()
	return font
}

func clamp01f(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func mixColor(a, b color.NRGBA, t float64) color.NRGBA {
	t = clamp01f(t)
	inv := 1.0 - t
	return core.NewColor(
		uint8(float64(a.R)*inv+float64(b.R)*t+0.5),
		uint8(float64(a.G)*inv+float64(b.G)*t+0.5),
		uint8(float64(a.B)*inv+float64(b.B)*t+0.5),
		uint8(float64(a.A)*inv+float64(b.A)*t+0.5),
	)
}

func themeANSIPalette(defaultFg, defaultBg color.NRGBA) [16]color.NRGBA {
	t := gfxtheme.DefaultTheme
	black := mixColor(defaultBg, core.ColorBlack, 0.58)
	white := mixColor(defaultFg, core.ColorWhite, 0.18)
	red := t.Danger
	green := t.Success
	yellow := t.Warning
	blue := t.Primary
	magenta := mixColor(t.Primary, t.Danger, 0.48)
	cyan := mixColor(t.Primary, t.Success, 0.52)

	return [16]color.NRGBA{
		black,
		red,
		green,
		yellow,
		blue,
		magenta,
		cyan,
		white,
		mixColor(black, white, 0.42),
		mixColor(red, white, 0.24),
		mixColor(green, white, 0.22),
		mixColor(yellow, white, 0.20),
		mixColor(blue, white, 0.24),
		mixColor(magenta, white, 0.22),
		mixColor(cyan, white, 0.20),
		mixColor(white, core.ColorWhite, 0.28),
	}
}

func colorFromXterm256(idx int, palette [16]color.NRGBA) color.NRGBA {
	if idx < 0 {
		return core.ColorTransparent
	}
	if idx < 16 {
		return palette[idx]
	}
	if idx >= 16 && idx <= 231 {
		n := idx - 16
		r := n / 36
		g := (n % 36) / 6
		b := n % 6
		levels := [6]uint8{0, 95, 135, 175, 215, 255}
		return core.NewColor(levels[r], levels[g], levels[b], 255)
	}
	if idx >= 232 && idx <= 255 {
		v := uint8(8 + (idx-232)*10)
		return core.NewColor(v, v, v, 255)
	}
	return palette[7]
}

func resolvePTYColor(c pty.Color, def color.NRGBA, palette [16]color.NRGBA) (color.NRGBA, bool) {
	if c == pty.ColorDefault {
		return def, false
	}

	v := int(c)
	if v >= 256 {
		rgb := v - 256
		r := uint8((rgb >> 16) & 0xFF)
		g := uint8((rgb >> 8) & 0xFF)
		b := uint8(rgb & 0xFF)
		return core.NewColor(r, g, b, 255), true
	}
	if v >= 0 && v <= 255 {
		return colorFromXterm256(v, palette), true
	}
	return def, false
}

func convertTerminalCell(cell pty.Cell, defaultFg, defaultBg color.NRGBA, palette [16]color.NRGBA) ui.TerminalCell {
	style := cell.Style

	fgColor, fgExplicit := resolvePTYColor(style.Fg, defaultFg, palette)
	bgColor, bgExplicit := resolvePTYColor(style.Bg, defaultBg, palette)

	if style.Bold && style.Fg >= pty.ColorBlack && style.Fg <= pty.ColorWhite {
		if bright, ok := resolvePTYColor(style.Fg+8, fgColor, palette); ok {
			fgColor = bright
		}
	}
	if style.Dim {
		alpha := int(fgColor.A) * 65 / 100
		if alpha < 40 {
			alpha = 40
		}
		fgColor.A = uint8(alpha)
	}
	if style.Reverse {
		fgColor, bgColor = bgColor, fgColor
		if !bgExplicit {
			bgColor = defaultBg
			bgExplicit = true
		}
		if !fgExplicit {
			fgColor = defaultFg
		}
	}
	if !bgExplicit {
		bgColor = core.ColorTransparent
	}

	ch := cell.Char
	if ch == 0 {
		ch = ' '
	}

	return ui.TerminalCell{
		Char:      ch,
		Fg:        fgColor,
		Bg:        bgColor,
		Underline: style.Underline,
	}
}

func convertTerminalSnapshot(
	lines [][]pty.Cell,
	defaultFg, defaultBg color.NRGBA,
	palette [16]color.NRGBA,
) [][]ui.TerminalCell {
	out := make([][]ui.TerminalCell, len(lines))
	for y := range lines {
		out[y] = make([]ui.TerminalCell, len(lines[y]))
		for x := range lines[y] {
			out[y][x] = convertTerminalCell(lines[y][x], defaultFg, defaultBg, palette)
		}
	}
	return out
}

func terminalFrameSignature(lines [][]pty.Cell, cursorX, cursorY int, showCursor bool) uint64 {
	h := uint64(1469598103934665603)
	add := func(v uint64) {
		h ^= v
		h *= 1099511628211
	}

	for _, line := range lines {
		add(uint64(len(line)))
		for _, cell := range line {
			add(uint64(cell.Char))
			add(uint64(uint32(int32(cell.Style.Fg))))
			add(uint64(uint32(int32(cell.Style.Bg))))
			flags := uint64(0)
			if cell.Style.Bold {
				flags |= 1 << 0
			}
			if cell.Style.Dim {
				flags |= 1 << 1
			}
			if cell.Style.Italic {
				flags |= 1 << 2
			}
			if cell.Style.Underline {
				flags |= 1 << 3
			}
			if cell.Style.Blink {
				flags |= 1 << 4
			}
			if cell.Style.Reverse {
				flags |= 1 << 5
			}
			add(flags)
		}
	}

	add(uint64(cursorX + 1))
	add(uint64(cursorY + 1))
	if showCursor {
		add(0x9e3779b97f4a7c15)
	}
	return h
}

func (a *TerminalApp) syncTerminalSize() {
	output := a.e("Output")
	if output == nil {
		return
	}
	rows, cols := estimateGrid(output.Bounds(), a.outputFont(output))
	curRows, curCols := a.sessionSize()
	if rows == curRows && cols == curCols {
		return
	}
	a.setSessionSize(rows, cols)

	term := a.currentTerminal()
	if term != nil {
		_ = term.Resize(rows, cols)
	}
}

func (a *TerminalApp) refreshTerminalView() {
	term := a.currentTerminal()
	if term == nil {
		return
	}

	lines, cursorX, cursorY := term.SnapshotCells()
	running := term.IsRunning()
	output := a.e("Output")

	defaultFg := gfxtheme.DefaultTheme.Foreground
	defaultBg := gfxtheme.DefaultTheme.Background
	if output != nil {
		defaultFg = output.AttrColor("textColor", defaultFg)
		defaultBg = output.AttrColor("background", defaultBg)
	}
	palette := themeANSIPalette(defaultFg, defaultBg)

	a.mu.Lock()
	hadFrame := a.hasFrame
	prevSig := a.lastFrameSig
	a.mu.Unlock()
	followBottom := true
	if output != nil && hadFrame {
		followBottom = output.TextAreaAtBottom(1)
	}

	sig := terminalFrameSignature(lines, cursorX, cursorY, running)
	changed := !hadFrame || sig != prevSig

	if changed {
		if output != nil {
			output.SetTerminalBuffer(
				convertTerminalSnapshot(lines, defaultFg, defaultBg, palette),
				cursorX,
				cursorY,
				running,
			)
			if followBottom {
				output.ScrollTextAreaToBottom()
			}
		}
		a.mu.Lock()
		a.lastFrameSig = sig
		a.hasFrame = true
		a.mu.Unlock()
		a.RequestFrame()
	}

	if running {
		a.setStatus("shell running")
	} else {
		a.setStatus("shell exited - restart to continue")
	}
}

func (a *TerminalApp) startRefreshLoop() {
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-a.stopCh:
				return
			case <-ticker.C:
				a.syncTerminalSize()
				a.refreshTerminalView()
			}
		}
	}()
}

func main() {
	app := &TerminalApp{
		rows:   defaultRows,
		cols:   defaultCols,
		stopCh: make(chan struct{}),
	}
	app.SetOptions(gapp.Options{Title: "Terminal"})
	if err := app.LoadString(terminalUI, app); err != nil {
		log.Fatalf("Failed to load UI: %v", err)
	}

	app.Configure(func(core *gapp.App) {
		core.OnQuit = app.shutdown
		core.OnEscape = app.HandleEscape
		app.startRefreshLoop()
	})

	if err := app.startSession(defaultRows, defaultCols); err != nil {
		app.setStatus(fmt.Sprintf("failed to start shell: %v", err))
	}

	app.AddFocusable(
		app.e("Output"),
	)
	app.Focus(app.e("Output"))

	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
