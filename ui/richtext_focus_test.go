package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// linkRig is a button, a paragraph with two links, and a list item whose text has a
// third link, with the links counting their clicks.
func linkRig() (*Runtime, *[3]int) {
	var clicks [3]int
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Gap(8).
			Child(Button("Before").Key("before")).
			Child(Rich(Seg("See "), Link("one", func() { clicks[0]++ }), Seg(" and "),
				Link("two", func() { clicks[1]++ }), Seg(".")).Key("para")).
			Child(Div().Role(access.RoleListItem).
				Child(Rich(Seg("Item with "), Link("three", func() { clicks[2]++ })).Key("item")))
	}, 400, 300, 1)
	return rt, &clicks
}

// Tab stops on each link of rich text in order, after the controls before it, and
// Enter follows the link that has focus.
func TestTabStopsOnEachLinkOfRichText(t *testing.T) {
	rt, clicks := linkRig()
	focusedName := func() string {
		tree := rt.AccessTree()
		n := tree.Node(tree.Focus)
		if n == nil {
			return ""
		}
		return n.Role.String() + " " + n.Name
	}
	want := []string{"button Before", "link one", "link two", "link three", "button Before"}
	for i, w := range want {
		rt.Press(input.KeyTab, 0)
		if got := focusedName(); got != w {
			t.Fatalf("Tab %d: focus on %q, want %q", i+1, got, w)
		}
	}
	rt.Press(input.KeyTab, input.ModShift)
	if got := focusedName(); got != "link three" {
		t.Fatalf("Shift+Tab: focus on %q, want the last link", got)
	}
	rt.Press(input.KeyTab, input.ModShift)
	rt.Press(input.KeyEnter, 0)
	rt.Press(input.KeySpace, 0)
	if *clicks != [3]int{0, 1, 0} {
		t.Fatalf("clicks %v after Enter and Space on the second link, want Enter to follow it once", *clicks)
	}
}

// The link that has keyboard focus is drawn with a ring of the theme's focus color,
// and focusing another element removes it.
func TestFocusedLinkHasARing(t *testing.T) {
	rt, _ := linkRig()
	ring := rt.theme.FocusRing.scene()
	rings := func() int {
		n := 0
		for _, q := range paintNow(rt).Quads {
			if q.BorderColor == ring && q.BorderWidth > 0 && q.Background.A == 0 {
				n++
			}
		}
		return n
	}
	before := rings()
	rt.Press(input.KeyTab, 0)
	rt.Press(input.KeyTab, 0)
	if got := rings(); got != before+1 {
		t.Fatalf("%d ring quads with the link focused, want one more than the %d before", got, before)
	}
	rt.Press(input.KeyTab, input.ModShift)
	if got := rings(); got > before+1 || rt.focusLink != 0 {
		t.Fatalf("focus back on the button: %d ring quads, focusLink %d", got, rt.focusLink)
	}
}

// Assistive technology finds every link, also the one inside the list item, whose name
// is its text; it can focus a link, which then has keyboard focus, and click it.
func TestLinksAreFocusableAccessNodes(t *testing.T) {
	rt, clicks := linkRig()
	rt.AccessBridge().Activate()
	frame(rt)
	tree := rt.AccessTree()
	three := tree.Find(access.RoleLink, "three")
	if three == nil {
		t.Fatal("the link inside the list item is not in the tree")
	}
	if p := tree.Node(three.Parent); p == nil || p.Role != access.RoleListItem {
		t.Fatalf("the link's parent is %+v, want the list item", p)
	}
	if !three.States.Has(access.StateFocusable) {
		t.Fatal("the link is not focusable")
	}
	rt.runAccess(three.ID, access.Request{Action: access.ActionFocus})
	frame(rt)
	if tree := rt.AccessTree(); tree.Focus != three.ID || !tree.Node(three.ID).States.Has(access.StateFocused) {
		t.Fatalf("focus is on %v after focusing the link %v", tree.Focus, three.ID)
	}
	rt.Press(input.KeyEnter, 0)
	rt.runAccess(three.ID, access.Request{Action: access.ActionClick})
	if clicks[2] != 2 {
		t.Fatalf("the link ran %d times after Enter and a click from assistive technology, want 2", clicks[2])
	}
}

// The segments of one link made by LinkSegs are one link: one Tab stop, one access
// link named by their joined text, one ring, and Enter runs the link once.
func TestLinkSegsAreOneLink(t *testing.T) {
	clicks := 0
	rt := newTestRuntime(func(ctx *Context) Element {
		segs := append([]Segment{Seg("Read ")}, LinkSegs(func() { clicks++ }, Seg("the "), Seg("guide").Bold(), Seg(" now"))...)
		return Div().Child(Button("Before")).Child(Rich(append(segs, Seg("."))...))
	}, 400, 300, 1)
	if got := len(rt.tabStops()); got != 2 {
		t.Fatalf("%d tab stops, want 2: the button and the link", got)
	}
	rt.Press(input.KeyTab, 0)
	rt.Press(input.KeyTab, 0)
	tree := rt.AccessTree()
	f := tree.Node(tree.Focus)
	if f == nil || f.Role != access.RoleLink || f.Name != "the guide now" {
		t.Fatalf("focus on %+v, want the link \"the guide now\"", f)
	}
	links := 0
	tree.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleLink {
			links++
		}
		return true
	})
	if links != 1 {
		t.Errorf("%d access links, want 1", links)
	}
	ring := rt.theme.FocusRing.scene()
	rings := 0
	for _, q := range paintNow(rt).Quads {
		if q.BorderColor == ring && q.BorderWidth > 0 && q.Background.A == 0 {
			rings++
		}
	}
	if rings != 1 {
		t.Errorf("%d ring quads around a link on one line, want 1", rings)
	}
	rt.Press(input.KeyEnter, 0)
	rt.Press(input.KeyTab, 0) // back to the button: the link is one stop
	tree = rt.AccessTree()
	if f := tree.Node(tree.Focus); f == nil || f.Role != access.RoleButton || clicks != 1 {
		t.Fatalf("after Enter and Tab: focus on %+v and %d clicks, want the button and 1", f, clicks)
	}
}
