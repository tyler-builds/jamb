package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// toastRig renders a Save button and the window's Toaster.
func toastRig(t *testing.T) *accessRig {
	t.Helper()
	r := newAccessRig(feedbackRoot(Button("Save").Key("save")))
	r.frame()
	return r
}

// advance moves the clock on by d, a frame every 100 ms, as a running window does.
func advance(r *accessRig, d time.Duration) {
	for left := d; left > 0; left -= 100 * time.Millisecond {
		r.now = r.now.Add(min(left, 100*time.Millisecond))
		r.rt.Frame(r.now)
	}
}

// present reports whether the toast with the given ID is on the stack.
func present(r *accessRig, id uint64) bool {
	for _, e := range r.rt.toasts {
		if e.id == id {
			return true
		}
	}
	return false
}

func TestToastAnnouncesByVariant(t *testing.T) {
	r := toastRig(t)
	takeAnnouncements()
	r.rt.toast(ToastInfo, "Saved")
	r.rt.toast(ToastSuccess, "Project created")
	r.rt.toast(ToastWarning, "Disk nearly full")
	r.rt.toast(ToastError, "Upload failed")
	// A frame publishes and drains the queue, so the test reads it first.
	got := takeAnnouncements()
	want := []struct {
		text string
		p    access.Politeness
	}{
		{"Saved", access.Polite},
		{"Project created", access.Polite},
		{"Disk nearly full", access.Assertive},
		{"Upload failed", access.Assertive},
	}
	if len(got) != len(want) {
		t.Fatalf("%d announcements, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Text != want[i].text || got[i].Politeness != want[i].p {
			t.Errorf("announcement %d = %q (%v), want %q (%v)", i, got[i].Text, got[i].Politeness, want[i].text, want[i].p)
		}
	}
}

func TestToastRegionAndToastsAreNamed(t *testing.T) {
	r := toastRig(t)
	r.rt.toast(ToastInfo, "Saved")
	r.frame()
	accesstest.Find(t, r.tree(), access.RoleGroup, "Notifications")
	accesstest.Find(t, r.tree(), access.RoleGroup, "Saved")
	accesstest.Find(t, r.tree(), access.RoleButton, "Dismiss notification")
}

func TestToastDoesNotTakeFocus(t *testing.T) {
	r := toastRig(t)
	r.focusKey(t, "save")
	r.rt.toast(ToastWarning, "Disk nearly full").Action("Undo", func() {})
	r.frame()
	if got := r.focusedKey(); got != "save" {
		t.Errorf("focus is on %q after a toast appeared, want it to stay on save", got)
	}
}

func TestToastAutoDismissesAfterItsDuration(t *testing.T) {
	r := toastRig(t)
	id := r.rt.toast(ToastInfo, "Saved").e.id
	r.frame()
	advance(r, 4900*time.Millisecond)
	if !present(r, id) {
		t.Fatal("the info toast went before its 5 seconds")
	}
	advance(r, 200*time.Millisecond)
	if present(r, id) {
		t.Error("the info toast is still up after 5 seconds")
	}
}

func TestToastPausesWhileHovered(t *testing.T) {
	r := toastRig(t)
	id := r.rt.toast(ToastInfo, "Saved").e.id
	r.frame()
	rc := r.rt.nodeRect(r.keyed(t, toastKey(id)))
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.frame()
	advance(r, 10*time.Second)
	if !present(r, id) {
		t.Fatal("a hovered toast was dismissed")
	}
	r.rt.PointerMove(0, 0, 0)
	r.frame()
	advance(r, 5500*time.Millisecond)
	if present(r, id) {
		t.Error("a toast the pointer left was not dismissed after its time ran out")
	}
}

func TestToastPausesWhileFocused(t *testing.T) {
	r := toastRig(t)
	id := r.rt.toast(ToastInfo, "Saved").e.id
	r.frame()
	r.press(input.KeyF6, 0)
	r.press(input.KeyTab, 0)
	if !r.rt.focusWithin(toastKey(id)) {
		t.Fatalf("Tab did not reach the toast; focus is on %q", r.focusedKey())
	}
	advance(r, 10*time.Second)
	if !present(r, id) {
		t.Error("a toast with keyboard focus was dismissed")
	}
}

func TestToastErrorIsStickyUntilDismissed(t *testing.T) {
	r := toastRig(t)
	id := r.rt.toast(ToastError, "Upload failed").e.id
	r.frame()
	advance(r, 60*time.Second)
	if !present(r, id) {
		t.Fatal("an error toast timed out")
	}
	r.focusAccess(t, access.RoleButton, "Dismiss notification")
	r.press(input.KeyEnter, 0)
	if present(r, id) {
		t.Error("Enter on the close button did not dismiss the toast")
	}
}

func TestToastStackKeepsTheNewest(t *testing.T) {
	r := toastRig(t)
	var ids []uint64
	for _, s := range []string{"one", "two", "three", "four"} {
		ids = append(ids, r.rt.toast(ToastInfo, s).e.id)
	}
	r.frame()
	if len(r.rt.toasts) != toastMaxDefault {
		t.Fatalf("%d toasts on the stack, want %d", len(r.rt.toasts), toastMaxDefault)
	}
	if present(r, ids[0]) {
		t.Error("the oldest toast is still on the stack")
	}
	if !present(r, ids[3]) {
		t.Error("the newest toast is not on the stack")
	}
}

func TestToastActionRunsAndDismisses(t *testing.T) {
	r := toastRig(t)
	ran := 0
	id := r.rt.toast(ToastSuccess, "Project created").Action("Undo", func() { ran++ }).e.id
	r.frame()
	r.focusAccess(t, access.RoleButton, "Undo")
	r.press(input.KeyEnter, 0)
	if ran != 1 {
		t.Errorf("the action ran %d times, want once", ran)
	}
	if present(r, id) {
		t.Error("the toast stayed after its action ran")
	}
}

func TestToastCloseButtonDismisses(t *testing.T) {
	r := toastRig(t)
	id := r.rt.toast(ToastInfo, "Saved").e.id
	r.frame()
	r.focusAccess(t, access.RoleButton, "Dismiss notification")
	r.press(input.KeyEnter, 0)
	if present(r, id) {
		t.Error("the close button did not dismiss the toast")
	}
}

func TestToastF6MovesFocusIntoRegionAndBack(t *testing.T) {
	r := toastRig(t)
	r.rt.toast(ToastInfo, "Saved")
	r.frame()
	r.focusKey(t, "save")
	r.press(input.KeyF6, 0)
	if got := r.focusedKey(); got != "jamb.toasts" {
		t.Fatalf("F6 put focus on %q, want the toast region", got)
	}
	r.press(input.KeyF6, 0)
	if got := r.focusedKey(); got != "save" {
		t.Errorf("F6 again put focus on %q, want the element that had it", got)
	}
}

func TestToastEscapeDismissesFocusedToastAndReturnsFocus(t *testing.T) {
	r := toastRig(t)
	id := r.rt.toast(ToastInfo, "Saved").e.id
	r.frame()
	r.focusKey(t, "save")
	r.press(input.KeyF6, 0)
	r.press(input.KeyTab, 0) // into the toast: its close button
	r.press(input.KeyEscape, 0)
	if present(r, id) {
		t.Error("Escape did not dismiss the focused toast")
	}
	if got := r.focusedKey(); got != "save" {
		t.Errorf("after the last toast went, focus is on %q, want save", got)
	}
}

func TestToastRegionEscapeReturnsFocus(t *testing.T) {
	r := toastRig(t)
	r.rt.toast(ToastInfo, "Saved")
	r.frame()
	r.focusKey(t, "save")
	r.press(input.KeyF6, 0)
	r.press(input.KeyEscape, 0)
	if got := r.focusedKey(); got != "save" {
		t.Errorf("Escape in the region put focus on %q, want save", got)
	}
}

func TestToastTimeWaitsForAnAnimationFrame(t *testing.T) {
	r := toastRig(t)
	r.rt.toast(ToastInfo, "Saved")
	r.frame()
	if !r.rt.NeedsFrame() {
		t.Error("a toast on screen asks for no frames, so its time would not run")
	}
}

func TestToastIconsDifferByShape(t *testing.T) {
	th := LightTheme
	seen := map[*icons.Icon]ToastVariant{}
	for v := ToastInfo; v <= ToastError; v++ {
		ic, _ := toastIcon(th, v)
		if other, ok := seen[ic]; ok {
			t.Errorf("variants %v and %v share an icon", other, v)
		}
		seen[ic] = v
	}
}

// TestToastOverflowKeepsFocusedAndHoveredToasts checks that a stack over its
// maximum drops the oldest toast that nobody is using: the focused one and the
// hovered one stay, and the oldest of the rest goes.
func TestToastOverflowKeepsFocusedAndHoveredToasts(t *testing.T) {
	r := toastRig(t)
	a := r.rt.toast(ToastInfo, "one").e.id
	r.frame()
	r.focusAccess(t, access.RoleButton, "Dismiss notification") // the first toast's close button
	b := r.rt.toast(ToastInfo, "two").e.id
	r.frame()
	rc := r.rt.nodeRect(r.keyed(t, toastKey(b)))
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.frame()
	c := r.rt.toast(ToastInfo, "three").e.id
	r.frame()
	// The stack moved up under the pointer, so the pointer is put back on the hovered toast.
	rc = r.rt.nodeRect(r.keyed(t, toastKey(b)))
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.frame()
	d := r.rt.toast(ToastInfo, "four").e.id
	r.frame()
	if !present(r, a) {
		t.Error("the focused toast was dropped when the stack overflowed")
	}
	if !present(r, b) {
		t.Error("the hovered toast was dropped when the stack overflowed")
	}
	if present(r, c) {
		t.Error("the oldest toast that nobody is using is still on the stack")
	}
	if !present(r, d) {
		t.Error("the newest toast is not on the stack")
	}
	if len(r.rt.toasts) != toastMaxDefault {
		t.Errorf("%d toasts, want %d", len(r.rt.toasts), toastMaxDefault)
	}
}

// A sticky toast, such as an error, has no time to run, so it asks for no frames:
// it stays until it is dismissed, and the app is idle while it is up.
func TestStickyToastAsksNoFrames(t *testing.T) {
	r := toastRig(t)
	r.rt.toast(ToastError, "Failed")
	advance(r, 500*time.Millisecond) // its enter motion ends first
	if r.rt.NeedsFrame() {
		t.Error("an error toast asks for frames, though it has no time to run")
	}
}

// The time of a timed toast starts when it is shown. While only a sticky toast was
// up no frames ran, and that idle time is not the timed toast's.
func TestTimedToastClockStartsWhenShown(t *testing.T) {
	r := toastRig(t)
	r.rt.toast(ToastError, "Failed")
	r.frame()
	r.now = r.now.Add(60 * time.Second) // idle: no frame runs while only the error is up
	id := r.rt.toast(ToastInfo, "Saved").e.id
	r.frame()
	advance(r, 4900*time.Millisecond)
	if !present(r, id) {
		t.Error("the info toast went at once: the idle time was counted against it")
	}
}
