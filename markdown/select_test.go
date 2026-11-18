package markdown

import (
	"image/color"
	"strings"
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

// isRed reports whether c is exactly red: the background of inline code in the test.
func isRed(c color.RGBA) bool { return c.R == 255 && c.G == 0 && c.B == 0 }

// redPixels counts the red pixels of the screenshot in the rows of r and the
// columns from x0 to x1.
func redPixels(u *uitest.UI, r ui.Rect, x0, x1 float32) int {
	img := u.Screenshot()
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(x0); x < int(x1); x++ {
			if isRed(img.RGBAAt(x, y)) {
				n++
			}
		}
	}
	return n
}

// redColumns returns the first and the last column of the screenshot that has
// a red pixel on the middle row of r.
func redColumns(t *testing.T, u *uitest.UI, r ui.Rect) (float32, float32) {
	t.Helper()
	img := u.Screenshot()
	y, first, last := int(r.Y+r.H/2), -1, -1
	for x := int(r.X); x < int(r.X+r.W); x++ {
		if isRed(img.RGBAAt(x, y)) {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	if first < 0 {
		t.Fatal("the inline code has no red background to select over")
	}
	return float32(first), float32(last)
}

// The selection shows over inline code with an opaque background, as over plain
// text, whether it covers the code and the text around it or lies inside the
// code; and Ctrl+C copies the code with the text.
func TestSelectionShowsOverInlineCode(t *testing.T) {
	red := ui.RGB(255, 0, 0)
	u := show(t, func() *ViewElement {
		return View("Gaps 34 to 37 in `docs/gaps.md`.\n\n`internal/gui/design.go`").
			Styles(Style{InlineCode: ui.Style{Background: red}})
	})
	mixed := textBounds(t, u, "Gaps 34 to 37 in docs/gaps.md.")
	_, end := redColumns(t, u, mixed)
	drag(u, mixed.X+1, mixed.Y+mixed.H/2, end+20, mixed.Y+mixed.H/2)
	if got := copied(u); got != "Gaps 34 to 37 in docs/gaps.md." {
		t.Errorf("copied %q, want the paragraph with its code", got)
	}
	if n := redPixels(u, mixed, mixed.X, mixed.X+mixed.W); n > 0 {
		t.Errorf("%d pixels of the selected code show its background without the selection", n)
	}

	code := textBounds(t, u, "internal/gui/design.go")
	x0, x1 := redColumns(t, u, code)
	w := x1 - x0
	drag(u, x0+w/4, code.Y+code.H/2, x0+w*3/4, code.Y+code.H/2)
	if got := copied(u); got == "" || !strings.Contains("internal/gui/design.go", got) {
		t.Errorf("copied %q, want a part of the code", got)
	}
	if n := redPixels(u, code, x0+w/3, x0+w*2/3); n > 0 {
		t.Errorf("%d pixels inside a selection within the code show its background without the selection", n)
	}
	if redPixels(u, code, x0, x0+w/8) == 0 {
		t.Error("the code before the selection lost its background")
	}
}
