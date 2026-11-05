package app

import (
	"errors"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/ui"
)

// fakeWin is the platform window of a headless host: the methods the close
// uses, and nothing else.
type fakeWin struct {
	platform.Window
	closes, focuses int
	box             func(dialog.MessageOptions, func(dialog.Button))
}

func (w *fakeWin) Close() { w.closes++ }
func (w *fakeWin) Focus() { w.focuses++ }
func (w *fakeWin) ShowMessageBox(o dialog.MessageOptions, done func(dialog.Button)) {
	w.box(o, done)
}

// doc is a document window's unsaved state.
type doc struct {
	dirty   bool
	saves   int
	failing error // what Save returns, once
}

func (d *doc) unsaved(name string, canSave bool) func() *Changes {
	return func() *Changes {
		if !d.dirty {
			return nil
		}
		c := &Changes{Name: name}
		if canSave {
			c.Save = func() error {
				d.saves++
				if err := d.failing; err != nil {
					d.failing = nil
					return err
				}
				d.dirty = false
				return nil
			}
		}
		return c
	}
}

// guardFixture is a set of headless windows and the UI thread's queue.
type guardFixture struct {
	t     *testing.T
	posts chan func()
}

func newGuardFixture(t *testing.T) *guardFixture {
	f := &guardFixture{t: t, posts: make(chan func(), 8)}
	old := quitPost
	quitPost = func(fn func()) { f.posts <- fn }
	t.Cleanup(func() {
		quitPost = old
		application.windows = nil
		application.closing = nil
	})
	return f
}

func (f *guardFixture) window(title string, unsaved func() *Changes, root func(*ui.Context) ui.Element) *host {
	if root == nil {
		root = func(*ui.Context) ui.Element { return ui.Div().Child(ui.Text(title)) }
	}
	h := &host{cfg: Window{Title: title, Unsaved: unsaved, Strings: ui.EnglishStrings()}, win: &fakeWin{}}
	h.rt = ui.NewRuntime(root, nil, nil)
	h.rt.SetViewport(500, 400, 1)
	h.prompt = ui.NewSignal[*closePrompt](nil)
	h.rt.SetHostLayer(h.closeLayer)
	h.rt.Frame(time.Now())
	application.windows = append(application.windows, h)
	return h
}

// prompt returns the title of h's open prompt, or "".
func (f *guardFixture) prompt(h *host) string {
	h.rt.Frame(time.Now())
	title := ""
	h.rt.AccessTree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleAlertDialog {
			title = n.Name
		}
		return true
	})
	return title
}

// press clicks the prompt's button with the name, as the user does.
func (f *guardFixture) press(h *host, name string) {
	f.t.Helper()
	h.rt.Frame(time.Now())
	var b *access.Node
	h.rt.AccessTree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleButton && n.Name == name {
			b = n
		}
		return b == nil
	})
	if b == nil {
		f.t.Fatalf("%s has no %q button", h.cfg.Title, name)
	}
	x, y := b.Bounds.X+b.Bounds.W/2, b.Bounds.Y+b.Bounds.H/2
	h.inputSeq++ // as OnEvent counts the press
	h.rt.PointerMove(x, y, 0)
	h.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	h.rt.PointerUp(x, y, input.ButtonLeft, 0)
	h.rt.Frame(time.Now())
}

// settle runs what the saves posted to the UI thread.
func (f *guardFixture) settle() {
	for {
		select {
		case fn := <-f.posts:
			fn()
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

func TestGuardAsksEachWindowInTurn(t *testing.T) {
	f := newGuardFixture(t)
	report, scratch := &doc{dirty: true}, &doc{dirty: true}
	a := f.window("A", report.unsaved("Report.txt", true), nil)
	b := f.window("B", scratch.unsaved("", false), nil)
	clean := f.window("C", (&doc{}).unsaved("x", true), nil)
	closed := 0
	if !guardClose([]*host{a, b, clean}, func() { closed++ }) {
		t.Fatal("guardClose found nothing unsaved")
	}
	if got := f.prompt(a); got != "Save changes to “Report.txt”?" {
		t.Fatalf("A's prompt: %q", got)
	}
	if f.prompt(b) != "" || a.win.(*fakeWin).focuses != 1 {
		t.Fatal("B was asked before A, or A did not come to the front")
	}

	f.press(a, "Save")
	f.settle()
	if report.saves != 1 || f.prompt(a) != "" {
		t.Fatalf("A saved %d times; its prompt %q", report.saves, f.prompt(a))
	}
	if got := f.prompt(b); got != "Discard your changes?" {
		t.Fatalf("B's prompt: %q", got)
	}
	if closed != 0 {
		t.Fatal("windows closed before every one was answered")
	}
	f.press(b, "Discard")
	if closed != 1 || application.closing != nil {
		t.Fatalf("closed %d times; closing %v", closed, application.closing)
	}

	// Nothing unsaved: guardClose asks nothing.
	if guardClose([]*host{a, clean}, func() { t.Error("done ran for a close that asks nothing") }) {
		t.Error("guardClose asked about a clean window")
	}
}

func TestGuardCancelKeepsEverything(t *testing.T) {
	f := newGuardFixture(t)
	a := f.window("A", (&doc{dirty: true}).unsaved("a", true), nil)
	b := f.window("B", (&doc{dirty: true}).unsaved("b", true), nil)
	guardClose([]*host{a, b}, func() { t.Error("closed after Cancel") })
	f.press(a, "Don't save")
	f.press(b, "Cancel")
	if application.closing != nil || f.prompt(a) != "" || f.prompt(b) != "" {
		t.Fatal("the close went on after Cancel")
	}
	// A second close while one is asking brings its prompt forward.
	guardClose([]*host{a}, func() {})
	if !guardClose([]*host{b}, func() { t.Error("the second close ran") }) || a.win.(*fakeWin).focuses < 2 {
		t.Error("a second close did not defer to the first")
	}
}

func TestGuardFailedSaveAndStaleAnswers(t *testing.T) {
	f := newGuardFixture(t)
	da := &doc{dirty: true, failing: errors.New("disk full")}
	db := &doc{dirty: true}
	a := f.window("A", da.unsaved("a.txt", true), nil)
	b := f.window("B", db.unsaved("b.txt", true), nil)
	closed := 0
	guardClose([]*host{a, b}, func() { closed++ })

	f.press(a, "Save")
	f.settle()
	if got := f.prompt(a); got != "Save changes to “a.txt”?" || !f.hasButton(a, "Try again") {
		t.Fatalf("after a failed save: prompt %q, Try again %v", got, f.hasButton(a, "Try again"))
	}
	f.press(a, "Don't save")
	// A is edited after its answer, while B is asked: A is asked again.
	a.inputSeq++
	f.press(b, "Save")
	f.settle()
	if closed != 0 || f.prompt(a) == "" {
		t.Fatalf("A, edited after Don't save, was not asked again (closed %d)", closed)
	}
	f.press(a, "Save") // asked again, with a fresh prompt
	f.settle()
	if closed != 1 || da.dirty {
		t.Fatalf("closed %d times; A dirty %v", closed, da.dirty)
	}
}

func (f *guardFixture) hasButton(h *host, name string) bool {
	found := false
	h.rt.AccessTree().Walk(func(n *access.Node) bool {
		found = found || (n.Role == access.RoleButton && n.Name == name)
		return !found
	})
	return found
}

// A window whose root is not a Div asks with the system's message box.
func TestGuardMessageBoxWithoutHostLayer(t *testing.T) {
	f := newGuardFixture(t)
	d := &doc{dirty: true}
	h := f.window("Plain", d.unsaved("plain.txt", true), func(*ui.Context) ui.Element { return ui.Text("hi") })
	var asked dialog.MessageOptions
	h.win.(*fakeWin).box = func(o dialog.MessageOptions, done func(dialog.Button)) { asked = o; done(dialog.No) }
	closed := 0
	guardClose([]*host{h}, func() { closed++ })
	if asked.Buttons != dialog.ButtonsYesNoCancel || closed != 1 {
		t.Fatalf("message box %+v; closed %d", asked, closed)
	}
	// Cancel, answered at once, keeps it open: the caller must not close it.
	d.dirty = true
	h.win.(*fakeWin).box = func(o dialog.MessageOptions, done func(dialog.Button)) { done(dialog.Cancel) }
	if !guardClose([]*host{h}, func() { t.Error("closed after Cancel") }) {
		t.Error("guardClose told the caller to close after Cancel")
	}
}

// The window's own close asks, and the close that follows the answer goes
// through without asking again.
func TestGuardOnClose(t *testing.T) {
	f := newGuardFixture(t)
	h := f.window("Doc", (&doc{dirty: true}).unsaved("d", true), nil)
	w := h.win.(*fakeWin)
	if h.OnClose(w) {
		t.Fatal("a window with unsaved changes closed at once")
	}
	f.press(h, "Don't save")
	if w.closes != 1 || !h.approved {
		t.Fatalf("the answer did not close the window (closes %d, approved %v)", w.closes, h.approved)
	}
	if !h.OnClose(w) {
		t.Fatal("the approved close was refused")
	}
}
