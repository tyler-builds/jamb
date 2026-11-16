package markdown

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

const mixedDoc = "# Title\n\nFirst paragraph.\n\n- one\n- two\n\n```go\na := 1\nb := 2\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\nLast line."

// textBounds returns the bounds of the static text named name.
func textBounds(t *testing.T, u *uitest.UI, name string) ui.Rect {
	t.Helper()
	return u.ByRole(access.RoleStaticText, name).Bounds()
}

// drag drags the left button from x0, y0 to x1, y1.
func drag(u *uitest.UI, x0, y0, x1, y1 float32) {
	rt := u.Runtime()
	rt.PointerMove(x0, y0, 0)
	rt.PointerDown(x0, y0, input.ButtonLeft, 1, 0)
	rt.PointerMove((x0+x1)/2, (y0+y1)/2, 0)
	rt.PointerMove(x1, y1, 0)
	rt.PointerUp(x1, y1, input.ButtonLeft, 0)
	u.Frame()
}

// copied returns what Ctrl+C puts on the clipboard.
func copied(u *uitest.UI) string {
	u.SetClipboard("")
	u.Press("ctrl+c")
	return u.Clipboard()
}

// The blocks of a view are one selection: a drag from the first paragraph to
// the last selects all between them, and Ctrl+C copies it with a blank line
// between blocks, a line break between list items and table rows, a tab
// between cells, and the lines of a code block as they are.
func TestSelectAcrossBlocks(t *testing.T) {
	u := show(t, func() *ViewElement { return View(mixedDoc) })
	first, last := textBounds(t, u, "First paragraph."), textBounds(t, u, "Last line.")
	drag(u, first.X+1, first.Y+first.H/2, last.X+last.W+20, last.Y+last.H/2)
	want := "First paragraph.\n\none\ntwo\n\na := 1\nb := 2\n\na\tb\n1\t2\n\nLast line."
	if got := copied(u); got != want {
		t.Errorf("drag copied %q, want %q", got, want)
	}
	u.Press("ctrl+a")
	if got := copied(u); got != "Title\n\n"+want {
		t.Errorf("Ctrl+A copied %q, want %q", got, "Title\n\n"+want)
	}
}

// A drag upward, from the code block into the paragraph, selects the same as
// one downward; a selection that ends at the start of a block leaves it out.
func TestSelectAcrossBlocksUpward(t *testing.T) {
	u := show(t, func() *ViewElement { return View(mixedDoc) })
	first := textBounds(t, u, "First paragraph.")
	code := textBounds(t, u, "a := 1\nb := 2")
	line := code.H / 2 // the code's first line is in its top half
	drag(u, code.X+1, code.Y+line/2, first.X+30, first.Y+first.H/2)
	got := copied(u)
	if len(got) < len("paragraph.\n\none\ntwo") || got[len(got)-len("\n\none\ntwo"):] != "\n\none\ntwo" {
		t.Errorf("upward drag copied %q, want the paragraph's tail and the list", got)
	}
	// Shift+click in the last paragraph extends the selection from where the
	// drag began, at the start of the code block.
	last := textBounds(t, u, "Last line.")
	rt := u.Runtime()
	rt.PointerDown(last.X+last.W-10, last.Y+last.H/2, input.ButtonLeft, 1, input.ModShift)
	rt.PointerUp(last.X+last.W-10, last.Y+last.H/2, input.ButtonLeft, input.ModShift)
	u.Frame()
	if got, want := copied(u), "a := 1\nb := 2\n\na\tb\n1\t2\n\nLast line."; got != want {
		t.Errorf("Shift+click copied %q, want %q", got, want)
	}
}
