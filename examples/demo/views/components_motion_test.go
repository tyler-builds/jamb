package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// The motion samples of the Components page show their labels, and the delayed
// entrance of the delay sample finishes, which runs its OnDone. The runtime times its
// delays on the wall clock, so the frames here start from it.
func TestMotionSamplesShowAndDelayedEntranceFinishes(t *testing.T) {
	r := newDemoRig(t)
	r.now = time.Now()
	showComponents(r)
	tr := r.tree()
	for _, label := range []string{
		"Spring: Move, then Move back", "Keyframes: Play", "Timeline: Sequence, Then, Parallel",
		"Repeat: a timeline that loops", "Stagger: siblings enter one after another", "Delay and OnDone",
	} {
		accesstest.Find(t, tr, access.RoleStaticText, label)
	}
	for i := 0; i < 10 && !hasText(r, "Finished: the tile is in place."); i++ {
		r.frame()
	}
	if !hasText(r, "Finished: the tile is in place.") {
		t.Fatal("the delayed entrance does not finish, so its OnDone does not run")
	}
	if len(r.warnings) > 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}

// demoUI renders the demo app in a uitest UI on page, with the Components page
// filtered to the sections that match query, and lets it settle.
func demoUI(t *testing.T, page Page, query string) *uitest.UI {
	t.Helper()
	s := NewStore()
	s.Page.Set(page)
	s.SectionQuery.Set(query)
	u := uitest.New(t, App(s), uitest.Size(1200, 900), uitest.Theme(DemoDarkTheme))
	u.Settle()
	return u
}

// The motion samples move as they should when played: the springs, keyframes,
// timeline, stagger and delayed entrance neither pop nor jump, and all settle.
func TestMotionSamplesMoveSmoothly(t *testing.T) {
	u := demoUI(t, PageComponents, "Motion")
	rec := u.Record()
	u.Click("motion-move")
	u.Settle()
	u.Click("motion-move")
	u.Advance(100 * time.Millisecond)
	u.Click("motion-move") // retargeted mid-flight
	u.Settle()
	for _, key := range []string{"motion-play-keyframes", "motion-play-timeline", "motion-play-stagger", "motion-play-delay"} {
		u.Click(key)
		u.Settle()
	}
	rec.Check(t, uitest.Allow{Kind: uitest.Overshoot, Element: "section:Motion",
		Reason: "the Bounce 0.5 spring tile passes its target by design; the Bounce 0 one beside it does not"})
}

// The rows of the reorder sample glide to their new places when the list is
// shuffled, and when a row moves up, also while a move is still in flight:
// none of them jumps, and each moves over many frames.
func TestReorderSampleGlides(t *testing.T) {
	u := demoUI(t, PageComponents, "Motion")
	rec := u.Record()
	u.ByRole(access.RoleButton, "Move Echo up").Click()
	u.Advance(100 * time.Millisecond)
	u.ByRole(access.RoleButton, "Move Echo up").Click() // retargeted mid-flight
	u.Settle()
	u.Click("motion-shuffle")
	u.Settle()
	rec.Check(t)

	// Echo moved, and over many frames: the rows glide rather than jump.
	var ys []float32
	for _, f := range rec.Frames() {
		for _, s := range f.Elements {
			if s.Key == "motion-row-Echo" && !s.Exiting && (len(ys) == 0 || ys[len(ys)-1] != s.Bounds.Y) {
				ys = append(ys, s.Bounds.Y)
			}
		}
	}
	if len(ys) < 10 {
		t.Errorf("Echo was drawn at %d places, want it to glide over 10 or more: %v", len(ys), ys)
	}
}

// A card of the expanding card sample grows to show its details, over many
// frames, and the cards below it glide down with it; closing it, and opening
// another while one is open, glide too. Nothing pops or jumps.
func TestExpandCardSampleGrows(t *testing.T) {
	u := demoUI(t, PageComponents, "Motion")
	closed := u.Get("motion-card-Build").Bounds().H
	rec := u.Record()
	u.ByRole(access.RoleButton, "Show details for Build").Click()
	var hs, ys []float32
	for range 40 {
		u.Advance(16 * time.Millisecond)
		hs = append(hs, u.Get("motion-card-Build").Bounds().H)
		ys = append(ys, u.Get("motion-card-Docs").Bounds().Y)
	}
	full := hs[len(hs)-1]
	if full <= closed {
		t.Fatalf("the open card is %v tall, want more than the closed %v", full, closed)
	}
	grew := 0
	for i := 1; i < len(hs); i++ {
		if hs[i] < hs[i-1] {
			t.Fatalf("the card shrank while it opened: %v", hs)
		}
		if hs[i] > hs[i-1] {
			grew++
		}
	}
	if grew < 10 {
		t.Errorf("the card grew over %d frames (%v), want 10 or more", grew, hs)
	}
	if d := (ys[len(ys)-1] - ys[0]) - (full - hs[0]); d > 1 || d < -1 {
		t.Errorf("the last card moved %v while the first grew %v, want them together", ys[len(ys)-1]-ys[0], full-hs[0])
	}
	u.Settle()
	u.ByRole(access.RoleButton, "Show details for Release").Click()
	u.Settle()
	u.ByRole(access.RoleButton, "Hide details for Release").Click()
	u.Settle()
	rec.Check(t)
}
