package ui

import (
	"strconv"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// clickKey presses the element with the key at its middle and releases it, then
// renders the frame.
func clickKey(t *testing.T, r *accessRig, key string) {
	t.Helper()
	rc, ok := r.rt.Find(key)
	if !ok {
		t.Fatalf("no element %q", key)
	}
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
}

// TestCopyButtonCopiesAndConfirms checks that a text copy button copies its text,
// shows its confirmation as its label, announces it politely, and returns to its
// label once the confirmation's delay has passed. It shows no toast by default.
func TestCopyButtonCopiesAndConfirms(t *testing.T) {
	var copied string
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(CopyButton("abc123").Label("Copy link").Copied("Link copied").
			OnCopy(func(s string) { copied = s }).Key("copy"))
	})
	accesstest.Find(t, r.tree(), access.RoleButton, "Copy link")
	takeAnnouncements()

	// The press and release run the click without a frame, which publishes the tree
	// and takes the announcements.
	rc, _ := r.rt.Find("copy")
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	announced := takeAnnouncements()
	r.frame()
	if s, _ := r.rt.clipboard(); s != "abc123" {
		t.Errorf("clipboard %q, want abc123", s)
	}
	if copied != "abc123" {
		t.Errorf("OnCopy got %q, want abc123", copied)
	}
	accesstest.Find(t, r.tree(), access.RoleButton, "Link copied")
	if len(r.rt.toasts) != 0 {
		t.Errorf("a copy button shows %d toasts by default, want none", len(r.rt.toasts))
	}
	var said []access.Announcement
	for _, a := range announced {
		if a.Text == "Link copied" {
			said = append(said, a)
		}
	}
	if len(said) != 1 || said[0].Politeness != access.Polite {
		t.Errorf("announcements %v, want one polite \"Link copied\"", said)
	}

	r.now = r.now.Add(copyFeedbackDelay)
	r.frame()
	accesstest.Find(t, r.tree(), access.RoleButton, "Copy link")
}

// TestCopyButtonValueIsReadAtTheClick checks that Value computes the text when the
// button is pressed, not when it is made, and that an icon button keeps its name.
func TestCopyButtonValueIsReadAtTheClick(t *testing.T) {
	n := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(CopyButton("").Value(func() string {
			n++
			return "value " + strconv.Itoa(n)
		}).Key("copy"))
	})
	accesstest.Find(t, r.tree(), access.RoleButton, "Copy")
	if n != 0 {
		t.Fatalf("Value ran %d times before a click", n)
	}
	clickKey(t, r, "copy")
	if s, _ := r.rt.clipboard(); s != "value 1" {
		t.Errorf("clipboard %q, want the value read at the click", s)
	}
	accesstest.Find(t, r.tree(), access.RoleButton, "Copy")
}

// TestCopyButtonToast checks that Toast(true) also shows a toast in the window's
// Toaster.
func TestCopyButtonToast(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().
			Child(CopyButton("x").Label("Copy").Toast(true).Key("copy")).
			Child(Toaster().Key("toaster"))
	})
	clickKey(t, r, "copy")
	if len(r.rt.toasts) != 1 {
		t.Errorf("%d toasts after a copy with Toast(true), want 1", len(r.rt.toasts))
	}
}
