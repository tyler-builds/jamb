// Package uitest drives a jamb UI headlessly in tests: it renders a root
// component in a ui.Runtime of a given size, steps a clock of its own, finds
// elements by key, accessible name or role, acts on them as a user does, and
// reads what they show, their accessibility states and how they were drawn.
//
//	u := uitest.New(t, views.Root(store))
//	u.Get("title").Type("Trip to Lisbon")
//	u.Press("ctrl+s")
//	if !u.ByRole(access.RoleButton, "Save").Disabled() {
//		t.Error("Save is enabled after saving")
//	}
//
// Every action runs a frame, so its effects are visible at once. Lookups that
// find nothing fail the test with a message naming what was looked for. A UI
// has its own clipboard, and never touches the system's. Like the runtime it
// wraps, a UI is not safe for concurrent use. See docs/testing.md.
package uitest

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// frameStep is how far the clock moves for each frame that Advance renders.
const frameStep = 16 * time.Millisecond

// waitLimit bounds Settle (in clock time) and WaitFor (in real time).
const waitLimit = 10 * time.Second

// host is the part of the runtime's host side (see internal/uihost) that a UI uses.
type host interface {
	SetClock(func() time.Time)
	LocalClipboard() string
	SetLocalClipboard(string)
	LocalClipboardImage() image.Image
	SetLocalClipboardImage(image.Image)
	PressKeys(string) error
	Close()
	TextOf(string) (string, bool)
	AccessNode(*access.Tree, string) *access.Node
	Perform(access.ID, access.Request)
}

// UI is a root component rendered in a headless runtime for a test. Create one
// with New.
type UI struct {
	tb       testing.TB
	rt       *ui.Runtime
	host     host
	now      time.Time
	scale    float32
	warnings []string
}

// options are what New's Options set.
type options struct {
	w, h, scale float32
	theme       *ui.Theme
	bindings    []ui.Binding
	settings    *ui.SystemSettings
	start       time.Time
}

// An Option configures New.
type Option func(*options)

// Size sets the window size in logical pixels. The default is 800 × 600.
func Size(w, h float32) Option { return func(o *options) { o.w, o.h = w, h } }

// Scale sets the display scale, 1 by default; 1.5 is a 150% display.
func Scale(s float32) Option { return func(o *options) { o.scale = s } }

// Theme sets the theme. The default is ui.DarkTheme.
func Theme(t *ui.Theme) Option { return func(o *options) { o.theme = t } }

// Bindings adds key bindings, as app.Window.Bindings does.
func Bindings(bs ...ui.Binding) Option {
	return func(o *options) { o.bindings = append(o.bindings, bs...) }
}

// Settings sets the system settings (high contrast, reduced motion, text
// scale...) before the first frame.
func Settings(s ui.SystemSettings) Option { return func(o *options) { o.settings = &s } }

// Start sets the time the clock starts at. The default is 9:00 local time on
// Monday 5 January 2026, so dates the UI shows do not depend on when the test
// runs.
func Start(t time.Time) Option { return func(o *options) { o.start = t } }

// New renders root in a headless runtime and runs the first frame. The
// runtime's clock is the UI's own (see Now and Advance), and its warnings,
// such as duplicate keys, are logged to tb (see Warnings).
func New(tb testing.TB, root func(*ui.Context) ui.Element, opts ...Option) *UI {
	tb.Helper()
	o := options{w: 800, h: 600, scale: 1, start: time.Date(2026, time.January, 5, 9, 0, 0, 0, time.Local)}
	for _, opt := range opts {
		opt(&o)
	}
	u := &UI{tb: tb, now: o.start, scale: o.scale}
	u.rt = ui.NewRuntime(root, o.theme, nil)
	u.host = uihost.Of(u.rt).(host)
	u.host.SetClock(func() time.Time { return u.now })
	u.rt.SetWarnFunc(func(s string) {
		u.warnings = append(u.warnings, s)
		tb.Logf("jamb: %s", s)
	})
	if o.settings != nil {
		u.rt.SetSystemSettings(*o.settings)
	}
	u.rt.AddBindings(o.bindings...)
	u.rt.SetViewport(o.w, o.h, o.scale)
	u.Frame()
	return u
}

// Runtime returns the runtime the UI drives, for what this package does not
// cover. Run a frame (see Frame) after changing it directly.
func (u *UI) Runtime() *ui.Runtime { return u.rt }

// Warnings returns the warnings the runtime gave so far, in order.
func (u *UI) Warnings() []string { return u.warnings }

// Close closes the window the way a window does when it closes: the fetches of its
// queries are canceled, and no new one starts (see ui.Query). The UI cannot be
// used after it.
func (u *UI) Close() { u.host.Close() }

// ---- time ----

// Now returns the UI's clock. It moves only when Advance, Settle or WaitFor
// move it.
func (u *UI) Now() time.Time { return u.now }

// Frame runs one frame at the current time: changes made to signals since the
// last frame are rendered, laid out and painted.
func (u *UI) Frame() { u.rt.Frame(u.now) }

// Advance moves the clock forward by d, running a frame every 16 ms of it, as
// a window does: transitions, animations, timers, tooltip delays and toasts
// move as they would in that time.
func (u *UI) Advance(d time.Duration) {
	for d > 0 {
		step := min(d, frameStep)
		u.now = u.now.Add(step)
		u.Frame()
		d -= step
	}
}

// Settle advances the clock until the UI comes to rest (see
// ui.Runtime.Settled): transitions, entrance and exit motions and scrolling
// have ended. Repeating animations never end and are ignored. It fails the test
// if the UI is still moving after 10 seconds of clock time.
func (u *UI) Settle() {
	u.tb.Helper()
	for elapsed := time.Duration(0); !u.rt.Settled(); elapsed += frameStep {
		if elapsed >= waitLimit {
			u.tb.Fatalf("uitest: the UI did not settle within %v of clock time; something keeps animating", waitLimit)
		}
		u.Advance(frameStep)
	}
}

// WaitFor runs frames until cond reports true, and fails the test, naming
// what, if that does not happen within 10 seconds of real time. Use it for
// work that runs on other goroutines and reaches the UI with Post, such as a
// background load or a combobox's debounced suggestions: each round sleeps
// briefly, then advances the clock by a frame.
func (u *UI) WaitFor(what string, cond func() bool) {
	u.tb.Helper()
	deadline := time.Now().Add(waitLimit)
	for !cond() {
		if time.Now().After(deadline) {
			u.tb.Fatalf("uitest: %s did not happen within %v", what, waitLimit)
		}
		time.Sleep(2 * time.Millisecond)
		u.Advance(frameStep)
	}
}

// ---- keyboard ----

// Type types text into the focused element, one character at a time, and
// runs a frame.
func (u *UI) Type(text string) { u.rt.Type(text) }

// Press presses and releases keys, written as for ui.Bind and
// case-insensitive: "enter", "tab", "shift+tab", "ctrl+a", or strokes
// separated by spaces for a chord such as "ctrl+k ctrl+s". Each stroke runs a
// frame. Press sends the keys only, not the text they would type; use Type for
// text. A malformed keys string fails the test.
func (u *UI) Press(keys string) {
	u.tb.Helper()
	if err := u.host.PressKeys(keys); err != nil {
		u.tb.Fatalf("uitest: Press(%q): %v", keys, err)
	}
}

// ---- clipboard ----

// Clipboard returns the text on the UI's clipboard: what a copy or cut put
// there, or SetClipboard. It is "" when the clipboard is empty or holds an
// image.
func (u *UI) Clipboard() string { return u.host.LocalClipboard() }

// SetClipboard puts text on the UI's clipboard, for a paste to read.
func (u *UI) SetClipboard(text string) { u.host.SetLocalClipboard(text) }

// ClipboardImage returns the image on the UI's clipboard, or nil.
func (u *UI) ClipboardImage() image.Image { return u.host.LocalClipboardImage() }

// SetClipboardImage puts an image on the UI's clipboard, replacing any text.
func (u *UI) SetClipboardImage(img image.Image) { u.host.SetLocalClipboardImage(img) }

// ---- finding ----

// Tree returns the accessibility tree of the UI as it is now, for the checks
// of the access/accesstest package.
func (u *UI) Tree() *access.Tree { return u.rt.AccessTree() }

// Has reports whether an element with the key is in the tree.
func (u *UI) Has(key string) bool {
	_, ok := u.rt.Find(key)
	return ok
}

// Get returns the first element in tree order whose key is key, and fails the
// test if there is none. Keys of components and composite elements (Button,
// TextInput...) belong to the element they render.
func (u *UI) Get(key string) *Element {
	u.tb.Helper()
	e := &Element{u: u, key: key, desc: fmt.Sprintf("element with key %q", key)}
	if !u.Has(key) {
		u.tb.Fatalf("uitest: no %s; the element tree is:\n%s", e.desc, u.dump())
	}
	return e
}

// ByName returns the first node of the accessibility tree, in reading order,
// whose accessible name is name, and fails the test if there is none.
func (u *UI) ByName(name string) *Element {
	u.tb.Helper()
	return u.byNode(fmt.Sprintf("node named %q", name), func(t *access.Tree) *access.Node {
		var found *access.Node
		t.Walk(func(n *access.Node) bool {
			if n.Name == name {
				found = n
			}
			return found == nil
		})
		return found
	})
}

// ByRole returns the first node of the accessibility tree, in reading order,
// with the role and the accessible name, and fails the test if there is none.
func (u *UI) ByRole(role access.Role, name string) *Element {
	u.tb.Helper()
	return u.byNode(fmt.Sprintf("%s named %q", role, name), func(t *access.Tree) *access.Node {
		return t.Find(role, name)
	})
}

// Focused returns the node that has keyboard focus, and fails the test if
// nothing has. The element stays that node when focus moves on.
func (u *UI) Focused() *Element {
	u.tb.Helper()
	t := u.Tree()
	n := t.Node(t.Focus)
	if t.Focus == 0 || n == nil {
		u.tb.Fatalf("uitest: nothing has focus; the access tree is:\n%s", t.Dump())
	}
	id := n.ID
	return u.byNode(fmt.Sprintf("focused %s %q", n.Role, n.Name), func(t *access.Tree) *access.Node { return t.Node(id) })
}

func (u *UI) byNode(desc string, find func(*access.Tree) *access.Node) *Element {
	u.tb.Helper()
	e := &Element{u: u, find: find, desc: desc}
	if t := u.Tree(); find(t) == nil {
		u.tb.Fatalf("uitest: no %s in the access tree:\n%s", desc, t.Dump())
	}
	return e
}

// Click clicks the center of the element with the key (see Get).
func (u *UI) Click(key string) {
	u.tb.Helper()
	u.Get(key).Click()
}

// dump returns the element tree, for failure messages.
func (u *UI) dump() string {
	var b strings.Builder
	u.rt.Dump(&b, 0)
	return b.String()
}
