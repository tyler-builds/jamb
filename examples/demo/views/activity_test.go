package views

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/scene"
	"github.com/tyler-builds/jamb/ui"
)

// The Activity page lists 10,000 events; only the rows on screen are built.
func TestActivityBuildsOnlyVisibleRows(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageActivity)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())
	if n := rt.Stats().Nodes; n >= 2000 {
		t.Fatalf("%d nodes for %d events, want fewer than 2,000", n, len(events))
	}
	if _, ok := rt.Find("event-10000"); ok {
		t.Error("the last event is built before it is scrolled to")
	}
}

// Wheeling to the bottom of the list shows the last event.
func TestActivityScrollsToLastEvent(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageActivity)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	list, ok := rt.Find("events")
	if !ok {
		t.Fatal("no event list")
	}
	x, y := list.X+list.W/2, list.Y+list.H/2
	for i := 0; i < 1000; i++ {
		rt.Scroll(x, y, 0, 1000, false, 0)
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
	}
	for i := 0; i < 120; i++ { // let the smooth scroll settle
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
	}
	last, ok := rt.Find("event-10000")
	if !ok {
		t.Fatal("the last event is not built after scrolling to the bottom")
	}
	list, _ = rt.Find("events")
	if last.Y < list.Y || last.Y+last.H > list.Y+list.H {
		t.Errorf("last event %+v is not inside the list %+v", last, list)
	}
}

// The toolbar's Jump to latest and Jump to top buttons scroll the list to its
// last and first events.
func TestActivityJumpButtons(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageActivity)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	settle := func() {
		for i := 0; i < 300; i++ {
			now = now.Add(16 * time.Millisecond)
			rt.Frame(now)
		}
	}
	if err := rt.ClickKey("jump-latest"); err != nil {
		t.Fatal(err)
	}
	settle()
	if _, ok := rt.Find("event-10000"); !ok {
		t.Fatal("Jump to latest did not show the last event")
	}
	if err := rt.ClickKey("jump-top"); err != nil {
		t.Fatal(err)
	}
	settle()
	if _, ok := rt.Find("event-1"); !ok {
		t.Fatal("Jump to top did not show the first event")
	}
}

// Compact mode changes every row's height; the list measures the new height.
func TestActivityCompactRowsRemeasure(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageActivity)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())
	height := func() float32 {
		r, _ := rt.Find("event-1") // a status change: one line
		return r.H
	}
	before := height()
	s.Compact.Set(true)
	rt.Frame(time.Now().Add(time.Second))
	if after := height(); after >= before {
		t.Errorf("row height %v in compact mode, want less than %v", after, before)
	}
}

// Rows differ in height: a comment's text wraps under its header, so it is
// taller than a one-line status change.
func TestActivityRowsVaryInHeight(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageActivity)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())
	status, _ := rt.Find("event-1")
	comment, _ := rt.Find("event-2")
	if events[1].kind != commentEvent || events[0].kind != statusEvent {
		t.Fatal("the first events are not a status change and a comment")
	}
	if comment.H <= status.H {
		t.Errorf("comment row is %v tall, status row %v; want the comment taller", comment.H, status.H)
	}
}

// Clicking a build row shows its details. The expansion is keyed by the event's
// ID, so it survives the rows leaving and returning to the list.
func TestActivityExpandIsKeyedByID(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageActivity)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())
	if _, ok := rt.Find("details-4"); ok {
		t.Fatal("the details of build 4 are open before any click")
	}
	if err := rt.ClickKey("event-4"); err != nil {
		t.Fatal(err)
	}
	if _, ok := rt.Find("details-4"); !ok {
		t.Fatal("clicking the build row did not show its details")
	}
	if !s.Expanded.Get()[4] {
		t.Fatal("the store does not hold the expansion of event 4")
	}

	// Hide every event, then show them again: the row is built anew.
	s.EventQuery.Set("no such event")
	rt.Frame(time.Now())
	s.EventQuery.Set("")
	rt.Frame(time.Now())
	if _, ok := rt.Find("details-4"); !ok {
		t.Fatal("the expansion of event 4 was lost when it left and returned")
	}
	if err := rt.ClickKey("event-4"); err != nil {
		t.Fatal(err)
	}
	if _, ok := rt.Find("details-4"); ok {
		t.Error("clicking the expanded row did not collapse it")
	}
}

// The filter keeps the events whose header or comment matches, counts them,
// and highlights the matches. A query with no match shows the empty state.
func TestActivityFilterHighlightsAndCounts(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageActivity)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())
	soft := scene.Color(matchHighlight(ui.DarkTheme()).Background)
	highlights := func() int {
		n := 0
		for _, q := range rt.Frame(time.Now()).Quads {
			if q.Background == soft {
				n++
			}
		}
		return n
	}
	base := highlights()

	const q = "dev runner"
	want := 0
	for _, e := range events {
		if strings.Contains(strings.ToLower(e.text()), q) {
			want++
		}
	}
	s.EventQuery.Set("Dev Runner")
	rt.Frame(time.Now())
	if got := len(s.matchingEvents("Dev Runner")); got != want || want == 0 {
		t.Fatalf("filter kept %d events, want %d (and some)", got, want)
	}
	if got := highlights(); got <= base {
		t.Errorf("filtering painted %d highlight quads, want more than the %d before", got, base)
	}

	s.EventQuery.Set("no such event")
	rt.Frame(time.Now())
	if _, ok := rt.Find("events"); ok {
		t.Error("the list is still shown for a filter with no matches")
	}
	if _, ok := rt.Find("no-events"); !ok {
		t.Error("no empty state for a filter with no matches")
	}
}

// Matching uses the same events on every render, so the list does not rebuild
// its index while the filter holds.
func TestMatchingEventsKeepsTheSliceWhileTheQueryHolds(t *testing.T) {
	s := NewStore()
	a := s.matchingEvents("dev runner")
	b := s.matchingEvents("  Dev Runner ")
	if len(a) == 0 || &a[0] != &b[0] {
		t.Error("the same query returned a different slice")
	}
	if all := s.matchingEvents(""); len(all) != len(events) {
		t.Errorf("a blank query returned %d events, want %d", len(all), len(events))
	}
}
