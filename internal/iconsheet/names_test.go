package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/icons"
)

// The icon list in docs/drawing.md names every icon; keep it in sync with icons.All.
func TestDrawingDocListsEveryIcon(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "docs", "drawing.md"))
	if err != nil {
		t.Skip("docs/drawing.md not found:", err)
	}
	for _, ic := range icons.All {
		if !strings.Contains(string(readme), ic.Name) {
			t.Errorf("docs/drawing.md icon list is missing %q", ic.Name)
		}
	}
}
