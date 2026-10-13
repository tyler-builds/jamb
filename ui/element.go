package ui

import (
	"fmt"
	"time"
)

// Element is anything that can appear in the UI tree: *DivElement,
// *TextElement, *IconElement, *ImageElement, *ButtonElement,
// *TextInputElement, *ComponentElement, *UniformListElement and *ListElement. A nil Element
// renders nothing.
type Element interface {
	isElement()
}

// expander is implemented by composite elements that turn into a tree of
// simpler elements when built (buttons, inputs, ...).
type expander interface {
	Element
	expand(ctx *Context) Element
}

// keyed elements carry an explicit identity among their siblings.
type keyed interface {
	elementKey() string
}

// handlers holds event callbacks. All run on the UI thread.
type handlers struct {
	onClick        func()
	onDoubleClick  func()
	onMouseDown    func(*MouseEvent)
	onMouseUp      func(*MouseEvent)
	onMouseMove    func(*MouseEvent)
	onMouseEnter   func()
	onMouseLeave   func()
	onScroll       func(*ScrollEvent)
	onKeyDown      func(*KeyEvent)
	onKeyUp        func(*KeyEvent)
	onTextInput    func(string)
	onFocus        func()
	onBlur         func()
	onClickOutside func()
}

// DivElement is the general-purpose container: a flex box with optional
// background, border, radius, shadow, clipping, scrolling and event handlers.
// Create one with Div and configure it with chained methods.
type DivElement struct {
	style        Style
	hover        *Style
	pressed      *Style
	focused      *Style
	focusVisible *Style
	focusWithin  *Style
	children     []Element
	key          string
	focusable    bool
	tabIndex     int  // see TabIndex
	noTabStop    bool // see TabStop
	tabSkip      bool // the focusable elements inside are left out of the Tab order (see DataGrid)
	noDisplay    bool // see Display
	spaceOnly    bool // Space runs OnClick but Enter does not (check boxes, switches, radio buttons)
	capture      bool
	autoFocus    bool
	handle       *FocusHandle
	overlay      Placement
	overlayGap   float32
	revealRow    bool // a combo box's highlighted row: the next paint scrolls it into view (see revealRowsNow)
	matchAnchor  bool // set by a Select's list: at least as wide as the field it opens from
	tooltip      string
	part         string          // the widget part this element draws, see asPart
	keyContext   string          // the KeyContext spec, as given
	keyCtx       keyCtx          // its parsed entries, for BindIn bindings
	actions      []actionHandler // see OnAction
	vlist        *listState      // set on the scroll container of a UniformList
	vrows        *varState       // set on the scroll container of a ListOf
	h            handlers
	enter        *motionSpec       // see Enter
	exit         *motionSpec       // see Exit
	dragSpec     *DragSpec         // see OnDrag
	dragging     *Style            // see Dragging
	drops        []DropTarget      // see OnDrop
	area         WindowArea        // see WindowArea
	contextMenu  func() []MenuItem // see ContextMenu
	menuBar      bool              // set by MenuBar on the bar
	modal        *modalSpec        // set on the backdrop of a Modal (see modal.go)
	modalFooter  bool              // set on a Modal's footer, which the initial focus skips
	acc          *accessInfo       // accessibility role, name and states (see access.go)
}

// Div creates an empty container. Children stack vertically (Column) unless
// Row is called.
func Div() *DivElement { return &DivElement{} }

func (*DivElement) isElement()           {}
func (d *DivElement) elementKey() string { return d.key }

// Key gives the element a stable identity among its siblings, so its state
// (hover, scroll position, focus, animations) survives reordering.
func (d *DivElement) Key(k string) *DivElement { d.key = k; return d }

// Keyf is Key with fmt.Sprintf formatting.
func (d *DivElement) Keyf(format string, args ...any) *DivElement {
	d.key = fmt.Sprintf(format, args...)
	return d
}

// Child appends one child. nil is ignored.
func (d *DivElement) Child(e Element) *DivElement {
	if !isNil(e) {
		d.children = append(d.children, e)
	}
	return d
}

// Children appends children. nil entries are ignored.
func (d *DivElement) Children(es ...Element) *DivElement {
	for _, e := range es {
		d.Child(e)
	}
	return d
}

// Style merges s into the element's style (set fields win).
func (d *DivElement) Style(s Style) *DivElement { d.style = d.style.Merge(s); return d }

// Hover merges s on top while the pointer is over the element.
func (d *DivElement) Hover(s Style) *DivElement { d.hover = mergeInto(d.hover, s); return d }

// Pressed merges s on top while the element is being pressed.
func (d *DivElement) Pressed(s Style) *DivElement { d.pressed = mergeInto(d.pressed, s); return d }

// Focused merges s on top while the element has keyboard focus. It implies Focusable.
func (d *DivElement) Focused(s Style) *DivElement {
	d.focused = mergeInto(d.focused, s)
	d.focusable = true
	return d
}

// FocusVisible merges s on top while the element has focus and focus is
// keyboard-driven: it arrived by Tab or a key press and no click has happened
// since. It is the focus ring to use for controls; Focused applies even after
// a click. It implies Focusable.
func (d *DivElement) FocusVisible(s Style) *DivElement {
	d.focusVisible = mergeInto(d.focusVisible, s)
	d.focusable = true
	return d
}

// FocusWithin merges s on top while the element or any descendant has
// keyboard focus — for example to highlight a field's frame while the text
// inside it is being edited.
func (d *DivElement) FocusWithin(s Style) *DivElement {
	d.focusWithin = mergeInto(d.focusWithin, s)
	return d
}

func mergeInto(dst *Style, s Style) *Style {
	if dst == nil {
		return &s
	}
	m := dst.Merge(s)
	return &m
}

// When applies fn only if cond is true; handy for conditional styling:
//
//	ui.Div().When(selected, func(d *ui.DivElement) { d.Background(accent) })
func (d *DivElement) When(cond bool, fn func(d *DivElement)) *DivElement {
	if cond {
		fn(d)
	}
	return d
}

// ---- layout ----

// Flex is accepted for readability; every Div is already a flex container.
func (d *DivElement) Flex() *DivElement { return d }

// Row lays children out horizontally.
func (d *DivElement) Row() *DivElement { d.style.Direction = Row; return d }

// Column lays children out vertically (the default).
func (d *DivElement) Column() *DivElement { d.style.Direction = Column; return d }

// Wrap lets children flow onto additional lines (rows of chips, tag
// clouds, galleries). Gap spaces the lines as well as the items.
func (d *DivElement) Wrap() *DivElement { d.style.Wrap = true; return d }

// Gap sets the space between children.
func (d *DivElement) Gap(px float32) *DivElement { d.style.Gap = px; return d }

// Padding sets padding on all sides.
func (d *DivElement) Padding(px float32) *DivElement { d.style.Padding = All(px); return d }

// PaddingX sets left and right padding.
func (d *DivElement) PaddingX(px float32) *DivElement {
	d.style.Padding.Left, d.style.Padding.Right = px, px
	return d
}

// PaddingY sets top and bottom padding.
func (d *DivElement) PaddingY(px float32) *DivElement {
	d.style.Padding.Top, d.style.Padding.Bottom = px, px
	return d
}

// PaddingEdges sets each side's padding.
func (d *DivElement) PaddingEdges(e Edges) *DivElement { d.style.Padding = e; return d }

// Margin sets margin on all sides.
func (d *DivElement) Margin(px float32) *DivElement { d.style.Margin = All(px); return d }

// MarginX sets left and right margin.
func (d *DivElement) MarginX(px float32) *DivElement {
	d.style.Margin.Left, d.style.Margin.Right = px, px
	return d
}

// MarginY sets top and bottom margin.
func (d *DivElement) MarginY(px float32) *DivElement {
	d.style.Margin.Top, d.style.Margin.Bottom = px, px
	return d
}

// MarginTop sets the top margin.
func (d *DivElement) MarginTop(px float32) *DivElement { d.style.Margin.Top = px; return d }

// MarginBottom sets the bottom margin.
func (d *DivElement) MarginBottom(px float32) *DivElement { d.style.Margin.Bottom = px; return d }

// Width sets the width (border box).
func (d *DivElement) Width(l Length) *DivElement { d.style.Width = l; return d }

// Height sets the height (border box).
func (d *DivElement) Height(l Length) *DivElement { d.style.Height = l; return d }

// Size sets width and height.
func (d *DivElement) Size(w, h Length) *DivElement {
	d.style.Width, d.style.Height = w, h
	return d
}

func (d *DivElement) MinWidth(l Length) *DivElement  { d.style.MinWidth = l; return d }
func (d *DivElement) MinHeight(l Length) *DivElement { d.style.MinHeight = l; return d }
func (d *DivElement) MaxWidth(l Length) *DivElement  { d.style.MaxWidth = l; return d }
func (d *DivElement) MaxHeight(l Length) *DivElement { d.style.MaxHeight = l; return d }

// Grow makes the element take a share of the free space on its parent's main axis.
func (d *DivElement) Grow(f float32) *DivElement { d.style.Grow = f; return d }

// Basis sets the main-axis size the element starts from before growing or
// shrinking. Basis(ui.Px(0)) with Grow(1) gives equal-width columns.
func (d *DivElement) Basis(l Length) *DivElement { d.style.Basis = l; return d }

// Shrink sets how much the element shrinks when space is short (default 1).
func (d *DivElement) Shrink(f float32) *DivElement {
	d.style.Shrink = f
	d.style.NoShrink = f == 0
	return d
}

// Align positions children on the cross axis: Start, Center, End, Stretch.
func (d *DivElement) Align(a Alignment) *DivElement { d.style.Align = a; return d }

// AlignSelf overrides the parent's Align for this element.
func (d *DivElement) AlignSelf(a Alignment) *DivElement { d.style.AlignSelf = a; return d }

// Justify distributes children on the main axis: Start, Center, End,
// SpaceBetween, SpaceAround, SpaceEvenly.
func (d *DivElement) Justify(a Alignment) *DivElement { d.style.Justify = a; return d }

// Center centers children on both axes.
func (d *DivElement) Center() *DivElement {
	d.style.Align, d.style.Justify = Center, Center
	return d
}

// Absolute takes the element out of the flow; position it with Top/Right/Bottom/Left.
func (d *DivElement) Absolute() *DivElement { d.style.Absolute = true; return d }

func (d *DivElement) inset(i int, px float32) *DivElement {
	*d.style.Inset.side(i) = px
	d.style.insetSet[i] = true
	return d
}

// Top, Right, Bottom and Left position an Absolute element relative to its parent's padding box.
func (d *DivElement) Top(px float32) *DivElement    { return d.inset(0, px) }
func (d *DivElement) Right(px float32) *DivElement  { return d.inset(1, px) }
func (d *DivElement) Bottom(px float32) *DivElement { return d.inset(2, px) }
func (d *DivElement) Left(px float32) *DivElement   { return d.inset(3, px) }

// Inset pins all four sides (Absolute + Top/Right/Bottom/Left).
func (d *DivElement) Inset(px float32) *DivElement {
	d.Absolute()
	return d.Top(px).Right(px).Bottom(px).Left(px)
}

// Overlay makes the element float: it is positioned next to its parent's
// border box at placement p (4 px away unless OverlayGap says otherwise),
// painted after the rest of the window, never clipped by ancestors and
// hit-tested before everything else. If it does not fit on the requested
// side it flips to the opposite side when that has more room, then shifts to
// stay at least 8 px inside the window.
func (d *DivElement) Overlay(p Placement) *DivElement {
	d.style.Absolute = true
	d.overlay, d.overlayGap = p, 4
	return d
}

// OverlayGap sets the distance between an Overlay and its parent.
func (d *DivElement) OverlayGap(px float32) *DivElement { d.overlayGap = px; return d }

// ZIndex paints the element above (higher) or below (lower) its siblings.
func (d *DivElement) ZIndex(z int) *DivElement { d.style.ZIndex = z; return d }

// ClipContent hides children that overflow the element.
func (d *DivElement) ClipContent() *DivElement {
	d.style.OverflowX, d.style.OverflowY = OverflowHidden, OverflowHidden
	return d
}

// ScrollY makes content scroll vertically (and clips it).
func (d *DivElement) ScrollY() *DivElement {
	d.style.OverflowY = OverflowScroll
	if d.style.OverflowX == OverflowVisible {
		d.style.OverflowX = OverflowHidden
	}
	return d
}

// ScrollX makes content scroll horizontally (and clips it).
func (d *DivElement) ScrollX() *DivElement {
	d.style.OverflowX = OverflowScroll
	if d.style.OverflowY == OverflowVisible {
		d.style.OverflowY = OverflowHidden
	}
	return d
}

// Display shows the element when on is true. Display(false) hides it as CSS
// display: none does, but keeps it in the tree: its component instances, Local
// state, signals, text and selection, scroll positions and list state survive,
// so Display(true) shows it again as it was. While hidden, the element and its
// subtree take no space in layout, are not painted, are not hit, are skipped by
// Tab and cannot take focus, are absent from the accessibility tree, and run no
// repeating animation or frame. A focused element that becomes hidden hands
// focus to its nearest focusable ancestor, or to nothing when there is none.
//
// AccessHidden is a different thing: it only leaves the element out of the
// accessibility tree, and the element is still drawn, laid out and reachable.
// Use Display for content that should not be seen or reached, and AccessHidden for
// content that is seen but not announced.
func (d *DivElement) Display(on bool) *DivElement { d.noDisplay = !on; return d }

// ---- visuals ----

// Background sets the fill color.
func (d *DivElement) Background(c Color) *DivElement { d.style.Background = c; return d }

// Border sets a uniform border.
func (d *DivElement) Border(width float32, c Color) *DivElement {
	d.style.BorderWidth, d.style.BorderEdges, d.style.BorderColor = width, Edges{}, c
	return d
}

// BorderEdges sets a border of color c with its own width on each side (top,
// right, bottom, left); a zero side has none. Each side takes up space like
// padding, and a rounded element's corners curve from one side into the next.
// Its color eases with Transition, its widths do not (they are layout).
//
//	ui.Div().BorderEdges(ui.Edges{Bottom: 2}, accent) // an underline
func (d *DivElement) BorderEdges(e Edges, c Color) *DivElement {
	d.style.BorderWidth, d.style.BorderEdges, d.style.BorderColor = 0, e, c
	return d
}

// Radius rounds all corners.
func (d *DivElement) Radius(r float32) *DivElement { d.style.Radius = r; return d }

// RadiusCorners rounds each corner separately.
func (d *DivElement) RadiusCorners(c Corners) *DivElement { d.style.Corners = c; return d }

// Shadow adds a drop shadow (call repeatedly to layer shadows).
func (d *DivElement) Shadow(s Shadow) *DivElement {
	d.style.Shadows = append(append([]Shadow(nil), d.style.Shadows...), s)
	return d
}

// Opacity fades the element and its children (0..1).
func (d *DivElement) Opacity(o float32) *DivElement {
	if o <= 0 {
		o = 0.0001
	}
	d.style.Opacity = o
	return d
}

// Cursor sets the mouse cursor shown over the element.
func (d *DivElement) Cursor(c Cursor) *DivElement { d.style.Cursor = c; return d }

// Transition eases changes of color, Opacity, Offset and Scale over the given
// duration. Paint-only properties ease without a re-layout.
func (d *DivElement) Transition(t time.Duration) *DivElement { d.style.Transition = t; return d }

// Offset moves the element by (x, y) logical pixels after layout. Its place in
// the layout does not change, so siblings stay put; hit testing and Find follow
// the element. Combine with Transition to slide it.
func (d *DivElement) Offset(x, y float32) *DivElement {
	d.style.OffsetX, d.style.OffsetY = x, y
	return d
}

// Scale draws the element and its children s times their size about the
// element's center (1 = normal). Hit testing ignores scale, so a scaled
// element is hit where it is laid out. Overlays inside a scaled element are
// placed by their unscaled anchor.
func (d *DivElement) Scale(s float32) *DivElement { d.style.Scale = s; return d }

// TextColor sets the inherited text and icon color.
func (d *DivElement) TextColor(c Color) *DivElement { d.style.TextColor = c; return d }

// FontSize sets the inherited font size.
func (d *DivElement) FontSize(px float32) *DivElement { d.style.FontSize = px; return d }

// FontWeight sets the inherited font weight.
func (d *DivElement) FontWeight(w FontWeight) *DivElement { d.style.FontWeight = w; return d }

// ---- events ----

// OnClick runs fn when the element is pressed and released with the primary
// button. The deepest element with a click handler wins; ancestors are not
// also clicked.
func (d *DivElement) OnClick(fn func()) *DivElement {
	d.h.onClick = fn
	if d.style.Cursor == CursorDefault {
		d.style.Cursor = CursorPointer
	}
	return d
}

func (d *DivElement) OnDoubleClick(fn func()) *DivElement          { d.h.onDoubleClick = fn; return d }
func (d *DivElement) OnMouseDown(fn func(*MouseEvent)) *DivElement { d.h.onMouseDown = fn; return d }
func (d *DivElement) OnMouseUp(fn func(*MouseEvent)) *DivElement   { d.h.onMouseUp = fn; return d }
func (d *DivElement) OnMouseMove(fn func(*MouseEvent)) *DivElement { d.h.onMouseMove = fn; return d }
func (d *DivElement) OnMouseEnter(fn func()) *DivElement           { d.h.onMouseEnter = fn; return d }
func (d *DivElement) OnMouseLeave(fn func()) *DivElement           { d.h.onMouseLeave = fn; return d }
func (d *DivElement) OnScroll(fn func(*ScrollEvent)) *DivElement   { d.h.onScroll = fn; return d }

// OnKeyDown receives key presses while this element or a descendant has focus
// (or, on the root element, when nothing has focus).
func (d *DivElement) OnKeyDown(fn func(*KeyEvent)) *DivElement { d.h.onKeyDown = fn; return d }
func (d *DivElement) OnKeyUp(fn func(*KeyEvent)) *DivElement   { d.h.onKeyUp = fn; return d }

// OnClickOutside runs fn when any mouse button is pressed outside the element
// and its descendants, for closing menus and popovers. On an Overlay, presses
// on its parent (the anchor) count as inside, so the anchor's own OnClick
// decides what happens instead of closing and then reopening. Escape also
// runs fn on an Overlay: it closes the topmost open one, one per press.
func (d *DivElement) OnClickOutside(fn func()) *DivElement { d.h.onClickOutside = fn; return d }

// Tooltip shows s in a small floating label below the element after the
// pointer has rested on it for half a second. An empty string means none.
func (d *DivElement) Tooltip(s string) *DivElement { d.tooltip = s; return d }

// KeyContext names this element for contextual key bindings (BindIn). spec
// is whitespace-separated entries, each an identifier ("Editor") or a
// key=value pair ("mode=full"), as in GPUI. The bindings a context enables
// apply while focus is on the element or inside it, so a palette can own the
// arrow keys while its field has focus:
//
//	ui.Div().KeyContext("Palette").Child(ui.TextInput(query))
//
// KeyContext panics on a malformed spec, such as "Editor mode=".
func (d *DivElement) KeyContext(spec string) *DivElement {
	kc, err := parseKeyCtx(spec)
	if err != nil {
		panic(fmt.Sprintf("ui.KeyContext(%q): %s", spec, err))
	}
	d.keyContext, d.keyCtx = spec, kc
	return d
}

// OnAction runs fn when a is dispatched while focus is on this element or
// inside it: by a key binding, by ctx.Dispatch or by the command palette. The
// deepest element with a handler for a wins. A later OnAction for the same
// action replaces the earlier one.
func (d *DivElement) OnAction(a *Action, fn func()) *DivElement {
	for i := range d.actions {
		if d.actions[i].action == a {
			d.actions[i].fn = fn
			return d
		}
	}
	d.actions = append(d.actions, actionHandler{a, fn})
	return d
}

// OnTextInput receives typed text while focused. It implies Focusable.
func (d *DivElement) OnTextInput(fn func(string)) *DivElement {
	d.h.onTextInput = fn
	d.focusable = true
	return d
}

func (d *DivElement) OnFocus(fn func()) *DivElement { d.h.onFocus = fn; d.focusable = true; return d }
func (d *DivElement) OnBlur(fn func()) *DivElement  { d.h.onBlur = fn; d.focusable = true; return d }

// Focusable lets the element take keyboard focus when clicked or tabbed to.
func (d *DivElement) Focusable() *DivElement { d.focusable = true; return d }

// TabIndex sets where the element sits in the Tab order and implies Focusable.
// Elements with a positive index come first, in ascending order (ties in tree
// order); the rest, at index 0, follow in tree order, as in HTML. Shift+Tab
// walks the order backwards. A negative index keeps the element focusable by
// click or handle, but Tab skips it.
func (d *DivElement) TabIndex(i int) *DivElement {
	d.tabIndex, d.focusable = i, true
	return d
}

// TabStop(false) makes Tab skip the element while it stays focusable by click
// or handle; TabStop(true) puts it back in the Tab order. It does not make an
// element focusable on its own.
func (d *DivElement) TabStop(on bool) *DivElement { d.noTabStop = !on; return d }

// AutoFocus focuses the element when it first appears. It implies Focusable.
func (d *DivElement) AutoFocus() *DivElement {
	d.autoFocus, d.focusable = true, true
	return d
}

// FocusHandle attaches h, so the element can be focused from code with
// h.Focus(). It implies Focusable.
func (d *DivElement) FocusHandle(h *FocusHandle) *DivElement {
	d.handle, d.focusable = h, true
	return d
}

// CapturePointer keeps delivering mouse move and up events to the element
// while a button pressed on it is held, even when the pointer leaves it
// (drag gestures such as sliders or text selection).
func (d *DivElement) CapturePointer() *DivElement { d.capture = true; return d }

// FocusHandle lets code move keyboard focus to an element, for shortcuts
// like Ctrl+F or focusing a field that was just created:
//
//	search := ui.NewFocusHandle()
//	ui.TextInput(query).FocusHandle(search)
//	... OnKeyDown: search.Focus()
//
// Use it on the UI thread (in event handlers and render functions).
type FocusHandle struct {
	rt      *Runtime
	id      uint64
	pending bool
	none    bool // "nothing focused", from FocusedHandle; id is 0
}

// NewFocusHandle creates a handle; attach it to one element.
func NewFocusHandle() *FocusHandle { return &FocusHandle{} }

// Focus focuses the element on the next frame, after any changes made by the
// same event have been rebuilt. So a handle attached to a field that the
// event replaces (for example the title field of a newly created note)
// focuses the new field, not the old one.
func (h *FocusHandle) Focus() {
	h.pending = true
	if h.rt != nil {
		// A focus the code asks for scrolls the element into view, as Tab does, so a field
		// inside a scrolling body (a modal's) is shown when it takes focus.
		h.rt.revealFocus = true
		h.rt.pendingHandles = append(h.rt.pendingHandles, h)
		h.rt.requestFrame()
	}
}

// Blur removes focus from the element if it has it.
func (h *FocusHandle) Blur() {
	if h.Focused() {
		h.rt.setFocus(0)
	}
}

// Focused reports whether the element has keyboard focus.
func (h *FocusHandle) Focused() bool { return h.rt != nil && h.id != 0 && h.rt.focusID == h.id }

// FocusNow focuses the element at once, or clears focus for a "nothing
// focused" handle, and reports whether it did. If the element is no longer in
// the tree, or is hidden by Display, it does nothing and reports false: a
// restore must not wait and later take focus from wherever the user has moved
// on to. To focus an element that is about to appear, use Focus.
func (h *FocusHandle) FocusNow() bool {
	if h.rt == nil {
		return false
	}
	if h.none {
		h.rt.setFocus(0)
		return true
	}
	if n := h.rt.byID[h.id]; n != nil && !h.rt.undisplayed(n) {
		h.rt.setFocus(h.id)
		return true
	}
	return false
}

// FocusedHandle returns a handle to whatever has keyboard focus now, or to
// "nothing focused", so focus can be restored later, for example when a
// palette or dialog closes. Element IDs are stable across rebuilds, so the
// handle keeps working after re-renders.
func (rt *Runtime) FocusedHandle() *FocusHandle {
	if rt.focusID == 0 {
		return &FocusHandle{rt: rt, none: true}
	}
	return &FocusHandle{rt: rt, id: rt.focusID}
}

// FocusedHandle returns a handle to what has keyboard focus now; see
// Runtime.FocusedHandle.
func (c *Context) FocusedHandle() *FocusHandle { return c.rt.FocusedHandle() }

// ---- helpers ----

// If returns e when cond is true and nil otherwise.
func If(cond bool, e Element) Element {
	if cond {
		return e
	}
	return nil
}

// IfElse returns a when cond is true and b otherwise.
func IfElse(cond bool, a, b Element) Element {
	if cond {
		return a
	}
	return b
}

// Map builds one element per item.
func Map[T any](items []T, fn func(i int, item T) Element) []Element {
	out := make([]Element, 0, len(items))
	for i, it := range items {
		out = append(out, fn(i, it))
	}
	return out
}

// Spacer grows to fill free space in a row or column.
func Spacer() *DivElement { return Div().Grow(1) }

func isNil(e Element) bool {
	switch v := e.(type) {
	case nil:
		return true
	case *DivElement:
		return v == nil
	case *TextElement:
		return v == nil
	case *ComponentElement:
		return v == nil
	case *UniformListElement:
		return v == nil
	}
	return false
}
