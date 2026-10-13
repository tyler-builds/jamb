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
	clicks        int
	lastClick     time.Time
	lastX, lastY  float32
	layout        *text.Layout // from the last paint, for hit testing
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
	d := &DivElement{style: Style{Cursor: CursorText}.Merge(n.style), focusable: true, noTabStop: true, capture: true}
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
	d.h.onMouseMove = func(e *MouseEvent) { rt.selectMouseMove(id, e) }
	d.h.onMouseUp = func(*MouseEvent) {
		if n := rt.byID[id]; n != nil {
			n.st.sel.dragging = false
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

func (rt *Runtime) selectMouseDown(id uint64, e *MouseEvent) {
	n, s := rt.selectionOf(id)
	if n == nil || e.Button != input.ButtonLeft {
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
	switch {
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
	if p := rt.selectHit(n, s, e.X, e.Y); p != s.caret {
		s.caret = p
		rt.needPaint = true
		rt.requestFrame()
	}
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
	if rt.focusID != n.id {
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
	for _, r := range rt.selRects {
		li := 0
		if l.LineHeight > 0 {
			li = min(max(int(r.Y/l.LineHeight+0.5), 0), len(l.Lines)-1)
		}
		b := rt.px(scene.Rect{X: x + n.lineOffset(l, li) + r.X, Y: y + r.Y, W: r.W, H: r.H})
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
