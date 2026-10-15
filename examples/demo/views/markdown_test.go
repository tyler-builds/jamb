package views

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// The Markdown page shows its document, and a click on a task's box marks the task
// done in the text, which the page shows again.
func TestMarkdownPageTogglesATask(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageMarkdown)
	u := uitest.New(t, App(s), uitest.Size(1200, 760), uitest.Theme(DemoDarkTheme))
	if h := u.ByRole(access.RoleHeading, "Markdown").Node(); h.Heading != 1 {
		t.Errorf("the title is a heading of level %d, want 1", h.Heading)
	}
	u.ByRole(access.RoleGroup, "Table, 3 columns, 4 rows")
	u.ByRole(access.RoleGroup, "Code, go")
	task := u.ByRole(access.RoleCheckBox, "A task to do (click its box)")
	if task.Checked() {
		t.Fatal("the task is done before its box is clicked")
	}
	task.Perform(access.ActionClick)
	if !u.ByRole(access.RoleCheckBox, "A task to do (click its box)").Checked() {
		t.Error("the task is not done after a click on its box")
	}
}

// toggleTask changes the mark of the task it is asked for, and no other.
func TestToggleTask(t *testing.T) {
	src := "- [x] one\n- [ ] two\n  1. [ ] three\nNot a task: [ ]\n"
	for _, c := range []struct {
		i    int
		done bool
		want string
	}{
		{1, true, "- [x] one\n- [x] two\n  1. [ ] three\nNot a task: [ ]\n"},
		{0, false, "- [ ] one\n- [ ] two\n  1. [ ] three\nNot a task: [ ]\n"},
		{2, true, "- [x] one\n- [ ] two\n  1. [x] three\nNot a task: [ ]\n"},
		{3, true, src},
	} {
		if got := toggleTask(src, c.i, c.done); got != c.want {
			t.Errorf("toggleTask(%d, %v) = %q, want %q", c.i, c.done, got, c.want)
		}
	}
}
