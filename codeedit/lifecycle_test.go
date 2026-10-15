package codeedit

import (
	"errors"
	"testing"
	"time"

	"github.com/tyler-builds/jamb-grammars/golang"
	"github.com/tyler-builds/jamb/codeedit/highlight"
	"github.com/tyler-builds/jamb/ui"
)

// TestRemovingTheEditorClosesItsSession checks that the highlight session the editor started is
// closed when the editor's component is removed from the tree, and that the editor drops it.
func TestRemovingTheEditorClosesItsSession(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles the Go grammar")
	}
	doc := NewDocument("package main\n")
	ed := Editor(doc).Language(golang.Grammar)
	shown := ui.NewSignal(true)
	root := func(ctx *ui.Context) ui.Element {
		if !shown.Get() {
			return ui.Div().Key("gone")
		}
		return ui.Div().Key("wrap").Child(ed)
	}
	rt := ui.NewRuntime(root, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	rt.SetSystemSettings(ui.SystemSettings{ReducedMotion: true})
	rt.Frame(time.Unix(1_000_000, 0))
	s := ed.view.session
	if s == nil {
		t.Fatal("the editor started no highlight session")
	}
	shown.Set(false)
	rt.Frame(time.Unix(1_000_000, 1))
	if err := s.SetViewport(0, 1); !errors.Is(err, highlight.ErrClosed) {
		t.Fatalf("session after removing the editor: SetViewport err = %v, want ErrClosed", err)
	}
	if ed.view.session != nil {
		t.Fatal("the removed editor still holds its session")
	}
}
