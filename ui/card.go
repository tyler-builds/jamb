package ui

import (
	"slices"

	"github.com/tyler-builds/jamb/access"
)

// CardVariant is the look of a card: CardFilled, a raised surface with no
// border, or CardOutlined, a border with no fill.
type CardVariant uint8

const (
	CardFilled   CardVariant = iota // the Surface color, no border (the default)
	CardOutlined                    // a border in the Border color, no fill
)

// CardStyle is the look of each part of a card. A part's style merges over the
// built-in defaults, then over the theme's Card, then over the theme's Cards for
// the card's variant, then over the Styles of the instance; see docs/styling.md.
// Card is the card itself; the states apply only to a card with OnClick:
// CardHover while the pointer is over it, CardPressed while it is pressed, and
// CardFocusVisible while it has keyboard focus.
type CardStyle struct {
	Card             Style // the card: its shape, fill, border and padding
	CardHover        Style // a clickable card the pointer is over
	CardPressed      Style // a clickable card while it is pressed
	CardFocusVisible Style // a clickable card that has keyboard focus
}

// merge returns s with the set fields of o applied on top, part by part.
func (s CardStyle) merge(o CardStyle) CardStyle {
	return CardStyle{
		Card:             s.Card.Merge(o.Card),
		CardHover:        s.CardHover.Merge(o.CardHover),
		CardPressed:      s.CardPressed.Merge(o.CardPressed),
		CardFocusVisible: s.CardFocusVisible.Merge(o.CardFocusVisible),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s CardStyle) plain() CardStyle {
	return CardStyle{
		Card:             plainStyle(s.Card),
		CardHover:        plainStyle(s.CardHover),
		CardPressed:      plainStyle(s.CardPressed),
		CardFocusVisible: plainStyle(s.CardFocusVisible),
	}
}

// CardVariants is the look of each card variant, in Theme.Cards.
type CardVariants struct {
	// Filled and Outlined are the styles of the variants, and each merges over Card.
	Filled, Outlined CardStyle
}

// of returns the style of variant v.
func (cv CardVariants) of(v CardVariant) CardStyle {
	if v == CardOutlined {
		return cv.Outlined
	}
	return cv.Filled
}

// cardDefaults returns the built-in look of a card of variant v. Under high
// contrast both variants are the window color outlined in the text color, and a
// hovered card's border is the highlight color.
func cardDefaults(t *Theme, v CardVariant) CardStyle {
	d := CardStyle{
		Card:        Style{Direction: Column, Gap: 8, Padding: All(16), Radius: t.Radius, Background: t.Surface},
		CardHover:   Style{Background: t.SurfaceHover},
		CardPressed: Style{Background: t.SurfaceActive},
	}
	if v == CardOutlined {
		d.Card.Background = Color{}
		d.Card.BorderWidth, d.Card.BorderColor = 1, t.Border
		d.CardHover = Style{Background: t.SurfaceHover, BorderColor: t.BorderStrong}
	}
	if t.HighContrast {
		d.Card.Background, d.Card.BorderWidth, d.Card.BorderColor = t.Surface, 1, t.Text
		d.CardHover = Style{BorderColor: t.Accent}
		d.CardPressed = Style{BorderColor: t.Accent}
	}
	return d
}

// resolveCard returns the look of a card: the defaults, merged with the theme's
// Card and Cards and then the instance's Styles. Under high contrast the
// overrides are plain (see plainStyle).
func resolveCard(ctx *Context, v CardVariant, inst CardStyle) CardStyle {
	t := ctx.Theme()
	return layered(ctx, cardDefaults(t, v), CardStyle.merge, CardStyle.plain,
		themeLayer(t.Card), themeLayer(t.Cards.of(v)), instanceLayer(inst))
}

// CardElement is a surface that holds related content, such as one item of a
// grid of results. It lays its children out in a column by default (see Row,
// Gap and Padding). With OnClick the whole card is one control: it is a button
// to assistive technology, named by Label or by its text, takes keyboard focus,
// runs fn on Enter and Space, and shows hover and pressed states. Put no other
// control inside a clickable card, since a control inside a control cannot be
// reached on its own.
type CardElement struct {
	variant  CardVariant
	children []Element
	onClick  func()
	label    string
	key      string
	tooltip  string
	layout   Style // Row, Gap, Padding and Align, set on the instance
	styles   CardStyle
	size     sizing // the card's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// Card creates a filled card. A card is as wide as its container's column, or
// as its content in a row, as a Div is.
func Card() *CardElement { return &CardElement{} }

func (*CardElement) isElement()           {}
func (c *CardElement) elementKey() string { return c.key }

// Key identifies the card among its siblings (see DivElement.Key).
func (c *CardElement) Key(k string) *CardElement { c.key = k; return c }

// Variant sets the look: CardFilled (the default) or CardOutlined.
func (c *CardElement) Variant(v CardVariant) *CardElement { c.variant = v; return c }

// Outlined sets the look to CardOutlined.
func (c *CardElement) Outlined() *CardElement { return c.Variant(CardOutlined) }

// Child appends a child.
func (c *CardElement) Child(e Element) *CardElement {
	if !isNil(e) {
		c.children = append(c.children, e)
	}
	return c
}

// Children appends several children.
func (c *CardElement) Children(es ...Element) *CardElement {
	for _, e := range es {
		c.Child(e)
	}
	return c
}

// OnClick makes the whole card a button that runs fn (see CardElement).
func (c *CardElement) OnClick(fn func()) *CardElement { c.onClick = fn; return c }

// Label names a clickable card for assistive technology, in place of the text
// it holds, such as "Firefox, web browser" for a card that shows more.
func (c *CardElement) Label(s string) *CardElement { c.label = s; return c }

// Tooltip shows s below the card after the pointer rests on it. Assistive
// technology reads it as the card's description.
func (c *CardElement) Tooltip(s string) *CardElement { c.tooltip = s; return c }

// Row lays the children out in a row instead of a column.
func (c *CardElement) Row() *CardElement { c.layout.Direction = Row; return c }

// Gap sets the space between the children, 8 by default. Gap(0) removes it.
func (c *CardElement) Gap(px float32) *CardElement {
	c.layout.Gap = px
	if px == 0 {
		c.layout.Clear |= ClearGap
	}
	return c
}

// Padding sets the space inside the card on every side, 16 by default.
// Padding(0) removes it, as for a card that shows an image edge to edge.
func (c *CardElement) Padding(px float32) *CardElement {
	c.layout.Padding = All(px)
	if px == 0 {
		c.layout.Clear |= ClearPadding
	}
	return c
}

// Align sets how the children line up across the card's main axis.
func (c *CardElement) Align(a Alignment) *CardElement { c.layout.Align = a; return c }

// The sizing methods give the card's outer box a size. See Sizing in docs/styling.md.
func (c *CardElement) Width(l Length) *CardElement { c.size.setWidth(l); return c }

// MinWidth sets the smallest width of the card.
func (c *CardElement) MinWidth(l Length) *CardElement { c.size.setMinWidth(l); return c }

// MaxWidth sets the largest width of the card.
func (c *CardElement) MaxWidth(l Length) *CardElement { c.size.setMaxWidth(l); return c }

// Grow makes the card take a share of the free space on its parent's main axis.
func (c *CardElement) Grow(f float32) *CardElement { c.size.setGrow(f); return c }

// GridSpan sets how many columns and rows the card covers in its parent's grid.
func (c *CardElement) GridSpan(cols, rows int) *CardElement { c.size.setSpan(cols, rows); return c }

// Styles sets the look of the parts of the card, for this instance only. Set
// fields win over the theme's Card and Cards and the defaults; see CardStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (c *CardElement) Styles(st CardStyle) *CardElement {
	c.styles = c.styles.merge(st)
	return c
}

func (c *CardElement) expand(ctx *Context) Element {
	st := resolveCard(ctx, c.variant, c.styles)
	d := Div().asPart("Card.Card").Style(st.Card).Style(c.layout).Tooltip(c.tooltip).Children(c.children...)
	if c.onClick == nil {
		if c.label != "" {
			d.Role(access.RoleGroup).Label(c.label)
		}
		return c.size.apply(d)
	}
	d.Hover(st.CardHover).Pressed(st.CardPressed).Focusable().OnClick(c.onClick)
	// The focus ring is added to the card's own shadows, as a button's is.
	ring := Shadow{Spread: 2, Color: ctx.Theme().FocusRing}
	d.FocusVisible(Style{Shadows: append(slices.Clone(st.Card.Shadows), ring)}.Merge(st.CardFocusVisible))
	d.Role(access.RoleButton)
	if c.label != "" {
		d.Label(c.label)
	}
	return c.size.apply(d)
}
