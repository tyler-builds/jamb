package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// A clickable card is one button named by its label: Enter and a click run its
// OnClick. A plain card is no control, and its text is read as it is.
func TestCardClickableIsAButton(t *testing.T) {
	clicks := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Card().Key("app").Label("Firefox, web browser").OnClick(func() { clicks++ }).
				Child(Text("Firefox")).Child(Text("Web browser"))).
			Child(Card().Key("plain").Child(Text("Just text")))
	})
	tr := r.rt.AccessTree()
	b := tr.Find(access.RoleButton, "Firefox, web browser")
	if b == nil || !b.States.Has(access.StateFocusable) {
		t.Fatalf("no focusable button named for the card:\n%s", tr.Dump())
	}
	r.clickKey(t, "app")
	focusNode(r, keyNode(t, r.rt, "app"))
	r.press(input.KeyEnter, 0)
	if clicks != 2 {
		t.Errorf("OnClick ran %d times after a click and Enter, want 2", clicks)
	}
	if tr.Find(access.RoleButton, "Just text") != nil {
		t.Error("a card without OnClick is a button")
	}
	if len(r.warnings) != 0 {
		t.Errorf("warnings: %v", r.warnings)
	}
}

// The outlined variant draws a border and no fill, and the filled one the reverse.
func TestCardVariants(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Card().Key("f").Child(Text("a"))).Child(Card().Key("o").Outlined().Child(Text("b")))
	})
	th := r.rt.theme
	f, o := keyNode(t, r.rt, "f"), keyNode(t, r.rt, "o")
	if f.style.Background != th.Surface || f.style.BorderWidth != 0 {
		t.Errorf("filled card: background %v, border %v; want Surface and none", f.style.Background, f.style.BorderWidth)
	}
	if !o.style.Background.IsZero() || o.style.BorderWidth != 1 || o.style.BorderColor != th.Border {
		t.Errorf("outlined card: background %v, border %v %v; want none and 1 in Border", o.style.Background, o.style.BorderWidth, o.style.BorderColor)
	}
}

func testCardParts(t *testing.T, applied map[string]bool) {
	type cardCase struct {
		part string
		set  func(*CardStyle, Style)
		act  func(*testing.T, *accessRig)
	}
	cases := []cardCase{
		{part: "Card.Card", set: func(s *CardStyle, m Style) { s.Card = m }},
		{part: "Card.CardHover", set: func(s *CardStyle, m Style) { s.CardHover = m },
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "card")) }},
		{part: "Card.CardPressed", set: func(s *CardStyle, m Style) { s.CardPressed = m },
			act: func(t *testing.T, r *accessRig) { pressNode(r, keyNode(t, r.rt, "card")) }},
		{part: "Card.CardFocusVisible", set: func(s *CardStyle, m Style) { s.CardFocusVisible = m },
			act: func(t *testing.T, r *accessRig) { focusNode(r, keyNode(t, r.rt, "card")) }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st CardStyle
			c.set(&st, Style{Background: markBG})
			r := newAccessRig(func(ctx *Context) Element {
				return Div().Column().Child(Card().Key("card").OnClick(func() {}).Child(Text("Body")).Styles(st))
			})
			if c.act != nil {
				c.act(t, r)
			}
			wantBackground(t, partNode(t, r.rt, "Card.Card", ""), c.part, markBG)
		})
	}
}
