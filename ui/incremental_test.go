package ui

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// ---- helpers ----

func frame(rt *Runtime) FrameStats {
	rt.Frame(time.Now())
	return rt.Stats()
}

func findText(n *node, s string) *node {
	if n.kind == kindText && n.textEl.text == s {
		return n
	}
	for _, c := range n.children {
		if f := findText(c, s); f != nil {
			return f
		}
	}
	return nil
}

func findKey(n *node, key string) *node {
	if n.div != nil && n.div.key == key {
		return n
	}
	for _, c := range n.children {
		if f := findKey(c, key); f != nil {
			return f
		}
	}
	return nil
}

func nodeSet(n *node, into map[*node]bool) map[*node]bool {
	into[n] = true
	for _, c := range n.children {
		nodeSet(c, into)
	}
	return into
}

// rowsApp renders a header plus n keyed row components, each reading its own signal.
func rowsApp(values []*Signal[string]) func(*Context) Element {
	return func(ctx *Context) Element {
		list := Div().Gap(4)
		for i, v := range values {
			list.Child(Component(func(ctx *Context) Element {
				return Div().Row().Gap(8).Padding(4).
					Child(Div().Size(Px(16), Px(16))).
					Child(Text(v.Get()))
			}).Key(fmt.Sprint(i)))
		}
		return Div().Child(Text("Header")).Child(list)
	}
}

// ---- #1: incremental rebuild and layout ----

func TestSignalUpdateRebuildsOnlyItsSubtree(t *testing.T) {
	values := make([]*Signal[string], 100)
	for i := range values {
		values[i] = NewSignal(fmt.Sprintf("row %d", i))
	}
	rt := newTestRuntime(rowsApp(values), 600, 4000, 1)
	before := nodeSet(rt.tree, map[*node]bool{})
	total := len(before)
	y50 := findText(rt.tree, "row 50").abs.Y

	values[37].Set("row 37 edited")
	st := frame(rt)

	if st.Rendered != 1 {
		t.Errorf("rendered %d components, want 1", st.Rendered)
	}
	if st.NodesBuilt != 3 {
		t.Errorf("built %d nodes, want 3 (the row's div, box and text)", st.NodesBuilt)
	}
	if st.NodesLaidOut > 6 {
		t.Errorf("laid out %d of %d nodes, want only the changed row and its ancestors", st.NodesLaidOut, total)
	}
	after := nodeSet(rt.tree, map[*node]bool{})
	kept := 0
	for n := range after {
		if before[n] {
			kept++
		}
	}
	if kept != total-3 {
		t.Errorf("kept %d of %d nodes, want all but the 3 rebuilt", kept, total)
	}
	if findText(rt.tree, "row 37 edited") == nil {
		t.Fatal("updated text not in tree")
	}
	if got := findText(rt.tree, "row 50").abs.Y; got != y50 {
		t.Errorf("row 50 moved from %v to %v although no size changed", y50, got)
	}
	if len(rt.byID) != total {
		t.Errorf("byID has %d entries, want %d (old nodes must be retired)", len(rt.byID), total)
	}
}

func TestSizeChangeInOneComponentMovesSiblings(t *testing.T) {
	values := []*Signal[string]{NewSignal("a"), NewSignal("b"), NewSignal("c")}
	rt := newTestRuntime(rowsApp(values), 200, 600, 1)
	yC := findText(rt.tree, "c").abs.Y
	values[1].Set(strings.Repeat("long words wrap ", 10))
	frame(rt)
	if got := findText(rt.tree, "c").abs.Y; got <= yC {
		t.Errorf("row c stayed at y=%v after row b grew (was %v)", got, yC)
	}
}

func TestHoverRestylesWithoutRendering(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).
			Child(Div().Key("btn").Size(Px(100), Px(40)).Background(Hex("#000000")).
				Hover(Style{Background: Hex("#ff0000")}))
	}, 400, 300, 1)
	btn := findKey(rt.tree, "btn")
	rt.PointerMove(50, 30, 0)
	st := frame(rt)
	if st.Rendered != 0 || st.NodesBuilt != 0 {
		t.Errorf("hover rendered %d components and built %d nodes, want 0", st.Rendered, st.NodesBuilt)
	}
	if st.Restyled == 0 || st.LaidOut {
		t.Errorf("hover restyled=%d laidOut=%v; want a restyle and no layout", st.Restyled, st.LaidOut)
	}
	if findKey(rt.tree, "btn") != btn || btn.style.Background != Hex("#ff0000") {
		t.Error("hover style not applied to the retained node")
	}
}

func TestLayoutAffectingHoverRelayouts(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().
			Child(Div().Key("grow").Height(Px(20)).Hover(Style{Height: Px(60)})).
			Child(Div().Key("below").Height(Px(20)))
	}, 400, 300, 1)
	rt.PointerMove(10, 10, 0)
	frame(rt)
	if y := findKey(rt.tree, "below").abs.Y; y != 60 {
		t.Errorf("sibling below a hover-grown element at y=%v, want 60", y)
	}
}

func TestComponentRenderingNilKeepsItsSlot(t *testing.T) {
	show := NewSignal(false)
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Gap(10).
			Child(Div().Key("a").Height(Px(20))).
			Child(Component(func(ctx *Context) Element {
				return If(show.Get(), Div().Key("mid").Height(Px(20)))
			})).
			Child(Div().Key("b").Height(Px(20)))
	}, 400, 300, 1)
	if y := findKey(rt.tree, "b").abs.Y; y != 30 {
		t.Fatalf("with nothing rendered, b at y=%v, want 30 (the empty slot must not add a gap)", y)
	}
	show.Set(true)
	frame(rt)
	if y := findKey(rt.tree, "b").abs.Y; y != 60 {
		t.Errorf("after showing mid, b at y=%v, want 60", y)
	}
	show.Set(false)
	frame(rt)
	if y := findKey(rt.tree, "b").abs.Y; y != 30 {
		t.Errorf("after hiding mid, b at y=%v, want 30", y)
	}
}

func TestRemovedComponentsAreDisposed(t *testing.T) {
	show := NewSignal(true)
	inner := NewSignal(0)
	renders := 0
	rt := newTestRuntime(func(ctx *Context) Element {
		if !show.Get() {
			return Text("gone")
		}
		return Component(func(ctx *Context) Element {
			renders++
			return Div().Key("scroller").Height(Px(50)).ScrollY().Child(Textf("%d", inner.Get()))
		})
	}, 400, 300, 1)
	instances, states := len(rt.instances), len(rt.states)
	show.Set(false)
	frame(rt)
	if len(rt.instances) >= instances || len(rt.states) >= states {
		t.Errorf("instances %d→%d, states %d→%d; removed component not cleaned up",
			instances, len(rt.instances), states, len(rt.states))
	}
	inner.Set(1)
	if rt.NeedsFrame() || renders != 1 {
		t.Errorf("signal of a removed component still triggers work (renders=%d)", renders)
	}
}

// ---- #3: wake coalescing ----

func TestManyUpdatesWakeOnce(t *testing.T) {
	values := make([]*Signal[string], 50)
	for i := range values {
		values[i] = NewSignal("x")
	}
	rt := newTestRuntime(rowsApp(values), 600, 4000, 1)
	var wakes atomic.Int32
	rt.SetWakeFunc(func() { wakes.Add(1) })
	for _, v := range values {
		v.Set("y")
	}
	rt.PointerMove(5, 5, 0)
	if n := wakes.Load(); n != 1 {
		t.Errorf("%d wake-ups before the frame, want 1", n)
	}
	frame(rt)
	values[0].Set("z")
	if n := wakes.Load(); n != 2 {
		t.Errorf("a change after the frame should wake again: wakes=%d", n)
	}
}

// ---- #2: signals from other goroutines (run with -race) ----

func TestSignalsFromOtherGoroutines(t *testing.T) {
	values := make([]*Signal[string], 20)
	for i := range values {
		values[i] = NewSignal("0")
	}
	rt := newTestRuntime(rowsApp(values), 600, 2000, 1)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				v := values[(i+g)%len(values)]
				_ = v.Get() // may run while a component renders
				v.Set(fmt.Sprint(i))
			}
		}(g)
	}
	for i := 0; i < 200; i++ {
		frame(rt)
	}
	close(stop)
	wg.Wait()
	frame(rt)
	for i, v := range values {
		if findText(rt.tree, v.Peek()) == nil {
			t.Errorf("row %d does not show its final value %q", i, v.Peek())
		}
	}
}

// ---- #6: identity ----

func keyedRows(order *Signal[[]string]) func(*Context) Element {
	return func(ctx *Context) Element {
		list := Div()
		for _, k := range order.Get() {
			list.Child(Div().Key(k).Height(Px(30)).Focusable().ScrollY().
				Child(Div().Height(Px(200))).
				Child(Text(k)))
		}
		return list
	}
}

func TestKeyedReorderKeepsStateWithElement(t *testing.T) {
	order := NewSignal([]string{"a", "b", "c"})
	rt := newTestRuntime(keyedRows(order), 300, 300, 1)
	// Scroll and focus row "b" (y 30..60).
	rt.Scroll(10, 45, 0, 50, true, 0)
	rt.PointerDown(10, 45, input.ButtonLeft, 1, 0)
	rt.PointerUp(10, 45, input.ButtonLeft, 0)
	frame(rt)
	b := findKey(rt.tree, "b")
	if b.st.scrollY != 50 || rt.focusID != b.id {
		t.Fatalf("setup: b scroll=%v focused=%v", b.st.scrollY, rt.focusID == b.id)
	}

	order.Set([]string{"c", "a", "b"})
	frame(rt)
	b, a := findKey(rt.tree, "b"), findKey(rt.tree, "a")
	if b.st.scrollY != 50 || a.st.scrollY != 0 {
		t.Errorf("after reorder scroll a=%v b=%v; want state to follow the key", a.st.scrollY, b.st.scrollY)
	}
	if rt.focusID != b.id {
		t.Error("focus did not follow the keyed element")
	}

	order.Set([]string{"new", "c", "a", "b"}) // insert at the front
	frame(rt)
	if findKey(rt.tree, "b").st.scrollY != 50 || findKey(rt.tree, "new").st.scrollY != 0 {
		t.Error("inserting an element moved state between keyed siblings")
	}

	order.Set([]string{"new", "c", "a"}) // remove the focused element
	frame(rt)
	if rt.focusID != 0 {
		t.Error("focus still points at a removed element")
	}
	order.Set([]string{"new", "c", "a", "b"}) // bring it back: fresh state
	frame(rt)
	if findKey(rt.tree, "b").st.scrollY != 0 {
		t.Error("a removed and re-added element resurrected its old scroll state")
	}
}

func TestUnkeyedStateFollowsPosition(t *testing.T) {
	// Documented behavior: without keys, identity is the sibling position.
	// Inserting at the front therefore shifts state to the next element,
	// which is exactly why lists of dynamic data should use Key.
	items := NewSignal([]string{"a", "b"})
	rt := newTestRuntime(func(ctx *Context) Element {
		list := Div()
		for _, s := range items.Get() {
			list.Child(Div().Height(Px(30)).ScrollY().Child(Div().Height(Px(200))).Child(Text(s)))
		}
		return list
	}, 300, 300, 1)
	rt.Scroll(10, 15, 0, 40, true, 0) // scroll the first row ("a")
	frame(rt)
	items.Set([]string{"z", "a", "b"})
	frame(rt)
	if got := findText(rt.tree, "z").parent.st.scrollY; got != 40 {
		t.Errorf("unkeyed: new first row scroll=%v, want 40 (state stays at position 0)", got)
	}
}

func TestTypeChangeResetsState(t *testing.T) {
	asText := NewSignal(false)
	rt := newTestRuntime(func(ctx *Context) Element {
		if asText.Get() {
			return Div().Child(Text("plain"))
		}
		return Div().Child(Div().Height(Px(30)).ScrollY().Child(Div().Height(Px(200))))
	}, 300, 300, 1)
	rt.Scroll(10, 15, 0, 40, true, 0)
	frame(rt)
	asText.Set(true)
	frame(rt)
	asText.Set(false)
	frame(rt)
	if got := rt.tree.children[0].st.scrollY; got != 0 {
		t.Errorf("state survived a change of element type at the same position: scroll=%v", got)
	}
}

func TestDuplicateKeysWarnAndStaySeparate(t *testing.T) {
	var warnings []string
	rt := NewRuntime(func(ctx *Context) Element {
		return Div().
			Child(Div().Key("dup").Height(Px(30)).ScrollY().Child(Div().Height(Px(200)))).
			Child(Div().Key("dup").Height(Px(30)).ScrollY().Child(Div().Height(Px(200))))
	}, DarkTheme(), nil)
	rt.SetWarnFunc(func(s string) { warnings = append(warnings, s) })
	rt.SetViewport(300, 300, 1)
	frame(rt)
	if len(warnings) != 1 || !strings.Contains(warnings[0], `duplicate key "dup"`) {
		t.Fatalf("warnings = %q, want one duplicate-key warning", warnings)
	}
	first, second := rt.tree.children[0], rt.tree.children[1]
	if first.id == second.id || first.st == second.st {
		t.Fatal("siblings with the same key share identity/state")
	}
	rt.Scroll(10, 45, 0, 40, true, 0) // scroll the second one
	frame(rt)
	if first.st.scrollY != 0 || second.st.scrollY != 40 {
		t.Errorf("scroll first=%v second=%v; state leaked between duplicates", first.st.scrollY, second.st.scrollY)
	}
}

func TestShrinkingScrollRangeClampsImmediately(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		list := Div().Height(Full).ScrollY()
		for i := 0; i < 20; i++ {
			list.Child(Div().Height(Px(50)).Shrink(0))
		}
		return list
	}, 300, 400, 1) // content 1000, viewport 400: max scroll 600
	rt.Scroll(10, 10, 0, 600, true, 0)
	frame(rt)
	rt.SetViewport(300, 700, 1) // max scroll is now 300
	frame(rt)
	if st := rt.tree.st; st.scrollY != 300 || rt.animating {
		t.Errorf("after the window grew, scrollY=%v animating=%v; want an immediate clamp to 300", st.scrollY, rt.animating)
	}
}
