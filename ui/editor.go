package ui

import (
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/text"
)

// The editor is the text-editing core behind TextInput and TextArea: one
// node that measures and paints its own text, selection and caret, and
// handles keyboard, mouse, clipboard and undo itself.

const (
	caretBlinkStop = 15 * time.Second // after this long without activity the caret stays solid
	undoGroupTime  = time.Second
	maxUndo        = 500
)

type editKind uint8

const (
	editOther editKind = iota
	editTyping
	editDeleting
)

type editSnap struct {
	value         string
	caret, anchor int
}

// editState is the persistent state of one editor (kept in nodeState).
type editState struct {
	caret, anchor int // byte offsets; the selection is between them
	prefX         float32
	havePrefX     bool

	undo, redo []editSnap
	lastKind   editKind
	lastEdit   time.Time
	known      string // value as last seen, to notice outside changes
	knownSet   bool

	scrollX   float32 // single-line horizontal scroll
	activity  time.Time
	dragging  bool
	clicks    int
	lastClick time.Time
	lastX     float32
	lastY     float32
	reveal    bool

	// The access content of the text, and the layout it was made from (see textContent).
	content       *access.TextContent
	contentLayout *text.Layout

	// From the last paint, for hit testing.
	layout           *text.Layout
	originX, originY float32 // text origin relative to the node
}

func (es *editState) selection() (int, int) {
	return min(es.caret, es.anchor), max(es.caret, es.anchor)
}

func (es *editState) hasSelection() bool { return es.caret != es.anchor }

// sync notices value changes that did not come from this editor (the app
// set the signal): they reset undo history and clamp the caret.
func (es *editState) sync(v string) {
	if es.knownSet && es.known == v {
		return
	}
	if es.knownSet {
		es.undo, es.redo = nil, nil
		es.lastKind = editOther
	}
	es.known, es.knownSet = v, true
	es.caret = clampToRune(v, es.caret)
	es.anchor = clampToRune(v, es.anchor)
}

func clampToRune(s string, i int) int {
	i = min(max(i, 0), len(s))
	for i > 0 && i < len(s) && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}

// editorElement is the internal element TextInput and TextArea expand into.
type editorElement struct {
	value       *Signal[string]
	placeholder string
	name        string // the accessible name, see TextInputElement.Label
	labelledBy  []string
	describedBy []string
	multiline   bool
	align       Alignment // where a single-line editor's text sits in its box (see alignOffset)
	onSubmit    func(string)
	onChange    func(string)
	onBlur      func()
	onCancel    func()
	description string // a literal description, read after the name (see fieldOptions)
	onStep      func(dir int)
	style       Style
	autoFocus   bool
	handle      *FocusHandle
	part        string // the Text part of the field, see asPart
	selection   Color  // the background of the selected text (FieldStyle.Selection)
	// selectionText is the color of the selected text, drawn over the selection;
	// unset, the selected text keeps its color.
	selectionText Color
	// placeholderColor is the color of the placeholder; unset, it is the text color.
	placeholderColor Color
	// invalid and required are the states a Field gives the field (see fieldWire).
	invalid, required bool
	// readOnly shows the text and lets it be selected and copied, but not
	// changed. disabled also takes the field out of the tab order and ignores
	// the pointer.
	readOnly, disabled bool
}

// locked reports whether the text can't be changed by the user or by
// assistive technology.
func (e *editorElement) locked() bool { return e.readOnly || e.disabled }

func (*editorElement) isElement() {}

func (e *editorElement) textStyle(n *node) text.Style {
	st := n.textStyle()
	st.NoWrap = !e.multiline
	return st
}

// fieldMinChars is the text whose width is the automatic minimum of a text field
// in a shrinking row (its min-content width): about six digits, as a short value
// needs. The minimum grows with the text size, and MinWidth replaces it.
const fieldMinChars = "000000"

// measureEditor sizes the editor to its content: one line at least.
func (n *node) measureEditor(maxWidth float32) (float32, float32) {
	e := n.editor
	v := e.value.Peek()
	if v == "" {
		v = e.placeholder
	}
	w := maxWidth
	if !e.multiline {
		w = float32(math.Inf(1))
	}
	l := n.rt.text.Layout(v, e.textStyle(n), w)
	return l.Width + 2, l.Height // room for the caret at the end
}

// buildEditor realizes an editor element. Its handlers live on a synthetic
// div so focus, key and pointer routing work exactly as for any element.
func (rt *Runtime) buildEditor(e *editorElement, n *node, inh textStyle) {
	n.editor = e
	s := Style{Cursor: CursorText}.Merge(e.style)
	n.style = s
	n.ts = inh.inherit(&s)
	n.lay.Style = s.layoutStyle(1)
	n.lay.Measure = n
	n.st = rt.state(n.id)
	if n.st.edit == nil {
		n.st.edit = &editState{}
	}
	id := n.id
	if e.disabled {
		// A disabled field shows its text and nothing else: it can't be focused,
		// selected or edited.
		s.Cursor = CursorDefault
		n.style = s
		d := &DivElement{style: s}
		d.asPart(e.part)
		d.access().disabled = true
		d.access().labelledBy, d.access().describedBy = e.labelledBy, e.describedBy
		n.div = d
		rt.setOverhang(n)
		return
	}
	d := &DivElement{style: s, focusable: true, capture: true, autoFocus: e.autoFocus, handle: e.handle}
	d.asPart(e.part)
	if e.multiline {
		d.KeyContext("Editor mode=full")
	} else {
		d.KeyContext("Editor mode=single_line")
	}
	// The keys of a field are actions (see DefaultBindings).
	for _, ea := range editorActions {
		fn := ea.fn
		d.OnAction(ea.a, func() { fn(rt, id) })
	}
	d.h.onMouseDown = func(ev *MouseEvent) { rt.editorMouseDown(id, ev) }
	d.h.onMouseMove = func(ev *MouseEvent) { rt.editorMouseMove(id, ev) }
	d.h.onMouseUp = func(*MouseEvent) {
		if n := rt.byID[id]; n != nil {
			n.st.edit.dragging = false
		}
	}
	if e.onStep != nil {
		// A stepping field takes Up and Down from the keys of the field, so they
		// step its value rather than move the caret.
		d.OnAction(moveUp, func() { e.onStep(1) })
		d.OnAction(moveDown, func() { e.onStep(-1) })
	}
	d.h.onTextInput = func(s string) { rt.editorInsert(id, s, editTyping) }
	d.h.onFocus = func() { rt.editorTouched(id) }
	d.h.onBlur = e.onBlur
	if len(e.labelledBy) > 0 || len(e.describedBy) > 0 {
		d.access().labelledBy, d.access().describedBy = e.labelledBy, e.describedBy
	}
	if e.description != "" {
		d.access().description = e.description
	}
	d.access().invalid, d.access().required = e.invalid, e.required
	d.access().readOnly = e.readOnly
	n.div = d
	rt.setOverhang(n)
}

func (rt *Runtime) editorOf(id uint64) (*node, *editState, string) {
	n := rt.byID[id]
	if n == nil || n.editor == nil || n.st == nil || n.st.edit == nil {
		return nil, nil, ""
	}
	v := n.editor.value.Peek()
	n.st.edit.sync(v)
	return n, n.st.edit, v
}

// editorTouched restarts the caret blink and asks for the caret to be
// scrolled into view on the next paint.
func (rt *Runtime) editorTouched(id uint64) {
	if n := rt.byID[id]; n != nil && n.st != nil && n.st.edit != nil {
		n.st.edit.activity = rt.now()
		n.st.edit.reveal = true
	}
	rt.needPaint = true
	rt.requestFrame()
}

func (rt *Runtime) now() time.Time {
	if rt.clock != nil {
		return rt.clock()
	}
	return time.Now()
}

// ---- edits ----

// editorSet replaces the value, recording undo history. Consecutive typing
// (or deleting) within a second forms one undo step; a typed space ends it.
func (rt *Runtime) editorSet(id uint64, nv string, caret int, kind editKind) {
	n, es, v := rt.editorOf(id)
	if n == nil || (nv != v && n.editor.locked()) {
		return
	}
	if nv != v {
		now := rt.now()
		grouped := kind != editOther && kind == es.lastKind && now.Sub(es.lastEdit) < undoGroupTime
		if !grouped {
			es.undo = append(es.undo, editSnap{v, es.caret, es.anchor})
			if len(es.undo) > maxUndo {
				es.undo = es.undo[1:]
			}
		}
		es.redo = nil
		es.lastKind, es.lastEdit = kind, now
		es.known = nv
	}
	es.caret, es.anchor = caret, caret
	es.havePrefX = false
	if nv != v {
		n.editor.value.Set(nv)
		if n.editor.onChange != nil {
			n.editor.onChange(nv)
		}
	}
	rt.editorTouched(id)
}

func (rt *Runtime) editorInsert(id uint64, s string, kind editKind) {
	n, es, v := rt.editorOf(id)
	if n == nil {
		return
	}
	if !n.editor.multiline {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	a, b := es.selection()
	if a != b {
		kind = editOther // replacing a selection is its own undo step
	}
	rt.editorSet(id, v[:a]+s+v[b:], a+len(s), kind)
	if s == " " {
		es.lastKind = editOther // words are undo steps
	}
}

func (rt *Runtime) editorDelete(id uint64, from, to int) {
	_, _, v := rt.editorOf(id)
	from, to = min(from, to), max(from, to)
	if from == to {
		return
	}
	rt.editorSet(id, v[:from]+v[to:], from, editDeleting)
}

func (rt *Runtime) editorUndo(id uint64, redo bool) {
	n, es, v := rt.editorOf(id)
	if n == nil || n.editor.locked() {
		return
	}
	from, to := &es.undo, &es.redo
	if redo {
		from, to = to, from
	}
	if len(*from) == 0 {
		return
	}
	snap := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	*to = append(*to, editSnap{v, es.caret, es.anchor})
	es.known = snap.value
	es.caret, es.anchor = snap.caret, snap.anchor
	es.lastKind = editOther
	n.editor.value.Set(snap.value)
	if n.editor.onChange != nil {
		n.editor.onChange(snap.value)
	}
	rt.editorTouched(id)
}

// ---- keyboard ----

// editorVertical moves the caret by lines, keeping its horizontal position.
func (rt *Runtime) editorVertical(es *editState, v string, k input.Key) int {
	l := es.layout
	if l == nil || len(l.Lines) == 0 {
		return es.caret
	}
	if !es.havePrefX {
		es.prefX, _ = l.Caret(es.caret)
		es.havePrefX = true
	}
	step := 1
	if k == input.KeyPageUp || k == input.KeyPageDown {
		step = max(int(rt.viewH/l.LineHeight)-2, 1)
	}
	if k == input.KeyUp || k == input.KeyPageUp {
		step = -step
	}
	target := l.LineOf(es.caret) + step
	switch {
	case target < 0:
		return 0
	case target >= len(l.Lines):
		return len(v)
	}
	return l.PositionInLine(target, es.prefX)
}

// ---- mouse ----

func (rt *Runtime) editorHit(n *node, es *editState, ev *MouseEvent) int {
	if es.layout == nil {
		return es.caret
	}
	return es.layout.HitTest(ev.LocalX-es.originX, ev.LocalY-es.originY)
}

func (rt *Runtime) editorMouseDown(id uint64, ev *MouseEvent) {
	n, es, v := rt.editorOf(id)
	if n == nil || ev.Button != input.ButtonLeft {
		return
	}
	p := rt.editorHit(n, es, ev)
	now := rt.now()
	if now.Sub(es.lastClick) < 500*time.Millisecond && abs32(ev.X-es.lastX) < 4 && abs32(ev.Y-es.lastY) < 4 {
		es.clicks++
	} else {
		es.clicks = 1
	}
	es.lastClick, es.lastX, es.lastY = now, ev.X, ev.Y
	es.havePrefX = false
	switch {
	case es.clicks == 2:
		es.anchor, es.caret = text.WordAt(v, p)
	case es.clicks >= 3:
		start := strings.LastIndexByte(v[:p], '\n') + 1
		end := strings.IndexByte(v[p:], '\n')
		if end < 0 {
			end = len(v)
		} else {
			end += p
		}
		es.anchor, es.caret = start, end
	case ev.Mods.Has(input.ModShift):
		es.caret = p
	default:
		es.caret, es.anchor = p, p
		es.dragging = true
	}
	rt.editorTouched(id)
}

func (rt *Runtime) editorMouseMove(id uint64, ev *MouseEvent) {
	n, es, _ := rt.editorOf(id)
	if n == nil || !es.dragging {
		return
	}
	if p := rt.editorHit(n, es, ev); p != es.caret {
		es.caret = p
		rt.editorTouched(id)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// ---- paint ----

// alignOffset returns how far right a single line of width text is moved in a field of
// width cw, for its alignment a. Start, and any alignment that is not Center or End,
// moves it not at all. A text wider than the field moves not at all either, since it
// scrolls.
func alignOffset(a Alignment, cw, width float32) float32 {
	room := max(cw-width, 0)
	switch a {
	case Center:
		return room / 2
	case End:
		return room
	}
	return 0
}

func (rt *Runtime) paintEditor(n *node, x, y, w float32, fg Color, opacity float32) {
	e := n.editor
	es := n.st.edit
	v := e.value.Peek()
	es.sync(v)
	s := &n.style
	pad := s.Padding
	be := s.borderEdges()
	cx, cy := x+pad.Left+be.Left, y+pad.Top+be.Top
	cw := max(w-pad.Left-pad.Right-be.Left-be.Right, 0)
	st := e.textStyle(n)
	wrap := cw + 0.01
	if !e.multiline {
		wrap = float32(math.Inf(1))
	}
	l := rt.text.Layout(v, st, wrap)
	es.layout = l
	focused := rt.focusID == n.id
	caretX, caretTop := l.Caret(es.caret)

	// A single-line text that fits is not scrolled: it sits at its alignment, and the
	// alignment is left out once the text is wider than the field, which scrolls instead.
	off := float32(0)
	if !e.multiline {
		// Keep the caret inside the field by scrolling the text sideways.
		const margin = 2
		switch {
		case l.Width <= cw:
			es.scrollX = 0
		case focused:
			if caretX-es.scrollX > cw-margin {
				es.scrollX = caretX - cw + margin
			}
			if caretX-es.scrollX < 0 {
				es.scrollX = caretX
			}
		}
		es.scrollX = min(max(es.scrollX, 0), max(l.Width-cw+margin, 0))
		off = alignOffset(e.align, cw, l.Width)
	} else {
		es.scrollX = 0
	}
	es.originX, es.originY = cx-x-es.scrollX+off, cy-y
	tx := cx + off - es.scrollX

	clip := !e.multiline
	if clip {
		rt.scene.PushClip(rt.px(scene.Rect{X: cx - 1, Y: y, W: cw + 2, H: n.lay.H}))
	}
	selected := focused && es.hasSelection() && (!e.selection.IsZero() || !e.selectionText.IsZero())
	if selected {
		a, b := es.selection()
		sel := e.selection.Alpha(opacity)
		rt.selRects = l.SelectionRects(a, b, rt.selRects[:0])
		for _, r := range rt.selRects {
			if !e.selection.IsZero() {
				rt.scene.AddQuad(scene.Quad{
					Bounds:     rt.px(scene.Rect{X: tx + r.X, Y: cy + r.Y, W: r.W, H: r.H}),
					Background: sel.scene(),
					Radii:      scene.Uniform(2 * rt.scale),
				})
			}
		}
	}
	if v == "" {
		if e.placeholder != "" {
			pc := fg
			if !e.placeholderColor.IsZero() {
				pc = e.placeholderColor
			}
			pl := rt.text.Layout(e.placeholder, st, wrap)
			rt.text.Draw(&rt.scene, pl, tx, cy, cw, rt.scale, 0, pc.Alpha(opacity).scene())
		}
	} else {
		rt.text.Draw(&rt.scene, l, tx, cy, cw, rt.scale, 0, fg.Alpha(opacity).scene())
		if selected && !e.selectionText.IsZero() {
			// The selected text is drawn again over its background, clipped to the
			// selection, in the selection's text color (HighlightText under high contrast).
			for _, r := range rt.selRects {
				rt.scene.PushClip(rt.px(scene.Rect{X: tx + r.X, Y: cy + r.Y, W: r.W, H: r.H}))
				rt.text.Draw(&rt.scene, l, tx, cy, cw, rt.scale, 0, e.selectionText.Alpha(opacity).scene())
				rt.scene.PopClip()
			}
		}
	}

	if focused {
		caretW := rt.caretWidth
		caret := scene.Rect{X: tx + caretX - caretW/2, Y: cy + caretTop, W: caretW, H: l.LineHeight}
		rt.textInput = caret
		rt.hasTextInput = true
		if es.reveal {
			es.reveal = false
			rt.reveal, rt.revealFrom = caret, n
		}
		if rt.caretVisible(es) {
			rt.scene.AddQuad(scene.Quad{Bounds: rt.px(caret), Background: fg.Alpha(opacity).scene()})
		}
	}
	if clip {
		rt.scene.PopClip()
	}
}

// caretVisible implements blinking: on, then off, each for the system's blink
// time (530 ms by default), starting at the last activity, and solid after a
// while of inactivity or when the system's blink is off. While it blinks it
// schedules exactly the frames needed to toggle it. In a window that is not
// active the caret is hidden and nothing blinks, as a native text field does.
func (rt *Runtime) caretVisible(es *editState) bool {
	if rt.windowInactive {
		return false
	}
	since := rt.lastFrame.Sub(es.activity)
	if since < 0 || since >= caretBlinkStop || rt.sys.CaretBlinkOff {
		return true
	}
	blink := rt.sys.CaretBlink
	phase := int(since / blink)
	next := es.activity.Add(time.Duration(phase+1) * blink)
	rt.wakeAt(next)
	return phase%2 == 0
}

// revealCaret scrolls the nearest vertical scroll container so the caret
// rectangle is visible. It reports whether anything scrolled.
func (rt *Runtime) revealCaret() bool {
	n := rt.revealFrom
	r := rt.reveal
	rt.revealFrom = nil
	if n == nil {
		return false
	}
	const margin = 12
	for a := n.parent; a != nil; a = a.parent {
		if a.style.OverflowY != OverflowScroll || a.st == nil {
			continue
		}
		view := a.abs.Intersect(a.clip)
		var delta float32
		switch {
		case r.Y < view.Y+margin:
			delta = r.Y - (view.Y + margin)
		case r.Bottom() > view.Bottom()-margin:
			delta = r.Bottom() - (view.Bottom() - margin)
		}
		if delta == 0 {
			return false
		}
		maxY := max(a.lay.ContentH-a.lay.H, 0)
		a.st.targetY = min(max(a.st.targetY+delta, 0), maxY)
		a.st.scrollY = a.st.targetY
		return true
	}
	return false
}
