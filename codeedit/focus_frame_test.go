package codeedit

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// focusStates returns the focus states that style elements of the tree, by the inspector's
// names of their layers ("focus within", "focus visible").
func focusStates(tg inspect.Target) map[string]bool {
	out := map[string]bool{}
	var walk func(id uint64)
	walk = func(id uint64) {
		for _, f := range tg.Fields(id) {
			for _, l := range f.Layers {
				for _, s := range []string{"focus within", "focus visible"} {
					if strings.HasPrefix(l.Source, inspect.SourceState+s) {
						out[s] = true
					}
				}
			}
		}
		if e, ok := tg.Element(id); ok {
			for _, c := range e.Children {
				walk(c)
			}
		}
	}
	walk(tg.Root())
	return out
}

// The editor draws no focus ring: the caret shows focus, as in code editors, so the frame
// does not change on a click or while typing. Under high contrast the frame shows focus
// with a border, from the click on, and typing adds no ring.
func TestEditorFrameFocus(t *testing.T) {
	for _, hc := range []bool{false, true} {
		h := newHarness(t, "package main\n") // the harness clicks into the editor
		if hc {
			h.rt.SetTheme(ui.HighContrastTheme(ui.HighContrastBlack(), ui.DarkTheme()))
		}
		tg := uihost.Of(h.rt).(inspect.Target)
		tg.Attach(func() {})
		h.frame()
		check := func(when string) {
			t.Helper()
			got := focusStates(tg)
			if got["focus visible"] || got["focus within"] != hc {
				t.Errorf("high contrast %v, %s: the focus states are %v, want focus within %v and no focus visible", hc, when, got, hc)
			}
		}
		check("after a click")
		h.typeText("x")
		h.key(input.KeyRight, 0)
		h.frame()
		check("after typing")
	}
}
