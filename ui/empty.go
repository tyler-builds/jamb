package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// EmptyStyle is the look of each part of an empty state. A part's style merges
// over the built-in defaults, then over the theme's Empty, then over the Styles of
// the instance; see docs/styling.md. Empty is the box that centers the rest, Icon
// the icon above the title (its TextColor is the icon's color), Title and
// Description the two texts, and Action the box around the action, which an empty
// state has only when it has one.
type EmptyStyle struct {
	Empty       Style // the box: its padding and the space between the parts
	Icon        Style // the icon, in its color
	Title       Style // the title
	Description Style // the text under the title
	Action      Style // the box around the action
}

// merge returns s with the set fields of o applied on top, part by part.
func (s EmptyStyle) merge(o EmptyStyle) EmptyStyle {
	return EmptyStyle{
		Empty:       s.Empty.Merge(o.Empty),
		Icon:        s.Icon.Merge(o.Icon),
		Title:       s.Title.Merge(o.Title),
		Description: s.Description.Merge(o.Description),
		Action:      s.Action.Merge(o.Action),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s EmptyStyle) plain() EmptyStyle {
	return EmptyStyle{
		Empty:       plainStyle(s.Empty),
		Icon:        plainStyle(s.Icon),
		Title:       plainStyle(s.Title),
		Description: plainStyle(s.Description),
		Action:      plainStyle(s.Action),
	}
}

// emptyDefaults returns the built-in look of an empty state, from the colors of
// t: the parts centered in a column, the icon and the description muted, and the
// title semibold in the text color. Under high contrast the icon and the
// description are the text color.
func emptyDefaults(t *Theme) EmptyStyle {
	d := EmptyStyle{
		Empty:       Style{Direction: Column, Align: Center, Justify: Center, Gap: 6, Padding: All(24)},
		Icon:        Style{Padding: Edges{Bottom: 4}, TextColor: t.TextMuted},
		Title:       Style{FontSize: 15, FontWeight: FontSemibold, TextColor: t.Text, TextAlign: Center},
		Description: Style{FontSize: 13, TextColor: t.TextMuted, TextAlign: Center},
		Action:      Style{Padding: Edges{Top: 8}},
	}
	if t.HighContrast {
		d.Icon.TextColor, d.Description.TextColor = t.Text, t.Text
	}
	return d
}

// resolveEmpty returns the look of an empty state: the defaults, merged with the
// theme's Empty and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveEmpty(ctx *Context, inst EmptyStyle) EmptyStyle {
	t := ctx.Theme()
	return layered(ctx, emptyDefaults(t), EmptyStyle.merge, EmptyStyle.plain,
		themeLayer(t.Empty), instanceLayer(inst))
}

// EmptyElement is what a list, a table or a page shows when it has nothing to
// show: an icon, a title, an optional description and an optional action, such
// as a button that clears a filter, centered and muted. It fits the empty slots
// of the widgets, such as DataGrid.Empty. It is a group named by its title, with
// the description as its description.
type EmptyElement struct {
	title   string
	desc    string
	icon    *icons.Icon
	iconSet bool // Icon was called; a nil icon then hides it
	action  Element
	key     string
	styles  EmptyStyle
	size    sizing // the box's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// Empty creates an empty state titled title, with an inbox icon. An empty state
// fills the width of its container by default, and centers its parts in it.
func Empty(title string) *EmptyElement { return &EmptyElement{title: title} }

func (*EmptyElement) isElement()           {}
func (e *EmptyElement) elementKey() string { return e.key }

// Key identifies the empty state among its siblings (see DivElement.Key).
func (e *EmptyElement) Key(k string) *EmptyElement { e.key = k; return e }

// Description sets the text under the title, such as what to do next.
func (e *EmptyElement) Description(s string) *EmptyElement { e.desc = s; return e }

// Icon replaces the inbox icon. Icon(nil) shows no icon.
func (e *EmptyElement) Icon(ic *icons.Icon) *EmptyElement { e.icon, e.iconSet = ic, true; return e }

// Action adds el under the description, such as a button that clears a filter or
// creates the first item.
func (e *EmptyElement) Action(el Element) *EmptyElement { e.action = el; return e }

// The sizing methods give the empty state's outer box a size. See Sizing in docs/styling.md.
func (e *EmptyElement) Width(l Length) *EmptyElement { e.size.setWidth(l); return e }

// MinWidth sets the smallest width of the empty state.
func (e *EmptyElement) MinWidth(l Length) *EmptyElement { e.size.setMinWidth(l); return e }

// MaxWidth sets the largest width of the empty state.
func (e *EmptyElement) MaxWidth(l Length) *EmptyElement { e.size.setMaxWidth(l); return e }

// Grow makes the empty state take a share of the free space on its parent's main axis.
func (e *EmptyElement) Grow(f float32) *EmptyElement { e.size.setGrow(f); return e }

// GridSpan sets how many columns and rows the empty state covers in its parent's grid.
func (e *EmptyElement) GridSpan(cols, rows int) *EmptyElement { e.size.setSpan(cols, rows); return e }

// Styles sets the look of the parts of the empty state, for this instance only.
// Set fields win over the theme's Empty and the defaults; see EmptyStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (e *EmptyElement) Styles(st EmptyStyle) *EmptyElement {
	e.styles = e.styles.merge(st)
	return e
}

func (e *EmptyElement) expand(ctx *Context) Element {
	st := resolveEmpty(ctx, e.styles)
	// The box is a group named by its title, and described by its description.
	box := Div().asPart("Empty.Empty").Style(st.Empty).Role(access.RoleGroup).Label(e.title)
	if e.desc != "" {
		box.Description(e.desc)
	}
	ic := icons.Inbox
	if e.iconSet {
		ic = e.icon
	}
	if ic != nil {
		box.Child(Div().asPart("Empty.Icon").Style(st.Icon).Child(Icon(ic).Size(32)))
	}
	if e.title != "" {
		box.Child(Div().asPart("Empty.Title").Style(st.Title).Child(Text(e.title)))
	}
	if e.desc != "" {
		box.Child(Div().asPart("Empty.Description").Style(st.Description).Child(Text(e.desc)))
	}
	if e.action != nil {
		box.Child(Div().asPart("Empty.Action").Style(st.Action).Child(e.action))
	}
	return e.size.apply(box)
}
