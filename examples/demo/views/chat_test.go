package views

import (
	"fmt"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// chatRig is the window of one project, drawn at its usual size.
type chatRig struct {
	t   *testing.T
	s   *Store
	id  int
	rt  *ui.Runtime
	now time.Time
}

func newChatRig(t *testing.T, s *Store, id int) *chatRig {
	t.Helper()
	c := &chatRig{t: t, s: s, id: id, now: time.Unix(1_000_000, 0)}
	c.rt = ui.NewRuntime(ProjectDetail(s, id), DemoDarkTheme, nil)
	c.rt.SetViewport(680, 640, 1)
	c.frame()
	// The thread is in the Comments tab, which the window opens on Overview.
	if err := c.rt.ClickKey("detail-tabs/tab:comments"); err != nil {
		t.Fatal(err)
	}
	c.frame()
	return c
}

func (c *chatRig) frame() {
	c.now = c.now.Add(16 * time.Millisecond)
	c.rt.Frame(c.now)
}

// settle runs frames until any scroll has finished.
func (c *chatRig) settle() {
	for range 300 {
		c.frame()
	}
}

// rowKey is the key of a message's row in the thread list.
func rowKey(m Message) string { return fmt.Sprintf("jamb.vrow:%d", m.ID) }

// thread is the project's messages.
func (c *chatRig) thread() []Message { return c.s.Threads.Peek()[c.id] }

// scrollUp moves the thread 400 px toward its start.
func (c *chatRig) scrollUp() {
	list, ok := c.rt.Find("thread")
	if !ok {
		c.t.Fatal("no thread list")
	}
	c.rt.Scroll(list.X+list.W/2, list.Y+list.H/2, 0, -400, true, 0)
	c.settle()
}

// The thread opens at its latest message, and no jump button shows.
func TestChatStartsAtTheLatestMessage(t *testing.T) {
	s := NewStore()
	c := newChatRig(t, s, s.Projects.Peek()[0].ID)
	c.settle()
	if _, ok := c.rt.Find("jump-latest"); ok {
		t.Error("Jump to latest shows while the thread is at its end")
	}
	last, ok := c.rt.Find(rowKey(c.thread()[len(c.thread())-1]))
	list, _ := c.rt.Find("thread")
	if !ok || last.Y < list.Y || last.Y+last.H > list.Y+list.H {
		t.Errorf("the latest message is not in view (row %+v, list %+v)", last, list)
	}
}

// Scrolling up shows Jump to latest; clicking it returns to the end and hides
// the button again.
func TestChatJumpToLatestAppearsWhenScrolledUp(t *testing.T) {
	s := NewStore()
	c := newChatRig(t, s, s.Projects.Peek()[0].ID)
	c.scrollUp()
	if _, ok := c.rt.Find("jump-latest"); !ok {
		t.Fatal("Jump to latest is not shown after scrolling up")
	}
	if err := c.rt.ClickKey("jump-latest"); err != nil {
		t.Fatal(err)
	}
	c.settle()
	if _, ok := c.rt.Find("jump-latest"); ok {
		t.Error("Jump to latest is still shown after jumping to the end")
	}
	if _, ok := c.rt.Find(rowKey(c.thread()[len(c.thread())-1])); !ok {
		t.Error("the latest message is not built after Jump to latest")
	}
}

// A message that arrives while the user reads older messages does not move
// the view.
func TestChatDoesNotMoveWhenScrolledUp(t *testing.T) {
	s := NewStore()
	c := newChatRig(t, s, s.Projects.Peek()[0].ID)
	c.scrollUp()
	n := len(c.thread())
	anchor := c.thread()[n-3]
	before, ok := c.rt.Find(rowKey(anchor))
	if !ok {
		t.Fatal("the message to watch is not in view")
	}
	s.Comment(c.id, teammate, "A new message while reading history.")
	c.frame()
	after, ok := c.rt.Find(rowKey(anchor))
	if !ok || after.Y != before.Y {
		t.Errorf("the message moved from %v to %v when a new one arrived", before.Y, after.Y)
	}
	if _, ok := c.rt.Find("jump-latest"); !ok {
		t.Error("Jump to latest is hidden although the thread is not at its end")
	}
}

// A message that arrives while the thread is at its end is shown.
func TestChatFollowsTheEndWhenAtIt(t *testing.T) {
	s := NewStore()
	c := newChatRig(t, s, s.Projects.Peek()[0].ID)
	c.settle()
	m := s.Comment(c.id, me, "Following the thread.")
	c.settle()
	row, ok := c.rt.Find(rowKey(m))
	list, _ := c.rt.Find("thread")
	if !ok || row.Y < list.Y || row.Y+row.H > list.Y+list.H {
		t.Errorf("the new message is not in view (row %+v, list %+v)", row, list)
	}
	if _, ok := c.rt.Find("jump-latest"); ok {
		t.Error("Jump to latest shows after a message arrived at the end")
	}
}

// Enter sends the comment, and a teammate answers from a goroutine through
// Context.Post, on the UI thread.
func TestChatSendAndTeammateReply(t *testing.T) {
	saved := replyDelay
	replyDelay = 0
	t.Cleanup(func() { replyDelay = saved })

	s := NewStore()
	c := newChatRig(t, s, s.Projects.Peek()[0].ID)
	before := len(c.thread())
	if err := c.rt.ClickKey("composer"); err != nil {
		t.Fatal(err)
	}
	c.rt.Type("Looks good")
	c.rt.Press(input.KeyEnter, 0)
	if got := c.thread(); len(got) != before+1 || got[before].Author != me || got[before].Body != "Looks good" {
		t.Fatalf("after Enter the thread has %d messages, want %d with the comment last", len(got), before+1)
	}

	for i := 0; i < 500 && len(c.thread()) < before+2; i++ {
		time.Sleep(5 * time.Millisecond)
		c.frame()
	}
	got := c.thread()
	if len(got) != before+2 {
		t.Fatalf("the teammate's reply did not arrive: %d messages, want %d", len(got), before+2)
	}
	if reply := got[before+1]; reply.Author != teammate || reply.Body == "" {
		t.Errorf("the reply is from %q (%q), want a teammate", reply.Author, reply.Body)
	}
	c.settle()
	if _, ok := c.rt.Find("jump-latest"); ok {
		t.Error("the thread is not at its end after the reply")
	}
}

// Renaming in the window changes the project everywhere, and an empty name is
// not saved.
func TestRenameInTheWindow(t *testing.T) {
	s := NewStore()
	id := s.Projects.Peek()[0].ID
	c := newChatRig(t, s, id)
	if err := c.rt.ClickKey("name"); err != nil {
		t.Fatal(err)
	}
	c.rt.Press(input.KeyA, input.ModCtrl)
	c.rt.Type("Renamed")
	if p, _ := s.Project(id); p.Name != "Renamed" {
		t.Errorf("the project is named %q, want Renamed", p.Name)
	}
	c.rt.Press(input.KeyA, input.ModCtrl)
	c.rt.Press(input.KeyBackspace, 0)
	if p, _ := s.Project(id); p.Name != "Renamed" {
		t.Errorf("an empty name was saved as %q", p.Name)
	}
}

// Every project's thread has 150 to 300 messages, with bodies of different
// lengths, so the rows vary in height.
func TestSeededThreadsVary(t *testing.T) {
	s := NewStore()
	for _, p := range s.Projects.Peek() {
		msgs := s.Threads.Peek()[p.ID]
		if len(msgs) < 150 || len(msgs) > 300 {
			t.Errorf("%s has %d messages, want 150 to 300", p.Name, len(msgs))
		}
		lengths := map[int]bool{}
		for _, m := range msgs {
			lengths[len(m.Body)] = true
		}
		if len(lengths) < 10 {
			t.Errorf("%s has only %d distinct message lengths", p.Name, len(lengths))
		}
	}
}
