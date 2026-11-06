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
| `u.HasRole(role, name)`, `u.HasName(name)` | report whether such a node is in the accessibility tree, without failing, so they can be polled in `WaitFor` |
| `e.ByRole(role, name)`, `e.ByName(name)` | the first node of that kind in element `e`'s subtree, its own node included |
| `e.HasRole(role, name)` | reports whether `e`'s subtree has such a node, without failing |

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

`ByRole` returns the first match in reading order. When two controls share a
name, such as a page's Remove button and the one in its confirmation dialog,
search inside the one you mean. An element found by key scopes the search to
the nodes of everything inside it, so a plain layout container works too:

```go
u.ByRole(access.RoleAlertDialog, "Remove app?").ByRole(access.RoleButton, "Remove").Click()
u.Get("toolbar").ByRole(access.RoleButton, "Remove").Click()
```

## Acting

On an element, at its center:

- `e.Click()`, `e.DoubleClick()`, `e.RightClick()`. A right click opens the
  element's context menu. `Click` and `DoubleClick` fail the test when the
  element is disabled, or busy as a `Button` with `Loading(true)` is, since the
  click would do nothing; wait for it with `WaitFor(..., func() bool { return
  !e.Disabled() && !e.States().Has(access.StateBusy) })`.
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

### Screenshots

`u.Screenshot()` runs a frame and returns it as an `*image.RGBA`, the window's
size times its scale. It is drawn by jamb's CPU renderer, which the Linux adapter
uses for windows too, so it needs no window, display or GPU and runs in plain
`go test` on every platform. Use it to look at a page by eye, or to compare it with
a saved image. On Windows a window's text is drawn with ClearType by the GPU, so
its text differs slightly from a screenshot's.

```go
u := uitest.New(t, setup.Root(model), uitest.Size(640, 720), uitest.Scale(1.25))
f, _ := os.Create(filepath.Join(t.TempDir(), "done.png"))
defer f.Close()
png.Encode(f, u.Screenshot())
```

A running app can also save its own frames with `JAMB_SCRIPT="idle; shot a.png;
exit"` (see [Tools](tools.md)).

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
- `u.WaitForQueries()` waits, as `WaitFor` does, until no `ui.Query` has a
  fetch in flight or a retry waiting (`rt.Fetching()`), then settles. `Settle`
  alone does not wait for fetches, which run in real time.
- `u.Now()` is the clock's time.

```go
u.Get("add").Hover()
u.Advance(time.Second)
u.ByRole(access.RoleTooltip, "Add one") // fails if the tooltip is not shown
```

## Motion

A test can check that the UI moves well: that nothing pops in or out, jumps,
is cut short or keeps moving where a person would notice. `u.Record()` starts
recording the frames the UI runs, and `rec.Check(t)` fails the test on what it
finds:

```go
rec := u.Record()
u.Click("open")       // the popover fades in
u.Settle()
u.Press("escape")     // and out
u.Settle()
rec.Check(t)
```

Every frame from the one shown at `Record` is kept until `rec.Stop()` or the
next `Record`: those of actions, `Frame`, `Advance`, `Settle`, `WaitFor` and
`Screenshot`. Each holds, for every element drawn, its bounds, opacity, scale
and the rest of its `ui.DrawnStyle`, and whether the frame followed an input (a
click, a key, typing, a hover, a wheel turn or an accessibility action). Input
sent through `u.Runtime()` is not seen as input; call `rec.MarkInput()` before
it. A UI that is not recording samples nothing, so recording costs nothing
until a test asks for it.

Elements are followed by their identity in the runtime, the one their state
keeps: a key, or their place among their siblings. Findings name them by the
keys of their keyed ancestors and their own, then, for an element with no key,
its place below the nearest key: `toaster/jamb.toast:1`,
`settings/div[1]/text "Saved"`.

| Finding | What a person sees |
|---|---|
| `uitest.PopIn` | an element appears at full opacity and size, with no transition |
| `uitest.PopOut` | an element disappears at full opacity and size, with no transition |
| `uitest.CutExit` | an element is removed partway through its exit fade, while it is still seen |
| `uitest.Jump` | an element's position or size changes in one frame, while something animates and not following an input |
| `uitest.Overshoot` | a motion passes where it comes to rest and comes back, with no input during it |
| `uitest.NeverSettles` | the UI still animates at the end of the recording, long after the last input |

The thresholds:

| Rule | Threshold |
|---|---|
| pops | opacity 0.5 or more; 24 × 24 logical pixels or more; no change of opacity, scale or size on the next frame |
| cut exit | last drawn above opacity 0.05, and its last step to nothing is bigger than the fade's step before it |
| jump | 12 logical pixels or more in one frame, and more than 3 times the element's change on the frames before and after it (taken as at least 1 px) |
| overshoot | 2 logical pixels or more past where the motion rests; a motion ends after 3 frames without change |
| never settles | not settled (see `rt.Settled()`) at the last frame, 1 s or more after the last input |

Some motion is not counted:

- An element moved by a scroll, or brought into view by one or by the layout,
  as it was in the tree all along. Hiding an element with `Display(false)`
  and showing it again counts as removing and adding it.
- An element that draws nothing of its own (a layout container with no fill,
  border or shadow): the rules look at what it holds. A pop is reported once, on
  the outermost element that came or went with it.
- An element that moves with its parent: the parent is reported.
- An element that a repeating animation moves: going back to its start is the
  loop.
- An element seen by no one: clipped away, or at opacity 0.05 or less.

`rec.Findings()` returns what was found without failing, for a test that only
reports it. `rec.Frames()` has every frame, to look into a finding.

When a finding is intended, or waits for a fix, allow it with the reason, and
`Check` logs it instead of failing. An `Allow` matches findings of its `Kind`
on elements whose name contains its `Element` (`""` matches any). A rule with
no reason, and a rule that matched no finding, fail the test, so stale rules
come out:

```go
rec.Check(t,
	uitest.Allow{Kind: uitest.Overshoot, Element: "bouncy",
		Reason: "the tile springs with Bounce 0.5 by design"},
	uitest.Allow{Kind: uitest.PopIn, Element: "panel:",
		Reason: "a tab shows its panel at once"})
```

The demo's `examples/demo/views/motion_test.go` records its dialogs, toasts,
tooltips, pickers and pages this way.

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
