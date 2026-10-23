# Testing your app

A jamb UI runs without a window: `ui.Runtime` builds, lays out and paints the
element tree in memory, and takes synthetic input. The `ui/uitest` package wraps
it for `go test`. It renders a root component at a size you choose, on a clock
of its own, and lets a test act the way a user does. A test can then read what
the UI shows, its accessibility states, and how each element was drawn. No
window opens and the system clipboard is never touched, so these tests run in
CI like any other Go test.

## A complete example

A sign-in form with a name field, a check box and a button:

```go
package views

import (
	"fmt"

	"github.com/tyler-builds/jamb/ui"
)

type Form struct {
	Name     *ui.Signal[string]
	Remember *ui.Signal[bool]
	Status   *ui.Signal[string]
}

func NewForm() *Form {
	return &Form{Name: ui.NewSignal(""), Remember: ui.NewSignal(false), Status: ui.NewSignal("")}
}

func (f *Form) Root(ctx *ui.Context) ui.Element {
	name := f.Name.Get()
	return ui.Div().Column().Gap(8).Padding(16).Children(
		ui.TextInput(f.Name).Key("name").Label("Name"),
		ui.Checkbox("Remember me", f.Remember).Key("remember"),
		ui.Button("Sign in").Key("sign-in").Disabled(name == "").
			OnClick(func() { f.Status.Set(fmt.Sprintf("Welcome, %s", f.Name.Peek())) }),
		ui.Div().Key("status").Child(ui.Text(f.Status.Get())),
	)
}
```

Its test:

```go
package views

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui/uitest"
)

func TestSignIn(t *testing.T) {
	f := NewForm()
	u := uitest.New(t, f.Root, uitest.Size(400, 300))

	signIn := u.ByRole(access.RoleButton, "Sign in")
	if !signIn.Disabled() {
		t.Fatal("Sign in is enabled before a name is typed")
	}
	u.Get("name").Type("Ada")
	u.Get("remember").Click()
	signIn.Click()

	if got := u.Get("status").Text(); got != "Welcome, Ada" {
		t.Errorf("status = %q", got)
	}
	if !u.Get("remember").Checked() {
		t.Error("Remember me is not checked")
	}

	u.Press("shift+tab") // focus back to Remember me, shown as keyboard focus
	if !u.Get("remember").FocusRing() {
		t.Error("no focus ring around Remember me after Shift+Tab")
	}
}
```

Each action runs a frame, so its effect can be checked on the next line. A
lookup that finds nothing fails the test. The message names what was looked
for and prints the element tree or the accessibility tree.

This example is a test of the package (`ui/uitest/guide_test.go`). The notes
example has tests in this style too: `examples/notes/views/keyboard_test.go`
tabs through note rows and checks their focus ring.

## Creating the UI

```go
u := uitest.New(t, root, uitest.Size(1100, 720), uitest.Bindings(views.Bindings()...))
```

`uitest.New(tb, root, options...)` renders `root` and runs the first frame.
The options are:

| Option | Default | Sets |
|---|---|---|
| `Size(w, h)` | 800 × 600 | the window size, in logical pixels |
| `Scale(s)` | 1 | the display scale; 1.5 is a 150% display |
| `Theme(t)` | `ui.DarkTheme()` | the theme |
| `Bindings(bs...)` | none | key bindings, as `app.Window.Bindings` adds them |
| `Settings(s)` | the defaults | `ui.SystemSettings`: high contrast, reduced motion, text scale... |
| `Start(t)` | 9:00 local time, Monday 5 January 2026 | the time the clock starts at |

The runtime's warnings, such as duplicate keys, are logged to the test, and
`u.Warnings()` returns them. `u.Runtime()` is the `*ui.Runtime` underneath, for
what the package does not cover, such as `SetZoom`, `SetStrings` or
`Dispatch`. After changing it directly, call `u.Frame()`.
`u.Close()` closes the window as the app does, so the fetches of its queries are
canceled (see [Loading data](data.md)).

## Finding elements

| Call | Finds |
|---|---|
| `u.Get(key)` | the first element in tree order with that key |
| `u.ByRole(role, name)` | the first accessibility node with that role and accessible name |
| `u.ByName(name)` | the first accessibility node with that accessible name, whatever its role |
| `u.Focused()` | the accessibility node that has keyboard focus |
| `u.Has(key)` | reports whether an element has the key, without failing |

Keys are the most stable way to find an element: give the elements a test
needs a `Key`. A component's or composite element's key (`Button`,
`TextInput`...) belongs to the element it renders. Finding by role and name
tests what assistive technology sees, and it finds elements that have no key.

An `*uitest.Element` is looked up again each time it is used, so it follows the
UI as it re-renders. A method used after its element has gone fails the test.
`e.Exists()` reports whether the element is still there, and never fails.

An element found by key reads its accessibility node from the element. When the
keyed element has no node of its own, as with the box a `TextInput` draws
around its field, the node is the first one inside it. An element found by role
or name reads `Text` and `Style` through its key, so it needs one.

## Acting

On an element, at its center:

- `e.Click()`, `e.DoubleClick()`, `e.RightClick()`. A right click opens the
  element's context menu.
- `e.Hover()` moves the pointer there. Advance the clock to show a tooltip.
- `e.Scroll(dx, dy)` turns the wheel over it by logical pixels. A positive `dy`
  shows what is below. The scroll container moves at once.
- `e.Focus()` gives it keyboard focus, as assistive technology does.
- `e.Type(text)` focuses it, unless it has focus, and types.
- `e.Perform(action)` runs an accessibility action, such as
  `access.ActionExpand`, `ActionToggle` or `ActionSelect`. It fails when the
  element does not support the action.

On the UI:

- `u.Click(key)` is `u.Get(key).Click()`.
- `u.Type(text)` types into whatever has focus.
- `u.Press(keys)` presses keys, written as for `ui.Bind`: `"enter"`, `"tab"`,
  `"shift+tab"`, `"ctrl+a"`, `"f2"`, or a chord such as `"ctrl+k ctrl+s"`.
  Press sends the key events only, not the text a key would type. Use `Type`
  for text.

Focus is shown as keyboard focus (the `FocusVisible` styles, the focus ring)
after a key press, and not after a click or `Focus`. That is why the example
presses Shift+Tab before it checks the ring.

## Reading

| Call | Returns |
|---|---|
| `e.Text()` | the text inside the element: its text elements and the values of its text fields, joined by spaces |
| `e.Name()`, `e.Value()` | the accessible name and value (a text field's text, a combo box's choice) |
| `e.Focused()`, `e.Disabled()`, `e.Selected()`, `e.Checked()`, `e.Expanded()` | an accessibility state |
| `e.States()` | all of the states, an `access.State` |
| `e.Node()` | the `*access.Node`, for anything else: role, description, range, position in a set |
| `e.Bounds()` | the rectangle, in logical pixels, window coordinates |
| `e.Key()` | the key |
| `e.Style()` | how the element was drawn, a `ui.DrawnStyle` (see below) |
| `e.FocusRing()` | whether a ring is drawn around the element |

`u.Tree()` is the whole accessibility tree, and the checks of
`access/accesstest` take it:

```go
accesstest.Focused(t, u.Tree(), access.RoleButton, "Sign in")
```

### How an element was drawn

`e.Style()` returns the element's `ui.DrawnStyle`, read from what the latest
frame painted. Hover, press and focus styles are merged in, and transitions are
at the point they reached:

| Field | Is |
|---|---|
| `Painted` | whether the frame drew it; an element scrolled out of view or hidden by `Display(false)` was not, and the other fields are zero |
| `Bounds` | where it was drawn |
| `Background`, `Gradient` | the fill; for a gradient, its first color, with `Gradient` set |
| `BorderColor`, `BorderWidth` | the border, zero when none is drawn |
| `TextColor` | the color of its text and icons, which its children inherit |
| `Opacity` | its opacity, its ancestors' included, 1 when opaque |
| `Ring`, `RingWidth` | the ring around it, as the built-in controls draw their focus ring; `RingWidth` is 0 when there is none |

Colors are the element's own, before its opacity applies:

```go
// The row is ui.Div().Key("row").Hover(ui.Style{Background: hover})...
u.Get("row").Hover()
if s := u.Get("row").Style(); s.Background != hover {
	t.Errorf("the hovered row is filled with %v, want %v", s.Background, hover)
}
```

The style belongs to the element that draws it. When a row draws the ring and
the text inside it has the key, give the row a key too. The notes example keys
its note rows `row-<id>` for this. Outside uitest, `rt.StyleOf(key)` returns the
same `DrawnStyle` from a `ui.Runtime`.

## Time

The UI has a clock of its own, which starts at a fixed time so dates the UI
shows do not depend on when the test runs. Timers, tooltips, toasts,
transitions and motions run on it, and it moves only when the test moves it:

- `u.Frame()` runs a frame at the current time, for changes made to signals
  outside an action.
- `u.Advance(d)` moves the clock by `d`, with a frame every 16 ms of it, as a
  window would. Use it for tooltip delays, toasts, timers and partway
  transitions.
- `u.Settle()` advances until transitions, entrance and exit motions, and
  scrolling have ended (`rt.Settled()`). Repeating animations never end, so it
  ignores them. It fails after 10 seconds of clock time.
- `u.WaitFor(what, cond)` runs frames until `cond` holds, and fails, naming
  `what`, after 10 seconds of real time. Use it for work on other goroutines
  that reaches the UI with `Post`, such as a background load or a combobox's
  debounced suggestions. It sleeps briefly between frames and advances the
  clock by a frame each time.
- `u.Now()` is the clock's time.

```go
u.Get("add").Hover()
u.Advance(time.Second)
u.ByRole(access.RoleTooltip, "Add one") // fails if the tooltip is not shown
```

## The clipboard

The UI has a clipboard of its own, the in-process one a headless runtime
uses, so a test never reads or changes the system clipboard:

```go
u.Get("name").Type("Ada")
u.Press("ctrl+a")
u.Press("ctrl+c")
if u.Clipboard() != "Ada" { ... }

u.SetClipboard("Grace")
u.Press("ctrl+v")
```

`u.ClipboardImage()` and `u.SetClipboardImage(img)` do the same for images.

## Without uitest

`ui.Runtime` has the lower-level calls that uitest builds on: `Frame`,
`ClickKey`, `Type`, `Press`, `Find`, `StyleOf`, `Dump`, `AccessTree`, and the
pointer, wheel and key methods. [Tools](tools.md) lists them. They take no
`testing.TB` and read the real clock.
