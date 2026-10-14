package codeedit

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
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

// The editor's frame shows focus as soon as a click focuses it, and typing changes nothing:
// no focus ring comes with the first key, as it did when the ring was a FocusVisible style.
func TestEditorFrameShowsFocusOnClickAndNotOnTyping(t *testing.T) {
	h := newHarness(t, "package main\n") // the harness clicks into the editor
	tg := uihost.Of(h.rt).(inspect.Target)
	tg.Attach(func() {})
	h.frame()
	if got := focusStates(tg); !got["focus within"] || got["focus visible"] {
		t.Fatalf("after a click the focus states are %v, want focus within only", got)
	}
	h.typeText("x")
	h.key(input.KeyRight, 0)
	h.frame()
	if got := focusStates(tg); !got["focus within"] || got["focus visible"] {
		t.Fatalf("after typing the focus states are %v, want focus within only", got)
	}
}
