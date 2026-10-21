package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// closeTest is a window with an opener and a dialog that build makes, shown while
// open is set. It counts what the dialog's OnClose did.
type closeTest struct {
	r      *accessRig
	open   *Signal[bool]
	closes int
}

func newCloseTest(build func(done func()) Element) *closeTest {
	c := &closeTest{open: NewSignal(false)}
	c.r = newAccessRig(func(ctx *Context) Element {
		page := Div().Size(Full, Full).Column().Child(Button("Open").Key("opener"))
		if !c.open.Get() {
			return page
		}
		return page.Child(build(func() { c.closes++; c.open.Set(false) }))
	})
	return c
}

// show focuses the opener, opens the dialog and builds it. Then it settles the
// dialog's entrance, so a press reaches it.
func (c *closeTest) show(t *testing.T) {
	t.Helper()
	c.r.focusKey(t, "opener")
	c.open.Set(true)
	for range 12 {
		c.r.frame()
	}
}

// TestModalCloseButtonIsOffByDefault checks that a modal with an OnClose has no
// close button until CloseButton shows one.
func TestModalCloseButtonIsOffByDefault(t *testing.T) {
	c := newCloseTest(func(done func()) Element {
		return Modal("Settings", Text("Body")).OnClose(done)
	})
	c.show(t)
	if partNodeOrNil(c.r.rt, "Modal.Close") != nil {
		t.Fatal("a modal without CloseButton has a close button")
	}
	c.r.stateOf(t, access.RoleDialog, "Settings")
	accesstest.Absent(t, c.r.tree(), access.RoleButton, englishStrings().Close)
}

// TestModalCloseButtonNeedsOnClose checks that the close button is not built
// without an OnClose to run.
func TestModalCloseButtonNeedsOnClose(t *testing.T) {
	c := newCloseTest(func(done func()) Element {
		return Modal("Settings", Text("Body")).CloseButton(true)
	})
	c.show(t)
	if partNodeOrNil(c.r.rt, "Modal.Close") != nil || partNodeOrNil(c.r.rt, "Modal.Header") != nil {
		t.Fatal("a modal without OnClose has a close button")
	}
}

// TestModalCloseButtonPressCloses checks that a press on the close button runs
// OnClose, and that Enter and Space on it do too.
func TestModalCloseButtonPressCloses(t *testing.T) {
	build := func(done func()) Element {
		return Modal("Settings", Text("Body")).CloseButton(true).OnClose(done)
	}
	c := newCloseTest(build)
	c.show(t)
	c.r.clickKey(t, "jamb.modal/close")
	if c.closes != 1 || c.open.Peek() {
		t.Fatalf("a press closed %d times, open %v", c.closes, c.open.Peek())
	}

	for _, k := range []input.Key{input.KeyEnter, input.KeySpace} {
		c = newCloseTest(build)
		c.show(t)
		c.r.focusAccess(t, access.RoleButton, englishStrings().Close)
		c.r.press(k, 0)
		if c.closes != 1 || c.open.Peek() {
			t.Fatalf("key %v on the close button closed %d times, open %v", k, c.closes, c.open.Peek())
		}
	}
}

// TestModalCloseButtonKeyIsScopedByTheModal checks that the close button's key
// follows the modal's key, so two modals do not share one.
func TestModalCloseButtonKeyIsScopedByTheModal(t *testing.T) {
	c := newCloseTest(func(done func()) Element {
		return Modal("Settings", Text("Body")).Key("prefs").CloseButton(true).OnClose(done)
	})
	c.show(t)
	c.r.clickKey(t, "prefs/close")
	if c.closes != 1 {
		t.Fatalf("a press on prefs/close closed %d times, want 1", c.closes)
	}
}

// TestModalCloseButtonKeepsInitialFocus checks that the close button, which is
// first in the Tab order, does not take the initial focus: the first field does.
func TestModalCloseButtonKeepsInitialFocus(t *testing.T) {
	c := newCloseTest(func(done func()) Element {
		return Modal("Settings", TextInput(NewSignal("")).Label("Name").Key("name")).
			CloseButton(true).OnClose(done)
	})
	c.show(t)
	if got := c.r.focusedKey(); got != "name" {
		t.Fatalf("initial focus is on %q, want the field", got)
	}
}

// TestDrawerCloseButtonOnEachSide checks that a drawer shows its close button at
// every side, and that a press on it closes the drawer.
func TestDrawerCloseButtonOnEachSide(t *testing.T) {
	for _, side := range []DrawerSide{DrawerRight, DrawerLeft, DrawerTop, DrawerBottom} {
		c := newCloseTest(func(done func()) Element {
			return Drawer("Settings", Text("Body")).Side(side).CloseButton(true).OnClose(done)
		})
		c.show(t)
		if partNodeOrNil(c.r.rt, "Drawer.Close") == nil {
			t.Errorf("side %d: the drawer has no close button", side)
			continue
		}
		c.r.clickKey(t, "jamb.modal/close")
		if c.closes != 1 {
			t.Errorf("side %d: a press closed the drawer %d times, want 1", side, c.closes)
		}
	}
}

// TestAlertDialogHasNoCloseButton checks that an alert dialog, which asks for an
// answer, has no close button.
func TestAlertDialogHasNoCloseButton(t *testing.T) {
	c := newCloseTest(func(done func()) Element {
		return AlertDialog("Saved", "The file is saved.").Confirm("OK", done)
	})
	c.show(t)
	for _, part := range []string{"AlertDialog.Close", "Modal.Close", "Drawer.Close"} {
		if partNodeOrNil(c.r.rt, part) != nil {
			t.Errorf("an alert dialog has a %s part", part)
		}
	}
	accesstest.Absent(t, c.r.tree(), access.RoleButton, englishStrings().Close)
}

// TestCloseButtonIsLocalised checks that the close button is named in the
// language of the strings.
func TestCloseButtonIsLocalised(t *testing.T) {
	for _, loc := range []string{"it", "zh-CN"} {
		c := newCloseTest(func(done func()) Element {
			return Modal("Impostazioni", Text("Body")).CloseButton(true).OnClose(done)
		})
		c.r.rt.SetStrings(StringsFor(loc))
		c.show(t)
		want := StringsFor(loc).Close
		if want == englishStrings().Close {
			t.Fatalf("%s: the close name is the English one", loc)
		}
		c.r.stateOf(t, access.RoleButton, want)
	}
}
