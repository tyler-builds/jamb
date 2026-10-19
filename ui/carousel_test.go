package ui

import (
	"slices"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// carouselSlides returns n slides, each a text "Slide body k".
func carouselSlides(n int) []Element {
	var out []Element
	for i := range n {
		out = append(out, Div().Height(Px(60)).Child(Textf("Slide body %d", i+1)))
	}
	return out
}

// partCarousel is a carousel of 3 slides on the second, with an AutoPlay slow enough not
// to step in a test, disabled or not, with the instance style st.
func partCarousel(st CarouselStyle, disabled bool) Element {
	return Div().Column().Width(Px(400)).
		Child(Carousel(NewSignal(1), carouselSlides(3)...).Key("car").Label("Photos").AutoPlay(time.Hour).
			Disabled(disabled).Styles(st))
}

// carRig is an access rig with a carousel of slides keyed "car", over index, and the
// announcements and the changes it made.
type carRig struct {
	*accessRig
	index   *Signal[int]
	changes []int
	heard   []string
}

// newCarRig builds a carousel of n slides over a new index at first, with cfg applied.
func newCarRig(t *testing.T, n, first int, cfg func(*CarouselElement) *CarouselElement) *carRig {
	t.Helper()
	cr := &carRig{index: NewSignal(first)}
	cr.accessRig = newAccessRig(func(*Context) Element {
		c := Carousel(cr.index, carouselSlides(n)...).Key("car").Label("Photos").OnChange(func(i int) {
			if i != cr.index.Peek() {
				t.Errorf("OnChange(%d) ran before the signal was set", i)
			}
			cr.changes = append(cr.changes, i)
		})
		if cfg != nil {
			c = cfg(c)
		}
		return Div().Column().Width(Px(400)).Child(c).Child(Button("Elsewhere").Key("away"))
	})
	takeAnnouncements()
	cr.bridge.Listen(func(u access.Update) {
		for _, a := range u.Announcements {
			cr.heard = append(cr.heard, a.Text)
		}
	})
	return cr
}

// announced returns the announcements since the last call.
func (r *carRig) announced() []string {
	r.frame()
	for _, a := range takeAnnouncements() {
		r.heard = append(r.heard, a.Text)
	}
	h := r.heard
	r.heard = nil
	return h
}

// click clicks the element whose access node has the role and name.
func (r *carRig) click(t *testing.T, role access.Role, name string) {
	t.Helper()
	rc := r.rt.nodeRect(accessNode(t, r.accessRig, role, name))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
}

// wait moves the window's clock on by d, and lets a frame run.
func (r *carRig) wait(d time.Duration) {
	r.now = r.now.Add(d)
	r.frame()
}

func testCarouselParts(t *testing.T, applied map[string]bool) {
	type carCase struct {
		part     string
		set      func(*CarouselStyle, Style)
		disabled bool
		run      func(t *testing.T, r *accessRig)
	}
	one := func(part string) func(t *testing.T, r *accessRig) {
		return func(t *testing.T, r *accessRig) { wantBackground(t, partNode(t, r.rt, part, ""), part, markBG) }
	}
	dot := func(r *accessRig, name string) *node { return accessNode(t, r, access.RoleButton, name) }
	cases := []carCase{
		{part: "Carousel.Root", set: func(s *CarouselStyle, m Style) { s.Root = m }, run: one("Carousel.Root")},
		{part: "Carousel.RootDisabled", set: func(s *CarouselStyle, m Style) { s.RootDisabled = m }, disabled: true,
			run: func(t *testing.T, r *accessRig) {
				wantBackground(t, partNode(t, r.rt, "Carousel.Root", ""), "Carousel.RootDisabled", markBG)
			}},
		{part: "Carousel.Viewport", set: func(s *CarouselStyle, m Style) { s.Viewport = m }, run: one("Carousel.Viewport")},
		{part: "Carousel.Slide", set: func(s *CarouselStyle, m Style) { s.Slide = m }, run: one("Carousel.Slide")},
		{part: "Carousel.Controls", set: func(s *CarouselStyle, m Style) { s.Controls = m }, run: one("Carousel.Controls")},
		{part: "Carousel.NavButton", set: func(s *CarouselStyle, m Style) { s.NavButton = m },
			run: func(t *testing.T, r *accessRig) { wantBackgroundAll(t, r, "Carousel.NavButton") }},
		{part: "Carousel.PlayPause", set: func(s *CarouselStyle, m Style) { s.PlayPause = m }, run: one("Carousel.PlayPause")},
		{part: "Carousel.Dots", set: func(s *CarouselStyle, m Style) { s.Dots = m }, run: one("Carousel.Dots")},
		{part: "Carousel.Dot", set: func(s *CarouselStyle, m Style) { s.Dot = m },
			run: func(t *testing.T, r *accessRig) { wantBackground(t, dot(r, "Go to slide 1"), "Carousel.Dot", markBG) }},
		{part: "Carousel.DotCurrent", set: func(s *CarouselStyle, m Style) { s.DotCurrent = m },
			run: func(t *testing.T, r *accessRig) {
				wantBackground(t, dot(r, "Go to slide 2"), "Carousel.DotCurrent", markBG)
			}},
		{part: "Carousel.DotHover", set: func(s *CarouselStyle, m Style) { s.DotHover = m },
			run: func(t *testing.T, r *accessRig) {
				hoverNode(r, dot(r, "Go to slide 3"))
				wantBackground(t, dot(r, "Go to slide 3"), "Carousel.DotHover", markBG)
			}},
		{part: "Carousel.DotFocusVisible", set: func(s *CarouselStyle, m Style) { s.DotFocusVisible = m },
			run: func(t *testing.T, r *accessRig) {
				focusNode(r, dot(r, "Go to slide 2"))
				wantBackground(t, dot(r, "Go to slide 2"), "Carousel.DotFocusVisible", markBG)
			}},
		{part: "Carousel.DotDisabled", set: func(s *CarouselStyle, m Style) { s.DotDisabled = m }, disabled: true,
			run: func(t *testing.T, r *accessRig) {
				wantBackground(t, dot(r, "Go to slide 3"), "Carousel.DotDisabled", markBG)
			}},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st CarouselStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(*Context) Element { return partCarousel(st, c.disabled) })
			c.run(t, r)
		})
	}
}

func TestCarouselAccessTree(t *testing.T) {
	r := newCarRig(t, 3, 0, nil)
	root := r.stateOf(t, access.RoleGroup, "Photos")
	if root.Description != "Carousel" {
		t.Fatalf("the carousel is described %q, want \"Carousel\"", root.Description)
	}
	tr := r.tree()
	var slides []string
	tr.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleGroup && len(n.Name) > 6 && n.Name[:6] == "Slide " {
			slides = append(slides, n.Name)
		}
		return true
	})
	if !slices.Equal(slides, []string{"Slide 1 of 3"}) {
		t.Fatalf("the slides in the tree are %v, want only the one shown", slides)
	}
	if !nodeTextContains(r.rt, "Slide body 1") || nodeTextContains(r.rt, "Slide body 2") {
		t.Fatal("the carousel does not show only its first slide")
	}
	accesstest.Check(t, r.stateOf(t, access.RoleGroup, "Slides"), access.RoleGroup, "Slides", access.StateSelectionContainer)
	for i, name := range []string{"Go to slide 1", "Go to slide 2", "Go to slide 3"} {
		n := r.stateOf(t, access.RoleButton, name)
		if n.States.Has(access.StateSelected) != (i == 0) {
			t.Fatalf("%s has states [%s]", name, n.States)
		}
	}
	if !r.stateOf(t, access.RoleButton, "Previous slide").States.Has(access.StateDisabled) {
		t.Fatal("Previous slide is enabled on the first slide without Loop")
	}
	if r.stateOf(t, access.RoleButton, "Next slide").States.Has(access.StateDisabled) {
		t.Fatal("Next slide is disabled on the first slide")
	}
	if partNodeOrNil(r.rt, "Carousel.PlayPause") != nil {
		t.Fatal("a carousel without AutoPlay has a play button")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestCarouselUnnamedIsNamedCarousel(t *testing.T) {
	r := newAccessRig(func(*Context) Element {
		return Div().Column().Child(Carousel(NewSignal(0), carouselSlides(2)...))
	})
	if n := r.stateOf(t, access.RoleGroup, "Carousel"); n.Description != "" {
		t.Fatalf("an unnamed carousel is described %q; its name says it already", n.Description)
	}
}

func TestCarouselButtonsAndDotsChangeTheSlide(t *testing.T) {
	r := newCarRig(t, 3, 0, nil)
	r.announced()
	r.click(t, access.RoleButton, "Next slide")
	if r.index.Peek() != 1 || !nodeTextContains(r.rt, "Slide body 2") {
		t.Fatalf("Next gave slide %d", r.index.Peek())
	}
	if got := r.announced(); !slices.Contains(got, "Slide 2 of 3") {
		t.Fatalf("the change was announced as %v", got)
	}
	r.click(t, access.RoleButton, "Next slide")
	if !r.stateOf(t, access.RoleButton, "Next slide").States.Has(access.StateDisabled) {
		t.Fatal("Next slide is enabled on the last slide without Loop")
	}
	r.click(t, access.RoleButton, "Go to slide 1")
	r.click(t, access.RoleButton, "Previous slide")
	if want := []int{1, 2, 0}; !slices.Equal(r.changes, want) {
		t.Fatalf("OnChange ran with %v, want %v", r.changes, want)
	}
	if !r.stateOf(t, access.RoleButton, "Go to slide 1").States.Has(access.StateSelected) {
		t.Fatal("the dot of the slide shown is not selected")
	}
	// A change made from outside is shown, and is neither reported nor announced.
	r.announced()
	r.index.Set(2)
	r.frame()
	if !nodeTextContains(r.rt, "Slide body 3") || len(r.changes) != 3 {
		t.Fatalf("an index set from outside shows the wrong slide or ran OnChange (%v)", r.changes)
	}
	if got := r.announced(); len(got) != 0 {
		t.Fatalf("a change from outside was announced: %v", got)
	}
}

func TestCarouselLoopWraps(t *testing.T) {
	r := newCarRig(t, 3, 2, func(c *CarouselElement) *CarouselElement { return c.Loop() })
	if r.stateOf(t, access.RoleButton, "Next slide").States.Has(access.StateDisabled) {
		t.Fatal("Next slide is disabled on the last slide of a looping carousel")
	}
	r.click(t, access.RoleButton, "Next slide")
	if r.index.Peek() != 0 {
		t.Fatalf("Next on the last slide gave %d, want the first", r.index.Peek())
	}
	r.click(t, access.RoleButton, "Previous slide")
	if r.index.Peek() != 2 {
		t.Fatalf("Previous on the first slide gave %d, want the last", r.index.Peek())
	}
}

func TestCarouselKeysOnTheControls(t *testing.T) {
	r := newCarRig(t, 3, 0, nil)
	r.focusAccess(t, access.RoleButton, "Go to slide 1")
	r.rt.keyboardModality = true
	r.press(input.KeyRight, 0)
	if r.index.Peek() != 1 {
		t.Fatalf("Right gave slide %d", r.index.Peek())
	}
	r.frame()
	if !r.focusedAccess(t, access.RoleButton, "Go to slide 2") {
		t.Fatal("the focus did not follow the slide to its dot")
	}
	r.press(input.KeyRight, 0)
	r.press(input.KeyRight, 0) // the last slide: no change without Loop
	r.press(input.KeyLeft, 0)
	if want := []int{1, 2, 1}; !slices.Equal(r.changes, want) {
		t.Fatalf("the keys gave %v, want %v", r.changes, want)
	}
	// Only the dot of the slide shown is a Tab stop.
	if r.keyed(t, "car") == nil {
		t.Fatal("no focusable element in the carousel")
	}
	stops := 0
	for _, n := range partNodes(r.rt, "Carousel.Dot") {
		if n.div.focusable && !n.div.noTabStop {
			stops++
		}
	}
	if stops != 1 {
		t.Fatalf("the dots have %d Tab stops, want 1", stops)
	}
}

func TestCarouselAutoPlay(t *testing.T) {
	r := newCarRig(t, 3, 0, func(c *CarouselElement) *CarouselElement { return c.AutoPlay(time.Second) })
	r.announced()
	r.wait(500 * time.Millisecond)
	if r.index.Peek() != 0 {
		t.Fatal("AutoPlay moved before its time")
	}
	r.wait(600 * time.Millisecond)
	if r.index.Peek() != 1 {
		t.Fatalf("after a second AutoPlay shows %d, want 1", r.index.Peek())
	}
	if got := r.announced(); len(got) != 0 {
		t.Fatalf("AutoPlay announced %v", got)
	}
	// The pointer over the carousel holds it.
	hoverNode(r.accessRig, partNode(t, r.rt, "Carousel.Viewport", ""))
	r.wait(1100 * time.Millisecond)
	r.wait(1100 * time.Millisecond)
	if r.index.Peek() != 1 {
		t.Fatalf("AutoPlay moved to %d with the pointer over the carousel", r.index.Peek())
	}
	r.rt.PointerMove(-100, -100, 0)
	r.wait(1100 * time.Millisecond)
	if r.index.Peek() != 2 {
		t.Fatalf("AutoPlay did not go on after the pointer left: %d", r.index.Peek())
	}
	// It stops at the last slide without Loop.
	r.wait(1100 * time.Millisecond)
	r.wait(1100 * time.Millisecond)
	if r.index.Peek() != 2 {
		t.Fatalf("AutoPlay went past the last slide to %d", r.index.Peek())
	}
	if want := []int{1, 2}; !slices.Equal(r.changes, want) {
		t.Fatalf("OnChange ran with %v, want %v", r.changes, want)
	}
}

func TestCarouselAutoPlayWaitsForFocusInside(t *testing.T) {
	r := newCarRig(t, 3, 0, func(c *CarouselElement) *CarouselElement { return c.AutoPlay(time.Second).Loop() })
	r.focusAccess(t, access.RoleButton, "Next slide")
	r.wait(1100 * time.Millisecond)
	r.wait(1100 * time.Millisecond)
	if r.index.Peek() != 0 {
		t.Fatalf("AutoPlay moved to %d with focus inside the carousel", r.index.Peek())
	}
	r.focusKey(t, "away")
	r.wait(1100 * time.Millisecond)
	if r.index.Peek() != 1 {
		t.Fatalf("AutoPlay did not go on after focus left: %d", r.index.Peek())
	}
}

// TestCarouselPlayButton checks the button that stops and starts AutoPlay: it is the first
// control in the Tab order, a button named for what it does with no pressed state (the
// WAI-ARIA carousel pattern changes the name or the state, not both), and a press stops
// AutoPlay until the next press.
func TestCarouselPlayButton(t *testing.T) {
	r := newCarRig(t, 3, 0, func(c *CarouselElement) *CarouselElement { return c.AutoPlay(time.Second).Loop() })
	n := r.stateOf(t, access.RoleButton, "Pause slides")
	if n.States.Has(access.StateToggle) || n.States.Has(access.StateChecked) {
		t.Fatalf("the playing carousel's button has states [%s], want a plain button", n.States)
	}
	r.focusKey(t, "car")
	if !r.focusedAccess(t, access.RoleButton, "Pause slides") {
		t.Fatal("the play button is not the carousel's first Tab stop")
	}
	r.click(t, access.RoleButton, "Pause slides")
	if st := r.stateOf(t, access.RoleButton, "Play slides").States; st.Has(access.StateToggle) || st.Has(access.StateChecked) {
		t.Fatalf("the stopped carousel's button has states [%s], want a plain button named Play slides", st)
	}
	r.focusKey(t, "away")
	r.rt.PointerMove(-100, -100, 0)
	for range 3 {
		r.wait(1100 * time.Millisecond)
	}
	if r.index.Peek() != 0 {
		t.Fatalf("a paused carousel moved to %d", r.index.Peek())
	}
	r.click(t, access.RoleButton, "Play slides")
	r.focusKey(t, "away")
	r.rt.PointerMove(-100, -100, 0)
	r.wait(1100 * time.Millisecond)
	if r.index.Peek() != 1 {
		t.Fatalf("Play did not start AutoPlay again: %d", r.index.Peek())
	}
}

// TestCarouselReducedMotionStartsPaused checks that under reduced motion AutoPlay starts
// stopped, with the button showing Play, and that the user can start it.
func TestCarouselReducedMotionStartsPaused(t *testing.T) {
	r := newCarRig(t, 3, 0, func(c *CarouselElement) *CarouselElement { return c.AutoPlay(time.Second).Loop() })
	r.rt.SetSystemSettings(SystemSettings{ReducedMotion: true})
	r.frame()
	r.stateOf(t, access.RoleButton, "Play slides")
	r.click(t, access.RoleButton, "Play slides")
	r.focusKey(t, "away")
	r.rt.PointerMove(-100, -100, 0)
	r.wait(1100 * time.Millisecond)
	if r.index.Peek() != 1 {
		t.Fatalf("Play under reduced motion did not start AutoPlay: %d", r.index.Peek())
	}
	r.stateOf(t, access.RoleButton, "Pause slides")
}

func TestCarouselAutoPlayOffUnderReducedMotion(t *testing.T) {
	r := newCarRig(t, 3, 0, func(c *CarouselElement) *CarouselElement { return c.AutoPlay(time.Second).Loop() })
	r.rt.SetSystemSettings(SystemSettings{ReducedMotion: true})
	r.frame()
	for range 4 {
		r.wait(1100 * time.Millisecond)
	}
	if r.index.Peek() != 0 {
		t.Fatalf("AutoPlay moved to %d under reduced motion", r.index.Peek())
	}
	r.rt.SetSystemSettings(SystemSettings{})
	r.frame()
	r.wait(1100 * time.Millisecond)
	if r.index.Peek() != 1 {
		t.Fatalf("AutoPlay did not start when reduced motion went off: %d", r.index.Peek())
	}
}

func TestCarouselAutoPlayStopsWhenRemoved(t *testing.T) {
	shown := NewSignal(true)
	index := NewSignal(0)
	r := newAccessRig(func(*Context) Element {
		root := Div().Column()
		if shown.Get() {
			root.Child(Carousel(index, carouselSlides(3)...).AutoPlay(time.Second))
		}
		return root
	})
	shown.Set(false)
	r.frame()
	r.now = r.now.Add(1100 * time.Millisecond)
	r.frame()
	if index.Peek() != 0 {
		t.Fatalf("a removed carousel moved to %d", index.Peek())
	}
}

func TestCarouselDisabled(t *testing.T) {
	r := newCarRig(t, 3, 1, func(c *CarouselElement) *CarouselElement { return c.Disabled(true).AutoPlay(time.Second) })
	for _, name := range []string{"Previous slide", "Next slide", "Go to slide 1", "Go to slide 3"} {
		if !r.stateOf(t, access.RoleButton, name).States.Has(access.StateDisabled) {
			t.Fatalf("%s is enabled in a disabled carousel", name)
		}
	}
	r.click(t, access.RoleButton, "Go to slide 3")
	r.wait(1100 * time.Millisecond)
	if r.index.Peek() != 1 || len(r.changes) != 0 {
		t.Fatalf("a disabled carousel changed to %d", r.index.Peek())
	}
}

func TestCarouselClampsTheIndex(t *testing.T) {
	r := newCarRig(t, 3, 7, nil)
	if !nodeTextContains(r.rt, "Slide body 3") {
		t.Fatal("an index past the end does not show the last slide")
	}
	r.stateOf(t, access.RoleGroup, "Slide 3 of 3")
}

func TestCarouselHeightGivesTheViewportTheRoom(t *testing.T) {
	r := newCarRig(t, 2, 0, func(c *CarouselElement) *CarouselElement { return c.Height(Px(240)) })
	if h := r.rt.nodeRect(keyNode(t, r.rt, "car")).H; h != 240 {
		t.Fatalf("the carousel is %v high, want 240", h)
	}
	vp := r.rt.nodeRect(partNode(t, r.rt, "Carousel.Viewport", "")).H
	ctl := r.rt.nodeRect(partNode(t, r.rt, "Carousel.Controls", "")).H
	if vp+ctl+8 != 240 {
		t.Fatalf("the viewport (%v) and the controls (%v) do not fill the height", vp, ctl)
	}
}

func TestCarouselUnderHighContrast(t *testing.T) {
	r := newAccessRig(func(*Context) Element { return partCarousel(CarouselStyle{}, false) })
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	if c := accessNode(t, r, access.RoleButton, "Go to slide 1").style.Background; c != th.Text {
		t.Errorf("high contrast: a dot is %v, want the text color %v", c, th.Text)
	}
	cur := accessNode(t, r, access.RoleButton, "Go to slide 2")
	if cur.style.Background != th.Accent || cur.style.Width != Px(24) {
		t.Errorf("high contrast: the current dot is %v and %v wide, want Highlight and 24 px", cur.style.Background, cur.style.Width)
	}
}
