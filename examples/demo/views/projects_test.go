package views

import (
	"fmt"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// projectRig shows the projects page for a fresh store.
func projectRig(t *testing.T) (*Store, *ui.Runtime) {
	t.Helper()
	s := NewStore()
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element { return Projects(s) }, ui.DarkTheme, nil)
	rt.SetViewport(1200, 900, 1)
	rt.Frame(time.Now())
	return s, rt
}

// pointAt returns the point at the fraction fy of the height of the element
// with key, at its horizontal middle.
func pointAt(t *testing.T, rt *ui.Runtime, key string, fy float32) (float32, float32) {
	t.Helper()
	r, ok := rt.Find(key)
	if !ok {
		t.Fatalf("no element with key %q", key)
	}
	return r.X + r.W/2, r.Y + r.H*fy
}

// pressOn presses on the element with key and drags past the threshold.
func pressOn(t *testing.T, rt *ui.Runtime, key string) {
	t.Helper()
	x, y := pointAt(t, rt, key, 0.5)
	rt.PointerMove(x, y, 0)
	rt.Frame(time.Now())
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerMove(x, y+6, 0)
	rt.Frame(time.Now())
}

// ids lists the project IDs in list order.
func ids(ps []Project) []int {
	var out []int
	for _, p := range ps {
		out = append(out, p.ID)
	}
	return out
}

func TestDraggingProjectAfterAnotherReorders(t *testing.T) {
	s, rt := projectRig(t)
	pressOn(t, rt, "1")
	x, y := pointAt(t, rt, "3", 0.8) // the lower part of project 3: after it
	rt.PointerMove(x, y, 0)
	rt.Frame(time.Now())
	if mark := s.DropMark.Peek(); mark != (dropMark{ID: 3, After: true}) {
		t.Errorf("insertion mark over the lower half = %+v; want after project 3", mark)
	}
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	rt.Frame(time.Now())

	if got, want := fmt.Sprint(ids(s.Projects.Peek())), "[2 3 1 4 5 6]"; got != want {
		t.Errorf("order after the drop = %s; want %s", got, want)
	}
	if s.DropMark.Peek() != (dropMark{}) {
		t.Error("the insertion line is still shown after the drop")
	}
}

func TestReorderWorksUnderAStatusFilter(t *testing.T) {
	s, rt := projectRig(t)
	s.Filter.Set(int(StatusPaused)) // shows 5 and 6 only
	rt.Frame(time.Now())
	pressOn(t, rt, "6")
	x, y := pointAt(t, rt, "5", 0.2) // the upper part of project 5: before it
	rt.PointerMove(x, y, 0)
	rt.Frame(time.Now())
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	rt.Frame(time.Now())
	if got, want := fmt.Sprint(ids(s.Projects.Peek())), "[1 2 3 4 6 5]"; got != want {
		t.Errorf("order under a filter = %s; want %s", got, want)
	}
}

func TestDroppingOnDoneSetsStatus(t *testing.T) {
	s, rt := projectRig(t)
	pressOn(t, rt, "1")
	x, y := pointAt(t, rt, "filter-Done", 0.5)
	rt.PointerMove(x, y, 0)
	rt.Frame(time.Now())
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	rt.Frame(time.Now())
	for _, p := range s.Projects.Peek() {
		if p.ID == 1 && p.Status != StatusDone {
			t.Errorf("project 1 status after the drop = %v; want Done", p.Status)
		}
	}
}
