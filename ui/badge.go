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
	fill, ink := badgeColors(t, b.variant)
	if b.kind == badgeDot {
		d := Div().Size(Px(8), Px(8)).Shrink(0).Radius(4).Background(fill)
		if t.HighContrast {
			d.Background(ink)
		}
		b.decorate(d)
		return d
	}
	pill := Div().Row().Align(Center).Shrink(0).Radius(9).PaddingX(7).PaddingY(1).MinWidth(Px(18)).Background(fill)
	if t.HighContrast {
		pill.Border(1, t.Text)
	}
	label := Text(b.label()).Size(11).Semibold().Color(ink).NoWrap()
	if b.leadDot {
		pill.Gap(4).Child(Div().Hidden().Size(Px(6), Px(6)).Shrink(0).Radius(3).Background(ink))
	}
	pill.Child(label)
	b.decorate(pill)
	return pill
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
