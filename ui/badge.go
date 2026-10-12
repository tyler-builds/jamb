package ui

import (
	"fmt"

	"github.com/tyler-builds/jamb/access"
)

// BadgeVariant is the color of a badge.
type BadgeVariant uint8

const (
	BadgeNeutral BadgeVariant = iota
	BadgeAccent
	BadgeSuccess
	BadgeWarning
	BadgeDanger
)

// badgeKind says what a badge shows.
type badgeKind uint8

const (
	badgeCount badgeKind = iota
	badgeText
	badgeDot
)

// BadgeStyle is the look of each part of a badge. A part's style merges over the
// built-in defaults of its variant, then over the theme's Badge, then over the
// theme's Badges for that variant, then over the Styles of the instance; see
// docs/styling.md. Badge is the pill, Label its text and Dot the dot: the dot
// that a badge with Dot() shows before its text, or a badge that is only a dot
// (BadgeDot). A variant's colors are in its defaults, so an override of Badge or
// Dot sets the color for the variants it applies to.
type BadgeStyle struct {
	Badge Style // the pill: its shape, fill and border
	Label Style // the text of the badge
	Dot   Style // the dot, before the text or alone
}

// merge returns s with the set fields of o applied on top, part by part.
func (s BadgeStyle) merge(o BadgeStyle) BadgeStyle {
	return BadgeStyle{Badge: s.Badge.Merge(o.Badge), Label: s.Label.Merge(o.Label), Dot: s.Dot.Merge(o.Dot)}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s BadgeStyle) plain() BadgeStyle {
	return BadgeStyle{Badge: plainStyle(s.Badge), Label: plainStyle(s.Label), Dot: plainStyle(s.Dot)}
}

// BadgeVariants holds the style of each variant of a badge, for the theme's
// Badges. It merges over Theme.Badge, which applies to every variant.
type BadgeVariants struct {
	Neutral, Accent, Success, Warning, Danger BadgeStyle
}

// of returns the style of variant v.
func (bv BadgeVariants) of(v BadgeVariant) BadgeStyle {
	switch v {
	case BadgeAccent:
		return bv.Accent
	case BadgeSuccess:
		return bv.Success
	case BadgeWarning:
		return bv.Warning
	case BadgeDanger:
		return bv.Danger
	}
	return bv.Neutral
}

// badgeDefaults returns the built-in look of a badge of variant v: a pill, or,
// with dot, a dot on its own. The colors come from badgeColors.
func badgeDefaults(t *Theme, v BadgeVariant, dot bool) BadgeStyle {
	fill, ink := badgeColors(t, v)
	if dot {
		if t.HighContrast {
			fill = ink
		}
		return BadgeStyle{Dot: Style{Width: Px(8), Height: Px(8), NoShrink: true, Radius: 4, Background: fill}}
	}
	d := BadgeStyle{
		Badge: Style{Direction: Row, Align: Center, NoShrink: true, Radius: 9, Padding: XY(7, 1),
			MinWidth: Px(18), Background: fill, fitWidth: true},
		Label: Style{FontSize: 11, FontWeight: FontSemibold, TextColor: ink},
		Dot:   Style{Width: Px(6), Height: Px(6), NoShrink: true, Radius: 3, Background: ink},
	}
	if t.HighContrast {
		d.Badge.BorderWidth, d.Badge.BorderColor = 1, t.Text
	}
	return d
}

// resolveBadge returns the look of a badge of variant v: the defaults, merged
// with the theme's Badge, the theme's Badges for the variant, and the instance's
// Styles. Under high contrast the overrides are plain (see plainStyle).
func resolveBadge(t *Theme, v BadgeVariant, dot bool, inst BadgeStyle) BadgeStyle {
	st := badgeDefaults(t, v, dot)
	for _, o := range []BadgeStyle{t.Badge, t.Badges.of(v), inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// BadgeElement is a small label: a count, a word such as "New", or a dot. A
// badge is drawn as a pill in its variant's color. High contrast draws it
// outlined, so the variant is also told by the text.
type BadgeElement struct {
	kind     badgeKind
	count    int
	text     string
	max      int
	variant  BadgeVariant
	noun     string
	key      string
	attached bool // set by DivElement.Badged: the owner names the badge
	leadDot  bool // a dot before the text (see Dot)
	styles   BadgeStyle
	size     sizing // the badge's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// Badge creates a badge showing count. A count above the maximum (99 by
// default; see Max) shows as "99+". A count of zero shows nothing, so a
// badge can stay in place while its count is empty.
func Badge(count int) *BadgeElement {
	return &BadgeElement{kind: badgeCount, count: count, max: 99}
}

// BadgeText creates a badge with a word in it, such as "New" or "Beta".
func BadgeText(text string) *BadgeElement {
	return &BadgeElement{kind: badgeText, text: text}
}

// BadgeDot creates a badge that is only a dot, for a thing that is new or
// changed without a count.
func BadgeDot() *BadgeElement { return &BadgeElement{kind: badgeDot} }

func (*BadgeElement) isElement()           {}
func (b *BadgeElement) elementKey() string { return b.key }

func (b *BadgeElement) Key(k string) *BadgeElement { b.key = k; return b }

// Variant sets the color: BadgeNeutral, BadgeAccent, BadgeSuccess, BadgeWarning
// or BadgeDanger.
func (b *BadgeElement) Variant(v BadgeVariant) *BadgeElement { b.variant = v; return b }

// Max sets the count above which the badge shows "n+", such as "99+".
func (b *BadgeElement) Max(n int) *BadgeElement { b.max = n; return b }

// Noun is the word that follows the count when the badge names its owner, as
// "items" makes a badge of 6 read "6 items". It is not drawn.
func (b *BadgeElement) Noun(s string) *BadgeElement { b.noun = s; return b }

// Dot puts a dot in front of the text, in the text color, as a status chip does.
// The dot is hidden from assistive technology: the text is what is read.
func (b *BadgeElement) Dot() *BadgeElement { b.leadDot = true; return b }

// The sizing methods give the badge's outer box a size. A badge is as wide as its
// text by default; see Sizing in docs/styling.md.
func (b *BadgeElement) Width(l Length) *BadgeElement    { b.size.setWidth(l); return b }
func (b *BadgeElement) MinWidth(l Length) *BadgeElement { b.size.setMinWidth(l); return b }
func (b *BadgeElement) MaxWidth(l Length) *BadgeElement { b.size.setMaxWidth(l); return b }
func (b *BadgeElement) Grow(f float32) *BadgeElement    { b.size.setGrow(f); return b }

// Styles sets the look of the parts of the badge, for this instance only. Set
// fields win over the theme's Badge and Badges and the defaults; see BadgeStyle
// and docs/styling.md. Calling Styles again merges with the earlier values.
func (b *BadgeElement) Styles(st BadgeStyle) *BadgeElement {
	b.styles = b.styles.merge(st)
	return b
}

// label is the text drawn in the badge. A dot draws none.
func (b *BadgeElement) label() string {
	switch b.kind {
	case badgeText:
		return b.text
	case badgeCount:
		if b.count > b.max {
			return fmt.Sprintf("%d+", b.max)
		}
		return fmt.Sprint(b.count)
	}
	return ""
}

// accName is what the badge adds to the name of its owner, such as "6 items".
// It is empty for a dot and for a count of zero.
func (b *BadgeElement) accName() string {
	if b.kind == badgeDot || b.hidden() {
		return ""
	}
	if b.kind == badgeCount && b.count > b.max {
		// The number is "99+" in the badge, and the owner says the real count.
		if b.noun != "" {
			return fmt.Sprintf("more than %d %s", b.max, b.noun)
		}
		return fmt.Sprintf("more than %d", b.max)
	}
	if b.noun != "" {
		return b.label() + " " + b.noun
	}
	return b.label()
}

// hidden reports whether the badge draws nothing: a count of zero.
func (b *BadgeElement) hidden() bool {
	return b.kind == badgeCount && b.count <= 0
}

func (b *BadgeElement) expand(ctx *Context) Element {
	if b.hidden() {
		return nil
	}
	t := ctx.Theme()
	if b.kind == badgeDot {
		st := resolveBadge(t, b.variant, true, b.styles)
		d := Div().asPart("Badge.Dot").Style(st.Dot)
		b.decorate(d)
		return b.size.apply(d)
	}
	st := resolveBadge(t, b.variant, false, b.styles)
	pill := Div().asPart("Badge.Badge")
	if b.leadDot {
		pill.Style(Style{Gap: 4})
	}
	pill.Style(st.Badge)
	label := Div().asPart("Badge.Label").Style(st.Label).Child(Text(b.label()).NoWrap())
	if b.leadDot {
		pill.Child(Div().asPart("Badge.Dot").Hidden().Style(st.Dot))
	}
	pill.Child(label)
	b.decorate(pill)
	return b.size.apply(pill)
}

// decorate sets the access settings of a badge. A badge that belongs to an
// owner is hidden: the owner names itself with the badge's text. A badge that
// stands alone is a group named by its text, so it is read as one thing.
func (b *BadgeElement) decorate(d *DivElement) {
	if b.attached {
		d.Hidden()
		return
	}
	if name := b.accName(); name != "" {
		d.Role(access.RoleGroup).Label(name)
	}
}

// badgeColors returns the fill and the text color of a badge. The text is dark
// on the bright variants (Success and Warning) and light on the others, so
// each pair keeps its contrast in both themes.
func badgeColors(t *Theme, v BadgeVariant) (fill, ink Color) {
	if t.HighContrast {
		return t.Surface, t.Text
	}
	switch v {
	case BadgeAccent:
		return t.Accent, t.AccentText
	case BadgeSuccess:
		return t.Success, badgeDark(t)
	case BadgeWarning:
		return t.Warning, badgeDark(t)
	case BadgeDanger:
		return t.Danger, t.AccentText
	}
	return t.SurfaceActive, t.Text
}

// badgeDark is the dark text on a bright fill: the background in the dark theme
// and the text in the light theme.
func badgeDark(t *Theme) Color {
	if t.Dark {
		return t.Background
	}
	return t.Text
}

// Badged adds b to the end of d and names it in d's accessible name, so that
// the row reads as "Projects, 6 items" rather than as the row followed by a
// separate badge. The badge is then hidden from assistive technology. See the
// Badge API in the README.
func (d *DivElement) Badged(b *BadgeElement) *DivElement {
	b.attached = true
	if name := b.accName(); name != "" {
		d.access().badge = name
	}
	return d.Child(b)
}
