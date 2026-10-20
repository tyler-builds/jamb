package codeedit

import (
	"sync/atomic"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/codeedit/highlight"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// An EditorElement is a code editor under construction. It is made by Editor and configured with the
// methods below, each of which returns the editor so that they chain. An EditorElement is an
// element: place it in a tree as it is, and it renders itself. Its size is Full by 300
// pixels unless the sizing methods (Size, Width, Height and the others) say otherwise.
type EditorElement struct {
	ui.Composite
	doc         *Document
	label       string
	lineNumbers bool
	readOnly    bool
	tabSize     int
	comment     string
	wrap        wrapSettings
	hoverP      HoverProvider
	hoverDelay  time.Duration
	complete    CompletionProvider
	size        sizes // see Size
	styles      Style
	hl          Highlighter
	lang        string
	// view is the state of the editor's latest render, which the tests read.
	view *viewState
}

// Editor returns an editor that shows doc. Line numbers are on, tabs are four columns wide,
// lines do not wrap, and the editor is writable with no comment prefix.
func Editor(doc *Document) *EditorElement {
	return &EditorElement{doc: doc, lineNumbers: true, tabSize: 4,
		wrap: wrapSettings{col: defaultWrapColumn, indent: IndentSame}, hoverDelay: defaultHoverDelay}
}

// Key gives the editor a stable identity among its siblings, so its state (the scroll
// position, the find bar) survives reordering.
func (e *EditorElement) Key(k string) *EditorElement { e.SetKey(k); return e }

// Label sets the name that assistive technology reads for the editor.
func (e *EditorElement) Label(s string) *EditorElement { e.label = s; return e }

// LineNumbers shows or hides the column of line numbers. It is shown by default. The
// fold markers are always shown.
func (e *EditorElement) LineNumbers(on bool) *EditorElement { e.lineNumbers = on; return e }

// ReadOnly shows the text and lets it be selected, moved through and copied, but not
// changed.
func (e *EditorElement) ReadOnly(on bool) *EditorElement { e.readOnly = on; return e }

// TabSize sets how many columns a tab stop is, for the display of tabs, for Tab and for
// the folding of indented lines. It is 4 by default.
func (e *EditorElement) TabSize(n int) *EditorElement {
	if n < 1 {
		n = 1
	}
	e.tabSize = n
	return e
}

// CommentPrefix sets the prefix that the comment command toggles at the start of each
// line, such as "// " for Go. With no prefix, the comment command does nothing.
func (e *EditorElement) CommentPrefix(prefix string) *EditorElement { e.comment = prefix; return e }

// Size sets the width and the height of the editor.
func (e *EditorElement) Size(w, h ui.Length) *EditorElement {
	e.size.setWidth(w)
	e.size.setHeight(h)
	return e
}

// The sizing methods give the editor's box a size, as they do for the widgets of the ui
// package. The editor is Full wide by default. See Sizing in docs/styling.md.
func (e *EditorElement) Width(l ui.Length) *EditorElement { e.size.setWidth(l); return e }

// Height sets the height of the editor, which is 300 pixels by default.
func (e *EditorElement) Height(h ui.Length) *EditorElement { e.size.setHeight(h); return e }

// MinWidth sets the smallest width of the editor.
func (e *EditorElement) MinWidth(l ui.Length) *EditorElement { e.size.setMinWidth(l); return e }

// MinHeight sets the smallest height of the editor.
func (e *EditorElement) MinHeight(l ui.Length) *EditorElement { e.size.setMinHeight(l); return e }

// MaxWidth sets the largest width of the editor.
func (e *EditorElement) MaxWidth(l ui.Length) *EditorElement { e.size.setMaxWidth(l); return e }

// MaxHeight sets the largest height of the editor.
func (e *EditorElement) MaxHeight(l ui.Length) *EditorElement { e.size.setMaxHeight(l); return e }

// Grow makes the editor take a share of the free space on its parent's main axis.
func (e *EditorElement) Grow(f float32) *EditorElement { e.size.setGrow(f); return e }

// GridSpan sets how many columns and rows the editor covers in its parent's grid.
func (e *EditorElement) GridSpan(cols, rows int) *EditorElement {
	e.size.setSpan(cols, rows)
	return e
}

// Styles sets the look of the parts of the editor, for this editor only. Set fields win
// over the theme's Style and the defaults. Calling Styles again merges with the earlier values.
func (e *EditorElement) Styles(st Style) *EditorElement {
	e.styles = e.styles.merge(st)
	return e
}

// Highlighter sets the syntax highlighter of the editor, when it is not a session the editor
// starts itself (see Language). The highlighter must have been made from the document's text,
// and the editor sends it each later change. A nil highlighter shows plain text.
func (e *EditorElement) Highlighter(h Highlighter) *EditorElement { e.hl = h; return e }

// Language highlights the document in the language named lang, as a code fence names it, such
// as "go", "python" or "ts" (see highlight.Languages). The editor starts a highlight session on
// the document's text, sends it each change, and paints the spans it has. It closes the session
// when the language or the document changes. A document over the highlight limit, or in a
// language the highlighter does not know, is shown without colors, and "" shows plain text.
func (e *EditorElement) Language(lang string) *EditorElement { e.lang = lang; return e }

// Document returns the document the editor shows.
func (e *EditorElement) Document() *Document { return e.doc }

// Render builds the editor. It is called by the UI, and reads the signals of the editor's
// state, so the editor renders again when the text, the selections or the view change.
func (e *EditorElement) Render(ctx *ui.Context) ui.Element {
	vs := ui.Local(ctx, "codeedit.view", newViewState()).Peek()
	vs.rev.Get() // the editor renders again when the view changes
	vs.ctx = ctx
	vs.latest.Store(ctx)
	if !vs.closeOnDispose {
		// The session is closed when the editor's component is removed. Registered once per
		// view state, which lives as long as the component does.
		vs.closeOnDispose = true
		ctx.OnDispose(func() {
			vs.closeSession()
			vs.stopRequests()
		})
	}
	e.view = vs
	if vs.doc != e.doc {
		vs.attachDoc(e.doc)
	}
	vs.syncSession(e.lang)
	var hl Highlighter
	switch {
	case e.hl != nil:
		hl = e.hl
	case vs.session != nil:
		hl = vs.session
	}
	if vs.hl != hl {
		// The spans and the viewport belong to the highlighter they came from.
		vs.hl = hl
		vs.spans = spanIndex{}
		vs.vpSet = false
	}
	vs.tab = e.tabSize
	vs.wrap = e.wrap
	vs.hoverP, vs.hoverDelay = e.hoverP, e.hoverDelay
	vs.completeP = e.complete
	if e.readOnly {
		vs.completeP = nil // nothing to complete in a text that does not change
	}
	vs.doc.SetTabSize(e.tabSize)
	t := ctx.Theme()
	st := e.styles.resolve(ctx)
	name := e.label
	if name == "" {
		name = ctx.Strings().CodeEditorLabel
	}
	ps := paintSettings{st: st, t: t, numbers: e.lineNumbers, tab: e.tabSize, readOnly: e.readOnly, focused: vs.focused}
	canvas := ui.Canvas(func(p *ui.Painter, w, h float32) {
		vs.paint(p, w, h, ps)
	}).Size(ui.Full, ui.Full).Grow(1).Role(access.RoleTextArea).Label(name)
	canvas.Cursor(ui.CursorText).KeyContext("CodeEditor").Focusable().FocusHandle(vs.canvasFocus)
	e.wire(canvas, vs)
	canvas.TextAccess(ui.TextAccess{
		Info:   func() *access.TextInfo { return vs.textAccess(e.readOnly, st.Text.FontSize) },
		Select: vs.selectText,
		Reveal: vs.revealText,
	})
	// The caret shows focus, as in code editors, so the frame draws no focus ring. BoxFocused
	// styles the frame while the editor has focus: unset by default, a border under high
	// contrast (see Style). The default size comes before the Box style, and the sizes the
	// app set come after it, so they win over it.
	root := ui.Div().Column().Size(ui.Full, ui.Px(300)).Style(st.Box).FocusWithin(st.BoxFocused)
	e.size.apply(root)
	if vs.bar != barNone {
		root.Child(vs.findBar(ctx, st, e.readOnly))
	}
	// The canvas sits in a box of its own, which the popups hang from at the text they are about.
	area := ui.Div().Size(ui.Full, ui.Full).Grow(1).Child(canvas)
	if vs.hover.open {
		area.Child(vs.hoverPopup(st))
	}
	if vs.completion.open {
		area.Child(vs.completionPopup(st))
	}
	root.Child(area)
	return root
}

// wire attaches the pointer, keyboard, text and focus handling to the canvas of the editor.
func (e *EditorElement) wire(canvas *ui.DivElement, vs *viewState) {
	canvas.OnFocus(func() { vs.focused = true; vs.restart = true; vs.touch() })
	canvas.OnBlur(func() { vs.focused = false; vs.dragging = false; vs.closePopups(); vs.touch() })
	canvas.OnMouseDown(func(ev *ui.MouseEvent) { vs.mouseDown(ev) })
	canvas.OnMouseMove(func(ev *ui.MouseEvent) { vs.mouseMove(ev) })
	canvas.OnMouseUp(func(*ui.MouseEvent) { vs.dragging = false })
	canvas.OnScroll(func(ev *ui.ScrollEvent) {
		vs.scroll(ev.DeltaX, ev.DeltaY)
		ev.StopPropagation()
	})
	canvas.OnTextInput(func(s string) {
		if !e.readOnly {
			vs.edit(func(d *Document) error { return d.TypeText(s) })
			vs.typedCompletion(s)
		}
	})
	e.wireIME(canvas, vs)
	for _, b := range editorActions {
		fn := b.fn
		canvas.OnAction(b.action, func() {
			vs.commitComposition() // a key the editor handles types what is composed first
			fn(e, vs)
		})
	}
}

// viewState is the state of one editor, kept by the UI for the editor's node: the document
// it shows and what it has subscribed to, the scroll position, the find bar, and the
// geometry of the last paint, which the pointer handlers read.
type viewState struct {
	doc         *Document
	cancelDoc   []func()
	hl          Highlighter
	rev         *ui.Signal[uint64]         // changes when the view should render again
	ctx         *ui.Context                // the context of the latest render, for the clipboard
	latest      atomic.Pointer[ui.Context] // the same, for the highlight worker, which posts to it from its goroutine
	tab         int
	focused     bool
	dragging    bool
	reveal      bool // the next paint scrolls the primary caret into view
	scrollX     float32
	scrollY     float32
	maxCells    int    // the widest shown row, in cells, from the last paint
	fontName    string // the name of the monospace face, read by the first paint
	geo         geometry
	spans       spanIndex
	capStyle    []capStyle
	capTheme    *ui.Theme // the theme that capStyle was made from
	vpLo        int
	vpHi        int
	vpSet       bool
	bar         barKind
	query       *ui.Signal[string]
	repl        *ui.Signal[string]
	gotoLine    *ui.Signal[string]
	optCase     *ui.Signal[bool]
	optWord     *ui.Signal[bool]
	optRegex    *ui.Signal[bool]
	finder      *Finder
	finderKey   finderKey
	finderErr   bool // the query is an invalid pattern
	focusBar    bool // the bar takes the keyboard focus when it next renders
	canvasFocus *ui.FocusHandle
	model       *editorModel // the Text pattern's model, kept while the text and the layout are the same (see textModel)
	// comp is what an input method composes at the primary caret, shown but not typed (see ime.go).
	comp ui.Composition
	// session is the highlight session started for sessionKey, when Language set one; langErr is
	// why a session could not be started (highlight.ErrTooLarge or highlight.ErrUnknownLanguage).
	session        *highlight.Session
	sessionKey     sessionKey
	langErr        error
	closeOnDispose bool // closeSession is registered with the component's OnDispose

	// wrap is how the editor wraps its rows, and wc the wrap of the rows at the width of the last
	// paint. upstream is the affinity of the primary caret at the edge of two segments, as of the
	// document's version upVer (see setUpstream).
	wrap     wrapSettings
	wc       wrapCache
	upstream bool
	upVer    uint64

	// The hooks of a language server: the hover's provider and delay, and the hover popup; the
	// provider of the completions, and the completion list.
	hoverP     HoverProvider
	hoverDelay time.Duration
	hover      hoverState
	completeP  CompletionProvider
	completion completionState

	// activity is the time of the latest edit, move or click, from which the caret blinks. restart
	// asks the next paint to set it to that paint's time (see Painter.Now).
	activity time.Time
	restart  bool
}

// sessionKey is what a highlight session was started for: its language, and its document.
type sessionKey struct {
	lang string
	doc  *Document
}

// syncSession starts a highlight session for lang on the document, or closes the one it has when
// lang or the document changed. The session's worker asks for a frame through ctx.Post when it
// has new spans, so the call is safe from the worker. A failure to start (a document over the
// size limit, or an unknown language) leaves the text uncolored and is kept in langErr.
func (vs *viewState) syncSession(lang string) {
	key := sessionKey{lang: lang, doc: vs.doc}
	if key == vs.sessionKey && (lang == "" || vs.session != nil || vs.langErr != nil) {
		return
	}
	vs.closeSession()
	vs.sessionKey = key
	vs.langErr = nil
	if lang == "" {
		return
	}
	rev := vs.rev
	// The session outlives the render that started it, and the window may have changed since
	// (see ui.Dock), so the notice goes to the context of the latest render.
	s, err := highlight.NewSession(lang, []byte(vs.doc.Text()), highlight.Options{
		Notify: func() {
			if c := vs.latest.Load(); c != nil {
				c.Post(func() { rev.Set(rev.Peek() + 1) })
			}
		},
	})
	if err != nil {
		vs.langErr = err
		return
	}
	vs.session = s
}

// closeSession closes the highlight session the view started, if it has one. Close waits for the
// session's worker, which does not wait for the UI, so it is safe on the UI thread.
func (vs *viewState) closeSession() {
	if vs.session != nil {
		_ = vs.session.Close()
		vs.session = nil
	}
}

// geometry is where the last paint put the text, in the canvas's pixels.
type geometry struct {
	lineH   float32
	charW   float32
	gutterW float32 // the width of the numbers and the fold markers, up to the text
	textX   float32 // where the text starts, before the horizontal scroll
	foldX   float32 // the left edge of the fold markers
	viewW   float32
	viewH   float32
	painted bool
}

// newViewState returns the state of a new editor.
func newViewState() *viewState {
	return &viewState{
		rev:         ui.NewSignal(uint64(0)),
		query:       ui.NewSignal(""),
		repl:        ui.NewSignal(""),
		gotoLine:    ui.NewSignal(""),
		optCase:     ui.NewSignal(false),
		optWord:     ui.NewSignal(false),
		optRegex:    ui.NewSignal(false),
		tab:         4,
		canvasFocus: ui.NewFocusHandle(),
	}
}

// touch asks the editor to render again.
func (vs *viewState) touch() { vs.rev.Set(vs.rev.Peek() + 1) }

// attachDoc makes the editor show doc. It cancels the subscriptions to the document it had,
// and subscribes to its changes: each change renders the editor again, and each edit is
// sent to the highlighter and to the wrap of the rows at once.
func (vs *viewState) attachDoc(doc *Document) {
	for _, c := range vs.cancelDoc {
		c()
	}
	vs.doc = doc
	vs.cancelDoc = []func(){
		doc.OnChange(vs.touch),
		doc.OnEdit(func(c Change) {
			if vs.hl != nil {
				// The highlighter reports its own errors (see its Err); the edit itself
				// is never refused, so a keystroke is not held up by the highlighter.
				_ = vs.hl.Edit(highlight.EditRecord{Start: c.From, OldEnd: c.To, Text: []byte(c.Text)})
			}
		}),
		doc.OnEdit(func(c Change) { vs.wc.edit(doc, c) }),
		doc.onRedraw(vs.touch),
		doc.OnEdit(func(c Change) { vs.closeHover(); vs.completionEdited(c) }),
	}
	vs.stopRequests()
	vs.wc = wrapCache{}
	vs.dragging = false
	vs.vpSet = false
	vs.spans = spanIndex{}
}

// edit runs fn on the document and asks the editor to show the result: the caret is kept
// in view. Errors from the document are returned to the caller, so they can be dropped
// where a key has nothing to do.
func (vs *viewState) edit(fn func(*Document) error) {
	_ = fn(vs.doc)
	vs.followCompletion()
	vs.reveal = true
	vs.restart = true
	vs.touch()
}

// move runs a caret movement on the document, and scrolls the caret into view. While the rows
// wrap, the vertical moves and Home and End go by display rows (see moveWrapped).
func (vs *viewState) move(m Move, extend bool) {
	if !vs.wrapping() || !vs.moveWrapped(m, extend) {
		vs.doc.Move(m, extend, vs.pageRows())
	}
	vs.movedCompletion()
	vs.reveal = true
	vs.restart = true
	vs.touch()
}

// pageRows is how many shown rows a page of the editor holds.
func (vs *viewState) pageRows() int {
	if vs.geo.lineH <= 0 {
		return 1
	}
	return max(int(vs.geo.viewH/vs.geo.lineH)-1, 1)
}

// scroll moves the view by dx and dy canvas pixels. The paint clamps the position. A view that
// wraps its rows to its width does not scroll horizontally.
func (vs *viewState) scroll(dx, dy float32) {
	if vs.fitsView() {
		dx = 0
	}
	vs.scrollX += dx
	vs.scrollY += dy
	if vs.scrollX < 0 {
		vs.scrollX = 0
	}
	if vs.scrollY < 0 {
		vs.scrollY = 0
	}
	vs.touch()
}

// hit returns the row and the byte offset under the canvas point (x, y), from the last paint,
// and reports whether there was a paint to hit against.
func (vs *viewState) hit(x, y float32) (row, p int, ok bool) {
	dr, p, _, ok := vs.hitAt(x, y)
	return dr.row, p, ok
}

// hitAt returns the display row under the canvas point (x, y), from the last paint, and the byte
// offset there. A point below the text is on the last display row, and a point in the indent of
// a segment is at its start. upstream is true when the offset is the end of a segment that is
// not the last of its row, where the caret shows at the end of that segment rather than at the
// start of the next. ok is false when there was no paint to hit against.
func (vs *viewState) hitAt(x, y float32) (dr displayRow, p int, upstream, ok bool) {
	g := vs.geo
	if !g.painted || g.lineH <= 0 {
		return displayRow{}, 0, false, false
	}
	ix := vs.rowIndex()
	dr, found := vs.rowAt(ix, max(int((y+vs.scrollY)/g.lineH), 0))
	if !found {
		dr, _ = vs.rowAt(ix, max(ix.count()-1, 0))
	}
	cell := int((x - g.textX + vs.scrollX + g.charW/2) / g.charW)
	p, upstream = vs.offsetOn(dr, max(cell, 0))
	return dr, p, upstream, true
}

// mouseDown handles a press: a fold marker folds or unfolds its row, a line number selects
// its row, and a press in the text places the caret. Alt adds a caret, Shift extends the
// primary selection, and a double or triple click selects a word or a row.
func (vs *viewState) mouseDown(ev *ui.MouseEvent) {
	if ev.Button != input.ButtonLeft {
		return
	}
	vs.commitComposition() // a click types what is composed first: the layout it hits shows it
	vs.closePopups()
	g := vs.geo
	dr, p, up, ok := vs.hitAt(ev.LocalX, ev.LocalY)
	if !ok {
		return
	}
	row := dr.row
	vs.restart = true
	switch {
	case ev.LocalX >= g.foldX && ev.LocalX < g.textX && dr.seg == 0 && vs.doc.opensBlock(row, vs.tab):
		if !vs.doc.Unfold(row) {
			vs.doc.Fold(row)
		}
	case ev.LocalX < g.textX:
		vs.doc.SelectRow(row)
	case ev.Mods.Has(input.ModAlt):
		vs.doc.AddCaret(p)
	case ev.Mods.Has(input.ModShift):
		vs.doc.extendPrimary(p)
		vs.setUpstream(up)
	case ev.Clicks == 2:
		vs.doc.SelectWord(p)
	case ev.Clicks >= 3:
		vs.doc.SelectRow(row)
	default:
		vs.doc.SetCaret(p)
		vs.setUpstream(up)
		vs.dragging = true
	}
	vs.touch()
}

// mouseMove extends the primary selection while the button is held, and otherwise schedules
// the hover at the text under the pointer.
func (vs *viewState) mouseMove(ev *ui.MouseEvent) {
	if !vs.dragging {
		vs.pointerRest(ev)
		return
	}
	if _, p, up, ok := vs.hitAt(ev.LocalX, ev.LocalY); ok && (p != vs.doc.Primary().Head || up != vs.upstreamNow()) {
		vs.doc.extendPrimary(p)
		vs.setUpstream(up)
		vs.touch()
	}
}

// extendPrimary moves the head of the primary selection to p, keeping its anchor.
func (d *Document) extendPrimary(p int) {
	sels := append([]Selection(nil), d.sels...)
	sels[d.primary] = Selection{Anchor: sels[d.primary].Anchor, Head: p}
	d.setSels(sels, d.primary)
}
