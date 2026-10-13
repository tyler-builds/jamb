package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// CollapsibleStyle is the look of each part of a collapsible section. A part's
// style merges over the built-in defaults, then over the theme's Collapsible,
// then over the Styles of the instance; see docs/styling.md. Section is the
// frame around the header and the content. Header is the row that opens and
// closes the section, Chevron its arrow, and Content the area under it. The
// states are parts of their own: HeaderOpen while the section is open,
// HeaderHover while the pointer is over an enabled header, HeaderFocusVisible
// while the header has keyboard focus, and HeaderDisabled when the section
// cannot be opened or closed.
type CollapsibleStyle struct {
	Section            Style // the frame around the header and the content
	Header             Style // the header row, which opens and closes the section
	HeaderHover        Style // the header while the pointer is over an enabled one
	HeaderOpen         Style // the header while the section is open
	HeaderFocusVisible Style // the header while it has keyboard focus
	HeaderDisabled     Style // the header when the section is disabled
	Chevron            Style // the arrow at the start of the header
	Content            Style // the area under the header, while the section is open
}

// merge returns s with the set fields of o applied on top, part by part.
func (s CollapsibleStyle) merge(o CollapsibleStyle) CollapsibleStyle {
	return CollapsibleStyle{
		Section:            s.Section.Merge(o.Section),
		Header:             s.Header.Merge(o.Header),
		HeaderHover:        s.HeaderHover.Merge(o.HeaderHover),
		HeaderOpen:         s.HeaderOpen.Merge(o.HeaderOpen),
		HeaderFocusVisible: s.HeaderFocusVisible.Merge(o.HeaderFocusVisible),
		HeaderDisabled:     s.HeaderDisabled.Merge(o.HeaderDisabled),
		Chevron:            s.Chevron.Merge(o.Chevron),
		Content:            s.Content.Merge(o.Content),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s CollapsibleStyle) plain() CollapsibleStyle {
	return CollapsibleStyle{
		Section:            plainStyle(s.Section),
		Header:             plainStyle(s.Header),
		HeaderHover:        plainStyle(s.HeaderHover),
		HeaderOpen:         plainStyle(s.HeaderOpen),
		HeaderFocusVisible: plainStyle(s.HeaderFocusVisible),
		HeaderDisabled:     plainStyle(s.HeaderDisabled),
		Chevron:            plainStyle(s.Chevron),
		Content:            plainStyle(s.Content),
	}
}

// collapsibleDefaults returns the built-in look of a collapsible section, from
// the colors of t. Under high contrast a disabled header is GrayText, not dimmed.
func collapsibleDefaults(t *Theme) CollapsibleStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := CollapsibleStyle{
		Section: Style{Direction: Column, BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius},
		Header: Style{Direction: Row, Align: Center, Gap: 8, MinHeight: Px(40), Padding: XY(12, 0),
			NoShrink: true, TextColor: t.Text, FontWeight: FontMedium},
		HeaderHover:        Style{Background: t.SurfaceHover},
		HeaderFocusVisible: Style{Shadows: []Shadow{ring}},
		Chevron:            Style{TextColor: t.TextMuted},
		Content:            Style{Padding: Edges{Top: 4, Right: 12, Bottom: 12, Left: 12}},
	}
	if t.HighContrast {
		d.HeaderDisabled = Style{TextColor: t.Disabled}
	} else {
		d.HeaderDisabled = Style{Opacity: 0.5}
	}
	return d
}

// resolveCollapsible returns the look of a collapsible section: the defaults,
// merged with the theme's Collapsible and then the instance's Styles. Under high
// contrast the two overrides are plain (see plainStyle).
func resolveCollapsible(t *Theme, inst CollapsibleStyle) CollapsibleStyle {
	st := collapsibleDefaults(t)
	for _, o := range []CollapsibleStyle{t.Collapsible, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// CollapsibleElement is a section with a header row, which opens and closes the
// content under it. The header is a button: Space and Enter open and close it, as
// a click does, and its chevron points right when closed and down when open.
// The content is not built while the section is closed.
//
// The header's accessible name is the title. The header carries the Expanded
// state, so assistive technology can open and close it; the content is a group
// named by the header. Microsoft's guidance is that an expander's header is what
// a user activates and that its content is the region it shows
// (https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/expander).
type CollapsibleElement struct {
	title        string
	open         *Signal[bool]
	content      Element
	key          string
	disabled     bool
	keep         bool // see KeepMounted
	onChange     func(bool)
	styles       CollapsibleStyle
	renderHeader func(title string, state PartState) Element
	// group is set by an Accordion, and runs after a change made by the header.
	group func(open bool)
	size  sizing // the section's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// Collapsible creates a section titled title, whose content shows while open is
// true. The content is shown below the header when the section is open.
func Collapsible(title string, open *Signal[bool], content Element) *CollapsibleElement {
	return &CollapsibleElement{title: title, open: open, content: content}
}

func (*CollapsibleElement) isElement()           {}
func (c *CollapsibleElement) elementKey() string { return c.key }

// Key gives the section a stable identity. It also names the header, which the
// content is labelled by, so give two sections with the same title different keys.
func (c *CollapsibleElement) Key(k string) *CollapsibleElement { c.key = k; return c }

// Disabled shows the section but does not let it open or close.
func (c *CollapsibleElement) Disabled(d bool) *CollapsibleElement { c.disabled = d; return c }

// KeepMounted keeps the content once the section has been opened. The content is
// built the first time the section opens, as it is by default, and then stays in
// the tree while the section is closed, hidden with Display. Its state survives
// closing and opening the section. Without KeepMounted the content is discarded
// when the section closes.
func (c *CollapsibleElement) KeepMounted() *CollapsibleElement { c.keep = true; return c }

// OnChange runs fn with the new state after the user opens or closes the section.
// The signal is set first.
func (c *CollapsibleElement) OnChange(fn func(open bool)) *CollapsibleElement {
	c.onChange = fn
	return c
}

// The sizing methods give the section's outer box a width. A section fills its
// container by default; see Sizing in docs/styling.md.
func (c *CollapsibleElement) Width(l Length) *CollapsibleElement    { c.size.setWidth(l); return c }
func (c *CollapsibleElement) MinWidth(l Length) *CollapsibleElement { c.size.setMinWidth(l); return c }
func (c *CollapsibleElement) MaxWidth(l Length) *CollapsibleElement { c.size.setMaxWidth(l); return c }
func (c *CollapsibleElement) Grow(f float32) *CollapsibleElement    { c.size.setGrow(f); return c }
func (c *CollapsibleElement) GridSpan(cols, rows int) *CollapsibleElement {
	c.size.setSpan(cols, rows)
	return c
}

// Styles sets the look of the parts of the section, for this instance only. Set
// fields win over the theme's Collapsible and the defaults; see CollapsibleStyle
// and docs/styling.md. Calling Styles again merges with the earlier values.
func (c *CollapsibleElement) Styles(st CollapsibleStyle) *CollapsibleElement {
	c.styles = c.styles.merge(st)
	return c
}

// RenderHeader replaces the title in the header with what fn returns. The chevron
// stays, so the open state is still shown, and the header keeps its behavior,
// focus and accessible name, which is still the title. fn is called again when
// the header's pointer or focus state changes, with the state in the PartState
// (Selected means the section is open). The content is hidden from assistive
// technology.
//
//	ui.Collapsible("Sync", open, body).RenderHeader(func(title string, s ui.PartState) ui.Element {
//		return ui.Div().Row().Gap(8).Child(ui.Text(title)).Child(ui.Badge(3))
//	})
func (c *CollapsibleElement) RenderHeader(fn func(title string, state PartState) Element) *CollapsibleElement {
	c.renderHeader = fn
	return c
}

// headerKey is the key of the header, which the content is labelled by.
func (c *CollapsibleElement) headerKey() string {
	if c.key != "" {
		return c.key + "/header"
	}
	return "collapsible:" + c.title
}

// set opens or closes the section and runs OnChange, then the accordion's hook.
func (c *CollapsibleElement) set(open bool) {
	c.open.Set(open)
	if c.onChange != nil {
		c.onChange(open)
	}
	if c.group != nil {
		c.group(open)
	}
}

func (c *CollapsibleElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	on := c.open.Get()
	st := resolveCollapsible(t, c.styles)
	chevron := icons.ChevronRight
	if on {
		chevron = icons.ChevronDown
	}
	// The header is the top of the section, inside its border, so its corners are the
	// section's inner corners: its hover fill then does not cover the border at a rounded
	// corner. The bottom corners are square while the section is open, since the content
	// follows the header.
	corners := innerCorners(st.Section.radii(), st.Section.borderEdges())
	if on {
		corners.BottomLeft, corners.BottomRight = 0, 0
	}

	// draw returns the header in the state ps describes. The chevron is always
	// drawn; the title is the slot's content when RenderHeader is set.
	draw := func(ps PartState) *DivElement {
		header := Div().asPart("Collapsible.Header").Key(c.headerKey()).Style(st.Header).
			Style(Style{Corners: corners}).
			Role(access.RoleButton).Label(c.title).AccessExpanded(on).
			Child(Div().asPart("Collapsible.Chevron").Style(st.Chevron).Child(Icon(chevron).Size(14)))
		if on {
			header.Style(st.HeaderOpen)
		}
		if c.renderHeader != nil {
			header.Child(Div().AccessHidden().Child(c.renderHeader(c.title, ps)))
		} else {
			header.Child(Text(c.title))
		}
		if c.disabled {
			header.access().disabled = true
			header.Style(st.HeaderDisabled).Cursor(CursorNotAllowed)
		} else {
			header.Cursor(CursorPointer).Hover(st.HeaderHover).
				Focusable().FocusVisible(st.HeaderFocusVisible).
				OnClick(func() { c.set(!on) })
		}
		return header
	}
	var header Element
	if c.renderHeader == nil {
		header = draw(PartState{})
	} else {
		// With a render slot the header is its own component, so that it re-renders
		// when its pointer or focus state changes, and the slot gets the new state.
		header = Component(func(ctx *Context) Element {
			return draw(PartState{Hovered: ctx.hovered(), Pressed: ctx.pressed(), Focused: ctx.isFocused(),
				Selected: on, Disabled: c.disabled, HighContrast: t.HighContrast})
		}).Key(c.headerKey())
	}

	col := Div().asPart("Collapsible.Section").Column().Style(st.Section).Child(header)
	if c.key != "" {
		col.Key(c.key)
	}
	// With KeepMounted the content is built once the section has opened, and is
	// then kept hidden while the section is closed.
	kept := false
	if c.keep {
		opened := Local(ctx, "opened", false)
		if on && !opened.Peek() {
			opened.Set(true)
		}
		kept = opened.Peek()
	}
	if (on || kept) && c.content != nil {
		col.Child(Div().asPart("Collapsible.Content").Column().Display(on).Role(access.RoleGroup).Label(c.title).LabelledBy(c.headerKey()).
			Style(st.Content).Child(c.content))
	}
	return c.size.apply(col)
}

// AccordionElement is a group of collapsible sections. By default each section
// opens and closes on its own. SingleOpen makes opening one section close the
// others, as Microsoft's expander guidance describes for groups of expanders
// (https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/expander),
// and leaves the choice to the app: the default is independent sections.
//
// The group draws no part of its own. Its sections are Collapsibles, styled by
// Theme.Collapsible and by the Styles of each section.
type AccordionElement struct {
	sections []*CollapsibleElement
	key      string
	single   bool
	size     sizing // the group's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// Accordion creates a group of sections, shown in order.
func Accordion(sections ...*CollapsibleElement) *AccordionElement {
	return &AccordionElement{sections: sections}
}

func (*AccordionElement) isElement()           {}
func (a *AccordionElement) elementKey() string { return a.key }

// Key gives the group a stable identity among its siblings.
func (a *AccordionElement) Key(k string) *AccordionElement { a.key = k; return a }

// The sizing methods give the group's outer box a width, as for a Collapsible.
func (a *AccordionElement) Width(l Length) *AccordionElement    { a.size.setWidth(l); return a }
func (a *AccordionElement) MinWidth(l Length) *AccordionElement { a.size.setMinWidth(l); return a }
func (a *AccordionElement) MaxWidth(l Length) *AccordionElement { a.size.setMaxWidth(l); return a }
func (a *AccordionElement) Grow(f float32) *AccordionElement    { a.size.setGrow(f); return a }
func (a *AccordionElement) GridSpan(cols, rows int) *AccordionElement {
	a.size.setSpan(cols, rows)
	return a
}

// SingleOpen makes opening a section close the others. A section that is closed
// stays closed, so the group can be closed entirely.
func (a *AccordionElement) SingleOpen() *AccordionElement { a.single = true; return a }

func (a *AccordionElement) expand(ctx *Context) Element {
	col := Div().Column().Gap(8)
	if a.key != "" {
		col.Key(a.key)
	}
	for i, s := range a.sections {
		if a.single {
			s.group = func(open bool) {
				if !open {
					return
				}
				for j, o := range a.sections {
					if j != i && o.open.Peek() {
						o.set(false)
					}
				}
			}
		} else {
			s.group = nil
		}
		col.Child(s)
	}
	return a.size.apply(col)
}
