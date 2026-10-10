package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// The Components page settles after it is scrolled with the pointer over its
// "Hover: scale and lift" card, as the JAMB_SCRIPT idle step needs. The card's
// hover lifted it out from under the pointer, so the hover ended and began
// again every frame and the page never settled. Repeating animations (the
// Motion bars) are not waited for, as the idle step expects.
func TestComponentsPageSettlesAfterScrollUnderHover(t *testing.T) {
	s := NewStore()
	rt := ui.NewRuntime(App(s), ui.DarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	now := time.Unix(1_000_000, 0)
	frame := func() {
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
	}
	frame()
	s.Page.Set(PageComponents)
	for range 20 {
		frame()
	}
	// (600, 400) is on the edge of the hover card, which the scroll moves under
	// the pointer. It is the point where the loop was found.
	rt.PointerMove(600, 400, 0)
	for range 3 {
		rt.Scroll(600, 400, 0, 600, false, 0)
	}
	quiet := 0
	for i := 0; i < 300 && quiet < 3; i++ {
		frame()
		if rt.Settled() {
			quiet++
		} else {
			quiet = 0
		}
	}
	if quiet < 3 {
		t.Fatal("the Components page does not settle after scrolling under the hover card")
	}
}
