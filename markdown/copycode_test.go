package markdown

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// codeBlocks returns the code blocks of the UI's tree, in tree order.
func codeBlocks(u *uitest.UI) []*access.Node {
	var out []*access.Node
	for _, n := range nodes(u, access.RoleGroup) {
		if n.Name == "Code" || strings.HasPrefix(n.Name, "Code, ") {
			out = append(out, n)
		}
	}
	return out
}

// copyButtons returns the copy buttons of the UI's tree, in tree order.
func copyButtons(u *uitest.UI) []*access.Node {
	var out []*access.Node
	for _, n := range nodes(u, access.RoleButton) {
		if n.Name == "Copy" {
			out = append(out, n)
		}
	}
	return out
}

// rect returns the bounds of n, in the logical pixels of a window at zoom 1.
func rect(n *access.Node) ui.Rect {
	return ui.Rect{X: n.Bounds.X, Y: n.Bounds.Y, W: n.Bounds.W, H: n.Bounds.H}
}

// The code of each block of copyDoc, in order.
var copyCodes = []string{
	"first line\n    second, indented\n\nfourth after a blank",
	"npm install",
	"func main() {\n\tfmt.Println(\"hi\")\n}",
	"go test ./...\ngo vet ./...",
	"tilde fence",
}

// clickNode clicks the middle of n.
func clickNode(u *uitest.UI, n *access.Node) {
	u.ClickAt(n.Bounds.X+n.Bounds.W/2, n.Bounds.Y+n.Bounds.H/2)
}

const copyDoc = "Some text with `inline code`.\n\n" +
	"```\nfirst line\n    second, indented\n\nfourth after a blank\n```\n\n" +
	"```\nnpm install\n```\n\n" +
	"```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n```\n\n" +
	"1. An item with code:\n\n   ```sh\n   go test ./...\n   go vet ./...\n   ```\n\n" +
	"~~~\ntilde fence\n~~~\n"

// Each code block's button copies the block's code as it is written between the
// fences: no fence, no language, no trailing line break; in a list, without the
// item's indentation. Code in a line has no button.
func TestCopyCodeCopiesEachBlock(t *testing.T) {
	u := show(t, func() *ViewElement { return View(copyDoc).CopyCode(true) })
	want := copyCodes
	buttons := copyButtons(u)
	if len(buttons) != len(want) {
		t.Fatalf("%d copy buttons; want one for each of the %d code blocks", len(buttons), len(want))
	}
	for i, w := range want {
		u.SetClipboard("")
		clickNode(u, copyButtons(u)[i])
		if got := u.Clipboard(); got != w {
			t.Errorf("button %d copied %q; want %q", i, got, w)
		}
	}
}

// A block with Windows line breaks copies its code with plain ones, as the
// block shows it.
func TestCopyCodeCRLF(t *testing.T) {
	u := show(t, func() *ViewElement { return View("```\r\na\r\nb\r\n```\r\n").CopyCode(true) })
	clickNode(u, copyButtons(u)[0])
	if got := u.Clipboard(); got != "a\nb" {
		t.Errorf("copied %q; want %q", got, "a\nb")
	}
}

// Without CopyCode, code blocks have no button.
func TestCopyCodeIsOffByDefault(t *testing.T) {
	u := show(t, func() *ViewElement { return View(copyDoc) })
	if n := len(copyButtons(u)); n != 0 {
		t.Errorf("%d copy buttons without CopyCode; want none", n)
	}
}

// The button moves neither a block nor its text, in a block of one line, of
// several, with a language and in a list; it sits inside the block's top right
// corner, centered on the first line.
func TestCopyCodeKeepsTheLayout(t *testing.T) {
	bounds := func(on bool) (blocks, texts, buttons []ui.Rect) {
		u := show(t, func() *ViewElement { return View(copyDoc).CopyCode(on) })
		for _, b := range codeBlocks(u) {
			blocks = append(blocks, rect(b))
		}
		for _, c := range copyCodes {
			if u.HasName(c) { // the text of a list item's block is in the item's name
				texts = append(texts, u.ByName(c).Bounds())
			} else {
				texts = append(texts, ui.Rect{})
			}
		}
		texts = append(texts, u.ByName("Some text with inline code.").Bounds())
		for _, b := range copyButtons(u) {
			buttons = append(buttons, rect(b))
		}
		return blocks, texts, buttons
	}
	offBlocks, offTexts, _ := bounds(false)
	onBlocks, onTexts, buttons := bounds(true)
	if len(onBlocks) != 5 || len(offBlocks) != 5 || len(buttons) != 5 {
		t.Fatalf("%d and %d code blocks, %d buttons; want 5 of each", len(offBlocks), len(onBlocks), len(buttons))
	}
	for i := range offBlocks {
		if onBlocks[i] != offBlocks[i] {
			t.Errorf("block %d is at %v with its button, %v without", i, onBlocks[i], offBlocks[i])
		}
	}
	for i := range offTexts {
		// The text's box ends further right, with room for the button; it starts
		// where it did, as high.
		on, off := onTexts[i], offTexts[i]
		if on.X != off.X || on.Y != off.Y || on.H != off.H {
			t.Errorf("text %d is at %v with the button, %v without", i, on, off)
		}
	}
	for i, btn := range buttons {
		box := onBlocks[i]
		if btn.X < box.X || btn.Right() > box.Right() || btn.Y < box.Y || btn.Bottom() > box.Bottom() {
			t.Errorf("button %d at %v is not inside its block at %v", i, btn, box)
		}
		if gap := box.Right() - btn.Right(); gap > 16 {
			t.Errorf("button %d is %v from its block's right edge; want it in the corner", i, gap)
		}
		// Centered on the first line: in a block of one line, on the block's middle.
		line := box.Y + 1 + 12 + 14*1.4/2 // border, padding, half a line
		if mid := btn.Y + btn.H/2; mid < line-2 || mid > line+2 {
			t.Errorf("button %d is centered at %v; want it on the first line, at %v", i, mid, line)
		}
	}
	one := onBlocks[1]
	if mid, btn := one.Y+one.H/2, buttons[1]; btn.Y+btn.H/2 < mid-2 || btn.Y+btn.H/2 > mid+2 {
		t.Errorf("the button of a block of one line is centered at %v; want the block's middle, %v", btn.Y+btn.H/2, mid)
	}
}

// After a copy the button's icon changes for a moment, then comes back.
func TestCopyCodeConfirms(t *testing.T) {
	u := show(t, func() *ViewElement { return View("```\nnpm install\n```\n").CopyCode(true) })
	btn := rect(copyButtons(u)[0])
	shot := func() *image.RGBA {
		r := image.Rect(int(btn.X), int(btn.Y), int(btn.Right()), int(btn.Bottom()))
		img := u.Screenshot()
		out := image.NewRGBA(r)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				out.Set(x, y, img.At(x, y))
			}
		}
		return out
	}
	before := shot()
	clickNode(u, copyButtons(u)[0])
	u.ClickAt(btn.X-200, btn.Bottom()+200) // move the pointer away: the hover look is not the confirmation
	copied := shot()
	if same(before, copied) {
		t.Errorf("the button looks the same after a copy; want a check in place of the copy icon")
	}
	u.Advance(2 * time.Second)
	u.Settle()
	if !same(before, shot()) {
		t.Errorf("the button does not come back to its copy icon after the confirmation")
	}
}

func same(a, b *image.RGBA) bool {
	if a.Rect != b.Rect {
		return false
	}
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			return false
		}
	}
	return true
}

// Tab reaches the button, and Enter copies.
func TestCopyCodeByKeyboard(t *testing.T) {
	u := show(t, func() *ViewElement { return View("```\nnpm install\n```\n").CopyCode(true) })
	u.Press("tab")
	if f := u.Focused().Node(); f.Role != access.RoleButton || f.Name != "Copy" {
		t.Fatalf("Tab: focus on the %s %q; want the copy button", f.Role, f.Name)
	}
	u.Press("enter")
	if got := u.Clipboard(); got != "npm install" {
		t.Errorf("Enter on the button copied %q; want %q", got, "npm install")
	}
}

// With the button, the code is still selected and copied as text, and the
// document's whole selection holds no word of the button.
func TestCopyCodeKeepsSelection(t *testing.T) {
	code := "first line\n    second, indented"
	u := show(t, func() *ViewElement { return View("Before.\n\n```\n" + code + "\n```\n\nAfter.").CopyCode(true) })
	u.ByName(code).Click()
	u.Press("Ctrl+A")
	u.Press("Ctrl+C")
	got := u.Clipboard()
	if !strings.Contains(got, code) || strings.Contains(got, "Copy") {
		t.Errorf("selecting all and copying gave %q; want the document's text, with the code and no button", got)
	}
}

// A long line still scrolls sideways under the button, which stays in the
// corner; scrolled to the end, the line ends clear of the button.
func TestCopyCodeScrollsSideways(t *testing.T) {
	line := strings.Repeat("unbrokenword_", 12) + "end"
	code := line + "\nshort"
	u := show(t, func() *ViewElement { return View("```\n" + code + "\n```\n").CopyCode(true) })
	block := u.ByRole(access.RoleGroup, "Code")
	btn := rect(copyButtons(u)[0])
	// Before the scroll, the line runs under the button, and nothing of it shows
	// between the button and the block's border.
	{
		box := block.Bounds()
		img := u.Screenshot()
		bg := img.At(int(box.X)+4, int(box.Y)+4)
		for x := int(btn.Right()) + 1; x < int(box.Right())-1; x++ {
			for y := int(btn.Y); y < int(btn.Bottom()); y++ {
				if !sameColor(img.At(x, y), bg) {
					t.Fatalf("the long line shows at %d,%d, between the button at %v and the border at %v", x, y, btn, box.Right())
				}
			}
		}
	}
	block.Scroll(5000, 0)
	if x := block.Node().Scroll.X; x != 100 {
		t.Fatalf("a long scroll right took the block to %v%%; want its end", x)
	}
	if now := rect(copyButtons(u)[0]); now != btn {
		t.Errorf("the button moved from %v to %v with the block's scroll", btn, now)
	}
	// The text's last glyph is drawn left of the button: no pixel of the text's
	// color between the button and the end of the line's room.
	box := block.Bounds()
	img := u.Screenshot()
	bg := img.At(int(box.X)+4, int(box.Y)+4) // the block's padding
	for x := int(btn.X) - 3; x < int(btn.X); x++ {
		for y := int(btn.Y); y < int(btn.Bottom()); y++ {
			if !sameColor(img.At(x, y), bg) {
				t.Fatalf("the end of the long line is drawn at %d,%d, under or against the button at %v", x, y, btn)
			}
		}
	}
	// The end of the line shows left of the button.
	drawn := false
	for x := int(btn.X) - 60; x < int(btn.X)-3 && !drawn; x++ {
		for y := int(btn.Y); y < int(btn.Bottom()); y++ {
			if !sameColor(img.At(x, y), bg) {
				drawn = true
				break
			}
		}
	}
	if !drawn {
		t.Errorf("the end of the long line is not drawn left of the button")
	}
}

// A right click on the code opens the menu it opens without the button.
func TestCopyCodeKeepsTheTextMenu(t *testing.T) {
	menu := func(on bool) []string {
		u := show(t, func() *ViewElement { return View("```\nnpm install\n```\n").CopyCode(on) })
		u.ByName("npm install").RightClick()
		var items []string
		for _, n := range nodes(u, access.RoleMenuItem) {
			items = append(items, n.Name)
		}
		return items
	}
	off, on := menu(false), menu(true)
	if len(off) == 0 {
		t.Fatalf("the code has no menu without the button")
	}
	if strings.Join(on, ",") != strings.Join(off, ",") {
		t.Errorf("the code's menu is %q with the button; want %q", on, off)
	}
}
