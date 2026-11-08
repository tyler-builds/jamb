package codeedit

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// rightClick presses and releases the right button at (x, y) of the window.
func (h *harness) rightClick(x, y float32) {
	h.rt.PointerMove(x, y, 0)
	h.rt.PointerDown(x, y, input.ButtonRight, 1, 0)
	h.rt.PointerUp(x, y, input.ButtonRight, 0)
	h.frame()
}

// menuItem returns the access node of the open menu's item named name, or nil.
func (h *harness) menuItem(name string) *access.Node {
	return h.rt.AccessTree().Find(access.RoleMenuItem, name)
}

// choose clicks the open menu's item at index i, separators counted.
func (h *harness) choose(i int) {
	h.t.Helper()
	r, ok := h.rt.Find("jamb.menu.0." + string(rune('0'+i)))
	if !ok {
		h.t.Fatalf("no menu item %d", i)
	}
	h.click(r.X+r.W/2, r.Y+r.H/2, 0, 1)
}

// A right click in the text keeps a selection it lands in and opens the editor's menu,
// whose Copy and Paste run the editor's commands; a right click outside the selections
// moves the caret there first.
func TestContextMenu(t *testing.T) {
	h := newHarness(t, "alpha beta")
	h.doc.SetSelections(Selection{Anchor: 0, Head: 5})
	x, y := h.lineXY(0, 2)
	h.rightClick(x, y)
	for name, disabled := range map[string]bool{"Undo": true, "Cut": false, "Copy": false, "Paste": false, "Select all": false} {
		n := h.menuItem(name)
		if n == nil {
			t.Fatalf("the menu has no %s", name)
		}
		if got := n.States.Has(access.StateDisabled); got != disabled {
			t.Errorf("%s disabled = %v, want %v", name, got, disabled)
		}
	}
	h.choose(4) // Copy
	if got := h.doc.Primary(); got != (Selection{Anchor: 0, Head: 5}) {
		t.Fatalf("the selection is %v after Copy, want the word kept", got)
	}
	x, y = h.lineXY(0, 10)
	h.rightClick(x, y) // after the text, outside the selection: the caret moves to the end
	if h.menuItem("Copy").States.Has(access.StateDisabled) == false {
		t.Error("Copy is enabled after the press moved the caret out of the selection")
	}
	h.choose(5) // Paste
	if got := h.doc.Text(); got != "alpha betaalpha" {
		t.Errorf("text %q after Paste, want the word pasted at the end", got)
	}
}

// A read-only editor's menu has only Copy and Select all.
func TestReadOnlyContextMenu(t *testing.T) {
	h := newHarness(t, "fixed", func(e *EditorElement) *EditorElement { return e.ReadOnly(true) })
	x, y := h.lineXY(0, 1)
	h.rightClick(x, y)
	for _, name := range []string{"Undo", "Redo", "Cut", "Paste"} {
		if h.menuItem(name) != nil {
			t.Errorf("a read-only editor's menu has %s", name)
		}
	}
	if h.menuItem("Copy") == nil || h.menuItem("Select all") == nil {
		t.Error("a read-only editor's menu lacks Copy or Select all")
	}
}
