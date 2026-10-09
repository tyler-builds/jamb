package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/icons"
)

// The README lists every icon; keep it in sync with icons.All.
func TestReadmeListsEveryIcon(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "..", "README.md"))
	if err != nil {
		t.Skip("README not found:", err)
	}
	for _, ic := range icons.All {
		if !strings.Contains(string(readme), ic.Name) {
			t.Errorf("README icon list is missing %q", ic.Name)
		}
	}
}
