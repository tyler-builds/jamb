package codeedit

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// TestVersionIsTrackedRead checks that a component which reads Version while it renders, and
// subscribes to nothing, renders again after each kind of change: typing in the editor, a
// programmatic edit, undo, redo, SetText and a selection change. A read outside a render
// subscribes nothing.
func TestVersionIsTrackedRead(t *testing.T) {
	doc := NewDocument("one two")
	var shown uint64
	renders := 0
	status := ui.Component(func(*ui.Context) ui.Element {
		renders++
		shown = doc.Version()
		return ui.Text("modified")
	})
	ed := Editor(doc).Label("Source").Size(ui.Px(480), ui.Px(300))
	root := func(*ui.Context) ui.Element { return ui.Div().Child(status).Child(ui.Div().Key("wrap").Child(ed)) }
	rt := ui.NewRuntime(root, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	rt.SetSystemSettings(ui.SystemSettings{ReducedMotion: true})
	h := &harness{t: t, rt: rt, ed: ed, doc: doc, now: time.Unix(1_000_000, 0)}
	h.frame()
	r, ok := rt.Find("wrap")
	if !ok {
		t.Fatal("the editor is not in the tree")
	}
	h.click(r.X+100, r.Y+40, 0, 1)

	steps := []struct {
		name string
		do   func()
	}{
		{"typing", func() { h.typeText("x") }},
		{"Replace", func() {
			if err := doc.Replace(0, 3, "ONE"); err != nil {
				t.Fatal(err)
			}
		}},
		{"Undo", func() { doc.Undo() }},
		{"Redo", func() { doc.Redo() }},
		{"SetText", func() { doc.SetText("fresh") }},
		{"SetSelections", func() { doc.SetSelections(Selection{Anchor: 0, Head: 2}) }},
	}
	for _, s := range steps {
		before := doc.Version()
		s.do()
		h.frame()
		if doc.Version() == before {
			t.Fatalf("%s did not change the version", s.name)
		}
		if shown != doc.Version() {
			t.Fatalf("after %s the component shows version %d, want %d", s.name, shown, doc.Version())
		}
	}

	// With no change, the component is not rendered again.
	n := renders
	h.frame()
	h.frame()
	if renders != n {
		t.Fatalf("the component rendered %d more times without a change", renders-n)
	}
}

// TestVersionOutsideRenderDoesNotSubscribe checks that Version read outside a render is a plain
// read: it returns the version, and no render follows a change.
func TestVersionOutsideRenderDoesNotSubscribe(t *testing.T) {
	doc := NewDocument("a")
	renders := 0
	rt := ui.NewRuntime(func(*ui.Context) ui.Element { renders++; return ui.Text("x") }, ui.DarkTheme(), nil)
	rt.SetViewport(200, 100, 1)
	rt.Frame(time.Unix(1_000_000, 0))
	v := doc.Version()
	if err := doc.Replace(0, 1, "b"); err != nil {
		t.Fatal(err)
	}
	if doc.Version() == v {
		t.Fatal("Replace did not change the version")
	}
	n := renders
	rt.Frame(time.Unix(1_000_001, 0))
	if renders != n || rt.NeedsFrame() {
		t.Fatalf("a change after a read outside a render rendered the root (%d renders, needs frame %v)", renders-n, rt.NeedsFrame())
	}
}
