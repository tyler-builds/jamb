package ui

import (
	"slices"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// hoverCardRig is a hover card on a button, with a button beside it. set, when
// not nil, configures the card.
func hoverCardRig(set func(*HoverCardElement)) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		hc := HoverCard(Button("Dana Lee").Key("trigger"), func() Element {
			return Div().Key("card").Width(Px(160)).Child(Text("Product designer"))
		})
		if set != nil {
			set(hc)
		}
		return Div().Row().Padding(20).Gap(200).Align(Start).
			Child(hc).
			Child(Button("Elsewhere").Key("outside"))
	})
}

// wait moves the clock on by d and runs two frames at the new time: the first
// runs the timers that are due, and the second shows what they changed.
func (r *accessRig) wait(d time.Duration) {
	r.now = r.now.Add(d)
	r.rt.Frame(r.now)
	r.rt.Frame(r.now)
}

// pointTo moves the pointer to the center of the element with key, without
// moving the clock.
func (r *accessRig) pointTo(t *testing.T, key string) {
	t.Helper()
	rc, ok := r.rt.Find(key)
	if !ok {
		t.Fatalf("no element %q", key)
	}
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.rt.Frame(r.now)
}

// cardShown reports whether the hover card's content is shown.
func cardShown(r *accessRig) bool {
	_, ok := r.rt.Find("card")
	return ok
}

// TestHoverCardOpensAfterTheDelay checks that the card opens once the pointer has
// rested on the trigger for the open delay, and closes the close delay after the
// pointer leaves.
func TestHoverCardOpensAfterTheDelay(t *testing.T) {
	r := hoverCardRig(nil)
	r.pointTo(t, "trigger")
	r.wait(499 * time.Millisecond)
	if cardShown(r) {
		t.Fatal("the card opened before the open delay")
	}
	r.wait(time.Millisecond)
	if !cardShown(r) {
		t.Fatal("the card did not open after the open delay")
	}
	if got := r.focusedKey(); got != "" {
		t.Fatalf("the card moved focus to %q", got)
	}

	r.pointTo(t, "outside")
	r.wait(299 * time.Millisecond)
	if !cardShown(r) {
		t.Fatal("the card closed before the close delay")
	}
	r.wait(time.Millisecond)
	if cardShown(r) {
		t.Fatal("the card did not close after the close delay")
	}
}

// TestHoverCardLeavingEarlyDoesNotOpen checks that a pointer that passes over the
// trigger without resting opens nothing.
func TestHoverCardLeavingEarlyDoesNotOpen(t *testing.T) {
	r := hoverCardRig(nil)
	r.pointTo(t, "trigger")
	r.wait(300 * time.Millisecond)
	r.pointTo(t, "outside")
	r.wait(time.Second)
	if cardShown(r) {
		t.Fatal("the card opened though the pointer left before the open delay")
	}
}

// TestHoverCardStaysWhileThePointerCrossesToIt checks that the card stays open
// while the pointer crosses the gap from the trigger to the card, and while it is
// over the card.
func TestHoverCardStaysWhileThePointerCrossesToIt(t *testing.T) {
	r := hoverCardRig(nil)
	r.pointTo(t, "trigger")
	r.wait(500 * time.Millisecond)
	trig, _ := r.rt.Find("trigger")
	// The gap between the trigger and the card, below the trigger.
	r.rt.PointerMove(trig.X+10, trig.Y+trig.H+3, 0)
	r.wait(200 * time.Millisecond)
	r.pointTo(t, "card")
	r.wait(time.Second)
	if !cardShown(r) {
		t.Fatal("the card closed while the pointer was over it")
	}
	r.pointTo(t, "outside")
	r.wait(300 * time.Millisecond)
	if cardShown(r) {
		t.Fatal("the card did not close after the pointer left it")
	}
}

// TestHoverCardOpensOnKeyboardFocus checks that keyboard focus on the trigger
// opens the card after the delay without moving focus, that Escape closes it, and
// that focus leaving closes it at once.
func TestHoverCardOpensOnKeyboardFocus(t *testing.T) {
	r := hoverCardRig(nil)
	r.press(input.KeyTab, 0)
	wantFocus(t, r, "trigger", "Tab")
	r.wait(500 * time.Millisecond)
	if !cardShown(r) {
		t.Fatal("keyboard focus on the trigger did not open the card")
	}
	wantFocus(t, r, "trigger", "the card opened")

	r.press(input.KeyEscape, 0)
	if cardShown(r) {
		t.Fatal("Escape did not close the card")
	}
	wantFocus(t, r, "trigger", "Escape")

	r.press(input.KeyTab, 0)
	r.press(input.KeyTab, input.ModShift)
	r.wait(500 * time.Millisecond)
	if !cardShown(r) {
		t.Fatal("focus back on the trigger did not open the card")
	}
	r.press(input.KeyTab, 0)
	wantFocus(t, r, "outside", "Tab away")
	if cardShown(r) {
		t.Fatal("the card stayed open after focus left the trigger")
	}
}

// TestHoverCardDelays checks OpenDelay and CloseDelay.
func TestHoverCardDelays(t *testing.T) {
	r := hoverCardRig(func(h *HoverCardElement) { h.OpenDelay(100 * time.Millisecond).CloseDelay(50 * time.Millisecond) })
	r.pointTo(t, "trigger")
	r.wait(100 * time.Millisecond)
	if !cardShown(r) {
		t.Fatal("the card did not open after its open delay")
	}
	r.pointTo(t, "outside")
	r.wait(50 * time.Millisecond)
	if cardShown(r) {
		t.Fatal("the card did not close after its close delay")
	}
}

// TestHoverCardDescribesTheTrigger checks that the open card is a tooltip that
// describes the trigger, and that the trigger has no description while it is
// closed.
func TestHoverCardDescribesTheTrigger(t *testing.T) {
	r := hoverCardRig(nil)
	if n := accesstest.Find(t, r.tree(), access.RoleButton, "Dana Lee"); n.Description != "" || len(n.DescribedBy) != 0 {
		t.Fatalf("closed: description %q and described by %v, want none", n.Description, n.DescribedBy)
	}
	r.pointTo(t, "trigger")
	r.wait(500 * time.Millisecond)
	tr := r.tree()
	card := accesstest.Find(t, tr, access.RoleTooltip, "Product designer")
	n := accesstest.Find(t, tr, access.RoleButton, "Dana Lee")
	if n.Description != "Product designer" || !slices.Contains(n.DescribedBy, card.ID) {
		t.Fatalf("open: description %q and described by %v, want the card's text and %v", n.Description, n.DescribedBy, card.ID)
	}
	if card.States.Has(access.StateFocusable) {
		t.Error("the card is focusable")
	}
}

// testHoverCardParts checks the hover card's one part.
func testHoverCardParts(t *testing.T, applied map[string]bool) {
	applied["HoverCard.Panel"] = true
	t.Run("HoverCard.Panel", func(t *testing.T) {
		r := hoverCardRig(func(h *HoverCardElement) { h.Styles(HoverCardStyle{Panel: Style{Background: markBG}}) })
		r.pointTo(t, "trigger")
		r.wait(500 * time.Millisecond)
		wantBackground(t, partNode(t, r.rt, "HoverCard.Panel", ""), "HoverCard.Panel", markBG)
	})
}
