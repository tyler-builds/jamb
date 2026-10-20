package ui

import (
	"slices"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// ToggleStyle is the look of each part of a toggle. A part's style merges over
// the built-in defaults of its variant, then over the theme's Toggle, then over
// the Styles of the instance; see docs/styling.md. Root is the button, with the
// size, font and colors of a Button of the same variant. The states are parts of
// their own: RootChecked while the toggle is on, RootHover while the pointer is
// over an enabled toggle, RootPressed while it is pressed, RootDisabled when it
// cannot change, and RootFocusVisible while it has keyboard focus (its focus ring
// is added to the shadows of Root).
type ToggleStyle struct {
	Root             Style // the button: its size, shape, fill, border, font and text color
	RootChecked      Style // the button while the toggle is on
	RootHover        Style // the button while the pointer is over it, if enabled
	RootPressed      Style // the button while it is pressed
	RootDisabled     Style // the button when it is disabled
	RootFocusVisible Style // the button while it has keyboard focus
	Label            Style // the text of the label
	Icon             Style // the icon
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ToggleStyle) merge(o ToggleStyle) ToggleStyle {
	return ToggleStyle{
		Root:             s.Root.Merge(o.Root),
		RootChecked:      s.RootChecked.Merge(o.RootChecked),
		RootHover:        s.RootHover.Merge(o.RootHover),
		RootPressed:      s.RootPressed.Merge(o.RootPressed),
		RootDisabled:     s.RootDisabled.Merge(o.RootDisabled),
		RootFocusVisible: s.RootFocusVisible.Merge(o.RootFocusVisible),
		Label:            s.Label.Merge(o.Label),
		Icon:             s.Icon.Merge(o.Icon),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ToggleStyle) plain() ToggleStyle {
	return ToggleStyle{
		Root:             plainStyle(s.Root),
		RootChecked:      plainStyle(s.RootChecked),
		RootHover:        plainStyle(s.RootHover),
		RootPressed:      plainStyle(s.RootPressed),
		RootDisabled:     plainStyle(s.RootDisabled),
		RootFocusVisible: plainStyle(s.RootFocusVisible),
		Label:            plainStyle(s.Label),
		Icon:             plainStyle(s.Icon),
	}
}

// toggleDefaults returns the built-in look of a toggle of variant v (ButtonGhost
// or ButtonSecondary), on or off, from the colors of t. At rest it is the button
// of that variant, so a toggle sits well beside buttons. On, it has a soft accent
// fill and an accent border, so the state is not carried by the fill alone; the
// weight of the text does not change. Under high contrast an on toggle is
// Highlight with HighlightText, also while hovered or pressed, and a disabled one
// is GrayText and not dimmed.
func toggleDefaults(t *Theme, v ButtonVariant, size controlSize, iconOnly, on, disabled bool) ToggleStyle {
	b := buttonDefaults(t, v, size, iconOnly)
	d := ToggleStyle{Root: b.Root, RootHover: b.RootHover, RootPressed: b.RootPressed, RootDisabled: b.RootDisabled,
		RootFocusVisible: b.RootFocusVisible, Label: b.Label, Icon: b.Icon}
	if d.Root.BorderWidth == 0 {
		// A ghost toggle has a border with no color at rest, so turning it on draws the
		// border without moving the content.
		d.Root.BorderWidth = 1
	}
	d.RootChecked = Style{Background: t.AccentSoft, BorderColor: t.Accent, TextColor: t.Text}
	if on {
		// The pointer over an on toggle keeps its fill and darkens its border.
		d.RootHover = Style{BorderColor: t.AccentHover, TextColor: t.Text}
	}
	if t.HighContrast {
		d.RootChecked = Style{Background: t.Accent, BorderColor: t.Accent, TextColor: t.AccentText}
		if on {
			d.RootHover, d.RootPressed = d.RootChecked, d.RootChecked
			if disabled {
				d.RootDisabled = Style{Background: t.Disabled, TextColor: t.Surface, BorderWidth: 1, BorderColor: t.Disabled}
			}
		}
	}
	return d
}

// resolveToggle returns the look of a toggle: the defaults of its variant and
// state, merged with the theme's Toggle and then the instance's Styles. Under high
// contrast the two overrides are plain (see plainStyle).
func resolveToggle(ctx *Context, v ButtonVariant, size controlSize, iconOnly, on, disabled bool, inst ToggleStyle) ToggleStyle {
	t := ctx.Theme()
	return layered(ctx, toggleDefaults(t, v, size, iconOnly, on, disabled), ToggleStyle.merge, ToggleStyle.plain,
		themeLayer(t.Toggle), instanceLayer(inst))
}

// ToggleElement is a button that stays pressed while it is on, such as a Bold
// button in a toolbar, bound to a bool signal. A click or Space turns it on or
// off. Enter does not: it goes on to the modal's default button, as for a switch.
// Assistive technology reads it as a toggle button with its state.
type ToggleElement struct {
	label       string
	on          *Signal[bool]
	icon        *icons.Icon
	iconOnly    bool
	variant     ButtonVariant
	ctlSize     controlSize // see Small and Large
	key         string
	disabled    bool
	onChange    func(bool)
	styles      ToggleStyle
	size        sizing       // the button's sizes, from the sizing methods (see Sizing in docs/styling.md)
	focusHandle *FocusHandle // see FocusHandle
	tooltip     string       // see Tooltip
}

// Toggle creates a toggle that is on while on is true, with label as its text and
// its accessible name. Its look is that of a Ghost button by default.
func Toggle(label string, on *Signal[bool]) *ToggleElement {
	return &ToggleElement{label: label, on: on, variant: ButtonGhost}
}

func (*ToggleElement) isElement()           {}
func (b *ToggleElement) elementKey() string { return b.key }

// Key identifies the toggle among its siblings (see DivElement.Key).
func (b *ToggleElement) Key(k string) *ToggleElement { b.key = k; return b }

// Icon sets an icon drawn before the label.
func (b *ToggleElement) Icon(ic *icons.Icon) *ToggleElement { b.icon = ic; return b }

// IconOnly draws the icon without the label, as a square button. The label stays
// the toggle's accessible name. A toggle with no icon still shows its label.
func (b *ToggleElement) IconOnly() *ToggleElement { b.iconOnly = true; return b }

// Ghost gives the toggle the look of a ButtonGhost button: no fill or border at
// rest. It is the default.
func (b *ToggleElement) Ghost() *ToggleElement { b.variant = ButtonGhost; return b }

// Secondary gives the toggle the look of a ButtonSecondary button: a surface
// fill and a border at rest.
func (b *ToggleElement) Secondary() *ToggleElement { b.variant = ButtonSecondary; return b }

// Small makes the toggle use the small control size, as Button.Small does.
func (b *ToggleElement) Small() *ToggleElement { b.ctlSize = sizeSmall; return b }

// Large makes the toggle use the large control size, as Button.Large does. Small
// and Large replace each other.
func (b *ToggleElement) Large() *ToggleElement { b.ctlSize = sizeLarge; return b }

// Disabled shows the toggle but does not let it change.
func (b *ToggleElement) Disabled(d bool) *ToggleElement { b.disabled = d; return b }

// OnChange runs fn with the new state after the user turns the toggle on or off.
// The signal is set first.
func (b *ToggleElement) OnChange(fn func(bool)) *ToggleElement { b.onChange = fn; return b }

// FocusHandle lets code focus the toggle (see NewFocusHandle).
func (b *ToggleElement) FocusHandle(h *FocusHandle) *ToggleElement { b.focusHandle = h; return b }

// Tooltip shows s below the toggle after the pointer rests on it. Assistive
// technology reads it as the toggle's description, unless it has another.
func (b *ToggleElement) Tooltip(s string) *ToggleElement { b.tooltip = s; return b }

// The sizing methods give the toggle's outer box a size. A toggle is as wide as
// its label by default, as a button is (see Sizing in docs/styling.md).
func (b *ToggleElement) Width(l Length) *ToggleElement { b.size.setWidth(l); return b }

// Height sets the height of the toggle.
func (b *ToggleElement) Height(l Length) *ToggleElement { b.size.setHeight(l); return b }

// MinWidth sets the smallest width of the toggle.
func (b *ToggleElement) MinWidth(l Length) *ToggleElement { b.size.setMinWidth(l); return b }

// MinHeight sets the smallest height of the toggle.
func (b *ToggleElement) MinHeight(l Length) *ToggleElement { b.size.setMinHeight(l); return b }

// MaxWidth sets the largest width of the toggle.
func (b *ToggleElement) MaxWidth(l Length) *ToggleElement { b.size.setMaxWidth(l); return b }

// MaxHeight sets the largest height of the toggle.
func (b *ToggleElement) MaxHeight(l Length) *ToggleElement { b.size.setMaxHeight(l); return b }

// Grow makes the toggle take a share of the free space on its parent's main axis.
func (b *ToggleElement) Grow(f float32) *ToggleElement { b.size.setGrow(f); return b }

// GridSpan sets how many columns and rows the toggle covers in its parent's grid.
func (b *ToggleElement) GridSpan(cols, rows int) *ToggleElement { b.size.setSpan(cols, rows); return b }

// Styles sets the look of the parts of the toggle, for this instance only. Set
// fields win over the theme's Toggle and the defaults; see ToggleStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (b *ToggleElement) Styles(st ToggleStyle) *ToggleElement {
	b.styles = b.styles.merge(st)
	return b
}

func (b *ToggleElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	on := b.on.Get()
	iconOnly := b.iconOnly && b.icon != nil
	st := resolveToggle(ctx, b.variant, b.ctlSize, iconOnly, on, b.disabled, b.styles)
	d := Div().asPart("Toggle.Root").Style(st.Root)
	if on {
		d.Style(st.RootChecked)
	}
	if b.icon != nil {
		d.Child(Div().asPart("Toggle.Icon").Style(st.Icon).Child(Icon(b.icon).Size(buttonIconSize(b.ctlSize))))
	}
	if !iconOnly {
		d.Child(Div().asPart("Toggle.Label").Style(st.Label).Child(Text(b.label).NoWrap()))
	}
	d.Role(access.RoleButton).Label(b.label).AccessChecked(on)
	d.spaceOnly = true // Enter goes on to the modal default button, as on Windows
	// A tooltip also shows on a disabled toggle, to say why it is disabled.
	d.Tooltip(b.tooltip)
	if b.disabled {
		d.Style(st.RootDisabled).Cursor(CursorNotAllowed)
		d.access().disabled = true
		return b.size.apply(d)
	}
	d.Hover(st.RootHover).Pressed(st.RootPressed)
	if b.focusHandle != nil {
		d.FocusHandle(b.focusHandle)
	}
	// The focus ring is added to the shadows of the button, as a Button's is.
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	shadows := st.Root.Shadows
	if on && st.RootChecked.Shadows != nil {
		shadows = st.RootChecked.Shadows
	}
	focus := Style{Shadows: append(slices.Clone(shadows), ring)}.Merge(st.RootFocusVisible)
	return b.size.apply(d.Cursor(CursorPointer).Focusable().FocusVisible(focus).
		OnClick(func() {
			b.on.Set(!on)
			if b.onChange != nil {
				b.onChange(!on)
			}
		}))
}

// ToggleGroupStyle is the look of each part of a toggle group. A part's style
// merges over the built-in defaults, then over the theme's ToggleGroup, then over
// the Styles of the instance; see docs/styling.md. Group is the joined row of
// items, which draws the outer border. Each option is an Item, with its Icon and
// its Label; an item after the first draws the line between it and the one before
// it, and the first and last items round their outer corners to fit the Group.
// The states are parts of their own: ItemSelected for the selected item,
// ItemHover while the pointer is over an enabled item that is not selected,
// ItemPressed while an item is pressed, ItemFocusVisible while an item has
// keyboard focus, ItemDisabled for an item that cannot be chosen, and
// GroupInvalid when the group's value is not valid (see Field).
type ToggleGroupStyle struct {
	Group            Style // the row of items, with the outer border
	GroupInvalid     Style // the row when the group's value is not valid (see Field)
	Item             Style // each option: its size, padding, font and text color
	ItemSelected     Style // the selected option
	ItemHover        Style // an option under the pointer, if enabled and not selected
	ItemPressed      Style // an option while it is pressed
	ItemFocusVisible Style // an option that has keyboard focus
	ItemDisabled     Style // an option that cannot be chosen
	Label            Style // the text of each option
	Icon             Style // the icon of each option
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ToggleGroupStyle) merge(o ToggleGroupStyle) ToggleGroupStyle {
	return ToggleGroupStyle{
		Group:            s.Group.Merge(o.Group),
		GroupInvalid:     s.GroupInvalid.Merge(o.GroupInvalid),
		Item:             s.Item.Merge(o.Item),
		ItemSelected:     s.ItemSelected.Merge(o.ItemSelected),
		ItemHover:        s.ItemHover.Merge(o.ItemHover),
		ItemPressed:      s.ItemPressed.Merge(o.ItemPressed),
		ItemFocusVisible: s.ItemFocusVisible.Merge(o.ItemFocusVisible),
		ItemDisabled:     s.ItemDisabled.Merge(o.ItemDisabled),
		Label:            s.Label.Merge(o.Label),
		Icon:             s.Icon.Merge(o.Icon),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ToggleGroupStyle) plain() ToggleGroupStyle {
	return ToggleGroupStyle{
		Group:            plainStyle(s.Group),
		GroupInvalid:     plainStyle(s.GroupInvalid),
		Item:             plainStyle(s.Item),
		ItemSelected:     plainStyle(s.ItemSelected),
		ItemHover:        plainStyle(s.ItemHover),
		ItemPressed:      plainStyle(s.ItemPressed),
		ItemFocusVisible: plainStyle(s.ItemFocusVisible),
		ItemDisabled:     plainStyle(s.ItemDisabled),
		Label:            plainStyle(s.Label),
		Icon:             plainStyle(s.Icon),
	}
}

// toggleGroupDefaults returns the built-in look of a toggle group, from the colors
// of t. The row is a Secondary button split into items, at the height of a
// button of the same size, and the selected item is filled with Accent, as the
// selected tab of TabsSegmented is. Under high contrast the lines are the text
// color, the selected item is Highlight with HighlightText, and a disabled item is
// GrayText and not dimmed.
func toggleGroupDefaults(t *Theme, size controlSize, iconOnly bool) ToggleGroupStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	h, _, fs := size.field()
	px := float32(14)
	switch size {
	case sizeSmall:
		px = 10
	case sizeLarge:
		px = 18
	}
	if iconOnly {
		px = (h - 2 - buttonIconSize(size)) / 2
	}
	line := t.BorderStrong
	if t.HighContrast {
		line = t.Text
	}
	d := ToggleGroupStyle{
		Group: Style{Direction: Row, Align: Stretch, NoShrink: true, fitWidth: true, Radius: t.Radius,
			Background: t.Surface, BorderWidth: 1, BorderColor: line},
		GroupInvalid: Style{BorderColor: messageIconColor(t, Style{})},
		// The group's border takes 1 px above and below, so an item is 2 px shorter
		// than a button and the group is as tall as one.
		Item: Style{Direction: Row, Align: Center, Justify: Center, Gap: 7, MinHeight: Px(h - 2),
			Padding: XY(px, 0), FontSize: fs, FontWeight: FontMedium, TextColor: t.Text, BorderColor: line,
			NoShrink: true, Transition: t.Motion.hover(), Spring: t.Motion.Spring},
		ItemSelected:     Style{Background: t.Accent, TextColor: t.AccentText},
		ItemHover:        Style{Background: t.SurfaceHover},
		ItemPressed:      Style{Background: t.SurfaceActive},
		ItemFocusVisible: Style{Shadows: []Shadow{ring}},
		ItemDisabled:     Style{Opacity: 0.5},
	}
	if t.HighContrast {
		d.ItemDisabled = Style{Opacity: 1, TextColor: t.Disabled}
	} else {
		d.Group.Shadows = []Shadow{ShadowSm}
	}
	return d
}

// resolveToggleGroup returns the look of a toggle group: the defaults, merged with
// the theme's ToggleGroup and then the instance's Styles. Under high contrast the
// two overrides are plain (see plainStyle).
func resolveToggleGroup(ctx *Context, size controlSize, iconOnly bool, inst ToggleGroupStyle) ToggleGroupStyle {
	t := ctx.Theme()
	return layered(ctx, toggleGroupDefaults(t, size, iconOnly), ToggleGroupStyle.merge, ToggleGroupStyle.plain,
		themeLayer(t.ToggleGroup), instanceLayer(inst))
}

// ToggleGroupElement is a row of joined toggles, one per option, of which one is
// selected, bound to a value. The selected option is the one whose value equals
// the signal's. It is a radio group for the keyboard and for assistive
// technology: the group is one Tab stop, at the selected option, and the arrow
// keys move to the next option and select it. Up and Left go back; Down and Right
// go on. They wrap around and skip disabled options. Space chooses the focused
// option; Enter goes on to the modal's default button or the form.
type ToggleGroupElement[T comparable] struct {
	value       *Signal[T]
	options     []Option[T]
	name        string // the group's accessible name, see Label
	labelledBy  []string
	key         string
	disabled    bool
	iconOnly    bool        // see IconOnly
	ctlSize     controlSize // see Small and Large
	onChange    func(T)
	styles      ToggleGroupStyle
	rules       []Validator[T] // the validators of the value (see Validate)
	wire        fieldWire      // what a Field gives the group (see Field)
	size        sizing         // the group's sizes, from the sizing methods (see Sizing in docs/styling.md)
	focusHandle *FocusHandle   // see FocusHandle
	tooltip     string         // see Tooltip
}

// ToggleGroup creates a row of joined toggles that reads and writes value, one
// per option.
func ToggleGroup[T comparable](value *Signal[T], options ...Option[T]) *ToggleGroupElement[T] {
	return &ToggleGroupElement[T]{value: value, options: options}
}

func (*ToggleGroupElement[T]) isElement()           {}
func (g *ToggleGroupElement[T]) elementKey() string { return g.key }

// Key identifies the toggle group among its siblings (see DivElement.Key).
func (g *ToggleGroupElement[T]) Key(k string) *ToggleGroupElement[T] { g.key = k; return g }

// Label names the group for assistive technology, such as "Alignment". The
// group has no visible name of its own, so give it one.
func (g *ToggleGroupElement[T]) Label(s string) *ToggleGroupElement[T] { g.name = s; return g }

// LabelledBy names the group by the text of the elements with the given keys,
// such as a visible label beside it (see DivElement.LabelledBy).
func (g *ToggleGroupElement[T]) LabelledBy(keys ...string) *ToggleGroupElement[T] {
	g.labelledBy = keys
	return g
}

// IconOnly draws the icon of each option without its label, as square items. The
// label stays the option's accessible name. An option with no icon still shows
// its label.
func (g *ToggleGroupElement[T]) IconOnly() *ToggleGroupElement[T] { g.iconOnly = true; return g }

// Small makes the group use the small control size, as Button.Small does.
func (g *ToggleGroupElement[T]) Small() *ToggleGroupElement[T] { g.ctlSize = sizeSmall; return g }

// Large makes the group use the large control size, as Button.Large does. Small
// and Large replace each other.
func (g *ToggleGroupElement[T]) Large() *ToggleGroupElement[T] { g.ctlSize = sizeLarge; return g }

// Disabled shows every option but lets none of them be chosen.
func (g *ToggleGroupElement[T]) Disabled(d bool) *ToggleGroupElement[T] { g.disabled = d; return g }

// OnChange runs fn with the chosen value after the user selects an option, by a
// click, Space or the arrow keys. The signal is set first.
func (g *ToggleGroupElement[T]) OnChange(fn func(T)) *ToggleGroupElement[T] {
	g.onChange = fn
	return g
}

// Validate sets the validators of the chosen value. The zero value is no choice,
// so Required[T] rejects it (see Field).
func (g *ToggleGroupElement[T]) Validate(rules ...Validator[T]) *ToggleGroupElement[T] {
	g.rules = rules
	return g
}

// formCheck and formWire make a toggle group a formControl (see Field).
func (g *ToggleGroupElement[T]) formCheck() error { return firstError(g.value.Get(), g.rules) }

func (g *ToggleGroupElement[T]) formWire(w fieldWire) Element {
	c := *g
	c.wire = w
	return &c
}

// FocusHandle lets code focus the group's tab stop: the selected option, or the
// first enabled one (see NewFocusHandle). A Field that wraps the control gives it
// its own handle, which takes precedence.
func (g *ToggleGroupElement[T]) FocusHandle(h *FocusHandle) *ToggleGroupElement[T] {
	g.focusHandle = h
	return g
}

// Tooltip shows s below the group after the pointer rests on it. Assistive
// technology reads it as the group's description, unless it has another.
func (g *ToggleGroupElement[T]) Tooltip(s string) *ToggleGroupElement[T] { g.tooltip = s; return g }

// The sizing methods give the group's outer box a size. A group is as wide as its
// items by default; see Sizing in docs/styling.md.
func (g *ToggleGroupElement[T]) Width(l Length) *ToggleGroupElement[T] { g.size.setWidth(l); return g }

// MinWidth sets the smallest width of the toggle group.
func (g *ToggleGroupElement[T]) MinWidth(l Length) *ToggleGroupElement[T] {
	g.size.setMinWidth(l)
	return g
}

// MaxWidth sets the largest width of the toggle group.
func (g *ToggleGroupElement[T]) MaxWidth(l Length) *ToggleGroupElement[T] {
	g.size.setMaxWidth(l)
	return g
}

// Grow makes the toggle group take a share of the free space on its parent's main axis.
func (g *ToggleGroupElement[T]) Grow(f float32) *ToggleGroupElement[T] { g.size.setGrow(f); return g }

// GridSpan sets how many columns and rows the toggle group covers in its parent's grid.
func (g *ToggleGroupElement[T]) GridSpan(cols, rows int) *ToggleGroupElement[T] {
	g.size.setSpan(cols, rows)
	return g
}

// Styles sets the look of the parts of the toggle group, for this instance only.
// Set fields win over the theme's ToggleGroup and the defaults; see
// ToggleGroupStyle and docs/styling.md. Calling Styles again merges with the
// earlier values.
func (g *ToggleGroupElement[T]) Styles(st ToggleGroupStyle) *ToggleGroupElement[T] {
	g.styles = g.styles.merge(st)
	return g
}

func (g *ToggleGroupElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		cur := g.value.Get()
		// One focus handle per option, kept across renders, so the arrow keys can
		// move focus to the next option.
		handles := Local(ctx, "handles", newHandles(len(g.options))).Peek()
		stop := tabStop(g.options, cur, g.disabled)
		iconOnly := g.iconOnly && slices.ContainsFunc(g.options, func(o Option[T]) bool { return o.Icon != nil })
		st := resolveToggleGroup(ctx, g.ctlSize, iconOnly, g.styles)
		w := g.wire
		handleOf := func(j int) *FocusHandle {
			if j == stop && w.handle != nil {
				return w.handle
			}
			if j == stop && g.focusHandle != nil {
				return g.focusHandle
			}
			return handles[j]
		}

		row := Div().asPart("ToggleGroup.Group").Style(st.Group).Role(access.RoleGroup).SelectionContainer().Tooltip(g.tooltip)
		name, labelledBy := g.name, g.labelledBy
		if w.label != "" {
			name, labelledBy = w.label, w.labelledBy
		}
		if name != "" {
			row.Label(name)
		}
		if len(labelledBy) > 0 {
			row.LabelledBy(labelledBy...)
		}
		if w.description != "" {
			row.Description(w.description)
		}
		row.access().invalid, row.access().required = w.invalid, w.required
		if w.invalid {
			row.Style(st.GroupInvalid)
		}
		// The first and last items round their outer corners inside the group's border.
		inner := max(st.Group.Radius-st.Group.BorderWidth, 0)
		last := len(g.options) - 1
		for i, o := range g.options {
			on := o.Value == cur
			dis := g.disabled || o.Disabled
			item := Div().asPart("ToggleGroup.Item").Style(st.Item)
			var c Corners
			if i == 0 {
				c.TopLeft, c.BottomLeft = inner, inner
			}
			if i == last {
				c.TopRight, c.BottomRight = inner, inner
			}
			if i > 0 {
				item.Style(Style{BorderEdges: Edges{Left: 1}})
			}
			if !c.isZero() {
				item.Style(Style{Corners: c})
			}
			if on {
				item.Style(st.ItemSelected)
			}
			if o.Icon != nil {
				item.Child(Div().asPart("ToggleGroup.Icon").Style(st.Icon).Child(Icon(o.Icon).Size(buttonIconSize(g.ctlSize))))
			}
			if !iconOnly || o.Icon == nil {
				item.Child(Div().asPart("ToggleGroup.Label").Style(st.Label).Child(Text(o.Label).NoWrap()))
			}
			if g.key != "" {
				item.Keyf("%s/%v", g.key, o.Value)
			}
			item.Role(access.RoleRadioButton).Label(o.Label).AccessChecked(on)
			if dis {
				item.access().disabled = true
				item.Cursor(CursorNotAllowed).Style(st.ItemDisabled)
				row.Child(item)
				continue
			}
			if !on {
				item.Hover(st.ItemHover)
			}
			item.Pressed(st.ItemPressed)
			item.Cursor(CursorPointer).Focusable().FocusHandle(handleOf(i)).FocusVisible(st.ItemFocusVisible).
				OnClick(func() { g.choose(g.options[i].Value) }).
				OnKeyDown(func(e *KeyEvent) {
					if e.Mods != 0 {
						return
					}
					dir := 0
					switch e.Key {
					case input.KeyUp, input.KeyLeft:
						dir = -1
					case input.KeyDown, input.KeyRight:
						dir = 1
					default:
						return
					}
					e.StopPropagation()
					if j := nextEnabled(g.options, i, dir, g.disabled); j >= 0 {
						g.choose(g.options[j].Value)
						handleOf(j).Focus()
					}
				})
			if w.onBlur != nil {
				item.OnBlur(w.onBlur)
			}
			item.spaceOnly = true // Enter goes on to the modal default button, as on Windows
			if i != stop {
				item.TabStop(false) // reached with the arrow keys, not Tab
			}
			row.Child(item)
		}
		return g.size.apply(row)
	})
}

// choose sets the value to v and runs OnChange.
func (g *ToggleGroupElement[T]) choose(v T) {
	g.value.Set(v)
	if g.wire.onEdit != nil {
		g.wire.onEdit()
	}
	if g.onChange != nil {
		g.onChange(v)
	}
}
