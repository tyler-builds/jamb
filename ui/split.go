package ui

import (
	"fmt"
	"maps"
	"math"
	"strconv"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/layout"
)

// splitHit is the width of the divider's hit area, in logical pixels. The divider
// takes only the bar's thickness in the layout (see splitRender.line), so the panes
// meet at the bar, which is their border. The hit area is centered on the bar and
// lies over the edges of the panes beside it, so the divider is easy to grab even
// when the bar is a pixel wide.
const splitHit = 9

// The keyboard moves a divider by a small step, and by a large one with Shift. A step
// is in logical pixels when the pane it resizes is a pixel pane (or a fill pane), and
// in percent of the length available to the panes when it is a percentage pane.
const (
	splitStepPx, splitLargePx           = 10, 50
	splitStepPercent, splitLargePercent = 5, 10
)

// SplitStyle is the look of each part of a split. A part's style merges over the
// built-in defaults, then over the theme's Split, then over the Styles of the
// instance; see docs/styling.md.
//
// Divider is the bar drawn in the middle of the divider: its Width is the thickness
// of a horizontal split's divider, and its Height the thickness of a vertical one; it
// runs the divider's whole length by default.
// DividerHover is the bar while the pointer is over the divider, DividerActive while
// the divider is dragged, and DividerFocusVisible while it has keyboard focus. Grip
// is the mark drawn over the middle of the bar, and Pane is the box of each pane.
type SplitStyle struct {
	Divider             Style // the bar in the middle of the divider; its Width (Height when vertical) is the thickness
	DividerHover        Style // the bar while the pointer is over the divider
	DividerActive       Style // the bar while the divider is dragged
	DividerFocusVisible Style // the bar while the divider has keyboard focus
	Grip                Style // the mark over the middle of the bar
	Pane                Style // the box of each pane
}

// merge returns s with the set fields of o applied on top, part by part.
func (s SplitStyle) merge(o SplitStyle) SplitStyle {
	return SplitStyle{
		Divider:             s.Divider.Merge(o.Divider),
		DividerHover:        s.DividerHover.Merge(o.DividerHover),
		DividerActive:       s.DividerActive.Merge(o.DividerActive),
		DividerFocusVisible: s.DividerFocusVisible.Merge(o.DividerFocusVisible),
		Grip:                s.Grip.Merge(o.Grip),
		Pane:                s.Pane.Merge(o.Pane),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s SplitStyle) plain() SplitStyle {
	return SplitStyle{
		Divider:             plainStyle(s.Divider),
		DividerHover:        plainStyle(s.DividerHover),
		DividerActive:       plainStyle(s.DividerActive),
		DividerFocusVisible: plainStyle(s.DividerFocusVisible),
		Grip:                plainStyle(s.Grip),
		Pane:                plainStyle(s.Pane),
	}
}

// splitDefaults returns the built-in look of a split, from the colors of t. Under
// high contrast the bar and the grip are the text color, the focus ring is a solid
// outline, and the hover and drag states add no color, since the bar is already
// visible.
func splitDefaults(t *Theme, vertical bool) SplitStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	bar, grip := t.Border, t.TextMuted
	if t.HighContrast {
		bar, grip = t.Text, t.Text
	}
	// The bar is 1 px thick and runs the whole length of the divider: the divider centers it,
	// so without a length it would have none and draw nothing.
	thick, mark := Style{Width: Px(1), Height: Full}, Style{Width: Px(4), Height: Px(24)}
	if vertical {
		thick, mark = Style{Height: Px(1), Width: Full}, Style{Width: Px(24), Height: Px(4)}
	}
	d := SplitStyle{
		Divider:             thick.Merge(Style{Background: bar}),
		DividerHover:        Style{Background: t.Accent},
		DividerActive:       Style{Background: t.AccentActive},
		DividerFocusVisible: Style{Shadows: []Shadow{ring}},
		Grip:                mark.Merge(Style{Radius: 2, Background: grip}),
		Pane:                Style{OverflowX: OverflowHidden, OverflowY: OverflowHidden},
	}
	if t.HighContrast {
		d.DividerHover, d.DividerActive = Style{}, Style{}
	}
	return d
}

// resolveSplit returns the look of a split: the defaults, merged with the theme's
// Split and then the instance's Styles. Under high contrast the two overrides are
// plain (see plainStyle).
func resolveSplit(ctx *Context, vertical bool, inst SplitStyle) SplitStyle {
	t := ctx.Theme()
	return layered(ctx, splitDefaults(t, vertical), SplitStyle.merge, SplitStyle.plain,
		themeLayer(t.Split), instanceLayer(inst))
}

// SplitOrientation says how the panes of a split are laid out.
type SplitOrientation uint8

const (
	SplitHorizontal SplitOrientation = iota // the panes sit side by side (the default)
	SplitVertical                           // the panes are stacked, one above the other
)

// SplitGrip is when a split's divider shows its grip, the mark over the middle of its bar
// (the Grip part). The bar itself is always drawn.
type SplitGrip uint8

const (
	// SplitGripShown always shows the grip. It is the default.
	SplitGripShown SplitGrip = iota
	// SplitGripOnHover shows the grip only while the pointer is over the divider, while it is
	// dragged, and while it has keyboard focus. Otherwise the divider is the bar alone, a line
	// between the panes like a border.
	SplitGripOnHover
	// SplitGripHidden never shows the grip: the divider is the bar alone. It still changes color
	// when the pointer is over it (DividerHover), so it can be found.
	SplitGripHidden
)

// PaneElement is one pane of a split, made by Pane and given to Split.
type PaneElement struct {
	content     Element
	key         string
	size        Length          // the starting size, when sized is set (see Size)
	sized       bool            // the pane has a size, from Size or SizeSignal; otherwise it fills
	sig         *Signal[Length] // the size the app owns (see SizeSignal); nil: the split keeps it
	min, max    Length          // the limits, in pixels or percent; max is Auto when there is none
	collapsible bool            // the pane can be collapsed (see Collapsible)
	collapsed   *Signal[bool]   // the collapse state the app owns (see Collapsed); nil: the split keeps it
	name        string          // the accessible name of the pane's group, see Label
}

// Pane makes a pane of a split that shows content. The pane takes a size with Size or
// SizeSignal, and a pane with neither fills the space that the sized panes leave; see
// Split.
func Pane(content Element) *PaneElement {
	return &PaneElement{content: content, min: Px(0), max: Auto}
}

// Size sets the pane's starting size, in ui.Px or ui.Percent. A percentage is of the length
// the panes have, which is the split's length less its dividers. The split keeps the size as
// the user moves the dividers, in the same unit. The starting size applies when the pane first
// appears, and is ignored when the pane has a SizeSignal.
func (p *PaneElement) Size(l Length) *PaneElement {
	p.size, p.sized = splitLength("Size", l), true
	return p
}

// SizeSignal stores the pane's size in v, and sets it when the user moves a divider beside the
// pane, so the app can save the size and restore it. The signal holds ui.Px or ui.Percent
// lengths: a divider keeps the unit of the pane, and a pixel pane keeps its pixels when the
// window is resized. The split reads the signal, so the app can also change the size.
func (p *PaneElement) SizeSignal(v *Signal[Length]) *PaneElement {
	if v == nil {
		panic("ui.Pane.SizeSignal: the signal is nil; make one with ui.NewSignal(ui.Px(200))")
	}
	p.sig, p.sized = v, true
	return p
}

// Min sets the least size of the pane, in ui.Px or ui.Percent. A divider stops before the pane
// goes below it.
func (p *PaneElement) Min(l Length) *PaneElement {
	p.min = splitLength("Min", l)
	return p
}

// Max sets the most size of the pane, in ui.Px or ui.Percent. A divider stops before the pane
// goes past it. When Min and Max contradict each other, Min wins.
func (p *PaneElement) Max(l Length) *PaneElement {
	p.max = splitLength("Max", l)
	return p
}

// Collapsible lets the user collapse the pane to zero size, with Enter or a double-click on the
// divider that collapses it, and restore it the same way. The divider is the one between the
// pane and the split's nearest fill pane, on that side; with no fill pane, the divider after the
// pane, or before it for the last pane. A divider collapses one pane, and the first one in order
// keeps it when two would share it. A collapsed pane keeps its size, and its divider and the
// divider beside it do not move while it is collapsed. The state is kept by the split unless
// Collapsed is given.
func (p *PaneElement) Collapsible() *PaneElement {
	p.collapsible = true
	return p
}

// Collapsed stores in v whether the pane is collapsed, so the app can save the state, and makes
// the pane collapsible (see Collapsible). A collapsed pane is hidden, and its content stays in
// the tree, so its state survives the collapse.
func (p *PaneElement) Collapsed(v *Signal[bool]) *PaneElement {
	p.collapsed, p.collapsible = v, true
	return p
}

// Label names the pane's group for assistive technology, such as "Notes". A pane without a name
// is not a group in the accessibility tree.
func (p *PaneElement) Label(name string) *PaneElement {
	p.name = name
	return p
}

// Key gives the pane an identity among the panes of its split, so its content keeps its state
// when panes are added, removed or reordered. The keys of a split's panes must differ. A pane
// without a key is known by its index.
func (p *PaneElement) Key(k string) *PaneElement {
	p.key = k
	return p
}

// identity returns the pane's identity among the panes of its split: its key, or its index,
// written so that it cannot be taken for a key.
func (p *PaneElement) identity(i int) string {
	if p.key != "" {
		return p.key
	}
	return "#" + strconv.Itoa(i)
}

// splitLength returns l, which must be a pixel or a percentage length for the method named
// method; a split's sizes and limits take no other kind.
func splitLength(method string, l Length) Length {
	if l.Unit != layout.UnitPx && l.Unit != layout.UnitPercent {
		panic("ui.Pane." + method + ": the length must be a pixel or percentage length, such as ui.Px(200) or ui.Percent(20)")
	}
	return l
}

// SplitElement lays out panes side by side, or stacked, with a divider between each two
// neighbours. The user drags a divider, or moves it with the keyboard, to give one pane
// more of the space. The panes are Elements of any kind, so a split can contain another
// split.
//
// A pane with a Size keeps it as the user moves the dividers, in its own unit: a pixel pane
// keeps its pixels, and a percentage pane keeps its share of the length the panes have, which is
// the split's length less its dividers. A pane with neither a Size nor a SizeSignal fills the
// space that the sized panes leave, in proportion to their weights (equal by default). So a drag
// that changes a sized pane changes every fill pane, each in proportion to its weight. A drag
// between two fill panes moves space only between those two: their weights change, and their sum
// stays the same. A drag between two sized panes moves space only between those two, each in its
// own unit and within its limits, and stops when either reaches a limit. A divider beside a
// collapsed pane does not move.
//
// The layout is resolved at the split's length (see resolvePanes). A pixel pane keeps its size
// when the window is resized, a percentage pane scales with it, and a fill pane takes the
// change. When no pane can take the space that is left over, the last visible pane is drawn
// with it, and its stored size does not change.
//
// Each divider is one focusable element with the window splitter role of the Accessibility
// Programmatic Guidelines (https://www.w3.org/WAI/ARIA/apg/patterns/windowsplitter/). Its value
// is the size of the pane it resizes, in that pane's unit, with the least and the greatest size
// that pane can take. The arrow keys move it along the split (Left and Right, or Up and Down in
// a vertical split), Shift for a larger step, and Home and End to the limits. Enter collapses or
// restores the pane that the divider collapses (see Pane.Collapsible), and a double-click on the
// divider does the same.
type SplitElement struct {
	panes     []*PaneElement
	key       string
	orient    SplitOrientation
	name      string // the accessible name of the dividers, see Label
	onChange  func()
	onRelease func() // runs when a pointer drag of a divider ends (see dock.go)
	grip      SplitGrip
	styles    SplitStyle
	size      sizing // the split's outer box, from the sizing methods (see Sizing in docs/styling.md)
}

// Split lays out panes in order, with a divider between each two. One pane renders alone, with
// no divider, and no pane renders an empty box of the split's size. The panes need distinct
// keys (see Pane.Key), and the split fills its container by default (see Width).
func Split(panes ...*PaneElement) *SplitElement {
	seen := make(map[string]bool, len(panes))
	for i, p := range panes {
		if p == nil {
			panic(fmt.Sprintf("ui.Split: pane %d is nil; make it with ui.Pane", i))
		}
		id := p.identity(i)
		if seen[id] {
			panic(fmt.Sprintf("ui.Split: two panes have the key %q; the keys of a split's panes must differ", id))
		}
		seen[id] = true
	}
	return &SplitElement{panes: panes}
}

func (*SplitElement) isElement()           {}
func (s *SplitElement) elementKey() string { return s.key }

// Key gives the split a stable identity among its siblings.
func (s *SplitElement) Key(k string) *SplitElement { s.key = k; return s }

// Orientation sets whether the panes sit side by side (SplitHorizontal, the
// default) or are stacked (SplitVertical).
func (s *SplitElement) Orientation(o SplitOrientation) *SplitElement { s.orient = o; return s }

// Label names every divider of the split for assistive technology, such as "Resize sidebar". Without
// it, a divider whose two panes both have a Pane.Label is named after them, such as "Resize Sidebar
// and Editor", and any other divider is named with the Resize panes text of the window's Strings.
func (s *SplitElement) Label(name string) *SplitElement { s.name = name; return s }

// Grip sets when the divider shows its grip: always (SplitGripShown, the default), only while
// the pointer is over the divider or it is dragged or focused (SplitGripOnHover), or never
// (SplitGripHidden). Without the grip, the divider is a line between the panes.
func (s *SplitElement) Grip(g SplitGrip) *SplitElement { s.grip = g; return s }

// OnChange runs fn after the user changes the size of a pane, from the keyboard or the pointer,
// and only when a size actually changes. The sizes are set first (see Pane.SizeSignal). Collapsing
// or restoring a pane is not a size change, so it does not run fn.
func (s *SplitElement) OnChange(fn func()) *SplitElement { s.onChange = fn; return s }

// The sizing methods give the split's outer box a size. A split fills its container
// by default; see Sizing in docs/styling.md.
func (s *SplitElement) Width(l Length) *SplitElement { s.size.setWidth(l); return s }

// Height sets the height of the split.
func (s *SplitElement) Height(l Length) *SplitElement { s.size.setHeight(l); return s }

// MinWidth sets the smallest width of the split.
func (s *SplitElement) MinWidth(l Length) *SplitElement { s.size.setMinWidth(l); return s }

// MinHeight sets the smallest height of the split.
func (s *SplitElement) MinHeight(l Length) *SplitElement { s.size.setMinHeight(l); return s }

// MaxWidth sets the largest width of the split.
func (s *SplitElement) MaxWidth(l Length) *SplitElement { s.size.setMaxWidth(l); return s }

// MaxHeight sets the largest height of the split.
func (s *SplitElement) MaxHeight(l Length) *SplitElement { s.size.setMaxHeight(l); return s }

// Grow makes the split take a share of the free space on its parent's main axis.
func (s *SplitElement) Grow(f float32) *SplitElement { s.size.setGrow(f); return s }

// GridSpan sets how many columns and rows the split covers in its parent's grid.
func (s *SplitElement) GridSpan(cols, rows int) *SplitElement { s.size.setSpan(cols, rows); return s }

// Styles sets the look of the parts of the split, for this instance only. Set fields
// win over the theme's Split and the defaults; see SplitStyle and docs/styling.md.
// Calling Styles again merges with the earlier values.
func (s *SplitElement) Styles(st SplitStyle) *SplitElement {
	s.styles = s.styles.merge(st)
	return s
}

// vertical reports whether the split stacks its panes.
func (s *SplitElement) vertical() bool { return s.orient == SplitVertical }

// collapseDivider returns the index of the divider that collapses pane i, or -1 when the split has
// no divider for it. The divider is the one between the pane and the nearest fill pane, on that
// fill pane's side; on a tie the later fill pane is the one. With no fill pane the last pane takes
// the remainder, so the divider after a pane collapses it, and the last pane collapses through the
// divider before it.
func (s *SplitElement) collapseDivider(i int) int {
	n := len(s.panes)
	dist := func(f int) int {
		if f > i {
			return f - i
		}
		return i - f
	}
	near := -1
	for f, p := range s.panes {
		if f == i || p.sized {
			continue
		}
		if near < 0 || dist(f) < dist(near) || dist(f) == dist(near) && f > i {
			near = f
		}
	}
	switch {
	case near > i:
		return i
	case near >= 0:
		return i - 1
	case i < n-1:
		return i
	case n > 1:
		return n - 2
	}
	return -1
}

// collapseOwner returns the pane that divider d collapses, or -1 when it collapses none: the first
// collapsible pane, in order, whose collapseDivider is d.
func (s *SplitElement) collapseOwner(d int) int {
	for i, p := range s.panes {
		if p.collapsible && s.collapseDivider(i) == d {
			return i
		}
	}
	return -1
}

// paneMemo is the state the split keeps for a pane whose state the app does not own: its size, its
// fill weight and whether it is collapsed. The split keeps it by the pane's identity.
type paneMemo struct {
	size      Length
	weight    float64
	collapsed bool
}

// splitDrag is a drag of a divider: the divider, the pointer's position along the split when the
// drag began, the sizes of its two panes then in logical pixels, and the change last applied.
type splitDrag struct {
	on     bool
	d      int
	at     float64
	sa, sb float64
	last   float64
}

// paneView is a pane as a render sees it: its configuration with the size, weight and collapse state
// the app or the split holds for it, and its limits at the split's length.
type paneView struct {
	pane      *PaneElement
	sized     bool
	size      Length
	weight    float64
	collapsed bool
	lo, hi    float64 // the limits in logical pixels; hi is +Inf when there is no most
}

// percent reports whether the pane's size is a percentage.
func (v paneView) percent() bool { return v.sized && v.size.Unit == layout.UnitPercent }

// splitModel is the state of a split's panes at its length: each pane's view, and the sizes the
// layout gives them (see resolvePanes). Its base sizes are the ones a move changes; shown sizes
// are what is drawn.
type splitModel struct {
	measured    bool // the split has a length: it has been laid out
	avail       float64
	views       []paneView
	base, shown []float64
}

// inert reports whether divider d cannot move: a collapsed pane is on either side of it.
func (m splitModel) inert(d int) bool { return m.views[d].collapsed || m.views[d+1].collapsed }

// driver returns the pane that divider d resizes for an arrow key moving it in direction dir (-1 for
// Left and Up, 1 for Right and Down), and for its accessibility value (dir 1). It is the sized pane of
// the two when only one is sized. When both are sized, it is the pane the key enlarges: the before
// pane for a key that moves the divider forward, and the after pane for one that moves it back.
// When both fill, it is the before pane.
func (m splitModel) driver(d, dir int) int {
	a, b := m.views[d], m.views[d+1]
	switch {
	case a.sized && !b.sized:
		return d
	case b.sized && !a.sized:
		return d + 1
	case a.sized && b.sized && dir < 0:
		return d + 1
	}
	return d
}

// pairRange returns how far the before pane of divider d can move, in logical pixels, from the sizes
// sa and sb of its two panes: the least and the greatest change that keeps both within their limits.
func (m splitModel) pairRange(d int, sa, sb float64) (dmin, dmax float64) {
	a, b := m.views[d], m.views[d+1]
	dmin = max(a.lo-sa, sb-b.hi)
	dmax = min(a.hi-sa, sb-b.lo)
	if dmin > dmax {
		return 0, 0
	}
	return dmin, dmax
}

// toUnit returns the logical pixels px of pane v in the pane's unit: pixels, or percent of the length
// available to the panes.
func (m splitModel) toUnit(v paneView, px float64) float64 {
	if !v.percent() {
		return px
	}
	if m.avail <= 0 {
		return 0
	}
	return px / m.avail * 100
}

// fromUnit returns x in the unit of pane v as logical pixels; see toUnit.
func (m splitModel) fromUnit(v paneView, x float64) float64 {
	if !v.percent() {
		return x
	}
	return x / 100 * m.avail
}

// step returns the size of an arrow step of pane v, large with Shift, in logical pixels and in the
// pane's unit.
func (m splitModel) step(v paneView, large bool) (px, unit float64) {
	if v.percent() {
		unit = splitStepPercent
		if large {
			unit = splitLargePercent
		}
		return m.fromUnit(v, unit), unit
	}
	unit = splitStepPx
	if large {
		unit = splitLargePx
	}
	return unit, unit
}

// rangeOf returns the accessibility value of divider d, which is the size of the pane it resizes
// (see driver) in that pane's unit, the least and the greatest size that pane can take, and the
// small and large steps.
func (m splitModel) rangeOf(d int) (value, lo, hi, small, large float64) {
	if !m.measured {
		return 0, 0, 0, 0, 0
	}
	drv := m.driver(d, 1)
	sa, sb := m.base[d], m.base[d+1]
	dmin, dmax := m.pairRange(d, sa, sb)
	v := m.views[drv]
	if drv == d {
		value, lo, hi = sa, sa+dmin, sa+dmax
	} else {
		value, lo, hi = sb, sb-dmax, sb-dmin
	}
	_, small = m.step(v, false)
	_, large = m.step(v, true)
	return m.toUnit(v, value), m.toUnit(v, lo), m.toUnit(v, hi), small, large
}

// splitRender is the state of one render of a split, which its dividers share: the split, the key
// and orientation of its box, the accessible name of its dividers, and the signals that hold its
// length, the state of its panes and the drag.
type splitRender struct {
	s      *SplitElement
	key    string
	vert   bool
	length *Signal[float64]
	memo   *Signal[map[string]paneMemo]
	drag   *Signal[splitDrag]
	// line is the room each divider takes in the layout: the bar's thickness, its
	// Divider Width (Height when vertical) in pixels, or 1.
	line float64
}

// dividerLine returns the thickness of the bar of st, in logical pixels: the
// Divider part's Width, or its Height in a vertical split, when it is in pixels,
// and 1 otherwise.
func dividerLine(st SplitStyle, vert bool) float64 {
	l := st.Divider.Width
	if vert {
		l = st.Divider.Height
	}
	if l.Unit == layout.UnitPx && l.Value >= 0 {
		return float64(l.Value)
	}
	return 1
}

// model returns the state of the panes at a split length of total logical pixels. Before the split
// has been laid out, total is 0 and the model is not measured.
func (r *splitRender) model(total float64) splitModel {
	panes := r.s.panes
	m := splitModel{
		measured: total > 0,
		avail:    max(total-float64(max(len(panes)-1, 0))*r.line, 0),
		views:    make([]paneView, len(panes)),
	}
	mem := r.memo.Get()
	ins := make([]paneIn, len(panes))
	for i, p := range panes {
		e, ok := mem[p.identity(i)]
		if !ok {
			e = paneMemo{size: p.size, weight: 1}
		}
		v := paneView{pane: p, sized: p.sized, size: e.size, weight: e.weight, collapsed: p.collapsible && e.collapsed}
		if p.sig != nil {
			v.size = p.sig.Get()
		}
		if p.collapsed != nil {
			v.collapsed = p.collapsible && p.collapsed.Get()
		}
		if v.sized && v.size.Unit != layout.UnitPx && v.size.Unit != layout.UnitPercent {
			panic("ui.Pane: the length in the SizeSignal must be a pixel or percentage length, such as ui.Px(200)")
		}
		v.lo, v.hi = paneLimits(p, m.avail)
		m.views[i] = v
		ins[i] = paneIn{sized: v.sized, size: v.size, lo: v.lo, hi: v.hi, weight: v.weight, hidden: v.collapsed}
	}
	m.base, m.shown = resolvePanes(ins, m.avail)
	return m
}

// write stores the state of pane i of the model m, with f applied to it.
func (r *splitRender) write(m splitModel, i int, f func(*paneMemo)) {
	v := m.views[i]
	e := paneMemo{size: v.size, weight: v.weight, collapsed: v.collapsed}
	f(&e)
	mem := maps.Clone(r.memo.Peek())
	if mem == nil {
		mem = map[string]paneMemo{}
	}
	mem[v.pane.identity(i)] = e
	r.memo.Set(mem)
}

// setSize sets the size of sized pane i to px logical pixels, in the pane's unit.
func (r *splitRender) setSize(m splitModel, i int, px float64) {
	v := m.views[i]
	px = max(px, 0)
	l := Px(float32(px))
	if v.percent() {
		l = Percent(float32(m.toUnit(v, px)))
	}
	if v.pane.sig != nil {
		v.pane.sig.Set(l)
		return
	}
	r.write(m, i, func(e *paneMemo) { e.size = l })
}

// setPair sets the sizes of the two panes of divider d after its before pane moves by delta logical
// pixels from the sizes sa and sb that the move starts from. A fill pane beside a sized one takes what
// the sized one gives up, through the layout; two fill panes keep their sum of weights, and take the
// new ratio of their sizes.
func (r *splitRender) setPair(m splitModel, d int, sa, sb, delta float64) {
	a, b := m.views[d], m.views[d+1]
	switch {
	case a.sized && b.sized:
		r.setSize(m, d, sa+delta)
		r.setSize(m, d+1, sb-delta)
	case a.sized:
		r.setSize(m, d, sa+delta)
	case b.sized:
		r.setSize(m, d+1, sb-delta)
	default:
		wa, wb := max(a.weight, 0), max(b.weight, 0)
		sum := wa + wb
		if sum == 0 {
			wa, wb, sum = 1, 1, 2
		}
		if pair := sa + sb; pair > 0 {
			wa = sum * max(sa+delta, 0) / pair
			wb = sum - wa
		}
		r.write(m, d, func(e *paneMemo) { e.weight = wa })
		r.write(m, d+1, func(e *paneMemo) { e.weight = wb })
	}
}

// changed runs OnChange after a size changed.
func (r *splitRender) changed() {
	if r.s.onChange != nil {
		r.s.onChange()
	}
}

// move moves divider d by delta logical pixels of its before pane, from the sizes the panes have now,
// and stops it at the limits of the two panes. It runs OnChange if a size changed.
func (r *splitRender) move(d int, delta float64) {
	m := r.model(r.length.Peek())
	if !m.measured || m.inert(d) {
		return
	}
	sa, sb := m.base[d], m.base[d+1]
	dmin, dmax := m.pairRange(d, sa, sb)
	delta = min(max(delta, dmin), dmax)
	if delta == 0 {
		return
	}
	r.setPair(m, d, sa, sb, delta)
	r.changed()
}

// nudge moves divider d by one arrow step in direction dir (-1 or 1), by the step of the pane it
// resizes (see driver), large with Shift.
func (r *splitRender) nudge(d, dir int, large bool) {
	m := r.model(r.length.Peek())
	if !m.measured || m.inert(d) {
		return
	}
	px, _ := m.step(m.views[m.driver(d, dir)], large)
	r.move(d, float64(dir)*px)
}

// dragTo moves the divider that is being dragged to the pointer's position along the split, pos. The
// move is set from the sizes the drag began with, so the divider follows the pointer and stops at the
// limits. It runs OnChange when the position changes.
func (r *splitRender) dragTo(d int, pos float64) {
	dr := r.drag.Peek()
	if !dr.on || dr.d != d {
		return
	}
	m := r.model(r.length.Peek())
	if !m.measured {
		return
	}
	dmin, dmax := m.pairRange(d, dr.sa, dr.sb)
	delta := min(max(pos-dr.at, dmin), dmax)
	if delta == dr.last {
		return
	}
	dr.last = delta
	r.drag.Set(dr)
	r.setPair(m, d, dr.sa, dr.sb, delta)
	r.changed()
}

// setValue moves divider d so that the pane it resizes has the size x, in that pane's unit, within the
// limits.
func (r *splitRender) setValue(d int, x float64) {
	m := r.model(r.length.Peek())
	if !m.measured || m.inert(d) {
		return
	}
	drv := m.driver(d, 1)
	px := m.fromUnit(m.views[drv], x)
	if drv == d {
		r.move(d, px-m.base[d])
	} else {
		r.move(d, m.base[d+1]-px)
	}
}

// toggle collapses or restores the pane that divider d collapses (see collapseOwner).
func (r *splitRender) toggle(d int) {
	i := r.s.collapseOwner(d)
	if i < 0 {
		return
	}
	m := r.model(r.length.Peek())
	v := m.views[i]
	if v.pane.collapsed != nil {
		v.pane.collapsed.Set(!v.collapsed)
		return
	}
	r.write(m, i, func(e *paneMemo) { e.collapsed = !v.collapsed })
}

// sizePane sets the box of pane i of the model m: its size from the layout, or hidden when collapsed.
// Before the split has a length, the panes share it equally, and the layout corrects them once the
// length is known.
func (r *splitRender) sizePane(d *DivElement, m splitModel, i int) {
	if m.views[i].collapsed {
		d.Display(false)
		return
	}
	if !m.measured {
		d.Grow(1).Basis(Px(0))
		return
	}
	l := Px(float32(m.shown[i]))
	if r.vert {
		d.Height(l)
	} else {
		d.Width(l)
	}
	d.Shrink(0).MinWidth(Px(0)).MinHeight(Px(0))
}

// handle returns divider d, between panes d and d+1: a handle as thick as the bar, which holds the bar
// and is the window splitter, focusable, with its value and steps. Its hit area is a child wider than
// the handle, which lies over the panes' edges, drawn and hit above them, and holds the grip; the
// pointer's presses there reach the handle.
func (r *splitRender) handle(st SplitStyle, d int) Element {
	return Component(func(ctx *Context) Element {
		m := r.model(r.length.Get())
		dr := r.drag.Get()
		dragged := dr.on && dr.d == d
		bar := Div().asPart("Split.Divider").Style(st.Divider)
		switch {
		case dragged:
			bar.Style(st.DividerActive)
		case ctx.hovered():
			bar.Style(st.DividerHover)
		}
		if ctx.KeyboardFocus() {
			bar.Style(st.DividerFocusVisible)
		}
		line := float32(r.line)
		hit := Div().asPart("Split.Handle").Shrink(0).Center().ZIndex(1)
		area := Div().Absolute().Center()
		over := (line - splitHit) / 2 // the area's offset from the handle: negative, it reaches past it
		if r.vert {
			hit.Column().Height(Px(line)).Cursor(input.CursorResizeNS)
			area.Left(0).Right(0).Top(over).Height(Px(splitHit)).Cursor(input.CursorResizeNS)
		} else {
			hit.Row().Width(Px(line)).Cursor(input.CursorResizeEW)
			area.Top(0).Bottom(0).Left(over).Width(Px(splitHit)).Cursor(input.CursorResizeEW)
		}
		hit.Child(bar).Child(area)
		if r.s.grip == SplitGripShown || r.s.grip == SplitGripOnHover && (ctx.hovered() || dragged || ctx.KeyboardFocus()) {
			area.Child(Div().asPart("Split.Grip").Style(st.Grip))
		}
		hit.Role(access.RoleSeparator).Label(r.dividerName(ctx, d)).Focusable()
		value, lo, hi, small, large := m.rangeOf(d)
		a := hit.access()
		a.rng = &access.Range{Value: value, Min: lo, Max: hi, SmallChange: small, LargeChange: large}
		a.setRange = func(x float64) { r.setValue(d, x) }
		hit.OnKeyDown(func(e *KeyEvent) {
			if e.Mods&^input.ModShift != 0 {
				return
			}
			shift := e.Mods&input.ModShift != 0
			back, forward := input.KeyLeft, input.KeyRight
			if r.vert {
				back, forward = input.KeyUp, input.KeyDown
			}
			switch e.Key {
			case input.KeyEnter:
				if r.s.collapseOwner(d) < 0 {
					return
				}
				r.toggle(d)
			case back:
				r.nudge(d, -1, shift)
			case forward:
				r.nudge(d, 1, shift)
			case input.KeyHome:
				r.move(d, math.Inf(-1))
			case input.KeyEnd:
				r.move(d, math.Inf(1))
			default:
				return
			}
			e.StopPropagation()
		})
		hit.OnMouseDown(func(e *MouseEvent) {
			if e.Button != input.ButtonLeft {
				return
			}
			m := r.model(r.length.Peek())
			if !m.measured || m.inert(d) {
				return
			}
			r.drag.Set(splitDrag{on: true, d: d, at: r.axis(e), sa: m.base[d], sb: m.base[d+1]})
		}).
			OnMouseMove(func(e *MouseEvent) { r.dragTo(d, r.axis(e)) }).
			OnMouseUp(func(*MouseEvent) {
				if r.drag.Peek().on {
					r.drag.Set(splitDrag{})
				}
				if r.s.onRelease != nil {
					r.s.onRelease()
				}
			}).
			CapturePointer().
			OnDoubleClick(func() { r.toggle(d) })
		return hit
	})
}

// dividerName returns the accessible name of divider d. The split's Label names every divider. Without
// it, a divider whose two panes both have a Label is named after them, as "Resize" followed by the two
// names (see Strings.ResizeBetween); any other divider is the window's Resize panes text.
func (r *splitRender) dividerName(ctx *Context, d int) string {
	if r.s.name != "" {
		return r.s.name
	}
	strs := ctx.Strings()
	first, second := r.s.panes[d].name, r.s.panes[d+1].name
	if first != "" && second != "" {
		return strs.ResizeBetween(first, second)
	}
	return strs.ResizePanes
}

// axis returns the pointer's position along the split.
func (r *splitRender) axis(e *MouseEvent) float64 {
	if r.vert {
		return float64(e.Y)
	}
	return float64(e.X)
}

// watchSplit records that the split rendered at a length of total logical pixels along its
// orientation, so that a change of its length renders it again (see splitsResized).
func (rt *Runtime) watchSplit(inst *componentInstance, key string, vert bool, total float64) {
	if rt.splitWatches == nil {
		rt.splitWatches = map[*componentInstance]*splitWatch{}
	}
	w := rt.splitWatches[inst]
	if w == nil {
		w = &splitWatch{}
		rt.splitWatches[inst] = w
	}
	w.key, w.vert, w.seen = key, vert, total
}

// splitWatch is a split that sizes its panes from its own length. See Runtime.splitsResized. node is
// the split's box as the last check found it, so the check does not search the tree again.
type splitWatch struct {
	key  string
	vert bool
	seen float64 // the length the split last rendered at
	node *node
}

// splitAxis returns the length of the split with key along its orientation, from the last layout. It
// is zero before the split has been laid out.
func (rt *Runtime) splitAxis(key string, vert bool) float64 {
	if n := rt.findKey(rt.tree, key); n != nil {
		return axisLength(n, vert)
	}
	return 0
}

// axisLength returns the length of the node n along the orientation, from its layout.
func axisLength(n *node, vert bool) float64 {
	if vert {
		return float64(n.lay.H)
	}
	return float64(n.lay.W)
}

// splitsResized reports whether a split's length changed in the last layout. Such a split is
// marked for a render, which sizes its panes to the length it has now, before the frame is painted.
// The box of each split is kept, and it is looked up again only when it is no longer live (the
// node left the tree, or was built again) or its key changed, so a check costs one map read per
// split. A split that is not in the tree, such as one parked by a retained panel, is searched for
// on each check until it is back.
func (rt *Runtime) splitsResized() bool {
	changed := false
	for inst, w := range rt.splitWatches {
		if inst.disposed {
			delete(rt.splitWatches, inst)
			continue
		}
		if w.node == nil || rt.byID[w.node.id] != w.node || w.node.key != w.key {
			w.node = rt.findKey(rt.tree, w.key)
		}
		if w.node == nil {
			continue
		}
		if axisLength(w.node, w.vert) != w.seen {
			inst.invalidate()
			changed = true
		}
	}
	return changed
}

func (s *SplitElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		vert := s.vertical()
		st := resolveSplit(ctx, vert, s.styles)
		key := s.key
		if key == "" {
			key = fmt.Sprintf("split:%x", ctx.id)
		}
		r := &splitRender{
			s:      s,
			key:    key,
			vert:   vert,
			length: Local(ctx, "length", 0.0),
			memo:   Local(ctx, "memo", map[string]paneMemo(nil)),
			drag:   Local(ctx, "drag", splitDrag{}),
			line:   dividerLine(st, vert),
		}
		// The split finds its own box by key, and its length is the last layout's. The
		// layout comes after the render, so a split that has not been laid out yet renders
		// with no length, and the runtime renders it again once its length is known.
		total := ctx.rt.splitAxis(key, vert)
		if total != r.length.Peek() {
			r.length.Set(total)
		}
		ctx.rt.watchSplit(ctx.inst, key, vert, total)
		if len(s.panes) == 0 {
			return s.size.apply(Div())
		}
		m := r.model(total)
		root := Div().Key(key).Size(Full, Full).ClipContent()
		if vert {
			root.Column()
		} else {
			root.Row()
		}
		for i, p := range s.panes {
			pane := Div().Key(key + "/pane:" + p.identity(i)).asPart("Split.Pane").Column().Style(st.Pane)
			r.sizePane(pane, m, i)
			pane.Child(p.content)
			if p.name != "" {
				pane.Role(access.RoleGroup).Label(p.name)
			}
			root.Child(pane)
			if i < len(s.panes)-1 {
				root.Child(r.handle(st, i))
			}
		}
		return s.size.apply(root)
	})
}
