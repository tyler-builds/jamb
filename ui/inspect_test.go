package ui

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
)

// inspRuntime makes a headless runtime for root, laid out once, with the
// inspector's target side.
func inspRuntime(t *testing.T, theme *Theme, root func(ctx *Context) Element) (*Runtime, inspect.Target) {
	t.Helper()
	rt := NewRuntime(root, theme, nil)
	rt.SetViewport(800, 600, 1)
	rt.Frame(time.Now())
	tg, ok := uihost.Of(rt).(inspect.Target)
	if !ok {
		t.Fatal("the runtime's host side does not implement inspect.Target")
	}
	tg.Attach(func() {})
	rt.Frame(time.Now()) // renders the components again, with their layers recorded
	return rt, tg
}

// inspFind returns the first element of the subtree at id, in tree order, for
// which match is true.
func inspFind(tg inspect.Target, id uint64, match func(inspect.Element) bool) (uint64, bool) {
	e, ok := tg.Element(id)
	if !ok {
		return 0, false
	}
	if match(e) {
		return id, true
	}
	for _, c := range e.Children {
		if f, ok := inspFind(tg, c, match); ok {
			return f, true
		}
	}
	return 0, false
}

// inspByKey returns the element with the key, or 0.
func inspByKey(t *testing.T, tg inspect.Target, key string) uint64 {
	t.Helper()
	id, ok := inspFind(tg, tg.Root(), func(e inspect.Element) bool { return e.Key == key })
	if !ok {
		t.Fatalf("no element with key %q", key)
	}
	return id
}

// inspField returns the field named name of the element, or false when it is not set.
func inspField(tg inspect.Target, id uint64, name string) (inspect.Field, bool) {
	for _, f := range tg.Fields(id) {
		if f.Name == name {
			return f, true
		}
	}
	return inspect.Field{}, false
}

// inspSample is a window with the elements the tests inspect: a card that
// changes on hover, a button with a width and a click count, a check box whose
// box is styled by the theme and by its instance, and a labelled text.
func inspSample(clicks *int, checked *Signal[bool], tick *Signal[int]) func(ctx *Context) Element {
	return func(ctx *Context) Element {
		_ = tick.Get() // the window reads tick, so setting it rebuilds the window
		return Div().Key("root").Column().Gap(4).Children(
			Div().Key("greeting").Child(Text("Hello")),
			Div().Key("card").Padding(8).Background(Hex("#112233")).Hover(Style{Background: Hex("#445566")}).
				Width(Px(120)).Height(Px(40)),
			Button("Save").Key("save").Width(Px(90)).OnClick(func() { *clicks++ }),
			Checkbox("Remember", checked).Styles(CheckboxStyle{Box: Style{Radius: 9}}),
		)
	}
}

func TestInspectTreeReportsKindsKeysPartsAndBoxes(t *testing.T) {
	th := DarkTheme()
	clicks, checked, tick := 0, NewSignal(false), NewSignal(0)
	rt, tg := inspRuntime(t, th, inspSample(&clicks, checked, tick))

	root, ok := tg.Element(tg.Root())
	if !ok || root.Key != "root" || root.Kind != "div" || root.Parent != 0 {
		t.Fatalf("root = %+v, ok %v; want the div keyed root with no parent", root, ok)
	}
	card := inspByKey(t, tg, "card")
	ce, _ := tg.Element(card)
	r, ok := rt.Find("card")
	if !ok || ce.Box.X != r.X || ce.Box.Y != r.Y || ce.Box.W != r.W || ce.Box.H != r.H {
		t.Fatalf("card box = %+v, want the laid-out rectangle %+v", ce.Box, r)
	}
	if ce.Padding != (inspect.Edges{Top: 8, Right: 8, Bottom: 8, Left: 8}) {
		t.Fatalf("card padding = %+v, want 8 on every side", ce.Padding)
	}
	if ce.Parent != root.ID {
		t.Fatalf("card parent = %d, want the root %d", ce.Parent, root.ID)
	}
	box, ok := inspFind(tg, tg.Root(), func(e inspect.Element) bool { return e.Part == "Checkbox.Box" })
	if !ok {
		t.Fatal("no element with the part Checkbox.Box")
	}
	if be, _ := tg.Element(box); be.Part != "Checkbox.Box" {
		t.Fatalf("part of the check box = %q", be.Part)
	}
	greeting := inspByKey(t, tg, "greeting")
	if _, ok := inspFind(tg, greeting, func(e inspect.Element) bool { return e.Kind == "text" && e.Text == "Hello" }); !ok {
		t.Fatal("the text Hello is not under the greeting element")
	}
}

func TestInspectStyleSourcesDistinguishStateSizingThemeAndElement(t *testing.T) {
	th := DarkTheme()
	th.Checkbox.Box = Style{Radius: 5} // the theme's part
	clicks, checked, tick := 0, NewSignal(false), NewSignal(0)
	rt, tg := inspRuntime(t, th, inspSample(&clicks, checked, tick))

	card := inspByKey(t, tg, "card")
	if f, ok := inspField(tg, card, "Padding"); !ok || f.Source != inspect.SourceElement || f.Value != "8" {
		t.Fatalf("Padding = %+v, ok %v; want 8 from the element method", f, ok)
	}
	if f, ok := inspField(tg, card, "Background"); !ok || f.Source != inspect.SourceElement || f.Value != "#112233ff" {
		t.Fatalf("Background = %+v, ok %v; want #112233ff from the element method", f, ok)
	}

	// The pointer over the card: its hover layer sets the background.
	r, _ := rt.Find("card")
	rt.PointerMove(r.X+r.W/2, r.Y+r.H/2, 0)
	rt.Frame(time.Now())
	if f, ok := inspField(tg, card, "Background"); !ok || f.Source != inspect.SourceState+"hover" || f.Value != "#445566ff" {
		t.Fatalf("hovered Background = %+v, ok %v; want #445566ff from the hover state", f, ok)
	}

	// The button's width comes from its sizing method.
	save := inspByKey(t, tg, "save")
	if f, ok := inspField(tg, save, "Width"); !ok || f.Source != inspect.SourceSizing || f.Value != "90px" {
		t.Fatalf("Width of the button = %+v, ok %v; want 90px from the sizing method", f, ok)
	}

	// The check box's box: the default sets the radius, the theme sets it to 5,
	// and the instance overrides both with 9.
	boxID, _ := inspFind(tg, tg.Root(), func(e inspect.Element) bool { return e.Part == "Checkbox.Box" })
	if f, ok := inspField(tg, boxID, "Radius"); !ok || f.Source != inspect.SourceInstance || f.Value != "9" {
		t.Fatalf("Radius of the check box = %+v, ok %v; want 9 from the instance Styles", f, ok)
	}
	th2 := DarkTheme()
	th2.Checkbox.Box = Style{Radius: 5}
	_, tg2 := inspRuntime(t, th2, func(ctx *Context) Element {
		return Div().Key("root").Child(Checkbox("Plain", NewSignal(false)))
	})
	themed, _ := inspFind(tg2, tg2.Root(), func(e inspect.Element) bool { return e.Part == "Checkbox.Box" })
	if f, ok := inspField(tg2, themed, "Radius"); !ok || f.Source != inspect.SourceTheme || f.Value != "5" {
		t.Fatalf("Radius of the themed check box = %+v, ok %v; want 5 from the theme part", f, ok)
	}
}

func TestInspectLiveEditAppliesAndIsDroppedOnRebuild(t *testing.T) {
	clicks, checked, tick := 0, NewSignal(false), NewSignal(0)
	rt, tg := inspRuntime(t, DarkTheme(), inspSample(&clicks, checked, tick))
	card := inspByKey(t, tg, "card")
	before, _ := rt.Find("card")

	if err := tg.Edit(card, "Background", "#ff0000"); err != nil {
		t.Fatal(err)
	}
	if err := tg.Edit(card, "Padding", "0"); err != nil {
		t.Fatal(err)
	}
	if err := tg.Edit(card, "Width", "200"); err != nil {
		t.Fatal(err)
	}
	rt.Frame(time.Now())
	if !tg.Edited(card) {
		t.Fatal("the card has no live edit after Edit")
	}
	if f, ok := inspField(tg, card, "Background"); !ok || f.Source != inspect.SourceEdit || f.Value != "#ff0000ff" {
		t.Fatalf("edited Background = %+v, ok %v; want #ff0000ff from the live edit", f, ok)
	}
	if _, ok := inspField(tg, card, "Padding"); ok {
		t.Fatal("a zero Padding edit did not clear the padding")
	}
	if r, _ := rt.Find("card"); r.W != 200 || r.W == before.W {
		t.Fatalf("card width after the edit = %v, want 200", r.W)
	}

	// The literal lists the fields in the order Style declares them (see Style).
	want := "ui.Style{\n\tBackground: ui.Hex(\"#ff0000ff\"),\n\tWidth: ui.Px(200),\n\tClear: ui.ClearPadding,\n}"
	if got := tg.GoLiteral(card); got != want {
		t.Fatalf("GoLiteral =\n%s\nwant\n%s", got, want)
	}

	// A rebuild of the card's component builds the card again, and the edit is gone.
	tick.Set(1)
	rt.Frame(time.Now())
	card = inspByKey(t, tg, "card")
	if tg.Edited(card) {
		t.Fatal("the edit survived the rebuild of its element")
	}
	if f, _ := inspField(tg, card, "Background"); f.Value != "#112233ff" {
		t.Fatalf("Background after the rebuild = %q, want the style's own #112233ff", f.Value)
	}
	if r, _ := rt.Find("card"); r.W != before.W {
		t.Fatalf("card width after the rebuild = %v, want %v", r.W, before.W)
	}
}

func TestInspectEditRefusesValuesThatCannotTakeEffect(t *testing.T) {
	clicks, checked, tick := 0, NewSignal(false), NewSignal(0)
	_, tg := inspRuntime(t, DarkTheme(), inspSample(&clicks, checked, tick))
	card := inspByKey(t, tg, "card")
	for _, c := range []struct{ field, value string }{
		{"Width", "auto"},      // automatic is the zero length: it would leave the width as it is
		{"Opacity", "0"},       // no Clear bit covers it
		{"Background", "nope"}, // not a color
		{"Padding", "1 2 3"},   // three numbers
		{"Cursor", "pointer"},  // not an editable field
		{"NoSuchField", "1"},   // not a field
		{"FontSize", "0"},      // zero is refused where no Clear bit covers the field
	} {
		if err := tg.Edit(card, c.field, c.value); err == nil {
			t.Errorf("Edit(%s, %q) succeeded, want an error", c.field, c.value)
		}
	}
	if tg.Edited(card) {
		t.Fatal("a refused edit was kept")
	}
	if err := tg.Edit(card, "Radius", "0"); err != nil {
		t.Fatalf("a zero Radius is covered by ClearRadius: %v", err)
	}
}

func TestInspectDetachDropsEditsAndPicking(t *testing.T) {
	clicks, checked, tick := 0, NewSignal(false), NewSignal(0)
	rt, tg := inspRuntime(t, DarkTheme(), inspSample(&clicks, checked, tick))
	card := inspByKey(t, tg, "card")
	tg.Attach(func() {})
	if err := tg.Edit(card, "Background", "#ff0000"); err != nil {
		t.Fatal(err)
	}
	tg.Pick(true)
	tg.Detach()
	rt.Frame(time.Now())
	if f, _ := inspField(tg, card, "Background"); f.Value != "#112233ff" {
		t.Fatalf("Background after Detach = %q, want the style's own #112233ff", f.Value)
	}
	if tg.Picking() || tg.Edited(card) {
		t.Fatal("Detach left pick mode or a live edit on")
	}
}

// TestInspectPickingDoesNotChangeLayoutOrHitTesting checks the overlay and pick
// mode: the outlines are painted, and the layout and the element under a point
// are the same with and without them. A press in pick mode selects the element
// and does not click it.
func TestInspectPickingDoesNotChangeLayoutOrHitTesting(t *testing.T) {
	clicks, checked, tick := 0, NewSignal(false), NewSignal(0)
	rt, tg := inspRuntime(t, DarkTheme(), inspSample(&clicks, checked, tick))
	var dumpBefore strings.Builder
	rt.Dump(&dumpBefore, 0)
	save, _ := rt.Find("save")
	px, py := save.X+save.W/2, save.Y+save.H/2
	var idsBefore []uint64
	for _, n := range rt.hitTest(px, py) {
		idsBefore = append(idsBefore, n.id)
	}
	quadsBefore := len(rt.frame(time.Now()).Quads)

	changes := 0
	tg.Attach(func() { changes++ })
	tg.Select(inspByKey(t, tg, "card"))
	tg.Pick(true)
	if !tg.Picking() {
		t.Fatal("Pick(true) did not turn pick mode on")
	}
	rt.PointerMove(px, py, 0)
	rt.Frame(time.Now())
	if hover := rt.insp.hover; hover == 0 {
		t.Fatal("pick mode did not find the element under the pointer")
	}
	if n := len(rt.frame(time.Now()).Quads); n <= quadsBefore {
		t.Fatalf("the outlines added no quads: %d, before %d", n, quadsBefore)
	}

	rt.PointerDown(px, py, input.ButtonLeft, 1, 0)
	rt.PointerUp(px, py, input.ButtonLeft, 0)
	rt.Frame(time.Now())
	if clicks != 0 {
		t.Fatalf("the press in pick mode clicked the button %d times, want 0", clicks)
	}
	if tg.Picking() {
		t.Fatal("the press did not end pick mode")
	}
	// The press selects the deepest element under the point, the label of the
	// button, so the button is an ancestor of it.
	sel := tg.Selected()
	if sel == 0 {
		t.Fatal("the press selected nothing")
	}
	under := false
	for id := sel; id != 0; {
		e, _ := tg.Element(id)
		if e.Key == "save" {
			under = true
		}
		id = e.Parent
	}
	if !under {
		t.Fatalf("the press selected element %d, which is not in the button keyed save", sel)
	}
	if changes == 0 {
		t.Fatal("the inspector was not told about the selection")
	}

	var dumpAfter strings.Builder
	rt.Dump(&dumpAfter, 0)
	if dumpAfter.String() != dumpBefore.String() {
		t.Fatalf("the layout changed with the outlines:\nbefore:\n%s\nafter:\n%s", dumpBefore.String(), dumpAfter.String())
	}
	var idsAfter []uint64
	for _, n := range rt.hitTest(px, py) {
		idsAfter = append(idsAfter, n.id)
	}
	if !slices.Equal(idsAfter, idsBefore) {
		t.Fatalf("the elements under the point changed with the outlines: %v, before %v", idsAfter, idsBefore)
	}
}

func TestInspectEscapeEndsPickMode(t *testing.T) {
	clicks, checked, tick := 0, NewSignal(false), NewSignal(0)
	rt, tg := inspRuntime(t, DarkTheme(), inspSample(&clicks, checked, tick))
	changes := 0
	tg.Attach(func() { changes++ })
	tg.Pick(true)
	changes = 0
	rt.KeyDown(input.KeyEscape, 0, false)
	if tg.Picking() {
		t.Fatal("Escape did not end pick mode")
	}
	if changes == 0 {
		t.Fatal("the inspector was not told that Escape ended pick mode")
	}
	if tg.Selected() != 0 {
		t.Fatal("Escape selected an element")
	}
}

func TestInspectAccessReportsTheWarningsOfAnElement(t *testing.T) {
	checked := NewSignal(false)
	_, tg := inspRuntime(t, DarkTheme(), func(ctx *Context) Element {
		return Div().Key("root").Children(
			Button("Save").Key("save"),
			Button("").Key("icon-only"),
			Checkbox("Remember", checked),
		)
	})
	save := inspByKey(t, tg, "save")
	if got := tg.AccessNames()[save]; got != `button "Save"` {
		t.Fatalf("AccessNames of the button = %q, want button \"Save\"", got)
	}
	if a := tg.Access(save); !a.Exposed || a.Role != "button" || a.Name != "Save" || len(a.Warnings) != 0 {
		t.Fatalf("access of the labelled button = %+v; want a button named Save without warnings", a)
	}
	bare := inspByKey(t, tg, "icon-only")
	a := tg.Access(bare)
	if len(a.Warnings) != 1 || !strings.Contains(a.Warnings[0], "has no accessible name") {
		t.Fatalf("access of the unlabelled button = %+v; want the accessible name warning", a)
	}
}

// TestInspectSelectRepaintsTheWindow checks that a selection made in the
// inspector asks the inspected window for a frame that paints the outline, so
// the outline shows before the window gets input of its own.
func TestInspectSelectRepaintsTheWindow(t *testing.T) {
	clicks, checked, tick := 0, NewSignal(false), NewSignal(0)
	rt, tg := inspRuntime(t, DarkTheme(), inspSample(&clicks, checked, tick))
	rt.Frame(time.Now())
	if rt.NeedsFrame() || rt.framePending.Load() {
		t.Fatal("the window is not at rest before the selection")
	}
	tg.Select(inspByKey(t, tg, "card"))
	if !rt.needPaint || !rt.framePending.Load() {
		t.Fatalf("Select did not ask for a paint: needPaint %v, framePending %v", rt.needPaint, rt.framePending.Load())
	}
}

// TestInspectOutlinesAnElementWithNoHeight checks that an element with no height
// still gets a visible mark: a filled line 2 pixels thick across its width.
func TestInspectOutlinesAnElementWithNoHeight(t *testing.T) {
	rt, tg := inspRuntime(t, DarkTheme(), func(ctx *Context) Element {
		return Div().Key("root").Column().Children(
			Div().Key("flat").Width(Px(300)).Height(Px(0)),
			Text("below"),
		)
	})
	flat := inspByKey(t, tg, "flat")
	if e, _ := tg.Element(flat); e.Box.H != 0 || e.Box.W != 300 {
		t.Fatalf("the flat element is %v x %v, want 300 x 0", e.Box.W, e.Box.H)
	}
	tg.Select(flat)
	found := false
	for _, q := range rt.frame(time.Now()).Quads {
		if q.Bounds.W == 300 && q.Bounds.H == 2 && q.Background.A > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("no 300 x 2 marker was painted for the element with no height")
	}
}

// TestInspectHoverOutlinesAnElementUntilCleared checks the light outline that
// the inspector's tree asks for: Hover paints it over the element at once, and
// Hover(0) takes it away.
func TestInspectHoverOutlinesAnElementUntilCleared(t *testing.T) {
	th := DarkTheme()
	clicks, checked, tick := 0, NewSignal(false), NewSignal(0)
	rt, tg := inspRuntime(t, th, inspSample(&clicks, checked, tick))
	rt.Frame(time.Now())
	card := inspByKey(t, tg, "card")
	r, _ := rt.Find("card")
	fill := th.Accent.Alpha(0.12).scene()
	hovered := func() bool {
		for _, q := range rt.frame(time.Now()).Quads {
			if q.Background == fill && q.Bounds == rt.px(r) {
				return true
			}
		}
		return false
	}

	tg.Hover(card)
	if !rt.needPaint || !rt.framePending.Load() {
		t.Fatal("Hover did not ask the window for a paint")
	}
	if !hovered() {
		t.Fatal("the hovered card has no light accent fill")
	}
	tg.Hover(0)
	if !rt.needPaint {
		t.Fatal("clearing the hover did not ask for a paint")
	}
	if hovered() {
		t.Fatal("the light fill stayed after Hover(0)")
	}
}

// TestInspectSelectionShowsTheBoxModel checks the bands of the selected element:
// its padding as a tinted band of the padding's width, and its content box filled.
func TestInspectSelectionShowsTheBoxModel(t *testing.T) {
	clicks, checked, tick := 0, NewSignal(false), NewSignal(0)
	rt, tg := inspRuntime(t, DarkTheme(), inspSample(&clicks, checked, tick))
	tg.Select(inspByKey(t, tg, "card"))
	r, _ := rt.Find("card")
	var band, content bool
	for _, q := range rt.frame(time.Now()).Quads {
		if q.BorderColor == inspectPaddingTint.scene() && q.Bounds == rt.px(r) && q.BorderSides.Top == 8 {
			band = true
		}
		if q.Background == inspectContentTint.scene() && q.Bounds == rt.px(outset(r, Edges{-8, -8, -8, -8})) {
			content = true
		}
	}
	if !band || !content {
		t.Fatalf("the selected card's padding band %v, content box %v; want both", band, content)
	}
}

func TestInspectShortenKeepsWholeCharacters(t *testing.T) {
	if got := shorten("héllo wörld", 4); got != "héll…" {
		t.Errorf("shorten = %q, want héll…", got)
	}
	if got := shorten("abc", 3); got != "abc" {
		t.Errorf("shorten of a short text = %q, want it unchanged", got)
	}
}

func TestInspectParsesEditValues(t *testing.T) {
	if l, err := parseLength("50%"); err != nil || l != Percent(50) {
		t.Errorf("parseLength(50%%) = %v, %v", l, err)
	}
	if l, err := parseLength("2fr"); err != nil || l != Fr(2) {
		t.Errorf("parseLength(2fr) = %v, %v", l, err)
	}
	if l, err := parseLength("12px"); err != nil || l != Px(12) {
		t.Errorf("parseLength(12px) = %v, %v", l, err)
	}
	if s, err := parseSides("4 8 12 16"); err != nil || s != [4]float32{4, 8, 12, 16} {
		t.Errorf("parseSides = %v, %v", s, err)
	}
	if s, err := parseSides("6"); err != nil || s != [4]float32{6, 6, 6, 6} {
		t.Errorf("parseSides(6) = %v, %v", s, err)
	}
	if got := formatStyleValue(reflect.ValueOf(Edges{Top: 1, Right: 2, Bottom: 3, Left: 4})); got != "1 2 3 4" {
		t.Errorf("formatStyleValue(edges) = %q", got)
	}
}
