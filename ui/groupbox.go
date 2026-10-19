package ui

import "github.com/tyler-builds/jamb/access"

// GroupBoxStyle is the look of each part of a group box. A part's style merges
// over the built-in defaults, then over the theme's GroupBox, then over the
// Styles of the instance; see docs/styling.md. GroupBox is the frame, Header the
// area that holds the Title and the Description, and Content the area under it
// that holds the children. A group box with neither a title nor a description
// has no header.
type GroupBoxStyle struct {
	GroupBox    Style // the frame: its shape, fill and border
	Header      Style // the area that holds the title and the description
	Title       Style // the title
	Description Style // the text under the title
	Content     Style // the area that holds the children
}

// merge returns s with the set fields of o applied on top, part by part.
func (s GroupBoxStyle) merge(o GroupBoxStyle) GroupBoxStyle {
	return GroupBoxStyle{
		GroupBox:    s.GroupBox.Merge(o.GroupBox),
		Header:      s.Header.Merge(o.Header),
		Title:       s.Title.Merge(o.Title),
		Description: s.Description.Merge(o.Description),
		Content:     s.Content.Merge(o.Content),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s GroupBoxStyle) plain() GroupBoxStyle {
	return GroupBoxStyle{
		GroupBox:    plainStyle(s.GroupBox),
		Header:      plainStyle(s.Header),
		Title:       plainStyle(s.Title),
		Description: plainStyle(s.Description),
		Content:     plainStyle(s.Content),
	}
}

// groupBoxDefaults returns the built-in look of a group box: a frame outlined in
// the border color, or, with fill, a frame filled with a raised surface color and
// no border. Under high contrast both are the window color outlined in the text
// color.
func groupBoxDefaults(t *Theme, fill bool) GroupBoxStyle {
	d := GroupBoxStyle{
		GroupBox:    Style{Direction: Column, Radius: t.Radius, BorderWidth: 1, BorderColor: t.Border},
		Header:      Style{Direction: Column, Gap: 2, Padding: Edges{Top: 12, Right: 14, Left: 14}},
		Title:       Style{FontWeight: FontSemibold, TextColor: t.Text},
		Description: Style{FontSize: 13, TextColor: t.TextMuted},
		Content:     Style{Direction: Column, Gap: 8, Padding: All(14)},
	}
	if fill {
		d.GroupBox.Background, d.GroupBox.BorderWidth = t.SurfaceHover, 0
	}
	if t.HighContrast {
		d.GroupBox.Background, d.GroupBox.BorderWidth, d.GroupBox.BorderColor = t.Surface, 1, t.Text
		d.Description.TextColor = t.Text
	}
	return d
}

// resolveGroupBox returns the look of a group box: the defaults, merged with the
// theme's GroupBox and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveGroupBox(ctx *Context, fill bool, inst GroupBoxStyle) GroupBoxStyle {
	t := ctx.Theme()
	return layered(ctx, groupBoxDefaults(t, fill), GroupBoxStyle.merge, GroupBoxStyle.plain,
		themeLayer(t.GroupBox), instanceLayer(inst))
}

// GroupBoxElement is a titled section that holds related content, such as the
// settings of one feature. It is outlined by default; Fill fills it instead. It
// is a group named by its title, with the description as its description.
type GroupBoxElement struct {
	title    string
	desc     string
	fill     bool
	children []Element
	key      string
	styles   GroupBoxStyle
	size     sizing // the box's sizes, from the sizing methods (see Sizing in docs/styling.md)
	tooltip  string // see Tooltip
}

// GroupBox creates a group box titled title. A group box fills the width of its
// container by default, and lays its children out in a column.
func GroupBox(title string) *GroupBoxElement { return &GroupBoxElement{title: title} }

func (*GroupBoxElement) isElement()           {}
func (g *GroupBoxElement) elementKey() string { return g.key }

// Key identifies the group box among its siblings (see DivElement.Key).
func (g *GroupBoxElement) Key(k string) *GroupBoxElement { g.key = k; return g }

// Description sets the text under the title.
func (g *GroupBoxElement) Description(s string) *GroupBoxElement { g.desc = s; return g }

// Fill fills the box with a raised surface color, without a border, in place of
// the default outline.
func (g *GroupBoxElement) Fill() *GroupBoxElement { g.fill = true; return g }

// Child appends a child to the content.
func (g *GroupBoxElement) Child(e Element) *GroupBoxElement {
	if !isNil(e) {
		g.children = append(g.children, e)
	}
	return g
}

// Children appends several children to the content.
func (g *GroupBoxElement) Children(es ...Element) *GroupBoxElement {
	for _, e := range es {
		g.Child(e)
	}
	return g
}

// Tooltip shows s below the group box after the pointer rests on it. Assistive
// technology reads it as the group's description, when it has none.
func (g *GroupBoxElement) Tooltip(s string) *GroupBoxElement { g.tooltip = s; return g }

// The sizing methods give the group box's outer box a size. See Sizing in docs/styling.md.
func (g *GroupBoxElement) Width(l Length) *GroupBoxElement { g.size.setWidth(l); return g }

// MinWidth sets the smallest width of the group box.
func (g *GroupBoxElement) MinWidth(l Length) *GroupBoxElement { g.size.setMinWidth(l); return g }

// MaxWidth sets the largest width of the group box.
func (g *GroupBoxElement) MaxWidth(l Length) *GroupBoxElement { g.size.setMaxWidth(l); return g }

// Grow makes the group box take a share of the free space on its parent's main axis.
func (g *GroupBoxElement) Grow(f float32) *GroupBoxElement { g.size.setGrow(f); return g }

// GridSpan sets how many columns and rows the group box covers in its parent's grid.
func (g *GroupBoxElement) GridSpan(cols, rows int) *GroupBoxElement {
	g.size.setSpan(cols, rows)
	return g
}

// Styles sets the look of the parts of the group box, for this instance only. Set
// fields win over the theme's GroupBox and the defaults; see GroupBoxStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (g *GroupBoxElement) Styles(st GroupBoxStyle) *GroupBoxElement {
	g.styles = g.styles.merge(st)
	return g
}

func (g *GroupBoxElement) expand(ctx *Context) Element {
	st := resolveGroupBox(ctx, g.fill, g.styles)
	// The box is a group named by its title, and described by its description.
	box := Div().asPart("GroupBox.GroupBox").Style(st.GroupBox).Role(access.RoleGroup).Label(g.title).Tooltip(g.tooltip)
	if g.desc != "" {
		box.Description(g.desc)
	}
	if g.title != "" || g.desc != "" {
		header := Div().asPart("GroupBox.Header").Style(st.Header)
		if g.title != "" {
			header.Child(Div().asPart("GroupBox.Title").Style(st.Title).Child(Text(g.title)))
		}
		if g.desc != "" {
			header.Child(Div().asPart("GroupBox.Description").Style(st.Description).Child(Text(g.desc)))
		}
		box.Child(header)
	}
	box.Child(Div().asPart("GroupBox.Content").Style(st.Content).Children(g.children...))
	return g.size.apply(box)
}
