package text

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tyler-builds/jamb/internal/scene"
)

// drawStep is one draw of a text at a place, with the paint of its runs.
type drawStep struct {
	l              *Layout
	x, y, box, scl float32
	align          int
	paint          func(span int) RunPaint
}

// checkCachedDraws draws each step twice, with DrawRich and with DrawRichCached
// through one DrawCache, and fails when the two scenes differ. It returns how many
// of the cached draws took their glyphs from the cache.
func checkCachedDraws(t *testing.T, sys *System, steps []drawStep) (hits int) {
	t.Helper()
	var c DrawCache
	var warm, plain, cached scene.Scene
	for i, s := range steps {
		// A draw that rasterizes can grow an atlas partway, and then its first
		// glyphs name the texture before the growth: rasterize first.
		warm.Reset()
		sys.DrawRich(&warm, s.l, s.x, s.y, s.box, s.scl, s.align, s.paint)
		plain.Reset()
		cached.Reset()
		sys.DrawRich(&plain, s.l, s.x, s.y, s.box, s.scl, s.align, s.paint)
		hit := c.valid && c.l == s.l && c.x == s.x && c.scale == s.scl && c.align == s.align &&
			(s.align == 0 || c.boxWidth == s.box) && c.gens == sys.gens()
		sys.DrawRichCached(&cached, s.l, &c, s.x, s.y, s.box, s.scl, s.align, s.paint)
		if hit {
			hits++
		}
		if len(plain.Glyphs)+len(plain.Images) == 0 {
			t.Fatalf("step %d drew no glyphs", i)
		}
		for _, d := range []struct {
			name      string
			got, want any
		}{
			{"ops", cached.Ops, plain.Ops},
			{"quads", cached.Quads, plain.Quads},
			{"glyphs", cached.Glyphs, plain.Glyphs},
			{"images", cached.Images, plain.Images},
		} {
			if !reflect.DeepEqual(d.got, d.want) {
				t.Fatalf("step %d (hit %v): cached %s differ from plain:\n got %v\nwant %v", i, hit, d.name, d.got, d.want)
			}
		}
	}
	return hits
}

func runColor(c scene.Color) func(int) RunPaint {
	return func(int) RunPaint { return RunPaint{Color: c} }
}

var (
	dcWhite = scene.Color{R: 1, G: 1, B: 1, A: 1}
	dcRed   = scene.Color{R: 1, A: 1}
)

// A text drawn again where it was, as in each frame of a vertical scroll, draws
// from the cache and draws what an uncached draw does, at each scale, alignment
// and fraction of a pixel.
func TestDrawCacheMatchesStill(t *testing.T) {
	sys := NewSystem()
	l := sys.Layout("The quick brown fox jumps over the lazy dog, twice: fi fl ffi.", Style{Size: 15}, 180)
	if len(l.Lines) < 2 {
		t.Fatalf("want a wrapped text, got %d lines", len(l.Lines))
	}
	for _, scl := range []float32{1, 1.25, 1.5, 2} {
		for _, x := range []float32{0, 10.3, 7.77} {
			for align := range 3 {
				t.Run(fmt.Sprintf("scale%v/x%v/align%d", scl, x, align), func(t *testing.T) {
					var steps []drawStep
					for f := range 6 {
						// The top moves each frame, by whole and by fractional pixels.
						steps = append(steps, drawStep{l, x, 20 + float32(f)*13.4, 240, scl, align, runColor(dcWhite)})
					}
					if hits := checkCachedDraws(t, sys, steps); hits < 3 {
						t.Errorf("%d draws from the cache, want at least 3", hits)
					}
				})
			}
		}
	}
}

// Whatever the draw before was, a cached draw draws what an uncached one does:
// when the text moves sideways, changes, changes scale, box or alignment, or
// changes color.
func TestDrawCacheMatchesChanges(t *testing.T) {
	sys := NewSystem()
	a := sys.Layout("Item 42 — total 1,337.50", Style{Size: 14}, inf)
	b := sys.Layout("Item 43 — total 9,001.25", Style{Size: 14}, inf)
	big := sys.Layout("Item 42 — total 1,337.50", Style{Size: 22}, inf)
	steps := []drawStep{
		{a, 5, 5, 0, 1, 0, runColor(dcWhite)},
		{a, 5, 6, 0, 1, 0, runColor(dcWhite)},
		{a, 5, 7, 0, 1, 0, runColor(dcWhite)}, // from the cache
		{a, 5.5, 7, 0, 1, 0, runColor(dcWhite)},
		{a, 6.25, 7, 0, 1, 0, runColor(dcWhite)},
		{a, 6.25, 8, 0, 1, 0, runColor(dcWhite)},
		{a, 6.25, 9, 0, 1, 0, runColor(dcWhite)},
		{b, 6.25, 9, 0, 1, 0, runColor(dcWhite)}, // another layout at the same place
		{b, 6.25, 9, 0, 1, 0, runColor(dcWhite)},
		{b, 6.25, 9, 0, 1, 0, runColor(dcRed)}, // another color
		{b, 6.25, 9, 0, 1.5, 0, runColor(dcRed)},
		{b, 6.25, 9, 0, 1.5, 0, runColor(dcRed)},
		{b, 6.25, 9, 0, 1.5, 0, runColor(dcRed)},
		{b, 6.25, 9, 300, 1.5, 2, runColor(dcRed)}, // aligned right in a box
		{b, 6.25, 9, 300, 1.5, 2, runColor(dcRed)},
		{b, 6.25, 9, 300, 1.5, 2, runColor(dcRed)},
		{b, 6.25, 9, 301.3, 1.5, 2, runColor(dcRed)}, // a wider box
		{b, 6.25, 9, 301.3, 1.5, 1, runColor(dcRed)}, // centered
		{b, 6.25, 9, 301.3, 1.5, 1, runColor(dcRed)},
		{b, 6.25, 9, 301.3, 1.5, 1, runColor(dcRed)},
		{big, 6.25, 9, 301.3, 1.5, 1, runColor(dcRed)},
		{big, 6.25, 9, 301.3, 1.5, 1, runColor(dcRed)},
		{big, 6.25, 9, 301.3, 1.5, 1, runColor(dcRed)},
	}
	if hits := checkCachedDraws(t, sys, steps); hits < 5 {
		t.Errorf("%d draws from the cache, want at least 5", hits)
	}
}

// Rich text keeps the paint of each run: a run whose color changes looks its
// glyphs up, and backgrounds, underlines and strikethroughs are drawn as they
// are without the cache.
func TestDrawCacheMatchesRich(t *testing.T) {
	sys := NewSystem()
	s := "plain bold italic link and the rest"
	spans := []Span{{Start: 6, End: 10, Weight: Bold}, {Start: 11, End: 17, Italic: true}, {Start: 18, End: 22}}
	l := sys.LayoutSpans(s, Style{Size: 16}, spans, 150)
	blue := scene.Color{B: 1, A: 1}
	paint := func(link scene.Color, under bool) func(int) RunPaint {
		return func(span int) RunPaint {
			switch span {
			case 0:
				return RunPaint{Color: dcRed, Background: scene.Color{G: 0.2, A: 0.5}}
			case 1:
				return RunPaint{Color: dcWhite, Strike: true}
			case 2:
				return RunPaint{Color: link, Underline: under, DecorationColor: blue}
			}
			return RunPaint{Color: dcWhite}
		}
	}
	steps := []drawStep{
		{l, 3.4, 0, 150, 1.25, 0, paint(blue, false)},
		{l, 3.4, 1, 150, 1.25, 0, paint(blue, false)},
		{l, 3.4, 2, 150, 1.25, 0, paint(blue, false)},
		{l, 3.4, 3, 150, 1.25, 0, paint(dcRed, true)}, // the link is hovered
		{l, 3.4, 4, 150, 1.25, 0, paint(dcRed, true)},
		{l, 3.4, 5, 150, 1.25, 0, paint(blue, false)},
		{l, 3.4, 5, 150, 1.25, 0, paint(blue, false)},
	}
	if hits := checkCachedDraws(t, sys, steps); hits < 4 {
		t.Errorf("%d draws from the cache, want at least 4", hits)
	}
}

// Clearing an atlas moves its glyphs: a cache recorded before the clear is not
// used after it.
func TestDrawCacheAtlasCleared(t *testing.T) {
	sys := NewSystem()
	l := sys.Layout("Cleared atlas, drawn again", Style{Size: 18}, inf)
	var c DrawCache
	var sc scene.Scene
	for range 2 {
		sc.Reset()
		sys.DrawCached(&sc, l, &c, 4, 4, 0, 1, 0, dcWhite)
	}
	if !c.valid {
		t.Fatal("the second draw at the same place recorded nothing")
	}
	// Fill the atlas with other glyphs at other places, then clear it.
	other := sys.Layout("ABCDEFGHIJKLMNOPQRSTUVWXYZ abcdefghijklmnopqrstuvwxyz 0123456789", Style{Size: 31}, inf)
	sys.Draw(&sc, other, 0.5, 0, 0, 1, 0, dcWhite)
	sys.atlas.clear()
	if sys.sub != nil {
		sys.sub.clear()
	}
	sys.Draw(&sc, other, 0.5, 0, 0, 1, 0, dcWhite) // other glyphs take the cleared places first
	steps := []drawStep{
		{l, 4, 9, 0, 1, 0, runColor(dcWhite)},
		{l, 4, 10, 0, 1, 0, runColor(dcWhite)},
	}
	var plain, cached scene.Scene
	for _, s := range steps {
		plain.Reset()
		sys.DrawRich(&plain, s.l, s.x, s.y, s.box, s.scl, s.align, s.paint) // rasterize (see checkCachedDraws)
		plain.Reset()
		cached.Reset()
		sys.DrawRich(&plain, s.l, s.x, s.y, s.box, s.scl, s.align, s.paint)
		sys.DrawRichCached(&cached, s.l, &c, s.x, s.y, s.box, s.scl, s.align, s.paint)
		if !reflect.DeepEqual(cached.Glyphs, plain.Glyphs) {
			t.Fatalf("after the clear, cached glyphs differ:\n got %v\nwant %v", cached.Glyphs, plain.Glyphs)
		}
	}
}

// Text drawn once, or moving sideways each frame, records nothing.
func TestDrawCacheRecordsOnlyStillText(t *testing.T) {
	sys := NewSystem()
	l := sys.Layout("Sideways", Style{Size: 14}, inf)
	var c DrawCache
	var sc scene.Scene
	for f := range 5 {
		sc.Reset()
		sys.DrawCached(&sc, l, &c, float32(f), 0, 0, 1, 0, dcWhite)
		if c.valid || len(c.entries) != 0 {
			t.Fatalf("frame %d: a text moving sideways recorded %d entries", f, len(c.entries))
		}
	}
}

// A drawn text records its glyphs once, and a draw from the cache allocates
// nothing.
func TestDrawCacheAllocs(t *testing.T) {
	sys := NewSystem()
	l := sys.Layout("Allocations, counted: one per buffer at most", Style{Size: 14}, 200)
	var c DrawCache
	var sc scene.Scene
	sys.DrawCached(&sc, l, &c, 2, 2, 0, 1, 0, dcWhite)
	sc.Reset()
	sys.DrawCached(&sc, l, &c, 2, 2, 0, 1, 0, dcWhite)
	if !c.valid {
		t.Fatal("not recorded")
	}
	if a := testing.AllocsPerRun(20, func() {
		sc.Reset()
		sys.DrawCached(&sc, l, &c, 2, 3, 0, 1, 0, dcWhite)
	}); a != 0 {
		t.Errorf("a draw from the cache allocates %v times, want 0", a)
	}
	// Recording again, into buffers already big enough, allocates nothing.
	var c2 DrawCache
	sys.DrawCached(&sc, l, &c2, 2, 2, 0, 1, 0, dcWhite)
	if a := testing.AllocsPerRun(20, func() {
		c2.valid = false
		sc.Reset()
		sys.DrawCached(&sc, l, &c2, 2, 2, 0, 1, 0, dcWhite)
	}); a != 0 {
		t.Errorf("recording into big enough buffers allocates %v times, want 0", a)
	}
	// Recording into empty buffers allocates once for each.
	if a := testing.AllocsPerRun(20, func() {
		c3 := DrawCache{lastL: l, lastX: 2, lastScale: 1}
		sc.Reset()
		sys.DrawCached(&sc, l, &c3, 2, 2, 0, 1, 0, dcWhite)
	}); a > 2 {
		t.Errorf("recording into empty buffers allocates %v times, want at most 2", a)
	}
}

// Reset empties a cache, which then draws as a new one does.
func TestDrawCacheReset(t *testing.T) {
	sys := NewSystem()
	l := sys.Layout("Reset", Style{Size: 14}, inf)
	var c DrawCache
	var sc scene.Scene
	for range 2 {
		sys.DrawCached(&sc, l, &c, 1, 1, 0, 1, 0, dcWhite)
	}
	c.Reset()
	if c.valid || c.l != nil || c.lastL != nil || len(c.entries) != 0 || len(c.colors) != 0 {
		t.Fatalf("Reset left %+v", c)
	}
}

// Reuse returns the layout of a LayoutRef until the text system evicts it.
func TestLayoutRefReuse(t *testing.T) {
	sys := NewSystem()
	st := Style{Size: 14}
	r := sys.LayoutRef("Reused while used, then evicted after a while", st, nil, 120)
	if r.Layout() != sys.LayoutSpans("Reused while used, then evicted after a while", st, nil, 120) {
		t.Fatal("LayoutRef and LayoutSpans return different layouts")
	}
	if r.narrow == nil {
		t.Fatal("want a wrapped layout, which uses the unconstrained one too")
	}
	for range 300 {
		if l, ok := sys.Reuse(r); !ok || l != r.Layout() {
			t.Fatal("Reuse lost a layout in use")
		}
		sys.EndFrame()
	}
	for range 300 {
		sys.EndFrame()
	}
	if _, ok := sys.Reuse(r); ok {
		t.Error("Reuse returned an evicted layout")
	}
	if _, ok := sys.Reuse(LayoutRef{}); ok {
		t.Error("Reuse of the zero LayoutRef reported a layout")
	}
}

// A color glyph's atlas entry depends on the text color: a run whose color
// changes looks its glyphs up again rather than drawing the recorded entries.
func TestDrawCacheColorGlyphs(t *testing.T) {
	sys := NewSystem()
	l := sys.Layout("ok 🦀 🎉 done", Style{Size: 20}, inf)
	color := false
	for _, ln := range l.Lines {
		for _, run := range ln.Runs {
			for _, g := range run.Glyphs {
				color = color || g.font != nil && g.font.hasColor()
			}
		}
	}
	if !color {
		t.Skip("no color emoji font on this system")
	}
	blue := scene.Color{B: 1, A: 1}
	steps := []drawStep{
		{l, 2, 2, 0, 1, 0, runColor(dcWhite)},
		{l, 2, 3, 0, 1, 0, runColor(dcWhite)},
		{l, 2, 4, 0, 1, 0, runColor(dcWhite)},
		{l, 2, 5, 0, 1, 0, runColor(blue)},
		{l, 2, 6, 0, 1, 0, runColor(scene.Color{B: 0.5, A: 0.5})},
		{l, 2, 7, 0, 1, 0, runColor(dcWhite)},
	}
	if hits := checkCachedDraws(t, sys, steps); hits < 4 {
		t.Errorf("%d draws from the cache, want at least 4", hits)
	}
}
