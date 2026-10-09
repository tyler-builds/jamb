package views

import (
	"errors"
	"strings"
	"testing"
)

func TestKeymapErrorShowsInStatusBar(t *testing.T) {
	defer ReportKeymap(nil)
	r := newRig(t)
	ReportKeymap(errors.New(`C:\notes\keymap.json: keymap section 1 (no context): "ctrl+q": unknown action "notes.nope"` + "\nmore"))
	r.frame()
	if _, ok := r.rt.Find("keymap-error"); !ok {
		t.Fatal("no keymap-error in the status bar after a report")
	}
	var b strings.Builder
	r.rt.Dump(&b, 100)
	// Dump quotes text with %q and shortens it to 32 bytes, so check the start:
	// the problem comes first, not the file path and section.
	if !strings.Contains(b.String(), `"keymap: unknown action \"notes.no`) || strings.Contains(b.String(), "more") {
		t.Errorf("status bar does not lead with the problem from the first line of the error:\n%s", b.String())
	}
	ReportKeymap(nil)
	r.frame()
	if _, ok := r.rt.Find("keymap-error"); ok {
		t.Error("keymap-error still shown after a clean load")
	}
}
