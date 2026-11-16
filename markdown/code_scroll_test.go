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

// A code block whose line is wider than the page scrolls sideways to the end of
// the line. Scrolling only vertically over the
// block scrolls the page, and a diagonal scroll over it scrolls the block
// sideways and the page down.
func TestCodeBlockScrollsSideways(t *testing.T) {
	line := strings.Repeat("unbrokenword_", 12) + "end"
	code := line + "\nshort"
	src := strings.Repeat("A paragraph before the code.\n\n", 8) + "```\n" + code + "\n```\n\n" +
		strings.Repeat("A paragraph after the code.\n\n", 30)
	u := show(t, func() *ViewElement { return View(src) })
	block := u.ByRole(access.RoleGroup, "Code")
	text := u.ByName(code)
	at := text.Bounds()
	sc := block.Node().Scroll
	if !sc.CanX || sc.CanY {
		t.Fatalf("the block scrolls %+v; want it to scroll sideways only", sc)
	}
	// The block is as wide as the page, so the part of the line it shows is the
	// page's part of the whole: the rest is past its edge.
	if sc.ViewX >= 90 {
		t.Errorf("the block shows %v%% of its line; want much of it past the edge", sc.ViewX)
	}

	block.Scroll(0, 120)
	after := text.Bounds()
	if after.X != at.X || after.Y >= at.Y {
		t.Fatalf("a vertical scroll over the block moved its text from %v to %v; want the page scrolled", at, after)
	}

	at = after
	block.Scroll(5000, 0)
	after = text.Bounds()
	if after.Y != at.Y || after.X >= at.X {
		t.Fatalf("a horizontal scroll over the block moved its text from %v to %v; want the block scrolled", at, after)
	}
	if x := block.Node().Scroll.X; x != 100 {
		t.Errorf("a long scroll right took the block to %v%%; want its end", x)
	}

	at = after
	block.Scroll(-40, 60)
	after = text.Bounds()
	if after.X <= at.X || after.Y >= at.Y {
		t.Errorf("a diagonal scroll over the block moved its text from %v to %v; want it moved right by the block and up by the page", at, after)
	}
}

// Selecting a code block's text and copying it gives the whole of its long line,
// unwrapped, though the block shows only part of it.
func TestCodeBlockCopiesItsWholeLine(t *testing.T) {
	line := strings.Repeat("unbrokenword_", 12) + "end"
	code := line + "\n    indented  twice"
	u := show(t, func() *ViewElement { return View("```\n" + code + "\n```\n") })
	text := u.ByName(code)
	text.Click()
	u.Press("Ctrl+A")
	u.Press("Ctrl+C")
	if got := u.Clipboard(); got != code {
		t.Errorf("copying the block gave %q; want %q", got, code)
	}
}

// Scrolled to the end of a line far longer than the block, so that the text's
// box is scrolled out of the block, the end of the line is still drawn, and a
// double click on its last word selects that word.
func TestCodeBlockShowsAndSelectsTheEndOfALongLine(t *testing.T) {
	line := strings.Repeat("unbroken word ", 12) + "lastword"
	code := line + "\nshort"
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Size(ui.Full, ui.Full).ScrollY().Child(View("```\n" + code + "\n```\n").Key("doc"))
	}, uitest.Size(300, 300))
	u.Frame()
	block := u.ByRole(access.RoleGroup, "Code")
	block.Scroll(5000, 0)
	u.Settle()
	box, text := block.Bounds(), u.ByName(code).Bounds()
	if text.Right() > box.X {
		t.Fatalf("the text's box ends at %v, inside the block from %v: the line is too short to test", text.Right(), box.X)
	}
	// The last word is drawn near the block's right edge, on the first line.
	x, y := box.Right()-40, box.Y+20
	img := u.Screenshot()
	bg := img.At(int(box.X)+2, int(box.Bottom())-20)
	drawn := false
	for px := int(box.Right()) - 80; px < int(box.Right())-15 && !drawn; px++ {
		for py := int(box.Y) + 10; py < int(box.Y)+30; py++ {
			if !sameColor(img.At(px, py), bg) {
				drawn = true
				break
			}
		}
	}
	if !drawn {
		t.Errorf("the end of the line is not drawn in the block")
	}
	rt := u.Runtime()
	rt.PointerMove(x, y, 0)
	for clicks := 1; clicks <= 2; clicks++ {
		rt.PointerDown(x, y, input.ButtonLeft, clicks, 0)
		rt.PointerUp(x, y, input.ButtonLeft, 0)
	}
	u.Frame()
	u.Press("Ctrl+C")
	if got := u.Clipboard(); got != "lastword" {
		t.Errorf("a double click on the line's last word copied %q; want %q", got, "lastword")
	}
}

func sameColor(a, b color.Color) bool {
	r1, g1, b1, _ := a.RGBA()
	r2, g2, b2, _ := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2
}
