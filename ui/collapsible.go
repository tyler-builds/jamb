package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

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
	title    string
	open     *Signal[bool]
	content  Element
	key      string
	disabled bool
	keep     bool // see KeepContent
	onChange func(bool)
	// group is set by an Accordion, and runs after a change made by the header.
	group func(open bool)
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

// KeepContent keeps the content once the section has been opened. The content is
// built the first time the section opens, as it is by default, and then stays in
// the tree while the section is closed, hidden with Display. Its state survives
// closing and opening the section. Without KeepContent the content is discarded
// when the section closes.
func (c *CollapsibleElement) KeepContent() *CollapsibleElement { c.keep = true; return c }

// OnChange runs fn with the new state after the user opens or closes the section.
// The signal is set first.
func (c *CollapsibleElement) OnChange(fn func(open bool)) *CollapsibleElement {
	c.onChange = fn
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
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	chevron := icons.ChevronRight
	if on {
		chevron = icons.ChevronDown
	}

	header := Div().Key(c.headerKey()).Row().Align(Center).Gap(8).MinHeight(Px(40)).PaddingX(12).Shrink(0).
		Radius(t.Radius).Role(access.RoleButton).Label(c.title).Expanded(on).
		Child(Icon(chevron).Size(14).Color(t.TextMuted)).
		Child(Text(c.title).Medium().Color(t.Text))
	if c.disabled {
		header.access().disabled = true
		header.Cursor(CursorNotAllowed)
		if t.HighContrast {
			header.TextColor(t.Disabled)
		} else {
			header.Opacity(0.5)
		}
	} else {
		header.Cursor(CursorPointer).Hover(Style{Background: t.SurfaceHover}).
			Focusable().FocusVisible(Style{Shadows: []Shadow{ring}}).
			OnClick(func() { c.set(!on) })
	}

	col := Div().Column().Border(1, t.Border).Radius(t.Radius).Child(header)
	if c.key != "" {
		col.Key(c.key)
	}
	// With KeepContent the content is built once the section has opened, and is
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
		col.Child(Div().Column().Display(on).Role(access.RoleGroup).Label(c.title).LabelledBy(c.headerKey()).
			PaddingEdges(Edges{Top: 4, Right: 12, Bottom: 12, Left: 12}).Child(c.content))
	}
	return col
}

// AccordionElement is a group of collapsible sections. By default each section
// opens and closes on its own. SingleOpen makes opening one section close the
// others, as Microsoft's expander guidance describes for groups of expanders
// (https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/expander),
// and leaves the choice to the app: the default is independent sections.
type AccordionElement struct {
	sections []*CollapsibleElement
	key      string
	single   bool
}

// Accordion creates a group of sections, shown in order.
func Accordion(sections ...*CollapsibleElement) *AccordionElement {
	return &AccordionElement{sections: sections}
}

func (*AccordionElement) isElement()           {}
func (a *AccordionElement) elementKey() string { return a.key }

// Key gives the group a stable identity among its siblings.
func (a *AccordionElement) Key(k string) *AccordionElement { a.key = k; return a }

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
	return col
}
