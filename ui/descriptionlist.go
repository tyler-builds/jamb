package ui

import "github.com/tyler-builds/jamb/access"

// DescriptionListStyle is the look of each part of a description list. A part's
// style merges over the built-in defaults, then over the theme's
// DescriptionList, then over the Styles of the instance; see docs/styling.md.
// DescriptionList is the grid of items, and Item one term with its value: the Term
// and the Value. The defaults depend on the layout (beside or Vertical) and on
// Bordered.
type DescriptionListStyle struct {
	DescriptionList Style // the grid of items: its gap, and its frame when bordered
	Item            Style // one term with its value
	Term            Style // the term, such as "Owner"
	Value           Style // the value of the term
}

// merge returns s with the set fields of o applied on top, part by part.
func (s DescriptionListStyle) merge(o DescriptionListStyle) DescriptionListStyle {
	return DescriptionListStyle{
		DescriptionList: s.DescriptionList.Merge(o.DescriptionList),
		Item:            s.Item.Merge(o.Item),
		Term:            s.Term.Merge(o.Term),
		Value:           s.Value.Merge(o.Value),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s DescriptionListStyle) plain() DescriptionListStyle {
	return DescriptionListStyle{
		DescriptionList: plainStyle(s.DescriptionList),
		Item:            plainStyle(s.Item),
		Term:            plainStyle(s.Term),
		Value:           plainStyle(s.Value),
	}
}

// descriptionListDefaults returns the built-in look of a description list: the
// term in muted text, in a column of its own beside the value, or above the value
// when vertical. A bordered list is outlined, with a line between its items, which
// are padded. Under high contrast the lines and the term are the text color.
func descriptionListDefaults(t *Theme, vertical, bordered bool) DescriptionListStyle {
	d := DescriptionListStyle{
		DescriptionList: Style{Gap: 12},
		Term:            Style{FontSize: 13, TextColor: t.TextMuted},
		Value:           Style{Direction: Column, MinWidth: Px(0), TextColor: t.Text},
	}
	if vertical {
		d.Item = Style{Direction: Column, Gap: 2}
	} else {
		d.Item = Style{Direction: Row, Align: Start, Gap: 12}
		d.Term.Width, d.Term.NoShrink = Px(120), true
		d.Value.Grow = 1
	}
	if bordered {
		d.DescriptionList = Style{BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius}
		d.Item.Padding, d.Item.BorderColor = XY(12, 8), t.Border
	}
	if t.HighContrast {
		d.DescriptionList.BorderColor, d.Item.BorderColor, d.Term.TextColor = t.Text, t.Text, t.Text
	}
	return d
}

// resolveDescriptionList returns the look of a description list: the defaults,
// merged with the theme's DescriptionList and then the instance's Styles. Under
// high contrast the two overrides are plain (see plainStyle).
func resolveDescriptionList(ctx *Context, vertical, bordered bool, inst DescriptionListStyle) DescriptionListStyle {
	t := ctx.Theme()
	return layered(ctx, descriptionListDefaults(t, vertical, bordered), DescriptionListStyle.merge,
		DescriptionListStyle.plain, themeLayer(t.DescriptionList), instanceLayer(inst))
}

// descItem is one term of a description list, with its value.
type descItem struct {
	term  string
	value Element
}

// DescriptionListElement shows terms with their values, such as the details of a
// record: "Owner" and "Dana Lee", "Status" and a badge. The items are laid out in
// a grid, row by row, one column by default. Each item is a group named by its
// term.
type DescriptionListElement struct {
	items    []descItem
	columns  int
	vertical bool
	bordered bool
	key      string
	styles   DescriptionListStyle
	size     sizing // the list's sizes, from the sizing methods (see Sizing in docs/styling.md)
	tooltip  string // see Tooltip
}

// DescriptionList creates an empty description list; Item and ItemText add its
// items. A list fills the width of its container by default.
func DescriptionList() *DescriptionListElement { return &DescriptionListElement{columns: 1} }

func (*DescriptionListElement) isElement()           {}
func (l *DescriptionListElement) elementKey() string { return l.key }

// Key identifies the list among its siblings (see DivElement.Key).
func (l *DescriptionListElement) Key(k string) *DescriptionListElement { l.key = k; return l }

// Item adds the term with value, which can be any element, such as a badge or a
// link.
func (l *DescriptionListElement) Item(term string, value Element) *DescriptionListElement {
	l.items = append(l.items, descItem{term: term, value: value})
	return l
}

// ItemText adds the term with a value that is text.
func (l *DescriptionListElement) ItemText(term, value string) *DescriptionListElement {
	return l.Item(term, Text(value))
}

// Columns lays the items out in n columns of the same width, row by row. The
// default is 1; a value below 1 counts as 1.
func (l *DescriptionListElement) Columns(n int) *DescriptionListElement {
	l.columns = max(n, 1)
	return l
}

// Vertical puts each term above its value, in place of the default column of
// terms beside the values.
func (l *DescriptionListElement) Vertical() *DescriptionListElement { l.vertical = true; return l }

// Bordered outlines the list and draws a line between its items, which are padded.
// The lines between items are the border edges of the items.
func (l *DescriptionListElement) Bordered() *DescriptionListElement { l.bordered = true; return l }

// Tooltip shows s below the list after the pointer rests on it.
func (l *DescriptionListElement) Tooltip(s string) *DescriptionListElement { l.tooltip = s; return l }

// The sizing methods give the list's outer box a size. See Sizing in docs/styling.md.
func (l *DescriptionListElement) Width(w Length) *DescriptionListElement {
	l.size.setWidth(w)
	return l
}

// MinWidth sets the smallest width of the list.
func (l *DescriptionListElement) MinWidth(w Length) *DescriptionListElement {
	l.size.setMinWidth(w)
	return l
}

// MaxWidth sets the largest width of the list.
func (l *DescriptionListElement) MaxWidth(w Length) *DescriptionListElement {
	l.size.setMaxWidth(w)
	return l
}

// Grow makes the list take a share of the free space on its parent's main axis.
func (l *DescriptionListElement) Grow(f float32) *DescriptionListElement { l.size.setGrow(f); return l }

// GridSpan sets how many columns and rows the list covers in its parent's grid.
func (l *DescriptionListElement) GridSpan(cols, rows int) *DescriptionListElement {
	l.size.setSpan(cols, rows)
	return l
}

// Styles sets the look of the parts of the list, for this instance only. Set
// fields win over the theme's DescriptionList and the defaults; see
// DescriptionListStyle and docs/styling.md. Calling Styles again merges with the
// earlier values.
func (l *DescriptionListElement) Styles(st DescriptionListStyle) *DescriptionListElement {
	l.styles = l.styles.merge(st)
	return l
}

func (l *DescriptionListElement) expand(ctx *Context) Element {
	st := resolveDescriptionList(ctx, l.vertical, l.bordered, l.styles)
	cols := make([]Length, l.columns)
	for i := range cols {
		cols[i] = Fr(1)
	}
	list := Div().asPart("DescriptionList.DescriptionList").Style(Style{GridColumns: cols}).
		Style(st.DescriptionList).Tooltip(l.tooltip)
	for i, it := range l.items {
		// Each item is a group named by its term, so the value is read with it.
		item := Div().asPart("DescriptionList.Item").Role(access.RoleGroup).Label(it.term)
		if l.bordered {
			// A line above every row but the first, and before every column but the first.
			var edges Edges
			if i >= l.columns {
				edges.Top = 1
			}
			if i%l.columns != 0 {
				edges.Left = 1
			}
			item.Style(Style{BorderEdges: edges})
		}
		item.Style(st.Item).
			Child(Div().asPart("DescriptionList.Term").Style(st.Term).Child(Text(it.term))).
			Child(Div().asPart("DescriptionList.Value").Style(st.Value).Child(it.value))
		list.Child(item)
	}
	return l.size.apply(list)
}
