package ui

import (
	"testing"
	"time"
)

// followRig is a chat-like Bottom ListOf: a column of the list and a footer
// below it whose height is a signal (a composer that grows), with rows whose
// heights are signals read by the rows alone, so a row grows without the list
// rendering again.
type followRig struct {
	*overlayRig
	h      *ListHandle
	items  *Signal[[]int]
	footer *Signal[float32]
	rowH   map[int]*Signal[float32]
	ends   []bool
}

func newFollowRig(n int) *followRig {
	f := &followRig{h: NewListHandle(), items: NewSignal(ids(n)), footer: NewSignal[float32](40), rowH: map[int]*Signal[float32]{}}
	for _, id := range ids(n) {
		f.rowH[id] = NewSignal(vHeight(id))
	}
	f.overlayRig = newOverlayRig(func(ctx *Context) Element {
		list := ListOf(f.items.Get(), vKey, func(_ int, id int) Element {
			s := f.rowH[id]
			return Component(func(*Context) Element {
				return Div().Key(vKey(id)).Height(Px(s.Get())).Child(Text(vKey(id)))
			})
		}).Key("list").Style(Style{Grow: 1}).Bottom().Handle(f.h).OnAtEnd(func(e bool) { f.ends = append(f.ends, e) })
		return Div().Column().Height(Px(600)).
			Child(list).
			Child(Component(func(*Context) Element { return Div().Key("footer").Shrink(0).Height(Px(f.footer.Get())) }))
	}, 800, 700)
	f.h.ScrollToEnd()
	f.settle()
	return f
}

func (f *followRig) settle() {
	for range 120 {
		f.now = f.now.Add(16 * time.Millisecond)
		f.frame()
	}
}

func (f *followRig) add(id int) {
	f.rowH[id] = NewSignal(vHeight(id))
	f.items.Set(append(append([]int{}, f.items.Peek()...), id))
}

// lastAtEnd reports the last value OnAtEnd gave.
func (f *followRig) lastAtEnd() bool { return len(f.ends) > 0 && f.ends[len(f.ends)-1] }

// checkPinned fails unless the last row ends at the bottom of the list and the
// list says it is at its end.
func (f *followRig) checkPinned(t *testing.T, what string) {
	t.Helper()
	ids := f.items.Peek()
	last := f.find(t, vKey(ids[len(ids)-1]))
	list := f.find(t, "list")
	if d := last.Y + last.H - (list.Y + list.H); d > 1 || d < -1 {
		t.Fatalf("%s: last row ends %v px from the bottom of the list", what, d)
	}
	if !f.h.AtEnd() || !f.lastAtEnd() {
		t.Fatalf("%s: AtEnd %v, OnAtEnd reported %v", what, f.h.AtEnd(), f.ends)
	}
}

func TestListOfBottomFollowsAppendedRows(t *testing.T) {
	f := newFollowRig(60)
	f.checkPinned(t, "start")
	for id := 100; id < 140; id++ {
		f.add(id)
		f.now = f.now.Add(16 * time.Millisecond)
		f.frame()
		f.checkPinned(t, "after an append")
	}
	// Several at once, then settle.
	for id := 200; id < 230; id++ {
		f.add(id)
	}
	f.settle()
	f.checkPinned(t, "after a burst of appends")
	for i, e := range f.ends[1:] {
		if !e {
			t.Fatalf("OnAtEnd reported false at change %d: %v", i+1, f.ends)
		}
	}
}

func TestListOfBottomFollowsGrowingRows(t *testing.T) {
	f := newFollowRig(60)
	last := 59
	// The last row grows a little each frame, as streaming text does.
	for range 80 {
		f.rowH[last].Set(f.rowH[last].Peek() + 7)
		f.now = f.now.Add(16 * time.Millisecond)
		f.frame()
		f.checkPinned(t, "while the last row grows")
	}
	// A row above the last grows too: the end stays in view.
	f.rowH[57].Set(f.rowH[57].Peek() + 150)
	f.now = f.now.Add(16 * time.Millisecond)
	f.frame()
	f.checkPinned(t, "after a row above the last grew")
	// A row shrinks.
	f.rowH[last].Set(30)
	f.now = f.now.Add(16 * time.Millisecond)
	f.frame()
	f.checkPinned(t, "after the last row shrank")
	for i, e := range f.ends[1:] {
		if !e {
			t.Fatalf("OnAtEnd reported false at change %d: %v", i+1, f.ends)
		}
	}
}

func TestListOfBottomFollowsViewportResize(t *testing.T) {
	f := newFollowRig(60)
	for _, fh := range []float32{120, 200, 40, 300, 10} {
		f.footer.Set(fh)
		f.now = f.now.Add(16 * time.Millisecond)
		f.frame()
		f.checkPinned(t, "after the viewport changed height")
	}
	// The footer and the rows change together.
	f.add(100)
	f.footer.Set(150)
	f.rowH[59].Set(90)
	f.now = f.now.Add(16 * time.Millisecond)
	f.frame()
	f.checkPinned(t, "after rows and the viewport changed together")
	for i, e := range f.ends[1:] {
		if !e {
			t.Fatalf("OnAtEnd reported false at change %d: %v", i+1, f.ends)
		}
	}
}

func TestListOfBottomScrolledUpDoesNotFollow(t *testing.T) {
	f := newFollowRig(60)
	f.rt.Scroll(100, 100, 0, -200, true, 0)
	f.settle()
	if f.h.AtEnd() || f.lastAtEnd() {
		t.Fatalf("at the end after scrolling up: %v", f.ends)
	}
	before := f.find(t, vKey(50)).Y
	f.add(100)
	f.rowH[59].Set(200)
	f.footer.Set(100)
	f.settle()
	if got := f.find(t, vKey(50)).Y; got != before {
		t.Fatalf("row moved from %v to %v while scrolled up", before, got)
	}
	if f.h.AtEnd() || f.lastAtEnd() {
		t.Fatalf("at the end after new content while scrolled up: %v", f.ends)
	}
	// ScrollToEnd with an append in the same frame lands on the new end.
	f.add(101)
	f.h.ScrollToEnd()
	f.settle()
	f.checkPinned(t, "after ScrollToEnd")
}
