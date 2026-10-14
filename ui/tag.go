package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// TagStyle is the look of each part of a tag. A part's style merges over the
// built-in defaults, then over the theme's Tag, then over the Styles of the
// instance; see docs/styling.md. Tag is the chip: its shape, fill and border.
// Label is its text, and Remove the box around the remove button, which a tag
// has only when it has OnRemove.
type TagStyle struct {
	Tag    Style // the chip: its shape, fill and border
	Label  Style // the text of the tag
	Remove Style // the box around the remove button
}

// merge returns s with the set fields of o applied on top, part by part.
func (s TagStyle) merge(o TagStyle) TagStyle {
	return TagStyle{Tag: s.Tag.Merge(o.Tag), Label: s.Label.Merge(o.Label), Remove: s.Remove.Merge(o.Remove)}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s TagStyle) plain() TagStyle {
	return TagStyle{Tag: plainStyle(s.Tag), Label: plainStyle(s.Label), Remove: plainStyle(s.Remove)}
}

// tagDefaults returns the built-in look of a tag: a small rounded chip in the
// surface color, with the text color. Under high contrast the chip is an outlined
// empty box.
func tagDefaults(t *Theme) TagStyle {
	d := TagStyle{
		Tag: Style{Direction: Row, Align: Center, Gap: 6, NoShrink: true, Radius: 6, Padding: XY(8, 3),
			Background: t.SurfaceActive, fitWidth: true},
		Label:  Style{FontSize: 13, FontWeight: FontMedium, TextColor: t.Text},
		Remove: Style{NoShrink: true},
	}
	if t.HighContrast {
		d.Tag.Background, d.Tag.BorderWidth, d.Tag.BorderColor = t.Surface, 1, t.Text
	}
	return d
}

// resolveTag returns the look of a tag: the defaults, merged with the theme's Tag
// and then the instance's Styles. Under high contrast the two overrides are plain
// (see plainStyle).
func resolveTag(ctx *Context, inst TagStyle) TagStyle {
	t := ctx.Theme()
	return layered(ctx, tagDefaults(t), TagStyle.merge, TagStyle.plain, themeLayer(t.Tag), instanceLayer(inst))
}

// TagElement is a small rounded chip with a text, for a label or a filter. A tag
// can have a leading icon, and a remove button that the user can press to take
// the tag away.
type TagElement struct {
	text     string
	icon     *icons.Icon
	onRemove func()
	key      string
	styles   TagStyle
	size     sizing // the tag's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// Tag creates a tag showing text. A tag is as wide as its text by default. Under
// a MaxWidth, a text that does not fit ends with an ellipsis.
func Tag(text string) *TagElement { return &TagElement{text: text} }

func (*TagElement) isElement()           {}
func (g *TagElement) elementKey() string { return g.key }

// Key identifies the tag among its siblings (see DivElement.Key).
func (g *TagElement) Key(k string) *TagElement { g.key = k; return g }

// Icon sets an icon in front of the text.
func (g *TagElement) Icon(ic *icons.Icon) *TagElement { g.icon = ic; return g }

// OnRemove adds a remove button after the text, which calls fn when it is
// pressed with the mouse, or with Enter or Space. The button is named "Remove"
// and the tag's text, such as "Remove Sales".
func (g *TagElement) OnRemove(fn func()) *TagElement { g.onRemove = fn; return g }

// The sizing methods give the tag's outer box a size. See Sizing in docs/styling.md.
func (g *TagElement) Width(l Length) *TagElement          { g.size.setWidth(l); return g }
func (g *TagElement) MinWidth(l Length) *TagElement       { g.size.setMinWidth(l); return g }
func (g *TagElement) MaxWidth(l Length) *TagElement       { g.size.setMaxWidth(l); return g }
func (g *TagElement) Grow(f float32) *TagElement          { g.size.setGrow(f); return g }
func (g *TagElement) GridSpan(cols, rows int) *TagElement { g.size.setSpan(cols, rows); return g }

// Styles sets the look of the parts of the tag, for this instance only. Set fields
// win over the theme's Tag and the defaults; see TagStyle and docs/styling.md.
// Calling Styles again merges with the earlier values.
func (g *TagElement) Styles(st TagStyle) *TagElement {
	g.styles = g.styles.merge(st)
	return g
}

func (g *TagElement) expand(ctx *Context) Element {
	st := resolveTag(ctx, g.styles)
	strs := ctx.Strings()
	// The tag is a group named by its text, so it is read as one thing.
	tag := Div().asPart("Tag.Tag").Style(st.Tag).Role(access.RoleGroup).Label(g.text)
	if g.icon != nil {
		tag.Child(Icon(g.icon).Size(14))
	}
	tag.Child(Div().asPart("Tag.Label").Style(st.Label).Child(Text(g.text).Truncate()))
	if g.onRemove != nil {
		tag.Child(Div().asPart("Tag.Remove").Style(st.Remove).Child(
			IconButton(icons.X).Label(strs.Remove(g.text)).Small().Ghost().OnClick(g.onRemove)))
	}
	return g.size.apply(tag)
}
