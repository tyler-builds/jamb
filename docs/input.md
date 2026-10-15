# Input

Keyboard and focus, text fields and the clipboard, actions and key bindings, events, drag and drop, and gestures.

## Text fields

Text fields support selection with the mouse (drag, double-click a word,
triple-click a paragraph) and with Shift+arrows, plus Ctrl+arrow word
movement, clipboard (Ctrl+C/X/V), grouped undo/redo (Ctrl+Z/Y), Home/End,
PageUp/PageDown and a blinking caret. Word keys follow the platform. On
Windows, as in Notepad, Ctrl+Right and Ctrl+Delete go to the start of the next
word unit, and Ctrl+Left and Ctrl+Backspace to the start of the unit before the
caret. A unit is a word, or a run of punctuation, with the spaces after it, and a
line break is a unit of its own (`text.WordStarts`). Elsewhere, the keys stop at
the end of the next word and the start of the word before, as on macOS and Linux
(`text.NextWord` and `text.PrevWord`). Text areas keep the caret visible
inside the nearest scroll container. Text is shaped with HarfBuzz (via
`go-text/typesetting`), so ligatures, right-to-left scripts, complex scripts
and font fallback for other writing systems all work. Text typed with an input
method (IME), such as Chinese, Japanese or Korean, is composed inside the field:
the text being composed shows at the caret, underlined, with the clause being
converted underlined more heavily, and the input method's candidate list opens
next to that clause. It becomes part of the value when the input method confirms
it. A click, a key the field handles, or focus moving away types it as shown.

## Clipboard

App code reads and writes the clipboard's plain text with `ctx.Clipboard()`,
which returns the text and whether there is any, and `ctx.SetClipboard(s)`.
Text fields use the same clipboard. Images go through
`ctx.ClipboardImage()`, which returns an `image.Image` (show it with
`ui.ImageFromImage`, or save it with `image/png`), and
`ctx.SetClipboardImage(img)`. `ctx.ClipboardHasImage()` says whether there is one
without reading it, cheaply enough for a render (to enable a Paste image menu
item). A `TextInput` or `TextArea` with `.OnPasteImage(fn)` hands fn the image
when the user pastes and the clipboard holds an image but no text, as a message
box that attaches screenshots wants. Copying text replaces an image and the other way
round, as on the system clipboard. On Windows an image is read as PNG when the
program that copied it offered one (browsers, Office and the Snipping Tool do)
and otherwise as a bitmap, and written as both, with its transparency. HTML is
future work.

## Focus and the keyboard

There are three kinds of focus style:

- `Focused(s)` applies whenever the element has keyboard focus, including
  after a click (as GPUI's `.focus()` does).
- `FocusVisible(s)` applies only while focus is keyboard-driven: it arrived by
  Tab or a key press, and no click has happened since. Use it for focus rings.
  A click on a focused element removes the ring; the next key press brings it
  back.
- `FocusWithin(s)` applies while the element or a descendant has focus, for a
  field's frame.

```go
ui.Div().Focusable().FocusVisible(ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: t.FocusRing}}})
```

Custom elements ask `ctx.KeyboardFocus()` (true when this element has focus and
focus is keyboard-driven) and draw their own ring; the component re-renders
when that changes.

- **Built-in rings.** `Button` and `IconButton` draw a 2 px ring in
  `theme.FocusRing` when keyboard-focused. The ring is a `Shadow` with spread
  and no blur or offset, which is drawn as a hollow border outside the box, so
  it adds no layout shift and shows over a transparent button. Text fields and
  text areas keep their `FocusWithin` frame: they always show focus.
- **Enter and Space** run the focused element's `OnClick`, with no modifiers
  and not for an auto-repeat. Text fields keep those keys. A binding for Enter
  or Space runs instead (see the dispatch order under "Actions and key
  bindings").
- **Tab order.** Elements with a positive `TabIndex` come first, in ascending
  order, with ties in tree order. The rest (index 0) follow in tree order, as in
  HTML, and Shift+Tab walks the order backwards. An element inside an `Overlay`
  takes its place from its anchor. `TabIndex` makes an element focusable. A
  negative index, or `TabStop(false)`, keeps an element focusable by click or
  `FocusHandle` while Tab skips it. Each link of a rich text (`ui.Link` or a
  segment with `OnClick`) is a stop of its own, in tree order, and Enter
  follows the focused link (see [docs/text.md](text.md#rich-text)).
- **Tab reveals.** Tab scrolls the element it moves to into view, in its nearest
  vertically scrolling ancestor.
- **Focus moves in OnBlur.** An `OnBlur` handler may move focus to another
  element. The move takes effect, and the element it left is not blurred a
  second time.

The command palette restores focus this way. It takes `ctx.FocusedHandle()`
when it opens and calls `FocusNow()` when it closes, so a command it runs
acts on the field the user was typing in, not on the palette.

## Drag and drop

Drag and drop is typed, as in GPUI: a drag carries a Go value, and a target
receives it only when the value's type matches the target's.

```go
// A source. The drag starts after the pointer moves 4 px with the button held;
// a press that does not move is still a click.
ui.Div().OnDrag(ui.Drag(note).
    Preview(func() ui.Element { return notePill(note) }). // drawn centered on the pointer; optional
    End(func(dropped bool) { ... })).                     // after a drop, a cancel, or a release over nothing
    Dragging(ui.Style{Opacity: 0.5})                      // the source's style while it is dragged

// A target for *store.Note. Each OnDrop takes one payload type; a target may
// have several, and the first on the deepest element that accepts wins.
ui.Div().OnDrop(ui.Drop(func(n *store.Note, e *ui.DropEvent) { ... }).
    Accept(func(n *store.Note) bool { return true }). // like GPUI's can_drop
    Over(ui.Style{BorderColor: accent}).              // merged while an accepted drag is over it
    Move(func(n *store.Note, e *ui.DropEvent) { ... }). // pointer moved over it: e.LocalX, e.LocalY
    Leave(func() { ... }))                            // the drag left it, or ended

// Files dropped from the desktop arrive as ui.Files (their paths).
ui.Div().OnDrop(ui.Drop(func(files ui.Files, e *ui.DropEvent) { ... }))
```

- Matching is by type assertion, so `ui.Drop[fmt.Stringer]` takes any payload
  that implements it, and a `ui.Drop[string]` never takes a `type label string`.
- `DropEvent` has logical pixels: `X`, `Y` in the window, `LocalX`,
  `LocalY` relative to the target, and the target's size `Width`, `Height`, so a row
  can insert before or after itself with `e.LocalY > e.Height/2`.
- Escape, or the window losing focus, cancels a drag; `End` then runs with
  `false`. The source may leave the tree mid-drag; the drop still delivers its
  payload.
- While a drag runs, nothing is hovered or pressed, and the cursor shows
  not-allowed unless the pointer is over a target that accepts the payload.
- A `Preview` is not hit-tested: targets under it are found as if it were
  not there. It starts exactly over the source and follows the pointer.
- Within 32 px of the top or bottom edge of a scroll container (left or
  right for a horizontal one), a drag scrolls it. Nothing animates outside
  that zone, so an idle drag costs no CPU.
- A file drop calls only the `Drop[ui.Files]` target under the pointer. Its
  `Over`, `Move` and `Leave` never run: Windows does not report the pointer
  during a drag from outside the app.

## Actions and key bindings

An action is a named command. Elements handle it with `OnAction`, key
bindings run it, and `ctx.Dispatch` runs it from code. Create actions once,
as package variables:

```go
var (
	NewNote = ui.NewAction("notes.new", "New note")
	Palette = ui.NewAction("notes.palette", "Command palette")
)

ui.Div().OnAction(NewNote, a.newNote)                // runs while focus is on or inside this element
ui.Div().KeyContext("Palette").Child(ui.TextInput(q)) // names a key context for BindIn
app.Window{Bindings: []ui.Binding{                   // or rt.AddBindings(...)
	ui.Bind("ctrl+n", NewNote),                      // global; strokes are "ctrl+shift+p", "alt+up", "f2"
	ui.Bind("ctrl+k ctrl+t", ToggleTheme),           // a chord: the first stroke waits up to a second
	ui.BindIn("Palette > Editor", "up", PaletteUp),  // only inside a Palette that holds a field
	ui.BindIn("Editor", "ctrl+a", ui.NoAction),      // turns Ctrl+A off in text fields
	ui.BindIn("Editor", "ctrl+z", ui.Unbind(actions.Undo)), // Ctrl+Z is no longer Undo in fields
}}
ctx.Dispatch(NewNote)                                // runs along the focus path; false if unhandled
ctx.KeysFor(NewNote)                                 // "Ctrl+N", for tooltips
ctx.AvailableActions()                               // actions handled on the focus path, deepest first
```

`action.Hide()` marks an action for menus and command palettes to leave out;
`action.Hidden()` tells them. Hidden actions still run and still appear in
`AvailableActions`.

**Turning keys off.** `ui.NoAction` disables keys. `BindIn("Editor", "ctrl+a",
ui.NoAction)` makes Ctrl+A do nothing in a text field: the key goes on to
`OnKeyDown` and text input, as if nothing had bound it. `ui.Unbind(a)` removes
`a`'s bindings for the same keys, and only those. `BindIn("Editor", "ctrl+z",
ui.Unbind(actions.Undo))` frees Ctrl+Z from Undo in fields, and a global binding for
Ctrl+Z still runs. A `NoAction` or `Unbind` affects only the bindings ranked
below it, so one added earlier, or one in a shallower context, does nothing.
`KeysFor` skips the bindings they turn off.

**Key contexts.** `KeyContext` takes whitespace-separated entries, each an
identifier (`Editor`) or an attribute (`mode=full`), as in GPUI. Identifiers
use letters, digits, `_` and `-`. A malformed spec panics, naming the problem:
`ui.KeyContext("Editor mode="): "mode=" has no value`.

**Expressions.** `ui.BindIn` takes a GPUI context expression. It is parsed when
the binding is made, and a malformed one panics. The operators, from loosest to
tightest, are `>`, `||`, `&&`, `==` and `!=`, and `!`; parentheses group.

| Syntax | Matches when | Example |
|---|---|---|
| `Editor` | some context on the focus path has the identifier | `BindIn("Editor", "ctrl+d", actions.DeleteForward)` |
| `mode == full`, `mode != full` | the attribute is, or is not, `full` (`!=` also matches when it is absent) | `BindIn("mode == full", "f2", rename)` |
| `!Terminal` | no context on the focus path has the identifier | `BindIn("!Terminal", "ctrl+s", save)` |
| `a && b`, `a \|\| b` | both, or either, match | `BindIn("Editor && mode == single_line", "enter", actions.Confirm)` |
| `a > b` | `b` matches inside a context that matches `a` | `BindIn("Palette > Editor", "up", PaletteUp)` |
| `(a \|\| b) > c` | grouping | `BindIn("Pane > (Editor \|\| Terminal)", "ctrl+w", close)` |

**Precedence.** A contextual binding's depth is the depth at which its
expression matches, counting contexts from the root; a global `Bind` has depth
0. Bindings run deepest first, and at equal depth the one added last runs
first. So a binding for the field's own context beats one for its parent, and
any contextual match beats a global binding.

This differs from GPUI on purpose. There, a global binding ranks at the
deepest level, so a later one can take a key that a field uses. In jamb a
global `Bind` ranks below every contextual binding, so a global shortcut never
takes a key that a focused field uses. To override a field's key, bind it in
the field's context, as `BindIn("Editor", ...)` does.

**Escape and overlays.** Escape closes the topmost open overlay that has an
`OnClickOutside`, one overlay per press, and otherwise hides a visible
tooltip. It runs after the bindings and the `OnKeyDown` handlers, so an app's
binding or handler for Escape takes it first; the notes palette binds Escape
to close itself and return focus to the note. While an overlay or tooltip
would take Escape, the field's built-in Cancel (which blurs a text field) does
not run, so one Escape closes an overlay even when its text field has focus.
With nothing open, Escape does what it did before. An overlay without
`OnClickOutside` is not closed by Escape. Holding Escape closes one overlay:
the auto-repeats of the press that closed it are ignored, so they neither close
the next overlay nor blur a field; press again to close the next one.

A menu takes Escape itself: the first press closes its open submenu, and the
next closes the menu (see [Menus](widgets-overlays.md#menus)).

Key presses are handled in this order:

1. the rest of a chord in progress;
2. all bindings, contextual and global, in precedence order: the first whose
   action has a handler on the focus path runs;
3. `OnKeyDown` on the focused element, then its ancestors up to the root;
4. Enter or Space, without modifiers and not an auto-repeat, runs the focused
   element's `OnClick` (text fields are not clickable, so they keep these keys);
5. Tab, which also reveals the element it moves to, and the zoom keys.

A key that is both a binding and the start of a longer chord waits up to one
second for the chord's next stroke. If the next key completes a binding, that
binding runs. If the next key matches nothing, the waiting keys are replayed as
if the chord had never started: the longest run that matches a binding runs as
that binding, and any other key goes through steps 3 and 4 and types its text.
Then the new key is handled from step 1. Moving focus while a chord waits drops
it, without replaying it.

So a chord that begins with a printable key, such as `g g`, delays typing that
key in a field by up to a second. Prefer a modifier (`ctrl+k ctrl+t`) or a
context such as `!Editor`. A key press that a single-stroke binding consumes
types no text when it is printable (letters, digits, space, `=` and `-`). A key
that waits for a chord keeps its text, which is typed if the chord is replayed.

**Keymap files.** Key bindings can also come from a JSON file, so users can
change them without code. `app.Window` takes the file and the actions it may
name:

```go
app.New().Run(app.Window{
	Bindings:   views.Bindings(),
	KeymapFile: "keymap.json", // a missing file is fine; changes are applied while the app runs
	Actions:    views.Actions(), // the actions the file may name
	OnKeymapLoad: func(err error) { // optional: nil on success, else the problems
		// show err in the UI
	},
	// ...
})
```

The file is a JSON array of sections. A section has an optional `context`
(the same expression `BindIn` takes; none means global) and its `bindings`,
which map keys to actions:

```json
[
  {"bindings": {"ctrl+shift+n": "notes.new"}},
  {"context": "Editor", "bindings": {
    "ctrl+d": "editor.delete-forward",
    "ctrl+a": null,
    "ctrl+z": {"unbind": "editor.undo"}
  }}
]
```

A value is an action name, `null` for `NoAction`, or `{"unbind": name}` for
`Unbind`. Later sections and later keys win, as code order does. The file's
bindings come after `Bindings`, so they override the defaults. The text-field
actions (`ui.EditorActions()`) and `NoAction` can always be named.

`keymap.Parse(data, actions)` parses the bytes of a file, and
`keymap.Load(path, actions)` reads one. Both return the valid bindings and
an error that lists every problem, naming its section (counted from 1), its
context and its keys, for example `keymap section 2 (context "Editor"): "ctrl+d":
unknown action "editor.delet-forward"`. `app.Window` prints the problems to
stderr with a `jamb:` prefix and still applies the valid bindings, so one bad
entry costs only itself.

The file is watched, and saving it applies the change while the app runs. A
file that does not parse at all (for example, a save caught half-written)
keeps the previous bindings and reports the error through `OnKeymapLoad`. On
Windows the watch uses the OS's change notifications, so nothing is polled
and an idle app uses no CPU for it. `app.Window` does this for you;
apps that wire it themselves can use a `keymap.File{Path, Actions}` and call
its `Apply(rt)`, which reads the file and replaces its bindings as one layer.
`rt.SetBindings(layer, bindings)` sets other layers. `ui.ParseBinding` makes a
single binding from a keys string and an optional context, returning an error
rather than panicking.

**Text fields.** A `TextInput` has the context `Editor mode=single_line`, and a
`TextArea` has `Editor mode=full`. Their keys are actions, bound by
`ui.DefaultBindings()`, which `ui.NewRuntime` adds first, so a binding of the
app's at the same depth wins. A package that defines a widget outside `ui` adds
its keys to those defaults with `ui.RegisterDefaultBindings` in its `init`, as
`codeedit` does, so they work without the app binding them. The actions are in
the `actions` package:
`actions.MoveLeft`, `MoveRight`,
`MoveWordLeft`, `MoveWordRight`, `SelectLeft`, `SelectRight`, `SelectWordLeft`,
`SelectWordRight`, `MoveUp`, `MoveDown`, `SelectUp`, `SelectDown`, `PageUp`,
`PageDown`, `SelectPageUp`, `SelectPageDown`, `MoveLineStart`, `MoveLineEnd`,
`SelectLineStart`, `SelectLineEnd`, `MoveToStart`, `MoveToEnd`,
`SelectToStart`, `SelectToEnd`, `DeleteBack`, `DeleteForward`,
`DeleteWordBack`, `DeleteWordForward`, `Newline`, `Indent`, `Confirm`,
`SelectAll`, `Copy`, `Cut`, `Paste`, `Undo`, `Redo` and `Cancel`. They are
hidden, so a palette does not list them. Rebind them like any action:

```go
rt.AddBindings(ui.BindIn("Editor", "ctrl+d", actions.DeleteForward))      // a new key
rt.AddBindings(ui.BindIn("Editor", "ctrl+backspace", actions.DeleteBack)) // replaces the default word delete
```

Behavior changes from earlier versions: a key the defaults don't list (Ctrl+Up,
Alt+Left, Shift+Delete, Shift+Enter in a single-line field, Ctrl+Shift+Backspace)
is no longer handled by a field, so it bubbles on. Shift+Tab and Ctrl+Tab are
unbound in fields, so they move focus as before. A `Tabs` group binds Ctrl+Tab and
Ctrl+Shift+Tab to its tabs (see Navigation and layout).

**Palettes.** A command palette's search field is an `Editor` inside a
`KeyContext("Palette")`. Its keys therefore belong to `"Palette > Editor"`:
`BindIn("Palette", "up", ...)` would lose to the field's own Up, because the
field's context is deeper, and the caret would move instead. This is GPUI's
rule; Zed writes `Picker > Editor` for the same reason.

## Events

Some event semantics to know:

- `OnClick` fires on the deepest clicked element only. Clicks inside an
  overlay never reach its anchor.
- Events on an overlay bubble through its anchor and the anchor's ancestors.
- Other mouse and key events bubble until `StopPropagation`.
- Key presses go to the bindings (deepest context first, global last), then
  to the focused element and its ancestors (`OnKeyDown`), then to Tab and the
  zoom keys. With nothing focused, the root comes first. See the dispatch order
  under "Actions and key bindings".
- Tab cycles focus in tab order, and Enter or Space clicks the focused
  element (see "Focus and the keyboard").

## Gestures

A mouse wheel and a touchpad's two-finger scroll arrive as scroll events, which
`OnScroll` and the built-in scrolling of containers handle, as before. A pinch
(zoom) gesture of a touchpad, or two fingers on a touchscreen, arrives as a
`PinchEvent` from the system's gesture recognizer, DirectManipulation.

- `OnPinch(func(e *ui.PinchEvent))` on any element receives the pinch whose
  center is over it. The deepest element under the center gets it first, then its
  ancestors, until one calls `e.StopPropagation()`.
- `e.Phase` is `ui.GestureBegin`, `ui.GestureUpdate` or `ui.GestureEnd`. An update
  carries what changed since the previous event: `e.Scale` multiplies the scale
  (1 means no change), and `e.PanX` and `e.PanY` move the content in logical
  pixels. `e.X`, `e.Y` give the center in the window, and `e.LocalX`, `e.LocalY`
  in the element. `e.Width`, `e.Height` are the element's size, as in a
  `MouseEvent`.
- `e.Inertia` is set while the gesture continues after the fingers lift. Under
  reduced motion its changes are left out, and the gesture still begins and ends.
- A pinch that no element stops zooms the window in the same steps as Ctrl+= and
  Ctrl+-, when the zoom shortcuts are on (`rt.SetZoomShortcuts`). A pinch that an
  element stops leaves the zoom alone.

```go
ui.Canvas(draw).OnPinch(func(e *ui.PinchEvent) {
	zoom.Set(min(max(zoom.Get()*e.Scale, 1), 4))
	e.StopPropagation() // the chart zooms, the window does not
})
```

The window takes pinch gestures from DirectManipulation, which is set up for
scaling alone. Pan stays with the wheel messages that a
touchpad already sends, because DirectManipulation's pan would scroll twice.
On a touchscreen, DirectManipulation is not given pan either, because a single
finger is a mouse drag there and would be taken from it. Two-finger pan on a
touchscreen is therefore not reported. While a DirectManipulation pinch runs, the
window drops the Ctrl+wheel messages that Windows sends for the same touchpad
pinch, so that the zoom happens once.

What has been verified: the routing, the zoom steps, the zoom shortcuts, reduced
motion and the gesture arithmetic are covered by headless tests (`ui/pinch_test.go`,
`internal/platform/windows/directmanip_test.go`), and the DirectManipulation class
and update manager are checked against the system. What has not: the gestures on
real hardware. They need a precision touchpad and a touchscreen, and the gesture
behaviour (inertia, the Ctrl+wheel that a pinch sends, whether two-finger scroll
still reaches the wheel) is to be confirmed by hand.
