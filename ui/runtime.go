package ui

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/layout"
	"github.com/tyler-builds/jamb/scene"
	"github.com/tyler-builds/jamb/text"
)

type nodeKind uint8

const (
	kindDiv nodeKind = iota
	kindText
	kindIcon
	kindImage
	kindEditor
)

// nodeFlags mark what a div node needs from paint and hit testing beyond
// drawing itself. They are checked for every node on every repaint, so they
// live on the node: reading the element instead costs a cache miss per node.
type nodeFlags uint8

const (
	flagOverlay nodeFlags = 1 << iota // Overlay: painted and hit-tested from the overlay list
	flagOutside                       // has OnClickOutside
	flagList                          // a UniformList's or ListOf's scroll container
	flagExit                          // has Exit: paint records what a ghost of it needs
	flagDrag                          // has OnDrag: a press on it can start a drag
	flagDrop                          // has OnDrop: a drag can end over it
	flagRich                          // rich text with a clickable or hoverable segment
)

// nodeState is per-element state that survives rebuilds, keyed by element ID.
type nodeState struct {
	gen uint32

	scrollX, scrollY float32
	targetX, targetY float32

	animReady      bool
	bg, border, fg Color

	edit    *editState
	focus   *FocusHandle // default handle of a text field
	list    *listState   // rows of a UniformList's scroll container
	varList *varState    // rows of a ListOf's scroll container
	anim    *animState   // progress of an Animate instance

	// Paint-only values the transitions last showed, and the times an
	// entrance or an exit began (zero when there is none).
	shown            visual
	enterAt, leaveAt time.Time

	// The largest ink overhang and scale growth this element has had, kept
	// for an element with a Transition: its displayed values ease from the
	// old target, so the overhang must still cover them.
	overhang, growOverhang float32
}

// node is the realization of an element: style resolved, laid out and
// painted. Nodes are retained between frames; a component re-render replaces
// only the nodes of its own subtree.
type node struct {
	lay      layout.Node
	rt       *Runtime
	id       uint64
	kind     nodeKind
	flags    nodeFlags // copied from the element so paint and hit testing need not load it
	parent   *node
	children []*node
	order    []*node // paint order: children sorted by ZIndex

	div    *DivElement
	textEl *TextElement
	rich   *richText // set when textEl has segments or highlights
	icon   *IconElement
	img    *ImageElement
	editor *editorElement
	st     *nodeState

	// owner is the innermost component instance that built this node;
	// boundary is set when this node is the root of an instance's output.
	owner    *componentInstance
	boundary *componentInstance
	gen      uint32 // frame in which the node was built
	fresh    bool   // the element did not exist before this build
	key      string // the element's key (or its component's), for Find and Dump

	style Style
	ts    textStyle

	// Written by paint, read by hit testing. base is the position before the
	// element's own offset; inOpacity and inFg are what its parent passed down.
	// They let an exiting element be painted where it was.
	abs       scene.Rect
	hit       scene.Rect // the box hit testing uses; see paintNode
	clip      scene.Rect
	baseX     float32
	baseY     float32
	inOpacity float32
	inFg      Color

	// paintGen is the paint in which the node was last painted, and painted
	// lists the children it painted in that paint, in paint order. Only
	// painted nodes are hit. keepGen marks the focused node and its
	// ancestors, which are painted even when off screen.
	paintGen uint32
	keepGen  uint32
	painted  []*node
}

type textStyle struct {
	size       float32
	weight     FontWeight
	family     string
	lineHeight float32
	align      Alignment
}

func (ts textStyle) inherit(s *Style) textStyle {
	if s.FontSize != 0 {
		ts.size = s.FontSize
	}
	if s.FontWeight != 0 {
		ts.weight = s.FontWeight
	}
	if s.FontFamily != "" {
		ts.family = s.FontFamily
	}
	if s.LineHeight != 0 {
		ts.lineHeight = s.LineHeight
	}
	if s.TextAlign != 0 {
		ts.align = s.TextAlign
	}
	return ts
}

func (n *node) textStyle() text.Style {
	// The Text size setting scales the font, and the line height with it, since
	// the line height is a multiple of the size.
	st := text.Style{Family: n.ts.family, Size: n.ts.size * n.rt.textFactor, Weight: n.ts.weight, LineHeight: n.ts.lineHeight}
	if n.textEl != nil {
		st.MaxLines = n.textEl.maxLines
		st.NoWrap = n.textEl.noWrap
		st.Italic = n.textEl.italic
	}
	return st
}

// Measure implements layout.Measurer for leaves.
func (n *node) Measure(maxWidth float32) (float32, float32) {
	switch n.kind {
	case kindText:
		l := n.textLayout(maxWidth)
		return l.Width, l.Height
	case kindIcon:
		return n.icon.size, n.icon.size
	case kindImage:
		if n.img.src == nil {
			return 0, 0
		}
		return float32(n.img.src.width), float32(n.img.src.height)
	case kindEditor:
		return n.measureEditor(maxWidth)
	}
	return 0, 0
}

// FrameStats describes the work done by the last Frame call.
type FrameStats struct {
	Nodes        int // live nodes in the tree
	Rendered     int // components (and composite elements) whose render function ran
	NodesBuilt   int // nodes created by those renders
	Restyled     int // nodes restyled in place (hover, press, focus)
	NodesLaidOut int // nodes whose children were re-positioned
	Measured     int // layout measurements that were not served from cache
	NodesPainted int // nodes painted (off-screen subtrees are culled and not counted)
	Built        bool
	LaidOut      bool
	Painted      bool
	BuildTime    time.Duration
	LayoutTime   time.Duration
	PaintTime    time.Duration
	Quads        int
	Glyphs       int
}

// Runtime turns a root render function into scenes and routes input to
// element handlers. The app package drives it; it has no platform or GPU
// dependencies, which also makes it usable headless in tests.
//
// A Runtime is not safe for concurrent use: call Frame and the input methods
// from one goroutine (the UI thread). Signals are the exception and may be
// set from anywhere.
type Runtime struct {
	rootEl   *ComponentElement
	rootInst *componentInstance
	theme    *Theme // the effective theme: the app's, or the high contrast one
	appTheme *Theme // the theme the app set with SetTheme
	sys      SystemSettings
	// textFactor scales the font size of text (see textStyle); caretWidth is the
	// width of the text caret in logical pixels. Both follow sys.
	textFactor float32
	caretWidth float32
	text       *text.System
	engine     layout.Engine
	scene      scene.Scene
	tree       *node
	byID       map[uint64]*node

	instances map[uint64]*componentInstance
	states    map[uint64]*nodeState
	gen       uint32

	viewW, viewH, scale float32 // the UI's own coordinates: device size divided by zoom

	// Zoom scales the whole UI. Inputs arrive in device-logical pixels
	// (devW × devH at devScale) and are divided by zoom.
	devW, devH, devScale float32
	zoom                 float32
	zoomKeys             bool
	onZoom               func(float32)
	wheelZoom            float32 // accumulated Ctrl+wheel delta

	// Dirty components, appended from any goroutine.
	dirtyMu  sync.Mutex
	dirty    []*componentInstance
	hasDirty atomic.Bool
	// framePending coalesces wake-ups: many changes before the next frame
	// produce a single call to wake.
	framePending atomic.Bool
	// Functions queued by Post, run at the start of the next frame.
	postMu sync.Mutex
	posted []func()

	needLayout, needPaint bool
	animating             bool                 // something animates: the next frame must come
	looping               bool                 // a repeating Animate runs: the next frame must come, but it never ends
	animInsts             []*componentInstance // Animate instances that were running; re-rendered by the next Frame
	listStale             bool                 // a UniformList must rebuild its rows before the frame ends
	scrolled              bool
	wake                  func()
	warn                  func(string)
	lastFrame             time.Time

	mouseX, mouseY float32
	mouseIn        bool
	hoverIDs       []uint64 // deepest first
	pressIDs       []uint64
	focusID        uint64
	blurring       uint64 // the element whose OnBlur is running, see setFocus
	// keyboardModality is true when the last input was a key press and false
	// after a pointer press. FocusVisible styles show only while it is true.
	keyboardModality bool
	// revealFocus asks the next paint to scroll the focused element into view
	// (set by Tab), in its nearest vertically scrolling ancestor.
	revealFocus bool
	cursor      input.Cursor
	hitPath     []*node

	stats    FrameStats
	restyled int // restyles since the last frame (they happen during input handling)

	services       Services
	windowCtl      WindowControls // the app's window, see SetWindowControls
	maximized      *Signal[bool]  // the window's state, read by Context.Window
	focused        *Signal[bool]
	memClipboard   string // used when no platform clipboard is wired in
	clock          func() time.Time
	captureID      uint64 // element receiving pointer moves while a button is held
	drag           dragState
	pendingFocus   uint64
	pendingHandles []*FocusHandle
	selRects       []text.Rect
	textInput      scene.Rect // caret of the focused editor, logical pixels
	hasTextInput   bool
	reveal         scene.Rect
	revealFrom     *node
	sbHot          scrollbarID // scrollbar under the pointer
	hoverSeg       segRef      // rich text segment under the pointer
	pressSeg       segRef      // rich text segment the left button went down on
	sbDrag         scrollDrag
	sbScratch      []scrollbar

	// Floating overlays and click-outside listeners found by the last paint.
	overlays []overlayItem
	outside  []*node
	// paintGen counts paints. unculled is above zero while painting inside a
	// scaled node, where the culling test (in unscaled coordinates) does not
	// apply, so nothing is culled there.
	paintGen uint32
	unculled int
	// Elements that left the tree while an exit animation runs, in the order
	// they left. inGhost is set while painting one, and marks what it adds.
	ghosts  []*node
	inGhost bool
	// Tooltip: the element the pointer rests on, the one showing a tooltip,
	// and when the candidate's delay ends. tipDismissed is a candidate that
	// Escape hid; it does not show again until the pointer moves to another.
	tipCandidate, tipShown, tipDismissed uint64
	// escapeDismissed is set when an Escape press closed an overlay or hid a
	// tooltip; that press's auto-repeats are then ignored, so holding Escape
	// closes one overlay, and the key reaches nothing else, until it is
	// pressed again.
	escapeDismissed bool
	tipAt           time.Time

	// Key bindings: keymap in add order, and the strokes of a chord in
	// progress, with the element that had focus when it started. swallowText
	// drops the text a consumed key would have typed (or, during a chord, keeps
	// it with the stroke).
	keymap      []Binding
	pending     []pendingStroke
	pendingAt   time.Time
	chordFocus  uint64
	swallowText bool

	// Menus (see menu.go). ctxMenu is the open context menu. menuReturn is the
	// focus that the open menu chain returns to when it closes. pointerFocus is
	// the focus before the latest press, which a menu opened by the pointer
	// returns to. menuFn is a submenu's hover change, waiting until menuAt.
	// altArmed is set while Alt alone is held, so that its release focuses the
	// menu bar.
	ctxMenu      *contextMenu
	menuReturn   *FocusHandle
	pointerFocus uint64
	menuAt       time.Time
	menuFn       func()
	altArmed     bool

	// Modals (see modal.go): the open ones, with the focus each returns to, and
	// the one whose initial focus is set once the next build is done.
	modals       []modalFrame
	pendingModal uint64

	// acc is the accessibility tree's state (see access.go). It is nil until
	// the tree is first asked for, so a runtime without assistive technology
	// keeps none of it.
	acc *accessState
}

// Services are platform capabilities the runtime uses when available. The
// app package wires them to the window; headless runtimes (tests) work
// without them, with an in-process clipboard.
type Services struct {
	Clipboard    func() (string, bool)
	SetClipboard func(string)
	// WakeAt schedules a frame at a future time (caret blinking).
	WakeAt func(time.Time)
}

// SetServices connects platform capabilities.
func (rt *Runtime) SetServices(s Services) { rt.services = s }

func (rt *Runtime) clipboard() (string, bool) {
	if rt.services.Clipboard != nil {
		return rt.services.Clipboard()
	}
	return rt.memClipboard, rt.memClipboard != ""
}

func (rt *Runtime) setClipboard(s string) {
	if rt.services.SetClipboard != nil {
		rt.services.SetClipboard(s)
		return
	}
	rt.memClipboard = s
}

func (rt *Runtime) wakeAt(t time.Time) {
	if rt.services.WakeAt != nil {
		rt.services.WakeAt(t)
	}
}

// TextInputRect returns the caret of the focused text editor in logical
// pixels, so the platform can place input-method windows next to it.
func (rt *Runtime) TextInputRect() (scene.Rect, bool) {
	r := rt.textInput
	z := rt.zoom
	return scene.Rect{X: r.X * z, Y: r.Y * z, W: r.W * z, H: r.H * z}, rt.hasTextInput
}

// NewRuntime creates a runtime for the given root render function.
func NewRuntime(root func(ctx *Context) Element, theme *Theme, ts *text.System) *Runtime {
	if theme == nil {
		theme = DarkTheme
	}
	if ts == nil {
		ts = text.NewSystem()
	}
	return &Runtime{
		rootEl:     Component(root),
		theme:      theme,
		appTheme:   theme,
		sys:        SystemSettings{TextScale: 100, CaretBlink: defaultCaretBlink, CaretWidth: 1},
		textFactor: 1,
		caretWidth: defaultCaretWidth,
		text:       ts,
		maximized:  NewSignal(false),
		focused:    NewSignal(false),
		byID:       map[uint64]*node{},
		instances:  map[uint64]*componentInstance{},
		states:     map[uint64]*nodeState{},
		keymap:     DefaultBindings(), // first, so bindings added later win at the same depth
		scale:      1,
		paintGen:   1, // nodes start at 0, so none counts as painted before the first paint
		devScale:   1,
		zoom:       1,
		zoomKeys:   true,
		warn:       func(s string) { fmt.Fprintln(os.Stderr, "jamb:", s) },
	}
}

// SetWakeFunc registers the function called (from any goroutine) when the
// runtime needs a new frame. It is called at most once between frames.
func (rt *Runtime) SetWakeFunc(fn func()) { rt.wake = fn }

// SetWarnFunc replaces where non-fatal warnings (such as duplicate keys) go.
// The default prints to stderr.
func (rt *Runtime) SetWarnFunc(fn func(string)) { rt.warn = fn }

func (rt *Runtime) requestFrame() {
	if rt.framePending.CompareAndSwap(false, true) && rt.wake != nil {
		rt.wake()
	}
}

// Post runs fn on the UI thread at the start of the next frame and requests
// that frame. Safe from any goroutine: background work hands results to the
// UI this way.
func (rt *Runtime) Post(fn func()) {
	rt.postMu.Lock()
	rt.posted = append(rt.posted, fn)
	rt.postMu.Unlock()
	rt.requestFrame()
}

// runPosted runs the functions queued by Post, in the order they were posted.
func (rt *Runtime) runPosted() {
	rt.postMu.Lock()
	fns := rt.posted
	rt.posted = nil
	rt.postMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

func (rt *Runtime) enqueueDirty(inst *componentInstance) {
	rt.dirtyMu.Lock()
	rt.dirty = append(rt.dirty, inst)
	rt.dirtyMu.Unlock()
	rt.hasDirty.Store(true)
	rt.requestFrame()
}

// SetViewport sets the window size in logical pixels and the device scale.
func (rt *Runtime) SetViewport(w, h, scale float32) {
	if w == rt.devW && h == rt.devH && scale == rt.devScale {
		return
	}
	rt.devW, rt.devH, rt.devScale = w, h, scale
	rt.applyZoom()
}

func (rt *Runtime) applyZoom() {
	rt.viewW, rt.viewH = rt.devW/rt.zoom, rt.devH/rt.zoom
	rt.scale = rt.devScale * rt.zoom
	rt.needLayout = true
	// Overlays are sized against the window, so a resize can change them
	// even when nothing around them changed.
	for _, n := range rt.byID {
		if n.div != nil && n.div.overlay != 0 {
			rt.invalidateLayout(n)
		}
	}
}

// ZoomLevels are the steps used by ZoomIn, ZoomOut and the shortcuts.
var ZoomLevels = []float32{0.5, 0.67, 0.75, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2, 2.5, 3}

// Zoom returns the UI scale factor (1 = 100%).
func (rt *Runtime) Zoom() float32 { return rt.zoom }

// SetZoom scales the whole UI, clamped to 50%–300%. Text and shapes are
// re-rendered at the new size, so they stay sharp.
func (rt *Runtime) SetZoom(z float32) {
	z = min(max(z, ZoomLevels[0]), ZoomLevels[len(ZoomLevels)-1])
	if z == rt.zoom {
		return
	}
	rt.zoom = z
	rt.applyZoom()
	rt.InvalidateAll() // components that show the zoom level re-render
	if rt.onZoom != nil {
		rt.onZoom(z)
	}
	rt.requestFrame()
}

// ZoomIn and ZoomOut move to the next zoom level; ResetZoom returns to 100%.
func (rt *Runtime) ZoomIn()    { rt.zoomStep(+1) }
func (rt *Runtime) ZoomOut()   { rt.zoomStep(-1) }
func (rt *Runtime) ResetZoom() { rt.SetZoom(1) }

func (rt *Runtime) zoomStep(dir int) {
	for i, z := range ZoomLevels {
		if dir > 0 && z > rt.zoom+0.001 {
			rt.SetZoom(z)
			return
		}
		if dir < 0 && z >= rt.zoom-0.001 && i > 0 {
			rt.SetZoom(ZoomLevels[i-1])
			return
		}
	}
}

// SetZoomShortcuts enables (default) or disables the built-in Ctrl+= /
// Ctrl+- / Ctrl+0 keys and Ctrl+wheel zooming.
func (rt *Runtime) SetZoomShortcuts(on bool) { rt.zoomKeys = on }

// OnZoomChange registers fn to be called after the zoom changes, so an app
// can remember it.
func (rt *Runtime) OnZoomChange(fn func(float32)) { rt.onZoom = fn }

// Invalidate forces the next Frame to repaint even if nothing changed.
func (rt *Runtime) Invalidate() {
	rt.needPaint = true
	rt.requestFrame()
}

// InvalidateAll re-renders every component on the next frame. Used for
// benchmarking the worst case; applications should rely on signals instead.
func (rt *Runtime) InvalidateAll() {
	for _, inst := range rt.instances {
		inst.invalidate()
	}
}

// SetTheme switches the theme and re-renders all components.
func (rt *Runtime) SetTheme(t *Theme) {
	rt.appTheme = t
	rt.applyTheme()
	rt.InvalidateAll()
}

// AppTheme returns the theme the app set with SetTheme. Under high contrast the
// active theme is the high contrast one, so this is the theme to show as chosen.
func (rt *Runtime) AppTheme() *Theme { return rt.appTheme }

// Theme returns the active theme.
func (rt *Runtime) Theme() *Theme { return rt.theme }

// Text returns the runtime's text system.
func (rt *Runtime) Text() *text.System { return rt.text }

// Cursor returns the cursor for the element under the pointer.
func (rt *Runtime) Cursor() input.Cursor { return rt.cursor }

// Stats returns statistics about the last frame.
func (rt *Runtime) Stats() FrameStats { return rt.stats }

// NeedsFrame reports whether something changed since the last Frame.
func (rt *Runtime) NeedsFrame() bool {
	return rt.looping || !rt.Settled()
}

// Settled reports whether the UI has come to rest: nothing changed since the
// last Frame and nothing animates toward an end. Repeating animations (see
// Animation.Repeat) never end, so they do not count; NeedsFrame still reports
// them.
func (rt *Runtime) Settled() bool {
	return !(rt.rootInst == nil || rt.animating || rt.needLayout || rt.needPaint || rt.hasDirty.Load())
}

// Frame brings the scene up to date and returns it. Each stage does only the
// work its inputs require: dirty components re-render and rebuild just their
// subtrees, layout revisits only invalidated or resized subtrees, and an idle
// frame returns the retained scene untouched.
func (rt *Runtime) Frame(now time.Time) *scene.Scene {
	rt.framePending.Store(false)
	continuing := rt.animating || rt.looping // the last frame's paint or animations asked for another
	rt.animating, rt.looping = false, false
	dt := float32(0.016)
	if !rt.lastFrame.IsZero() {
		dt = min(float32(now.Sub(rt.lastFrame).Seconds()), 0.1)
	}
	rt.lastFrame = now
	rt.stats = FrameStats{Restyled: rt.restyled}
	rt.restyled = 0

	rt.runPosted()
	rt.expirePending(now)
	if rt.tipCandidate != 0 && rt.tipShown != rt.tipCandidate && rt.tipCandidate != rt.tipDismissed && !now.Before(rt.tipAt) {
		rt.showTip(rt.tipCandidate)
	}
	if rt.menuFn != nil && !now.Before(rt.menuAt) {
		fn := rt.menuFn
		rt.menuFn = nil
		fn()
	}
	// Animations that were running re-render now, once per frame. They are not
	// invalidated while rendering, which would loop within this frame.
	for _, inst := range rt.animInsts {
		inst.invalidate()
	}
	rt.animInsts = nil
	if rt.rootInst == nil || rt.hasDirty.Load() {
		rt.buildDirty()
	}
	// Focus requests resolve now, against the freshly built tree. A "nothing
	// focused" handle (id 0) clears focus.
	for _, h := range rt.pendingHandles {
		if h.pending && h.rt == rt && (h.none || rt.byID[h.id] != nil) {
			h.pending = false
			rt.setFocus(h.id)
		}
	}
	rt.pendingHandles = slices.DeleteFunc(rt.pendingHandles, func(h *FocusHandle) bool { return !h.pending })
	relayout := rt.needLayout
	if rt.needLayout {
		rt.layout()
	}
	rt.layoutPreview()
	// A drag near a scroll container's edge scrolls it; the target under the
	// pointer is found again once the content has moved.
	dragScrolled := rt.autoscrollDrag(dt)
	if rt.needPaint || continuing || rt.animating || rt.looping {
		rt.paintFrame(dt)
	}
	if dragScrolled {
		rt.dropOver()
	}
	// A UniformList whose visible rows are missing (or whose row height was
	// just measured) rebuilds now, so no frame is returned with gaps.
	for i := 0; i < 2 && rt.listStale; i++ {
		rt.listStale = false
		if rt.hasDirty.Load() {
			rt.buildDirty()
		}
		if rt.needLayout {
			rt.layout()
		}
		rt.paintFrame(0)
	}
	if relayout || rt.scrolled {
		// Content may have moved under a stationary pointer.
		rt.scrolled = false
		rt.updateHover()
	}
	rt.publishAccess()
	rt.stats.Nodes = len(rt.byID)
	rt.stats.Quads, rt.stats.Glyphs = len(rt.scene.Quads), len(rt.scene.Glyphs)
	rt.text.EndFrame()
	return &rt.scene
}

// buildDirty re-renders the dirty components and accumulates the time spent.
func (rt *Runtime) buildDirty() {
	t0 := time.Now()
	rt.rebuildDirty()
	rt.stats.Built = true
	rt.stats.BuildTime += time.Since(t0)
}

// layout computes the layout of the whole tree.
func (rt *Runtime) layout() {
	t0 := time.Now()
	rt.markAccess()
	rt.engine.Compute(&rt.tree.lay, rt.viewW, rt.viewH)
	rt.needLayout, rt.needPaint = false, true
	rt.stats.LaidOut = true
	rt.stats.LayoutTime += time.Since(t0)
	rt.stats.NodesLaidOut += rt.engine.Arranged
	rt.stats.Measured += rt.engine.Measured
}

// paintFrame paints the scene and repaints once if a text field's caret
// asked for its scroll container to move.
func (rt *Runtime) paintFrame(dt float32) {
	t0 := time.Now()
	rt.markAccess() // paint moves nodes (scrolling) and records which are drawn
	rt.hasTextInput = false
	rt.paint(dt)
	if rt.revealCaret() {
		rt.paint(0) // the caret's scroll container moved; repaint at the new offset
	}
	rt.needPaint = false
	rt.stats.Painted = true
	rt.stats.PaintTime += time.Since(t0)
}

// ---- identity ----

func mix(a, b uint64) uint64 {
	z := a ^ (b + 0x9e3779b97f4a7c15 + (a << 6) + (a >> 2))
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func hashString(s string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// typeTag distinguishes element types so that an element replaced by one of
// a different type at the same position starts with fresh state.
func typeTag(e Element) uint64 {
	switch e.(type) {
	case *DivElement:
		return 1
	case *TextElement:
		return 2
	case *IconElement:
		return 3
	case *ImageElement:
		return 4
	case *ComponentElement:
		return 5
	case *ButtonElement:
		return 6
	case *TextInputElement:
		return 7
	case *TextAreaElement:
		return 8
	case *editorElement:
		return 9
	case *UniformListElement:
		return 11
	case *AnimateElement:
		return 12
	case *ListElement:
		return 13
	}
	return 10
}

// childID derives a child's identity from its parent, its type and either
// its key or (when unkeyed) its position among its siblings.
func childID(parent uint64, index int, e Element) uint64 {
	if k, ok := e.(keyed); ok {
		if key := k.elementKey(); key != "" {
			return mix(mix(parent, hashString(key)|1<<63), typeTag(e))
		}
	}
	return mix(mix(parent, uint64(index)+1), typeTag(e))
}

// expandedID is the ID of the element produced by a component or expander.
func expandedID(id uint64) uint64 { return mix(id, 0x5eed) }

const rootID = 0x600d

func (rt *Runtime) state(id uint64) *nodeState {
	st := rt.states[id]
	if st == nil {
		st = &nodeState{}
		rt.states[id] = st
	}
	st.gen = rt.gen
	return st
}

// claimed reports whether id was already used in this frame's build (which
// means two siblings share a key).
func (rt *Runtime) claimed(id uint64) bool {
	if n := rt.byID[id]; n != nil && n.gen == rt.gen {
		return true
	}
	if inst := rt.instances[id]; inst != nil && inst.gen == rt.gen {
		return true
	}
	return false
}

// ---- build ----

// rebuildDirty re-renders dirty instances, outermost first, and splices
// their new subtrees into the retained tree.
func (rt *Runtime) rebuildDirty() {
	rt.gen++
	rt.markAccess()
	rt.hasDirty.Store(false)
	rt.dirtyMu.Lock()
	queue := rt.dirty
	rt.dirty = nil
	rt.dirtyMu.Unlock()

	if rt.rootInst == nil {
		rt.rootInst = rt.instance(rootID, rt.rootEl, nil, nil, rt.rootTextStyle())
		rt.setTree(rt.rootInst.root)
		rt.needLayout = true
		return
	}
	// Outermost first: re-rendering a parent re-renders its children anyway,
	// which clears their dirty flags so they are skipped below.
	sort.SliceStable(queue, func(i, j int) bool { return queue[i].depth < queue[j].depth })
	for _, inst := range queue {
		if inst.disposed || !inst.dirty.Load() {
			continue
		}
		rt.rerender(inst)
	}
	if rt.focusID != 0 && rt.byID[rt.focusID] == nil {
		rt.focusID = 0
	}
	if id := rt.pendingFocus; id != 0 {
		rt.pendingFocus = 0
		if rt.byID[id] != nil {
			rt.setFocus(id)
		}
	}
	if id := rt.pendingModal; id != 0 {
		rt.pendingModal = 0
		if n := rt.byID[id]; n != nil {
			if f := rt.modalFocus(n); f != 0 {
				rt.setFocus(f)
			}
		}
	}
}

func (rt *Runtime) rootTextStyle() textStyle {
	return textStyle{size: rt.theme.FontSize, weight: FontRegular}
}

func (rt *Runtime) setTree(n *node) { rt.tree = n }

// emptyDiv realizes components whose output is nil.
var emptyDiv = Div().Absolute()

// rerender re-runs one instance and replaces its subtree in place.
func (rt *Runtime) rerender(inst *componentInstance) {
	if inst == rt.rootInst {
		inst.inh = rt.rootTextStyle()
	}
	old := inst.root
	rt.renderInstance(inst)
	n := inst.root
	if inst == rt.drag.preview {
		// The drag preview hangs off no node, so it is laid out on its own.
		rt.drag.previewStale = true
		return
	}
	if p := inst.parentNode; p == nil {
		rt.setTree(n)
	} else {
		// Instances always realize to exactly one node, so the new subtree
		// takes the old one's slot without touching any sibling.
		i := slices.Index(p.children, old)
		p.children[i] = n
		p.lay.Children[i] = &n.lay
		rt.reorder(p)
		rt.invalidateLayout(p)
	}
	rt.needLayout = true
}

// renderInstance runs inst and rebuilds its subtree from the output,
// disposing whatever the previous output had that the new one does not.
func (rt *Runtime) renderInstance(inst *componentInstance) {
	oldRoot := inst.root
	oldChildren := inst.children
	inst.children = nil
	inst.render()
	id := expandedID(inst.id)
	inst.root = rt.buildNode(inst.out, inst.parentNode, id, inst.depth, inst.inh, inst)
	if inst.root == nil {
		// A component that renders nothing still owns a slot: an inert,
		// absolutely positioned, zero-size node that takes no part in flex
		// layout, gaps, painting or hit testing.
		inst.root = rt.newNode(kindDiv, id, inst.parentNode, inst)
		inst.root.div = emptyDiv
		inst.root.style = emptyDiv.style
		inst.root.lay.Style = emptyDiv.style.layoutStyle(0)
	}
	inst.root.boundary = inst
	if k, ok := inst.el.(keyed); ok && inst.root.key == "" {
		inst.root.key = k.elementKey() // keep answering to the component's key after re-renders
	}
	for _, c := range oldChildren {
		if c.gen != rt.gen || c.parent != inst {
			rt.disposeInstance(c)
		}
	}
	rt.retire(oldRoot)
}

// retire forgets nodes of a replaced subtree. Subtrees of instances that
// were kept (and re-attached to the new tree) are skipped.
// An element with an Exit that really leaves is not forgotten: it becomes a
// ghost, drawn until its exit ends. Its subtree is forgotten with it.
// listenOutside registers n's OnClickOutside. Listeners are kept here rather
// than collected by paint, so one that is off screen still hears clicks. A
// rebuilt element replaces the entry of the node it replaces.
func (rt *Runtime) listenOutside(n *node) {
	for i, o := range rt.outside {
		if o.id == n.id {
			rt.outside[i] = n
			return
		}
	}
	rt.outside = append(rt.outside, n)
}

func (rt *Runtime) retire(n *node) { rt.retireNode(n, true) }

func (rt *Runtime) retireNode(n *node, mayExit bool) {
	if n == nil {
		return
	}
	if b := n.boundary; b != nil && b.root == n && !b.disposed && b.gen == rt.gen && n.gen != rt.gen {
		return // a reused instance's subtree, now part of the new tree
	}
	leaving := rt.byID[n.id] == n
	if leaving {
		delete(rt.byID, n.id)
		if st := rt.states[n.id]; st != nil && st.gen != rt.gen {
			delete(rt.states, n.id)
		}
	}
	if leaving && n.flags&flagOutside != 0 {
		rt.outside = slices.DeleteFunc(rt.outside, func(o *node) bool { return o.id == n.id })
	}
	if leaving && rt.ctxMenu != nil && rt.ctxMenu.id == n.id {
		rt.closeContextMenu(false) // the element it hung from has gone
	}
	if leaving && n.div != nil && n.div.modal != nil {
		rt.modalClosed(n)
	}
	// Only an element that was drawn in the last paint has something to fade
	// out; one that was off screen (culled) simply goes.
	exits := mayExit && leaving && n.div != nil && n.div.exit != nil && n.paintGen == rt.paintGen && !rt.sys.ReducedMotion
	if exits {
		n.st.leaveAt = rt.now()
		rt.ghosts = append(rt.ghosts, n)
	}
	for _, c := range n.children {
		rt.retireNode(c, !exits)
	}
}

func (rt *Runtime) disposeInstance(inst *componentInstance) {
	if inst.disposed {
		return
	}
	inst.disposed = true
	inst.unsubscribe()
	if rt.instances[inst.id] == inst {
		delete(rt.instances, inst.id)
		delete(rt.states, inst.id)
	}
	for _, c := range inst.children {
		rt.disposeInstance(c)
	}
}

func (rt *Runtime) newNode(kind nodeKind, id uint64, parent *node, owner *componentInstance) *node {
	n := &node{rt: rt, kind: kind, id: id, parent: parent, owner: owner, gen: rt.gen}
	n.fresh = rt.byID[id] == nil // a new element, not a rebuild of an existing one
	if n.fresh {
		rt.dropGhost(id)
	}
	rt.byID[id] = n
	rt.stats.NodesBuilt++
	return n
}

// instance claims (creating if needed) the instance for a component or
// composite element, rendering it if it is new, dirty or was given a new
// element by its re-rendered parent. Otherwise its subtree is reused.
func (rt *Runtime) instance(id uint64, el Element, owner *componentInstance, parent *node, inh textStyle) *componentInstance {
	inst := rt.instances[id]
	if inst == nil || inst.disposed {
		inst = &componentInstance{rt: rt, id: id}
		inst.dirty.Store(true)
		rt.instances[id] = inst
	}
	inst.gen = rt.gen
	inst.parent = owner
	inst.depth = 0
	if owner != nil {
		inst.depth = owner.depth + 1
		owner.children = append(owner.children, inst)
	}
	inst.parentNode = parent
	if inst.el != el {
		inst.el = el
		inst.dirty.Store(true)
	}
	if inst.dirty.Load() {
		inst.inh = inh
		rt.renderInstance(inst)
		return inst
	}
	// Reuse: re-attach the retained subtree and pass down inherited text
	// style if it changed.
	inst.root.parent = parent
	if inst.inh != inh {
		rt.applyInherited(inst.root, inh)
	}
	inst.inh = inh
	return inst
}

func (rt *Runtime) buildNode(el Element, parent *node, id uint64, depth int, inh textStyle, owner *componentInstance) *node {
	n := rt.buildNodeKind(el, parent, id, depth, inh, owner)
	if n != nil && n.key == "" {
		// The output of a keyed component or field answers to its key.
		if k, ok := el.(keyed); ok {
			n.key = k.elementKey()
		}
	}
	return n
}

func (rt *Runtime) buildNodeKind(el Element, parent *node, id uint64, depth int, inh textStyle, owner *componentInstance) *node {
	if depth > 256 {
		panic("ui: element tree is deeper than 256 levels; a component probably renders itself recursively")
	}
	switch e := el.(type) {
	case nil:
		return nil
	case *ComponentElement:
		if e == nil {
			return nil
		}
		return rt.instance(id, e, owner, parent, inh).root
	case expander:
		return rt.instance(id, e, owner, parent, inh).root
	case *DivElement:
		if e == nil {
			return nil
		}
		n := rt.newNode(kindDiv, id, parent, owner)
		n.div = e
		if e.overlay != 0 {
			n.flags |= flagOverlay
		}
		if e.h.onClickOutside != nil {
			n.flags |= flagOutside
			rt.listenOutside(n)
		}
		if e.vlist != nil || e.vrows != nil {
			n.flags |= flagList
		}
		if e.exit != nil {
			n.flags |= flagExit
		}
		if e.dragSpec != nil {
			n.flags |= flagDrag
		}
		if len(e.drops) > 0 {
			n.flags |= flagDrop
		}
		rt.attachFocus(n)
		n.style = rt.resolveStyle(n)
		n.lay.Style = n.style.layoutStyle(1)
		if e.overlay != 0 {
			n.lay.Style.Position = layout.Floating
		}
		n.ts = inh.inherit(&n.style)
		if e.hover != nil || e.pressed != nil || e.focused != nil || e.focusWithin != nil || n.style.Transition > 0 ||
			n.style.OverflowX == OverflowScroll || n.style.OverflowY == OverflowScroll || e.enter != nil || e.exit != nil {
			n.st = rt.state(id)
			if n.fresh {
				// A new element forgets any exit its ID had, and enters now.
				n.st.leaveAt, n.st.enterAt = time.Time{}, time.Time{}
				if e.enter != nil {
					n.st.enterAt = rt.now()
				}
			}
		}
		rt.setOverhang(n)
		for i, c := range e.children {
			cid := childID(id, i, c)
			if rt.claimed(cid) {
				cid = rt.duplicateKey(cid, c)
			}
			cn := rt.buildNode(c, n, cid, depth+1, n.ts, owner)
			if cn == nil {
				continue
			}
			n.children = append(n.children, cn)
			n.lay.Children = append(n.lay.Children, &cn.lay)
		}
		if e.tooltip != "" && rt.tipShown == id {
			c := rt.tooltipElement(e.tooltip)
			cid := childID(id, len(e.children), c)
			if rt.claimed(cid) {
				cid = rt.duplicateKey(cid, c)
			}
			if cn := rt.buildNode(c, n, cid, depth+1, n.ts, owner); cn != nil {
				n.children = append(n.children, cn)
				n.lay.Children = append(n.lay.Children, &cn.lay)
			}
		}
		if e.contextMenu != nil && rt.ctxMenu != nil && rt.ctxMenu.id == id {
			c := contextMenuAt(rt, rt.ctxMenu)
			cid := childID(id, len(e.children), c)
			if rt.claimed(cid) {
				cid = rt.duplicateKey(cid, c)
			}
			if cn := rt.buildNode(c, n, cid, depth+1, n.ts, owner); cn != nil {
				n.children = append(n.children, cn)
				n.lay.Children = append(n.lay.Children, &cn.lay)
			}
		}
		rt.reorder(n)
		return n
	case *editorElement:
		n := rt.newNode(kindEditor, id, parent, owner)
		rt.buildEditor(e, n, inh)
		rt.attachFocus(n)
		return n
	case *TextElement:
		n := rt.newNode(kindText, id, parent, owner)
		n.textEl = e
		n.style = e.style
		n.ts = inh.inherit(&e.style)
		if e.isRich() {
			n.rich = buildRich(e)
			if n.rich.interactive {
				n.flags |= flagRich
			}
		}
		n.lay.Style = e.style.layoutStyle(1)
		n.lay.Measure = n
		rt.setOverhang(n)
		return n
	case *IconElement:
		n := rt.newNode(kindIcon, id, parent, owner)
		n.icon = e
		n.style = e.style
		n.lay.Style = e.style.layoutStyle(0)
		n.lay.Measure = n
		rt.setOverhang(n)
		return n
	case *ImageElement:
		n := rt.newNode(kindImage, id, parent, owner)
		n.img = e
		n.style = e.style
		n.lay.Style = e.style.layoutStyle(1)
		n.lay.Measure = n
		rt.setOverhang(n)
		return n
	}
	panic(fmt.Sprintf("ui: unsupported element type %T; build elements with ui constructors such as ui.Div, ui.Text or ui.Component", el))
}

// attachFocus connects a focus handle to its element and applies AutoFocus
// to elements that just appeared.
func (rt *Runtime) attachFocus(n *node) {
	d := n.div
	if d == nil {
		return
	}
	if h := d.handle; h != nil {
		h.rt, h.id = rt, n.id
		if h.pending && !slices.Contains(rt.pendingHandles, h) {
			rt.pendingHandles = append(rt.pendingHandles, h)
		}
	}
	if d.autoFocus && n.fresh {
		rt.pendingFocus = n.id
	}
	if d.modal != nil && n.fresh {
		rt.modalMounted(n)
	}
}

// duplicateKey reports two siblings sharing a key and gives the later one a
// distinct identity, so state is never shared between them.
func (rt *Runtime) duplicateKey(id uint64, e Element) uint64 {
	key := ""
	if k, ok := e.(keyed); ok {
		key = k.elementKey()
	}
	if rt.warn != nil {
		rt.warn(fmt.Sprintf("duplicate key %q among siblings; keys must be unique within a parent, "+
			"otherwise hover, focus and scroll state can attach to the wrong element. "+
			"Derive keys from a unique field such as an ID.", key))
	}
	for i := uint64(1); ; i++ {
		alt := mix(id, i)
		if !rt.claimed(alt) {
			return alt
		}
	}
}

// reorder rebuilds a node's paint order (children sorted by ZIndex).
func (rt *Runtime) reorder(n *node) {
	n.order = append(n.order[:0], n.children...)
	hasZ := false
	for _, c := range n.children {
		hasZ = hasZ || c.style.ZIndex != 0
	}
	if hasZ {
		sort.SliceStable(n.order, func(a, b int) bool { return n.order[a].style.ZIndex < n.order[b].style.ZIndex })
	}
}

// resolveStyle merges a div's hover/pressed/focused styles for its current state.
func (rt *Runtime) resolveStyle(n *node) Style {
	e := n.div
	s := e.style
	if e.hover != nil && rt.isHovered(n.id) {
		s = s.Merge(*e.hover)
	}
	if e.pressed != nil && rt.isPressed(n.id) {
		s = s.Merge(*e.pressed)
	}
	if e.focused != nil && rt.focusID == n.id {
		s = s.Merge(*e.focused)
	}
	if e.focusVisible != nil && rt.focusID == n.id && rt.keyboardModality {
		s = s.Merge(*e.focusVisible)
	}
	if e.focusWithin != nil && rt.focusInside(n.id) {
		s = s.Merge(*e.focusWithin)
	}
	if rt.drag.active {
		if e.dragging != nil && rt.drag.source == n.id {
			s = s.Merge(*e.dragging)
		}
		if rt.drag.targetSpec != nil && rt.drag.target == n.id {
			if over := rt.drag.targetSpec.overStyle(); over != nil {
				s = s.Merge(*over)
			}
		}
	}
	return s
}

// focusInside reports whether the focused element is id or a descendant.
func (rt *Runtime) focusInside(id uint64) bool {
	for f := rt.byID[rt.focusID]; f != nil; f = f.parent {
		if f.id == id {
			return true
		}
	}
	return false
}

// glyphOverhang is how far text and icons may draw past their boxes: glyph
// overshoot and subpixel antialiasing.
const glyphOverhang = 2

// inkOverhang returns how far a node's drawing can reach beyond its box on
// each side, and how far it is scaled up beyond 1, from its style and its
// entrance and exit motions. Offsets and motion slides move the drawing, so
// they add to the reach. The values bound what paint draws, which is what
// culling relies on.
func inkOverhang(s *Style, enter, exit *motionSpec) (over, grow float32) {
	for _, sh := range s.Shadows {
		// A blur of sigma Blur/2 is drawn out to about four sigmas.
		reach := max(abs32(sh.X), abs32(sh.Y)) + 2*sh.Blur + max(sh.Spread, 0)
		over = max(over, reach)
	}
	over += abs32(s.OffsetX) + abs32(s.OffsetY)
	grow = max(s.Scale-1, 0)
	for _, sp := range [2]*motionSpec{enter, exit} {
		if sp == nil {
			continue
		}
		over += abs32(sp.m.x) + abs32(sp.m.y)
		grow = max(grow, sp.m.scale-1)
	}
	return over, grow
}

// setOverhang sets n's ink overhang from its style and motions and reports
// whether it changed. An element with a Transition keeps the largest value it
// has had, since its displayed offset and scale ease from the old target.
func (rt *Runtime) setOverhang(n *node) bool {
	var enter, exit *motionSpec
	if n.div != nil {
		enter, exit = n.div.enter, n.div.exit
	}
	over, grow := inkOverhang(&n.style, enter, exit)
	if n.kind == kindText || n.kind == kindIcon {
		over += glyphOverhang
	}
	if n.st != nil && n.style.Transition > 0 {
		over, grow = max(over, n.st.overhang), max(grow, n.st.growOverhang)
		n.st.overhang, n.st.growOverhang = over, grow
	}
	if over == n.lay.Overhang && grow == n.lay.GrowOverhang {
		return false
	}
	n.lay.Overhang, n.lay.GrowOverhang = over, grow
	return true
}

// restyle re-resolves one node's style after an interaction state change,
// without re-rendering anything. Layout is invalidated only if the new
// style affects layout.
func (rt *Runtime) restyle(id uint64) {
	n := rt.byID[id]
	if n == nil || n.div == nil {
		return
	}
	d := n.div
	if d.hover == nil && d.pressed == nil && d.focused == nil && d.focusWithin == nil && d.focusVisible == nil &&
		d.dragging == nil && len(d.drops) == 0 {
		return
	}
	s := rt.resolveStyle(n)
	n.style = s
	rt.restyled++
	rt.needPaint = true
	ls := s.layoutStyle(1)
	if d.overlay != 0 {
		ls.Position = layout.Floating
	}
	if ls != n.lay.Style {
		n.lay.Style = ls
		rt.invalidateLayout(n)
	}
	// A hover can add a shadow or an offset, which changes what the node draws
	// beyond its box, so its ink is recomputed.
	if rt.setOverhang(n) {
		rt.invalidateLayout(n)
	}
	inh := rt.rootTextStyle()
	if n.parent != nil {
		inh = n.parent.ts
	}
	rt.applyInherited(n, inh)
	rt.requestFrame()
}

// applyInherited updates inherited text style below n, invalidating layout
// of text whose font changed. It stops where nothing changes.
func (rt *Runtime) applyInherited(n *node, inh textStyle) {
	ts := inh.inherit(&n.style)
	if ts == n.ts {
		return
	}
	n.ts = ts
	if n.kind == kindText {
		rt.invalidateLayout(n)
	}
	for _, c := range n.children {
		rt.applyInherited(c, ts)
	}
}

// invalidateLayout marks n and its ancestors for re-layout.
func (rt *Runtime) invalidateLayout(n *node) {
	for ; n != nil; n = n.parent {
		n.lay.Invalidate()
	}
	rt.needLayout = true
}

func contains(ids []uint64, id uint64) bool {
	return slices.Contains(ids, id)
}

func (rt *Runtime) isHovered(id uint64) bool { return contains(rt.hoverIDs, id) }
func (rt *Runtime) isPressed(id uint64) bool { return contains(rt.pressIDs, id) }
