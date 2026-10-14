package ui

import (
	"fmt"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// Crumb is one location of a Breadcrumbs: its label, an optional icon and the
// function that runs when the location is chosen. A location without OnClick is
// plain text. The last location is the current one, and it is plain text whatever
// its OnClick is.
type Crumb struct {
	Label   string
	Icon    *icons.Icon
	OnClick func()
}

// BreadcrumbsStyle is the look of each part of a breadcrumb trail. A part's style
// merges over the built-in defaults, then over the theme's Breadcrumbs, then over
// the Styles of the instance; see docs/styling.md. Bar is the row that holds the
// trail, Item each location that can be chosen, and ItemHover the one the pointer
// is over. Current is the last location, Separator the chevron between two
// locations, and Overflow the box around the "..." button that lists the hidden
// locations (see MaxItems).
type BreadcrumbsStyle struct {
	Bar       Style // the row that holds the trail
	Item      Style // each location that can be chosen, and each plain location before the last
	ItemHover Style // a location that can be chosen, while the pointer is over it
	Current   Style // the last location, the current one
	Separator Style // the chevron between two locations
	Overflow  Style // the box around the "..." button of the hidden locations
}

// merge returns s with the set fields of o applied on top, part by part.
func (s BreadcrumbsStyle) merge(o BreadcrumbsStyle) BreadcrumbsStyle {
	return BreadcrumbsStyle{
		Bar:       s.Bar.Merge(o.Bar),
		Item:      s.Item.Merge(o.Item),
		ItemHover: s.ItemHover.Merge(o.ItemHover),
		Current:   s.Current.Merge(o.Current),
		Separator: s.Separator.Merge(o.Separator),
		Overflow:  s.Overflow.Merge(o.Overflow),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s BreadcrumbsStyle) plain() BreadcrumbsStyle {
	return BreadcrumbsStyle{
		Bar:       plainStyle(s.Bar),
		Item:      plainStyle(s.Item),
		ItemHover: plainStyle(s.ItemHover),
		Current:   plainStyle(s.Current),
		Separator: plainStyle(s.Separator),
		Overflow:  plainStyle(s.Overflow),
	}
}

// breadcrumbsDefaults returns the built-in look of a breadcrumb trail, from the
// colors of t. The locations that can be chosen are drawn in TextMuted, and the
// current location in Text.
func breadcrumbsDefaults(t *Theme) BreadcrumbsStyle {
	return BreadcrumbsStyle{
		Bar: Style{Direction: Row, Align: Center, Gap: 2, fitWidth: true},
		Item: Style{Direction: Row, Align: Center, Gap: 6, MinHeight: Px(28), Padding: XY(6, 0),
			Radius: t.Radius / 2, NoShrink: true, TextColor: t.TextMuted, Transition: 110 * time.Millisecond},
		ItemHover: Style{Background: t.SurfaceHover, TextColor: t.Text},
		Current: Style{Direction: Row, Align: Center, Gap: 6, MinHeight: Px(28), Padding: XY(6, 0),
			NoShrink: true, TextColor: t.Text},
		Separator: Style{NoShrink: true, TextColor: t.TextSubtle},
		Overflow:  Style{NoShrink: true},
	}
}

// resolveBreadcrumbs returns the look of a breadcrumb trail: the defaults, merged
// with the theme's Breadcrumbs and then the instance's Styles. Under high contrast
// the two overrides are plain (see plainStyle).
func resolveBreadcrumbs(t *Theme, inst BreadcrumbsStyle) BreadcrumbsStyle {
	st := breadcrumbsDefaults(t)
	for _, o := range []BreadcrumbsStyle{t.Breadcrumbs, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// crumbSlots returns the locations of a trail of n locations to show, in order,
// as indexes into the trail. With limit zero, or a trail no longer than limit,
// every location is shown. Otherwise the first location is shown, then the index
// -1 for the "..." button, which stands for the locations between the first and
// the next index, and then the last limit-1 locations. A limit below 2 counts as 2.
func crumbSlots(n, limit int) []int {
	if limit <= 0 || n <= max(limit, 2) {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	limit = max(limit, 2)
	out := []int{0, -1}
	for i := n - (limit - 1); i < n; i++ {
		out = append(out, i)
	}
	return out
}

// BreadcrumbsElement is a trail of locations, from the outermost to the current
// one. A location with an OnClick is a link: it takes keyboard focus in the Tab
// order, and Enter and Space choose it, as a click does. The trail is a group
// named by its Label, and the current location is selected in it, so assistive
// technology reports which location is current. The separators are decorative and
// are hidden from assistive technology.
type BreadcrumbsElement struct {
	items []Crumb
	limit int // see MaxItems
	key   string
	name  string // the accessible name of the trail, see Label
	// styles is the look of this trail's parts, set by Styles.
	styles BreadcrumbsStyle
	size   sizing // the trail's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// Breadcrumbs creates a trail over items, shown left to right, with the last item
// as the current location.
func Breadcrumbs(items ...Crumb) *BreadcrumbsElement {
	return &BreadcrumbsElement{items: items}
}

func (*BreadcrumbsElement) isElement()           {}
func (b *BreadcrumbsElement) elementKey() string { return b.key }

// Key gives the trail a stable identity. With a key, its locations and its menu
// keep their identity too, so their focus survives a render.
func (b *BreadcrumbsElement) Key(k string) *BreadcrumbsElement { b.key = k; return b }

// Label names the trail for assistive technology, such as "Folder path". It
// defaults to the Breadcrumb text of the window's Strings.
func (b *BreadcrumbsElement) Label(name string) *BreadcrumbsElement { b.name = name; return b }

// MaxItems collapses a trail longer than n locations: the first location and the
// last n-1 are shown, with a "..." button between them. The button opens a menu of
// the hidden locations, and a hidden location is chosen in that menu. A value below
// 2 counts as 2. Zero, the default, shows every location.
func (b *BreadcrumbsElement) MaxItems(n int) *BreadcrumbsElement { b.limit = n; return b }

// Styles sets the look of the parts of the trail, for this instance only. Set
// fields win over the theme's Breadcrumbs and the defaults; see BreadcrumbsStyle
// and docs/styling.md. Calling Styles again merges with the earlier values.
func (b *BreadcrumbsElement) Styles(s BreadcrumbsStyle) *BreadcrumbsElement {
	b.styles = b.styles.merge(s)
	return b
}

// The sizing methods give the trail's outer box a size. A trail is as wide as its
// locations by default; see Sizing in docs/styling.md.
func (b *BreadcrumbsElement) Width(l Length) *BreadcrumbsElement    { b.size.setWidth(l); return b }
func (b *BreadcrumbsElement) MinWidth(l Length) *BreadcrumbsElement { b.size.setMinWidth(l); return b }
func (b *BreadcrumbsElement) MaxWidth(l Length) *BreadcrumbsElement { b.size.setMaxWidth(l); return b }
func (b *BreadcrumbsElement) GridSpan(cols, rows int) *BreadcrumbsElement {
	b.size.setSpan(cols, rows)
	return b
}

// keyOf scopes the key of a location or of the menu by the trail's Key. Without a
// Key the locations have no keys, so their identity follows their position.
func (b *BreadcrumbsElement) keyOf(s string) string {
	if b.key == "" {
		return ""
	}
	return b.key + "/" + s
}

func (b *BreadcrumbsElement) expand(ctx *Context) Element {
	th := ctx.Theme()
	st := resolveBreadcrumbs(th, b.styles)
	strs := ctx.Strings()
	name := b.name
	if name == "" {
		name = strs.Breadcrumb
	}
	bar := b.size.apply(Div().asPart("Breadcrumbs.Bar").Role(access.RoleGroup).Label(name).SelectionContainer().Style(st.Bar))
	slots := crumbSlots(len(b.items), b.limit)
	for k, s := range slots {
		if k > 0 {
			bar.Child(Div().asPart("Breadcrumbs.Separator").AccessHidden().Style(st.Separator).
				Child(Icon(icons.ChevronRight).Size(14)))
		}
		if s < 0 {
			// The hidden locations lie between the first location and the next slot.
			bar.Child(b.overflow(strs, st, b.items[slots[k-1]+1:slots[k+1]]))
			continue
		}
		bar.Child(b.crumb(th, st, s))
	}
	return bar
}

// crumb returns the location at index i: the current one, a link, or plain text.
func (b *BreadcrumbsElement) crumb(th *Theme, st BreadcrumbsStyle, i int) *DivElement {
	c := b.items[i]
	key := b.keyOf(fmt.Sprintf("crumb:%d", i))
	if i == len(b.items)-1 {
		d := Div().asPart("Breadcrumbs.Current").Key(key).Role(access.RoleStaticText).Label(c.Label).
			AccessSelected(true).Style(st.Current)
		return crumbContent(d, c)
	}
	d := Div().asPart("Breadcrumbs.Item").Key(key).Style(st.Item)
	if c.OnClick == nil {
		d.Role(access.RoleStaticText).Label(c.Label)
		return crumbContent(d, c)
	}
	// The ring is added to the shadows of the item, as a button adds its own.
	ring := Shadow{Spread: 2, Color: th.FocusRing}
	d.Role(access.RoleLink).Label(c.Label).Hover(st.ItemHover).Focusable().
		FocusVisible(Style{Shadows: []Shadow{ring}}).OnClick(c.OnClick)
	return crumbContent(d, c)
}

// crumbContent adds the icon and the text of a location to d.
func crumbContent(d *DivElement, c Crumb) *DivElement {
	if c.Icon != nil {
		d.Child(Icon(c.Icon).Size(16))
	}
	return d.Child(Text(c.Label).NoWrap())
}

// overflow returns the "..." button that opens a menu of the hidden locations. A
// hidden location without an OnClick is listed as disabled.
func (b *BreadcrumbsElement) overflow(strs *Strings, st BreadcrumbsStyle, hidden []Crumb) *DivElement {
	items := make([]MenuItem, len(hidden))
	for i, c := range hidden {
		items[i] = MenuItem{Label: c.Label, Icon: c.Icon, OnClick: c.OnClick, Disabled: c.OnClick == nil}
	}
	menu := DropdownMenu(Button("...").Ghost().Small().Label(strs.ShowHiddenLocations),
		func() []MenuItem { return items })
	return Div().asPart("Breadcrumbs.Overflow").Style(st.Overflow).Child(menu.Key(b.keyOf("overflow")))
}
