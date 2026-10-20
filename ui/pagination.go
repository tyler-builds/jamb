package ui

import (
	"fmt"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// PaginationStyle is the look of each part of a pagination control. A part's style
// merges over the built-in defaults, then over the theme's Pagination, then over
// the Styles of the instance; see docs/styling.md. Bar is the row that holds the
// control, Page each page number and PageHover the one the pointer is over.
// PageCurrent is the current page: it is drawn over Page, filled, and is not
// chosen. Nav is the Previous and Next button, which takes PageHover too while it
// is enabled, and Ellipsis the "..." that stands for pages that are skipped.
type PaginationStyle struct {
	Bar         Style // the row that holds the control
	Page        Style // each page number
	PageHover   Style // a page the pointer is over, if it can be chosen
	PageCurrent Style // the current page, over Page
	Nav         Style // the Previous and Next buttons
	Ellipsis    Style // the "..." where pages are skipped
}

// merge returns s with the set fields of o applied on top, part by part.
func (s PaginationStyle) merge(o PaginationStyle) PaginationStyle {
	return PaginationStyle{
		Bar:         s.Bar.Merge(o.Bar),
		Page:        s.Page.Merge(o.Page),
		PageHover:   s.PageHover.Merge(o.PageHover),
		PageCurrent: s.PageCurrent.Merge(o.PageCurrent),
		Nav:         s.Nav.Merge(o.Nav),
		Ellipsis:    s.Ellipsis.Merge(o.Ellipsis),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s PaginationStyle) plain() PaginationStyle {
	return PaginationStyle{
		Bar:         plainStyle(s.Bar),
		Page:        plainStyle(s.Page),
		PageHover:   plainStyle(s.PageHover),
		PageCurrent: plainStyle(s.PageCurrent),
		Nav:         plainStyle(s.Nav),
		Ellipsis:    plainStyle(s.Ellipsis),
	}
}

// paginationDefaults returns the built-in look of a pagination control, from the
// colors of t. The current page is filled with Accent, as a selected segment is.
func paginationDefaults(t *Theme) PaginationStyle {
	cell := Style{Direction: Row, Align: Center, Justify: Center, MinWidth: Px(32), MinHeight: Px(32),
		Padding: XY(6, 0), Radius: t.Radius, NoShrink: true, TextColor: t.TextMuted, Transition: t.Motion.hover(), Spring: t.Motion.Spring}
	return PaginationStyle{
		Bar:         Style{Direction: Row, Align: Center, Gap: 4, fitWidth: true, TextColor: t.Text},
		Page:        cell,
		PageHover:   Style{Background: t.SurfaceHover, TextColor: t.Text},
		PageCurrent: Style{Background: t.Accent, TextColor: t.AccentText},
		Nav:         cell,
		Ellipsis: Style{Direction: Row, Align: Center, Justify: Center, MinWidth: Px(32), MinHeight: Px(32),
			NoShrink: true, TextColor: t.TextSubtle},
	}
}

// paginationDisabled returns the look of a Previous or Next button that cannot be
// chosen: dimmed, or GrayText under high contrast, as a disabled button is.
func paginationDisabled(t *Theme) Style {
	if t.HighContrast {
		return Style{TextColor: t.Disabled}
	}
	return Style{Opacity: 0.5}
}

// resolvePagination returns the look of a pagination control: the defaults, merged
// with the theme's Pagination and then the instance's Styles. Under high contrast
// the two overrides are plain (see plainStyle).
func resolvePagination(ctx *Context, inst PaginationStyle) PaginationStyle {
	t := ctx.Theme()
	return layered(ctx, paginationDefaults(t), PaginationStyle.merge, PaginationStyle.plain,
		themeLayer(t.Pagination), instanceLayer(inst))
}

// pageSlots returns the pages to show of a control over pages pages, in order, with
// page cur as the current one and siblings pages shown on each side of it. The
// value 0 stands for an ellipsis, where pages are skipped. The first and the last
// page are always shown, and up to seven pages are shown in full.
func pageSlots(cur, pages, siblings int) []int {
	if pages <= 7 {
		out := make([]int, pages)
		for i := range out {
			out[i] = i + 1
		}
		return out
	}
	siblings = max(siblings, 0)
	lo, hi := max(cur-siblings, 1), min(cur+siblings, pages)
	var out []int
	// An ellipsis never stands for a single page: that page is shown instead,
	// since it takes the same room.
	if lo > 1 {
		out = append(out, 1)
		switch {
		case lo == 3:
			out = append(out, 2)
		case lo > 3:
			out = append(out, 0)
		}
	}
	for p := lo; p <= hi; p++ {
		out = append(out, p)
	}
	if hi < pages {
		switch {
		case hi == pages-2:
			out = append(out, pages-1)
		case hi < pages-2:
			out = append(out, 0)
		}
		out = append(out, pages)
	}
	return out
}

// PaginationElement is a row of page buttons, with Previous and Next, that reads and
// writes a page number. Pages are numbered from 1. The current page is selected and
// cannot be chosen; the other pages are buttons in the Tab order, and Enter and
// Space choose them, as a click does. Previous and Next are disabled at the ends.
// The row is a group named by its Label, and the current page is named "Page n,
// current page".
type PaginationElement struct {
	page     *Signal[int]
	pages    int
	siblings int
	compact  bool
	key      string
	name     string // the accessible name of the control, see Label
	onChange func(int)
	styles   PaginationStyle
	size     sizing // the control's sizes, from the sizing methods (see Sizing in docs/styling.md)
	tooltip  string // see Tooltip
}

// Pagination creates a page control over pages pages, numbered from 1, that reads
// and writes page. The page shown is page, held between 1 and pages. One page is
// shown on each side of the current page; see Siblings.
func Pagination(page *Signal[int], pages int) *PaginationElement {
	return &PaginationElement{page: page, pages: pages, siblings: 1}
}

func (*PaginationElement) isElement()           {}
func (p *PaginationElement) elementKey() string { return p.key }

// Key gives the control a stable identity. It also scopes the keys of its pages, so
// give two controls over the same signal different keys. Without a key the signal's
// identity scopes them.
func (p *PaginationElement) Key(k string) *PaginationElement { p.key = k; return p }

// Label names the control for assistive technology, such as "Search results pages".
// It defaults to the Pagination text of the window's Strings.
func (p *PaginationElement) Label(name string) *PaginationElement { p.name = name; return p }

// Siblings sets how many pages are shown on each side of the current page. The
// default is 1, and a negative value counts as zero.
func (p *PaginationElement) Siblings(n int) *PaginationElement { p.siblings = n; return p }

// OnChange runs fn with the new page after the user chooses a page, by a click,
// by Enter or Space, or by Previous and Next. The signal is set first.
func (p *PaginationElement) OnChange(fn func(page int)) *PaginationElement {
	p.onChange = fn
	return p
}

// Tooltip shows s below the control after the pointer rests on it. Assistive
// technology reads it as the control's description, unless it has another.
func (p *PaginationElement) Tooltip(s string) *PaginationElement { p.tooltip = s; return p }

// Compact shows only Previous, the text "Page 3 of 20" and Next, for a place with
// no room for the page numbers.
func (p *PaginationElement) Compact() *PaginationElement { p.compact = true; return p }

// Styles sets the look of the parts of the control, for this instance only. Set
// fields win over the theme's Pagination and the defaults; see PaginationStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (p *PaginationElement) Styles(s PaginationStyle) *PaginationElement {
	p.styles = p.styles.merge(s)
	return p
}

// The sizing methods give the control's outer box a size. The control is as wide as
// its pages by default; see Sizing in docs/styling.md.
func (p *PaginationElement) Width(l Length) *PaginationElement { p.size.setWidth(l); return p }

// MinWidth sets the smallest width of the pagination.
func (p *PaginationElement) MinWidth(l Length) *PaginationElement { p.size.setMinWidth(l); return p }

// MaxWidth sets the largest width of the pagination.
func (p *PaginationElement) MaxWidth(l Length) *PaginationElement { p.size.setMaxWidth(l); return p }

// GridSpan sets how many columns and rows the pagination covers in its parent's grid.
func (p *PaginationElement) GridSpan(cols, rows int) *PaginationElement {
	p.size.setSpan(cols, rows)
	return p
}

// scope is the prefix of the keys of this control's pages.
func (p *PaginationElement) scope() string {
	if p.key != "" {
		return p.key
	}
	return fmt.Sprintf("pagination:%p", p.page)
}

// choose sets the page to n and runs OnChange.
func (p *PaginationElement) choose(n int) {
	p.page.Set(n)
	if p.onChange != nil {
		p.onChange(n)
	}
}

func (p *PaginationElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		th := ctx.Theme()
		st := resolvePagination(ctx, p.styles)
		s := ctx.Strings()
		pages := max(p.pages, 1)
		cur := min(max(p.page.Get(), 1), pages)
		name := p.name
		if name == "" {
			name = s.Pagination
		}
		bar := p.size.apply(Div().asPart("Pagination.Bar").Role(access.RoleGroup).Label(name).SelectionContainer().Style(st.Bar).
			Tooltip(p.tooltip))
		bar.Child(p.nav(th, st, "previous", s.PreviousPage, icons.ChevronLeft, cur-1, cur > 1))
		if p.compact {
			bar.Child(Text(s.PageOf(cur, pages)).NoWrap())
		} else {
			for _, n := range pageSlots(cur, pages, p.siblings) {
				if n == 0 {
					bar.Child(Div().asPart("Pagination.Ellipsis").AccessHidden().Style(st.Ellipsis).Child(Text("...").NoWrap()))
					continue
				}
				bar.Child(p.cell(s, th, st, n, cur))
			}
		}
		bar.Child(p.nav(th, st, "next", s.NextPage, icons.ChevronRight, cur+1, cur < pages))
		return bar
	})
}

// cell returns the button of page n. The current page, cur, is drawn filled and is
// not a button: it is named as the current page and is selected in the group.
func (p *PaginationElement) cell(s *Strings, th *Theme, st PaginationStyle, n, cur int) *DivElement {
	key := p.scope() + fmt.Sprintf("/page:%d", n)
	label := fmt.Sprint(n)
	if n == cur {
		return Div().asPart("Pagination.Page").Key(key).Style(st.Page.Merge(st.PageCurrent)).
			Role(access.RoleStaticText).Label(s.CurrentPage(n)).AccessSelected(true).
			Child(Text(label).NoWrap())
	}
	ring := Shadow{Spread: 2, Color: th.FocusRing}
	return Div().asPart("Pagination.Page").Key(key).Style(st.Page).
		Role(access.RoleButton).Label(s.Page(n)).Hover(st.PageHover).
		Focusable().FocusVisible(Style{Shadows: []Shadow{ring}}).
		OnClick(func() { p.choose(n) }).Child(Text(label).NoWrap())
}

// nav returns the Previous or the Next button, which goes to page to. A button that
// is not enabled is dimmed and cannot be chosen.
func (p *PaginationElement) nav(th *Theme, st PaginationStyle, key, name string, ic *icons.Icon, to int, enabled bool) *DivElement {
	d := Div().asPart("Pagination.Nav").Key(p.scope() + "/" + key).Style(st.Nav).
		Role(access.RoleButton).Label(name).Child(Icon(ic).Size(16))
	if !enabled {
		d.Style(paginationDisabled(th)).Cursor(CursorNotAllowed)
		d.access().disabled = true
		return d
	}
	ring := Shadow{Spread: 2, Color: th.FocusRing}
	return d.Hover(st.PageHover).Focusable().FocusVisible(Style{Shadows: []Shadow{ring}}).
		OnClick(func() { p.choose(to) })
}
