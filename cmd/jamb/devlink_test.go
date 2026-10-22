package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The overlay adds the devtools link to the app's package without writing into the
// app's directory, so that jamb dev links the inspector into any app it builds.
func TestDevtoolsOverlayLinksTheInspectorIntoTheAppPackage(t *testing.T) {
	dir, work := t.TempDir(), t.TempDir()
	overlay, err := devtoolsOverlay(work, dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatal(err)
	}
	var o struct{ Replace map[string]string }
	if err := json.Unmarshal(b, &o); err != nil {
		t.Fatalf("the overlay is not JSON: %v", err)
	}
	src, ok := o.Replace[filepath.Join(dir, devtoolsLinkName)]
	if !ok {
		t.Fatalf("the overlay maps %v, want the app's package directory", o.Replace)
	}
	text, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "package main") || !strings.Contains(string(text), `import _ "`+devtoolsImport+`"`) {
		t.Errorf("the link is %q, want a blank import of %s in package main", text, devtoolsImport)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("the app's directory was written to: %d entries", len(entries))
	}
}
