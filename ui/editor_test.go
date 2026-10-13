package ui

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

type editorRig struct {
	rt  *Runtime
	now time.Time
}

func newEditorRig(t *testing.T, root func(*Context) Element) *editorRig {
	r := &editorRig{now: time.Unix(1_000_000, 0)}
	r.rt = NewRuntime(root, DarkTheme(), nil)
	r.rt.clock = func() time.Time { return r.now }
	r.rt.SetViewport(500, 400, 1)
	frame(r.rt)
	return r
}

func (r *editorRig) click(x, y float32) {
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	frame(r.rt)
}

func (r *editorRig) typeText(s string) {
	for _, c := range s {
		r.rt.TextInput(string(c))
		r.now = r.now.Add(50 * time.Millisecond)
	}
	frame(r.rt)
}

func (r *editorRig) key(k input.Key, mods input.Modifiers) {
	r.rt.KeyDown(k, mods, false)
	r.now = r.now.Add(50 * time.Millisecond)
	frame(r.rt)
}

func (r *editorRig) editor() (*node, *editState) {
	n := r.rt.byID[r.rt.focusID]
	if n == nil || n.editor == nil {
		return nil, nil
	}
	return n, n.st.edit
}

const ctrl, shift = input.ModCtrl, input.ModShift

func TestEditorTypingSelectionAndUndo(t *testing.T) {
	v := NewSignal("")
	r := newEditorRig(t, func(ctx *Context) Element { return Div().Padding(10).Child(TextInput(v)) })
	r.click(100, 27)
	r.typeText("hello world")
	if v.Peek() != "hello world" {
		t.Fatalf("value %q", v.Peek())
	}
	// Select "world" with Ctrl+Shift+Left and replace it.
	r.key(input.KeyLeft, ctrl|shift)
	_, es := r.editor()
	if a, b := es.selection(); v.Peek()[a:b] != "world" {
		t.Fatalf("selected %q", v.Peek()[a:b])
	}
	r.typeText("there")
	if v.Peek() != "hello there" {
		t.Fatalf("after replacing the selection: %q", v.Peek())
	}
	// "t" replaced the selection (its own undo step); "here" was grouped typing.
	r.key(input.KeyZ, ctrl)
	if v.Peek() != "hello t" {
		t.Fatalf("first undo -> %q, want %q", v.Peek(), "hello t")
	}
	for v.Peek() != "" {
		before := v.Peek()
		r.key(input.KeyZ, ctrl)
		if v.Peek() == before {
			t.Fatalf("undo stuck at %q", before)
		}
	}
	r.key(input.KeyY, ctrl)
	if v.Peek() != "hello " {
		t.Errorf("redo after undoing everything: %q, want the first word group %q", v.Peek(), "hello ")
	}
}

func TestEditorUndoGroupsTypingByWord(t *testing.T) {
	v := NewSignal("")
	r := newEditorRig(t, func(ctx *Context) Element { return TextInput(v) })
	r.click(50, 17)
	r.typeText("one two three")
	r.key(input.KeyZ, ctrl)
	if v.Peek() != "one two " {
		t.Errorf("undo removed back to %q, want %q (one word per step)", v.Peek(), "one two ")
	}
	r.now = r.now.Add(5 * time.Second)
	r.typeText("x")
	r.now = r.now.Add(5 * time.Second)
	r.typeText("y")
	r.key(input.KeyZ, ctrl)
	if v.Peek() != "one two x" {
		t.Errorf("a pause should split undo steps: got %q", v.Peek())
	}
}

func TestEditorClipboard(t *testing.T) {
	v := NewSignal("alpha beta")
	r := newEditorRig(t, func(ctx *Context) Element { return TextInput(v) })
	r.click(50, 17)
	r.key(input.KeyA, ctrl)
	r.key(input.KeyC, ctrl)
	r.key(input.KeyEnd, 0)
	r.typeText(" ")
	r.key(input.KeyV, ctrl)
	if v.Peek() != "alpha beta alpha beta" {
		t.Fatalf("after copy/paste: %q", v.Peek())
	}
	r.key(input.KeyHome, 0)
	r.key(input.KeyRight, ctrl|shift) // select "alpha " with its space, as Windows does
	r.key(input.KeyX, ctrl)
	if v.Peek() != "beta alpha beta" {
		t.Errorf("after cut: %q", v.Peek())
	}
	// Pasting multi-line text into a single-line field flattens it.
	r.rt.memClipboard = "a\nb"
	r.key(input.KeyV, ctrl)
	if !strings.HasPrefix(v.Peek(), "a b") {
		t.Errorf("newlines were not flattened: %q", v.Peek())
	}
}

func TestTextAreaLinesAndVerticalMovement(t *testing.T) {
	v := NewSignal("")
	r := newEditorRig(t, func(ctx *Context) Element { return TextArea(v).Width(Px(300)) })
	r.click(50, 20)
	r.typeText("first line")
	r.key(input.KeyEnter, 0)
	r.typeText("2nd")
	r.key(input.KeyEnter, 0)
	r.typeText("third line here")
	if v.Peek() != "first line\n2nd\nthird line here" {
		t.Fatalf("value %q", v.Peek())
	}
	// From the end of line 3, Up lands at the end of the short line 2 and
	// Up again returns to roughly the original column on line 1.
	r.key(input.KeyUp, 0)
	_, es := r.editor()
	if es.caret != len("first line\n2nd") {
		t.Errorf("Up onto a shorter line: caret %d, want %d", es.caret, len("first line\n2nd"))
	}
	r.key(input.KeyUp, 0)
	if es.caret != len("first line") {
		t.Errorf("Up again keeps the preferred column: caret %d, want %d", es.caret, len("first line"))
	}
	r.key(input.KeyHome, 0)
	if es.caret != 0 {
		t.Errorf("Home: %d", es.caret)
	}
	r.key(input.KeyEnd, shift)
	if a, b := es.selection(); v.Peek()[a:b] != "first line" {
		t.Errorf("Shift+End selected %q", v.Peek()[a:b])
	}
	r.key(input.KeyEnd, ctrl)
	if es.caret != len(v.Peek()) {
		t.Errorf("Ctrl+End: %d", es.caret)
	}
}

func TestEditorMouseDragAndDoubleClick(t *testing.T) {
	v := NewSignal("drag across these words")
	r := newEditorRig(t, func(ctx *Context) Element { return Div().Padding(20).Child(TextInput(v).Width(Px(400))) })
	r.rt.PointerDown(42, 37, input.ButtonLeft, 1, 0)
	frame(r.rt)
	_, es := r.editor()
	if es == nil {
		t.Fatal("clicking the field did not focus its editor")
	}
	start := es.caret
	// Drag far outside the field: capture keeps the selection growing to the end.
	r.rt.PointerMove(480, 300, 0)
	r.rt.PointerUp(480, 300, input.ButtonLeft, 0)
	frame(r.rt)
	if es.anchor != start || es.caret != len(v.Peek()) {
		t.Errorf("drag selection [%d,%d), want [%d,%d)", es.anchor, es.caret, start, len(v.Peek()))
	}
	// Double-click selects a word.
	r.now = r.now.Add(time.Second)
	x := float32(20 + 10 + 60)
	r.rt.PointerDown(x, 37, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, 37, input.ButtonLeft, 0)
	r.rt.PointerDown(x, 37, input.ButtonLeft, 2, 0)
	r.rt.PointerUp(x, 37, input.ButtonLeft, 0)
	frame(r.rt)
	a, b := es.selection()
	if w := v.Peek()[a:b]; strings.ContainsRune(w, ' ') || w == "" {
		t.Errorf("double-click selected %q, want one word", w)
	}
}

func TestEditorCtrlBackspaceAndOutsideChange(t *testing.T) {
	v := NewSignal("")
	r := newEditorRig(t, func(ctx *Context) Element { return TextInput(v) })
	r.click(50, 17)
	r.typeText("delete these words")
	r.key(input.KeyBackspace, ctrl)
	if v.Peek() != "delete these " {
		t.Fatalf("Ctrl+Backspace: %q", v.Peek())
	}
	// The app replaces the text: history resets and the caret stays valid.
	v.Set("new")
	frame(r.rt)
	r.key(input.KeyZ, ctrl)
	_, es := r.editor()
	if v.Peek() != "new" || es.caret > len("new") {
		t.Errorf("after an outside change: value %q caret %d", v.Peek(), es.caret)
	}
}

func TestTextAreaKeepsCaretVisibleInScroller(t *testing.T) {
	v := NewSignal(strings.Repeat("line\n", 60))
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Height(Full).ScrollY().Child(TextArea(v).Borderless())
	})
	r.click(20, 10)
	r.key(input.KeyEnd, ctrl)
	scroller := r.rt.tree
	if scroller.st.scrollY == 0 {
		t.Fatal("Ctrl+End did not scroll the caret into view")
	}
	caret, _ := r.rt.TextInputRect()
	if caret.Bottom() > 400 || caret.Y < 0 {
		t.Errorf("caret at y=%v..%v is outside the 400px viewport", caret.Y, caret.Bottom())
	}
	r.key(input.KeyHome, ctrl)
	if scroller.st.scrollY != 0 {
		t.Errorf("Ctrl+Home left scrollY at %v", scroller.st.scrollY)
	}
}

func TestFocusHandleAutoFocusAndFocusWithin(t *testing.T) {
	show := NewSignal(false)
	v, search := NewSignal(""), NewSignal("")
	h := NewFocusHandle()
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().
			Child(TextInput(search).Key("search").FocusHandle(h)).
			Child(If(show.Get(), TextInput(v).Key("title").AutoFocus()))
	})
	h.Focus()
	frame(r.rt)
	if !h.Focused() {
		t.Fatal("FocusHandle.Focus did not focus the search field")
	}
	box := r.rt.tree.children[0] // the search field's frame
	if box.style.BorderColor != DarkTheme().Accent {
		t.Error("FocusWithin did not highlight the field frame")
	}
	show.Set(true)
	r.rt.Frame(time.Now())
	box = r.rt.tree.children[0] // the root re-rendered: look the frame up again
	if h.Focused() {
		t.Error("AutoFocus on the new field did not take focus")
	}
	if box.style.BorderColor == DarkTheme().Accent {
		t.Error("the search frame kept its focus style after losing focus")
	}
}

func TestTextAreaTypingIsIncremental(t *testing.T) {
	v := NewSignal(strings.Repeat("Some paragraph text that wraps a little.\n", 40))
	other := NewSignal(0)
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().
			Child(Component(func(ctx *Context) Element { return Textf("%d", other.Get()) })).
			Child(Div().Height(Px(300)).ScrollY().Child(TextArea(v)))
	})
	r.click(100, 60)
	r.rt.TextInput("x")
	st := frame(r.rt)
	if st.Rendered != 1 {
		t.Errorf("typing re-rendered %d components, want only the text area", st.Rendered)
	}
}

// TestEditorWordKeysUseTheWindowsConvention checks the word keys against Notepad,
// which the Windows convention follows: Ctrl+Right stops at the start of the next
// word unit, a unit being a word or a run of punctuation with the spaces after it,
// Ctrl+Left at the start of the unit before the caret, and Ctrl+Delete and
// Ctrl+Backspace delete to those places. The stops are the ones Notepad shows for
// "A small note-taking app. Next".
func TestEditorWordKeysUseTheWindowsConvention(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the Windows word convention is the one on Windows")
	}
	v := NewSignal("")
	r := newEditorRig(t, func(ctx *Context) Element { return Div().Padding(10).Child(TextInput(v)) })
	r.click(100, 27)
	r.typeText("A small note-taking app. Next")
	caret := func() int {
		_, es := r.editor()
		return es.caret
	}
	r.key(input.KeyHome, 0)
	for _, want := range []int{2, 8, 12, 13, 20, 23, 25, 29} {
		r.key(input.KeyRight, ctrl)
		if got := caret(); got != want {
			t.Fatalf("Ctrl+Right stopped at %d, want %d", got, want)
		}
	}
	for _, want := range []int{25, 23, 20, 13, 12, 8, 2, 0} {
		r.key(input.KeyLeft, ctrl)
		if got := caret(); got != want {
			t.Fatalf("Ctrl+Left stopped at %d, want %d", got, want)
		}
	}
	r.key(input.KeyRight, ctrl|shift) // Ctrl+Shift+Right from the start selects "A "
	if _, es := r.editor(); es != nil {
		if a, b := es.selection(); v.Peek()[a:b] != "A " {
			t.Errorf("Ctrl+Shift+Right selected %q, want \"A \"", v.Peek()[a:b])
		}
	}
	r.key(input.KeyHome, 0)
	r.key(input.KeyDelete, ctrl) // deletes "A " to the start of "small"
	if v.Peek() != "small note-taking app. Next" {
		t.Errorf("Ctrl+Delete left %q", v.Peek())
	}
	r.key(input.KeyEnd, 0)
	r.key(input.KeyBackspace, ctrl) // deletes "Next" back to the start of its unit
	if v.Peek() != "small note-taking app. " {
		t.Errorf("Ctrl+Backspace left %q", v.Peek())
	}
}
