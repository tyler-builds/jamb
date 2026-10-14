package codeedit

import "testing"

// TestTextPatternNamesItsFont checks that the Text pattern reports the name of the monospace face the
// editor draws with, not the internal family key.
func TestTextPatternNamesItsFont(t *testing.T) {
	h := newHarness(t, "text")
	activateAccess(h)
	h.frame()
	node := accessField(t, h)
	if node.Text == nil {
		t.Fatal("the editor has no Text pattern")
	}
	if name := node.Text.FontName; name == "" || name == "mono" {
		t.Fatalf("FontName = %q, want the face's own name", name)
	}
}
