package ui

import (
	"slices"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// SplitButtonStyle is the look of each part of a split button. A part's style
// merges over the built-in defaults of its variant, then over the theme's
// SplitButton, then over the Styles of the instance; see docs/styling.md.
//
// Root is the row that joins the two segments, with the shadow of a button. Main
// is the segment that runs the action, with the size, font and colors of a Button
// of the same variant, and Arrow the segment that opens the menu. The two share
// the line between them, and only their outer corners are rounded. Each segment
// has the states of a button: Hover while the pointer is over it, Pressed while it
// is pressed (and, for the arrow, while its menu is open), Disabled when the split
// button is disabled, and FocusVisible while it has keyboard focus (its focus ring
// is added to its shadows). Label and Icon are the action's text and icon, and
// Chevron the arrow's icon.
type SplitButtonStyle struct {
	Root              Style // the row of the two segments
	Main              Style // the segment that runs the action
	MainHover         Style // the action segment while the pointer is over it, if enabled
	MainPressed       Style // the action segment while it is pressed
	MainDisabled      Style // the action segment when the split button is disabled
	MainFocusVisible  Style // the action segment while it has keyboard focus
	Arrow             Style // the segment that opens the menu
	ArrowHover        Style // the arrow segment while the pointer is over it, if enabled
	ArrowPressed      Style // the arrow segment while it is pressed or its menu is open
	ArrowDisabled     Style // the arrow segment when the split button is disabled
	ArrowFocusVisible Style // the arrow segment while it has keyboard focus
	Label             Style // the text of the action
	Icon              Style // the icon of the action
	Chevron           Style // the arrow's icon
}

// merge returns s with the set fields of o applied on top, part by part.
func (s SplitButtonStyle) merge(o SplitButtonStyle) SplitButtonStyle {
	return SplitButtonStyle{
		Root:              s.Root.Merge(o.Root),
		Main:              s.Main.Merge(o.Main),
		MainHover:         s.MainHover.Merge(o.MainHover),
		MainPressed:       s.MainPressed.Merge(o.MainPressed),
		MainDisabled:      s.MainDisabled.Merge(o.MainDisabled),
		MainFocusVisible:  s.MainFocusVisible.Merge(o.MainFocusVisible),
		Arrow:             s.Arrow.Merge(o.Arrow),
		ArrowHover:        s.ArrowHover.Merge(o.ArrowHover),
		ArrowPressed:      s.ArrowPressed.Merge(o.ArrowPressed),
		ArrowDisabled:     s.ArrowDisabled.Merge(o.ArrowDisabled),
		ArrowFocusVisible: s.ArrowFocusVisible.Merge(o.ArrowFocusVisible),
		Label:             s.Label.Merge(o.Label),
		Icon:              s.Icon.Merge(o.Icon),
		Chevron:           s.Chevron.Merge(o.Chevron),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s SplitButtonStyle) plain() SplitButtonStyle {
	return SplitButtonStyle{
		Root:              plainStyle(s.Root),
		Main:              plainStyle(s.Main),
		MainHover:         plainStyle(s.MainHover),
		MainPressed:       plainStyle(s.MainPressed),
		MainDisabled:      plainStyle(s.MainDisabled),
		MainFocusVisible:  plainStyle(s.MainFocusVisible),
		Arrow:             plainStyle(s.Arrow),
		ArrowHover:        plainStyle(s.ArrowHover),
		ArrowPressed:      plainStyle(s.ArrowPressed),
		ArrowDisabled:     plainStyle(s.ArrowDisabled),
		ArrowFocusVisible: plainStyle(s.ArrowFocusVisible),
		Label:             plainStyle(s.Label),
		Icon:              plainStyle(s.Icon),
		Chevron:           plainStyle(s.Chevron),
	}
}

// splitButtonDefaults returns the built-in look of a split button of variant v at
// size, from the colors of t: a Button of that variant cut in two. The shadow is
// the row's, so it is not drawn between the segments. A variant with a border keeps
// it around both segments, and the action segment's right edge is the line between
// them; a variant with no border draws the line on the arrow segment's left edge,
// in a tint of its text color (the text color itself under high contrast).
func splitButtonDefaults(t *Theme, v ButtonVariant, size controlSize) SplitButtonStyle {
	b := buttonDefaults(t, v, size, false)
	r := b.Root.Radius
	main, arrow := b.Root, b.Root
	main.Shadows, arrow.Shadows = nil, nil
	main.Corners = Corners{TopLeft: r, BottomLeft: r}
	arrow.Corners = Corners{TopRight: r, BottomRight: r}
	arrow.Gap = 0
	switch size {
	case sizeSmall:
		arrow.Padding = XY(5, 0)
	case sizeLarge:
		arrow.Padding = XY(9, 0)
	default:
		arrow.Padding = XY(7, 0)
	}
	if w := b.Root.BorderWidth; w > 0 {
		arrow.BorderWidth, arrow.BorderEdges = 0, Edges{Top: w, Right: w, Bottom: w}
	} else {
		line := b.Root.TextColor.Alpha(0.4)
		if t.HighContrast {
			line = b.Root.TextColor
		}
		arrow.BorderEdges, arrow.BorderColor = Edges{Left: 1}, line
	}
	return SplitButtonStyle{
		Root:         Style{Direction: Row, Align: Stretch, NoShrink: true, fitWidth: true, Radius: r, Shadows: b.Root.Shadows},
		Main:         main,
		MainHover:    b.RootHover,
		MainPressed:  b.RootPressed,
		MainDisabled: b.RootDisabled,
		Arrow:        arrow,
		ArrowHover:   b.RootHover,
		// The arrow keeps its pressed look while its menu is open.
		ArrowPressed:  b.RootPressed,
		ArrowDisabled: b.RootDisabled,
		Label:         b.Label,
		Icon:          b.Icon,
	}
}

// resolveSplitButton returns the look of a split button: the defaults of its
// variant, merged with the theme's SplitButton and then the instance's Styles.
// Under high contrast the two overrides are plain (see plainStyle).
func resolveSplitButton(ctx *Context, v ButtonVariant, size controlSize, inst SplitButtonStyle) SplitButtonStyle {
	t := ctx.Theme()
	return layered(ctx, splitButtonDefaults(t, v, size), SplitButtonStyle.merge, SplitButtonStyle.plain,
		themeLayer(t.SplitButton), instanceLayer(inst))
}

// SplitButtonElement is a button with a main action and a menu of related ones,
// such as Save beside Save as and Save a copy: two joined segments, the action and
// an arrow that opens the menu below the button. The action runs on a click, Enter
// or Space. The menu opens on a click on the arrow, on Enter, Space or Down while
// the arrow has focus, and on Alt+Down while either segment has focus. It closes as
// a DropdownMenu's does, and Escape returns the focus to where it was.
//
// Each segment is a Tab stop. Assistive technology sees a group named by the label
// with two buttons: the action, named by the label, and the arrow, named
// Strings.MoreOptions, which reports whether its menu is expanded.
type SplitButtonElement struct {
	label    string
	onClick  func()
	build    func() []MenuItem
	icon     *icons.Icon
	variant  ButtonVariant
	ctlSize  controlSize // see Small and Large
	disabled bool
	tooltip  string
	styles   SplitButtonStyle
	key      string
	handle   *FocusHandle // see FocusHandle
	size     sizing
}

// SplitButton creates a split button that runs onClick for its label, and opens
// the menu that build returns from its arrow. build runs each time the menu opens;
// an empty result opens nothing. The default variant is Primary, as for Button.
func SplitButton(label string, onClick func(), build func() []MenuItem) *SplitButtonElement {
	return &SplitButtonElement{label: label, onClick: onClick, build: build}
}

func (*SplitButtonElement) isElement()           {}
func (b *SplitButtonElement) elementKey() string { return b.key }

// Key identifies the split button among its siblings (see DivElement.Key).
func (b *SplitButtonElement) Key(k string) *SplitButtonElement { b.key = k; return b }

// Icon sets an icon drawn before the label.
func (b *SplitButtonElement) Icon(ic *icons.Icon) *SplitButtonElement { b.icon = ic; return b }

// Variant sets the look of the split button (see ButtonVariant).
func (b *SplitButtonElement) Variant(v ButtonVariant) *SplitButtonElement { b.variant = v; return b }

// Primary sets the look to that of a ButtonPrimary button.
func (b *SplitButtonElement) Primary() *SplitButtonElement { return b.Variant(ButtonPrimary) }

// Secondary sets the look to that of a ButtonSecondary button.
func (b *SplitButtonElement) Secondary() *SplitButtonElement { return b.Variant(ButtonSecondary) }

// Ghost sets the look to that of a ButtonGhost button.
func (b *SplitButtonElement) Ghost() *SplitButtonElement { return b.Variant(ButtonGhost) }

// Danger sets the look to that of a ButtonDanger button.
func (b *SplitButtonElement) Danger() *SplitButtonElement { return b.Variant(ButtonDanger) }

// Small makes the split button use the small control size, as Button.Small does.
func (b *SplitButtonElement) Small() *SplitButtonElement { b.ctlSize = sizeSmall; return b }

// Large makes the split button use the large control size, as Button.Large does.
// Small and Large replace each other.
func (b *SplitButtonElement) Large() *SplitButtonElement { b.ctlSize = sizeLarge; return b }

// Disabled sets whether the split button is disabled: neither the action nor the
// menu can be used.
func (b *SplitButtonElement) Disabled(d bool) *SplitButtonElement { b.disabled = d; return b }

// Tooltip shows s below the split button after the pointer rests on it. Assistive
// technology reads it as the group's description, unless it has another.
func (b *SplitButtonElement) Tooltip(s string) *SplitButtonElement { b.tooltip = s; return b }

// FocusHandle lets code focus the action segment (see NewFocusHandle).
func (b *SplitButtonElement) FocusHandle(h *FocusHandle) *SplitButtonElement { b.handle = h; return b }

// The sizing methods give the split button's outer box a size. It is as wide as its
// label and arrow by default, as a button is; a wider box widens the action
// segment. See Sizing in docs/styling.md.
func (b *SplitButtonElement) Width(l Length) *SplitButtonElement { b.size.setWidth(l); return b }

// MinWidth sets the smallest width of the split button.
func (b *SplitButtonElement) MinWidth(l Length) *SplitButtonElement { b.size.setMinWidth(l); return b }

// MaxWidth sets the largest width of the split button.
func (b *SplitButtonElement) MaxWidth(l Length) *SplitButtonElement { b.size.setMaxWidth(l); return b }

// Grow makes the split button take a share of the free space on its parent's main axis.
func (b *SplitButtonElement) Grow(f float32) *SplitButtonElement { b.size.setGrow(f); return b }

// GridSpan sets how many columns and rows the split button covers in its parent's grid.
func (b *SplitButtonElement) GridSpan(cols, rows int) *SplitButtonElement {
	b.size.setSpan(cols, rows)
	return b
}

// Styles sets the look of the parts of the split button, for this instance only.
// Set fields win over the theme's SplitButton and the defaults of the variant; see
// SplitButtonStyle and docs/styling.md. Calling Styles again merges with the
// earlier values.
func (b *SplitButtonElement) Styles(st SplitButtonStyle) *SplitButtonElement {
	b.styles = b.styles.merge(st)
	return b
}

func (b *SplitButtonElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		rt, t := ctx.rt, ctx.Theme()
		strs := ctx.Strings()
		st := resolveSplitButton(ctx, b.variant, b.ctlSize, b.styles)
		iconSize := buttonIconSize(b.ctlSize)
		open := Local(ctx, "open", false)
		items := Local(ctx, "items", []MenuItem(nil))
		path := Local(ctx, "path", []int{-1})
		closeMenu := func(restore bool) {
			if !open.Peek() {
				return
			}
			open.Set(false)
			rt.menuClosed(restore)
		}
		openMenu := func(keyboard bool) {
			if open.Peek() || b.disabled || b.build == nil {
				return
			}
			built := b.build()
			if len(built) == 0 {
				return
			}
			rt.menuOpened()
			items.Set(built)
			path.Set(highlightPath(built, keyboard))
			open.Set(true)
		}

		// The focus ring of a segment is added to its shadows, as a Button's is.
		ring := Shadow{Spread: 2, Color: t.FocusRing}
		main := Div().asPart("SplitButton.Main").Style(st.Main).Grow(1).Role(access.RoleButton).Label(b.label)
		if b.icon != nil {
			main.Child(Div().asPart("SplitButton.Icon").Style(st.Icon).Child(Icon(b.icon).Size(iconSize)))
		}
		main.Child(Div().asPart("SplitButton.Label").Style(st.Label).Child(Text(b.label).NoWrap()))
		arrow := Div().asPart("SplitButton.Arrow").Style(st.Arrow).Role(access.RoleButton).Label(strs.MoreOptions).
			AccessExpanded(open.Get()).
			Child(Div().asPart("SplitButton.Chevron").Style(st.Chevron).Child(Icon(icons.ChevronDown).Size(iconSize - 2)))

		root := Div().asPart("SplitButton.Root").Style(st.Root).Role(access.RoleGroup).Label(b.label).
			Tooltip(b.tooltip).Child(main).Child(arrow)
		b.size.apply(root)
		if b.disabled {
			main.Style(st.MainDisabled).Cursor(CursorNotAllowed)
			arrow.Style(st.ArrowDisabled).Cursor(CursorNotAllowed)
			main.access().disabled, arrow.access().disabled = true, true
			return root
		}
		main.Hover(st.MainHover).Pressed(st.MainPressed).Focusable().
			FocusVisible(Style{Shadows: append(slices.Clone(st.Main.Shadows), ring)}.Merge(st.MainFocusVisible))
		if b.handle != nil {
			main.FocusHandle(b.handle)
		}
		if b.onClick != nil {
			main.OnClick(b.onClick)
		}
		if open.Get() {
			arrow.Style(st.ArrowPressed)
		} else {
			arrow.Hover(st.ArrowHover)
		}
		arrow.Pressed(st.ArrowPressed).Focusable().
			FocusVisible(Style{Shadows: append(slices.Clone(st.Arrow.Shadows), ring)}.Merge(st.ArrowFocusVisible)).
			OnClick(func() {
				if open.Peek() {
					closeMenu(true)
				} else {
					openMenu(false)
				}
			}).
			OnKeyDown(func(e *KeyEvent) {
				if open.Peek() || e.Repeat || e.Mods != 0 {
					return
				}
				if e.Key == input.KeyEnter || e.Key == input.KeySpace || e.Key == input.KeyDown {
					openMenu(true)
					e.StopPropagation()
				}
			})
		// Alt+Down opens the menu from either segment, as it opens a list.
		root.OnKeyDown(func(e *KeyEvent) {
			if !open.Peek() && !e.Repeat && e.Key == input.KeyDown && e.Mods == input.ModAlt {
				openMenu(true)
				e.StopPropagation()
			}
		})
		if open.Get() {
			m := &menuView{ctx: ctx, t: t, items: items.Get(), path: path, close: closeMenu, label: b.label,
				st: resolveMenu(ctx, MenuStyle{})}
			root.Child(m.top(4))
		}
		return root
	})
}
