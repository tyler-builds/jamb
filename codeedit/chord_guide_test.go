package codeedit_test

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/codeedit"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// The recipe of "Keys" in docs/codeedit.md: an app's Ctrl+K waits for the editor's
// Ctrl+K Ctrl+I chord, and runs at once when the app unbinds that chord.

var palette = ui.NewAction("test.palette", "Command palette")

// chordUI shows an editor inside an element that handles palette, with the app's
// bindings bs, and returns the UI and the count of palette's runs.
func chordUI(t *testing.T, bs ...ui.Binding) (*uitest.UI, *int) {
	runs := new(int)
	doc := codeedit.NewDocument("select 1")
	root := func(*ui.Context) ui.Element {
		return ui.Div().OnAction(palette, func() { *runs++ }).
			Child(codeedit.Editor(doc).Key("sql").Label("Query").Size(ui.Px(400), ui.Px(200)))
	}
	u := uitest.New(t, root, uitest.Bindings(bs...))
	u.Get("sql").Click()
	return u, runs
}

// A global Ctrl+K, and one in the editor's context, wait.
func TestCtrlKWaitsForTheHoverChord(t *testing.T) {
	for _, b := range []ui.Binding{ui.Bind("ctrl+k", palette), ui.BindIn("CodeEditor", "ctrl+k", palette)} {
		u, runs := chordUI(t, b)
		u.Press("ctrl+k")
		if *runs != 0 {
			t.Fatal("Ctrl+K ran at once, without waiting for the editor's Ctrl+K Ctrl+I")
		}
		u.Advance(1100 * time.Millisecond)
		if *runs != 1 {
			t.Fatalf("Ctrl+K ran %d times after the chord's second, want 1", *runs)
		}
	}
}

func TestCtrlKRunsAtOnceWithTheChordUnbound(t *testing.T) {
	for _, bs := range [][]ui.Binding{
		// The global binding, with the chord unbound.
		{
			ui.Bind("ctrl+k", palette),
			ui.BindIn("CodeEditor", "ctrl+k ctrl+i", ui.Unbind(codeedit.CmdShowHover)),
		},
		// The editor's own binding, as docs/codeedit.md binds Find, and the hover on another key.
		{
			ui.BindIn("CodeEditor", "ctrl+k ctrl+i", ui.Unbind(codeedit.CmdShowHover)),
			ui.BindIn("CodeEditor", "ctrl+k", palette),
			ui.BindIn("CodeEditor", "ctrl+shift+i", codeedit.CmdShowHover),
		},
	} {
		u, runs := chordUI(t, bs...)
		u.Press("ctrl+k")
		if *runs != 1 {
			t.Errorf("Ctrl+K ran %d times at once, want 1", *runs)
		}
		if len(bs) == 3 {
			if got := u.Runtime().KeysFor(codeedit.CmdShowHover); got != "Ctrl+Shift+I" {
				t.Errorf("the hover's keys are %q, want Ctrl+Shift+I", got)
			}
		}
	}
}
