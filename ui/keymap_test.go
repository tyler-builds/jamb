package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

type keyRig struct {
	rt  *Runtime
	now time.Time
}

func newKeyRig(root func(*Context) Element) *keyRig {
	r := &keyRig{now: time.Unix(1_000_000, 0)}
	r.rt = NewRuntime(root, DarkTheme(), nil)
	r.rt.clock = func() time.Time { return r.now }
	r.rt.SetViewport(500, 400, 1)
	r.frame()
	return r
}

func (r *keyRig) frame() { r.rt.Frame(r.now) }

func (r *keyRig) key(k input.Key, mods input.Modifiers) {
	r.rt.KeyDown(k, mods, false)
	r.rt.KeyUp(k, mods)
	r.frame()
}

func (r *keyRig) click(x, y float32) {
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
}

// focusedEdit returns the editor state of the focused text field.
func focusedEdit(t *testing.T, rt *Runtime) *editState {
	t.Helper()
	n := rt.byID[rt.focusID]
	if n == nil || n.st == nil || n.st.edit == nil {
		t.Fatal("focus is not on a text field")
	}
	return n.st.edit
}

// panicMsg returns the panic message of f, or "" if f does not panic.
func panicMsg(f func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	f()
	return ""
}

func TestBindParsesKeys(t *testing.T) {
	a := NewAction("test.parse", "Parse")
	tests := []struct {
		keys string
		want []keystroke
	}{
		{"ctrl+shift+p", []keystroke{{input.KeyP, input.ModCtrl | input.ModShift}}},
		{"Alt+Up", []keystroke{{input.KeyUp, input.ModAlt}}},
		{"ctrl+k ctrl+t", []keystroke{{input.KeyK, input.ModCtrl}, {input.KeyT, input.ModCtrl}}},
		{"ctrl+=", []keystroke{{input.KeyEqual, input.ModCtrl}}},
		{"ctrl+-", []keystroke{{input.KeyMinus, input.ModCtrl}}},
		{"esc", []keystroke{{input.KeyEscape, 0}}},
		{"f12", []keystroke{{input.KeyF12, 0}}},
		{"super+shift+alt+ctrl+0", []keystroke{{input.Key0, input.ModSuper | input.ModShift | input.ModAlt | input.ModCtrl}}},
		{"pageup pagedown", []keystroke{{input.KeyPageUp, 0}, {input.KeyPageDown, 0}}},
	}
	for _, tc := range tests {
		if got := Bind(tc.keys, a).strokes; !slices.Equal(got, tc.want) {
			t.Errorf("Bind(%q) = %v, want %v", tc.keys, got, tc.want)
		}
	}
	if got := BindIn("Palette", "up", a); got.pred == nil || got.strokes[0].key != input.KeyUp {
		t.Errorf("BindIn kept no expression or the wrong key %v", got.strokes[0].key)
	}
}

func TestBindPanicsOnBadKeys(t *testing.T) {
	a := NewAction("test.bad", "Bad")
	tests := []struct{ keys, want string }{
		{"ctrl+shfit+p", "shfit"},
		{"foo", "foo"},
		{"ctrl+", "ctrl+"},
		{"", "no keys"},
	}
	for _, tc := range tests {
		msg := panicMsg(func() { Bind(tc.keys, a) })
		if msg == "" || !strings.Contains(msg, tc.want) {
			t.Errorf("Bind(%q) panic = %q, want a message containing %q", tc.keys, msg, tc.want)
		}
	}
}

// ParseBinding returns the same problems Bind and BindIn panic with, without
// the call prefix, and a good binding for good input.
func TestParseBinding(t *testing.T) {
	a := NewAction("test.parse", "Parse")
	if b, err := ParseBinding("", "ctrl+k ctrl+t", a); err != nil || b.action != a || b.pred != nil {
		t.Errorf("ParseBinding global: %v, %v", b, err)
	}
	if b, err := ParseBinding("Editor", "enter", a); err != nil || b.expr != "Editor" || b.pred == nil {
		t.Errorf("ParseBinding in a context: %v, %v", b, err)
	}
	for _, tc := range []struct{ expr, keys, want string }{
		{"", "ctrl+shfit+p", `unknown modifier "shfit"`},
		{"", "", "no keys"},
		{"Editr &&", "ctrl+b", "expression"},
	} {
		_, err := ParseBinding(tc.expr, tc.keys, a)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ParseBinding(%q, %q) error %v, want one containing %q", tc.expr, tc.keys, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), "ui.") {
			t.Errorf("ParseBinding error %q names the call", err)
		}
	}
	if _, err := ParseBinding("", "ctrl+n", nil); err == nil {
		t.Error("ParseBinding with a nil action: no error")
	}
}

func TestGlobalBindingFiresWithNothingOrAFieldFocused(t *testing.T) {
	newAct := NewAction("test.new", "New")
	v := NewSignal("")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Padding(10).OnAction(newAct, func() { hits++ }).Child(TextInput(v))
	})
	r.rt.AddBindings(Bind("ctrl+n", newAct))
	r.key(input.KeyN, input.ModCtrl)
	if hits != 1 {
		t.Fatalf("with nothing focused: %d hits, want 1", hits)
	}
	r.click(100, 27)
	if r.rt.focusID == 0 {
		t.Fatal("click did not focus the field")
	}
	r.key(input.KeyN, input.ModCtrl)
	if hits != 2 {
		t.Fatalf("with a field focused: %d hits, want 2", hits)
	}
}

func TestGlobalBindingDoesNotTakeFieldKeys(t *testing.T) {
	selAll := NewAction("test.selectall", "Select all")
	v := NewSignal("hello")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Padding(10).OnAction(selAll, func() { hits++ }).Child(TextInput(v))
	})
	r.rt.AddBindings(Bind("ctrl+a", selAll))
	r.click(100, 27)
	r.key(input.KeyA, input.ModCtrl)
	if a, b := focusedEdit(t, r.rt).selection(); a != 0 || b != 5 {
		t.Errorf("Ctrl+A in a field selected [%d,%d), want [0,5)", a, b)
	}
	if hits != 0 {
		t.Errorf("the global Ctrl+A binding ran while a field was focused")
	}
	r.rt.setFocus(0)
	r.key(input.KeyA, input.ModCtrl)
	if hits != 1 {
		t.Errorf("with nothing focused: %d hits, want 1", hits)
	}
}

func TestContextBindingTakesKeysFromItsField(t *testing.T) {
	up := NewAction("test.up", "Up")
	palette, outside := NewFocusHandle(), NewFocusHandle()
	p, v := NewSignal("hello"), NewSignal("world")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Div().KeyContext("Palette").OnAction(up, func() { hits++ }).
				Child(TextInput(p).FocusHandle(palette))).
			Child(TextInput(v).FocusHandle(outside))
	})
	// A plain "Palette" binding would lose to the field's own Up (Editor is
	// deeper); see TestPlainContextLosesToFieldKeys.
	r.rt.AddBindings(BindIn("Palette > Editor", "up", up))

	palette.Focus()
	r.frame()
	r.key(input.KeyEnd, 0)
	if es := focusedEdit(t, r.rt); es.caret != 5 {
		t.Fatalf("caret %d after End, want 5", es.caret)
	}
	r.key(input.KeyUp, 0)
	if hits != 1 {
		t.Errorf("Up in the palette: %d hits, want 1", hits)
	}
	if es := focusedEdit(t, r.rt); es.caret != 5 {
		t.Errorf("Up in the palette moved the caret to %d", es.caret)
	}

	outside.Focus()
	r.frame()
	r.key(input.KeyEnd, 0)
	r.key(input.KeyUp, 0)
	if hits != 1 {
		t.Errorf("Up in a field outside the palette ran the palette binding (%d hits)", hits)
	}
	if es := focusedEdit(t, r.rt); es.caret != 0 {
		t.Errorf("Up in the outside field: caret %d, want 0", es.caret)
	}
}

func TestBindingPrecedence(t *testing.T) {
	outerAct := NewAction("test.outer", "Outer")
	innerAct := NewAction("test.inner", "Inner")
	laterAct := NewAction("test.later", "Later")
	missingAct := NewAction("test.missing", "Missing")
	field := NewFocusHandle()
	v := NewSignal("")
	counts := map[string]int{}
	counter := func(name string) func() { return func() { counts[name]++ } }

	r := newKeyRig(func(ctx *Context) Element {
		return Div().KeyContext("Outer").OnAction(outerAct, counter("outer")).
			Child(Div().KeyContext("Inner").OnAction(innerAct, counter("inner")).
				OnAction(laterAct, counter("later")).
				Child(TextInput(v).FocusHandle(field)))
	})
	// Enter is also bound by the field itself (Editor, one level deeper than
	// Inner), so Inner's binding for it must name the field: "Inner > Editor".
	r.rt.AddBindings(
		BindIn("Outer", "enter", outerAct),
		BindIn("Inner > Editor", "enter", innerAct),
		// Same depth: the later binding wins; Missing has no handler on the path.
		BindIn("Inner", "f1", laterAct),
		BindIn("Inner", "f1", innerAct),
		BindIn("Inner", "f2", innerAct),
		BindIn("Inner", "f2", missingAct),
	)
	field.Focus()
	r.frame()

	r.key(input.KeyEnter, 0)
	if counts["inner"] != 1 || counts["outer"] != 0 {
		t.Errorf("Enter: counts %v, want only the deeper Inner binding", counts)
	}
	r.key(input.KeyF1, 0)
	if counts["inner"] != 2 || counts["later"] != 0 {
		t.Errorf("F1 (same depth, later added wins): counts %v", counts)
	}
	r.key(input.KeyF2, 0)
	if counts["inner"] != 3 {
		t.Errorf("F2 (Missing has no handler, so Inner runs): counts %v", counts)
	}
}

func TestDispatchFindsDeepestHandler(t *testing.T) {
	act := NewAction("test.dispatch", "Dispatch")
	field := NewFocusHandle()
	v := NewSignal("")
	var ran []string
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(act, func() { ran = append(ran, "root") }).
			Child(Div().Key("mid").OnAction(act, func() { ran = append(ran, "mid") }).
				Child(TextInput(v).FocusHandle(field)))
	})
	if !r.rt.Dispatch(act) || !slices.Equal(ran, []string{"root"}) {
		t.Fatalf("with nothing focused: ran %v, want [root]", ran)
	}
	field.Focus()
	r.frame()
	if !r.rt.Dispatch(act) || !slices.Equal(ran, []string{"root", "mid"}) {
		t.Fatalf("with a field focused: ran %v, want the deeper mid handler", ran)
	}
	if r.rt.Dispatch(NewAction("test.unhandled", "Nothing")) {
		t.Error("Dispatch of an unhandled action returned true")
	}
}

func TestChords(t *testing.T) {
	toggle := NewAction("test.toggle", "Toggle")
	field := NewFocusHandle()
	v := NewSignal("hello")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(toggle, func() { hits++ }).
			Child(TextInput(v).FocusHandle(field))
	})
	r.rt.AddBindings(Bind("ctrl+k ctrl+t", toggle), Bind("ctrl+k t", toggle))

	// The second stroke runs the chord, not the first.
	r.key(input.KeyK, input.ModCtrl)
	if hits != 0 || len(r.rt.pending) != 1 {
		t.Fatalf("after ctrl+k: hits %d, pending %d; want 0 and 1", hits, len(r.rt.pending))
	}
	r.key(input.KeyT, input.ModCtrl)
	if hits != 1 {
		t.Fatalf("ctrl+k ctrl+t: %d hits, want 1", hits)
	}

	// An unrelated key ends the chord, and its text is typed normally.
	field.Focus()
	r.frame()
	v.Set("")
	r.key(input.KeyK, input.ModCtrl)
	r.key(input.KeyX, 0)
	r.rt.TextInput("x")
	r.frame()
	if v.Peek() != "x" || len(r.rt.pending) != 0 {
		t.Errorf("after ctrl+k x: value %q, pending %d; want \"x\" and no chord", v.Peek(), len(r.rt.pending))
	}

	// A chord that waits too long is dropped.
	r.rt.setFocus(0)
	r.key(input.KeyK, input.ModCtrl)
	r.now = r.now.Add(1500 * time.Millisecond)
	r.key(input.KeyT, input.ModCtrl)
	if hits != 1 {
		t.Errorf("ctrl+k, a pause, ctrl+t: %d hits, want still 1", hits)
	}

	// With a field focused, the chord's last key does not type a letter.
	field.Focus()
	r.frame()
	v.Set("hello")
	r.key(input.KeyK, input.ModCtrl)
	r.rt.KeyDown(input.KeyT, 0, false)
	r.rt.TextInput("t")
	r.rt.KeyUp(input.KeyT, 0)
	r.frame()
	if hits != 2 || v.Peek() != "hello" {
		t.Errorf("ctrl+k t in a field: %d hits, value %q; want 2 and \"hello\"", hits, v.Peek())
	}
}

func TestAltUpReachesGlobalBinding(t *testing.T) {
	prev := NewAction("test.prev", "Previous")
	field := NewFocusHandle()
	v := NewSignal("hello")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(prev, func() { hits++ }).Child(TextInput(v).FocusHandle(field))
	})
	r.rt.AddBindings(Bind("alt+up", prev))
	field.Focus()
	r.frame()
	r.key(input.KeyEnd, 0)
	r.key(input.KeyUp, input.ModAlt)
	if hits != 1 {
		t.Errorf("Alt+Up in a field: %d hits, want 1", hits)
	}
	if es := focusedEdit(t, r.rt); es.caret != 5 {
		t.Errorf("Alt+Up moved the caret to %d, want it left at the end", es.caret)
	}
}

func TestKeysFor(t *testing.T) {
	tests := []struct {
		keys, want string
	}{
		{"ctrl+shift+p", "Ctrl+Shift+P"},
		{"shift+ctrl+p", "Ctrl+Shift+P"},
		{"alt+up", "Alt+Up"},
		{"ctrl+k ctrl+t", "Ctrl+K Ctrl+T"},
		{"ctrl+=", "Ctrl+="},
		{"esc", "Escape"},
		{"f2", "F2"},
		{"super+pagedown", "Super+PageDown"},
	}
	for _, tc := range tests {
		a := NewAction("test.keysfor", "KeysFor")
		r := newKeyRig(func(ctx *Context) Element { return Div() })
		r.rt.AddBindings(Bind(tc.keys, a))
		if got := r.rt.KeysFor(a); got != tc.want {
			t.Errorf("KeysFor(%q) = %q, want %q", tc.keys, got, tc.want)
		}
	}

	a := NewAction("test.unbound", "Unbound")
	r := newKeyRig(func(ctx *Context) Element { return Div() })
	if got := r.rt.KeysFor(a); got != "" {
		t.Errorf("unbound KeysFor = %q, want \"\"", got)
	}
	b := NewAction("test.latest", "Latest")
	r.rt.AddBindings(Bind("ctrl+n", b), Bind("ctrl+m", b))
	if got := r.rt.KeysFor(b); got != "Ctrl+M" {
		t.Errorf("KeysFor with two bindings = %q, want the latest, Ctrl+M", got)
	}
}

func TestAvailableActions(t *testing.T) {
	a := NewAction("test.a", "A")
	b := NewAction("test.b", "B")
	c := NewAction("test.c", "C")
	field := NewFocusHandle()
	v := NewSignal("")
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(a, func() {}).
			Child(Div().Key("mid").OnAction(b, func() {}).OnAction(a, func() {}).
				Child(TextInput(v).FocusHandle(field))).
			Child(Div().OnAction(c, func() {}))
	})
	if got := r.rt.AvailableActions(); !slices.Equal(got, []*Action{a}) {
		t.Errorf("nothing focused: %v, want [a]", names(got))
	}
	field.Focus()
	r.frame()
	// A focused field adds its own (hidden) editor actions at the front.
	got := r.rt.AvailableActions()
	var visible []*Action
	for _, act := range got {
		if !act.Hidden() {
			visible = append(visible, act)
		} else if !strings.HasPrefix(act.Name(), "editor.") {
			t.Errorf("hidden action %s is not an editor action", act.Name())
		}
	}
	if !slices.Equal(visible, []*Action{b, a}) {
		t.Errorf("field focused: %v, want [b a] (deepest first, no duplicates, no off-path c)", names(visible))
	}
	if len(got) <= 2 || got[len(got)-2] != b {
		t.Errorf("field focused: %v, want the field's editor actions, then b and a", names(got))
	}
}

func names(as []*Action) []string {
	var out []string
	for _, a := range as {
		out = append(out, a.Name())
	}
	return out
}

// The platform reports modifier keys pressed on their own as KeyUnknown.
// Releasing Ctrl and pressing it again between strokes must not end a chord.
func TestChordSurvivesModifierPresses(t *testing.T) {
	toggle := NewAction("test.toggle", "Toggle")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(toggle, func() { hits++ })
	})
	r.rt.AddBindings(Bind("ctrl+k ctrl+t", toggle))
	r.rt.KeyDown(input.KeyUnknown, input.ModCtrl, false) // Ctrl down
	r.key(input.KeyK, input.ModCtrl)
	r.rt.KeyUp(input.KeyUnknown, 0)                      // Ctrl up
	r.rt.KeyDown(input.KeyUnknown, input.ModCtrl, false) // Ctrl down again
	r.key(input.KeyT, input.ModCtrl)
	if hits != 1 {
		t.Errorf("ctrl+k ctrl+t with Ctrl re-pressed in between: %d hits, want 1", hits)
	}
}

// Contextual bindings beat global ones, and a deeper context beats a
// shallower one, whatever the order they were added in.
func TestBindingDepthOrder(t *testing.T) {
	global := NewAction("test.depth.global", "Global")
	outer := NewAction("test.depth.outer", "Outer")
	inner := NewAction("test.depth.inner", "Inner")
	missing := NewAction("test.depth.missing", "Missing")
	field := NewFocusHandle()
	v := NewSignal("")
	var ran []string
	rec := func(name string) func() { return func() { ran = append(ran, name) } }

	r := newKeyRig(func(ctx *Context) Element {
		return Div().KeyContext("Outer").OnAction(outer, rec("outer")).OnAction(global, rec("global")).
			Child(Div().KeyContext("Inner").OnAction(inner, rec("inner")).
				Child(TextInput(v).FocusHandle(field)))
	})
	// Added first, so each loses at its depth to what comes after it.
	r.rt.AddBindings(
		Bind("f1", global),
		BindIn("Outer", "f1", outer),
		BindIn("Inner", "f1", inner),
		Bind("f2", global),
		BindIn("Outer", "f2", outer),
		// Inner's binding has no handler on the path, so the next one runs.
		BindIn("Inner", "f3", missing),
		Bind("f3", global),
	)
	field.Focus()
	r.frame()

	r.key(input.KeyF1, 0)
	r.key(input.KeyF2, 0)
	r.key(input.KeyF3, 0)
	if want := []string{"inner", "outer", "global"}; !slices.Equal(ran, want) {
		t.Errorf("ran %v, want %v (deepest context, then shallower, then global)", ran, want)
	}
}

func TestHideMarksActions(t *testing.T) {
	a := NewAction("test.hide", "Hidden").Hide()
	b := NewAction("test.show", "Shown")
	if !a.Hidden() || b.Hidden() {
		t.Fatalf("Hidden: a %v, b %v; want true, false", a.Hidden(), b.Hidden())
	}
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(a, func() {}).OnAction(b, func() {})
	})
	if got := r.rt.AvailableActions(); !slices.Equal(got, []*Action{a, b}) {
		t.Errorf("AvailableActions %v, want the hidden action listed too", names(got))
	}
}

// A plain "Palette" binding loses to the field's own Up, because the field's
// binding (Editor) is deeper. The field moves its caret, as in GPUI.
func TestPlainContextLosesToFieldKeys(t *testing.T) {
	up := NewAction("test.plain.up", "Up")
	palette := NewFocusHandle()
	p := NewSignal("hello")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().KeyContext("Palette").OnAction(up, func() { hits++ }).
			Child(TextInput(p).FocusHandle(palette))
	})
	r.rt.AddBindings(BindIn("Palette", "up", up))
	palette.Focus()
	r.frame()
	r.key(input.KeyEnd, 0)
	r.key(input.KeyUp, 0)
	if hits != 0 {
		t.Errorf("Up with a plain Palette binding ran it %d times, want 0 (the field moves its caret)", hits)
	}
	if es := focusedEdit(t, r.rt); es.caret != 0 {
		t.Errorf("Up moved the caret to %d, want 0", es.caret)
	}
}

// Field keys are actions, so an app can bind them to other keys, and a later
// binding for the same key replaces the default.
func TestRebindFieldKeys(t *testing.T) {
	v := NewSignal("one two")
	r := newKeyRig(func(ctx *Context) Element { return Div().Padding(10).Child(TextInput(v)) })
	r.rt.AddBindings(BindIn("Editor", "ctrl+d", deleteForward))
	r.click(100, 27)
	r.key(input.KeyHome, 0)
	r.key(input.KeyD, ctrl)
	if v.Peek() != "ne two" {
		t.Errorf("Ctrl+D (DeleteForward): %q, want %q", v.Peek(), "ne two")
	}
	r.rt.AddBindings(BindIn("Editor", "ctrl+backspace", deleteBack))
	r.key(input.KeyEnd, 0)
	r.key(input.KeyBackspace, ctrl)
	if v.Peek() != "ne tw" {
		t.Errorf("Ctrl+Backspace after rebinding to DeleteBack: %q, want %q", v.Peek(), "ne tw")
	}
}

// The default Escape blurs a single-line field; in a multi-line field Tab
// indents, and Shift+Tab still moves focus.
func TestFieldEscapeEnterAndTab(t *testing.T) {
	v, area := NewSignal(""), NewSignal("")
	submitted := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(TextInput(v).OnSubmit(func(string) { submitted++ })).
			Child(TextArea(area).Width(Px(300)))
	})
	r.click(20, 27)
	r.key(input.KeyEnter, 0)
	if submitted != 1 {
		t.Errorf("Enter in a single-line field: %d submits, want 1", submitted)
	}
	r.key(input.KeyEscape, 0)
	if r.rt.focusID != 0 {
		t.Errorf("Escape left focus on %d, want none", r.rt.focusID)
	}

	r.click(20, 60)
	r.key(input.KeyTab, 0)
	if area.Peek() != "    " {
		t.Errorf("Tab in a multi-line field typed %q, want four spaces", area.Peek())
	}
	r.key(input.KeyTab, input.ModShift)
	if r.rt.focusID == 0 {
		t.Error("Shift+Tab in a multi-line field did not move focus")
	}
}

// Every editor action is hidden and has a default key, so menus leave it out
// and the keys work out of the box.
func TestEditorActionsHiddenAndBound(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element { return Div() })
	for _, e := range editorActions {
		if !e.a.Hidden() {
			t.Errorf("%s is not hidden", e.a.Name())
		}
		if r.rt.KeysFor(e.a) == "" {
			t.Errorf("%s has no default key", e.a.Name())
		}
	}
	if got := r.rt.KeysFor(redo); got != "Ctrl+Y" { // the last default for Redo
		t.Errorf("KeysFor(Redo) = %q, want Ctrl+Y", got)
	}
}

// NoAction turns a key off in a field: the field does nothing, ancestors see
// the key, and a lower-ranked global binding for it does not run.
func TestNoActionTurnsKeyOff(t *testing.T) {
	selAll := NewAction("test.noaction.selall", "Select all")
	field := NewFocusHandle()
	v := NewSignal("hello")
	hits, seen := 0, 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(selAll, func() { hits++ }).
			OnKeyDown(func(e *KeyEvent) {
				if e.Key == input.KeyA {
					seen++
				}
			}).
			Child(TextInput(v).FocusHandle(field))
	})
	r.rt.AddBindings(BindIn("Editor", "ctrl+a", NoAction), Bind("ctrl+a", selAll))
	field.Focus()
	r.frame()
	r.key(input.KeyA, ctrl)
	if a, b := focusedEdit(t, r.rt).selection(); a != b {
		t.Errorf("Ctrl+A with NoAction selected [%d,%d), want nothing selected", a, b)
	}
	if hits != 0 {
		t.Errorf("the global Ctrl+A binding ran (%d hits), want 0", hits)
	}
	if seen != 1 {
		t.Errorf("the ancestor's OnKeyDown saw Ctrl+A %d times, want 1", seen)
	}
}

// A NoAction ranked below an action binding for the same keys does not affect
// it: the deeper binding runs, and the field's own binding stays off.
func TestNoActionBelowActionBindingDoesNothing(t *testing.T) {
	act := NewAction("test.noaction.below", "Below")
	field := NewFocusHandle()
	v := NewSignal("hello")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().KeyContext("Inner").OnAction(act, func() { hits++ }).
			Child(TextInput(v).FocusHandle(field))
	})
	// Both match at the same depth; the one added later ranks higher.
	r.rt.AddBindings(BindIn("Inner > Editor", "ctrl+a", NoAction), BindIn("Inner > Editor", "ctrl+a", act))
	field.Focus()
	r.frame()
	r.key(input.KeyA, ctrl)
	if hits != 1 {
		t.Errorf("Ctrl+A: %d hits, want 1", hits)
	}
}

// Unbind(a) removes a's later bindings for the same keys, and only those: a
// global binding for the keys still runs.
func TestUnbindRemovesActionForKeys(t *testing.T) {
	global := NewAction("test.unbind.global", "Global")
	field := NewFocusHandle()
	v := NewSignal("hello")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(global, func() { hits++ }).Child(TextInput(v).FocusHandle(field))
	})
	r.rt.AddBindings(BindIn("Editor", "ctrl+z", Unbind(undo)), Bind("ctrl+z", global))
	field.Focus()
	r.frame()
	r.key(input.KeyEnd, 0)
	r.rt.TextInput("!")
	r.frame()
	r.key(input.KeyZ, ctrl)
	if v.Peek() != "hello!" {
		t.Errorf("Ctrl+Z with Undo unbound: value %q, want \"hello!\"", v.Peek())
	}
	if hits != 1 {
		t.Errorf("Ctrl+Z: the global binding ran %d times, want 1", hits)
	}
}

// Unbind of another action leaves Undo working.
func TestUnbindOfOtherActionChangesNothing(t *testing.T) {
	field := NewFocusHandle()
	v := NewSignal("hello")
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Child(TextInput(v).FocusHandle(field))
	})
	r.rt.AddBindings(BindIn("Editor", "ctrl+z", Unbind(selectAll)))
	field.Focus()
	r.frame()
	r.key(input.KeyEnd, 0)
	r.rt.TextInput("!")
	r.frame()
	r.key(input.KeyZ, ctrl)
	if v.Peek() != "hello" {
		t.Errorf("Ctrl+Z after Unbind(SelectAll): value %q, want \"hello\" (undone)", v.Peek())
	}
}

// KeysFor skips a binding that a later NoAction turns off, and returns the
// binding that is still in force.
func TestKeysForSkipsTurnedOffBindings(t *testing.T) {
	r := newKeyRig(func(ctx *Context) Element { return Div() })
	if got := r.rt.KeysFor(selectAll); got != "Ctrl+A" {
		t.Fatalf("KeysFor(SelectAll) by default = %q, want Ctrl+A", got)
	}
	r.rt.AddBindings(BindIn("Editor", "ctrl+a", NoAction))
	if got := r.rt.KeysFor(selectAll); got != "" {
		t.Errorf("KeysFor(SelectAll) after NoAction on its keys = %q, want \"\"", got)
	}
	r.rt.AddBindings(Bind("ctrl+a", selectAll))
	if got := r.rt.KeysFor(selectAll); got != "Ctrl+A" {
		t.Errorf("KeysFor(SelectAll) with a global binding = %q, want Ctrl+A (other context)", got)
	}

	r = newKeyRig(func(ctx *Context) Element { return Div() })
	r.rt.AddBindings(BindIn("Editor", "ctrl+y", Unbind(redo)))
	if got := r.rt.KeysFor(redo); got != "Ctrl+Shift+Z" {
		t.Errorf("KeysFor(Redo) after Unbind on Ctrl+Y = %q, want Ctrl+Shift+Z", got)
	}
	if got := r.rt.KeysFor(NoAction); got != "" {
		t.Errorf("KeysFor(NoAction) = %q, want \"\"", got)
	}
}

// A key that is both a binding and the start of a longer chord waits. The
// chord completes it; if the next key matches nothing, the held key runs as
// its own binding, and the key that followed goes on normally.
func TestChordWaitsThenFallsBack(t *testing.T) {
	single := NewAction("test.wait.single", "Single")
	chord := NewAction("test.wait.chord", "Chord")
	counts := map[string]int{}
	xSeen := 0
	var wakes []time.Time
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(single, func() { counts["single"]++ }).
			OnAction(chord, func() { counts["chord"]++ }).
			OnKeyDown(func(e *KeyEvent) {
				if e.Key == input.KeyX {
					xSeen++
				}
			})
	})
	r.rt.hostSetServices(platformServices{WakeAt: func(t time.Time) { wakes = append(wakes, t) }})
	r.rt.AddBindings(Bind("ctrl+k", single), Bind("ctrl+k ctrl+t", chord))

	r.key(input.KeyK, ctrl)
	if counts["single"] != 0 || len(r.rt.pending) != 1 {
		t.Fatalf("after ctrl+k: %v, pending %d; want nothing run and one stroke waiting", counts, len(r.rt.pending))
	}
	if len(wakes) != 1 || !wakes[0].Equal(r.now.Add(chordTimeout)) {
		t.Errorf("wake requests %v, want one at the chord timeout", wakes)
	}

	r.key(input.KeyT, ctrl)
	if counts["chord"] != 1 || counts["single"] != 0 {
		t.Errorf("ctrl+k ctrl+t: %v, want only the chord", counts)
	}

	r.key(input.KeyK, ctrl)
	r.now = r.now.Add(1100 * time.Millisecond)
	r.frame()
	if counts["single"] != 1 || len(r.rt.pending) != 0 {
		t.Errorf("ctrl+k then a timeout and a frame: %v, pending %d; want the single binding to run", counts, len(r.rt.pending))
	}

	r.key(input.KeyK, ctrl)
	r.key(input.KeyX, ctrl) // no chord starts with ctrl+k ctrl+x
	if counts["single"] != 2 || xSeen != 1 {
		t.Errorf("ctrl+k ctrl+x: %v, x seen %d times; want the single binding, then Ctrl+X handled normally", counts, xSeen)
	}
}

// The text of a held key is typed if the chord falls back, and not otherwise:
// with a global "g g", typing "go" in a field still types both letters.
func TestChordReplaysTextOfHeldKeys(t *testing.T) {
	top := NewAction("test.replay.top", "Top")
	field := NewFocusHandle()
	v := NewSignal("")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(top, func() { hits++ }).Child(TextInput(v).FocusHandle(field))
	})
	r.rt.AddBindings(Bind("g g", top))
	field.Focus()
	r.frame()
	press := func(k input.Key, text string) {
		r.rt.KeyDown(k, 0, false)
		if text != "" {
			r.rt.TextInput(text)
		}
		r.rt.KeyUp(k, 0)
		r.frame()
	}

	press(input.KeyG, "g")
	press(input.KeyO, "o")
	if v.Peek() != "go" || hits != 0 {
		t.Errorf("typing g then o: value %q, %d hits; want \"go\" and no hits", v.Peek(), hits)
	}

	v.Set("")
	press(input.KeyG, "g")
	press(input.KeyG, "g")
	if hits != 1 || v.Peek() != "" {
		t.Errorf("g g: %d hits, value %q; want 1 hit and no text", hits, v.Peek())
	}

	press(input.KeyG, "g")
	r.now = r.now.Add(1100 * time.Millisecond)
	r.frame()
	if v.Peek() != "g" || hits != 1 {
		t.Errorf("g then a timeout: value %q, %d hits; want \"g\" typed", v.Peek(), hits)
	}
}

// A chord that began in one focused element does not complete in another: its
// held keys are dropped, not replayed into the new field.
func TestChordDroppedWhenFocusMoves(t *testing.T) {
	top := NewAction("test.drop.top", "Top")
	a, b := NewFocusHandle(), NewFocusHandle()
	va, vb := NewSignal(""), NewSignal("")
	hits := 0
	r := newKeyRig(func(ctx *Context) Element {
		return Div().OnAction(top, func() { hits++ }).Column().
			Child(TextInput(va).FocusHandle(a)).
			Child(TextInput(vb).FocusHandle(b))
	})
	r.rt.AddBindings(Bind("g g", top))
	a.Focus()
	r.frame()
	r.rt.KeyDown(input.KeyG, 0, false)
	r.rt.TextInput("g")
	r.rt.KeyUp(input.KeyG, 0)
	b.Focus()
	r.frame()
	r.rt.KeyDown(input.KeyO, 0, false)
	r.rt.TextInput("o")
	r.rt.KeyUp(input.KeyO, 0)
	r.frame()
	if va.Peek() != "" || vb.Peek() != "o" || hits != 0 {
		t.Errorf("g in a, then focus moves to b and o: a %q, b %q, %d hits; want \"\", \"o\" and no hits", va.Peek(), vb.Peek(), hits)
	}
}
