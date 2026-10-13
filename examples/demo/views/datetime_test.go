package views

import (
	"testing"
	"time"
)

// TestCreateProjectKeepsTheDueDate checks that the New Project form's due date is the project's,
// and that a project created without one has none.
func TestCreateProjectKeepsTheDueDate(t *testing.T) {
	s := NewStore()
	due := time.Date(2026, time.November, 2, 0, 0, 0, 0, time.Local)
	id := s.CreateProject("Dated", "", due)
	if got := projectByID(s, id).Due; !got.Equal(due) {
		t.Fatalf("the due date is %v, want %v", got, due)
	}
	none := s.CreateProject("Undated", "", time.Time{})
	if !projectByID(s, none).Due.IsZero() {
		t.Fatal("a project created without a due date has one")
	}
	if dueText(time.Time{}) != "None" || dueText(due) != "Nov 2, 2026" {
		t.Fatalf("the Due column reads %q and %q", dueText(time.Time{}), dueText(due))
	}
}

// projectByID returns the project with the ID, from the store's list.
func projectByID(s *Store, id int) Project {
	for _, p := range s.Projects.Peek() {
		if p.ID == id {
			return p
		}
	}
	return Project{}
}
