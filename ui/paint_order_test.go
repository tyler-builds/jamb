package ui

import (
	"slices"
	"testing"
	"time"
)

// TestPaintOrderFollowsZIndex checks the paint order of a node's children: without
// a ZIndex it is the children's order, and with one the element above is painted
// last, as ZIndex says, while the children stay in their own order.
func TestPaintOrderFollowsZIndex(t *testing.T) {
	rt := NewRuntime(func(ctx *Context) Element {
		return Div().Size(Full, Full).Column().
			Child(Div().Key("box").Size(Full, Full).Column().
				Child(Div().Key("a").Size(Px(10), Px(10))).
				Child(Div().Key("b").Size(Px(10), Px(10)).ZIndex(2)).
				Child(Div().Key("c").Size(Px(10), Px(10))))
	}, DarkTheme(), benchText)
	rt.SetViewport(400, 400, 1)
	rt.Frame(time.Unix(1_000_000, 0))

	box := rt.findKey(rt.tree, "box")
	if box == nil {
		t.Fatal("the box is not in the tree")
	}
	var keys []string
	for _, c := range box.order {
		keys = append(keys, c.key)
	}
	if want := []string{"a", "c", "b"}; !slices.Equal(keys, want) {
		t.Fatalf("paint order = %v, want %v", keys, want)
	}
	keys = keys[:0]
	for _, c := range box.children {
		keys = append(keys, c.key)
	}
	if want := []string{"a", "b", "c"}; !slices.Equal(keys, want) {
		t.Fatalf("children = %v, want %v", keys, want)
	}
}
