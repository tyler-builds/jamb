package ui

import (
	"math"
	"sort"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/layout"
)

// splitRig is a headless window of 500 by 400 logical pixels whose root is the split that build
// returns.
func splitRig(t *testing.T, build func() Element) *accessRig {
	t.Helper()
	return newAccessRig(func(ctx *Context) Element { return build() })
}

// splitParts returns the nodes drawn for part, in order along the split: left to right, or top to
// bottom when vert.
func splitParts(r *accessRig, part string, vert bool) []*node {
	ns := partNodes(r.rt, part)
	sort.Slice(ns, func(i, j int) bool {
		a, b := r.rt.nodeRect(ns[i]), r.rt.nodeRect(ns[j])
		if vert {
			return a.Y < b.Y
		}
		return a.X < b.X
	})
	return ns
}

// handlesOf returns the dividers of the split, in order along it.
func handlesOf(r *accessRig, vert bool) []*node { return splitParts(r, "Split.Handle", vert) }

// handleNode returns the one divider of a split of two panes.
func handleNode(t *testing.T, r *accessRig) *node {
	t.Helper()
	return partNode(t, r.rt, "Split.Handle", "")
}

// containsProperty reports whether ps has the property p.
func containsProperty(ps []access.Property, p access.Property) bool {
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}

// paneSizes returns the widths of the panes of a flat split, in order, or their heights when vert.
// A collapsed pane is not drawn, so it is left out.
func paneSizes(r *accessRig, vert bool) []float64 {
	var out []float64
	for _, n := range splitParts(r, "Split.Pane", vert) {
		rc := r.rt.nodeRect(n)
		size := rc.W
		if vert {
			size = rc.H
		}
		if size > 0 {
			out = append(out, float64(size))
		}
	}
	return out
}

// wantLength fails unless got is want in the unit given.
func wantLength(t *testing.T, what string, got Length, unit layout.Unit, want float64) {
	t.Helper()
	if got.Unit != unit || math.Abs(float64(got.Value)-want) > 0.01 {
		t.Fatalf("%s is %v, want %v in unit %v", what, got, want, unit)
	}
}

// dragDivider presses the divider, moves it by (dx, dy) and releases it.
func dragDivider(r *accessRig, n *node, dx, dy float32) {
	rc := r.rt.nodeRect(n)
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.frame()
	r.rt.PointerMove(x+dx/2, y+dy/2, 0)
	r.frame()
	r.rt.PointerMove(x+dx, y+dy, 0)
	r.frame()
	r.rt.PointerUp(x+dx, y+dy, input.ButtonLeft, 0)
	r.frame()
}

// clicksSeen is the count of the counting pane, as its last render showed it.
var clicksSeen int

// countingPane is a pane that counts its clicks in a Local of its own. Its element has the key.
func countingPane(key string) Element {
	return Component(func(ctx *Context) Element {
		n := Local(ctx, "clicks", 0)
		clicksSeen = n.Get()
		return Div().Key(key).Height(Px(40)).Focusable().OnClick(func() { n.Set(n.Peek() + 1) })
	})
}

func TestSplitSizesPanesInTurn(t *testing.T) {
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).Size(Px(200)), Pane(Div()), Pane(Div()).Size(Percent(20)))
	})
	checkSizes(t, "widths", paneSizes(r, false), []float64{200, 185.6, 96.4})
	if n := len(handlesOf(r, false)); n != 2 {
		t.Fatalf("three panes have %d dividers, want 2", n)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSplitOnePaneRendersAloneAndNoPaneRendersNothing(t *testing.T) {
	r := splitRig(t, func() Element { return Split(Pane(Div()).Size(Px(200))) })
	if n := len(handlesOf(r, false)); n != 0 {
		t.Fatalf("one pane has %d dividers, want none", n)
	}
	checkSizes(t, "the pane", paneSizes(r, false), []float64{500})

	r = splitRig(t, func() Element { return Split() })
	if n := len(handlesOf(r, false)); n != 0 {
		t.Fatalf("no panes have %d dividers, want none", n)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSplitWindowResizeKeepsPixelsScalesPercentAndFillAbsorbs(t *testing.T) {
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).Size(Px(200)), Pane(Div()).Size(Percent(40)), Pane(Div()))
	})
	checkSizes(t, "widths at 500", paneSizes(r, false), []float64{200, 192.8, 89.2})
	r.rt.SetViewport(800, 400, 1)
	r.frame()
	checkSizes(t, "widths at 800", paneSizes(r, false), []float64{200, 312.8, 269.2})
}

func TestSplitOverConstrainedPanesShrinkAndUnderConstrainedLastPaneTakesTheRest(t *testing.T) {
	over := NewSignal(Px(400))
	overTwo := NewSignal(Px(400))
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).SizeSignal(over), Pane(Div()).SizeSignal(overTwo))
	})
	checkSizes(t, "over-constrained widths", paneSizes(r, false), []float64{245.5, 245.5})
	wantLength(t, "the first size", over.Peek(), layout.UnitPx, 400)

	under := NewSignal(Px(100))
	underTwo := NewSignal(Px(100))
	r = splitRig(t, func() Element {
		return Split(Pane(Div()).SizeSignal(under), Pane(Div()).SizeSignal(underTwo))
	})
	checkSizes(t, "under-constrained widths", paneSizes(r, false), []float64{100, 391})
	wantLength(t, "the stored size of the last pane", underTwo.Peek(), layout.UnitPx, 100)
}

func TestSplitDragBetweenPixelAndFillStoresPixels(t *testing.T) {
	sig := NewSignal(Px(200))
	r := splitRig(t, func() Element { return Split(Pane(Div()).SizeSignal(sig), Pane(Div())) })
	dragDivider(r, handlesOf(r, false)[0], 50, 0)
	wantLength(t, "the pixel pane", sig.Peek(), layout.UnitPx, 250)
	checkSizes(t, "widths after the drag", paneSizes(r, false), []float64{250, 241})
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSplitDragBetweenPercentAndFillStoresPercent(t *testing.T) {
	sig := NewSignal(Percent(40))
	r := splitRig(t, func() Element { return Split(Pane(Div()).SizeSignal(sig), Pane(Div())) })
	dragDivider(r, handlesOf(r, false)[0], 50, 0)
	wantLength(t, "the percentage pane", sig.Peek(), layout.UnitPercent, (196.4+50)/491*100)
}

func TestSplitDragBetweenPixelAndPercentMovesBothInTheirUnits(t *testing.T) {
	px := NewSignal(Px(200))
	pct := NewSignal(Percent(30))
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).SizeSignal(px), Pane(Div()).SizeSignal(pct))
	})
	dragDivider(r, handlesOf(r, false)[0], 50, 0)
	wantLength(t, "the pixel pane", px.Peek(), layout.UnitPx, 250)
	// The percentage pane gave up the same 50 pixels of the 491 available to the panes.
	wantLength(t, "the percentage pane", pct.Peek(), layout.UnitPercent, (0.3*491-50)/491*100)
}

func TestSplitDragStopsAtTheLimitOfEachUnit(t *testing.T) {
	t.Run("most in pixels", func(t *testing.T) {
		sig := NewSignal(Px(200))
		r := splitRig(t, func() Element {
			return Split(Pane(Div()).SizeSignal(sig).Max(Px(300)), Pane(Div()))
		})
		dragDivider(r, handlesOf(r, false)[0], 400, 0)
		wantLength(t, "the pixel pane", sig.Peek(), layout.UnitPx, 300)
	})
	t.Run("least of the fill pane", func(t *testing.T) {
		sig := NewSignal(Px(200))
		r := splitRig(t, func() Element {
			return Split(Pane(Div()).SizeSignal(sig), Pane(Div()).Min(Px(160)))
		})
		dragDivider(r, handlesOf(r, false)[0], 400, 0)
		wantLength(t, "the pixel pane", sig.Peek(), layout.UnitPx, 331)
	})
	t.Run("least in pixels, dragged back", func(t *testing.T) {
		sig := NewSignal(Px(200))
		r := splitRig(t, func() Element {
			return Split(Pane(Div()).SizeSignal(sig).Min(Px(120)), Pane(Div()))
		})
		dragDivider(r, handlesOf(r, false)[0], -400, 0)
		wantLength(t, "the pixel pane", sig.Peek(), layout.UnitPx, 120)
	})
	t.Run("most in percent", func(t *testing.T) {
		sig := NewSignal(Percent(40))
		r := splitRig(t, func() Element {
			return Split(Pane(Div()).SizeSignal(sig).Max(Percent(50)), Pane(Div()))
		})
		dragDivider(r, handlesOf(r, false)[0], 400, 0)
		wantLength(t, "the percentage pane", sig.Peek(), layout.UnitPercent, 50)
	})
}

func TestSplitDragBetweenTwoFillPanesMovesOnlyThem(t *testing.T) {
	r := splitRig(t, func() Element { return Split(Pane(Div()), Pane(Div()), Pane(Div())) })
	checkSizes(t, "widths before the drag", paneSizes(r, false), []float64{160.67, 160.67, 160.67})
	dragDivider(r, handlesOf(r, false)[0], 100, 0)
	checkSizes(t, "widths after the drag", paneSizes(r, false), []float64{260.67, 60.67, 160.67})
}

func TestSplitDragBetweenPercentPanesInAVerticalSplit(t *testing.T) {
	sig := NewSignal(Percent(30))
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).SizeSignal(sig), Pane(Div())).Orientation(SplitVertical)
	})
	// The split is 400 high, less the 9 px divider: 391 px for the panes.
	dragDivider(r, handlesOf(r, true)[0], 0, 40)
	wantLength(t, "the top pane", sig.Peek(), layout.UnitPercent, (0.3*391+40)/391*100)
}

func TestSplitOnChangeRunsOncePerChangedDragStep(t *testing.T) {
	changes := 0
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).Size(Px(200)), Pane(Div())).OnChange(func() { changes++ })
	})
	dragDivider(r, handlesOf(r, false)[0], 40, 0)
	// The pointer moves to half the distance and then to the end: two changes, and the release none.
	if changes != 2 {
		t.Fatalf("OnChange ran %d times in the drag, want 2", changes)
	}
}

func TestSplitKeysMovePixelPanesInPixels(t *testing.T) {
	sig := NewSignal(Px(200))
	changes := 0
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).SizeSignal(sig).Min(Px(100)).Max(Px(300)), Pane(Div()).Min(Px(160))).
			OnChange(func() { changes++ })
	})
	focusNode(r, handlesOf(r, false)[0])
	r.press(input.KeyRight, 0)
	wantLength(t, "after Right", sig.Peek(), layout.UnitPx, 210)
	r.press(input.KeyRight, input.ModShift)
	wantLength(t, "after Shift+Right", sig.Peek(), layout.UnitPx, 260)
	r.press(input.KeyLeft, 0)
	wantLength(t, "after Left", sig.Peek(), layout.UnitPx, 250)
	r.press(input.KeyEnd, 0)
	wantLength(t, "after End", sig.Peek(), layout.UnitPx, 300)
	r.press(input.KeyEnd, 0) // at the most: no change, no OnChange
	r.press(input.KeyHome, 0)
	wantLength(t, "after Home", sig.Peek(), layout.UnitPx, 100)
	if changes != 5 {
		t.Fatalf("OnChange ran %d times, want 5: one per change", changes)
	}
	r.press(input.KeyUp, 0) // not a key of a horizontal split
	wantLength(t, "after Up", sig.Peek(), layout.UnitPx, 100)
}

func TestSplitKeysMovePercentPanesInPercent(t *testing.T) {
	sig := NewSignal(Percent(40))
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).SizeSignal(sig).Max(Percent(60)), Pane(Div()))
	})
	focusNode(r, handlesOf(r, false)[0])
	r.press(input.KeyRight, 0)
	wantLength(t, "after Right", sig.Peek(), layout.UnitPercent, 45)
	r.press(input.KeyRight, input.ModShift)
	wantLength(t, "after Shift+Right", sig.Peek(), layout.UnitPercent, 55)
	r.press(input.KeyEnd, 0)
	wantLength(t, "after End", sig.Peek(), layout.UnitPercent, 60)
	r.press(input.KeyHome, 0)
	wantLength(t, "after Home", sig.Peek(), layout.UnitPercent, 0)
}

func TestSplitKeysInAVerticalSplitUseUpAndDown(t *testing.T) {
	sig := NewSignal(Px(100))
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).SizeSignal(sig), Pane(Div())).Orientation(SplitVertical)
	})
	focusNode(r, handlesOf(r, true)[0])
	r.press(input.KeyDown, 0)
	wantLength(t, "after Down", sig.Peek(), layout.UnitPx, 110)
	r.press(input.KeyUp, 0)
	wantLength(t, "after Up", sig.Peek(), layout.UnitPx, 100)
}

func TestSplitDividerIsAccessible(t *testing.T) {
	sig := NewSignal(Px(250))
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).SizeSignal(sig).Min(Px(100)).Label("Folders"), Pane(Div()).Label("Editor")).
			Label("Resize sidebar")
	})
	// The split learns its length one frame after its first render; the window settles before the
	// tree is read.
	r.frame()
	var sep *access.Node
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleSeparator {
			sep = n
		}
		return true
	})
	if sep == nil {
		t.Fatal("the divider is not in the accessibility tree")
	}
	if sep.Name != "Resize sidebar" {
		t.Fatalf("divider name %q", sep.Name)
	}
	want := access.Range{Value: 250, Min: 100, Max: 491, SmallChange: 10, LargeChange: 50}
	if sep.Range == nil || *sep.Range != want {
		t.Fatalf("divider range %+v, want %+v", sep.Range, want)
	}
	if !sep.States.Has(access.StateFocusable) {
		t.Fatal("the divider is not focusable")
	}
	var props []access.Property
	r.bridge.Listen(func(u access.Update) {
		for _, pc := range u.Properties {
			props = append(props, pc.Property)
		}
	})
	handlesOf(r, false)[0].div.acc.setRange(300)
	r.frame()
	wantLength(t, "the pixel pane after the range is set", sig.Peek(), layout.UnitPx, 300)
	if !containsProperty(props, access.PropertyRangeValue) {
		t.Fatalf("no RangeValue change was published; got %v", props)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSplitDividerRangeOfAPercentagePane(t *testing.T) {
	r := splitRig(t, func() Element { return Split(Pane(Div()).Size(Percent(40)), Pane(Div())) })
	r.frame()
	var sep *access.Node
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleSeparator {
			sep = n
		}
		return true
	})
	if sep == nil || sep.Range == nil {
		t.Fatal("the divider has no range")
	}
	if sep.Name != "Resize panes" {
		t.Fatalf("the default divider name is %q, want the window's Resize panes", sep.Name)
	}
	if math.Abs(sep.Range.Value-40) > 0.01 || math.Abs(sep.Range.Max-100) > 0.01 || sep.Range.SmallChange != 5 || sep.Range.LargeChange != 10 {
		t.Fatalf("divider range %+v, want value 40, most 100, steps 5 and 10 percent", sep.Range)
	}
	handlesOf(r, false)[0].div.acc.setRange(50)
	r.frame()
	checkSizes(t, "widths after the range is set", paneSizes(r, false), []float64{245.5, 245.5})
}

func TestSplitEnterCollapsesAndRestoresTheSize(t *testing.T) {
	collapsed := NewSignal(false)
	r := splitRig(t, func() Element {
		return Split(Pane(countingPane("left")).Size(Px(200)).Collapsed(collapsed), Pane(Div().Key("right")))
	})
	r.clickKey(t, "left")
	focusNode(r, handlesOf(r, false)[0])
	r.press(input.KeyEnter, 0)
	if !collapsed.Peek() {
		t.Fatal("Enter does not collapse the first pane")
	}
	r.press(input.KeyRight, 0)
	if !collapsed.Peek() {
		t.Fatal("a key restored the collapsed pane")
	}
	r.press(input.KeyEnter, 0)
	if collapsed.Peek() {
		t.Fatal("Enter does not restore the collapsed pane")
	}
	checkSizes(t, "the restored widths", paneSizes(r, false), []float64{200, 291})
	// The pane's own state survives the collapse: the click count, kept in a Local of the pane,
	// shows on the next click after the restore.
	r.clickKey(t, "left")
	if clicksSeen != 2 {
		t.Fatalf("the pane's count is %d after a click that followed the restore, want 2", clicksSeen)
	}
}

func TestSplitDoubleClickCollapsesTheLastPane(t *testing.T) {
	collapsed := NewSignal(false)
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).Size(Px(200)), Pane(Div()).Collapsible().Collapsed(collapsed))
	})
	rc := r.rt.nodeRect(handlesOf(r, false)[0])
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 2, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
	if !collapsed.Peek() {
		t.Fatal("a double-click does not collapse the last pane")
	}
	// The collapsed pane is not drawn, so the first pane takes the whole length less the divider.
	checkSizes(t, "widths with the last pane collapsed", paneSizes(r, false), []float64{491})
}

func TestSplitKeepsAPaneStateWhenAPaneIsInsertedBefore(t *testing.T) {
	inserted := NewSignal(false)
	r := splitRig(t, func() Element {
		return Component(func(ctx *Context) Element {
			panes := []*PaneElement{Pane(countingPane("left")).Key("a").Size(Px(200)), Pane(Div().Key("right")).Key("b")}
			if inserted.Get() {
				panes = append([]*PaneElement{Pane(Div().Key("new")).Key("x").Size(Px(50))}, panes...)
			}
			return Split(panes...)
		})
	})
	r.clickKey(t, "left")
	r.clickKey(t, "left")
	inserted.Set(true)
	r.frame()
	r.clickKey(t, "left")
	if clicksSeen != 3 {
		t.Fatalf("the pane's count is %d after a pane was inserted before it, want 3", clicksSeen)
	}
}

// TestSplitDragWhileOverConstrainedStartsFromTheShownSizes checks a drag that begins while the sized
// panes need more than the length. The divider starts from the sizes shown, which the layout shrank
// toward the least sizes, and it stops at the least size of the pane it can shrink.
func TestSplitDragWhileOverConstrainedStartsFromTheShownSizes(t *testing.T) {
	a, b := NewSignal(Px(400)), NewSignal(Px(400))
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).SizeSignal(a), Pane(Div()).SizeSignal(b).Min(Px(200)))
	})
	// The sizes need 800 px of the 491 available. The excess of 309 px is shared by the panes in
	// proportion to what each has above its least size (400 and 200 px): 194 and 297 px.
	checkSizes(t, "shown before the drag", paneSizes(r, false), []float64{194, 297})
	dragDivider(r, handlesOf(r, false)[0], 150, 0)
	// The second pane can give up only 97 px before its least size of 200 px.
	wantLength(t, "the first pane", a.Peek(), layout.UnitPx, 291)
	wantLength(t, "the second pane", b.Peek(), layout.UnitPx, 200)
	checkSizes(t, "shown after the drag", paneSizes(r, false), []float64{291, 200})
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// TestSplitVerticalWindowResizeKeepsPixelsScalesPercentAndFillAbsorbs checks the sizes of a stacked
// split when the window is resized: a pixel pane keeps its height, a percentage pane scales, and the
// fill pane takes the change. The split is 400 high, which leaves 382 px for the panes.
func TestSplitVerticalWindowResizeKeepsPixelsScalesPercentAndFillAbsorbs(t *testing.T) {
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).Size(Px(100)), Pane(Div()).Size(Percent(30)), Pane(Div())).Orientation(SplitVertical)
	})
	checkSizes(t, "heights at 400", paneSizes(r, true), []float64{100, 114.6, 167.4})
	r.rt.SetViewport(500, 600, 1)
	r.frame()
	checkSizes(t, "heights at 600", paneSizes(r, true), []float64{100, 174.6, 307.4})
}

// TestSplitDividerNamedAfterItsLabelledPanes checks the names of the dividers: one between two labelled
// panes takes their names, a divider with an unlabelled pane takes the default, and the split's Label
// names every divider.
func TestSplitDividerNamedAfterItsLabelledPanes(t *testing.T) {
	names := func(r *accessRig) []string {
		r.frame()
		var out []string
		r.tree().Walk(func(n *access.Node) bool {
			if n.Role == access.RoleSeparator {
				out = append(out, n.Name)
			}
			return true
		})
		return out
	}
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).Label("Sidebar").Size(Px(200)), Pane(Div()).Label("Editor"), Pane(Div()).Label("Inspector"))
	})
	checkNames(t, names(r), []string{"Resize Sidebar and Editor", "Resize Editor and Inspector"})

	r = splitRig(t, func() Element {
		return Split(Pane(Div()).Label("Sidebar").Size(Px(200)), Pane(Div()), Pane(Div()).Label("Inspector"))
	})
	checkNames(t, names(r), []string{"Resize panes", "Resize panes"})

	r = splitRig(t, func() Element {
		return Split(Pane(Div()).Label("Sidebar").Size(Px(200)), Pane(Div()).Label("Editor")).Label("Resize all")
	})
	checkNames(t, names(r), []string{"Resize all"})

	// The Italian pack writes the same name in Italian word order.
	if got := italianStrings().ResizeBetween("Barra", "Editor"); got != "Ridimensiona Barra e Editor" {
		t.Fatalf("the Italian name is %q", got)
	}
}

// checkNames fails unless got has the names want, in order.
func checkNames(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("separators %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("separators %q, want %q", got, want)
		}
	}
}

// TestSplitNestedSplitFollowsItsParentDivider checks that a nested split sizes its panes to the
// length its parent gives it when the parent's divider moves. The nested split's element is the same
// as before the drag, so it renders again only because its length changed.
func TestSplitNestedSplitFollowsItsParentDivider(t *testing.T) {
	r := splitRig(t, func() Element {
		inner := Split(Pane(Div().Key("a")), Pane(Div().Key("b")))
		return Split(Pane(Div().Key("side")).Size(Px(150)), Pane(inner)).Label("Resize outer")
	})
	// The contents of the inner split's panes are keyed, so their widths are read by key.
	widths := func() []float64 {
		var out []float64
		for _, key := range []string{"side", "a", "b"} {
			out = append(out, float64(r.rt.nodeRect(r.nodeByKey(t, key)).W))
		}
		return out
	}
	checkSizes(t, "widths before the drag", widths(), []float64{150, 166, 166})
	dragDivider(r, handlesOf(r, false)[0], 50, 0)
	checkSizes(t, "widths after the drag", widths(), []float64{200, 141, 141})
}

func TestSplitNestedSplitsHaveTheirOwnDividers(t *testing.T) {
	r := splitRig(t, func() Element {
		inner := Split(Pane(Div().Key("a")), Pane(Div().Key("b"))).Label("Resize inner")
		return Split(Pane(Div().Key("side")).Size(Px(150)), Pane(inner)).Label("Resize outer")
	})
	if n := len(partNodes(r.rt, "Split.Handle")); n != 2 {
		t.Fatalf("a nested split has %d dividers, want 2", n)
	}
	seps := 0
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleSeparator {
			seps++
		}
		return true
	})
	if seps != 2 {
		t.Fatalf("the accessibility tree has %d separators, want 2", seps)
	}
}

func TestSplitPaneGroupsAreNamed(t *testing.T) {
	r := splitRig(t, func() Element {
		return Split(Pane(Div()).Label("Folders"), Pane(Div()).Label("Editor"))
	})
	names := map[string]bool{}
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleGroup {
			names[n.Name] = true
		}
		return true
	})
	if !names["Folders"] || !names["Editor"] {
		t.Fatalf("pane groups are %v, want Folders and Editor", names)
	}
}

func TestSplitStylesReachTheParts(t *testing.T) {
	st := SplitStyle{Divider: Style{Background: markBG}, Pane: Style{Background: markFG}}
	r := splitRig(t, func() Element {
		return Split(Pane(Div()), Pane(Div())).Styles(st)
	})
	wantBackground(t, partNode(t, r.rt, "Split.Divider", ""), "Split.Divider", markBG)
	wantBackground(t, splitParts(r, "Split.Pane", false)[0], "Split.Pane", markFG)
}

func TestSplitSizingMethodsSizeTheBox(t *testing.T) {
	// The window's root is sized by the window, so the split sits in a column.
	r := splitRig(t, func() Element {
		return Div().Column().Child(Split(Pane(Div()), Pane(Div())).Width(Px(300)))
	})
	if w := r.rt.nodeRect(splitParts(r, "Split.Pane", false)[0].parent).W; math.Abs(float64(w)-300) > 0.5 {
		t.Fatalf("Width(Px(300)) gives a split %v wide", w)
	}
}

// TestSplitThemeAndClear checks the layers of the look: the theme's Split is
// under the instance's Styles, and a Clear in a part removes the default fill.
func TestSplitThemeAndClear(t *testing.T) {
	th := *DarkTheme()
	th.Split.Divider = Style{Background: markBG}
	r := newThemedRig(&th, func(ctx *Context) Element { return partSplit(SplitStyle{}) })
	wantBackground(t, partNode(t, r.rt, "Split.Divider", ""), "Split.Divider", markBG)

	r = newThemedRig(&th, func(ctx *Context) Element {
		return partSplit(SplitStyle{Divider: Style{Background: markFG}})
	})
	wantBackground(t, partNode(t, r.rt, "Split.Divider", ""), "Split.Divider", markFG)

	r = newThemedRig(DarkTheme(), func(ctx *Context) Element {
		return partSplit(SplitStyle{Divider: Style{Clear: ClearBackground}})
	})
	if n := partNode(t, r.rt, "Split.Divider", ""); !n.style.Background.IsZero() {
		t.Fatalf("a Clear of the background left %v on the divider", n.style.Background)
	}
}

// TestSplitHighContrastDividerIsVisible checks that the bar is the system's text
// color, that the hover and drag states add no color, and that a theme's override
// loses its color under high contrast.
func TestSplitHighContrastDividerIsVisible(t *testing.T) {
	sc := HighContrastBlack()
	th := *HighContrastTheme(sc, DarkTheme())
	th.Split.Divider = Style{Background: markBG}
	r := newThemedRig(&th, func(ctx *Context) Element { return partSplit(SplitStyle{}) })
	bar := partNode(t, r.rt, "Split.Divider", "")
	if bar.style.Background != th.Text {
		t.Fatalf("under high contrast the bar is %v, want the system text color %v", bar.style.Background, th.Text)
	}
	hoverNode(r, bar)
	if got := partNode(t, r.rt, "Split.Divider", ""); got.style.Background == th.Accent {
		t.Fatal("under high contrast the hover added the accent color")
	}
	if got := partNode(t, r.rt, "Split.Handle", ""); got == nil {
		t.Fatal("no handle")
	}
	focusNode(r, partNode(t, r.rt, "Split.Handle", ""))
	if n := partNode(t, r.rt, "Split.Divider", ""); len(n.style.Shadows) == 0 {
		t.Fatal("under high contrast the focused divider has no focus ring")
	}
}

// TestSplitDividerShowsTheResizeCursor checks that the divider shows the resize
// cursor of its orientation: east-west for a split side by side, north-south for a
// stacked one.
func TestSplitDividerShowsTheResizeCursor(t *testing.T) {
	r := splitRig(t, func() Element { return Split(Pane(Div()), Pane(Div())) })
	if c := handlesOf(r, false)[0].style.Cursor; c != input.CursorResizeEW {
		t.Fatalf("a horizontal split shows cursor %v, want the east-west resize cursor", c)
	}
	r = splitRig(t, func() Element { return Split(Pane(Div()), Pane(Div())).Orientation(SplitVertical) })
	if c := handlesOf(r, true)[0].style.Cursor; c != input.CursorResizeNS {
		t.Fatalf("a vertical split shows cursor %v, want the north-south resize cursor", c)
	}
}
