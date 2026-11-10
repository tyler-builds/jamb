package main

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// palette is the 16 ANSI colors.
var palette = [16]ui.Color{
	ui.Hex("#2a2a35"), ui.Hex("#f7768e"), ui.Hex("#9ece6a"), ui.Hex("#e0af68"),
	ui.Hex("#7aa2f7"), ui.Hex("#bb9af7"), ui.Hex("#7dcfff"), ui.Hex("#c0caf5"),
	ui.Hex("#565f89"), ui.Hex("#ff8fa3"), ui.Hex("#b9f27c"), ui.Hex("#ffc777"),
	ui.Hex("#8fb3ff"), ui.Hex("#c7a9ff"), ui.Hex("#a4daff"), ui.Hex("#ffffff"),
}

var sessionCount atomic.Int64

// point is a cell in the whole history: line 0 is the oldest scrollback line,
// and the screen's rows follow the scrollback.
type point struct{ x, line int }

func (p point) before(q point) bool { return p.line < q.line || p.line == q.line && p.x < q.x }

// session is one shell: its PTY, its emulator and the buffer its grid shows.
type session struct {
	key   string
	cmd   *exec.Cmd
	pty   *os.File
	buf   *ui.CellBuffer
	focus *ui.FocusHandle
	title *ui.Signal[string]
	dead  atomic.Bool

	mu         sync.Mutex // guards the fields below and emu; taken before buf's lock
	emu        *vt.Emulator
	scroll     int // lines scrolled back from the bottom
	lastSB     int // scrollback length at the last sync, to keep a scrolled view still
	wheel      float32
	focused    bool
	sel        [2]point // anchor and head
	hasSel     bool
	selecting  bool
	mouseModes map[ansi.Mode]bool // the mouse reporting modes the app turned on
	hoverX     int                // the cell under the pointer, for wheel reports
	hoverY     int
}

// mouseReporting reports whether the app asked for mouse events. Call it with mu held.
func (s *session) mouseReporting() bool { return len(s.mouseModes) > 0 }

var mouseModes = []ansi.Mode{
	ansi.ModeMouseX10, ansi.ModeMouseNormal, ansi.ModeMouseHighlight,
	ansi.ModeMouseButtonEvent, ansi.ModeMouseAnyEvent,
}

func newSession(onExit func(*session)) *session {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		panic(err)
	}
	s := &session{
		key:   fmt.Sprint("tab", sessionCount.Add(1)),
		cmd:   cmd,
		pty:   f,
		buf:   ui.NewCellBuffer(80, 24),
		focus: ui.NewFocusHandle(),
		title: ui.NewSignal(filepath.Base(shell)),
		emu:   vt.NewEmulator(80, 24),

		mouseModes: map[ansi.Mode]bool{},
	}
	s.emu.SetScrollbackSize(10000)
	// The callbacks run inside emu.Write, which holds mu.
	s.emu.SetCallbacks(vt.Callbacks{
		Title: func(t string) {
			if t != "" {
				s.title.Set(t)
			}
		},
		EnableMode: func(m ansi.Mode) {
			if slices.Contains(mouseModes, m) {
				s.mouseModes[m] = true
			}
		},
		DisableMode: func(m ansi.Mode) { delete(s.mouseModes, m) },
	})
	go io.Copy(f, s.emu) // keys and replies the emulator writes go to the shell
	go func() {
		b := make([]byte, 64*1024)
		for {
			n, err := f.Read(b)
			if err != nil {
				if !s.dead.Load() {
					onExit(s)
				}
				return
			}
			s.mu.Lock()
			s.emu.Write(b[:n])
			s.mu.Unlock()
			s.sync()
		}
	}()
	return s
}

func (s *session) kill() {
	if s.dead.Swap(true) {
		return
	}
	s.cmd.Process.Kill()
	s.pty.Close()
	s.emu.Close()
	go s.cmd.Wait()
}

// sync copies what the view shows, the screen or the scrollback above it, into
// the buffer. The buffer skips cells that did not change, and the grid repaints.
func (s *session) sync() {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.emu
	cols, rows := e.Width(), e.Height()
	sb := 0
	if !e.IsAltScreen() {
		sb = e.ScrollbackLen()
	}
	if s.scroll > 0 {
		s.scroll += sb - s.lastSB // new output: keep the lines in view where they are
	}
	s.lastSB = sb
	s.scroll = max(0, min(s.scroll, sb))
	top := sb - s.scroll

	lo, hi := s.sel[0], s.sel[1]
	if hi.before(lo) {
		lo, hi = hi, lo
	}
	b := s.buf
	b.Lock()
	defer b.Unlock()
	b.Resize(cols, rows)
	for r := range rows {
		line := top + r
		for x := range cols {
			var c *uv.Cell
			if line < sb {
				c = e.ScrollbackCellAt(x, line)
			} else {
				c = e.CellAt(x, line-sb)
			}
			cell := toCell(c)
			if p := (point{x, line}); s.hasSel && !p.before(lo) && !hi.before(p) {
				cell.BG = selectBG
			}
			b.Set(x, r, cell)
		}
	}
	cur := ui.CellCursor{Visible: s.scroll == 0 && !e.CursorHidden(), Color: ui.Hex("#c0caf5")}
	pos := e.CursorPosition()
	cur.X, cur.Y = pos.X, pos.Y
	switch style, _ := e.CursorStyle(); style {
	case vt.CursorBar:
		cur.Shape = ui.CursorBar
	case vt.CursorUnderline:
		cur.Shape = ui.CursorUnderline
	}
	if !s.focused {
		cur.Shape = ui.CursorHollow
	}
	b.SetCursor(cur)
}

func toCell(c *uv.Cell) ui.Cell {
	if c == nil {
		return ui.Cell{}
	}
	st := &c.Style
	out := ui.Cell{
		Text:      c.Content,
		FG:        toColor(st.Fg),
		BG:        toColor(st.Bg),
		Bold:      st.Attrs&uv.AttrBold != 0,
		Italic:    st.Attrs&uv.AttrItalic != 0,
		Underline: st.Underline != uv.UnderlineNone,
	}
	if st.Attrs&uv.AttrReverse != 0 {
		fg, bg := out.FG, out.BG
		if fg.IsZero() {
			fg = fgText
		}
		if bg.IsZero() {
			bg = bgPane
		}
		out.FG, out.BG = bg, fg
	}
	if st.Attrs&uv.AttrFaint != 0 {
		if out.FG.IsZero() {
			out.FG = fgText
		}
		out.FG = out.FG.Alpha(0.6)
	}
	return out
}

func toColor(c color.Color) ui.Color {
	switch v := c.(type) {
	case nil:
		return ui.Color{}
	case ansi.BasicColor:
		if v < 16 {
			return palette[v]
		}
	case ansi.IndexedColor:
		if v < 16 {
			return palette[v]
		}
	}
	r, g, b, _ := c.RGBA()
	return ui.RGB(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// resize fits the emulator and the PTY to the grid.
func (s *session) resize(cols, rows int) {
	s.mu.Lock()
	if cols != s.emu.Width() || rows != s.emu.Height() {
		s.emu.Resize(cols, rows)
		pty.Setsize(s.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	}
	s.mu.Unlock()
	s.sync()
}

// scrollBy moves the view n lines back (n > 0) or forward.
func (s *session) scrollBy(n int) {
	s.mu.Lock()
	s.scroll += n
	s.mu.Unlock()
	s.sync()
}

// pointAt is the history cell under a point in the grid.
func (s *session) pointAt(x, y float32) point {
	col, row := s.buf.CellAt(x, y)
	top := s.lastSB - s.scroll
	return point{min(col, s.emu.Width()-1), top + min(row, s.emu.Height()-1)}
}

// selection returns the selected text, one line per history line, without the
// blanks at the ends of lines.
func (s *session) selection() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasSel {
		return ""
	}
	lo, hi := s.sel[0], s.sel[1]
	if hi.before(lo) {
		lo, hi = hi, lo
	}
	sb := s.lastSB
	var out strings.Builder
	for line := lo.line; line <= hi.line; line++ {
		x0, x1 := 0, s.emu.Width()-1
		if line == lo.line {
			x0 = lo.x
		}
		if line == hi.line {
			x1 = hi.x
		}
		var row strings.Builder
		for x := x0; x <= x1; x++ {
			var c *uv.Cell
			if line < sb {
				c = s.emu.ScrollbackCellAt(x, line)
			} else {
				c = s.emu.CellAt(x, line-sb)
			}
			switch {
			case c == nil || c.Content == "":
				if c == nil || c.Width != 0 {
					row.WriteByte(' ')
				}
			default:
				row.WriteString(c.Content)
			}
		}
		out.WriteString(strings.TrimRight(row.String(), " "))
		if line < hi.line {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// send writes typed input to the shell, back at the bottom of the history.
func (s *session) send(fn func(e *vt.Emulator)) {
	s.mu.Lock()
	s.scroll, s.hasSel = 0, false
	fn(s.emu)
	s.mu.Unlock()
	s.sync()
}

var keyCodes = map[input.Key]rune{
	input.KeyEnter: uv.KeyEnter, input.KeyBackspace: uv.KeyBackspace, input.KeyTab: uv.KeyTab,
	input.KeyEscape: uv.KeyEscape, input.KeyUp: uv.KeyUp, input.KeyDown: uv.KeyDown,
	input.KeyLeft: uv.KeyLeft, input.KeyRight: uv.KeyRight, input.KeyHome: uv.KeyHome,
	input.KeyEnd: uv.KeyEnd, input.KeyPageUp: uv.KeyPgUp, input.KeyPageDown: uv.KeyPgDown,
	input.KeyDelete: uv.KeyDelete, input.KeyInsert: uv.KeyInsert,
	input.KeyF1: uv.KeyF1, input.KeyF2: uv.KeyF2, input.KeyF3: uv.KeyF3, input.KeyF4: uv.KeyF4,
	input.KeyF5: uv.KeyF5, input.KeyF6: uv.KeyF6, input.KeyF7: uv.KeyF7, input.KeyF8: uv.KeyF8,
	input.KeyF9: uv.KeyF9, input.KeyF10: uv.KeyF10, input.KeyF11: uv.KeyF11, input.KeyF12: uv.KeyF12,
}

// view is the session's grid, with its keys, selection and scrolling.
func (s *session) view(ctx *ui.Context, t *tabs) ui.Element {
	onKey := func(e *ui.KeyEvent) {
		ctrl, shift, alt := e.Mods.Has(input.ModCtrl), e.Mods.Has(input.ModShift), e.Mods.Has(input.ModAlt)
		s.mu.Lock()
		rows := s.emu.Height()
		s.mu.Unlock()
		switch {
		case ctrl && shift && e.Key == input.KeyT:
			t.open()
		case ctrl && shift && e.Key == input.KeyW:
			t.close(s)
		case ctrl && shift && e.Key == input.KeyC:
			if text := s.selection(); text != "" {
				ctx.SetClipboard(text)
			}
		case ctrl && shift && e.Key == input.KeyV:
			if text, ok := ctx.Clipboard(); ok {
				s.send(func(e *vt.Emulator) { e.Paste(text) })
			}
		case ctrl && e.Key == input.KeyPageUp:
			t.step(-1)
		case ctrl && e.Key == input.KeyPageDown:
			t.step(1)
		case shift && e.Key == input.KeyPageUp:
			s.scrollBy(rows - 1)
		case shift && e.Key == input.KeyPageDown:
			s.scrollBy(-(rows - 1))
		case e.Key == input.KeySpace && !ctrl && !alt:
			// The space arrives as text input; keep it from clicking.
		default:
			var mod uv.KeyMod
			if ctrl {
				mod |= uv.ModCtrl
			}
			if alt {
				mod |= uv.ModAlt
			}
			if shift {
				mod |= uv.ModShift
			}
			code, ok := keyCodes[e.Key]
			if !ok && (ctrl || alt) && e.Key >= input.KeyA && e.Key <= input.KeyZ {
				code, ok = 'a'+rune(e.Key-input.KeyA), true
			}
			if !ok && ctrl && e.Key == input.KeySpace {
				code, ok = uv.KeySpace, true
			}
			if !ok {
				return
			}
			s.send(func(em *vt.Emulator) { em.SendKey(uv.KeyPressEvent{Code: code, Mod: mod}) })
		}
		e.StopPropagation()
	}

	// Its own key gives each tab's grid its own element, so switching tabs moves
	// focus, and OnFocus runs, rather than keeping the last tab's focused element.
	return ui.CellGrid(s.buf, s.resize).Key(s.key + "-grid").FontSize(14).TextColor(fgText).
		FocusHandle(s.focus).
		OnFocus(func() { s.mu.Lock(); s.focused = true; s.mu.Unlock(); s.sync() }).
		OnBlur(func() { s.mu.Lock(); s.focused = false; s.mu.Unlock(); s.sync() }).
		OnKeyDown(onKey).
		OnTextInput(func(text string) { s.send(func(e *vt.Emulator) { e.SendText(text) }) }).
		Cursor(ui.CursorText).
		OnMouseDown(func(e *ui.MouseEvent) {
			if e.Button != input.ButtonLeft {
				return
			}
			s.focus.Focus()
			s.mu.Lock()
			p := s.pointAt(e.LocalX, e.LocalY)
			s.sel, s.hasSel, s.selecting = [2]point{p, p}, false, true
			s.mu.Unlock()
			s.sync()
		}).
		OnMouseMove(func(e *ui.MouseEvent) {
			s.mu.Lock()
			s.hoverX, s.hoverY = s.buf.CellAt(e.LocalX, e.LocalY)
			if !s.selecting {
				s.mu.Unlock()
				return
			}
			s.sel[1] = s.pointAt(e.LocalX, e.LocalY)
			s.hasSel = s.sel[0] != s.sel[1]
			s.mu.Unlock()
			s.sync()
		}).
		OnMouseUp(func(e *ui.MouseEvent) {
			s.mu.Lock()
			s.selecting = false
			s.mu.Unlock()
		}).
		OnScroll(func(e *ui.ScrollEvent) {
			_, ch := s.buf.CellSize()
			if ch <= 0 {
				return
			}
			s.mu.Lock()
			s.wheel -= e.DeltaY / ch
			n := int(s.wheel)
			s.wheel -= float32(n)
			app := s.emu.IsAltScreen() || s.mouseReporting()
			s.mu.Unlock()
			if n != 0 {
				if app {
					s.wheelToApp(n)
				} else {
					s.scrollBy(n)
				}
			}
			e.StopPropagation()
		})
}

// wheelToApp hands n wheel steps (n > 0 up) to the app on the screen: as mouse
// wheel reports when it asked for the mouse, or else as arrow keys, as other
// terminals do for full-screen apps such as less and man.
func (s *session) wheelToApp(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	up := n > 0
	for range min(abs(n), 10) {
		if s.mouseReporting() {
			b := vt.MouseWheelDown
			if up {
				b = vt.MouseWheelUp
			}
			s.emu.SendMouse(vt.MouseWheel{X: s.hoverX, Y: s.hoverY, Button: b})
		} else {
			code := uv.KeyDown
			if up {
				code = uv.KeyUp
			}
			s.emu.SendKey(uv.KeyPressEvent{Code: code})
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
