package ui

// TitleBarStyle is the look of the parts of a title bar. A part's style merges
// over the built-in defaults, then over the instance's Styles; see docs/styling.md.
// Bar is the strip, with its height, fill, bottom border and text color, which
// the items inherit; Center is the box of the centered element, which fills the
// room between the sides.
type TitleBarStyle struct {
	Bar    Style // the strip: its height, fill, bottom border and text
	Center Style // the box of the centered element
}

// merge returns s with the set fields of o applied on top, part by part.
func (s TitleBarStyle) merge(o TitleBarStyle) TitleBarStyle {
	return TitleBarStyle{
		Bar:    s.Bar.Merge(o.Bar),
		Center: s.Center.Merge(o.Center),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s TitleBarStyle) plain() TitleBarStyle {
	return TitleBarStyle{
		Bar:    plainStyle(s.Bar),
		Center: plainStyle(s.Center),
	}
}

// titleBarHeight is the height of a title bar at 100 percent text size.
const titleBarHeight = 40

// titleBarDimOpacity is the opacity of the content of a title bar while its window
// is not focused. The caption buttons keep full opacity.
const titleBarDimOpacity = 0.65

// The keys of the title bar's boxes. Assistive technology does not see them; the
// runtime reads the sides' widths from them after a layout (see titleBarInset).
const (
	titleStartKey  = "titlebar.start"
	titleEndKey    = "titlebar.end"
	titleCenterKey = "titlebar.center"
)

// titleBarDefaults returns the built-in look of a title bar, from the colors of t
// and the Text size scale: a strip in the panel color with a border along its
// bottom. Under high contrast the strip is the surface color, and its border and
// text the text color.
func titleBarDefaults(t *Theme, scale float32) TitleBarStyle {
	d := TitleBarStyle{
		// No fill and no border: the bar is part of the window's content, as a
		// Windows 11 title bar over Mica is, and an app that wants a band styles Bar.
		Bar:    Style{Direction: Row, Align: Center, Height: Px(titleBarHeight * scale), NoShrink: true, TextColor: t.Text},
		Center: Style{Direction: Row, Align: Center, Justify: Center},
	}
	return d
}

// resolveTitleBar returns the look of a title bar: the defaults, merged with the
// instance's Styles. Under high contrast the overrides are plain (see plainStyle).
func resolveTitleBar(ctx *Context, inst TitleBarStyle) TitleBarStyle {
	return layered(ctx, titleBarDefaults(ctx.Theme(), textScale(ctx)), TitleBarStyle.merge, TitleBarStyle.plain,
		instanceLayer(inst))
}

// TitleBarElement is a window's own title bar, for a window with the app's
// chrome (see app.ChromeCustom). It is a row: the children go at its start, the
// left, in order, as in a Div. Trailing items and the caption buttons go at its
// end, the right. Center puts an element in the middle of the window, not
// between the sides: the element gets the room between the wider side and the
// other one, so it is centered on the bar's whole width and never covers a side.
// Wider content than that room is clipped.
//
// The bar is a window drag area (see WindowAreaDrag): its empty parts move the
// window, and its children that take input still work. Its content dims while
// the window is not focused, and the caption buttons do not.
type TitleBarElement struct {
	children  []Element
	trailing  []Element
	center    Element
	noButtons bool // see Buttons
	key       string
	styles    TitleBarStyle
	size      sizing
}

// TitleBar creates an empty title bar, with the caption buttons at its end.
func TitleBar() *TitleBarElement { return &TitleBarElement{} }

func (*TitleBarElement) isElement()           {}
func (t *TitleBarElement) elementKey() string { return t.key }

// Key identifies the title bar among its siblings (see DivElement.Key).
func (t *TitleBarElement) Key(k string) *TitleBarElement { t.key = k; return t }

// Child appends an item at the start of the bar, the left. A nil item is ignored.
func (t *TitleBarElement) Child(e Element) *TitleBarElement {
	if !isNil(e) {
		t.children = append(t.children, e)
	}
	return t
}

// Trailing appends items at the end of the bar, before the caption buttons. nil
// items are ignored.
func (t *TitleBarElement) Trailing(els ...Element) *TitleBarElement {
	for _, e := range els {
		if !isNil(e) {
			t.trailing = append(t.trailing, e)
		}
	}
	return t
}

// Center puts e in the middle of the window's width. It is laid out in the room
// the sides leave: the wider of the start side and the end side is kept clear on
// both ends, so e never covers them. When the sides leave too little room, e is
// clipped to what is left. A nil e is ignored.
func (t *TitleBarElement) Center(e Element) *TitleBarElement {
	if !isNil(e) {
		t.center = e
	}
	return t
}

// Buttons sets whether the caption buttons (minimize, maximize or restore, and
// close, see TitleBarButtons) are at the end of the bar. They are shown by
// default. Turn them off when the window has no caption buttons of its own.
func (t *TitleBarElement) Buttons(show bool) *TitleBarElement { t.noButtons = !show; return t }

// The sizing methods give the bar's outer box a size. The bar is as high as the
// Text size scale makes its default, 40 px at 100 percent, and it fills the width
// of a column (see Sizing in docs/styling.md). Windows' own caption buttons are
// 32 px high, so a bar lower than 32 px cannot hold them.
func (t *TitleBarElement) Width(l Length) *TitleBarElement { t.size.setWidth(l); return t }

// Height sets the height of the bar. The default is 40 px at 100 percent Text
// size; see TitleBar.
func (t *TitleBarElement) Height(l Length) *TitleBarElement { t.size.setHeight(l); return t }

// Style sets the look of the bar's Bar part, for this instance only. It is the same
// as Styles(TitleBarStyle{Bar: s}); calling it again merges.
func (t *TitleBarElement) Style(s Style) *TitleBarElement {
	t.styles.Bar = t.styles.Bar.Merge(s)
	return t
}

// Styles sets the look of the parts of the bar, for this instance only. Set fields
// win over the defaults; see TitleBarStyle and docs/styling.md. Calling Styles
// again merges with the earlier values.
func (t *TitleBarElement) Styles(st TitleBarStyle) *TitleBarElement {
	t.styles = t.styles.merge(st)
	return t
}

func (t *TitleBarElement) expand(*Context) Element {
	return Component(func(ctx *Context) Element {
		th := ctx.Theme()
		st := resolveTitleBar(ctx, t.styles)
		// The content dims while the window is not focused, as the window's own title
		// bar does. Under high contrast it keeps full strength.
		dim := float32(1)
		if !ctx.Window().Focused() && !th.HighContrast {
			dim = titleBarDimOpacity
		}
		var center Element
		if t.center != nil {
			// The center's room depends on the sides' widths from the last layout (see
			// titleBarInset), and the bar renders again when they change.
			inset := ctx.rt.titleBarInset(ctx.inst)
			ctx.rt.watchTitleBar(ctx.inst, inset, true)
			center = Div().Key(titleCenterKey).Style(st.Center).Absolute().Top(0).Bottom(0).
				Left(inset).Right(inset).ClipContent().Opacity(dim).Child(t.center)
		} else {
			ctx.rt.watchTitleBar(ctx.inst, 0, false)
		}
		// The start side shrinks when the window is too narrow for both sides, and clips
		// what does not fit, so a long title truncates instead of covering the end.
		start := Div().Key(titleStartKey).Row().Align(Center).Gap(10).Shrink(1).MinWidth(Px(0)).ClipContent().
			Opacity(dim).Children(t.children...)
		end := Div().Key(titleEndKey).Row().Align(Center).Gap(10).Shrink(0).AlignSelf(Stretch).
			Child(Div().Row().Align(Center).Gap(10).Opacity(dim).Children(t.trailing...))
		if !t.noButtons {
			end.Child(TitleBarButtons())
		}
		bar := Div().Key(t.key).asPart("TitleBar.Bar").Style(st.Bar).WindowArea(WindowAreaDrag).
			Child(start).Child(Div().Grow(1).MinWidth(Px(0))).Child(end)
		if center != nil {
			bar.Child(center)
		}
		return t.size.apply(bar)
	})
}

// titleBarInset returns the room the centered element of the title bar of inst
// must keep clear on each end: the wider of its two sides, from the last layout.
// It is 0 before the bar has been laid out, and while it is not in the tree.
func (rt *Runtime) titleBarInset(inst *componentInstance) float32 {
	n := inst.root
	if n == nil || rt.byID[n.id] != n {
		return 0
	}
	var inset float32
	for _, c := range n.children {
		if c.key == titleStartKey || c.key == titleEndKey {
			inset = max(inset, c.lay.W)
		}
	}
	return inset
}

// watchTitleBar records the inset a title bar with a centered element rendered
// with, so that a change of it renders the bar again (see titleBarsResized). A bar
// without one is not watched.
func (rt *Runtime) watchTitleBar(inst *componentInstance, inset float32, centered bool) {
	if !centered {
		delete(rt.titleWatches, inst)
		return
	}
	if rt.titleWatches == nil {
		rt.titleWatches = map[*componentInstance]float32{}
	}
	rt.titleWatches[inst] = inset
}

// titleBarsResized reports whether a title bar's inset changed in the last layout.
// Such a bar is marked for a render, which places its centered element in the room
// the sides have now, before the frame is painted.
func (rt *Runtime) titleBarsResized() bool {
	changed := false
	for inst, seen := range rt.titleWatches {
		if inst.disposed {
			delete(rt.titleWatches, inst)
			continue
		}
		if rt.titleBarInset(inst) != seen {
			inst.invalidate()
			changed = true
		}
	}
	return changed
}

// layoutWatchesResized reports whether a split or a title bar changed in the last
// layout, and marks each one that did for a render. Both are checked, so neither
// waits for the other.
func (rt *Runtime) layoutWatchesResized() bool {
	splits := rt.splitsResized()
	titles := rt.titleBarsResized()
	return splits || titles
}
