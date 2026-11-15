package markdown

import (
	"strings"
	"testing"
)

// A wide table scrolls horizontally inside a page that scrolls vertically.
// Scrolling only vertically over the table scrolls the page, and only
// horizontally scrolls the table. A finger's pan scrolls along one axis (see
// the web platform's panAxisAngle), so a page scrolled with a finger that
// drifts sideways over a table scrolls the page.
func TestTableInScrollingPage(t *testing.T) {
	cols := strings.Repeat("| column with a long heading ", 12) + "|\n" + strings.Repeat("|---", 12) + "|\n" +
		"| first " + strings.Repeat("| cell ", 11) + "|\n"
	src := strings.Repeat("A paragraph before the table.\n\n", 8) + cols + "\n" +
		strings.Repeat("A paragraph after the table.\n\n", 30)
	u := show(t, func() *ViewElement { return View(src) })
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
