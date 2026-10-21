package ui

import (
	"fmt"
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
	r.rt.toast("", ToastInfo, "Saved")
	r.rt.toast("", ToastSuccess, "Project created")
	r.rt.toast("", ToastWarning, "Disk nearly full")
	r.rt.toast("", ToastError, "Upload failed")
	// A frame announces the toasts shown since the last one, and publishes and drains the queue.
	// The test runs the announcements itself, and reads the queue before a frame does.
	r.rt.announceToasts(r.now)
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
	r.rt.toast("", ToastInfo, "Saved")
	r.frame()
	accesstest.Find(t, r.tree(), access.RoleGroup, "Notifications")
	accesstest.Find(t, r.tree(), access.RoleGroup, "Saved")
	accesstest.Find(t, r.tree(), access.RoleButton, "Dismiss notification")
}

func TestToastDoesNotTakeFocus(t *testing.T) {
	r := toastRig(t)
	r.focusKey(t, "save")
	r.rt.toast("", ToastWarning, "Disk nearly full").Action("Undo", func() {})
	r.frame()
	if got := r.focusedKey(); got != "save" {
		t.Errorf("focus is on %q after a toast appeared, want it to stay on save", got)
	}
}

func TestToastAutoDismissesAfterItsDuration(t *testing.T) {
	r := toastRig(t)
	id := r.rt.toast("", ToastInfo, "Saved").e.id
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
	id := r.rt.toast("", ToastInfo, "Saved").e.id
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
	id := r.rt.toast("", ToastInfo, "Saved").e.id
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
	id := r.rt.toast("", ToastError, "Upload failed").e.id
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
		ids = append(ids, r.rt.toast("", ToastInfo, s).e.id)
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
	id := r.rt.toast("", ToastSuccess, "Project created").Action("Undo", func() { ran++ }).e.id
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
	id := r.rt.toast("", ToastInfo, "Saved").e.id
	r.frame()
	r.focusAccess(t, access.RoleButton, "Dismiss notification")
	r.press(input.KeyEnter, 0)
	if present(r, id) {
		t.Error("the close button did not dismiss the toast")
	}
}

func TestToastF6MovesFocusIntoRegionAndBack(t *testing.T) {
	r := toastRig(t)
	r.rt.toast("", ToastInfo, "Saved")
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
	id := r.rt.toast("", ToastInfo, "Saved").e.id
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
	r.rt.toast("", ToastInfo, "Saved")
	r.frame()
	r.focusKey(t, "save")
	r.press(input.KeyF6, 0)
	r.press(input.KeyEscape, 0)
	if got := r.focusedKey(); got != "save" {
		t.Errorf("Escape in the region put focus on %q, want save", got)
	}
}

// A toast's time runs without frames: the runtime asks to be woken when the
// toast runs out, and dismisses it then.
func TestToastWakesWhenItRunsOut(t *testing.T) {
	r := toastRig(t)
	var wakes []time.Time
	r.rt.hostSetServices(platformServices{WakeAt: func(at time.Time) { wakes = append(wakes, at) }})
	r.rt.toast("", ToastInfo, "Saved")
	created := r.now
	// The toast's entrance transition runs for a few frames first.
	for i := 0; i < 100 && r.rt.NeedsFrame(); i++ {
		r.frame()
	}
	if r.rt.NeedsFrame() {
		t.Fatal("a toast on screen asks for frames after its entrance")
	}
	end := created.Add(toastDefaultTime(ToastInfo))
	if len(wakes) == 0 || !wakes[len(wakes)-1].Equal(end) {
		t.Fatalf("wake-ups %v, want the toast's end at %v", wakes, end)
	}
	r.now = end.Add(-time.Millisecond)
	r.rt.Frame(r.now)
	if len(r.rt.toasts) != 1 {
		t.Fatal("the toast was dismissed before its time ran out")
	}
	r.now = end
	r.rt.Frame(r.now)
	if len(r.rt.toasts) != 0 {
		t.Error("the toast is still shown when its time has run out")
	}
}

func TestToastIconsDifferByShape(t *testing.T) {
	th := LightTheme()
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
	a := r.rt.toast("", ToastInfo, "one").e.id
	r.frame()
	r.focusAccess(t, access.RoleButton, "Dismiss notification") // the first toast's close button
	b := r.rt.toast("", ToastInfo, "two").e.id
	r.frame()
	rc := r.rt.nodeRect(r.keyed(t, toastKey(b)))
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.frame()
	c := r.rt.toast("", ToastInfo, "three").e.id
	r.frame()
	// The stack moved up under the pointer, so the pointer is put back on the hovered toast.
	rc = r.rt.nodeRect(r.keyed(t, toastKey(b)))
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.frame()
	d := r.rt.toast("", ToastInfo, "four").e.id
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
	r.rt.toast("", ToastError, "Failed")
	advance(r, 500*time.Millisecond) // its enter motion ends first
	if r.rt.NeedsFrame() {
		t.Error("an error toast asks for frames, though it has no time to run")
	}
}

// The time of a timed toast starts when it is shown. While only a sticky toast was
// up no frames ran, and that idle time is not the timed toast's.
func TestTimedToastClockStartsWhenShown(t *testing.T) {
	r := toastRig(t)
	r.rt.toast("", ToastError, "Failed")
	r.frame()
	r.now = r.now.Add(60 * time.Second) // idle: no frame runs while only the error is up
	id := r.rt.toast("", ToastInfo, "Saved").e.id
	r.frame()
	advance(r, 4900*time.Millisecond)
	if !present(r, id) {
		t.Error("the info toast went at once: the idle time was counted against it")
	}
}

// Toast.Dismiss from a handler, with the focus on the toast, moves the focus as
// the close button does: back to the element that had it before the region.
func TestToastDismissMovesFocusOffAToastThatHadIt(t *testing.T) {
	r := toastRig(t)
	toast := r.rt.toast("", ToastInfo, "Saved")
	r.frame()
	r.focusKey(t, "save")
	r.press(input.KeyF6, 0)
	r.press(input.KeyTab, 0) // into the toast: its close button
	toast.Dismiss()
	r.frame()
	if present(r, toast.e.id) {
		t.Fatal("Dismiss did not remove the toast")
	}
	if got := r.focusedKey(); got != "save" {
		t.Errorf("after Dismiss with focus in the toast, focus is on %q, want save", got)
	}
}

// toastCtxRig is toastRig, and it returns the Context of the window, for the
// public toast methods.
func toastCtxRig(t *testing.T) (*accessRig, *Context) {
	t.Helper()
	var ctx *Context
	root := feedbackRoot(Button("Save").Key("save"))
	r := newAccessRig(func(c *Context) Element { ctx = c; return root(c) })
	r.frame()
	return r, ctx
}

// spoken records the announcements the window publishes from now on.
func spoken(r *accessRig) *[]access.Announcement {
	var got []access.Announcement
	r.bridge.Listen(func(u access.Update) { got = append(got, u.Announcements...) })
	return &got
}

// textsOf returns the texts of the announcements.
func textsOf(as []access.Announcement) []string {
	var out []string
	for _, a := range as {
		out = append(out, a.Text)
	}
	return out
}

func TestToastWithIDUpdatesInPlace(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ctx.ToastWithID("upload", ToastInfo, "Uploading 20%").Title("Upload")
	r.frame()
	first := r.rt.toasts[0]
	node := r.keyed(t, toastKey(first.id)).id
	ctx.ToastWithID("upload", ToastInfo, "Uploading 40%").Title("Upload")
	ctx.ToastWithID("", ToastInfo, "Other")
	r.frame()
	if len(r.rt.toasts) != 2 {
		t.Fatalf("%d toasts, want 2: the update must not add one", len(r.rt.toasts))
	}
	if r.rt.toasts[0] != first || first.text != "Uploading 40%" || first.title != "Upload" {
		t.Errorf("the update did not change the toast in place: %+v", first)
	}
	if got := r.keyed(t, toastKey(first.id)).id; got != node {
		t.Error("the updated toast has a new element, so it animates out and in again")
	}
}

func TestToastWithIDOfAShownToastIsNewAfterItIsDismissed(t *testing.T) {
	r, ctx := toastCtxRig(t)
	first := ctx.ToastWithID("upload", ToastInfo, "Uploading").e
	r.frame()
	ctx.DismissToast("upload")
	r.frame()
	if present(r, first.id) {
		t.Fatal("DismissToast did not remove the toast")
	}
	again := ctx.ToastWithID("upload", ToastInfo, "Uploading").e
	r.frame()
	if again == first || !present(r, again.id) {
		t.Error("an ID whose toast was dismissed did not show a new toast")
	}
}

func TestDismissToastWithUnknownIDDoesNothing(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ctx.ToastWithID("upload", ToastInfo, "Uploading")
	r.frame()
	ver := r.rt.toastVer.Peek()
	ctx.DismissToast("missing")
	ctx.DismissToast("")
	if r.rt.toastVer.Peek() != ver || len(r.rt.toasts) != 1 {
		t.Error("DismissToast with no toast shown changed the stack")
	}
}

func TestToastUpdateRestartsItsTime(t *testing.T) {
	r, ctx := toastCtxRig(t)
	id := ctx.ToastWithID("upload", ToastInfo, "Uploading 20%").e.id
	r.frame()
	advance(r, 4*time.Second)
	ctx.ToastWithID("upload", ToastInfo, "Uploading 40%")
	r.frame()
	advance(r, 4900*time.Millisecond)
	if !present(r, id) {
		t.Fatal("an updated toast went before its time from the update ran out")
	}
	advance(r, 200*time.Millisecond)
	if present(r, id) {
		t.Error("an updated toast is still up after its time from the update")
	}
}

func TestToastUpdateWhileHoveredStaysPaused(t *testing.T) {
	r, ctx := toastCtxRig(t)
	id := ctx.ToastWithID("upload", ToastInfo, "Uploading").e.id
	r.frame()
	rc := r.rt.nodeRect(r.keyed(t, toastKey(id)))
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.frame()
	ctx.ToastWithID("upload", ToastInfo, "Uploaded")
	r.frame()
	advance(r, 10*time.Second)
	if !present(r, id) {
		t.Fatal("an updated toast that is hovered was dismissed")
	}
	r.rt.PointerMove(0, 0, 0)
	r.frame()
	advance(r, 5500*time.Millisecond)
	if present(r, id) {
		t.Error("an updated toast the pointer left was not dismissed after its time ran out")
	}
}

func TestToastUpdateWithTheSameTextIsNotAnnouncedAgain(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ann := spoken(r)
	ctx.ToastWithID("upload", ToastInfo, "Saved")
	r.frame()
	ctx.ToastWithID("upload", ToastInfo, "Saved")
	r.frame()
	if got := textsOf(*ann); len(got) != 1 || got[0] != "Saved" {
		t.Errorf("announcements %v, want [Saved] once", got)
	}
}

func TestToastChangedTextIsAnnouncedPolitely(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ann := spoken(r)
	ctx.ToastWithID("upload", ToastInfo, "Uploading 20%")
	r.frame()
	advance(r, toastAnnounceGap)
	ctx.ToastWithID("upload", ToastInfo, "Uploaded")
	r.rt.markAccess()
	r.frame()
	got := *ann
	if len(got) != 2 || got[1].Text != "Uploaded" || got[1].Politeness != access.Polite {
		t.Errorf("announcements %+v, want Uploaded politely after the first", got)
	}
	if got[1].Source != "jamb.toast:upload" {
		t.Errorf("the source is %q, want the toast's own", got[1].Source)
	}
}

func TestToastVariantChangeIsAnnouncedAtOnceAndAssertively(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ann := spoken(r)
	ctx.ToastWithID("upload", ToastInfo, "Uploading 20%")
	r.frame()
	ctx.ToastWithID("upload", ToastError, "Upload failed")
	r.frame()
	got := *ann
	if len(got) != 2 || got[1].Text != "Upload failed" || got[1].Politeness != access.Assertive {
		t.Errorf("announcements %+v, want Upload failed assertively at once", got)
	}
}

func TestToastUpdatesInOneHandlerAreAnnouncedOnce(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ann := spoken(r)
	for i := 1; i <= 5; i++ {
		ctx.ToastWithID("upload", ToastInfo, fmt.Sprintf("Uploading %d0%%", i))
	}
	r.frame()
	if got := textsOf(*ann); len(got) != 1 || got[0] != "Uploading 50%" {
		t.Errorf("announcements %v, want the last text once", got)
	}
}

// An update of the same kind of toast within the gap waits for the gap to end, and
// then says its latest text.
func TestToastUpdateWithinTheGapIsAnnouncedAtItsEnd(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ann := spoken(r)
	ctx.ToastWithID("upload", ToastInfo, "Uploading 20%")
	r.frame()
	advance(r, 500*time.Millisecond)
	ctx.ToastWithID("upload", ToastInfo, "Uploading 40%")
	r.frame()
	if got := textsOf(*ann); len(got) != 1 {
		t.Fatalf("announcements %v before the gap ended, want only the first", got)
	}
	advance(r, toastAnnounceGap)
	r.rt.markAccess()
	r.frame()
	if got := textsOf(*ann); len(got) != 2 || got[1] != "Uploading 40%" {
		t.Errorf("announcements %v, want Uploading 40%% once the gap ended", got)
	}
}

// A toast with a title is named by the title and described by its text, and it is
// announced as the title and the text together, from the toast's own source.
func TestToastWithTitleIsNamedAndAnnouncedWithIt(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ann := spoken(r)
	ctx.ToastWithID("upload", ToastSuccess, "Uploaded").Title("Upload")
	r.frame()
	n := accesstest.Find(t, r.tree(), access.RoleGroup, "Upload")
	if n.Description != "Uploaded" {
		t.Errorf("description %q, want Uploaded", n.Description)
	}
	got := *ann
	if len(got) != 1 || got[0].Text != "Upload: Uploaded" || got[0].Source != "jamb.toast:upload" {
		t.Errorf("announcements %+v, want Upload: Uploaded from the toast's source", got)
	}
}

func TestToastUpdateNeverMovesFocus(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ctx.ToastWithID("upload", ToastWarning, "Disk nearly full").Action("Undo", func() {})
	r.frame()
	r.focusKey(t, "save")
	ctx.ToastWithID("upload", ToastWarning, "Disk full").Action("Undo", func() {})
	r.frame()
	if got := r.focusedKey(); got != "save" {
		t.Errorf("focus is on %q after an update, want it to stay on save", got)
	}
}

func TestToastUpdateThatKeepsTheFocusedActionKeepsTheFocus(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ctx.ToastWithID("upload", ToastWarning, "Disk nearly full").Action("Undo", func() {})
	r.frame()
	r.focusAccess(t, access.RoleButton, "Undo")
	ctx.ToastWithID("upload", ToastWarning, "Disk full").Action("Undo", func() {})
	r.frame()
	if got := r.focusedKey(); got != toastActionKey(r.rt.toasts[0].id) {
		t.Errorf("focus is on %q after an update that keeps the action, want the action", got)
	}
}

// An update that removes the action the focus was on moves the focus to the toast
// region, as a toast that goes does.
func TestToastUpdateThatRemovesTheFocusedActionMovesFocusToTheRegion(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ctx.ToastWithID("upload", ToastWarning, "Disk nearly full").Action("Undo", func() {})
	r.frame()
	r.focusAccess(t, access.RoleButton, "Undo")
	ctx.ToastWithID("upload", ToastWarning, "Disk full")
	r.frame()
	if got := r.focusedKey(); got != "jamb.toasts" {
		t.Errorf("focus is on %q after its action went, want the toast region", got)
	}
}

func TestToastUpdateTakesNoRoomInTheStack(t *testing.T) {
	r, ctx := toastCtxRig(t)
	for _, id := range []string{"a", "b", "c"} {
		ctx.ToastWithID(id, ToastInfo, id)
	}
	r.frame()
	ctx.ToastWithID("a", ToastInfo, "a again")
	r.frame()
	if len(r.rt.toasts) != toastMaxDefault {
		t.Fatalf("%d toasts, want %d", len(r.rt.toasts), toastMaxDefault)
	}
	if r.rt.toasts[0].appID != "a" || r.rt.toasts[0].text != "a again" {
		t.Errorf("the update moved or dropped the first toast: %+v", r.rt.toasts[0])
	}
}

// The announcement of a titled toast is read in the language of the window.
func TestToastAnnouncementFollowsTheLanguage(t *testing.T) {
	cases := []struct{ tag, want string }{
		{"en", "Upload: Uploaded"},
		{"it-IT", "Upload: Uploaded"},
		{"zh-CN", "Upload：Uploaded"},
		{"zh-TW", "Upload：Uploaded"},
	}
	for _, c := range cases {
		if got := StringsFor(c.tag).ToastAnnouncement("Upload", "Uploaded"); got != c.want {
			t.Errorf("%s: %q, want %q", c.tag, got, c.want)
		}
	}
}

// A held update of a toast that is dismissed before the gap ends is not announced.
func TestToastHeldUpdateIsDroppedWhenDismissed(t *testing.T) {
	r, ctx := toastCtxRig(t)
	ann := spoken(r)
	ctx.ToastWithID("upload", ToastInfo, "Uploading 20%")
	r.frame()
	ctx.ToastWithID("upload", ToastInfo, "Uploading 40%")
	ctx.DismissToast("upload")
	advance(r, 3*time.Second)
	r.rt.markAccess()
	r.frame()
	if got := textsOf(*ann); len(got) != 1 {
		t.Errorf("announcements %v, want only the first: the held update went with its toast", got)
	}
}
