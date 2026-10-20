package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// alertTest is a window with an opener, and an alert dialog that is shown while
// open is set and that build makes. It records the answers.
type alertTest struct {
	r       *accessRig
	open    *Signal[bool]
	answers []string
}

func newAlertTest(build func(a *alertTest) *AlertDialogElement) *alertTest {
	a := &alertTest{open: NewSignal(false)}
	a.r = newAccessRig(func(ctx *Context) Element {
		page := Div().Size(Full, Full).Column().Child(Button("Open").Key("opener"))
		if !a.open.Get() {
			return page
		}
		return page.Child(build(a))
	})
	return a
}

// answer returns a function that records s and closes the dialog.
func (a *alertTest) answer(s string) func() {
	return func() {
		a.answers = append(a.answers, s)
		a.open.Set(false)
	}
}

// show focuses the opener, opens the dialog and builds it.
func (a *alertTest) show(t *testing.T) {
	t.Helper()
	a.r.focusKey(t, "opener")
	a.open.Set(true)
	a.r.frame()
	a.r.frame()
}

// deleteDialog is a destructive dialog with Cancel and an extra action.
func deleteDialog(a *alertTest) *AlertDialogElement {
	return AlertDialog("Delete 3 files?", "They cannot be restored.").Key("del").
		Intent(AlertDialogDestructive).
		Action(Button("Archive").Secondary().Key("archive").OnClick(a.answer("archive"))).
		Cancel("", a.answer("cancel")).
		Confirm("Delete", a.answer("delete"))
}

// The panel is an alert dialog named by its title and described by its message,
// and its buttons are in order: the actions, Cancel, then the confirm button.
func TestAlertDialogAccessTree(t *testing.T) {
	a := newAlertTest(deleteDialog)
	a.show(t)
	tree := a.r.tree()
	dlg := accesstest.Find(t, tree, access.RoleAlertDialog, "Delete 3 files?")
	if len(dlg.DescribedBy) != 1 {
		t.Fatalf("the dialog is described by %v", dlg.DescribedBy)
	}
	msg := tree.Node(dlg.DescribedBy[0])
	if msg == nil || msg.Name != "They cannot be restored." {
		t.Fatalf("the dialog is described by %+v, want its message", msg)
	}
	accesstest.Absent(t, tree, access.RoleDialog, "Delete 3 files?")
	var buttons []string
	tree.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleButton && n.Name != "Open" {
			buttons = append(buttons, n.Name)
		}
		return true
	})
	if got := strings.Join(buttons, ","); got != "Archive,Cancel,Delete" {
		t.Fatalf("the footer's buttons are %s, want Archive,Cancel,Delete", got)
	}
	if len(a.r.warnings) != 0 {
		t.Fatalf("warnings: %v", a.r.warnings)
	}
}

// A destructive dialog focuses Cancel, has a Danger confirm button, and Enter on
// Cancel cancels. Tab stays inside the dialog, and the focus returns to the
// opener when it closes.
func TestAlertDialogDestructive(t *testing.T) {
	a := newAlertTest(deleteDialog)
	a.show(t)
	if !a.r.focusedAccess(t, access.RoleButton, "Cancel") {
		t.Fatal("a destructive dialog does not open on Cancel")
	}
	danger := buttonDefaults(a.r.rt.theme, ButtonDanger, sizeDefault, false).Root.Background
	confirm := partNodes(a.r.rt, "Button.Root")
	found := false
	for _, n := range confirm {
		if n.div.acc != nil && n.style.Background == danger {
			found = true
		}
	}
	if !found {
		t.Fatal("the confirm button of a destructive dialog is not Danger")
	}
	var seen []string
	for range 4 {
		a.r.press(input.KeyTab, 0)
		seen = append(seen, a.r.stateOfFocused())
	}
	if got := strings.Join(seen, ","); got != "Delete,Archive,Cancel,Delete" {
		t.Fatalf("Tab visits %s, want Delete,Archive,Cancel,Delete", got)
	}
	a.r.press(input.KeyTab, input.ModShift)
	a.r.press(input.KeyEnter, 0)
	if strings.Join(a.answers, ",") != "cancel" {
		t.Fatalf("Enter on Cancel answered %v", a.answers)
	}
	if got := a.r.focusedKey(); got != "opener" {
		t.Fatalf("after closing, focus is on %q, want the opener", got)
	}
}

// stateOfFocused returns the name of the focused node of the published tree.
func (r *accessRig) stateOfFocused() string {
	tree := r.tree()
	if n := tree.Node(tree.Focus); n != nil {
		return n.Name
	}
	return ""
}

// Escape runs Cancel; a press on the backdrop does nothing.
func TestAlertDialogEscapeCancels(t *testing.T) {
	a := newAlertTest(deleteDialog)
	a.show(t)
	a.r.clickAt(5, 5)
	if len(a.answers) != 0 || !a.open.Peek() {
		t.Fatalf("a press on the backdrop answered %v", a.answers)
	}
	a.r.press(input.KeyEscape, 0)
	if strings.Join(a.answers, ",") != "cancel" {
		t.Fatalf("Escape answered %v, want cancel", a.answers)
	}
}

// An OK-only dialog confirms on Escape, opens on its OK button, and is named by
// the window's Strings.
func TestAlertDialogOKOnly(t *testing.T) {
	a := newAlertTest(func(a *alertTest) *AlertDialogElement {
		return AlertDialog("Saved", "The report was saved.").Confirm("", a.answer("ok"))
	})
	a.show(t)
	if !a.r.focusedAccess(t, access.RoleButton, "OK") {
		t.Fatal("an OK-only dialog does not open on OK")
	}
	a.r.press(input.KeyEscape, 0)
	if strings.Join(a.answers, ",") != "ok" {
		t.Fatalf("Escape answered %v, want ok", a.answers)
	}
}

// A Confirm dialog confirms on Enter from a field of its body, and not while the
// confirm button is loading.
func TestAlertDialogEnterConfirms(t *testing.T) {
	loading := NewSignal(false)
	name := NewSignal("")
	a := newAlertTest(func(a *alertTest) *AlertDialogElement {
		return AlertDialog("Rename", "Enter a new name.").Intent(AlertDialogConfirm).
			Body(TextInput(name).Key("name").Label("Name")).
			Cancel("", a.answer("cancel")).Confirm("Rename", a.answer("rename")).
			ConfirmLoading(loading.Get())
	})
	a.show(t)
	if !a.r.focusedAccess(t, access.RoleButton, "Rename") {
		t.Fatal("a Confirm dialog does not open on its confirm button")
	}
	a.r.focusKey(t, "name")
	loading.Set(true)
	a.r.frame()
	a.r.press(input.KeyEnter, 0)
	if len(a.answers) != 0 {
		t.Fatalf("Enter confirmed while the confirm button is loading: %v", a.answers)
	}
	loading.Set(false)
	a.r.frame()
	a.r.press(input.KeyEnter, 0)
	if strings.Join(a.answers, ",") != "rename" {
		t.Fatalf("Enter in the body answered %v, want rename", a.answers)
	}
}

// The tree reports the dialog's opening and closing, as it does a Modal's.
func TestAlertDialogOpensAndClosesInTheDiff(t *testing.T) {
	a := newAlertTest(deleteDialog)
	before := a.r.tree()
	a.show(t)
	opened := a.r.tree()
	u := access.Diff(before, opened)
	if len(u.Opened) != 1 || u.Opened[0].Role != access.RoleAlertDialog {
		t.Fatalf("opening reported %v", u.Opened)
	}
	a.open.Set(false)
	u = access.Diff(opened, a.r.tree())
	if len(u.Closed) != 1 || u.Closed[0].Role != access.RoleAlertDialog {
		t.Fatalf("closing reported %v", u.Closed)
	}
}

// The parts of an alert dialog take their styles.
func testAlertDialogParts(t *testing.T, applied map[string]bool) {
	cases := []struct {
		part string
		set  func(*AlertDialogStyle, Style)
	}{
		{"AlertDialog.Scrim", func(s *AlertDialogStyle, m Style) { s.Scrim = m }},
		{"AlertDialog.Panel", func(s *AlertDialogStyle, m Style) { s.Panel = m }},
		{"AlertDialog.Header", func(s *AlertDialogStyle, m Style) { s.Header = m }},
		{"AlertDialog.Icon", func(s *AlertDialogStyle, m Style) { s.Icon = m }},
		{"AlertDialog.Title", func(s *AlertDialogStyle, m Style) { s.Title = m }},
		{"AlertDialog.Message", func(s *AlertDialogStyle, m Style) { s.Message = m }},
		{"AlertDialog.Body", func(s *AlertDialogStyle, m Style) { s.Body = m }},
		{"AlertDialog.Footer", func(s *AlertDialogStyle, m Style) { s.Footer = m }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st AlertDialogStyle
			c.set(&st, Style{Background: markBG})
			r := newAccessRig(func(ctx *Context) Element { return partAlertDialog(st) })
			wantBackground(t, partNode(t, r.rt, c.part, ""), c.part, markBG)
		})
	}
}

// partAlertDialog is a warning dialog with Cancel, with the instance style st.
func partAlertDialog(st AlertDialogStyle) Element {
	return Div().Column().Child(AlertDialog("Discard changes?", "Your edits will be lost.").
		Intent(AlertDialogWarning).Cancel("", nil).Confirm("Discard", nil).Styles(st))
}

// Under high contrast the icon and the message are the text color.
func TestAlertDialogUnderHighContrast(t *testing.T) {
	th := HighContrastTheme(HighContrastBlack(), DarkTheme())
	r := newThemedRig(th, func(ctx *Context) Element { return partAlertDialog(AlertDialogStyle{}) })
	for _, part := range []string{"AlertDialog.Icon", "AlertDialog.Message"} {
		if c := partNode(t, r.rt, part, "").style.TextColor; c != th.Text {
			t.Errorf("%s is %v under high contrast, want the text color %v", part, c, th.Text)
		}
	}
}
