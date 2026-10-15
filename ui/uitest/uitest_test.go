package uitest_test

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// form is a small UI with the controls the tests drive.
type form struct {
	count    *ui.Signal[int]
	name     *ui.Signal[string]
	remember *ui.Signal[bool]
	more     *ui.Signal[bool]
	lit      *ui.Signal[bool]
	events   []string
}

func newForm() *form {
	return &form{count: ui.NewSignal(0), name: ui.NewSignal(""), remember: ui.NewSignal(false),
		more: ui.NewSignal(false), lit: ui.NewSignal(false)}
}

func (f *form) root(ctx *ui.Context) ui.Element {
	bg := ui.RGB(0, 0, 0)
	if f.lit.Get() {
		bg = ui.RGB(255, 255, 255)
	}
	rows := ui.Div().Column()
	for i := range 40 {
		rows.Child(ui.Div().Key(fmt.Sprintf("row-%d", i)).Height(ui.Px(30)).Child(ui.Text(fmt.Sprintf("Row %d", i))))
	}
	return ui.Div().Column().Gap(8).Padding(8).Children(
		ui.Div().Row().Gap(8).Children(
			ui.Button("Add").Key("add").Tooltip("Add one").OnClick(func() { f.count.Set(f.count.Peek() + 1) }),
			ui.Button("Locked").Key("locked").Disabled(true),
			ui.Div().Key("count").Child(ui.Text(fmt.Sprintf("Count: %d", f.count.Get()))),
		),
		ui.TextInput(f.name).Key("name").Label("Name"),
		ui.Checkbox("Remember me", f.remember).Key("remember"),
		ui.Collapsible("More", f.more, ui.Text("Hidden details")),
		ui.Div().Key("pad").Size(ui.Px(80), ui.Px(30)).
			OnDoubleClick(func() { f.events = append(f.events, "double") }).
			OnMouseDown(func(e *ui.MouseEvent) {
				if e.Button == input.ButtonRight {
					f.events = append(f.events, "right")
				}
			}),
		ui.Div().Key("fade").Size(ui.Px(40), ui.Px(20)).Style(ui.Style{Background: bg}).Transition(200*time.Millisecond),
		ui.Div().Key("list").Height(ui.Px(120)).Style(ui.Style{OverflowY: ui.OverflowScroll}).Child(rows),
	)
}

func TestFindClickAndText(t *testing.T) {
	f := newForm()
	u := uitest.New(t, f.root)
	u.Click("add")
	u.ByRole(access.RoleButton, "Add").Click()
	if got := u.Get("count").Text(); got != "Count: 2" {
		t.Errorf("count text = %q, want Count: 2", got)
	}
	if !u.Has("add") || u.Has("nope") {
		t.Error("Has does not tell the keys apart")
	}
	if got := u.ByName("Add").Key(); got != "add" {
		t.Errorf("the key of the node named Add is %q", got)
	}
	if !u.Get("locked").Disabled() || u.Get("add").Disabled() {
		t.Error("Disabled does not report the locked button only")
	}
}

func TestTypePressAndClipboard(t *testing.T) {
	f := newForm()
	u := uitest.New(t, f.root)
	u.Get("name").Type("Ada")
	if f.name.Peek() != "Ada" || u.ByRole(access.RoleTextField, "Name").Value() != "Ada" {
		t.Fatalf("after typing: signal %q, value %q", f.name.Peek(), u.ByRole(access.RoleTextField, "Name").Value())
	}
	if !u.Get("name").Focused() || u.Focused().Name() != "Name" {
		t.Error("the field typed into does not have focus")
	}
	u.Settle() // a blinking caret does not keep the UI from settling
	u.Press("ctrl+a")
	u.Press("ctrl+c")
	if got := u.Clipboard(); got != "Ada" {
		t.Errorf("clipboard after Ctrl+C = %q, want Ada", got)
	}
	u.SetClipboard("Grace")
	u.Press("ctrl+a")
	u.Press("ctrl+v")
	if f.name.Peek() != "Grace" {
		t.Errorf("after Ctrl+V the field holds %q, want Grace", f.name.Peek())
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	u.SetClipboardImage(img)
	if u.ClipboardImage() != img || u.Clipboard() != "" {
		t.Error("an image on the clipboard does not replace its text")
	}
}

func TestStatesAndFocusRing(t *testing.T) {
	f := newForm()
	u := uitest.New(t, f.root)
	box := u.ByRole(access.RoleCheckBox, "Remember me")
	box.Click()
	if !box.Checked() || !f.remember.Peek() {
		t.Error("a click does not check the check box")
	}
	if u.Get("add").FocusRing() {
		t.Error("a ring is drawn around Add before any key press")
	}
	u.Get("add").Focus()
	u.Press("shift+tab")
	u.Press("tab")
	if !u.Get("add").Focused() || !u.Get("add").FocusRing() {
		t.Errorf("after Tab, Add focused %v, ring %v; want both", u.Get("add").Focused(), u.Get("add").FocusRing())
	}
	if s := u.Get("add").Style(); s.Ring != u.Runtime().Theme().FocusRing {
		t.Errorf("ring color %v, want the theme's %v", s.Ring, u.Runtime().Theme().FocusRing)
	}
	u.Click("pad")
	if u.Get("add").FocusRing() {
		t.Error("the ring stayed on Add after a click elsewhere")
	}

	more := u.ByName("More")
	more.Perform(access.ActionExpand)
	if !more.Expanded() || !f.more.Peek() {
		t.Error("ActionExpand does not open the collapsible")
	}
}

func TestPointerGestures(t *testing.T) {
	f := newForm()
	u := uitest.New(t, f.root)
	u.Get("pad").DoubleClick()
	u.Get("pad").RightClick()
	if got := strings.Join(f.events, " "); got != "double right" {
		t.Errorf("events = %q, want double right", got)
	}
	before := u.Get("row-30").Bounds().Y
	u.Get("list").Scroll(0, 300)
	if d := before - u.Get("row-30").Bounds().Y; d != 300 {
		t.Errorf("scrolling by 300 moved a row by %v", d)
	}
}

func TestChordBinding(t *testing.T) {
	f := newForm()
	bump := ui.NewAction("bump", "Bump")
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().OnAction(bump, func() { f.count.Set(f.count.Peek() + 1) }).Child(f.root(ctx))
	}, uitest.Bindings(ui.Bind("ctrl+k ctrl+b", bump)), uitest.Size(600, 900))
	u.Press("ctrl+k ctrl+b")
	if f.count.Peek() != 1 {
		t.Errorf("the chord ran the action %d times, want once", f.count.Peek())
	}
}

func TestClockTransitionAndTooltip(t *testing.T) {
	start := time.Date(2030, time.March, 1, 12, 0, 0, 0, time.UTC)
	f := newForm()
	u := uitest.New(t, f.root, uitest.Start(start))
	if !u.Now().Equal(start) {
		t.Fatalf("clock starts at %v, want %v", u.Now(), start)
	}
	f.lit.Set(true)
	u.Advance(16 * time.Millisecond)
	if bg := u.Get("fade").Style().Background; bg == ui.RGB(255, 255, 255) || bg == ui.RGB(0, 0, 0) {
		t.Errorf("one frame into the transition the fill is %v", bg)
	}
	u.Settle()
	if bg := u.Get("fade").Style().Background; bg != ui.RGB(255, 255, 255) {
		t.Errorf("after Settle the fill is %v, want white", bg)
	}

	u.Get("add").Hover()
	if t0 := u.Tree().Find(access.RoleTooltip, "Add one"); t0 != nil {
		t.Fatal("the tooltip shows before its delay")
	}
	u.Advance(2 * time.Second)
	u.ByRole(access.RoleTooltip, "Add one")
}

func TestWaitForPostedWork(t *testing.T) {
	f := newForm()
	u := uitest.New(t, f.root)
	rt := u.Runtime()
	go func() {
		time.Sleep(10 * time.Millisecond)
		rt.Post(func() { f.count.Set(7) })
	}()
	u.WaitFor("the posted count", func() bool { return u.Get("count").Text() == "Count: 7" })
}

// fakeTB records the first failure and stops the helper that failed, so the
// tests can check failure messages.
type fakeTB struct {
	testing.TB
	msg string
}

type stopped struct{}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.msg = fmt.Sprintf(format, args...)
	panic(stopped{})
}

// fails runs fn with a fakeTB and returns the failure message, or "".
func fails(t *testing.T, fn func(tb testing.TB)) (msg string) {
	tb := &fakeTB{TB: t}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(stopped); !ok {
				panic(r)
			}
			msg = tb.msg
		}
	}()
	fn(tb)
	return ""
}

func TestFailureMessagesNameWhatWasLookedFor(t *testing.T) {
	f := newForm()
	for _, c := range []struct {
		name string
		fn   func(u *uitest.UI)
		want string
	}{
		{"key", func(u *uitest.UI) { u.Get("nope") }, `no element with key "nope"`},
		{"role", func(u *uitest.UI) { u.ByRole(access.RoleButton, "Delete") }, `no button named "Delete"`},
		{"name", func(u *uitest.UI) { u.ByName("Nobody") }, `no node named "Nobody"`},
		{"keys", func(u *uitest.UI) { u.Press("ctrl+nope") }, `Press("ctrl+nope")`},
		{"action", func(u *uitest.UI) { u.Get("add").Perform(access.ActionExpand) }, "does not support expand"},
		{"no node", func(u *uitest.UI) { u.Get("fade").Name() }, `the element with key "fade" has no node`},
	} {
		msg := fails(t, func(tb testing.TB) { c.fn(uitest.New(tb, f.root)) })
		if !strings.Contains(msg, c.want) {
			t.Errorf("%s: failure %q, want it to contain %q", c.name, firstLine(msg), c.want)
		}
	}
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}
