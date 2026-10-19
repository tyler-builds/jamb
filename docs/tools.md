# Tools for humans and coding agents

The `jamb` command in `cmd/jamb` has two commands: `jamb dev`, the watch-and-restart
runner, and `jamb package`, which builds the Windows release (see [Packaging](packaging.md#packaging)).

These environment variables work with any jamb app, with no code changes:

| Variable | Effect |
|---|---|
| `JAMB_TRACE=1` | Prints startup timings, per-frame build/layout/paint cost, and frames per second of activity, so you can confirm the app is idle. Also turns on the accessibility warnings (see [Accessibility](accessibility.md#accessibility)). |
| `JAMB_A11Y_WARN=1` | Prints accessibility warnings: an interactive element with no name, and an image without `Alt`. Each is reported once. On by default under `jamb dev`. |
| `JAMB_INSPECT=1` | Opens the visual inspector for the first window at start. The inspector is also on under `jamb dev`, where Ctrl+Shift+I opens it (see [Visual inspector](#visual-inspector)). |
| `JAMB_SCRIPT="idle; shot a.png; click 600 400; type hi; key enter; idle; shot b.png; exit"` | Drives the app with synthetic input and saves screenshots. This is how the demo was verified. Besides `click`, the steps include `rclick X Y` (the right button, which opens a context menu), `down X Y`, `move X Y` and `up X Y` for drags, and `drop X Y PATH...` for files dropped from the desktop. The `drop` step goes through the same handler as a real drop, but the Windows message handling itself is only exercised by a real drop. `window N` makes the following steps, and the `shot`s, target the Nth window opened (1-based); the first window is the default. `type TEXT` types everything after the one space that follows `type`, spaces included. `devicelost` simulates a GPU device loss; see [GPU device loss](architecture.md#gpu-device-loss). `a11ydump PATH` writes the accessibility tree as text (see [Accessibility](accessibility.md#accessibility)). |
| `JAMB_STRESS=rebuild\|paint` | Measures sustained frame cost, allocation and GC, prints the results, then exits. |
| `JAMB_VSYNC=0` | Turns off vsync, for throughput tests. |
| `JAMB_D3D_DEBUG=1` | Enables the Direct3D debug layer. |
| `JAMB_TEXT_AA=gray` | Forces grayscale text anti-aliasing instead of ClearType (Windows). |

`JAMB_SCRIPT` keys are `enter`, `escape` (or `esc`), `tab`, `backspace`,
`delete`, `insert`, `space`, the arrows, `home`, `end`, `pageup`, `pagedown`,
`menu` (the Apps key), `f1` to `f12`, `a` to `z`, `0` to `9`, `=` and `-`, with the modifiers `ctrl`,
`shift`, `alt` and `super`. An unknown key or modifier stops the script with an
error when the script is loaded. `idle` waits for `rt.Settled()`, which ignores
repeating animations, so a screen with a spinner does not hold the script.

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

## Visual inspector

The visual inspector is a window of its own that shows the elements of another
window. Like the browser's developer tools, it shows the element tree, the
style and layout of the selected element, and where each style value came
from. It never changes the layout of the window it inspects.

It is on in three cases:

- Under `jamb dev`: press Ctrl+Shift+I in the window.
- With `JAMB_INSPECT=1`: it opens for the first window at start.
- In an app that calls `App.DevTools(true)`, so that Ctrl+Shift+I works in a
  release build as well. Without that call, a release build has no inspector.

Ctrl+Shift+I opens the inspector of the focused window, or closes it again.
The shortcut is taken before the window's own key bindings.

The inspector shows:

- **The element tree**: each element's kind, key, widget part (such as
  `Checkbox.Box`), its text, the component whose output it is, and its role and
  name in the accessibility tree. Click
  **Pick an element**, then click an element in the window. The press selects
  it and is not passed to the element; Escape ends picking without a selection.
  While picking, the element under the pointer is outlined. A selected element
  stays outlined in the window, with its margin, border and padding drawn as
  tinted bands (orange, yellow and green) and its content box in blue, and a
  label with its kind, key and size. The pointer over a row of the tree
  outlines that element in the window, more lightly, until the pointer leaves
  the tree. An element with no width or no height is marked with a line 2
  pixels thick.
- **Layout**: the border box in window coordinates, padding, margin, border,
  content size and overflow.
- **Accessibility**: the role, name, description, value, states and actions of
  the element's node, and the warnings that `JAMB_A11Y_WARN` reports for it.
- **Style**: each set field of the resolved style, with the layer its value came
  from, and the values of the layers it overrides. The winning value comes first,
  and each overridden value follows, marked `overridden`, from the highest layer
  down:

  ```
  Radius = 9   (instance Styles)
    overridden: Radius = 5   (theme)
    overridden: Radius = 4   (built-in default)
  ```

  A layer that sets the same value as the layer below it is marked
  `same value as the layer below`. The sources are:
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
  `ui.StyleLayer` before it merges the layer, so the inspector can report it. The
  built-in widgets do this.
- **Live edits**: choose a style field, enter a value and click **Apply**. The
  value takes effect at once in the window. **Clear edits** drops them, and
  **Copy as Go** copies them to the clipboard as a `ui.Style` literal. An edit
  lasts until its element is built again, such as when its component renders.
- **Frames**: the build, layout and paint time of the last frame, the number of
  nodes, rendered components, quads and glyphs, the average and peak over the
  last 120 frames, and a line of the CPU time per frame.

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
