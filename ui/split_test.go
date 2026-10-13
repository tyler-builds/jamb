package ui

import (
	"math"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// splitRig is a headless window of 500 by 400 logical pixels whose root is the
// split that build returns.
func splitRig(t *testing.T, build func() Element) *accessRig {
	t.Helper()
	r := newAccessRig(func(ctx *Context) Element { return build() })
	return r
}

// handleNode returns the divider's focusable element.
func handleNode(t *testing.T, r *accessRig) *node {
	t.Helper()
	return partNode(t, r.rt, "Split.Handle", "")
}

// panesByX returns the split's two panes, the first on the left (or on top).
func panesByX(t *testing.T, r *accessRig) []*node {
	t.Helper()
	panes := partNodes(r.rt, "Split.Pane")
	if len(panes) != 2 {
		t.Fatalf("want 2 panes, got %d", len(panes))
	}
	if r.rt.nodeRect(panes[1]).X < r.rt.nodeRect(panes[0]).X {
		panes[0], panes[1] = panes[1], panes[0]
	}
	return panes
}

// paneWidth returns the width of the pane at index i (0 first, 1 second).
func paneWidth(t *testing.T, r *accessRig, i int) float32 {
	t.Helper()
	return r.rt.nodeRect(panesByX(t, r)[i]).W
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

// splitWith returns a horizontal fraction split of two labelled panes, with the
// position in pos and the options of opt applied.
func splitWith(pos *Signal[float64], opt func(*SplitElement) *SplitElement) func() Element {
	return func() Element {
		s := Split(Div().Key("left"), Div().Key("right")).Position(pos)
		if opt != nil {
			s = opt(s)
		}
		return s
	}
}

func TestSplitDragMovesTheDividerAndKeepsTheProportion(t *testing.T) {
	pos := NewSignal(0.5)
	r := splitRig(t, splitWith(pos, nil))
	if w := paneWidth(t, r, 0); math.Abs(float64(w)-250) > 0.5 {
		t.Fatalf("first pane is %v wide, want 250 for a half split of 500", w)
	}
	dragDivider(r, handleNode(t, r), 50, 0)
	// The divider's hit area is fixed, so the first pane's share is of the whole split.
	if got := pos.Peek(); math.Abs(got-0.6) > 0.01 {
		t.Fatalf("position after a drag of 50 px is %v, want 0.6", got)
	}
	if w := paneWidth(t, r, 0); math.Abs(float64(w)-300) > 1 {
		t.Fatalf("first pane is %v wide after the drag, want 300", w)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSplitDragRespectsTheLimits(t *testing.T) {
	pos := NewSignal(0.5)
	r := splitRig(t, splitWith(pos, func(s *SplitElement) *SplitElement {
		return s.MinFirst(Px(120)).MaxFirst(Px(300))
	}))
	dragDivider(r, handleNode(t, r), 400, 0)
	if got := pos.Peek(); math.Abs(got-0.6) > 0.01 {
		t.Fatalf("a drag past MaxFirst(300) gives %v, want 300/500 = 0.6", got)
	}
	dragDivider(r, handleNode(t, r), -600, 0)
	if got := pos.Peek(); math.Abs(got-0.24) > 0.01 {
		t.Fatalf("a drag past MinFirst(120) gives %v, want 120/500 = 0.24", got)
	}
}

func TestSplitMinSecondLimitsTheFirstPane(t *testing.T) {
	pos := NewSignal(0.5)
	r := splitRig(t, splitWith(pos, func(s *SplitElement) *SplitElement {
		return s.MinSecond(Px(200))
	}))
	dragDivider(r, handleNode(t, r), 400, 0)
	// 500 - 9 (the hit area) - 200 (MinSecond) = 291 pixels for the first pane.
	if got := pos.Peek(); math.Abs(got-291.0/500) > 0.01 {
		t.Fatalf("with MinSecond(200) the position is %v, want 291/500", got)
	}
}

func TestSplitKeyboardMovesByStepsAndTheEnds(t *testing.T) {
	pos := NewSignal(0.5)
	var changes []float64
	r := splitRig(t, splitWith(pos, func(s *SplitElement) *SplitElement {
		return s.MinFirst(Px(100)).MaxFirst(Px(400)).OnChange(func(v float64) { changes = append(changes, v) })
	}))
	focusNode(r, handleNode(t, r))
	r.press(input.KeyRight, 0)
	if got := pos.Peek(); math.Abs(got-0.55) > 1e-9 {
		t.Fatalf("Right moves to %v, want 0.55", got)
	}
	r.press(input.KeyRight, input.ModShift)
	if got := pos.Peek(); math.Abs(got-0.65) > 1e-9 {
		t.Fatalf("Shift+Right moves to %v, want 0.65", got)
	}
	r.press(input.KeyLeft, 0)
	if got := pos.Peek(); math.Abs(got-0.60) > 1e-9 {
		t.Fatalf("Left moves to %v, want 0.60", got)
	}
	r.press(input.KeyEnd, 0)
	if got := pos.Peek(); math.Abs(got-0.8) > 1e-9 {
		t.Fatalf("End moves to %v, want the MaxFirst limit 400/500 = 0.8", got)
	}
	r.press(input.KeyHome, 0)
	if got := pos.Peek(); math.Abs(got-0.2) > 1e-9 {
		t.Fatalf("Home moves to %v, want the MinFirst limit 100/500 = 0.2", got)
	}
	if len(changes) == 0 || changes[len(changes)-1] != pos.Peek() {
		t.Fatalf("OnChange reports %v, want the last position %v", changes, pos.Peek())
	}
	r.press(input.KeyUp, 0) // not a key of a horizontal split
	if got := pos.Peek(); math.Abs(got-0.2) > 1e-9 {
		t.Fatalf("Up moved a horizontal split to %v", got)
	}
}

func TestSplitVerticalUsesUpAndDown(t *testing.T) {
	pos := NewSignal(0.5)
	r := splitRig(t, func() Element {
		return Split(Div().Key("top"), Div().Key("bottom")).Orientation(SplitVertical).Position(pos)
	})
	focusNode(r, handleNode(t, r))
	r.press(input.KeyDown, 0)
	if got := pos.Peek(); math.Abs(got-0.55) > 1e-9 {
		t.Fatalf("Down in a vertical split moves to %v, want 0.55", got)
	}
	dragDivider(r, handleNode(t, r), 0, -100)
	// The pane heights are the fraction of 400 pixels: the drag of 100 px is a quarter.
	if got := pos.Peek(); math.Abs(got-(0.55-0.25)) > 0.01 {
		t.Fatalf("a drag of -100 px in a vertical split gives %v, want 0.3", got)
	}
	tops := partNodes(r.rt, "Split.Pane")
	if len(tops) != 2 {
		t.Fatalf("want 2 panes, got %d", len(tops))
	}
	if r.rt.nodeRect(tops[0]).Y > r.rt.nodeRect(tops[1]).Y {
		tops[0], tops[1] = tops[1], tops[0]
	}
	if h := r.rt.nodeRect(tops[0]).H; math.Abs(float64(h)-120) > 1 {
		t.Fatalf("the top pane is %v high, want 0.3 of 400 = 120", h)
	}
}

// clicksSeen is the count of the counting pane, as its last render showed it.
var clicksSeen int

// countingPane is a pane that counts its clicks in a Local of its own.
func countingPane() Element {
	return Component(func(ctx *Context) Element {
		n := Local(ctx, "clicks", 0)
		clicksSeen = n.Get()
		return Div().Key("left").Height(Px(40)).Focusable().OnClick(func() { n.Set(n.Peek() + 1) })
	})
}

func TestSplitEnterCollapsesAndRestoresTheSize(t *testing.T) {
	pos := NewSignal(0.4)
	collapsed := NewSignal(false)
	r := splitRig(t, func() Element {
		return Split(countingPane(), Div().Key("right")).Position(pos).Collapsible(SplitFirst).Collapsed(collapsed)
	})
	r.clickKey(t, "left")
	focusNode(r, handleNode(t, r))
	r.press(input.KeyEnter, 0)
	if !collapsed.Peek() {
		t.Fatal("Enter does not collapse the first pane")
	}
	if pos.Peek() != 0.4 {
		t.Fatalf("collapsing changed the position to %v; it must keep the restore size", pos.Peek())
	}
	r.press(input.KeyRight, 0)
	if pos.Peek() != 0.4 {
		t.Fatalf("a key moved a collapsed split to %v", pos.Peek())
	}
	r.press(input.KeyEnter, 0)
	if collapsed.Peek() {
		t.Fatal("Enter does not restore the collapsed pane")
	}
	if w := paneWidth(t, r, 0); math.Abs(float64(w)-200) > 1 {
		t.Fatalf("the restored first pane is %v wide, want 200", w)
	}
	// The pane's own state survives the collapse: the click count, kept in a Local of the
	// pane, shows on the next click after the restore.
	r.clickKey(t, "left")
	if clicksSeen != 2 {
		t.Fatalf("the pane's count is %d after a click that followed the restore, want 2", clicksSeen)
	}
}

func TestSplitDoubleClickCollapsesTheSecondPane(t *testing.T) {
	pos := NewSignal(0.5)
	collapsed := NewSignal(false)
	r := splitRig(t, splitWith(pos, func(s *SplitElement) *SplitElement {
		return s.Collapsible(SplitSecond).Collapsed(collapsed)
	}))
	rc := r.rt.nodeRect(handleNode(t, r))
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 2, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
	if !collapsed.Peek() {
		t.Fatal("a double-click does not collapse the second pane")
	}
	// The first pane is the one with the "left" key; the hidden second pane has no width.
	if w := r.rt.nodeRect(r.nodeByKey(t, "left")).W; math.Abs(float64(w)-(500-splitHit)) > 1 {
		t.Fatalf("with the second pane collapsed, the first is %v wide, want %v", w, 500-splitHit)
	}
}

func TestSplitPixelPositionKeepsTheFirstPaneOnResize(t *testing.T) {
	pos := NewSignal(200.0)
	r := splitRig(t, func() Element { return Split(Div(), Div()).PixelPosition(pos) })
	if w := paneWidth(t, r, 0); math.Abs(float64(w)-200) > 0.5 {
		t.Fatalf("first pane %v wide, want 200", w)
	}
	r.rt.SetViewport(800, 400, 1)
	r.frame()
	if w := paneWidth(t, r, 0); math.Abs(float64(w)-200) > 0.5 {
		t.Fatalf("after the window grew, the first pane is %v wide, want 200", w)
	}
}

func TestSplitFractionKeepsTheProportionOnResize(t *testing.T) {
	pos := NewSignal(0.25)
	r := splitRig(t, func() Element { return Split(Div(), Div()).Position(pos) })
	r.rt.SetViewport(800, 400, 1)
	r.frame()
	if w := paneWidth(t, r, 0); math.Abs(float64(w)-200) > 0.5 {
		t.Fatalf("after the window grew, the first pane is %v wide, want a quarter of 800", w)
	}
}

func TestSplitNestedSplitsHaveTheirOwnDividers(t *testing.T) {
	outer := NewSignal(0.5)
	inner := NewSignal(0.5)
	r := splitRig(t, func() Element {
		return Split(Div().Key("side"), Split(Div().Key("a"), Div().Key("b")).Position(inner).Label("Resize inner")).
			Position(outer).Label("Resize outer")
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

func TestSplitDividerIsAccessible(t *testing.T) {
	pos := NewSignal(0.5)
	r := splitRig(t, splitWith(pos, func(s *SplitElement) *SplitElement {
		return s.Label("Resize sidebar").FirstLabel("Folders").MinFirst(Px(100))
	}))
	// The split learns its length one frame after its first render; the window
	// settles before the tree is read.
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
	if sep.Range == nil || math.Abs(sep.Range.Value-0.5) > 1e-9 || sep.Range.Min != 100.0/500 || math.Abs(sep.Range.Max-491.0/500) > 1e-9 {
		t.Fatalf("divider range %+v, want value 0.5, min 0.2, max 491/500", sep.Range)
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
	handleNode(t, r).div.acc.setRange(0.7)
	r.frame()
	if pos.Peek() != 0.7 {
		t.Fatalf("setting the range value moved the split to %v", pos.Peek())
	}
	if !containsProperty(props, access.PropertyRangeValue) {
		t.Fatalf("no RangeValue change was published; got %v", props)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func containsProperty(ps []access.Property, p access.Property) bool {
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}

func TestSplitPaneGroupsAreNamed(t *testing.T) {
	r := splitRig(t, func() Element {
		return Split(Div(), Div()).FirstLabel("Folders").SecondLabel("Editor")
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
	pos := NewSignal(0.5)
	st := SplitStyle{Divider: Style{Background: markBG}, Pane: Style{Background: markFG}}
	r := splitRig(t, splitWith(pos, func(s *SplitElement) *SplitElement { return s.Styles(st) }))
	wantBackground(t, partNode(t, r.rt, "Split.Divider", ""), "Split.Divider", markBG)
	wantBackground(t, panesByX(t, r)[0], "Split.Pane", markFG)
}

func TestSplitSizingMethodsSizeTheBox(t *testing.T) {
	pos := NewSignal(0.5)
	// The window's root is sized by the window, so the split sits in a column.
	r := splitRig(t, func() Element {
		return Div().Column().Child(splitWith(pos, func(s *SplitElement) *SplitElement { return s.Width(Px(300)) })())
	})
	if w := r.rt.nodeRect(panesByX(t, r)[0].parent).W; math.Abs(float64(w)-300) > 0.5 {
		t.Fatalf("Width(Px(300)) gives a split %v wide", w)
	}
}

func TestSplitMinimumLimitsMustBePixels(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MinFirst with a percentage did not panic")
		}
	}()
	Split(Div(), Div()).MinFirst(Percent(20))
}

// TestSplitThemeAndClear checks the layers of the look: the theme's Split is
// under the instance's Styles, and a Clear in a part removes the default fill.
func TestSplitThemeAndClear(t *testing.T) {
	th := *DarkTheme()
	th.Split.Divider = Style{Background: markBG}
	pos := NewSignal(0.5)
	r := newThemedRig(&th, func(ctx *Context) Element { return partSplit(pos, SplitStyle{}) })
	wantBackground(t, partNode(t, r.rt, "Split.Divider", ""), "Split.Divider", markBG)

	r = newThemedRig(&th, func(ctx *Context) Element {
		return partSplit(pos, SplitStyle{Divider: Style{Background: markFG}})
	})
	wantBackground(t, partNode(t, r.rt, "Split.Divider", ""), "Split.Divider", markFG)

	r = newThemedRig(DarkTheme(), func(ctx *Context) Element {
		return partSplit(pos, SplitStyle{Divider: Style{Clear: ClearBackground}})
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
	pos := NewSignal(0.5)
	r := newThemedRig(&th, func(ctx *Context) Element { return partSplit(pos, SplitStyle{}) })
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
	focusNode(r, handleNode(t, r))
	if n := partNode(t, r.rt, "Split.Divider", ""); len(n.style.Shadows) == 0 {
		t.Fatal("under high contrast the focused divider has no focus ring")
	}
}

// TestSplitDividerShowsTheResizeCursor checks that the divider shows the resize
// cursor of its orientation: east-west for a split side by side, north-south for a
// stacked one.
func TestSplitDividerShowsTheResizeCursor(t *testing.T) {
	pos := NewSignal(0.5)
	r := splitRig(t, splitWith(pos, nil))
	if c := handleNode(t, r).style.Cursor; c != input.CursorResizeEW {
		t.Fatalf("a horizontal split shows cursor %v, want the east-west resize cursor", c)
	}
	r = splitRig(t, func() Element {
		return Split(Div(), Div()).Orientation(SplitVertical).Position(pos)
	})
	if c := handleNode(t, r).style.Cursor; c != input.CursorResizeNS {
		t.Fatalf("a vertical split shows cursor %v, want the north-south resize cursor", c)
	}
}
