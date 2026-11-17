package ui

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// upHeight is the height of a row of upRig: from 20 to 420, so that the mean of
// the measured rows, which estimates the rest, is far from most rows' own height.
func upHeight(id int) float32 { return 20 + float32(id*37%11)*40 }

// upRig is a 400-item ListOf whose rows' heights differ widely from the
// estimate, scrolled to its end, as a chat opens. A bottom rig is a Bottom
// list, as a chat is. Rows are as tall as upHeight plus their tall entry, and
// tick renders the list again.
func upRig(t *testing.T, bottom bool) *vRig {
	t.Helper()
	h := NewListHandle()
	v := &vRig{items: NewSignal(ids(400)), tick: NewSignal(0), tall: map[int]float32{}}
	v.overlayRig = newOverlayRig(func(ctx *Context) Element {
		_ = v.tick.Get()
		l := ListOf(v.items.Get(), vKey, func(_ int, id int) Element {
			return Div().Key(vKey(id)).Height(Px(upHeight(id) + v.tall[id])).Child(Text(vKey(id)))
		}).Key("list").Style(Style{Height: Px(400)}).Handle(h)
		if bottom {
			l = l.Bottom()
		}
		return Div().Child(l)
	}, 800, 600)
	h.ScrollToEnd()
	for range 60 {
		v.now = v.now.Add(16 * time.Millisecond)
		v.frame()
	}
	if c := v.container(); c.st.targetY < c.lay.ContentH-c.lay.H-1 {
		t.Fatal("list did not scroll to its end")
	}
	return v
}

// moved checks that every row on screen in both prev and cur moved down by
// want. Rows outside the viewport may be laid out at estimated places, so only
// rows the user can see count: a pixel of them or more. Each frame is drawn
// at a scroll offset rounded to whole pixels, so a row may be a pixel off.
func (v *vRig) moved(t *testing.T, what string, prev, cur map[int]float32, want float32) {
	t.Helper()
	list := v.find(t, "list")
	seen := false
	for id, y := range cur {
		p, ok := prev[id]
		on := func(y float32) bool { return y+upHeight(id)+v.tall[id] > list.Y+1 && y < list.Y+list.H-1 }
		if !ok || !on(y) || !on(p) {
			continue
		}
		seen = true
		if math.Abs(float64(y-p-want)) > 1 {
			t.Fatalf("%s: item %d on screen moved %v, want %v: the view jumped as rows above it were measured", what, id, y-p, want)
		}
	}
	if !seen {
		t.Fatalf("%s: no row stayed on screen", what)
	}
}

// TestListOfScrollUpKeepsRowsSteady scrolls a list up from its end in small
// precise steps, as a touchpad or a finger and its fling do. Each step brings
// rows into view whose height is not known yet; measuring them must not move
// the rows already on screen by more than the step. So too when the list is
// rendered again on each step and items are added at its end, as a chat's are
// while a reply streams in, and when rows above the view grow after they are
// first laid out, as a row does when its image is decoded.
func TestListOfScrollUpKeepsRowsSteady(t *testing.T) {
	for _, tc := range []struct {
		name           string
		bottom, stream bool
		grow           bool
	}{
		{name: "plain"},
		{name: "chat", bottom: true, stream: true},
		{name: "late growth", bottom: true, grow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := upRig(t, tc.bottom)
			prev := v.rowY(t, 1000)
			for step := 0; v.container().st.targetY > 200; step++ {
				v.rt.Scroll(100, 100, 0, -40, true, 0)
				if tc.stream {
					v.tick.Set(v.tick.Peek() + 1)
					if step%5 == 0 {
						v.items.Set(append(v.items.Peek(), 400+step/5))
					}
				}
				v.now = v.now.Add(16 * time.Millisecond)
				v.frame()
				cur := v.rowY(t, 1000)
				v.moved(t, "step "+strconv.Itoa(step), prev, cur, 40)
				prev = cur
				if tc.grow {
					// Rows that have been built above the view grow by 60.
					list := v.find(t, "list")
					for id, y := range cur {
						if _, ok := v.tall[id]; !ok && y+upHeight(id) <= list.Y {
							v.tall[id] = 60
							v.tick.Set(v.tick.Peek() + 1)
						}
					}
					v.frame()
					cur = v.rowY(t, 1000)
					v.moved(t, "growth at step "+strconv.Itoa(step), prev, cur, 0)
					prev = cur
				}
				if step > 10_000 {
					t.Fatal("never reached the top")
				}
			}
		})
	}
}

// TestListOfWheelUpKeepsRowsSteady scrolls a list up from its end with wheel
// notches, which ease toward their target over several frames. The rows on
// screen must end where the notches put them, however many rows between the
// view and the target were measured on the way.
func TestListOfWheelUpKeepsRowsSteady(t *testing.T) {
	v := upRig(t, true)
	for notch := 0; v.container().st.targetY > 1000; notch++ {
		prev := v.rowY(t, 400)
		// Three notches in quick succession, then the ease runs out.
		for range 3 {
			v.rt.Scroll(100, 100, 0, -100, false, 0)
			v.now = v.now.Add(16 * time.Millisecond)
			v.frame()
		}
		for i := range 300 {
			last := v.rowY(t, 400)
			v.now = v.now.Add(16 * time.Millisecond)
			v.frame()
			cur := v.rowY(t, 400)
			// Each frame of the ease moves the rows on screen down, never up.
			for id, y := range cur {
				if p, ok := last[id]; ok && y < p-0.5 && i > 0 {
					t.Fatalf("notch %d frame %d: item %d moved up %v while scrolling up", notch, i, id, p-y)
				}
			}
			if c := v.container(); c.st.scrollY == c.st.targetY {
				break
			}
		}
		v.moved(t, "notch "+strconv.Itoa(notch), prev, v.rowY(t, 400), 300)
		if notch > 500 {
			t.Fatal("never reached the top")
		}
	}
}

// TestListOfThumbDragUpKeepsRowsSteady drags the scrollbar's thumb up a list
// from its end. Each move scrolls by as much as the pointer moved, at the
// scale of the track; the corrections the list makes as it measures rows
// above the view must not be undone by the next move, which would jump.
func TestListOfThumbDragUpKeepsRowsSteady(t *testing.T) {
	v := upRig(t, true)
	sbs := v.rt.scrollbars(v.container(), nil)
	if len(sbs) == 0 {
		t.Fatal("no scrollbar")
	}
	x, y := sbs[0].thumb.X+sbs[0].thumb.W/2, sbs[0].thumb.Y+sbs[0].thumb.H/2
	v.rt.PointerMove(x, y, 0)
	v.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	prev := v.rowY(t, 1000)
	for step := range 300 {
		sb := v.rt.scrollbars(v.container(), nil)[0]
		want := 0.2 / (sb.track.H - sb.thumb.H) * sb.max
		y -= 0.2
		v.rt.PointerMove(x, y, 0)
		v.now = v.now.Add(16 * time.Millisecond)
		v.frame()
		cur := v.rowY(t, 1000)
		v.moved(t, "move "+strconv.Itoa(step), prev, cur, want)
		prev = cur
	}
	v.rt.PointerUp(x, y, input.ButtonLeft, 0)
}
