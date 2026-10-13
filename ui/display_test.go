package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// namedInTree reports whether the accessibility tree has an element with the name.
func namedInTree(r *accessRig, name string) bool {
	found := false
	r.tree().Walk(func(n *access.Node) bool {
		if n.Name == name {
			found = true
		}
		return true
	})
	return found
}

// listNode returns the scroll container of the ListOf in the runtime, or nil.
func listNode(r *accessRig) *node {
	for _, n := range r.rt.byID {
		if n.div != nil && n.div.vrows != nil {
			return n
		}
	}
	return nil
}

func TestDisplayFalseTakesNoSpace(t *testing.T) {
	shown := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Gap(10).Children(
			Div().Key("top").Height(Px(30)),
			Div().Key("panel").Display(shown.Get()).Height(Px(50)),
			Div().Key("below").Height(Px(20)),
		)
	})
	if got := r.rt.nodeRect(r.nodeByKey(t, "below")).Y; got != 40 {
		t.Fatalf("hidden panel leaves the element below at y=%v, want 40 (no space, no gap)", got)
	}
	if rc := r.rt.nodeRect(r.nodeByKey(t, "panel")); rc.W != 0 || rc.H != 0 {
		t.Fatalf("hidden panel is %vx%v, want 0x0", rc.W, rc.H)
	}
	shown.Set(true)
	r.frame()
	if got := r.rt.nodeRect(r.nodeByKey(t, "below")).Y; got != 100 {
		t.Fatalf("shown panel puts the element below at y=%v, want 100", got)
	}
	if rc := r.rt.nodeRect(r.nodeByKey(t, "panel")); rc.H != 50 {
		t.Fatalf("shown panel is %v high, want 50", rc.H)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestDisplayFalseIsNotPaintedOrHit(t *testing.T) {
	shown := NewSignal(false)
	clicks := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Div().Key("top").Height(Px(30)),
			// Absolute, so it sits over the top strip, where a click at (10, 10) lands.
			Div().Key("panel").Label("Panel").Absolute().Top(0).Left(0).Size(Px(100), Px(20)).
				Display(shown.Get()).OnClick(func() { clicks++ }),
		)
	})
	panel := r.nodeByKey(t, "panel")
	if panel.paintGen == r.rt.paintGen {
		t.Fatalf("hidden panel was painted")
	}
	r.clickAt(10, 10)
	if clicks != 0 {
		t.Fatalf("a click on a hidden panel ran its handler %d times", clicks)
	}
	if hit := r.rt.hitTest(10, 10); len(hit) > 0 && hit[0].key == "panel" {
		t.Fatalf("hit testing found the hidden panel")
	}
	shown.Set(true)
	r.frame()
	r.clickAt(10, 10)
	if clicks != 1 {
		t.Fatalf("a click on the shown panel ran its handler %d times, want 1", clicks)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestDisplayFalseIsSkippedByTabAndCannotTakeFocus(t *testing.T) {
	shown := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Button("Before").Key("before"),
			Div().Key("box").Display(shown.Get()).Column().Child(Button("Hidden button").Key("inner")),
			Button("After").Key("after"),
		)
	})
	inOrder := func(key string) bool {
		id := r.nodeByKey(t, key).id
		for _, x := range r.rt.tabOrder() {
			if x == id {
				return true
			}
		}
		return false
	}
	if inOrder("inner") {
		t.Fatalf("a hidden button is in the Tab order")
	}
	r.focusKey(t, "before")
	r.press(input.KeyTab, 0)
	if got := r.focusedKey(); got != "after" {
		t.Fatalf("Tab from Before moved focus to %q, want After", got)
	}
	// A handle or a click cannot focus it either.
	r.rt.setFocus(r.keyed(t, "inner").id)
	if r.rt.focusID == r.keyed(t, "inner").id {
		t.Fatalf("a hidden button took focus")
	}
	shown.Set(true)
	r.frame()
	if !inOrder("inner") {
		t.Fatalf("the shown button is not in the Tab order")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestDisplayFocusMovesToNearestFocusableAncestorWhenHidden(t *testing.T) {
	shown := NewSignal(true)
	text := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(
			Div().Key("outer").Focusable().Column().Child(
				Div().Key("box").Display(shown.Get()).Column().Child(TextInput(text).Key("field")),
			),
		)
	})
	r.focusKey(t, "field")
	shown.Set(false)
	r.frame()
	if got := r.focusedKey(); got != "outer" {
		t.Fatalf("focus is on %q after its element was hidden, want the focusable ancestor outer", got)
	}
	// With no focusable ancestor, focus is cleared.
	bare := NewSignal(true)
	r = newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Key("box").Display(bare.Get()).Column().Child(Button("Lone").Key("lone")))
	})
	r.focusKey(t, "lone")
	bare.Set(false)
	r.frame()
	if r.rt.focusID != 0 {
		t.Fatalf("focus is on element %d after its element was hidden, want none", r.rt.focusID)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestDisplayFalseIsAbsentFromAccessTree(t *testing.T) {
	shown := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Div().Key("box").Display(shown.Get()).Child(Button("Secret action").Key("secret")),
			Button("Visible action"),
		)
	})
	if namedInTree(r, "Secret action") {
		t.Fatalf("a hidden button is in the accessibility tree")
	}
	if !namedInTree(r, "Visible action") {
		t.Fatalf("the visible button is not in the accessibility tree")
	}
	shown.Set(true)
	r.frame()
	if !namedInTree(r, "Secret action") {
		t.Fatalf("the shown button is not in the accessibility tree")
	}
}

func TestDisplayFalseKeepsTextInputTextAndSelection(t *testing.T) {
	shown := NewSignal(true)
	text := NewSignal("hello")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Key("box").Display(shown.Get()).Child(TextInput(text).Key("field")))
	})
	r.focusKey(t, "field")
	r.press(input.KeyEnd, 0)
	r.press(input.KeyLeft, input.ModShift) // selects the "o"
	shown.Set(false)
	r.frame()
	shown.Set(true)
	r.frame()
	if got := text.Peek(); got != "hello" {
		t.Fatalf("text is %q after hide and show, want hello", got)
	}
	st := r.keyed(t, "field").st.edit
	if st == nil || st.anchor != 5 || st.caret != 4 {
		t.Fatalf("selection after hide and show is %+v, want anchor 5 and caret 4", st)
	}
}

func TestDisplayFalseKeepsLocalState(t *testing.T) {
	shown := NewSignal(true)
	var count *Signal[int]
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Key("box").Display(shown.Get()).Child(
			Component(func(ctx *Context) Element {
				count = Local(ctx, "count", 0)
				return Button("Add").Key("add").OnClick(func() { count.Set(count.Peek() + 1) })
			}),
		))
	})
	r.clickKey(t, "add")
	r.clickKey(t, "add")
	shown.Set(false)
	r.frame()
	shown.Set(true)
	r.frame()
	if got := count.Peek(); got != 2 {
		t.Fatalf("local count is %d after hide and show, want 2", got)
	}
	r.clickKey(t, "add")
	if got := count.Peek(); got != 3 {
		t.Fatalf("local count is %d after a click once shown, want 3", got)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestDisplayFalseKeepsListScrollPosition(t *testing.T) {
	shown := NewSignal(true)
	items := make([]int, 300)
	for i := range items {
		items[i] = i
	}
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Key("box").Display(shown.Get()).Height(Px(200)).Child(
			ListOf(items, func(i int) string { return fmt.Sprint("row", i) }, func(_ int, i int) Element {
				return Div().Height(Px(30)).Child(Text(fmt.Sprint("Row ", i)))
			}).Key("rows"),
		))
	})
	r.rt.Scroll(100, 100, 0, 500, true, 0)
	r.frame()
	before := listNode(r).st.scrollY
	if before <= 0 {
		t.Fatalf("the list did not scroll: %v", before)
	}
	shown.Set(false)
	r.frame()
	shown.Set(true)
	r.frame()
	if after := listNode(r).st.scrollY; after != before {
		t.Fatalf("list scroll position is %v after hide and show, want %v", after, before)
	}
}

func TestDisplayFalseRequestsNoFramesFromRepeatingAnimation(t *testing.T) {
	shown := NewSignal(true)
	// The animation is one element, reused by every render, as a kept panel is, so
	// it renders again only where the runtime asks it to.
	spin := Animate("spin", Animation{Duration: 100 * time.Millisecond, Repeat: true}, func(p float32) Element {
		return Div().Key("dot").Size(Px(10), Px(10)).Background(Hex("#ffffff")).Opacity(0.2 + p*0.5)
	})
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Key("box").Display(shown.Get()).Child(spin))
	})
	if !r.rt.NeedsFrame() {
		t.Fatalf("a shown repeating animation does not ask for frames")
	}
	shown.Set(false)
	r.frame()
	r.frame()
	r.frame()
	if r.rt.NeedsFrame() {
		t.Fatalf("a hidden repeating animation still asks for frames")
	}
	shown.Set(true)
	r.frame()
	if !r.rt.NeedsFrame() {
		t.Fatalf("the animation does not run again once shown")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestDisplayTrueLaysOutAndPaintsAsBefore(t *testing.T) {
	shown := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Key("box").Display(shown.Get()).Height(Px(40)).Background(Hex("#303030")))
	})
	shown.Set(true)
	r.frame()
	box := r.nodeByKey(t, "box")
	if rc := r.rt.nodeRect(box); rc.H != 40 || rc.W == 0 {
		t.Fatalf("shown element is %vx%v, want full width and height 40", rc.W, rc.H)
	}
	if box.paintGen != r.rt.paintGen {
		t.Fatalf("shown element was not painted")
	}
}

func TestDisplayFalseShowsAnAnimationAtItsCurrentProgress(t *testing.T) {
	shown := NewSignal(true)
	last := float32(-1)
	// Built once and reused by every render, as a kept panel is: nothing re-renders
	// it when it is shown, so the runtime must.
	fade := Animate("fade", Animation{Duration: 100 * time.Millisecond}, func(p float32) Element {
		last = p
		return Div().Key("dot").Size(Px(10), Px(10)).Background(Hex("#ffffff")).Opacity(0.2 + p*0.5)
	})
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Key("box").Display(shown.Get()).Child(fade))
	})
	shown.Set(false)
	r.frame()
	r.now = r.now.Add(time.Second) // the animation ends while hidden
	r.frame()
	if r.rt.NeedsFrame() {
		t.Fatalf("a hidden animation asks for frames")
	}
	shown.Set(true)
	r.frame()
	if last != 1 {
		t.Fatalf("the animation shows progress %v once shown, want its end (1)", last)
	}
}
