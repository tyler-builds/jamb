package markdown

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// A wide table scrolls horizontally inside a page that scrolls vertically.
// Scrolling only vertically over the table scrolls the page, and only
// horizontally scrolls the table. A finger's pan scrolls along one axis (see
// the web platform's panAxisAngle), so a page scrolled with a finger that
// drifts sideways over a table scrolls the page.
func TestTableInScrollingPage(t *testing.T) {
	// Each column holds a word that does not wrap, so the table is wider than
	// the page in any font: a column of words that wrap narrows to its longest
	// word, and twelve of those fit the page in Segoe UI or Arial.
	word := "|" + strings.Repeat("unbrokenword", 2) + " "
	cols := strings.Repeat(word, 12) + "|\n" + strings.Repeat("|---", 12) + "|\n" +
		"| first " + strings.Repeat(word, 11) + "|\n"
	src := strings.Repeat("A paragraph before the table.\n\n", 8) + cols + "\n" +
		strings.Repeat("A paragraph after the table.\n\n", 30)
	u := show(t, func() *ViewElement { return View(src) })
	if w := u.ByRole(access.RoleDataGrid, "").Bounds().W; w <= 900 {
		t.Fatalf("the table is %v wide, within the page's 900: it has nothing to scroll", w)
	}
	cell := u.ByName("first")
	at := cell.Bounds()

	cell.Scroll(0, 120)
	after := cell.Bounds()
	if after.X != at.X || after.Y >= at.Y {
		t.Fatalf("a vertical scroll over the table moved its cell from %v to %v; want the page scrolled", at, after)
	}

	at = after
	cell.Scroll(60, 0)
	after = cell.Bounds()
	if after.Y != at.Y || after.X >= at.X {
		t.Errorf("a horizontal scroll over the table moved its cell from %v to %v; want the table scrolled", at, after)
	}
}
