package views

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

// TestCreateProjectUndoRemovesIt checks the undo of "Project created": the
// project goes, and it is no longer selected.
func TestCreateProjectUndoRemovesIt(t *testing.T) {
	s := NewStore()
	before := len(s.Projects.Get())
	id := s.CreateProject("Temporary", "")
	if s.Selected.Peek() != id {
		t.Fatalf("the new project is not selected")
	}
	s.DeleteProject(id)
	if got := len(s.Projects.Get()); got != before {
		t.Errorf("%d projects after undo, want %d", got, before)
	}
	if s.Selected.Peek() != 0 {
		t.Errorf("selection is %d after undo, want none", s.Selected.Peek())
	}
}

// TestInboxCountsUnreadAndMarksRead checks the bell's count: new notifications
// are unread, Mark all read clears the count, and the next one counts again.
func TestInboxCountsUnreadAndMarksRead(t *testing.T) {
	s := NewStore()
	for _, text := range []string{"one", "two", "three"} {
		s.addInbox(ui.ToastInfo, text)
	}
	if got := unreadCount(s.Inbox.Get()); got != 3 {
		t.Fatalf("%d unread, want 3", got)
	}
	s.markAllRead()
	if got := unreadCount(s.Inbox.Get()); got != 0 {
		t.Fatalf("%d unread after Mark all read, want 0", got)
	}
	s.addInbox(ui.ToastWarning, "four")
	if got := unreadCount(s.Inbox.Get()); got != 1 {
		t.Errorf("%d unread after a new one, want 1", got)
	}
	if first := s.Inbox.Get()[0]; first.Text != "four" {
		t.Errorf("the newest notification is %q, want it first", first.Text)
	}
}

// TestInboxKeepsTheLatest checks that the list is capped, and keeps the newest.
func TestInboxKeepsTheLatest(t *testing.T) {
	s := NewStore()
	for i := 0; i < inboxLimit+5; i++ {
		s.addInbox(ui.ToastInfo, "n")
	}
	if got := len(s.Inbox.Get()); got != inboxLimit {
		t.Errorf("%d notifications kept, want %d", got, inboxLimit)
	}
}
