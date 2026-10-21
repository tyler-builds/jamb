package views

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// clickOK clicks the element with key and lets the frame run, failing the test when
// the element is not on the screen.
func clickOK(t *testing.T, r *demoRig, key string) {
	t.Helper()
	if err := r.rt.ClickKey(key); err != nil {
		t.Fatal(err)
	}
	r.frame()
}

// hasText reports whether the access tree has a static text that starts with prefix.
func showComponents(r *demoRig) {
	r.s.Page.Set(PageComponents)
	// The page is taller than the test window: a click on an element below the window
	// does nothing, so the window is made tall enough to show the whole page.
	r.rt.SetViewport(1200, 9000, 1)
	r.frame()
}

func hasText(r *demoRig, prefix string) bool {
	found := false
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleStaticText && strings.HasPrefix(n.Name, prefix) {
			found = true
		}
		return true
	})
	return found
}

// A blank name is refused with the field's error, and focus moves to the name. The
// form stays open and adds no project.
func TestNewProjectBlankNameShowsItsErrorAndKeepsFocus(t *testing.T) {
	r := newDemoRig(t)
	before := len(r.s.Projects.Peek())
	r.s.OpenNewProject()
	r.frame()
	clickOK(t, r, "project-create")
	if len(r.s.Projects.Peek()) != before || !r.s.NewOpen.Peek() {
		t.Fatal("a blank name created a project or closed the form")
	}
	tr := r.tree()
	name := accesstest.Find(t, tr, access.RoleTextField, "Name")
	accesstest.Check(t, name, access.RoleTextField, "Name", access.StateInvalid|access.StateRequired)
	if name.Description != "This field is required." {
		t.Fatalf("the name's description is %q, want the error", name.Description)
	}
	accesstest.Focused(t, tr, access.RoleTextField, "Name")
	if !hasText(r, "1 field needs attention.") {
		t.Fatal("the summary does not show the count")
	}
}

// The channels of the preferences need one channel on. Saving with none reports the
// group, and checking one clears the report.
func TestComponentsPreferencesNeedAChannel(t *testing.T) {
	r := newDemoRig(t)
	showComponents(r)
	clickOK(t, r, "form-email")
	clickOK(t, r, "form-push")
	clickOK(t, r, "prefs-save")
	group := accesstest.Find(t, r.tree(), access.RoleGroup, "Send alerts by")
	if !group.States.Has(access.StateInvalid) {
		t.Fatalf("the channels group has states [%s], want invalid", group.States)
	}
	if !hasText(r, "Choose at least one channel.") {
		t.Fatal("the channels error is not shown")
	}
	clickOK(t, r, "form-text")
	if hasText(r, "Choose at least one channel.") {
		t.Fatal("the channels error stays after a channel is on")
	}
}

// The Forms sample shows its summary on an empty save, and the simulated server
// refuses the name "taken" as the form's own message, after the save has run in the
// background.
func TestFormsSectionShowsSummaryAndServerRefusal(t *testing.T) {
	r := newDemoRig(t)
	showComponents(r)
	clickOK(t, r, "forms-save")
	if !hasText(r, "1 field needs attention.") {
		t.Fatal("an empty save does not show the summary")
	}
	clickOK(t, r, "forms-name")
	r.rt.Type("taken")
	r.frame()
	clickOK(t, r, "forms-save")
	deadline := time.Now().Add(10 * time.Second)
	for !hasText(r, "That name is already in use.") {
		if time.Now().After(deadline) {
			t.Fatal("the server's refusal is not shown")
		}
		r.frame()
		time.Sleep(20 * time.Millisecond)
	}
}
