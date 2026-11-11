package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// selectWatchRig is a selectable text in a scroll container, inside an element
// that records what its OnTextSelect reports, and a button below them.
func selectWatchRig(t *testing.T, got *[]TextSelection) *editorRig {
	t.Helper()
	return newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Key("watch").PaddingTop(20).OnTextSelect(func(s TextSelection) { *got = append(*got, s) }).
				Child(Div().Key("scroller").Height(Px(60)).ScrollY().
					Child(Text("Error 404: page not found").Selectable().Key("sel")).
					Child(Div().Height(Px(400))))).
			Child(Button("Other").Key("other"))
	})
}

func lastSel(t *testing.T, got []TextSelection) TextSelection {
	t.Helper()
	if len(got) == 0 {
		t.Fatal("OnTextSelect was not called")
	}
	return got[len(got)-1]
}

// OnTextSelect reports the selected text and where it is drawn, relative to
// the element that watches, and reports nothing once it is cleared.
func TestOnTextSelectReportsSelection(t *testing.T) {
	var got []TextSelection
	r := selectWatchRig(t, &got)
	if len(got) != 0 {
		t.Fatalf("reported %v before anything was selected", got)
	}
	x0, y := r.at(t, len("Error "))
	x1, _ := r.at(t, len("Error 404"))
	r.drag(x0, y, x1, y)
	s := lastSel(t, got)
	if s.Text != "404" {
		t.Fatalf("selected %q, want 404", s.Text)
	}
	w := r.rt.byID[r.rt.keyID(t, "watch")]
	if s.Bounds.W <= 0 || s.Bounds.H <= 0 || s.Last != s.Bounds {
		t.Fatalf("bounds %+v, last %+v: want one non-empty line", s.Bounds, s.Last)
	}
	if ax := w.abs.X + s.Bounds.X; ax < x0-1 || ax > x0+1 {
		t.Errorf("selection starts at x %v in the window, want %v", ax, x0)
	}
	if s.Bounds.Y < 20 {
		t.Errorf("bounds %+v are not relative to the watching element (below its 20 px padding)", s.Bounds)
	}

	// The same selection, painted again, is not reported again.
	n := len(got)
	r.rt.needPaint = true
	frame(r.rt)
	if len(got) != n {
		t.Errorf("an unchanged selection was reported again: %v", got[n:])
	}

	// Extending it reports the new text.
	x2, _ := r.at(t, len("Error 404: page"))
	r.rt.PointerDown(x2, y, input.ButtonLeft, 1, input.ModShift)
	r.rt.PointerUp(x2, y, input.ButtonLeft, input.ModShift)
	frame(r.rt)
	if s := lastSel(t, got); s.Text != "404: page" {
		t.Errorf("after Shift+click selected %q", s.Text)
	}
}

// Clearing the selection, moving focus away and scrolling the text out of view
// each report no selection.
func TestOnTextSelectReportsClearing(t *testing.T) {
	cases := []struct {
		name  string
		clear func(r *editorRig)
	}{
		{"ClearTextSelection", func(r *editorRig) {
			r.rt.clearTextSelection()
			frame(r.rt)
		}},
		{"focus moves", func(r *editorRig) {
			r.rt.ClickKey("other")
			frame(r.rt)
		}},
		{"scrolled away", func(r *editorRig) {
			sc, _ := r.rt.Find("scroller")
			r.rt.Scroll(sc.X+5, sc.Y+5, 0, 200, true, 0)
			frame(r.rt)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []TextSelection
			r := selectWatchRig(t, &got)
			x0, y := r.at(t, 0)
			x1, _ := r.at(t, len("Error"))
			r.drag(x0, y, x1, y)
			if s := lastSel(t, got); s.Text != "Error" {
				t.Fatalf("selected %q", s.Text)
			}
			c.clear(r)
			if s := lastSel(t, got); s != (TextSelection{}) {
				t.Errorf("reported %+v, want no selection", s)
			}
		})
	}
}

// Scrolling a selection within view reports where it moved to.
func TestOnTextSelectFollowsScroll(t *testing.T) {
	var got []TextSelection
	r := selectWatchRig(t, &got)
	x0, y := r.at(t, 0)
	x1, _ := r.at(t, len("Error"))
	r.drag(x0, y, x1, y)
	before := lastSel(t, got)
	sc, _ := r.rt.Find("scroller")
	r.rt.Scroll(sc.X+5, sc.Y+5, 0, 5, true, 0)
	frame(r.rt)
	after := lastSel(t, got)
	if after.Text != "Error" || after.Bounds.Y >= before.Bounds.Y {
		t.Errorf("after scrolling down 5 px: %+v, before %+v", after, before)
	}
}

// What OnTextSelect changes is drawn in the frame that reports the selection:
// a button placed by it appears with the selection and goes with it.
func TestOnTextSelectBuildsInSameFrame(t *testing.T) {
	r := newEditorRig(t, func(ctx *Context) Element {
		sel := Local(ctx, "sel", TextSelection{})
		s := sel.Get()
		return Div().Padding(10).OnTextSelect(sel.Set).
			Child(Text("Error 404: page not found").Selectable().Key("sel")).
			Child(If(s.Text != "", Div().Key("quote").Absolute().Left(s.Last.Right()).Top(s.Last.Y).Size(Px(20), Px(20))))
	})
	x0, y := r.at(t, 0)
	x1, _ := r.at(t, len("Error"))
	r.rt.PointerMove(x0, y, 0)
	r.rt.PointerDown(x0, y, input.ButtonLeft, 1, 0)
	r.rt.PointerMove(x1, y, 0)
	r.rt.PointerUp(x1, y, input.ButtonLeft, 0)
	r.rt.Frame(r.now) // one frame
	b, ok := r.rt.Find("quote")
	if !ok {
		t.Fatal("the element placed by the selection was not drawn in the frame that selected")
	}
	if b.X < x1-1 || b.X > x1+1 {
		t.Errorf("placed at x %v, want the end of the selection, %v", b.X, x1)
	}
	r.rt.clearTextSelection()
	r.rt.Frame(r.now)
	if _, ok := r.rt.Find("quote"); ok {
		t.Error("the element stayed after the selection was cleared")
	}
}
