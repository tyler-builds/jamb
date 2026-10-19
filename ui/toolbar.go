package ui

import (
	"slices"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// ToolbarStyle is the look of each part of a toolbar. A part's style merges over
// the built-in defaults, then over the theme's Toolbar, then over the Styles of
// the instance; see docs/styling.md. Bar is the row that holds the controls, and
// Separator the line that ToolbarSeparator draws between groups of them. The
// controls keep their own looks.
type ToolbarStyle struct {
	Bar       Style // the row of controls: its spacing, fill and border
	Separator Style // the line between groups of controls
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ToolbarStyle) merge(o ToolbarStyle) ToolbarStyle {
	return ToolbarStyle{
		Bar:       s.Bar.Merge(o.Bar),
		Separator: s.Separator.Merge(o.Separator),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ToolbarStyle) plain() ToolbarStyle {
	return ToolbarStyle{
		Bar:       plainStyle(s.Bar),
		Separator: plainStyle(s.Separator),
	}
}

// toolbarDefaults returns the built-in look of a toolbar, from the colors of t: a
// row of controls 2 pixels apart, on the surface color and outlined in the border
// color, and separators as thin vertical lines in the border color. Under high
// contrast the outline and the separators are the text color.
func toolbarDefaults(t *Theme) ToolbarStyle {
	d := ToolbarStyle{
		Bar: Style{Direction: Row, Align: Center, Gap: 2, Padding: All(4), Radius: t.Radius,
			Background: t.Surface, BorderWidth: 1, BorderColor: t.Border},
		Separator: Style{Width: Px(1), Height: Px(20), NoShrink: true, Margin: XY(4, 0), Background: t.Border},
	}
	if t.HighContrast {
		d.Bar.BorderColor, d.Separator.Background = t.Text, t.Text
	}
	return d
}

// resolveToolbar returns the look of a toolbar: the defaults, merged with the
// theme's Toolbar and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveToolbar(ctx *Context, inst ToolbarStyle) ToolbarStyle {
	t := ctx.Theme()
	return layered(ctx, toolbarDefaults(t), ToolbarStyle.merge, ToolbarStyle.plain,
		themeLayer(t.Toolbar), instanceLayer(inst))
}

// ToolbarElement is a row of controls, such as the formatting buttons above an
// editor. It follows the WAI-ARIA toolbar pattern: the toolbar is one Tab stop,
// and Left and Right move focus between its controls, wrapping at the ends, with
// Home and End to the first and the last. Tab enters it at the control that had
// focus last. A control that uses those keys itself keeps them, such as a text
// field or a slider inside the toolbar. Assistive technology sees a tool bar
// named by its label.
//
// The controls keep their own looks: a toolbar does not restyle them. Buttons
// and toggles in a toolbar are usually Ghost and Small.
type ToolbarElement struct {
	label    string
	children []Element
	key      string
	styles   ToolbarStyle
	size     sizing // the toolbar's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// Toolbar creates a toolbar named label for assistive technology. A toolbar fills
// the width of its container by default, and lays its controls out in a row.
//
//	ui.Toolbar("Formatting").
//		Child(ui.Toggle("Bold", bold).Small()).
//		Child(ui.Toggle("Italic", italic).Small()).
//		Child(ui.ToolbarSeparator()).
//		Child(ui.Button("Undo").Ghost().Small().OnClick(undo))
func Toolbar(label string) *ToolbarElement { return &ToolbarElement{label: label} }

func (*ToolbarElement) isElement()           {}
func (b *ToolbarElement) elementKey() string { return b.key }

// Key identifies the toolbar among its siblings, so it keeps the control it
// remembers when the siblings around it change (see DivElement.Key).
func (b *ToolbarElement) Key(k string) *ToolbarElement { b.key = k; return b }

// Child appends a control, or a ToolbarSeparator.
func (b *ToolbarElement) Child(e Element) *ToolbarElement {
	if !isNil(e) {
		b.children = append(b.children, e)
	}
	return b
}

// Children appends several controls.
func (b *ToolbarElement) Children(es ...Element) *ToolbarElement {
	for _, e := range es {
		b.Child(e)
	}
	return b
}

// The sizing methods give the toolbar's outer box a size. See Sizing in docs/styling.md.
func (b *ToolbarElement) Width(l Length) *ToolbarElement { b.size.setWidth(l); return b }

// MinWidth sets the smallest width of the toolbar.
func (b *ToolbarElement) MinWidth(l Length) *ToolbarElement { b.size.setMinWidth(l); return b }

// MaxWidth sets the largest width of the toolbar.
func (b *ToolbarElement) MaxWidth(l Length) *ToolbarElement { b.size.setMaxWidth(l); return b }

// Grow makes the toolbar take a share of the free space on its parent's main axis.
func (b *ToolbarElement) Grow(f float32) *ToolbarElement { b.size.setGrow(f); return b }

// GridSpan sets how many columns and rows the toolbar covers in its parent's grid.
func (b *ToolbarElement) GridSpan(cols, rows int) *ToolbarElement {
	b.size.setSpan(cols, rows)
	return b
}

// Styles sets the look of the parts of the toolbar, for this instance only. Set
// fields win over the theme's Toolbar and the defaults; see ToolbarStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (b *ToolbarElement) Styles(st ToolbarStyle) *ToolbarElement {
	b.styles = b.styles.merge(st)
	return b
}

func (b *ToolbarElement) expand(ctx *Context) Element {
	label, children, styles, size := b.label, b.children, b.styles, b.size
	return Component(func(ctx *Context) Element {
		rt := ctx.rt
		group := Local(ctx, "group", &rovingGroup{}).Peek()
		st := resolveToolbar(ctx, styles)
		bar := Div().asPart("Toolbar.Bar").Style(st.Bar).Role(access.RoleToolBar).Label(label).
			OnKeyDown(func(e *KeyEvent) {
				if e.Mods == 0 && rt.rovingKey(group, e.Key) {
					e.StopPropagation()
				}
			})
		bar.roving = group
		for _, c := range children {
			if _, ok := c.(toolbarSeparator); ok {
				bar.Child(Div().asPart("Toolbar.Separator").Style(st.Separator).Role(access.RoleSeparator))
				continue
			}
			bar.Child(c)
		}
		return size.apply(bar)
	})
}

// ToolbarSeparator is a thin vertical line between groups of controls in a
// toolbar. Arrow keys pass over it. Assistive technology sees a separator.
func ToolbarSeparator() Element { return toolbarSeparator{} }

type toolbarSeparator struct{}

func (toolbarSeparator) isElement() {}

// expand draws the separator with the theme's look, for one outside a toolbar.
func (toolbarSeparator) expand(ctx *Context) Element {
	st := resolveToolbar(ctx, ToolbarStyle{})
	return Div().asPart("Toolbar.Separator").Style(st.Separator).Role(access.RoleSeparator)
}

// rovingGroup is the state of a group of controls that is one Tab stop, such as
// a toolbar: the arrow keys move focus between its controls (see rovingKey), and
// Tab enters it at the control that had focus last (see rovingStop).
type rovingGroup struct {
	last uint64 // the element inside that had focus last, or 0
}

// rovingFocused tells the groups around the element id that it has focus, so
// they remember it.
func (rt *Runtime) rovingFocused(id uint64) {
	for a := rt.byID[id]; a != nil; a = a.parent {
		if a.div != nil && a.div.roving != nil {
			a.div.roving.last = id
		}
	}
}

// rovingItems returns the elements inside the group n that Tab would reach if
// the group were not one stop, in tree order.
func (rt *Runtime) rovingItems(n *node) []uint64 {
	var ids []uint64
	var walk func(c *node)
	walk = func(c *node) {
		if c.flags&flagNoDisplay != 0 {
			return
		}
		if d := c.div; d != nil && c != n && (d.tabSkip || d.roving != nil) {
			return // a grid's cells, or a group of its own, are not the group's controls
		}
		if d := c.div; d != nil && d.focusable && !d.noTabStop && d.tabIndex >= 0 {
			ids = append(ids, c.id)
		}
		for _, k := range c.children {
			walk(k)
		}
	}
	walk(n)
	return ids
}

// rovingStop returns the element of the group n that Tab stops at: the one with
// focus, or else the one that had it last, or else the first. It returns 0 when
// the group has no control that takes focus.
func (rt *Runtime) rovingStop(n *node) uint64 {
	ids := rt.rovingItems(n)
	if len(ids) == 0 {
		return 0
	}
	if slices.Contains(ids, rt.focusID) {
		return rt.focusID
	}
	if slices.Contains(ids, n.div.roving.last) {
		return n.div.roving.last
	}
	return ids[0]
}

// rovingKey moves focus within group for the key k, when focus is on one of the
// group's controls: Left and Right to the previous and the next control, wrapping
// at the ends, and Home and End to the first and the last. It reports whether
// the key moved focus.
func (rt *Runtime) rovingKey(group *rovingGroup, k input.Key) bool {
	if k != input.KeyLeft && k != input.KeyRight && k != input.KeyHome && k != input.KeyEnd {
		return false
	}
	var n *node
	for a := rt.byID[rt.focusID]; a != nil; a = a.parent {
		if a.div != nil && a.div.roving == group {
			n = a
			break
		}
	}
	if n == nil {
		return false
	}
	ids := rt.rovingItems(n)
	i := slices.Index(ids, rt.focusID)
	if i < 0 {
		return false
	}
	switch k {
	case input.KeyLeft:
		i = (i - 1 + len(ids)) % len(ids)
	case input.KeyRight:
		i = (i + 1) % len(ids)
	case input.KeyHome:
		i = 0
	case input.KeyEnd:
		i = len(ids) - 1
	}
	rt.setFocus(ids[i])
	rt.revealFocus = true
	rt.needPaint = true
	rt.requestFrame()
	return true
}
