package ui

import (
	"math"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/text"
)

// Selectable lets the user select the text and copy it, as in an error message
// or a details pane: drag across it, double-click a word, triple-click a
// paragraph, Shift+click to extend, and Ctrl+A and Ctrl+C (or Ctrl+Insert)
// once it has been clicked. A click puts keyboard focus on the text, but Tab
// does not stop on it, as with selectable text elsewhere in Windows. The
// selection is cleared when focus moves away. In rich text, a click on a link
// still follows the link.
func (t *TextElement) Selectable() *TextElement { t.selectable = true; return t }

// textSelection is the selection of a selectable text (kept in nodeState).
type textSelection struct {
	text          string // the text the selection is of; another text clears it
	anchor, caret int    // byte offsets; the selection is between them
	dragging      bool
	// wordA and wordB are the word a finger's long press selected, while its
	// drag selects by words from it (see wordDrag); equal otherwise.
	wordA, wordB int
	clicks       int
	lastClick    time.Time
	lastX, lastY float32
	layout       *text.Layout // from the last paint, for hit testing
	// Where the last paint drew the selection, in logical window coordinates:
	// the box around it, and its last line. drawnGen is that paint's paintGen.
	drawn, drawnLast scene.Rect
	drawnGen         uint32
}

func (s *textSelection) bounds() (int, int) { return min(s.anchor, s.caret), max(s.anchor, s.caret) }

var (
	textCopy      = NewAction("text.copy", "Copy").Hide()
	textSelectAll = NewAction("text.select-all", "Select all").Hide()
)

func selectableTextBindings() []Binding {
	return []Binding{
		BindIn("SelectableText", "ctrl+c", textCopy),
		BindIn("SelectableText", "ctrl+insert", textCopy),
		BindIn("SelectableText", "ctrl+a", textSelectAll),
	}
}

// buildSelectable gives a selectable text node the handlers a selection needs,
// on a synthetic div, as an editor has (see buildEditor).
func (rt *Runtime) buildSelectable(n *node) {
	n.st = rt.state(n.id)
	if n.st.sel == nil {
		n.st.sel = &textSelection{}
	}
	id := n.id
	d := &DivElement{style: Style{Cursor: CursorText}.Merge(n.style), focusable: true, noTabStop: true, capture: true, selects: true}
	d.KeyContext("SelectableText")
	d.OnAction(textCopy, func() { rt.copySelection(id) })
	d.OnAction(textSelectAll, func() {
		if n := rt.byID[id]; n != nil {
			s := n.st.sel
			s.text, s.anchor, s.caret = n.textEl.text, 0, len(n.textEl.text)
			rt.needPaint = true
			rt.requestFrame()
		}
	})
	d.h.onMouseDown = func(e *MouseEvent) { rt.selectMouseDown(id, e) }
	// Copy and Select all, unless an element around the text has a menu of its own.
	d.contextMenu = func() []MenuItem { return rt.selectableMenu(id) }
	d.menuFallback = true
	d.h.onMouseMove = func(e *MouseEvent) { rt.selectMouseMove(id, e) }
	d.h.onMouseUp = func(*MouseEvent) {
		if n := rt.byID[id]; n != nil {
			n.st.sel.dragging = false
			n.st.sel.wordA, n.st.sel.wordB = 0, 0
		}
	}
	d.h.onBlur = func() {
		if n := rt.byID[id]; n != nil && n.st.sel.anchor != n.st.sel.caret {
			n.st.sel.anchor = n.st.sel.caret
			rt.needPaint = true
			rt.requestFrame()
		}
	}
	n.div = d
}

// selectionOf returns the selection of a selectable text node, cleared when
// the text changed since it was made.
func (rt *Runtime) selectionOf(id uint64) (*node, *textSelection) {
	n := rt.byID[id]
	if n == nil || n.st == nil || n.st.sel == nil || n.textEl == nil {
		return nil, nil
	}
	s := n.st.sel
	if s.text != n.textEl.text {
		s.text, s.anchor, s.caret = n.textEl.text, 0, 0
	}
	return n, s
}

// selectHit returns the text position under the pointer.
func (rt *Runtime) selectHit(n *node, s *textSelection, x, y float32) int {
	l := s.layout
	if l == nil || len(l.Lines) == 0 || l.LineHeight <= 0 {
		return 0
	}
	st := &n.style
	lx, ly := x-n.abs.X-st.Padding.Left, y-n.abs.Y-st.Padding.Top
	li := min(max(int(math.Floor(float64(ly/l.LineHeight))), 0), len(l.Lines)-1)
	return l.HitTest(lx-n.lineOffset(l, li), ly)
}

// lineOffset is how far the text's alignment moves line li of l right.
func (n *node) lineOffset(l *text.Layout, li int) float32 {
	contentW := max(n.abs.W-n.style.Padding.Left-n.style.Padding.Right, 0)
	switch n.ts.align {
	case Center:
		return (contentW - l.Lines[li].Width) / 2
	case End:
		return contentW - l.Lines[li].Width
	}
	return 0
}

// selectableMenu is the context menu of a selectable text: Copy, and Select all.
func (rt *Runtime) selectableMenu(id uint64) []MenuItem {
	n, s := rt.selectionOf(id)
	if n == nil {
		return nil
	}
	a, b := s.bounds()
	return []MenuItem{
		{Action: textCopy, Disabled: a == b},
		{Action: textSelectAll, Disabled: n.textEl.text == ""},
	}
}

// selectContextPress readies a selectable text for the context menu a right
// press opens: the text takes focus, so Copy reaches it, and a press outside
// the selection clears it. A press inside the selection, or on a link, keeps it.
func (rt *Runtime) selectContextPress(id uint64, n *node, s *textSelection, e *MouseEvent) {
	if rt.focusID != id {
		rt.setFocus(id)
	}
	if n.rich != nil && rt.segmentAt([]*node{n}, e.X, e.Y).node != 0 {
		return
	}
	if p, a, b := rt.selectHit(n, s, e.X, e.Y), s.anchor, s.caret; a == b || p < min(a, b) || p > max(a, b) {
		s.anchor, s.caret = p, p
		rt.needPaint = true
		rt.requestFrame()
	}
}

func (rt *Runtime) selectMouseDown(id uint64, e *MouseEvent) {
	n, s := rt.selectionOf(id)
	if n == nil {
		return
	}
	if e.Button == input.ButtonRight {
		rt.selectContextPress(id, n, s, e)
		return
	}
	if e.Button != input.ButtonLeft {
		return
	}
	if n.rich != nil && rt.clickable(rt.segmentAt([]*node{n}, e.X, e.Y)) {
		return // a link: the press follows it
	}
	p := rt.selectHit(n, s, e.X, e.Y)
	v := n.textEl.text
	now := rt.now()
	if now.Sub(s.lastClick) < 500*time.Millisecond && abs32(e.X-s.lastX) < 4 && abs32(e.Y-s.lastY) < 4 {
		s.clicks++
	} else {
		s.clicks = 1
	}
	s.lastClick, s.lastX, s.lastY = now, e.X, e.Y
	s.wordA, s.wordB = 0, 0
	switch {
	case rt.wordPress:
		// A finger's long press selects the word, and its drag more words.
		s.anchor, s.caret = text.WordAt(v, p)
		s.wordA, s.wordB = s.anchor, s.caret
		s.dragging = true
	case s.clicks == 2:
		s.anchor, s.caret = text.WordAt(v, p)
	case s.clicks >= 3:
		start := strings.LastIndexByte(v[:p], '\n') + 1
		end := strings.IndexByte(v[p:], '\n')
		if end < 0 {
			end = len(v)
		} else {
			end += p
		}
		s.anchor, s.caret = start, end
	case e.Mods.Has(input.ModShift):
		s.caret = p
	default:
		s.anchor, s.caret = p, p
		s.dragging = true
	}
	rt.needPaint = true
	rt.requestFrame()
}

func (rt *Runtime) selectMouseMove(id uint64, e *MouseEvent) {
	n, s := rt.selectionOf(id)
	if n == nil || !s.dragging {
		return
	}
	p := rt.selectHit(n, s, e.X, e.Y)
	anchor, caret := s.anchor, p
	if s.wordA != s.wordB {
		anchor, caret = wordDrag(n.textEl.text, s.wordA, s.wordB, p)
	}
	if anchor != s.anchor || caret != s.caret {
		s.anchor, s.caret = anchor, caret
		rt.needPaint = true
		rt.requestFrame()
	}
}

// wordDrag returns the selection of a drag by words to the position p of the
// text v, which began at the word from a to b: that word, and the words from
// it to the one at p.
func wordDrag(v string, a, b, p int) (anchor, caret int) {
	start, end := text.WordAt(v, p)
	switch {
	case p < a:
		return b, start
	case p > b:
		return a, end
	}
	return a, b
}

// copySelection puts the selected text on the clipboard.
func (rt *Runtime) copySelection(id uint64) {
	n, s := rt.selectionOf(id)
	if n == nil {
		return
	}
	if a, b := s.bounds(); a < b {
		rt.setClipboard(n.textEl.text[a:b])
	}
}

// textSelectionRects draws the selection of a selectable text laid out as l
// at x, y (the content box), under the text, in the theme's selection color.
// It returns the selected rectangles, in pixels, when the selected text is
// drawn again over them in another color (under high contrast), else nil.
func (rt *Runtime) textSelectionRects(n *node, l *text.Layout, x, y, opacity float32) []scene.Rect {
	if n.st == nil || n.st.sel == nil {
		return nil
	}
	s := n.st.sel
	s.layout = l
	// The selection shows while the text has the focus, and while its context
	// menu has it, whose Copy is of what shows.
	if rt.focusID != n.id && rt.menuHeld != n.id {
		return nil
	}
	if s.text != n.textEl.text {
		s.text, s.anchor, s.caret = n.textEl.text, 0, 0
	}
	a, b := s.bounds()
	if a == b {
		return nil
	}
	// The field's selection colors: the Highlight color with HighlightText on
	// it under high contrast, as Windows draws it.
	c := rt.theme.Accent.Alpha(0.32)
	if rt.theme.HighContrast {
		c = rt.theme.Accent
	}
	var out []scene.Rect
	rt.selRects = l.SelectionRects(a, b, rt.selRects[:0])
	s.drawn, s.drawnLast, s.drawnGen = scene.Rect{}, scene.Rect{}, rt.paintGen
	for i, r := range rt.selRects {
		li := 0
		if l.LineHeight > 0 {
			li = min(max(int(r.Y/l.LineHeight+0.5), 0), len(l.Lines)-1)
		}
		lr := scene.Rect{X: x + n.lineOffset(l, li) + r.X, Y: y + r.Y, W: r.W, H: r.H}
		if i == 0 {
			s.drawn = lr
		} else {
			s.drawn = union(s.drawn, lr)
		}
		s.drawnLast = lr
		b := rt.px(lr)
		rt.scene.AddQuad(scene.Quad{Bounds: b, Background: c.Alpha(opacity).scene(), Radii: scene.Uniform(2 * rt.scale)})
		if rt.theme.HighContrast {
			out = append(out, b)
		}
	}
	return out
}

// redrawSelected draws the text again with draw, clipped to each selected
// rectangle, in the selection's text color.
func (rt *Runtime) redrawSelected(rects []scene.Rect, draw func(Color)) {
	for _, r := range rects {
		rt.scene.PushClip(r)
		draw(rt.theme.AccentText)
		rt.scene.PopClip()
	}
}

// union is the smallest rectangle around a and b.
func union(a, b scene.Rect) scene.Rect {
	x, y := min(a.X, b.X), min(a.Y, b.Y)
	return scene.Rect{X: x, Y: y, W: max(a.Right(), b.Right()) - x, H: max(a.Bottom(), b.Bottom()) - y}
}

// TextSelection is the text selected in a Selectable text, and where it is
// drawn. The zero value is no selection.
type TextSelection struct {
	Text string // the selected text
	// Bounds is the box around the selected text, and Last the selected part
	// of its last line, in logical pixels relative to the border box of the
	// element whose OnTextSelect reports it, so that an Absolute child of that
	// element can be placed by them. They are where the text is drawn, even
	// when part of it is scrolled out of view.
	Bounds, Last Rect
}

// OnTextSelect calls fn when the text the user has selected in a Selectable
// text inside the element changes or moves (as its scroll container
// scrolls), and with the zero TextSelection when the selection is cleared,
// loses focus or is scrolled entirely out of view. The innermost element with
// an OnTextSelect around the text reports it. fn runs after the frame is
// painted; what it changes is built, laid out and painted before the frame
// ends, so an element placed by the selection follows it as it scrolls.
//
// A selection is within one selectable text, such as one paragraph of a
// markdown.View.
//
//	sel := ui.Local(ctx, "sel", ui.TextSelection{})
//	ui.Div().OnTextSelect(sel.Set).Child(markdown.View(doc)).
//		Child(ui.If(sel.Get().Text != "", quoteButton(sel.Get().Last)))
func (d *DivElement) OnTextSelect(fn func(TextSelection)) *DivElement {
	d.watches().textSelect = fn
	return d
}

// ClearTextSelection clears the selection of the selectable text that has
// focus, if one has. Its OnTextSelect then reports no selection.
func (c *Context) ClearTextSelection() { c.rt.clearTextSelection() }

func (rt *Runtime) clearTextSelection() {
	if n, s := rt.selectionOf(rt.focusID); n != nil && s.anchor != s.caret {
		s.anchor = s.caret
		s.dragging = false
		rt.needPaint = true
		rt.requestFrame()
	}
}

// reportedSelection is the selection last reported to an OnTextSelect, and
// the id of the element it was reported to.
type reportedSelection struct {
	id  uint64
	sel TextSelection
}

// reportTextSelection calls the OnTextSelect around the focused selectable
// text when its selection has changed or moved since the last report, and the
// one last reported to with no selection when the selection has gone or
// another element now reports it. It runs after a paint, which records where
// the selection is drawn. It reports whether it called a function.
func (rt *Runtime) reportTextSelection() bool {
	var sel TextSelection
	var watcher *node
	if n := rt.byID[rt.focusID]; n != nil && n.textEl != nil && n.st != nil && n.st.sel != nil && n.paintGen == rt.paintGen {
		s := n.st.sel
		if a, b := s.bounds(); a < b && s.text == n.textEl.text && s.drawnGen == rt.paintGen && !s.drawn.Intersect(n.clip).Empty() {
			for p := n.parent; p != nil; p = p.parent {
				if p.div.onTextSelect() != nil {
					watcher = p
					break
				}
			}
			if watcher != nil {
				ox, oy := watcher.abs.X, watcher.abs.Y
				sel = TextSelection{Text: n.textEl.text[a:b],
					Bounds: Rect{X: s.drawn.X - ox, Y: s.drawn.Y - oy, W: s.drawn.W, H: s.drawn.H},
					Last:   Rect{X: s.drawnLast.X - ox, Y: s.drawnLast.Y - oy, W: s.drawnLast.W, H: s.drawnLast.H}}
			}
		}
	}
	var id uint64
	if watcher != nil {
		id = watcher.id
	}
	old := rt.selReported
	if old.id == id && old.sel == sel {
		return false
	}
	rt.selReported = reportedSelection{id, sel}
	if old.id != 0 && old.id != id {
		if o := rt.byID[old.id]; o != nil && o.div.onTextSelect() != nil {
			o.div.onTextSelect()(TextSelection{})
		}
	}
	if watcher != nil {
		watcher.div.onTextSelect()(sel)
	}
	return true
}
