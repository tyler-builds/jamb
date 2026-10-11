package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// partCollapsible is a section titled "Sync" with the content "Body", open or
// closed, disabled or not, with the instance style st. slot sets RenderHeader.
func partCollapsible(acc bool, open bool, st CollapsibleStyle, disabled bool, slot func(string, PartState) Element) Element {
	sig := NewSignal(open)
	c := Collapsible("Sync", sig, Text("Body")).Key("sec").Disabled(disabled).Styles(st)
	if slot != nil {
		c.RenderHeader(slot)
	}
	if acc {
		return Div().Column().Child(Accordion(c).Key("acc"))
	}
	return Div().Column().Child(c)
}

func testCollapsibleParts(t *testing.T, applied map[string]bool) {
	type collCase struct {
		part     string
		set      func(*CollapsibleStyle, Style)
		open     bool
		disabled bool
		drawn    string
		text     bool
		act      func(*testing.T, *accessRig)
	}
	hoverHeader := func(t *testing.T, r *accessRig) { hoverNode(r, partNode(t, r.rt, "Collapsible.Header", "")) }
	focusHeader := func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "Collapsible.Header", "")) }
	cases := []collCase{
		{part: "Collapsible.Section", set: func(s *CollapsibleStyle, m Style) { s.Section = m }, drawn: "Collapsible.Section"},
		{part: "Collapsible.Header", set: func(s *CollapsibleStyle, m Style) { s.Header = m }, drawn: "Collapsible.Header"},
		{part: "Collapsible.HeaderHover", set: func(s *CollapsibleStyle, m Style) { s.HeaderHover = m }, drawn: "Collapsible.Header", act: hoverHeader},
		{part: "Collapsible.HeaderOpen", set: func(s *CollapsibleStyle, m Style) { s.HeaderOpen = m }, open: true, drawn: "Collapsible.Header"},
		{part: "Collapsible.HeaderFocusVisible", set: func(s *CollapsibleStyle, m Style) { s.HeaderFocusVisible = m }, drawn: "Collapsible.Header", act: focusHeader},
		{part: "Collapsible.HeaderDisabled", set: func(s *CollapsibleStyle, m Style) { s.HeaderDisabled = m }, disabled: true, drawn: "Collapsible.Header"},
		{part: "Collapsible.Chevron", set: func(s *CollapsibleStyle, m Style) { s.Chevron = m }, drawn: "Collapsible.Chevron", text: true},
		{part: "Collapsible.Content", set: func(s *CollapsibleStyle, m Style) { s.Content = m }, open: true, drawn: "Collapsible.Content"},
	}
	// An Accordion's sections are Collapsibles, so each case runs for both.
	for _, c := range cases {
		for _, acc := range []bool{false, true} {
			name := c.part
			if acc {
				name = "Accordion" + strings.TrimPrefix(c.part, "Collapsible")
			}
			applied[name] = true
			t.Run(name, func(t *testing.T) {
				var st CollapsibleStyle
				c.set(&st, Style{Background: markBG, TextColor: markFG})
				r := newAccessRig(func(ctx *Context) Element { return partCollapsible(acc, c.open, st, c.disabled, nil) })
				if c.act != nil {
					c.act(t, r)
				}
				n := partNode(t, r.rt, c.drawn, "")
				if c.text {
					wantTextColor(t, n, name, markFG)
					return
				}
				wantBackground(t, n, name, markBG)
			})
		}
	}
}

// TestCollapsibleRenderHeader checks the header slot: the title is the slot's
// content and is hidden from assistive technology, the header keeps its name and
// expanded state, the slot is called again on hover with the state, and a click
// still opens the section.
func TestCollapsibleRenderHeader(t *testing.T) {
	sig := NewSignal(false)
	var last PartState
	var calls int
	slot := func(title string, s PartState) Element {
		calls++
		last = s
		return Text(fmt.Sprintf("%s (%d)", title, calls))
	}
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Collapsible("Sync", sig, Text("Body")).Key("sec").RenderHeader(slot))
	})
	accessNode(t, r, access.RoleButton, "Sync")
	r.tree().Walk(func(n *access.Node) bool {
		if strings.Contains(n.Name, "Sync (") {
			t.Errorf("the slot's content is in the access tree as %q", n.Name)
		}
		return true
	})
	before := calls
	hoverNode(r, partNode(t, r.rt, "Collapsible.Header", ""))
	if calls == before {
		t.Fatalf("the slot was not called again on hover")
	}
	if !last.Hovered {
		t.Errorf("the slot's last state has Hovered false, want true")
	}
	r.clickKey(t, "sec/header")
	if !sig.Peek() {
		t.Fatalf("a click on the header did not open the section")
	}
	if !last.Selected {
		t.Errorf("the slot's last state has Selected false after the section opened")
	}
}

// TestCollapsibleUnderHighContrast checks the guardrails: an app color does not
// reach the header under high contrast, its padding does, and the focus ring of
// the header stays.
func TestCollapsibleUnderHighContrast(t *testing.T) {
	theme := *DarkTheme
	theme.Collapsible = CollapsibleStyle{Header: Style{Background: markBG, Padding: XY(7, 0)}}
	sig := NewSignal(false)
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Div().Column().Child(Collapsible("Sync", sig, Text("Body")).Key("sec").
			Styles(CollapsibleStyle{Header: Style{Background: markBG, Padding: XY(9, 0)}}))
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	n := partNode(t, r.rt, "Collapsible.Header", "")
	if n.style.Background == markBG {
		t.Errorf("high contrast: an app color reached the header")
	}
	if n.style.Padding != XY(9, 0) {
		t.Errorf("high contrast: the instance padding %v did not apply", XY(9, 0))
	}
	focusNode(r, n)
	if len(n.style.Shadows) == 0 {
		t.Errorf("high contrast: the focused header has no focus ring")
	}
}
