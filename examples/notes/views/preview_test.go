package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
)

// Ctrl+Shift+V shows the Markdown preview of the note beside the editor, with the
// note's text at once, and the toolbar's toggle button hides it again. The note keeps
// the text as it was typed.
func TestPreviewToggles(t *testing.T) {
	s := seededStore(t, 1)
	n := s.Notes.Peek()[0]
	s.Selected.Set(n.ID)
	n.Body.Set("# Plan\n\n- [x] Book the train\n- [ ] Pack")
	u := newUI(t, s)

	if u.Has("preview") {
		t.Fatal("the preview is shown before it is turned on")
	}
	toggle := u.Get("preview-toggle")
	if toggle.Checked() {
		t.Fatal("the preview button is on before the preview is shown")
	}
	u.Press("ctrl+shift+v")
	if !u.Has("preview") || !toggle.Checked() {
		t.Fatal("Ctrl+Shift+V did not show the preview")
	}
	u.ByRole(access.RoleGroup, "Preview")
	u.ByRole(access.RoleHeading, "Plan")
	if !u.ByRole(access.RoleCheckBox, "Book the train").Checked() || u.ByRole(access.RoleCheckBox, "Pack").Checked() {
		t.Error("the tasks of the preview do not show their states")
	}
	toggle.Click()
	if u.Has("preview") {
		t.Fatal("the preview button did not hide the preview")
	}
	if n.Body.Peek() != "# Plan\n\n- [x] Book the train\n- [ ] Pack" {
		t.Errorf("the note holds %q", n.Body.Peek())
	}
}

// While the preview is open, typing updates it after a pause: during a burst of keys
// it shows the text from before the burst, and previewDelay after the last key it
// shows the text as typed. Another note shows at once.
func TestPreviewFollowsTypingAfterAPause(t *testing.T) {
	s := seededStore(t, 2)
	notes := s.Notes.Peek()
	n, other := notes[0], notes[1]
	s.Selected.Set(n.ID)
	n.Body.Set("# Plan")
	other.Body.Set("# Other note")
	u := newUI(t, s)
	u.Press("ctrl+shift+v")
	u.ByRole(access.RoleHeading, "Plan")

	body := u.ByRole(access.RoleTextArea, "Body")
	body.Focus()
	u.Press("ctrl+end")
	for _, key := range []string{"\n", "\n", "#", "#", " ", "D", "o", "n", "e"} {
		u.Type(key)
		u.Advance(previewDelay / 3) // typing faster than the pause
		if u.Tree().Find(access.RoleHeading, "Done") != nil {
			t.Fatalf("the preview showed the new heading during the burst, after %q", key)
		}
	}
	if n.Body.Peek() != "# Plan\n\n## Done" {
		t.Fatalf("the note holds %q", n.Body.Peek())
	}
	u.Advance(previewDelay + 20*time.Millisecond)
	u.ByRole(access.RoleHeading, "Done")

	s.Selected.Set(other.ID)
	u.Frame()
	u.ByRole(access.RoleHeading, "Other note")
}
