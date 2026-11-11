package ui

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/scene"
)

// drawnGlyphs returns where the last frame of rt drew glyphs, and in which color,
// leaving out where in an atlas each came from, which depends on what the text
// system drew before.
func drawnGlyphs(rt *Runtime) []scene.Glyph {
	out := make([]scene.Glyph, len(rt.scene.Glyphs))
	for i, g := range rt.scene.Glyphs {
		out[i] = scene.Glyph{Bounds: g.Bounds, Color: g.Color}
	}
	return out
}

// textPaints returns the paintedTexts of the text nodes of rt's tree.
func textPaints(rt *Runtime) map[uint64]*paintedText {
	out := map[uint64]*paintedText{}
	forNodes(rt.tree, func(n *node) {
		if n.textEl != nil && n.textPaint != nil {
			out[n.id] = n.textPaint
		}
	})
	return out
}

// The texts of a view that is rebuilt, scrolled and changed draw what a new
// runtime draws for the same view, though they draw from what their earlier paints
// kept, and a text node rebuilt in place keeps it.
func TestTextPaintMatchesFreshRuntime(t *testing.T) {
	type state struct {
		top    float32
		rows   []string
		accent bool
		n      int // a digit at the end of each text, which changes it in place
	}
	view := func(s *Signal[state]) func(*Context) Element {
		return func(ctx *Context) Element {
			st := s.Get()
			list := Div().Key("list").Column().MarginTop(st.top)
			for _, r := range st.rows {
				label := Textf("%s %d", r, st.n).Key("t-" + r)
				if st.accent && r == "beta" {
					label = Rich(Seg("be"), Seg(fmt.Sprintf("ta %d", st.n)).Bold()).Key("t-" + r)
				}
				list.Child(Div().Key("row-" + r).PaddingLeft(3.5).Child(label))
			}
			return list
		}
	}
	rows := []string{"alpha", "beta", "gamma", "delta"}
	later := []string{"gamma", "delta", "epsilon", "zeta"}
	steps := []state{
		{0, rows, false, 0},
		{0, rows, false, 0},
		{-7.5, rows, false, 0}, // scrolled
		{-15, rows, false, 0},
		{-15, []string{"alpha", "BETA", "gamma", "delta"}, false, 0}, // a row changes
		{-15, rows, false, 1}, // the texts change in place, keeping their widths
		{-15, rows, true, 1},  // rich text, in place
		{-15, rows, true, 2},
		{-22.5, rows, true, 2},  // scrolled
		{-22.5, later, true, 2}, // rows go and come
		{-30, later, true, 2},
		{-30, append(later, "eta"), false, 3},
	}
	s := NewSignal(steps[0])
	rt := NewRuntime(view(s), DarkTheme(), nil)
	rt.SetViewport(300, 200, 1.25)
	now := time.Unix(1_000_000, 0)
	for i, st := range steps {
		s.Set(st)
		for range 3 { // three frames, so that the cache records and is drawn from
			now = now.Add(16 * time.Millisecond)
			rt.Frame(now)
		}
		fresh := newTestRuntime(view(NewSignal(st)), 300, 200, 1.25)
		got, want := drawnGlyphs(rt), drawnGlyphs(fresh)
		if len(want) == 0 {
			t.Fatalf("step %d drew no glyphs", i)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("step %d: the glyphs differ from a new runtime's:\n got %v\nwant %v", i, got, want)
		}
	}
}

// A text node rebuilt in place keeps what its last paint drew, and a retired one
// gives it to a new text node.
func TestTextPaintKeptAndRecycled(t *testing.T) {
	rows := NewSignal([]string{"a", "b", "c"})
	tick := NewSignal(0)
	rt := newTestRuntime(func(ctx *Context) Element {
		tick.Get()
		list := Div().Column()
		for _, r := range rows.Get() {
			list.Child(Text("row " + r).Key("t-" + r))
		}
		return list
	}, 300, 200, 1)
	rt.Frame(time.Now())
	before := textPaints(rt)
	if len(before) != 3 {
		t.Fatalf("%d texts painted, want 3", len(before))
	}
	tick.Set(1) // rebuilds every node in place
	rt.Frame(time.Now())
	if after := textPaints(rt); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("a rebuild in place dropped what the texts painted: %v, then %v", before, after)
	}
	rows.Set([]string{"a"})
	rt.Frame(time.Now())
	if len(rt.textPaints) != 2 {
		t.Fatalf("%d paintedTexts kept from 2 retired texts", len(rt.textPaints))
	}
	kept := map[*paintedText]bool{}
	for _, p := range rt.textPaints {
		kept[p] = true
		if p.text != "" || p.ref.Layout() != nil {
			t.Errorf("a kept paintedText was not emptied: %q", p.text)
		}
	}
	rows.Set([]string{"a", "x", "y"})
	rt.Frame(time.Now())
	for id, p := range textPaints(rt) {
		if p != before[id] && !kept[p] {
			t.Errorf("a new text made a paintedText while %d were kept", len(kept))
		}
	}
}
