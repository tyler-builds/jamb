# Tools for humans and coding agents

The `jamb` command in `cmd/jamb` has three commands: `jamb dev`, the watch-and-restart
runner; `jamb ctl`, which reads and drives a running app (see [Control channel](#control-channel));
and `jamb package`, which builds the Windows release (see [Packaging](packaging.md#packaging)).

These environment variables work with any jamb app, with no code changes:

| Variable | Effect |
|---|---|
| `JAMB_TRACE=1` | Prints startup timings, per-frame build/layout/paint cost, and frames per second of activity, so you can confirm the app is idle. Also turns on the accessibility warnings (see [Accessibility](accessibility.md#accessibility)). |
| `JAMB_A11Y_WARN=1` | Prints accessibility warnings: an interactive element with no name, and an image without `Alt`. Each is reported once. On by default under `jamb dev`. |
| `JAMB_INSPECT=1` | Opens the visual inspector for the first window at start, if the program links it. Under `jamb dev` the inspector is linked, and Ctrl+Shift+I opens it (see [Visual inspector](#visual-inspector)). |
| `JAMB_CONTROL=1` | Opens the control channel, which `jamb ctl` and coding agents use to read and drive the running app, if the program links it. `jamb dev` links it and sets this (see [Control channel](#control-channel)). |
| `JAMB_SCENE=NAME` | Opens the first window on the scene called NAME instead of the app's own root, in a program built with the `jambdev` tag, as `jamb dev` builds (see [Scenes](#scenes)). |
| `JAMB_RENDERER=cpu` | On Linux, draws every window on the CPU instead of with Vulkan (see [Vulkan on Linux](architecture.md#vulkan-on-linux)). For comparing the two, or for a GPU driver that misbehaves. |
| `JAMB_HEADLESS=1` | Runs the app with no window on the screen: its windows exist in memory and the CPU renderer draws them, so nothing takes the focus or appears on the desktop (see [Headless runs](#headless-runs)). |
| `JAMB_HEADLESS_SCALE=S` | The display scale of a headless run's windows, from 0.5 to 4; 1 by default. |
| `JAMB_SCRIPT="idle; shot a.png; click 600 400; type hi; key enter; idle; shot b.png; exit"` | Drives the app with synthetic input and saves screenshots. This is how the demo was verified. Besides `click`, the steps include `rclick X Y` (the right button, which opens a context menu), `down X Y`, `move X Y` and `up X Y` for drags, and `drop X Y PATH...` for files dropped from the desktop. The `drop` step goes through the same handler as a real drop, but the Windows message handling itself is only exercised by a real drop. `window N` makes the following steps, and the `shot`s, target the Nth window opened (1-based); the first window is the default. `type TEXT` types everything after the one space that follows `type`, spaces included. `devicelost` simulates a GPU device loss; see [GPU device loss](architecture.md#gpu-device-loss). `mark` sets a loop point and `repeat N` goes back to the last mark N more times, or loops until the window closes with `repeat 0`; a `repeat` needs a `mark` before it. `a11ydump PATH` writes the accessibility tree as text (see [Accessibility](accessibility.md#accessibility)). |
| `JAMB_STRESS=rebuild\|paint` | Measures sustained frame cost, allocation and GC, prints the results, then exits. Its frame times come from the performance counter. |
| `JAMB_FRAMESTATS=PATH` | Records every frame of the first window, from its first frame until it closes, and writes `PATH` with one row per frame (CPU time, the interval between Presents, whether the frame rebuilt, updated or repainted the tree, and whether it came with no idle gap). The summary's present interval covers only the frames with no idle gap, so a pause between frames does not count as an interval. It writes `PATH-input.csv` beside it, with the latency of each input event that caused a frame, from its arrival to the start and the return of `Present`, and prints a summary to stderr. Time to the screen is not measured. |
| `JAMB_HEAPLOG=PATH` | Writes a CSV row of memory counters now and every 10 seconds (`JAMB_HEAPLOG_SECONDS` changes it): the Go heap, allocations, GC cycles, goroutines, and the process's private bytes and working set. For soak runs. |
| `JAMB_POSITION="X,Y"` | Places the first window's top-left corner at logical `(X, Y)`, before it is first shown. Negative values place it on a monitor left of or above the primary. Later windows cascade from it. |
| `JAMB_NOACTIVATE=1` | Shows every window without activating it, and never asks Windows to make one the foreground window, so a run leaves the user's focus and pointer alone. A window that is not active draws no text caret. |
| `JAMB_VSYNC=0` | Turns off vsync, for throughput tests. |
| `JAMB_D3D_DEBUG=1` | Enables the Direct3D debug layer. |
| `JAMB_TEXT_AA=gray` | Forces grayscale text anti-aliasing instead of ClearType (Windows). |

`JAMB_SCRIPT` keys are `enter`, `escape` (or `esc`), `tab`, `backspace`,
`delete`, `insert`, `space`, the arrows, `home`, `end`, `pageup`, `pagedown`,
`menu` (the Apps key), `f1` to `f12`, `a` to `z`, `0` to `9`, `=` and `-`, with the modifiers `ctrl`,
`shift`, `alt` and `super`. An unknown key or modifier stops the script with an
error when the script is loaded. In `move`, `click`, `rclick`, `down`, `up` and
`scroll`, `#KEY` in place of `X Y` aims at the center of the element with that
key, where it is when the step runs: `click #save`, `scroll #list 300`. A key
that no element has stops the program with an error. `tree PATH` writes the
accessibility tree as JSON, its nodes as the control channel's `tree` op gives
them, with their keys, roles, names, states and bounds. `idle` waits for `rt.Settled()`, which ignores
repeating animations, so a screen with a spinner does not hold the script, and
for every `ui.Query` fetch in flight or waiting to retry (`rt.Fetching()`), so a
screenshot after it shows the fetched data. `wait N` pauses the script for N
milliseconds without drawing, for work that neither of those tracks.

For tests of an app, the `ui/uitest` package wraps these headless tools with a
clock, a clipboard and failure messages of its own; see
[Testing your app](testing.md#testing-your-app). The headless tools are:

- `ui.NewRuntime(root, theme, nil)` plus `rt.Frame(time.Now())` runs the
  whole pipeline without a window.
- Scripted actions, each followed by a frame:
  - `rt.ClickKey("save")` clicks the element keyed `save`.
  - `rt.Type("text")` types into the focused element.
  - `rt.Press(input.KeyN, input.ModCtrl)` presses a key.
- `rt.Find(key)` returns an element's rectangle. A component's or text
  field's key belongs to what it renders.
- `rt.StyleOf(key)` returns how the latest frame drew an element: its fill,
  border, text color, opacity and focus ring, with state styles and
  transitions applied (`ui.DrawnStyle`).
- `rt.Dump(os.Stdout, depth)` prints the laid-out tree, with keys and text
  field contents. It found layout bugs during development.
- `rt.ContentWidth()` checks that nothing overflows at the minimum window size.
- `rt.Settled()` reports whether nothing changed since the last frame and
  nothing animates toward an end. Repeating animations never end, so they do
  not count; `rt.NeedsFrame()` still reports them.
- `go run ./internal/iconsheet icons.png` renders every icon.

See `ui/runtime_test.go`, `ui/editor_test.go`,
`examples/demo/views/dump_test.go` and `examples/notes/views/views_test.go` (the notes app
driven headlessly: Ctrl+N, typing, search, saving), and
`examples/notes/views/keyboard_test.go` for the same with `ui/uitest`.

## State across restarts

`jamb dev` restarts the app on each save, and the app comes back as you left
it: every signal the app makes with `ui.NewSignal`, and every `ui.Local`, has
the value it had before. The app declares nothing for this.

- Before it stops the app, `jamb dev` asks it for a checkpoint over the control
  channel. The new build reads it (`JAMB_STATE_FILE`) and gives each signal its
  old value as it is made.
- A signal is known by where it is made: `jamb dev` rewrites, in copies that it
  builds from, each `ui.NewSignal` call of the app's module into one with a
  name from the source, such as `views:NewForm/Form.Name` or
  `main:main/clicks`. Moving code or adding lines does not change the names;
  renaming a variable starts that signal again. The copies keep every line
  where it was, so compiler errors and stack traces point at your files.
- A local is known by its component and its name. The rewrite names each
  `ui.Component` call too, and a component is known by the calls of the
  components around it, with its key, or its place among its siblings from the
  same call. A component that moves keeps its locals, and one that takes
  another's place does not take them.
- The view comes back too: the element that had the focus has it again, text
  fields have their carets and selections where they were, and scroll
  containers are scrolled where they were. A `ListOf` keeps the item at the top
  of its view, since it estimates the heights of the items it has not measured.
- The values are copied whole, unexported fields included, and a pointer that
  two signals share is shared again after the restart.
- **Startup code.** Go apps often make a signal and then set it as they start,
  for example to the first note read from disk. The restart compares what the
  app gives each signal itself, its initial value and what the startup code
  sets, with what it gave it last time. When it is the same, the value from
  before the restart wins: the note you had selected stays selected. When it
  differs, the app's new value wins: a default you changed in the code takes
  effect, and data read at startup is not covered by older data. A struct
  field added or removed does not count as a difference, and a field added
  keeps its default.
- A value that holds a func, a channel or an interface of a type the app could
  not rebuild cannot be kept, and starts again. `jamb dev` names the app's
  signals that it could not keep; components' locals are counted, and named
  with `JAMB_TRACE=1`. After each restart it says how many values it kept.
- After the checkpoint, the app takes no more input, and its windows run their
  `OnClose` and their pending timers, as they do when they close, so what the
  app saves as it closes is on disk too.
- The new build starts beside the old one, which stays on screen until the new
  one has drawn and settled; then `jamb dev` stops the old one. The window is
  never gone between the two, and on Windows the new one opens where the old
  one is. Under `jamb dev`, `SingleInstance` is off, so that the two can run at
  once.
- On Wayland an app cannot place its windows, so `jamb dev` uses a window
  session (`xdg-session-management-v1`, which KWin has from Plasma 6.7) to have
  the compositor put the new build's windows where the old build's were. The
  compositor keeps a window's place when the window closes, so there the old
  build's windows close as `jamb dev` takes its state, and the screen is
  without them until the new build has drawn. A compositor without window
  sessions places the new windows itself.
- If the app ends within 5 seconds of a restart that gave it state, `jamb dev`
  starts it again without that state, and says so.
- None of this is in a release build: the rewrite happens only in `jamb dev`,
  and the code that keeps the values (package `devtools/hot`) is linked only
  with package `devtools`, and never into a web build. An app that links
  `devtools` itself, for the inspector, keeps nothing and does no work for it
  unless `jamb dev` started it.

## Control channel

The control channel lets a coding agent, a script or you read and drive a
running app: list its windows, read their accessibility trees, find elements,
click, type, press keys, scroll, take screenshots, wait until the UI has
settled or an element appears, and drive the app's clock. It is `JAMB_SCRIPT`
made interactive, and every answer is JSON.

Every app that `jamb dev` runs has it open. Another program opens it when it
links package `devtools` (`import _ "github.com/tyler-builds/jamb/devtools"`)
and runs with `JAMB_CONTROL=1`. A program that does not import `devtools`, such
as a release build, carries none of its code.

```
jamb ctl list                                   # the running apps that have it open
jamb ctl info                                   # the app's windows
jamb ctl find role=button name=Save
jamb ctl click key=save
jamb ctl wait for=idle
jamb ctl type text="hello world"
jamb ctl key keys="ctrl+a backspace"
jamb ctl wait for=found contains="saved" timeout_ms=2000
jamb ctl shot path=after.png
jamb ctl clock +500                             # time moves only when the agent moves it
```

With one app running, `jamb ctl` talks to it; with several, `-app PID` picks one.
`jamb ctl -` reads requests as JSON lines on stdin and writes the answers as JSON
lines, over one connection, which suits an agent that sends many requests.

### Requests

A request is a JSON object on one line, and so is its answer. The answer carries
the request's `id` back, and either a `result` or an `error` with a `code` and a
`message`:

```
{"id":1,"op":"click","key":"save"}
{"id":1,"result":{"accepted":true,"x":412,"y":96}}
{"id":2,"op":"click","key":"missing"}
{"id":2,"error":{"code":"not_found","message":"no element matches; read the tree to see what is on screen"}}
```

`window` picks a window by its place in the order the windows opened, 1 for the
first (the default). Coordinates and bounds are logical pixels from the window's
top left, as in `JAMB_SCRIPT`.

| Op | Arguments | Result |
|---|---|---|
| `info` | none | The protocol version, the process ID and the windows, with their titles, sizes and scale |
| `tree` | `limit` (100, at most 256), `offset`, `revision` | A page of the accessibility tree, depth first in reading order. A later page passes the first page's `revision`; if the tree changed, it fails with `stale` |
| `find` | `node`, `key`, `role`, `name`, `contains`, `limit` | The nodes that match all the arguments given: `key` and `name` exactly, `role` (such as `button` or `text field`) in any case, and `contains` in the name or value, in any case |
| `click` | `x` and `y`, or what `find` takes; `button` (`left`, `right`, `middle`), `count` (1 to 3), `timeout_ms` | Moves the pointer to the point, or to the center of the first match on screen, and clicks |
| `move` | `x` and `y`, or what `find` takes | Moves the pointer there, as a hover does: tooltips and hover cards open |
| `drag` | `x` and `y`, or what `find` takes; `dx`, `dy` | Presses the left button there, moves by `dx` and `dy` in steps, and releases |
| `type` | `text` (at most 16 KiB) | Types into the focused element |
| `key` | `keys`: names as in `JAMB_SCRIPT`, such as `enter` or `ctrl+shift+left`, separated by spaces | Presses and releases each key in order |
| `scroll` | `dx`, `dy`, and `x` and `y` or what `find` takes | Scrolls by `dx` and `dy` logical pixels at the point |
| `shot` | `path` (absolute; `jamb ctl` makes it so), `timeout_ms` | Writes the next frame as a PNG, to `path` or a new file in the control directory, and returns its path and size in physical pixels |
| `wait` | `for`: `idle`, `found` or `gone`; what `find` takes; `timeout_ms` (5000, at most 60000) | Answers when the window has settled and no `ui.Query` is fetching (`idle`, as the `idle` step of `JAMB_SCRIPT`), when a node matches (`found`, with the matches), or when none does (`gone`) |
| `actions` | none | The actions handled where the window's focus is, deepest element first, with their labels and keys: what the command palette lists |
| `dispatch` | `action`, an action's name | Runs the action where the focus is, as its key binding does, and says whether something handled it |
| `commands` | none | The app's commands (see below), with the JSON Schemas of their input and output; it takes no `window` |
| `run` | `command`, `args` (the input, a JSON value) | Runs the command and returns its `result`; it takes no `window` |
| `scenes` | none | The app's scenes and the one the first window shows (`current`, empty for the app's own root); see [Scenes](#scenes). It takes no `window` |
| `scene` | `name`, a scene's name; none goes back to the app's own root | Opens the scene in the first window, from fresh state, and returns it as `current`. A scene opened this way is not kept across `jamb dev`'s restarts; `JAMB_SCENE` is |
| `clock` | one of `advance_ms` (at most 3600000), `settle` (`true`) and `mode` (`live` or `seek`), or none | Moves the app's clock forward, moves it until the UI comes to rest, or switches its mode; see [Clock](#clock). Answers with the clock's `mode` and `now`, how far it moved (`elapsed_ms`), whether `settle` came to rest (`settled`), and how long until the next timer is due (`next_timer_ms`). It takes no `window` |

A node has an `id`, its `parent`, `role`, `name`, `key`, `value`, `description`,
`states` (such as `focused`, `disabled` or `offscreen`), `bounds` and the number
of its `children`. An item of a set, such as a row of a list, has `pos_in_set`
and `set_size`, and a scroll container that can scroll has `scroll`, its position
in percent of its range on each axis it scrolls on. IDs are strings, because
they use all 64 bits. A name, value or description longer than 512 bytes is cut,
and the node says `truncated`.

Each request sees the effects of the requests before it on the same window: a
request that arrives while earlier input is still to be drawn runs after the
next frame, so a `find` right after a `type` finds what the typing changed.
Effects that take longer than a frame, such as a `ui.Query` fetch or an
animation, need `wait for=idle` or `wait for=found`, or, while the agent drives
the clock, `clock`.

`click`, `move`, `drag` and `scroll` aimed at a node wait for the node to be
ready before they act: on screen, scrolled into view if a scroll container has
it out of view, and still, in the same place in two frames in a row, so the
input never lands where a moving node was. A node that is not ready within
`timeout_ms` (5000) fails with `not_visible`. Input at `x` and `y` acts at once.
The input ops answer when the input has reached the app. A request is never
retried, since a click that may have landed must not land twice.

The error codes are `bad_request`, `unknown_op`, `no_window`, `not_found`,
`not_visible` (a point outside the window, or a hidden window), `stale`,
`timeout`, `failed` and `quit`.

### Clock

An agent can drive the app's time instead of sleeping, as with Expo's Exact.
The clock starts live, following the wall clock. The first `clock` request that
moves it puts it in seek mode: it then stands still between requests, and moves
only when the agent moves it. Timers (`Context.After` and the debounces built on
it), tooltip and menu delays, toasts, transitions and animations, the caret and
the windows' frames all read it, so nothing changes between two requests, and a
`tree` or a `shot` shows the moment the clock reached.

```
jamb ctl clock                # the mode (live or seek) and the time
jamb ctl clock seek           # stops the clock where it is (mode=seek)
jamb ctl click key=save
jamb ctl clock +60000         # moves it a minute forward (advance_ms=60000)
jamb ctl clock settle         # moves it until the UI is at rest (settle=true)
jamb ctl clock live           # gives it back to the wall clock (mode=live)
```

- `advance_ms` runs a frame of every window for every 16 ms of the time it
  moves, as `uitest.Advance` does, so animations and timers run as they would
  in that time, and visible windows then draw the moment it reached.
- `settle` advances a frame at a time until every window has come to rest (as
  `ui.Runtime.Settled` means) and no timer is due within 10 seconds, so a
  debounce runs and a query's cache expiry, minutes away, does not. It stops
  after 10 seconds of clock time with `settled` false, when something keeps
  animating or a timer keeps coming due, as one that repeats does; repeating
  animations are ignored, as in `uitest.Settle`. It does not wait for a
  `ui.Query` fetch, which runs in real time: `wait for=idle` waits for that.
- There is one clock for the app: every window follows it, so a timer in one
  window never runs ahead of what another shows.
- In seek mode, `wait for=idle` and the input ops do not wait for an animation,
  which moves only when the clock does: idle means that a frame at the clock's
  time has nothing new to show.
- `clock live` goes on from where the agent left the clock, or from the wall
  clock's time if that is later, so the clock never steps back.
- `jamb ctl clock` also takes Exact's short form: `+MS`, `settle`, `seek` and
  `live`.

### Commands

An agent can do anything a user can by clicking and typing, and anything the
keys and the command palette can with `dispatch`. For what would be slow or
ambiguous that way, an app registers commands: operations with typed input and
output, which `commands` describes with JSON Schemas made from the Go types.

```go
type CreateIn struct {
	Title string   `json:"title" desc:"The note's title"`
	Tags  []string `json:"tags,omitempty"`
}

var _ = ui.NewCommand("notes.create", "Create a note and show it in the editor",
	func(in CreateIn) (NoteInfo, error) { ... })
```

```
jamb ctl commands
jamb ctl run command=notes.create args='{"title":"Release checklist","tags":["work"]}'
```

- `run` calls the function on the UI thread, with `args` decoded into its input
  type, and encodes what it returns as `result`.
- A field without `omitempty` (or `omitzero`) is required, and a field's `desc`
  tag is its description in the schema. Unknown fields, missing required
  fields and values of the wrong type fail with `bad_request`, before the
  function runs; an error the function returns fails with `failed`.
- A command is app-wide: it does not depend on the focus, as an action does.
  The notes example registers `notes.list`, `notes.get`, `notes.create`,
  `notes.update`, `notes.delete` and `notes.select` (see
  `examples/notes/views/commands.go`).

### Limits

- `find`, `wait` and the targets of the input ops match the accessibility
  tree, so an element with no accessible role, such as a plain `Div`, is not
  found by them. The input ops with only a `key` fall back to the element's
  layout, so `click key=...` works for any keyed element, but without waiting
  for it to be ready.
- A virtualized list (`ListOf`, `UniformList`) builds only the rows near its
  view, so a row far from it is not in the tree until the list is scrolled
  toward it. The rows' `set_size` tells how many there are.
- A window that is hidden, minimized or covered by other windows (Windows and
  GNOME stop drawing a covered window) draws no frames of its own. The channel
  then builds and lays out its frames without drawing them, so requests, waits
  and input work as on screen, but `shot` fails with `not_visible`: bring the
  window to the front to take a screenshot.
- A connection answers its requests one at a time, in order. To act while a
  `wait` is in progress, open a second connection.

### Where it listens

The app listens on a Unix domain socket named after its process ID (Windows 10
1803 and later have them too), in a directory that only its user may use:
`JAMB_CONTROL_DIR`, or `jamb-UID` in the temporary directory (`jamb-control` on
Windows). Beside the socket, `PID.json` describes the app: its executable, its
working directory and when it started. The app removes both when it ends; `jamb
dev` removes them when it restarts the app, and the next app to open its channel
removes those that a crashed app left.

## Headless runs

`JAMB_HEADLESS=1` runs an app with no window on the screen. Its windows exist
only in memory, the CPU renderer draws their frames, and nothing takes the
keyboard focus or appears on the desktop, so you can keep working while an
agent runs and drives the app. Everything else is the app as it runs in a
window: `JAMB_SCRIPT`, `JAMB_SCENE`, the control channel, `jamb ctl shot` and
`jamb dev`'s restarts.

```
# One scripted run: build, open on a scene, take a screenshot and the tree, quit.
JAMB_HEADLESS=1 JAMB_SCENE=Notes/Many \
  JAMB_SCRIPT="idle; shot many.png; click #preview-toggle; idle; tree tree.json; exit" jamb dev

# A live run that jamb ctl drives, rebuilt on each save.
JAMB_HEADLESS=1 jamb dev &
jamb ctl scene name=Notes/Empty
jamb ctl click key=preview-toggle
jamb ctl shot path=$PWD/after.png
```

- `jamb dev` stops when the app exits, so a script that ends with `exit` makes
  it a one-shot run. `jamb dev` also stops the app when it gets SIGTERM, so a
  headless app never outlives it unseen.
- A run looks the same on every machine: the platform's settings are the
  defaults (light mode, 100% text, animations on; `JAMB_DARK`,
  `JAMB_TEXT_SCALE` and the other overrides above still apply), the scale is
  `JAMB_HEADLESS_SCALE` (1 by default), and the fonts are the ones the app
  bundles or the system has.
- Frames are drawn when something changes, and at 60 per second while
  something animates, on the real clock, until `jamb ctl clock` drives it
  (see [Clock](#clock)).
- A headless run does not touch the system: the clipboard is the process's
  own, file dialogs answer as canceled and message boxes with no button, links
  are printed instead of opened, notifications are printed, and no tray icon,
  jump list, recent document or AppID is registered. A single-instance app runs
  on its own, beside the user's running copy.
- Without `jamb dev`, a program runs headless when it links package `devtools`
  (`import _ "github.com/tyler-builds/jamb/devtools"`) or needs neither scenes
  nor the control channel.

## Scenes

A scene is a named starting state of the app's UI, such as an empty inbox, a
list of 500 rows or an error, made from synthetic data. Development tools open
the app straight into one, so neither you nor a coding agent has to click
through the app to reach it, and nothing touches the user's real data.

Register scenes with `scene.Add` from an `init` function, in a file that starts
with `//go:build jambdev`. `jamb dev` builds with that tag; `go build` and
release builds don't, so they compile none of the scenes, however many there
are.

```go
//go:build jambdev

package views

import "github.com/tyler-builds/jamb/scene"

func init() {
	scene.Add("Notes/Empty", func() scene.Root {
		s := openTempStore()
		for _, n := range s.Notes.Peek() {
			s.Delete(n)
		}
		return Root(s)
	})
}
```

- A scene's function makes its own state and returns a root, as
  `app.Window.Root` is. It runs each time the scene opens, so every opening
  starts fresh. The scene replaces the root of the app's first window; the
  window's size, bindings, theme and other settings stay the app's own.
  Commands and actions that the app's `main` bound to its own state still act
  on that state, not the scene's.
- Slashes in a name group scenes: `Settings/Accounts/Signed out`.
- `JAMB_SCENE=Notes/Empty jamb dev` opens the app on the scene, and keeps it
  open across `jamb dev`'s restarts. Values that `jamb dev` keeps across a
  restart (see [State across restarts](#state-across-restarts)) are applied over
  the scene's, as over the app's.
- `jamb ctl scenes` lists the scenes and the one shown; `jamb ctl scene
  name=Notes/Empty` opens one in the running app, and `jamb ctl scene` with no
  name goes back to the app's own root.
- A test built with the tag opens a scene with `scene.Make` and renders it:
  `uitest.New(t, root)` (see [Testing your app](testing.md#testing-your-app)).
  Run such tests with `go test -tags jambdev ./...`; CI runs the tests named
  `Scenes` with the tag.
- gopls leaves out files with a build tag it isn't told about. To edit scene
  files with completion, add `"buildFlags": ["-tags=jambdev"]` to its
  settings.
- A program that sets `JAMB_SCENE` without the tag, or without package
  `devtools` linked, stops with an error that says so.

The notes example has scenes in `examples/notes/views/scenes_dev.go`, and a
test that opens each one in `scenes_dev_test.go`.

## Visual inspector

The visual inspector is a window of its own that shows the elements of another
window. Like the browser's developer tools, it shows the element tree, the
style and layout of the selected element, and where each style value came
from. It never changes the layout of the window it inspects.

The inspector is linked only into a program that imports package
`github.com/tyler-builds/jamb/devtools`, so a release build that does not import
it carries none of the inspector's code. `jamb dev` links it into every app it
builds, so the app does not need the import there. An app that wants the
inspector in a release build adds the import to its main package:

```go
import _ "github.com/tyler-builds/jamb/devtools"
```

With the inspector linked, it is on in three cases:

- Under `jamb dev`: press Ctrl+Shift+I in the window.
- With `JAMB_INSPECT=1`: it opens for the first window at start.
- In an app that calls `App.DevTools(true)`, so that Ctrl+Shift+I works in a
  release build as well. Without the import, `DevTools(true)` does nothing, and
  the app prints one warning to stderr that says how to link the inspector. The
  same warning is printed for `JAMB_INSPECT=1` in a program without the import.

Ctrl+Shift+I opens the inspector of the focused window, or closes it again.
The shortcut is taken before the window's own key bindings.

The inspector's window has a toolbar at the top, with the **Pick an element**
toggle, the title of the window it inspects and a filter for the tree. Under it,
a split that can be resized holds the element tree on the left and the details
of the selected element on the right. The details scroll as one pane, with the
frame timings pinned at the bottom. With nothing selected, the details say how
to pick an element.

The inspector shows:

- **The element tree**: on each row, the element's kind, its key in the link
  color, its widget part (such as `Checkbox.Box`), its text in quotes, and the
  short name of the component whose output it is, such as `views.Card`; a
  hidden element is dimmed. A long row ends with an ellipsis. The row's tooltip
  adds the element's role and name in the accessibility tree and the full
  component name. Text in the filter field turns the tree into a
  flat list of the elements whose kind, key, part, text or component contains
  it. Turn on **Pick an element** (or press Ctrl+Shift+C in the inspector), then
  click an element in the window. The press selects it and is not passed to the
  element; Escape ends picking without a selection. While picking, the element
  under the pointer is outlined. A selected element stays outlined in the
  window, with its margin, border and padding drawn as tinted bands (orange,
  yellow and green) and its content box in blue, and a label with its kind, key
  and size. The pointer over a row of the tree outlines that element in the
  window, more lightly, until the pointer leaves the tree. An element with no
  width or no height is marked with a line 2 pixels thick.
- **The header**: the element's kind, key and part, the full name of the
  component that rendered it (selectable, to copy), and the trail of its
  ancestors; a click on an ancestor selects it.
- **Layout**: a box model diagram, as the browser's developer tools draw it:
  the margin, border and padding around the content box, each with its four
  insets. Beside it, the position of the border box in window coordinates, its
  size, the content size and the overflow.
- **Accessibility**: the role, name, description, value, states and actions of
  the element's node, and the warnings that `JAMB_A11Y_WARN` reports for it, as
  warning alerts.
- **Style**: a table of each set field of the resolved style, with its value
  and the layer the value came from. Under it, a row for each value it
  overrides, struck through and marked `overridden`, from the highest layer
  down:

  ```
  Radius   9   instance Styles
           5   overridden: theme
           4   overridden: built-in default
  ```

  A layer that sets the same value as the layer below it is marked
  `same as below`. The sources are:
  - `built-in default`: the widget's own default for the part.
  - `theme`: the theme's style for the part, such as `Theme.Checkbox.Box`. This is
    the theme in effect, a predefined one or the high contrast theme.
  - `instance Styles`: the `Styles` of the element.
  - `element method`: set by a method of the element after its layers, such as
    `Padding`, or by a value the element computes, such as the color of a swatch.
  - `sizing method`: set by a sizing method of the widget, such as `Width`.
  - `state: hover (theme)`, `state: pressed (instance Styles)` and the other
    states: the style that the state applies, and the layer it came from. The
    states of the element are `hover`, `pressed`, `focus`, `focus visible`,
    `focus within`, `drag` and `drop target`; the states of a part that the widget
    applies are `disabled`, `checked`, `mixed`, `selected`, `invalid`,
    `highlighted`, `open` and `current`.
  - `live edit`: set by the inspector.
  - `cleared by <layer>`: the layer cleared the field (see `Clear`).

  A widget that builds a part's style from layers names each layer with
  `ui.StyleLayer`, or `ui.StyleLayerRef` for a layer it passes by pointer, before it
  merges the layer, so the inspector can report it. The built-in widgets do this.
- **Live edits**, at the top of the Style section: choose a style field, enter
  a value and click **Apply** (or press Enter). The value takes effect at once
  in the window. **Clear edits** drops them, and **Copy as Go** copies them to
  the clipboard as a `ui.Style` literal. An edit lasts until its element is
  built again, such as when its component renders.
- **Frames**: tiles with the CPU time of the last frame, the average and peak
  over the last 120 frames, and the number of nodes, rendered components, quads
  and glyphs; the build, layout and paint time of the last frame; and a line of
  the CPU time per frame with the 16.7 ms budget of a 60 Hz frame. The line is
  scaled to the larger of the peak and the budget, so one slow frame does not
  flatten the rest.

The limits:

- Live edits apply to divs only: a text, icon, image or editor is shown but not
  edited. The edit values are a color (`#rrggbb` or `#rrggbbaa`), a length (a
  number of pixels, `50%`, or `2fr`), a number, one or four numbers for edges and
  corners, `true`, or text. A width or height cannot be `auto` in an edit.
- A zero value clears the field only for Background, TextColor, BorderWidth,
  Radius, Corners, Padding, Margin and Gap. Any other field refuses zero, which
  would leave the field as it was. A zero side in four edges keeps the value
  under it, as in `ui.Style.Merge`.
- The layers are recorded only while an inspector is attached, and elements built
  before it attached are rendered again when it does. With no inspector, nothing
  is recorded.
- A value that an element method sets to the same value as the layer below it is
  reported as that layer, because the inspector finds a method's value by
  comparing it with the last layer's value. A method that sets a field to a
  different value is reported as `element method`.
- High contrast overrides that a chart or code editor sets field by field, rather
  than as a layer, are reported as `element method`.
- The window refreshes the inspector about four times a second while it draws.
  Each refresh rebuilds the accessibility tree for the selected element, so a
  very large window costs more while the inspector is open.
- Nothing is written to the code. Copy the literal into the application yourself.
- The inspector is a development tool. Its labels are in English, and they are
  not translated with `ui.Strings`.

## Accessibility checks

`scripts/a11y-check.ps1` checks the demo and notes apps through Windows UI
Automation. It uses Windows PowerShell and the .NET UI Automation client that
come with Windows, and the Go toolchain to build the examples. Run it from the
repository root:

```
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\a11y-check.ps1
```

The script builds both examples into a temporary folder and starts them. The
notes app runs with a temporary notes file, never your own. It runs 42 checks:
names and roles, Invoke, Toggle, ExpandCollapse, Selection, Value, Scroll,
focus with its event, and the Text pattern on the word "small". The demo's form
controls are checked too: a check box's Toggle, including its indeterminate
state; a radio button's SelectionItem; a select's ExpandCollapse and Value, and
its open list; a slider's and a number input's RangeValue with SetValue. The
navigation samples are checked too: a tab list's Tab, TabItem and
SelectionItem, and Select moves the panel; a collapsible header's
ExpandCollapse; a tree's items expanding, with their level and position in
their set; and Down, Right, Left and Up moving the focus through the tree by
keys, with focus events. Axe scans cover the Settings page, the Components page
scrolled to the navigation section, to the form controls, and the same page
with a select open. Each check prints PASS or FAIL, and the script exits 1 if
any fails. Options: `-TimeoutSeconds N` (default 30), `-SkipAxe`,
`-HighContrast` (run the apps with `JAMB_HIGH_CONTRAST=1`) and `-TextScale N`
(run them with `JAMB_TEXT_SCALE=N`, 100 to 225). The overrides are described
under System settings in the Accessibility section. The processes and temporary
files are removed on exit, after a failure too.

When AxeWindowsCLI is installed, the script also scans each app with it, and a scan
that finds errors is a failure. The script looks for it at
`%LOCALAPPDATA%\Programs\AxeWindowsCLI\AxeWindowsCLI.exe` and then on PATH.
AxeWindowsCLI runs the Axe.Windows rules, the engine of Accessibility Insights for
Windows, from the command line. Get it from the official
[axe-windows releases](https://github.com/microsoft/axe-windows/releases) (the
zip needs no elevation). To scan an app by hand while it runs:
`AxeWindowsCLI.exe --processid <pid> --verbosity verbose`. Accessibility Insights
for Windows is the interactive form of the same rules:
[github.com/microsoft/accessibility-insights-windows](https://github.com/microsoft/accessibility-insights-windows).

The manual pass with NVDA and Narrator is not automated. Its steps and expected
results are in [docs/accessibility-testing.md](accessibility-testing.md).
