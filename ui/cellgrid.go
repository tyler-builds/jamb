package ui

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/text"
)

// Cell is one character cell of a CellBuffer.
type Cell struct {
	// Text is the grapheme the cell shows; "" and " " are blank. A wide
	// grapheme is drawn from its own cell over the next one, which the writer
	// leaves blank.
	Text string
	// FG is the text color; zero means the grid's text color. BG is the cell's
	// background; zero means none, so the element's own background shows.
	FG, BG                  Color
	Bold, Italic, Underline bool
}

// CursorShape is how a CellBuffer's cursor is drawn.
type CursorShape uint8

const (
	CursorBlock CursorShape = iota
	CursorBar
	CursorUnderline
	// CursorHollow is an outlined block, as a terminal draws its cursor when
	// the window does not have focus.
	CursorHollow
)

// CellCursor is the cursor of a CellBuffer.
type CellCursor struct {
	X, Y    int
	Shape   CursorShape
	Visible bool
	// Color is the cursor's color; zero means the grid's text color.
	Color Color
}

// CellBuffer holds the cells a CellGrid shows: a fixed-pitch grid of
// characters, such as a terminal screen. Its writer may run on any goroutine:
// it calls Lock, changes cells with Set, SetCursor and Resize, then Unlock,
// which repaints the grid when something changed. A change repaints only the
// grid; nothing is built again.
type CellBuffer struct {
	mu         sync.Mutex
	cols, rows int
	cells      []Cell
	rowGen     []uint32 // bumped when a row changes, so paint can keep the rest
	cursor     CellCursor
	changed    bool

	rt           atomic.Pointer[Runtime]
	cellW, cellH atomic.Uint32 // float32 bits, logical pixels

	// The paint's cache, touched only on the UI thread with mu held.
	fitCols, fitRows int
	paintKey         gridKey
	paintRows        []gridRow
}

// NewCellBuffer returns a buffer of cols by rows blank cells.
func NewCellBuffer(cols, rows int) *CellBuffer {
	b := &CellBuffer{}
	b.Resize(cols, rows)
	return b
}

// Lock locks the buffer for changes.
func (b *CellBuffer) Lock() { b.mu.Lock() }

// Unlock unlocks the buffer and repaints the grid if its cells or cursor changed.
func (b *CellBuffer) Unlock() {
	changed := b.changed
	b.changed = false
	b.mu.Unlock()
	if rt := b.rt.Load(); changed && rt != nil {
		rt.Post(func() { rt.needPaint = true })
	}
}

// Size returns the buffer's columns and rows. Call it with the buffer locked.
func (b *CellBuffer) Size() (cols, rows int) { return b.cols, b.rows }

// Resize makes the buffer cols by rows, keeping the cells that still fit.
// Call it with the buffer locked, or before the buffer is shared.
func (b *CellBuffer) Resize(cols, rows int) {
	cols, rows = max(cols, 1), max(rows, 1)
	if cols == b.cols && rows == b.rows {
		return
	}
	cells := make([]Cell, cols*rows)
	for y := range min(rows, b.rows) {
		copy(cells[y*cols:y*cols+min(cols, b.cols)], b.cells[y*b.cols:])
	}
	b.cells, b.cols, b.rows = cells, cols, rows
	b.rowGen = make([]uint32, rows)
	b.paintRows = nil
	b.changed = true
}

// Set sets the cell at column x of row y. Cells outside the buffer are ignored.
// Call it with the buffer locked.
func (b *CellBuffer) Set(x, y int, c Cell) {
	if x < 0 || y < 0 || x >= b.cols || y >= b.rows {
		return
	}
	if c.Text == " " {
		c.Text = ""
	}
	i := y*b.cols + x
	if b.cells[i] == c {
		return
	}
	b.cells[i] = c
	b.rowGen[y]++
	b.changed = true
}

// At returns the cell at column x of row y. Call it with the buffer locked.
func (b *CellBuffer) At(x, y int) Cell {
	if x < 0 || y < 0 || x >= b.cols || y >= b.rows {
		return Cell{}
	}
	return b.cells[y*b.cols+x]
}

// SetCursor sets the cursor. Call it with the buffer locked.
func (b *CellBuffer) SetCursor(c CellCursor) {
	if b.cursor != c {
		b.cursor = c
		b.changed = true
	}
}

// CellSize returns the size of one cell as the grid last drew it, in logical
// pixels, or zeros before it was drawn. It does not need the lock.
func (b *CellBuffer) CellSize() (w, h float32) {
	return math.Float32frombits(b.cellW.Load()), math.Float32frombits(b.cellH.Load())
}

// CellAt returns the column and row under x, y, a point in the grid's own
// coordinates (MouseEvent.LocalX and LocalY), clamped to the grid. It does not
// need the lock.
func (b *CellBuffer) CellAt(x, y float32) (col, row int) {
	w, h := b.CellSize()
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	return max(int(x/w), 0), max(int(y/h), 0)
}

type cellGrid struct {
	buf      *CellBuffer
	onResize func(cols, rows int)
}

// CellGrid creates an element that shows the cells of b in the monospace font,
// one cell per character, as a terminal shows its screen. The cell backgrounds
// and glyphs are drawn by the GPU, and a row whose cells have not changed since
// the last frame is drawn from what that frame laid out.
//
// The font size is the element's FontSize (the theme's by default) and the
// default text color its TextColor. The grid fills its parent; onResize, when
// set, is called with how many whole cells fit when that changes, so the owner
// can resize the buffer (and, for a terminal, its PTY). It runs on the UI
// thread while the grid paints, so it must not wait on the goroutine that
// holds the buffer's lock.
func CellGrid(b *CellBuffer, onResize func(cols, rows int)) *DivElement {
	d := Div().Size(Full, Full).Role(access.RoleUnknown)
	d.grid = &cellGrid{buf: b, onResize: onResize}
	return d
}

// gridKey is what the cached rows of a grid were laid out for.
type gridKey struct {
	size, scale float32
	fg          Color
}

type gridRow struct {
	gen  uint32
	ok   bool
	bgs  []gridSpan
	runs []gridRun
	ul   []gridSpan
	// shapes are the cells drawn as shapes rather than glyphs (see cellShape).
	shapes []gridShape
}

type gridShape struct {
	col   int
	r     rune
	color Color
}

type gridSpan struct {
	col, n int
	color  Color
}

type gridRun struct {
	col   int
	l     *text.Layout
	color Color
}

// paintCellGrid draws the cells of grid n inside its content box at x, y.
func (rt *Runtime) paintCellGrid(n *node, x, y, w, h float32, fg Color, opacity float32) {
	g := n.div.grid
	b := g.buf
	b.rt.Store(rt)
	s := &n.style
	be := s.borderEdges()
	box := scene.Rect{X: x + be.Left + s.Padding.Left, Y: y + be.Top + s.Padding.Top}
	box.W = w - be.Left - be.Right - s.Padding.Left - s.Padding.Right
	box.H = h - be.Top - be.Bottom - s.Padding.Top - s.Padding.Bottom
	if box.W <= 0 || box.H <= 0 {
		return
	}
	size := s.FontSize
	if size == 0 {
		size = rt.theme.FontSize
	}
	size *= rt.textFactor
	st := text.Style{Family: text.FamilyMono, Size: size, Weight: text.Regular, NoWrap: true}
	m := rt.text.Layout("M", st, float32(math.Inf(1)))
	cw, ch := m.Width, m.Height
	if cw <= 0 || ch <= 0 {
		return
	}
	b.cellW.Store(math.Float32bits(cw))
	b.cellH.Store(math.Float32bits(ch))

	cols, rows := max(int(box.W/cw), 1), max(int(box.H/ch), 1)
	b.mu.Lock()
	refit := cols != b.fitCols || rows != b.fitRows
	b.fitCols, b.fitRows = cols, rows
	b.mu.Unlock()
	if refit && g.onResize != nil {
		g.onResize(cols, rows)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if key := (gridKey{size, rt.scale, fg}); key != b.paintKey {
		b.paintKey, b.paintRows = key, nil
	}
	if len(b.paintRows) != b.rows {
		b.paintRows = make([]gridRow, b.rows)
	}
	shown := min(rows, b.rows)
	for r := range shown {
		if pr := &b.paintRows[r]; !pr.ok || pr.gen != b.rowGen[r] {
			rt.layoutGridRow(b, r, pr, st, fg)
		}
	}

	sc := &rt.scene
	sc.PushClip(rt.px(box))
	cell := func(col, row, n int) scene.Rect {
		return rt.px(scene.Rect{X: box.X + float32(col)*cw, Y: box.Y + float32(row)*ch, W: float32(n) * cw, H: ch})
	}
	// Backgrounds first, so the glyphs of every row draw over them in one batch,
	// then the cells drawn as shapes, which are quads too.
	for r := range shown {
		for _, bg := range b.paintRows[r].bgs {
			sc.AddQuad(scene.Quad{Bounds: cell(bg.col, r, bg.n), Background: bg.color.Alpha(opacity).scene()})
		}
	}
	for r := range shown {
		for _, sh := range b.paintRows[r].shapes {
			drawCellShape(sc, sh.r, cell(sh.col, r, 1), sh.color.Alpha(opacity))
		}
	}
	for r := range shown {
		py := box.Y + float32(r)*ch
		for _, run := range b.paintRows[r].runs {
			rt.text.Draw(sc, run.l, box.X+float32(run.col)*cw, py, run.l.Width, rt.scale, 0, run.color.Alpha(opacity).scene())
		}
		for _, ul := range b.paintRows[r].ul {
			q := cell(ul.col, r, ul.n)
			t := max(round(rt.scale), 1)
			q.Y, q.H = q.Y+q.H-2*t, t
			sc.AddQuad(scene.Quad{Bounds: q, Background: ul.color.Alpha(opacity).scene()})
		}
	}
	if c := b.cursor; c.Visible && c.X >= 0 && c.Y >= 0 && c.X < b.cols && c.Y < shown {
		col := c.Color
		if col.IsZero() {
			col = fg
		}
		q := cell(c.X, c.Y, 1)
		t := max(round(rt.scale), 1)
		cq := scene.Quad{Bounds: q, Background: col.Alpha(opacity).scene()}
		switch c.Shape {
		case CursorBar:
			cq.Bounds.W = 2 * t
		case CursorUnderline:
			cq.Bounds.Y, cq.Bounds.H = q.Y+q.H-2*t, 2*t
		case CursorHollow:
			cq.Background = scene.Color{}
			cq.BorderWidth, cq.BorderColor = t, col.Alpha(opacity).scene()
		}
		sc.AddQuad(cq)
		if c.Shape == CursorBlock {
			// The character under a block cursor, in the cell's background color.
			if cl := b.cells[c.Y*b.cols+c.X]; cl.Text != "" {
				under := cl.BG
				if under.IsZero() {
					under = n.shownBG()
				}
				if under.IsZero() {
					under = rt.theme.Background
				}
				if sh, ok := cellShape(cl.Text); ok {
					drawCellShape(sc, sh, q, under.Alpha(opacity))
				} else {
					l := rt.text.Layout(cl.Text, cellStyle(st, cl), float32(math.Inf(1)))
					rt.text.Draw(sc, l, box.X+float32(c.X)*cw, box.Y+float32(c.Y)*ch, l.Width, rt.scale, 0, under.Alpha(opacity).scene())
				}
			}
		}
	}
	sc.PopClip()
}

func cellStyle(st text.Style, c Cell) text.Style {
	if c.Bold {
		st.Weight = text.Bold
	}
	st.Italic = c.Italic
	return st
}

// layoutGridRow lays out row r of b into pr: spans of one background, runs of
// text in one style, underlines, and the cells drawn as shapes. An ASCII run is
// one layout; any other grapheme is laid out alone at its cell, so a fallback
// font's advance cannot push the rest of the row off the grid.
func (rt *Runtime) layoutGridRow(b *CellBuffer, r int, pr *gridRow, st text.Style, fg Color) {
	pr.gen, pr.ok = b.rowGen[r], true
	pr.bgs, pr.runs, pr.ul, pr.shapes = pr.bgs[:0], pr.runs[:0], pr.ul[:0], pr.shapes[:0]
	row := b.cells[r*b.cols : (r+1)*b.cols]
	var buf []byte
	runCol := -1
	var runCell Cell
	flush := func() {
		for len(buf) > 0 && buf[len(buf)-1] == ' ' {
			buf = buf[:len(buf)-1]
		}
		if runCol >= 0 && len(buf) > 0 {
			l := rt.text.Layout(string(buf), cellStyle(st, runCell), float32(math.Inf(1)))
			pr.runs = append(pr.runs, gridRun{col: runCol, l: l, color: cellFG(runCell, fg)})
		}
		buf, runCol = buf[:0], -1
	}
	span := func(spans []gridSpan, col int, c Color) []gridSpan {
		if k := len(spans) - 1; k >= 0 && spans[k].col+spans[k].n == col && spans[k].color == c {
			spans[k].n++
			return spans
		}
		return append(spans, gridSpan{col: col, n: 1, color: c})
	}
	for x, c := range row {
		if !c.BG.IsZero() {
			pr.bgs = span(pr.bgs, x, c.BG)
		}
		if c.Underline {
			pr.ul = span(pr.ul, x, cellFG(c, fg))
		}
		if c.Text == "" {
			if runCol >= 0 {
				buf = append(buf, ' ') // a monospace space keeps the run on the grid
			}
			continue
		}
		if r, ok := cellShape(c.Text); ok {
			flush()
			pr.shapes = append(pr.shapes, gridShape{col: x, r: r, color: cellFG(c, fg)})
			continue
		}
		ascii := len(c.Text) == 1 && c.Text[0] < 0x80
		same := runCol >= 0 && runCell.FG == c.FG && runCell.Bold == c.Bold && runCell.Italic == c.Italic
		if !ascii || !same {
			flush()
		}
		if !ascii {
			l := rt.text.Layout(c.Text, cellStyle(st, c), float32(math.Inf(1)))
			pr.runs = append(pr.runs, gridRun{col: x, l: l, color: cellFG(c, fg)})
			continue
		}
		if runCol < 0 {
			runCol, runCell = x, c
		}
		buf = append(buf, c.Text[0])
	}
	flush()
}

func cellFG(c Cell, fg Color) Color {
	if c.FG.IsZero() {
		return fg
	}
	return c.FG
}
