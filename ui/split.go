package ui

import (
	"fmt"
	"math"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/layout"
)

// splitHit is the width of the divider's hit area, in logical pixels. The bar is
// drawn in its middle, and the area around it takes the pointer and the focus, so
// the divider is easy to grab even when the bar is a pixel wide.
const splitHit = 9

// The keyboard moves the divider by a small step, and by a large one with Shift. A
// fraction split moves in fractions of its length, and a pixel split in logical
// pixels.
const (
	splitStepFraction, splitLargeFraction = 0.05, 0.1
	splitStepPx, splitLargePx             = 10, 50
)

// SplitStyle is the look of each part of a split. A part's style merges over the
// built-in defaults, then over the theme's Split, then over the Styles of the
// instance; see docs/styling.md.
//
// Divider is the bar drawn in the middle of the divider: its Width is the thickness
// of a horizontal split's divider, and its Height the thickness of a vertical one.
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
	thick, mark := Style{Width: Px(1)}, Style{Width: Px(4), Height: Px(24)}
	if vertical {
		thick, mark = Style{Height: Px(1)}, Style{Width: Px(24), Height: Px(4)}
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
func resolveSplit(t *Theme, vertical bool, inst SplitStyle) SplitStyle {
	st := splitDefaults(t, vertical)
	for _, o := range []SplitStyle{t.Split, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// SplitOrientation says how the panes of a split are laid out.
type SplitOrientation uint8

const (
	SplitHorizontal SplitOrientation = iota // the panes sit side by side (the default)
	SplitVertical                           // the panes are stacked, one above the other
)

// SplitPane names one of the two panes of a split.
type SplitPane uint8

const (
	SplitFirst  SplitPane = iota // the pane on the left, or on top
	SplitSecond                  // the pane on the right, or below
)

// SplitElement lays out two panes with a divider between them that the user drags,
// or moves with the keyboard, to give one pane more of the space. The panes are
// Elements of any kind, so a split can contain another split.
//
// The position is a signal the app owns, so it can be saved and restored. By
// default it is a fraction: the first pane's share of the split's length, from 0 to
// 1, so the panes keep their proportions when the window is resized. PixelPosition
// holds the first pane's size in logical pixels instead, and the second pane takes
// the rest when the window is resized. The limits MinFirst, MaxFirst, MinSecond and
// MaxSecond are logical pixels, and the divider keeps to them while it moves.
//
// The divider is one focusable element with the window splitter role of the
// Accessibility Programmatic Guidelines
// (https://www.w3.org/WAI/ARIA/apg/patterns/windowsplitter/). Its value is the
// position, in the units of the signal, with the limits it can move between. The
// keys are the arrows (Left and Right in a horizontal split, Up and Down in a
// vertical one), with Shift for a larger step, Home and End for the limits, and
// Enter to collapse or restore a pane (see Collapsible). A double-click on the
// divider does the same as Enter.
type SplitElement struct {
	first, second Element
	key           string
	orient        SplitOrientation
	pos           *Signal[float64] // nil: a Local of the split holds the position
	pixels        bool             // pos holds the first pane's size in pixels, not a fraction
	collapsed     *Signal[bool]    // nil: a Local of the split holds the state
	collapsible   bool
	collapsePane  SplitPane
	minFirst      float64
	maxFirst      float64
	minSecond     float64
	maxSecond     float64
	name          string // the accessible name of the divider, see Label
	firstName     string // the accessible name of the first pane, see FirstLabel
	secondName    string // the accessible name of the second pane, see SecondLabel
	onChange      func(float64)
	styles        SplitStyle
	size          sizing // the split's outer box, from the sizing methods (see Sizing in docs/styling.md)
}

// Split creates a split of first and second, with the divider in the middle of
// the space, until a position is given (see Position). The split fills its
// container by default (see Width).
func Split(first, second Element) *SplitElement {
	return &SplitElement{first: first, second: second, maxFirst: math.Inf(1), maxSecond: math.Inf(1)}
}

func (*SplitElement) isElement()           {}
func (s *SplitElement) elementKey() string { return s.key }

// Key gives the split a stable identity among its siblings.
func (s *SplitElement) Key(k string) *SplitElement { s.key = k; return s }

// Orientation sets whether the panes sit side by side (SplitHorizontal, the
// default) or are stacked (SplitVertical).
func (s *SplitElement) Orientation(o SplitOrientation) *SplitElement { s.orient = o; return s }

// Position stores the first pane's share of the split's length in v, from 0 to 1,
// and sets it when the user moves the divider. The app owns the signal, so it can
// save the position and restore it. Without a signal the split keeps its position
// itself, starting at one half.
func (s *SplitElement) Position(v *Signal[float64]) *SplitElement {
	s.pos, s.pixels = v, false
	return s
}

// PixelPosition stores the first pane's size in logical pixels in v, along the
// split's orientation, and sets it when the user moves the divider. While the
// window is resized the first pane keeps its size and the second takes the change.
func (s *SplitElement) PixelPosition(v *Signal[float64]) *SplitElement {
	s.pos, s.pixels = v, true
	return s
}

// Collapsed stores in v whether the pane named by Collapsible is collapsed. The app
// owns the signal, so it can save the state. A collapsed pane is hidden, and its
// content stays in the tree (see Display), so its state survives the collapse. The
// position is kept while the pane is collapsed, so restoring it brings back the
// previous size.
func (s *SplitElement) Collapsed(v *Signal[bool]) *SplitElement { s.collapsed = v; return s }

// Collapsible lets the user collapse the pane p to zero size, by a double-click on
// the divider or Enter on it, and restore it the same way. While a pane is
// collapsed the divider does not move, and its keys and drag wait for the restore.
// See Collapsed for the state.
func (s *SplitElement) Collapsible(p SplitPane) *SplitElement {
	s.collapsible, s.collapsePane = true, p
	return s
}

// splitPx returns the logical pixels of l. The limits of a split are compared with
// its size in pixels, so they take Px lengths only.
func splitPx(method string, l Length) float64 {
	if l.Unit != layout.UnitPx {
		panic("ui.Split." + method + ": the limit is a pixel length, such as ui.Px(200)")
	}
	return float64(l.Value)
}

// MinFirst is the least space the first pane can have, in logical pixels.
func (s *SplitElement) MinFirst(l Length) *SplitElement {
	s.minFirst = splitPx("MinFirst", l)
	return s
}

// MaxFirst is the most space the first pane can have, in logical pixels.
func (s *SplitElement) MaxFirst(l Length) *SplitElement {
	s.maxFirst = splitPx("MaxFirst", l)
	return s
}

// MinSecond is the least space the second pane can have, in logical pixels.
func (s *SplitElement) MinSecond(l Length) *SplitElement {
	s.minSecond = splitPx("MinSecond", l)
	return s
}

// MaxSecond is the most space the second pane can have, in logical pixels.
func (s *SplitElement) MaxSecond(l Length) *SplitElement {
	s.maxSecond = splitPx("MaxSecond", l)
	return s
}

// Label names the divider for assistive technology, such as "Resize sidebar". The
// default is the Resize panes text of the window's Strings.
func (s *SplitElement) Label(name string) *SplitElement { s.name = name; return s }

// FirstLabel names the first pane for assistive technology, such as "Notes". A pane
// without a name is not a group in the accessibility tree.
func (s *SplitElement) FirstLabel(name string) *SplitElement { s.firstName = name; return s }

// SecondLabel names the second pane for assistive technology. See FirstLabel.
func (s *SplitElement) SecondLabel(name string) *SplitElement { s.secondName = name; return s }

// OnChange runs fn with the new position after the user moves the divider, from the
// keyboard or the pointer, to a position that differs from the current one. The
// signal is set first.
func (s *SplitElement) OnChange(fn func(float64)) *SplitElement { s.onChange = fn; return s }

// The sizing methods give the split's outer box a size. A split fills its container
// by default; see Sizing in docs/styling.md.
func (s *SplitElement) Width(l Length) *SplitElement          { s.size.setWidth(l); return s }
func (s *SplitElement) Height(l Length) *SplitElement         { s.size.setHeight(l); return s }
func (s *SplitElement) MinWidth(l Length) *SplitElement       { s.size.setMinWidth(l); return s }
func (s *SplitElement) MinHeight(l Length) *SplitElement      { s.size.setMinHeight(l); return s }
func (s *SplitElement) MaxWidth(l Length) *SplitElement       { s.size.setMaxWidth(l); return s }
func (s *SplitElement) MaxHeight(l Length) *SplitElement      { s.size.setMaxHeight(l); return s }
func (s *SplitElement) Grow(f float32) *SplitElement          { s.size.setGrow(f); return s }
func (s *SplitElement) GridSpan(cols, rows int) *SplitElement { s.size.setSpan(cols, rows); return s }

// Styles sets the look of the parts of the split, for this instance only. Set fields
// win over the theme's Split and the defaults; see SplitStyle and docs/styling.md.
// Calling Styles again merges with the earlier values.
func (s *SplitElement) Styles(st SplitStyle) *SplitElement {
	s.styles = s.styles.merge(st)
	return s
}

// splitDrag is the state of a drag: the pointer's position along the split when the
// drag began, and the first pane's size then, in logical pixels.
type splitDrag struct {
	at float64
	p  float64
}

// splitState is the state of a split as its panes and its divider show it.
type splitState struct {
	shown        float64 // the position the panes show: the position within its limits, or the collapsed end
	lo, hi       float64 // the least and the greatest value the divider can have
	small, large float64 // the steps of the arrow keys, and of Shift with them
	hiddenFirst  bool    // the first pane is collapsed
	hiddenSecond bool    // the second pane is collapsed
}

// vertical reports whether the split stacks its panes.
func (s *SplitElement) vertical() bool { return s.orient == SplitVertical }

// steps returns the arrow step and the Shift step, in the units of the position.
func (s *SplitElement) steps() (small, large float64) {
	if s.pixels {
		return splitStepPx, splitLargePx
	}
	return splitStepFraction, splitLargeFraction
}

// bounds returns the least and the greatest size of the first pane, in logical
// pixels, in a split of total pixels: the limits of both panes, and the divider's
// hit area between them. The greatest is never less than the least.
func (s *SplitElement) bounds(total float64) (lo, hi float64) {
	avail := total - splitHit
	lo = max(s.minFirst, avail-s.maxSecond, 0)
	hi = min(s.maxFirst, avail-s.minSecond)
	return lo, max(hi, lo)
}

// toPx returns the first pane's size in logical pixels for the position v, in a
// split of total pixels.
func (s *SplitElement) toPx(v, total float64) float64 {
	if s.pixels {
		return v
	}
	return v * total
}

// fromPx returns the position for the first pane's size p in logical pixels, in a
// split of total pixels. A split with no length has no fraction, so it is 0.
func (s *SplitElement) fromPx(p, total float64) float64 {
	if s.pixels {
		return p
	}
	if total <= 0 {
		return 0
	}
	return p / total
}

// values returns the least and the greatest position, in the units of the signal.
// Before the split has been laid out a fraction has the whole range.
func (s *SplitElement) values(total float64) (lo, hi float64) {
	if !s.pixels && total <= 0 {
		return 0, 1
	}
	lo, hi = s.bounds(total)
	if !s.pixels {
		lo, hi = lo/total, hi/total
	}
	return lo, hi
}

// state returns the state of the split from its position, its collapse state and
// the length it was last measured at. It reads the signals, so a component that
// calls it renders again when they change.
func (s *SplitElement) state(pos *Signal[float64], collapsed *Signal[bool], total float64) splitState {
	var st splitState
	st.hiddenFirst = s.collapsible && collapsed.Get() && s.collapsePane == SplitFirst
	st.hiddenSecond = s.collapsible && collapsed.Get() && s.collapsePane == SplitSecond
	st.small, st.large = s.steps()
	lo, hi := s.values(total)
	v := pos.Get()
	st.shown = v
	switch {
	case st.hiddenFirst:
		st.shown = s.fromPx(0, total)
	case st.hiddenSecond:
		st.shown = 0
		if total > 0 {
			st.shown = s.fromPx(total-splitHit, total)
		}
	case total > 0:
		st.shown = min(max(v, lo), hi)
	}
	st.lo, st.hi = lo, hi
	if st.hiddenFirst || st.hiddenSecond {
		// A collapsed divider has a single value, so it cannot move.
		st.lo, st.hi = st.shown, st.shown
	}
	return st
}

// length returns the split's length along its orientation, from the last layout.
// It is 0 before the split is laid out.
func (s *SplitElement) length(ctx *Context, key string) float64 {
	r, ok := ctx.rt.Find(key)
	if !ok {
		return 0
	}
	if s.vertical() {
		return float64(r.H)
	}
	return float64(r.W)
}

func (s *SplitElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		vert := s.vertical()
		st := resolveSplit(t, vert, s.styles)
		pos := s.pos
		if pos == nil {
			pos = Local(ctx, "position", 0.5)
		}
		collapsed := s.collapsed
		if collapsed == nil {
			collapsed = Local(ctx, "collapsed", false)
		}
		dragging := Local(ctx, "dragging", false)
		drag := Local(ctx, "drag", splitDrag{})

		// The split finds its own rectangle by key, which is its component's id
		// unless the app gave it a key. The layout of the last frame gives its length.
		key := s.key
		if key == "" {
			key = fmt.Sprintf("split:%x", ctx.id)
		}
		name := s.name
		if name == "" {
			name = ctx.Strings().ResizePanes
		}
		// The layout comes after the render, and while a render builds the tree it finds
		// no layout for the split yet. The last measured length is kept. A split that has
		// not been laid out asks again once the frame has been, and a new length renders
		// the split again, so the range it reports is the current one. A window resize
		// does not render the split, so its range follows the next render.
		known := Local(ctx, "length", 0.0)
		if measured := s.length(ctx, key); measured > 0 && measured != known.Peek() {
			known.Set(measured)
			ctx.Invalidate()
		} else if measured == 0 && known.Peek() == 0 {
			ctx.Post(func() {
				if m := s.length(ctx, key); m > 0 && known.Peek() == 0 {
					known.Set(m)
					ctx.Invalidate()
				}
			})
		}
		total := known.Get()
		ss := s.state(pos, collapsed, total)
		// isCollapsed reports whether the collapsible pane is collapsed, from the signal now.
		isCollapsed := func() bool { return s.collapsible && collapsed.Peek() }

		// setFirst moves the first pane to p logical pixels, within the limits, and
		// stores the position it gives. Nothing moves before the split has a length.
		setFirst := func(p float64, total float64) {
			if total <= 0 || isCollapsed() {
				return
			}
			lo, hi := s.bounds(total)
			nv := s.fromPx(min(max(p, lo), hi), total)
			if nv == pos.Peek() {
				return
			}
			pos.Set(nv)
			if s.onChange != nil {
				s.onChange(nv)
			}
		}
		// move moves the divider by delta, in the units of the position.
		move := func(delta float64) {
			total := s.length(ctx, key)
			setFirst(s.toPx(pos.Peek(), total)+s.toPx(delta, total), total)
		}
		toggle := func() {
			if s.collapsible {
				collapsed.Set(!collapsed.Peek())
			}
		}
		// axis returns the pointer's position along the split.
		axis := func(e *MouseEvent) float64 {
			if vert {
				return float64(e.Y)
			}
			return float64(e.X)
		}

		// The panes. The first is sized by the position, as a percentage of the split
		// in a fraction split and in pixels otherwise. The second takes the rest.
		first := Div().asPart("Split.Pane").Column().Style(st.Pane)
		second := Div().asPart("Split.Pane").Column().Style(st.Pane)
		setMain := func(d *DivElement, size Length, lo, hi float64) {
			maxL := Auto
			if !math.IsInf(hi, 1) {
				maxL = Px(float32(hi))
			}
			if vert {
				d.Height(size).MinHeight(Px(float32(lo))).MaxHeight(maxL)
			} else {
				d.Width(size).MinWidth(Px(float32(lo))).MaxWidth(maxL)
			}
		}
		fill := func(d *DivElement) { d.Grow(1).Basis(Px(0)) }
		switch {
		case ss.hiddenFirst:
			first.Display(false)
			setMain(second, Auto, s.minSecond, s.maxSecond)
			fill(second)
		case ss.hiddenSecond:
			second.Display(false)
			setMain(first, Auto, s.minFirst, s.maxFirst)
			fill(first)
		default:
			size := Px(float32(s.toPx(ss.shown, total)))
			if !s.pixels {
				size = Percent(float32(ss.shown * 100))
			}
			setMain(first, size, s.minFirst, s.maxFirst)
			setMain(second, Auto, s.minSecond, s.maxSecond)
			fill(second)
		}
		first.Child(s.first)
		second.Child(s.second)
		if s.firstName != "" {
			first.Role(access.RoleGroup).Label(s.firstName)
		}
		if s.secondName != "" {
			second.Role(access.RoleGroup).Label(s.secondName)
		}

		// The divider is a component of its own, so that it re-renders when its pointer or
		// focus state changes, and when the split's state does: it reads the same signals as
		// the split. Its focusable element is the hit area; the bar and the grip are drawn
		// over it. It does not take its state from the split's render, which would be
		// captured once and kept while the divider is reused.
		handle := Component(func(ctx *Context) Element {
			ds := s.state(pos, collapsed, known.Get())
			bar := Div().asPart("Split.Divider").Style(st.Divider)
			if dragging.Get() {
				bar.Style(st.DividerActive)
			} else if ctx.hovered() {
				bar.Style(st.DividerHover)
			}
			if ctx.KeyboardFocus() {
				bar.Style(st.DividerFocusVisible)
			}
			hit := Div().asPart("Split.Handle").Shrink(0).Center()
			if vert {
				hit.Column().Height(Px(splitHit)).Cursor(input.CursorResizeNS)
			} else {
				hit.Row().Width(Px(splitHit)).Cursor(input.CursorResizeEW)
			}
			hit.Child(bar).
				Child(Div().Absolute().Inset(0).Center().Child(Div().asPart("Split.Grip").Style(st.Grip)))

			hit.Role(access.RoleSeparator).Label(name).Focusable()
			a := hit.access()
			a.rng = &access.Range{Value: ds.shown, Min: ds.lo, Max: ds.hi, SmallChange: ds.small, LargeChange: ds.large}
			a.setRange = func(x float64) {
				if isCollapsed() {
					return
				}
				total := s.length(ctx, key)
				setFirst(s.toPx(x, total), total)
			}
			hit.OnKeyDown(func(e *KeyEvent) {
				if e.Mods&^input.ModShift != 0 {
					return
				}
				step := ds.small
				if e.Mods&input.ModShift != 0 {
					step = ds.large
				}
				back, forward := input.KeyLeft, input.KeyRight
				if vert {
					back, forward = input.KeyUp, input.KeyDown
				}
				switch e.Key {
				case input.KeyEnter:
					if !s.collapsible {
						return
					}
					toggle()
				case back:
					move(-step)
				case forward:
					move(step)
				case input.KeyHome:
					setFirst(math.Inf(-1), s.length(ctx, key))
				case input.KeyEnd:
					setFirst(math.Inf(1), s.length(ctx, key))
				default:
					return
				}
				e.StopPropagation()
			})
			hit.OnMouseDown(func(e *MouseEvent) {
				if e.Button != input.ButtonLeft || isCollapsed() {
					return
				}
				total := s.length(ctx, key)
				dragging.Set(true)
				drag.Set(splitDrag{at: axis(e), p: s.toPx(pos.Peek(), total)})
			}).
				OnMouseMove(func(e *MouseEvent) {
					if !dragging.Peek() {
						return
					}
					d := drag.Peek()
					setFirst(d.p+(axis(e)-d.at), s.length(ctx, key))
				}).
				OnMouseUp(func(*MouseEvent) { dragging.Set(false) }).
				CapturePointer().
				OnDoubleClick(toggle)
			return hit
		})

		root := Div().Key(key).Size(Full, Full)
		if vert {
			root.Column()
		} else {
			root.Row()
		}
		root.Child(first).Child(handle).Child(second)
		return s.size.apply(root)
	})
}
