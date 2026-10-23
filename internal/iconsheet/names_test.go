package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docs/drawing.md names the icons that are not plain Lucide names (the aliases
// and the hand-made icons); keep its list in sync with icons/set.go and
// icons/lucide_gen.go.
func TestDrawingDocListsRenamedIcons(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "docs", "drawing.md"))
	if err != nil {
		t.Skip("docs/drawing.md not found:", err)
	}
	for _, name := range []string{
		"Alert", "Dashboard", "Filter", "Home", "MoreHorizontal", "Refresh",
		"Maximize", "Minimize", "Restore", "StarFilled", "LucideMaximize", "LucideMinimize",
	} {
		if !strings.Contains(string(readme), "icons."+name) {
			t.Errorf("docs/drawing.md does not mention icons.%s", name)
		}
	}
}
