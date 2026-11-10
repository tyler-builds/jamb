# Changelog

All notable changes to jamb are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
version numbers follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Changes not yet
released are in [changelog/unreleased](changelog/README.md), one file each, until a release merges
them in here.

## [Unreleased]

## [0.8.0] - 2026-11-10

### Added

- Text fields, selectable text and `codeedit` editors have a context menu: Undo, Redo, Cut, Copy,
  Paste and Select all (Copy and Select all for read-only text). A right click focuses the text and
  keeps a selection it lands in, or else moves the caret to it.
- `Segment.ContextMenu` gives a rich text segment, such as a link, its own context menu, which a
  right click opens, and Shift+F10 or the Menu key while the link has focus.
- `markdown.ViewElement.OnLinkMenu` gives every link of a Markdown view a context menu for its
  destination.
- `ui.Style.MergeFrom`, `ui.StyleLayerRef` and `ui.ThemeStyleRef` do what `Style.Merge`,
  `ui.StyleLayer` and `ui.ThemeStyle` do, through pointers and without copying the style. A
  widget whose style is a struct of several `ui.Style` parts uses them to stay small in a web
  build (see "Styles of other widgets on a theme" in docs/theming.md).
- An app runs in a web page, experimentally: a js build opens its window as a canvas that fills the
  page (or `<canvas id="jamb">`), draws on the CPU, and takes the page's mouse, wheel, keyboard and
  touch input and its dark mode, reduced motion and forced colors settings. `scripts/web/build.sh`
  builds an app as a page, with TinyGo or with Go. Input methods, the system clipboard and file
  dialogs are not there yet (see docs/web.md).
- In a web page, text fields take text from a phone's keyboard and from input methods (IME), whose
  window sits by the caret, and copy and paste use the system's clipboard. On Apple devices Command
  works as Ctrl (see docs/web.md).
- In a web page, WebGL 2 draws the frames, about ten times faster than the CPU renderer, which
  draws only in a browser without WebGL 2 or when `JAMB_RENDERER=cpu` asks for it (a page sets it
  in `window.jambEnv`). An app draws again after the browser restores a lost WebGL context.
- `ui.CellGrid` shows a `ui.CellBuffer`, a grid of character cells such as a terminal's
  screen, drawn by the GPU. Its writer may run on any goroutine; a change repaints the grid
  without building anything, and lays out again only the rows that changed.
- `examples/term`, a small terminal for Linux with tabs, scrollback, selection and copy
  and paste, built on `ui.CellGrid`.

### Changed

- A `js` build (a web page, or Node) runs the code editor's highlighter in the JavaScript host's
  WebAssembly instead of wazero, so it no longer links wazero, and TinyGo can build apps that import
  `codeedit` or `markdown/mdcode`. Native builds still use wazero.
- While a context menu has focus, the element it returns focus to no longer gets `OnBlur`; it runs
  when the menu closes, only if focus went elsewhere.
- A right press runs the `OnMouseDown` handlers under the pointer before the context menu opens,
  and presses inside an open context menu no longer reach the `OnMouseDown` of the element it hangs
  from.
- A `DataGrid` that renders again keeps the rows that did not change instead of building every row
  in view: a row with the same text in every cell, in the same state (selected, holding the cursor,
  open), keeps its elements and only takes its new position. A click, a move of the cursor, a new
  selection or a new slice of rows builds the rows it changed. Inserting and removing a row among
  10,000 takes less than half the time it did, and moving one less than a third. A grid whose
  columns have `Cell`, `Edit`, `Editor` or `Interactive`, or that selects cells, builds every row as
  before.
- A `DataGrid` handles a change to its rows faster when it has many of them. It keeps a copy of the
  rows it made keys for, finds the rows a new slice changed by comparing it with that copy, and calls
  the key function for those rows alone. It no longer makes a table of every row's position on each
  change, and checks for a repeated key only among the rows that changed. Inserting and removing a
  row among 100,000 takes a quarter of the time it did. The copy costs memory as large as the rows,
  for rows of up to 128 bytes; larger rows are keyed again on each change, as before.
- Widget styles merge in place, through pointers, instead of as copies. A frame full of widgets
  builds about a third faster (`BenchmarkWidgetFrameNoInspector`: 1.99 to 1.34 ms), with the same
  allocations. Under TinyGo, which turns each copy of a large struct into code for every field, the
  change takes 1.2 MB of code out of a build, so its optimizer can run on jamb.
- `HighContrastTheme` and `Theme.Clone` copy the theme as a whole.
- `charts`, `markdown` and `codeedit` merge their styles through pointers. A web build of the
  demo, which uses all three, is 2.13 MB in Brotli, down from 2.31 MB.
- The widgets pass their looks through pointers: the defaults and the resolved look of a widget
  are filled in place, and its parts reach the elements by pointer. TinyGo passes a struct by value
  as one parameter per field and copies one field by field, so every such call was kilobytes of
  code. A web build of the demo is 2.31 MB in Brotli, down from 2.68 MB, and the smallest app
  1.05 MB, down from 1.12 MB. On the desktop a frame full of widgets builds 6% faster, with the
  same allocations.
- A web page fetches the Go fonts from the `fonts` directory that `scripts/web/build.sh` writes
  next to the app, instead of compiling them in: the smallest app is 1.26 MB in Brotli, down from
  1.57 MB. The page waits only for Go Regular, which it fetches while the app downloads; it fetches
  each other face the first time text uses it, and lays that text out again when it arrives. The
  `jamb_embedfonts` build tag compiles the fonts in, as before (see docs/web.md).
- A web page loads `jamb.js`, the host script that `build.sh` writes beside the app, and gives it
  the app's instance (`jambHost.attach(result.instance)`) before it runs the app. The script
  delivers the display's frames and the pointer, wheel, key and text events by calling functions
  the app exports. A page that does not load it still runs, and says so in the console.
- A web build without the devtools package no longer carries the inspector's window and the
  elements only it uses (tree, split, select and others): the smallest app is 1.12 MB in Brotli,
  down from 1.26 MB.
- An app that draws the icons of package `icons`, and parses no SVG of its own with `icons.New`
  or `ui.SVGFromBytes`, no longer links the XML decoder: the icons are written in a plain form
  that a few lines of code read. The smallest web app is 947 KB in Brotli, down from 958 KB.
- A program that does not import package `devtools` no longer links the functions that record,
  for the inspector, the layer that set each field of a style, or those that paint the
  inspector's marks. The smallest web app is 933 KB in Brotli, down from 947 KB.
- A web build is smaller: the smallest app is 958 KB in Brotli, down from 1.05 MB, and the demo
  1.77 MB, down from 1.88 MB. The build no longer rewrites the text shaper, the map functions
  and every caller of a panic so that goroutines can pause in them, since none of them pauses
  (see `scripts/web/wasm-opt.sh`). It also leaves the names of the app's functions out of
  `app.wasm` and writes them to `app.names`, which the page fetches only when the app stops
  because of an error, to show the stack with the names. `KEEP_NAMES=1` keeps the names in
  `app.wasm` (see docs/web.md).

### Fixed

- An app that imports package `devtools` no longer encodes every `ui.Local` as it is made when
  `jamb dev` did not start it. The demo's Components page, whose data grid holds 100,000 rows in
  a local, opens in 0.4 s in a browser instead of 3 s, and the page uses half the memory or less.
  A web build leaves the package that keeps state across restarts (`devtools/hot`) out.
- A `js` build no longer reads system font files. Under Node it read the host's, so the same build
  drew with different fonts there than in a browser; it now uses the embedded Go fonts and the fonts
  the app registers everywhere.
- A web page no longer pauses to collect memory every few frames while the pointer moves, the
  page scrolls or text is typed. Each call from the page into Go through `syscall/js` made
  references to JavaScript values, and TinyGo collects after every 32 of them, for a time in
  proportion to the app's memory. In the demo, a frame of scrolling the Components page took
  8.8 ms of work on average, with a pause of over 50 ms every seventh frame; it now takes 1 ms,
  with none over 3 ms.
- In a web page, a tap on a list that is still scrolling from a flick stops it and taps what it lands
  on, rather than only stopping the list.
- In a web page, a touch drags sliders, split dividers and the other elements that capture the
  pointer, instead of scrolling the page, and a touch scroll leaves no hover behind. An adapter asks
  the app with the new platform.TouchHandler whether a touch drags what is under it.
- In a web page, a touch scroll and its momentum move once per display frame, by what the finger
  moved since the last frame and by the momentum up to the frame's time, so they no longer judder
  when touch events and frames fall out of step.

## [0.7.1] - 2026-11-08

### Fixed

- Linux: the Vulkan renderer no longer crashes when a frame's text fills the glyph atlas, which then
  grows. Text drawn before and after the growth shares one texture.

## [0.7.0] - 2026-11-08

The `jamb` command for this version is `cmd/jamb/v0.4.3`, which adds the `files` list of `jamb.json`.

### Added

- Linux: windows are drawn on the GPU with Vulkan, loaded at run time, into dma-bufs handed to the
  Wayland compositor. A busy 1080p frame takes about 1.2 ms instead of 11 ms on the CPU. Without
  Vulkan, a qualifying GPU or the compositor's dma-buf support, windows are drawn on the CPU as
  before, and `JAMB_RENDERER=cpu` asks for that.
- Linux: on GNOME, which leaves window frames to apps, windows get rounded corners, a shadow that
  deepens while the window has the focus, and a thin outline, as GNOME's own apps do. The shadow is
  outside the window: snapping and placement leave it out, and the pointer resizes the window from
  just outside its edge. Maximized, fullscreen and snapped edges stay square.
- `app.Window.Corners` chooses rounded, slightly rounded or square window corners. On Windows 11 it
  sets DWM's corner preference.
- `jamb package`: a `files` list in `jamb.json` ships more files beside the executable, such as a
  helper program, in the zip, the installer and the MSIX. Programs and libraries among them are
  signed with the executable.

## [0.6.4] - 2026-11-06

The `jamb` command is released with it as `cmd/jamb/v0.4.2`: `jamb dev` builds with the `jambdev` tag,
which scenes need, and stops the app when it gets SIGTERM.

### Added

- Headless runs: `JAMB_HEADLESS=1` runs an app with no window on the screen. Its windows exist in
  memory and the CPU renderer draws them, so nothing takes the focus or appears on the desktop, and
  `JAMB_SCRIPT`, scenes, the control channel and `jamb dev` work as in a window. A headless run uses
  the default platform settings and touches nothing of the system's: the clipboard is the process's
  own, and no dialog, tray icon, notification or jump list is shown or registered.
  `JAMB_HEADLESS_SCALE` sets its scale. See docs/tools.md, "Headless runs".
- `JAMB_SCRIPT` steps aim at an element by key with `#KEY` in place of `X Y` (`click #save`), and
  the `tree PATH` step writes the accessibility tree as JSON.
- `DivElement.LayoutSpring` and `Style.LayoutSpring` move an element to a new place in the layout as a
  spring: a list reordered, a row added or removed above it, a parent that grew. The element keeps
  where it is drawn and its velocity, so a move mid-flight bends the motion. It is paint-only: nothing
  is laid out again while it moves, and hit testing and `Find` follow it. A scroll, a window resize,
  zoom, text scale and reduced motion never make it spring.
  `DivElement.ChildrenLayoutSpring` gives the spring to a parent's direct children. Debug builds warn
  about a springing element with no key. See Layout springs in docs/animation.md.
- `MotionTokens.Reflow` is the layout spring of the built-in widgets, 350 ms with no bounce by
  default, and `MotionTokens.ReflowSpring` returns it for an app's own lists.
- A layout spring now springs an element's size too. When the layout resizes an element with
  `LayoutSpring` (its own, or from `ChildrenLayoutSpring`), its own box (background, border, shadows,
  corners and focus ring) springs to the new size, keeping its velocity when the size changes
  mid-flight. What it holds keeps its final layout and moves with it, so text never scales, and is
  clipped to the drawn box only when the element clips. Hit testing, `Find`, `DrawnStyle.Bounds` and
  the accessibility bounds follow the drawn box. It is paint-only, and snaps where a move does: on
  first appearance, a window resize, zoom, text scale and reduced motion. See Layout springs in
  docs/animation.md.
- The demo's Motion section has an expanding card sample.
- `Matched(id)` on `Div` and `Button` gives an element a shared-element id.
  When a build removes an element with the id and adds another one with it, anywhere in the window,
  the new element flies from where the old one was drawn, with its velocity, opacity, corner radius
  and fill, to its own place and size, with its `LayoutSpring` or the theme's `Motion.Reflow`. Its
  content keeps its final layout, scaled uniformly to cover the moving frame and clipped to it, so a
  picture zooms without distortion. A flight is drawn above the rest of the window, outside its
  ancestors' opacity and scale but inside the rectangular clips of where it is going, replaces the
  old element's `Exit` and the new one's `Enter`, and can be matched back mid-flight with its
  velocity. Hit testing, `Find` and the accessibility bounds follow it. With reduced motion nothing
  flies. Two elements drawn at once with one id give a warning. See Shared-element motion in
  docs/animation.md.
- The demo's Motion section has a grid of pictures that open on the stage and close back into their
  tiles.
- `uitest.Sample.Matched` holds an element's shared-element id; the motion checks count an element
  that leaves where its match comes, and that match, as neither popping out nor in.
- `uitest.UI.Record` records the frames a test runs, with how every element was drawn in each, and
  `Recording.Check` fails the test on motion a person would notice: an element that pops in or out
  with no transition, is cut off partway through its exit fade, jumps in one frame, overshoots, or
  keeps moving long after the last input. `Recording.Findings` only reports, and `uitest.Allow`
  accepts an intended finding with its reason. See the Motion section of docs/testing.md.
- Scenes: named starting states of an app's UI, made from synthetic data, that development tools
  open the app straight into. Package `scene` registers them (`scene.Add`) from files built with the
  `jambdev` tag, so release builds compile none of them. `JAMB_SCENE=NAME` opens the first window on
  one, and the control channel lists them and switches between them (`jamb ctl scenes`, `jamb ctl
  scene name=...`). `jamb dev` builds with the `jambdev` tag. The notes example has scenes. See
  docs/tools.md, "Scenes".
- `TruncateMiddle()` keeps both ends of a one-line text that does not fit, with "…" in place of the
  middle, for file paths and names, and `TruncateHead()` keeps its end. The cuts fall between
  characters and refit as the text resizes; selecting, copying and the accessible name keep the whole
  text. The last of `Truncate`, `TruncateMiddle`, `TruncateHead` and `MaxLines` wins. The git example
  shows file paths cut in the middle.

### Changed

- `Collapsible` and `Accordion` spring open and closed, and `Sidebar` springs to its new width as it
  collapses and expands, with the theme's `Motion.Reflow`. The box clips its content while it moves,
  so the content is revealed and covered rather than popping in and out; what the widget removes is
  drawn inside the box, fading, until it comes to rest. With reduced motion they still open and close
  at once. Content still being revealed can only be clicked where it is drawn.

### Fixed

- `jamb dev` stops the app when it gets SIGTERM, as it does on Ctrl+C; the app kept running.
- When a toast leaves, the toasts that stay glide to their new places instead of jumping there in one
  frame, and a new toast appears in its place while the others move up to make room.
- A `TimePicker`'s list fades in and out as a `Select`'s and a `Combobox`'s do, instead of appearing
  and disappearing at once.

## [0.6.3] - 2026-11-06

### Changed

- On Linux the caption buttons are GNOME's round ones, and the title bar shows the buttons GNOME's
  `button-layout` setting names, on the side it names. The default title bar is 46 px high, as GNOME's
  header bars are, with the title in bold. New `ui.SystemSettings` fields `RoundCaptionButtons` and
  `CaptionButtons` carry the setting.
- On a Linux desktop that draws window frames, such as KDE, a window with `ChromeCustom` gets the
  desktop's frame above its title bar, and the bar leaves out its caption buttons.
- On Linux and macOS, font files are mapped into memory instead of read into the heap.
- The software renderer used on Linux draws the inside of each box, border and gradient without
  shading every pixel, about twice as fast for a busy frame. The pixels are the same.

### Fixed

- A component that left the tree, and everything it held, can be garbage collected. A long-lived
  `ui.Signal` or `ui.Computed` it had read, such as a cached `ui.Query`'s, or the layout engine's
  reused memory kept a pointer to it, which kept its whole subtree alive.
- A still image drawn smaller than its pixels, such as a 128 px icon at 36 px, is sharp. It is
  resampled once to the device pixels it covers, a large one off the UI thread, and drawn from that
  copy; it was sampled from its full pixels, which aliased. While its size changes it is drawn from
  its full pixels until the size settles.
- On Linux, bold, medium and semibold text in a variable UI font, such as Fedora's Noto Sans, is drawn in
  that font at the weight asked for. It was drawn in the embedded Go fonts, a different typeface.

## [0.6.2] - 2026-11-05

The `jamb` command is released with it as `cmd/jamb/v0.4.1`, with `jamb ctl` and the state that
`jamb dev` keeps across restarts. They need this release of the module: on an older one, `jamb dev`
stops with a message to update it.

### Added

- On desktops that draw no window frames, such as GNOME, a window with the system's title bar
  gets a title bar from jamb, with its title and the caption buttons, which moves, maximizes
  and resizes the window as a custom title bar does. The app needs no code for it. See
  docs/windows.md, "Default title bar". `ui.Runtime.SetDefaultTitleBar` draws it.
- A control channel for coding agents and scripts. `jamb ctl` reads and drives a running
  app: its windows, its accessibility tree, finding elements, clicks, hovering, dragging,
  typing, keys, scrolling, screenshots, and waits for the UI to settle or for an element to
  appear or go, each answered as JSON. Each request sees the effects of the ones before it,
  and input aimed at an element scrolls it into view and waits for it to stop moving. Every
  app that `jamb dev` runs has it open; other programs open it when they link package
  `devtools` and set `JAMB_CONTROL=1`. Release builds that do not import `devtools` carry
  none of it. See docs/tools.md, "Control channel".
- `ui.NewCommand` registers a command with typed input and output for coding agents, which
  the control channel lists with JSON Schemas made from the Go types (`jamb ctl commands`)
  and runs (`jamb ctl run`). The channel also lists and dispatches the actions handled
  where the focus is (`actions`, `dispatch`). The notes example has commands to list, get,
  create, update, delete and select notes.
- `jamb dev` keeps the app's state across its restarts, with no code in the app: every
  `ui.NewSignal` and `ui.Local` has the value it had before the save. Signals are named
  from the source by a rewrite of the copies `jamb dev` builds from, values are copied whole
  (unexported fields and shared pointers too), and a value the app's own startup sets
  differently than last time takes the new value. The focus, carets and scroll positions come
  back too, and a component's locals follow it when it moves. The new build starts beside
  the old one and replaces it once it has drawn, so the window is never gone between them;
  on Wayland, a window session (`xdg-session-management-v1`) puts the new windows where the
  old ones were.
  Release builds carry none of it. See docs/tools.md, "State across restarts".
- `ui.Runtime.ScrollIntoView(id)` scrolls the element of an access tree node into view, as
  an assistive technology's ScrollIntoView does, for tests and tools.
- `app.Window.Unsaved` and `app.Changes`: a window that would lose work by closing asks
  first, Save, Don't save or Cancel, before it closes or the app quits. Every window is
  answered before any closes, Cancel keeps them all open, a failed save offers Try again,
  and a window edited after its answer is asked again. See docs/windows.md, "Unsaved
  changes".
- `ui.Runtime.SetHostLayer` gives the host a layer above the app's UI, for dialogs of its
  own, without changing the app's elements, and `HostLayerShown` reports whether it shows.
- `ui.Strings` has the prompt's words: `SaveChanges`, `DiscardChanges`, `UnsavedLost`,
  `UnsavedNoSave`, `SaveFailed`, `Save`, `DontSave`, `Discard` and `TryAgain`.

### Fixed

- The control channel works with a window that draws no frames of its own: hidden,
  minimized, just opened, or covered by other windows, which GNOME stops drawing. It builds
  and lays out the window's frames without drawing them, so reads after input, waits and
  input aimed at an element answer as on screen. Before, they answered from the tree as it
  was before the input, or timed out, and `jamb dev` waited 10 seconds for each new build on
  GNOME before replacing the old one.
- The key of a `TextInput`, `TextArea`, `Slider` or `ColorPicker` is the automation ID of its
  accessible node. The key stayed on the frame the control renders around that node, so UI
  Automation, `LabelledBy` and `DescribedBy` could not find the control by its key.
- The notes example kept nothing pending after a failed write, so the next save, and the
  save as the app closed, reported success without writing the changes. A failed write now
  leaves them pending, and a failed save as the window closes asks to try again.

## [0.6.1] - 2026-11-04

### Added

- `ui.Baseline` for `Align` and `AlignSelf` in a row lines up the first line of text of each
  item, so text of two sizes sits on one line. See docs/layout.md.
- `Text.Shrink`, as on a `Div`: `Shrink(0)` keeps a text at its full width in a shrinking row.
- `DivElement.OnGridColumns(fn)` reports the number of columns of a grid, as many as an
  `AutoFill` track made, when it is first laid out and each time it changes.

### Fixed

- Scroll containers move their content by whole device pixels. Text and icons each rounded
  their own position to the pixel grid, so a fractional scroll offset moved an icon a pixel
  more or less than the label beside it. In a list that scrolls by a few pixels, such as a
  sidebar that barely overflows, the icons and headings moved while the labels seemed not to.
  At the end of its range an offset rounds up, so the end of the content stays in view.
- A canvas's drawing takes the canvas's `Radius` on its corners. It kept square corners, and
  `ClipContent` on the canvas clipped only its children.
- docs/drawing.md's example of decorative canvas art padded the canvas, which insets the
  drawing; the padding now goes on a child `Div`. The drawing fills the content box, as
  documented on `ui.Canvas`.

## [0.6.0] - 2026-11-04

This release brings preliminary Linux support: windows on Wayland desktops, drawn on the CPU,
with input, the clipboard, file dialogs, notifications, tray icons, system fonts through
fontconfig and color emoji. See docs/status.md for what is missing. Building a desktop app on
Fedora also brought cards, icons from your own SVGs, responsive grid columns and scoped lookups
in `uitest`. Two changes can break code: a `Basis(0)` flex item now keeps its content's minimum
size, as in CSS, and a `uitest` click on a disabled or busy element fails the test.

### Added

- Experimental Linux support for Wayland compositors: windows with server-side or the app's own
  decorations, fractional scaling, mouse and keyboard input with the desktop's keyboard layout and
  compose (dead) keys, the clipboard, the desktop's dark mode, high contrast and reduced motion
  settings, file dialogs through the desktop portal, and OS notifications through the desktop's
  notification server. Frames are drawn on the CPU. It needs
  libxkbcommon, which every Wayland desktop has; it is loaded at run time, so builds still need no
  cgo. See docs/status.md for what is missing.
- Tray icons on Linux: `app.NewTray` shows a StatusNotifierItem with a dbusmenu menu, which KDE
  Plasma and most panels host (GNOME with the AppIndicator extension). `CloseToTray` works with it.
  `Show` prints an error and leaves the icon hidden when the desktop has no tray.
- `App.KeepRunning(true)` keeps an app running after its last window closes, and `app.Quit()`
  closes every window (asking their `OnClose`) and ends the app.
- With `App.KeepRunning(true)`, `Run(app.Window{})` starts the app with no window, as a tray app
  launched at login does; it opens its windows later with `app.Open`.
- `uitest.UI.Screenshot()` returns a frame as an image, drawn by the CPU renderer, with no
  window or GPU, on every platform.
- `Stepper.Compact()` shows only the current step's title, for narrow windows.
- `Switch.ShowLabel()` draws the switch's label beside it; a click on the label toggles it.
- On Linux, `App.AppID` sets the Wayland app ID of the windows (the executable's name without
  it), so the desktop matches the app to its `.desktop` file without `StartupWMClass`.
- `examples/budget`, a household budget: accounts, transactions sorted into categories, a
  budget for each category each month, an editable transaction table, forms with validation,
  undo, and CSV import. See docs/examples.md.
- `ListOf` and `UniformList` take `Label`, which names the list for assistive technology.
- `Chart.TabStop(false)` leaves a chart and its legend out of the Tab order, for a sparkline
  that decorates a number. A click still focuses it, and assistive technology still reads it.
- `DataGrid.SortChildrenOnly()` keeps the top rows of a tree in the order given and sorts only
  the rows under them, for sections that keep their place while the rows in them sort.
- `ui.FormatCurrency(amount)` shows an amount of money in the currency of the user's region
  settings, as Windows formats it: the symbol, its position, the shape of a negative amount
  and the digits. `ui.CurrencySymbol()` returns the symbol, and `NumberOptions.Currency` makes
  a number field an amount of money that shows the symbol. See docs/text.md.
- `ui.ParseNumber(text)` and `ui.ParseDate(text)` read numbers, amounts and dates written in
  the user's locale, the counterparts of `FormatNumber`, `FormatCurrency` and `FormatDate`.
  `ParseDate` reads ISO 8601 dates too.
- On Linux, fonts come from fontconfig (libfontconfig, loaded at run time), so Fedora-family
  systems get their system fonts instead of the embedded Go fonts. A character the font lacks
  falls back to a font fontconfig finds for it, and emoji to the desktop's emoji font.
- Color glyphs off Windows: COLRv0, COLRv1 (gradients, transforms and composites; the hue,
  saturation, color and luminosity modes draw as normal) and CBDT/sbix bitmaps, so color emoji
  show on Linux. Variable fonts use their default instance.
- `App.FallbackFont(data)` adds a font for the characters a text's font lacks, tried before the
  platform's fallback fonts. See docs/text.md.
- `ui.AutoFill(min)` as a grid's only column track makes as many equal columns as fit the
  grid's width, each at least `min` wide, as CSS `repeat(auto-fill, minmax(min, 1fr))` does.
  See docs/layout.md.
- `ui.Card()`, a filled or outlined surface for related content. With `OnClick` the whole card
  is one button. Its look is `CardStyle`, in `Theme.Card` and per variant in `Theme.Cards`.
- `icons.New(name, svg)` and `icons.MustNew` make an icon from a whole SVG document, such as a
  brand mark; `icons.KeepColors` keeps its own colors instead of tinting it. See docs/drawing.md.
- `access.StateBusy`, set on a loading `Button` and exposed as aria-busy through UI Automation.
- `uitest`: `HasRole` and `HasName` never fail, so they can be polled in `WaitFor`;
  `Element.ByRole`, `ByName` and `HasRole` search inside an element, to tell apart two controls
  of the same name; `WaitForQueries` waits for `ui.Query` fetches. `Runtime.Fetching` reports
  a fetch in flight.
- `Button.IconEnd`, `Badge.Tooltip` (also the badge's accessible description),
  `ProgressBar.HideValue`, `ui.SidebarDivider()` with the `Divider` part,
  `Sidebar.NoIndicator()`, and `Basis` and margins on `Text`.

### Changed

- A second `app.Run` (or `RunE`) in a process returns an error instead of returning at once
  without a window. Use `App.KeepRunning` and `app.Open` to show windows one after another.
- The vector rasterizer, which draws charts, SVG images and canvases, keeps the edges that
  cross a row of pixels in order from one sample of the row to the next, instead of sorting
  them afresh at each of its 16 samples, and sorts the edges into rows in linear time. It
  draws the same pixels. A chart's plot redraws about 2.5 times as fast: a 100,000-point line
  in 5 ms instead of 12.3, an area in 7.5 ms instead of 18, and the system monitor's resize
  frame fell from 6.6 to 4.0 ms. A stroke allocates a third less.
- Flex items follow CSS for an explicit `Basis`: an item keeps its automatic minimum, so a
  `Basis(0)` child of an auto-height column is as tall as its content instead of 0 px, and
  growing items start from their basis and stop at their minimum. Equal `Basis(0)` columns stay
  equal until one reaches its longest word, which it now keeps. A row that sizes to its
  content sizes a growing `Basis(0)` item from its content, so a centered page no longer
  shrinks to its fixed items.
- `uitest` `Click` and `DoubleClick` fail the test on a disabled or busy element, instead of
  doing nothing.
- The `JAMB_SCRIPT` `idle` step also waits for `ui.Query` fetches, and the `wait` step is
  documented.

### Fixed

- A `Select` whose chosen label or placeholder is wider than the field ends the text with an
  ellipsis. The text pushed the chevron past the field's right edge; a `RenderValue` that does
  not fit is now clipped.
- A chart with `LegendRight` keeps its legend inside its box. A legend wider than the room
  beside the plot pushed past the chart's right edge; the plot now takes the width the legend
  leaves, and a legend taller than the chart scrolls.
- A combo box takes focus for its text when assistive technology asks to focus it, as UI
  Automation's SetFocus on a combo box does. It offered no focus, so it could only be reached
  with Tab.
- `ui.FormatNumber` rounds as Windows does, half away from zero on the digits of the shortest
  decimal form of the value: 1.005 at two places is 1.01, and 0.125 is 0.13. It rounded the
  binary value, half to even, so both came out a cent low.
- A keyed component or composite element, such as a chart, kept answering to its key in tests
  and in the inspector only until a component inside it rendered again. A chart's key was lost
  on the first frame of its enter animation.
- Spaces at the start of a span stay in the fallback font of the text before them, so right-to-left
  text that a fallback font draws keeps its order across spans.

## [0.5.0] - 2026-10-23

This release fixes the magnified content that v0.4.0 showed while a window's edge was dragged,
and adds what a new example app needed. Building a system monitor (`examples/monitor`, in the
style of Task Manager) brought tree rows in the data grid, charts with hidden axes for
sparklines, numbers and byte sizes in the user's locale, and the icons Windows shows for files.
Floating dock panels now belong to their dock's window, with its theme, zoom and shortcuts, and
dock back where they came from. Nothing is breaking.

### Added

- `examples/monitor`, a system monitor: live charts of the processors, memory, network and I/O,
  a process table sampled every second, alerts as Windows notifications, a notification area
  icon and a taskbar badge, in panels that can be rearranged and floated. See docs/examples.md.
- `charts.Axis.Hidden` hides an axis: no axis line, tick labels or title, and no margin for them.
  With `GridAuto` a hidden axis draws no grid lines, and `GridOn` still draws them. A line or area
  with both axes hidden, no legend and no title is a sparkline, and a chart sized to less than 120
  pixels high is drawn at its own height. See docs/charts.md.
- `ui.FormatNumber` shows a number rounded to a number of decimals, with the group separator,
  decimal point and minus sign of the user's locale, as `NumberFormat` does. `ui.FormatBytes`
  shows a size in bytes as Windows Explorer shows it, such as "1.23 GB". Its units are the
  `Byte`, `Bytes` and `KB` to `PB` fields of `Strings`, English in every built-in pack. See
  docs/text.md.
- Tree rows in `ui.DataGrid`: `Children(fn)` makes the rows a tree, with a chevron that opens and
  closes a row in the first column, and `Expanded`, `OnExpand` and `OnCollapse` keep and report
  the open rows as `ui.Tree` does. Siblings sort within their level, Left and Right open and close
  rows, and assistive technology reports each row's level and place among its siblings. The
  indent and chevron are the `TreeIndent` and `TreeChevron` parts of `DataGridStyle`. See
  docs/widgets-data.md.
- `app.FileIcon(path, size)` returns the icon Windows shows for a file, as File Explorer and
  Task Manager show it: an executable's own icon at the size asked for, or the icon of the file's
  type. See docs/os-integration.md.

### Changed

- A floating dock panel docks back where it was before it floated, as a tab of the group it
  left or as a group beside the one it was next to, when its window closes or with Dock back.
  Before, it docked along the right edge. The place is kept in the new `DockFloat.Home`.
- The window of a floating dock group is part of the dock's window. It shows that window's
  theme, text and zoom and keeps them in step both ways, uses its key bindings, and passes the
  actions it does not handle on to it, from the dock up. Before, a floated panel followed the
  system theme at 100 percent zoom, and the app's shortcuts did nothing in it.

### Fixed

- A window no longer shows its content magnified while the user drags its edge. The swap chain
  kept larger buffers during the drag and showed part of them with
  `IDXGISwapChain2::SetSourceSize`, which Windows scales to the window. The frame is now drawn in
  the buffers' top-left corner, which the unscaled swap chain shows 1:1, so the drag keeps its
  headroom without the scaling.

## [0.4.0] - 2026-10-23

The code editor highlights with @pierre/highlights instead of tree-sitter: 76 languages built in,
without the jamb-grammars module. This changes the highlighting API. The data grid gains movable,
pinned and grouped columns, cell selection, context menus and loading more rows; tabs scroll when
they overflow; buttons get a loading state; `AlertDialog` is new; text inputs format as the user
types; the code editor wraps lines and takes diagnostics, hovers, completions and definitions from
the app; and animations gain springs, keyframes, timelines, delays and staggers. Dialogs and sheets
can show a close button and stay out of the way as non-modal panels, toasts can be updated and
dismissed by ID, trees open and close from code, and the code editor offers quick fixes.

Building a Git history browser (`examples/git`) brought a second set: `ui.Query` loads data as
TanStack Query does, `ListBox`, `TitleBar` and `CopyButton` are new, times read "3 days ago" in
every language jamb ships, tabs can close, trees take menus and trailing content, the command
palette lists the app's own items, and the icons are the whole Lucide set. Several defaults
changed with them: split grips show on hover, clickable `Div`s get a focus ring, and segmented
tabs are neutral.

### Changed

- **Breaking:** The icons are the Lucide set (1.61.0, 1,903 icons, ISC license), generated from its
  SVG files, instead of 58 hand-drawn ones. Every old name still works: most map to the Lucide icon of
  the same name, and `Alert`, `Dashboard`, `Filter`, `Home`, `MoreHorizontal` and `Refresh` are
  aliases of `TriangleAlert`, `LayoutDashboard`, `Funnel`, `House`, `Ellipsis` and `RotateCw`. The
  drawings change slightly at each call site. `icons.All` is now a function, and `Icon.StrokeWidth`
  is gone. An icon the app never names is not linked; drawing any icon links the SVG engine, so the
  hello app's binary grew by 213 KB (2.5%), and a program that names every icon is 2.6 MB.
- **Breaking:** A `Split`'s grip shows only while the pointer is over the divider, or while it is
  dragged or focused (`SplitGripOnHover` is the default and the zero value). `Grip(ui.SplitGripShown)`
  keeps it drawn. The values of the `SplitGrip` constants changed with the new zero value.
- **Breaking:** Segmented tabs (`TabsSegmented`) are neutral by default: a raised segment on a sunken
  track, not an accent fill. High contrast keeps the system colors. `Styles` with an accent
  `TabSelected` brings the fill back; docs/widgets-navigation.md shows how.
- A focusable `Div` with `OnClick` that sets no `FocusVisible` style of its own draws the standard
  focus ring while it has keyboard focus.
- A `DescriptionList`'s term column is as wide as its widest term, up to half the list, instead of
  120 px.
- A `DataGrid` cell's render slot fills the row's height, so content sized with `ui.Full`, such as
  a canvas, fills the cell.
- `Text.Weight` is deprecated in favor of `Text.FontWeight`, the name `Div` uses.
- A `Split` divider takes only its bar's thickness in the layout, 1 px by default, instead of 9 px, so
  the panes meet at the bar and it reads as the border between them. Each pane is 8 px wider than
  before beside each divider. The 9 px hit area is unchanged in size; it lies over the panes' edges,
  as VS Code's and Zed's do, so a press a few pixels into a pane beside the bar drags the divider.
- **Breaking:** The visual inspector is linked only into a program that imports
  `github.com/tyler-builds/jamb/devtools`. Every app used to carry the inspector's code. An app
  that calls `App.DevTools(true)` and expects Ctrl+Shift+I to work in a release build must add
  `_ "github.com/tyler-builds/jamb/devtools"` to its main package. Without the import,
  `DevTools(true)` and `JAMB_INSPECT=1` do nothing, and the program prints one warning to stderr
  that says how to link it. Ctrl+Shift+I then passes to the window. `jamb dev` links the inspector
  into the apps it builds, so they need no import. The stripped hello binary is 780 KB smaller.
- **Breaking:** `codeedit.EditorElement.Language` takes the name of a language, as a code fence
  names it, such as `"go"`, `"python"` or `"ts"`, instead of a `highlight.Grammar`. Replace
  `Language(golang.Grammar)` with `Language("go")`. An unknown name shows plain text, with
  `highlight.ErrUnknownLanguage`.
- **Breaking:** `highlight.NewSession` takes a language name in place of a `highlight.Grammar`.
- **Breaking:** `mdcode.Highlight()` takes no grammars, and colors every language the highlighter
  knows. It lexes a block while the view renders (some microseconds for a block of a few
  kilobytes), so it no longer runs workers or calls `ready`.
- The code editor highlights with
  [@pierre/highlights](https://github.com/pierrecomputer/pierre/tree/main/packages/highlights)
  0.1.3, the syntax highlighter of the Pierre Computer Company: one WebAssembly module of 0.17 MB,
  embedded in `codeedit`, with a lexer for each of 76 languages. It runs in wazero as tree-sitter
  did. Measured on real files against the tree-sitter grammars, it highlights a whole file 50 to
  100 times faster, relexes after a keystroke in about 12 µs, and its copy of a document takes 2.6
  to 13 times less memory (see BENCHMARKS.md). An app with all languages is 20 MB smaller than one
  with the 18 grammars. It lexes rather than parses, so some colors differ from tree-sitter's: in Go,
  for one, it colors method calls as functions, and can color `pkg.Name` as a property where it is
  a type.
- Syntax token names are those of Zed's themes, as jamb's themes already use (`keyword.control`,
  `function.definition`, `property.json_key` and so on). JSON keys take the theme's `property`
  style, as in Zed, where tree-sitter named them `string.special.key`. `highlight.Closest` (and the
  Markdown view) gives CSS's `selector`, `selector.class`, `selector.id` and `selector.pseudo` the
  styles of `tag`, `property` and `attribute` when a theme does not style them.
- `highlight.DefaultMaxBytes` is 16 MB (was 5 MB).
- The session's spans no longer overlap, and leave out the spaces around a token.
- `compare-gpui.ps1` measures on a non-primary monitor without activating the windows. Its pointer
  phase posts mouse messages instead of moving the cursor, and its focused idle sample runs only
  with `-IncludeFocused`. `-Position` sets where the windows open.
- Rebuilding an element where it was keeps its node, instead of allocating a new one and retiring
  the old one, and the node and element structs are reordered to smaller allocation sizes. A full
  re-render of 10,000 nodes makes 35.8k allocations instead of 61.9k, and allocates 9.7 MiB instead
  of 24.3 MiB (see BENCHMARKS.md).
- A `ListOf` row is a component of its own, kept in the window while it stays in view. Moving the
  window builds only the rows that come into it, and a scroll step over 100,000 rows makes 285
  allocations instead of 547. A signal read in a row re-renders that row, not the list. A row that
  stays in the window is not rendered again when the window moves; a new item slice, or a new
  `ListElement` from the parent's render, renders every row, as before.
- The top modal is kept for the build it was found in, instead of being searched for on every key
  press and mouse press. A key press in a 10,000-node tree takes 0.47 µs, against 273 µs before.
- A `Tree` or `DataGrid` takes the space its parent leaves it and shows the rows that fit, instead of
  all of its rows. This is a behaviour change: a tree of 10,100 rows takes 2.0 ms a frame, against
  80.5 ms, and a parent with no bounded height still shows every row.
- A `DataGrid` builds its table of row positions only when keys can repeat. A sort of a grid whose
  keys are sorted and unique does not build it.
- Solid fills write opaque pixels directly and a canvas reuses one shader, so the code editor's
  scroll frame over 50,000 lines takes about 570 µs, against 720 µs on 2026-10-21, and a wrapped
  keystroke takes 0.80 ms, against 1.51 ms.
- The built-in themes and the 58 built-in icons are built on first use, not when the package starts,
  which makes the stripped hello binary about 660 KB smaller.
- Font files are parsed with the outlines of their glyphs cut, since shaping needs only each glyph's
  bounding box and jamb reads the outlines from the file itself. The hello app's private memory is
  about 2.5 MB smaller when idle. The binary grows by 12 KB.
- While the user drags a window's edge, its swap chain buffers keep room to spare, and a size they
  still hold moves only the part of them that is shown; the end of the drag fits the buffers to the
  window again. Any other change of size, such as a maximize, a restore or a resize from code, takes
  buffers of exactly the window's size: with room to spare at rest, a window maximized by a
  double-click on its title bar and then restored was shown zoomed in. A window does not paint again
  after a size change it has already shown.
- The renderer's instance buffers start at 4 KB, not 64 KB, and grow as before.
- A window's first frame is built while its GPU device is created, and is drawn before the window
  is shown. In the commit measurements, the blank gap of a warm launch fell from a median of 36.5 ms
  to 12.6 ms. The live startup benchmark did not improve (see BENCHMARKS.md).
- The Activity filter narrows its list incrementally, and the Components page builds its 10,000-folder
  tree once, not on each visit. Its 100,000 DataGrid runs are no longer built at startup, which saved
  about 12 MB of heap in every launch; the first visit to the page builds them, in about 3.5 ms.
- `jamb dev` links the inspector with `go build -overlay`, so the app's own directory is not written to.
- BENCHMARKS.md leads with the results of 2026-10-22, against 2026-10-21: the changes behind them,
  what was tried and dropped, and the known issues and gaps. The 2026-10-21 results follow under their
  own heading. The GPUI comparison has a fourth pass (2026-10-22), and the README and the
  getting-started guide quote its figures.

### Added

- `ui.Query` loads data for a component, as TanStack Query does: a per-window cache by key, one
  fetch per key however many components use it, loading, error and refetch state, retries with
  backoff, stale times, keeping the previous key's data while the next loads, invalidation and
  `SetQueryData`. The fetch runs off the UI thread with a context that is canceled when nothing
  uses the key and when the window closes. See docs/data.md.
- `ui.ListBox` is a list the user selects from, single or multiple, with the WAI-ARIA listbox keys,
  type-ahead, a context menu and UI Automation selection, built only in view. Its selection is a
  signal of keys, or a single key in a string signal (`SelectedKey`).
- `ui.TitleBar` is a container for a window's own title bar: it drags the window, dims while the
  window is in the background, ends with the caption buttons, and centers an optional `Center`
  element on the whole window, never over the sides. It has no fill or border of its own, so it is
  part of the window's content unless the app styles it.
- `ui.CopyButton` copies text, then shows "Copied" (or the text given) in place for 1.5 s and
  announces it. A toast is optional.
- `ui.FormatRelative` and `ui.TimeAgo` show a time as "3 days ago", "yesterday" or "in 2 hours", in
  English, Italian and Chinese, from CLDR's relative time patterns. `TimeAgo` updates itself as
  time passes, and its tooltip is the full date and time.
- Tabs can close: `Tab.Closable` and `Tabs.OnClose`, with a close button, a middle-click, Ctrl+W,
  Ctrl+F4 and Delete.
- `Tree.ContextMenu`, `TreeNode.Description` and `Tree.RenderTrailing` (counts and badges at the end
  of a row).
- `CommandPalette.Items` lists the app's own items (`PaletteItem`) before the commands, and
  `ItemsOnly` makes the palette a quick open, as VS Code's Ctrl+P is; ">" switches to commands.
- `Div.PaddingTop`, `PaddingRight`, `PaddingBottom`, `PaddingLeft`, `MarginLeft` and `MarginRight`.
- `ui.AvatarInitials` and `ui.AvatarColor`, the initials and fill an `Avatar` uses for a name.
- `Tag.Color` tints a tag with a category color.
- `TextInput.OnKeyDown` and `TextArea.OnKeyDown` run before the field's own keys.
- `DataColumn.CellStyle` styles one column's cells, such as clearing their padding.
- A toast shown in a window with no `Toaster` prints a development warning, once.
- `highlight.Languages`, `highlight.Supported` and `highlight.CaptureNames` list and check the
  language names and the token names, and `highlight.Highlight(lang, code)` colors a text that is
  not edited in one call.

- `DataGrid` keeps the order and widths of its columns in signals (`ColumnOrder`, `ColumnWidths`)
  and reports each finished change through `OnColumnsChange`, so an app can save the layout. A
  saved order that names unknown columns, or misses new ones, still applies.
- `DataColumn.Movable` (or `DataGrid.Movable()` for every column) lets the user move a column by
  dragging its header, or with Ctrl+Shift+Left and Right on the header, with the new position
  announced. Columns move only among those with the same pin.
- `DataColumn.Pin` keeps a column at the start (`PinStart`) or the end (`PinEnd`) of the grid while
  the other columns scroll sideways. Pinned cells keep the row's hover, selection and stripes.
- `DataColumn.Group` puts neighbouring columns under a shared header in a row above theirs; screen
  readers hear the group as each column header's description.
- `DataGrid.ContextMenu` and `HeaderContextMenu` build menus for a cell and a header, opened by a
  right-click, Shift+F10 or the Menu key. A right-click on a row that is not selected selects it.
- `DataGrid.LoadMore(hasMore, fn)` asks for more rows when the user nears the end, and
  `LoadingMore` shows a row with a busy ring at the end while they load.
- `DataGrid.CellSelection` selects cells instead of rows: click, Ctrl+click and Shift+click, Shift
  with the arrow keys for a rectangle, Shift+Space for a row, Ctrl+Space for a column and Ctrl+A
  for all, as `CellKey` values in a signal.
- `DataGridHandle` (`NewDataGridHandle`, `DataGrid.Handle`) scrolls a grid to a row or a column, or
  focuses a cell, from code.
- A grid whose columns are wider than its viewport builds only the columns in view, plus the pinned
  ones and the focused and edited cells.
- Tabs that don't fit scroll sideways, with the wheel and with arrow buttons at the ends of the
  strip (`ScrollButtons`), and the selected tab is scrolled into view. `OverflowMenu` adds a button
  that lists every tab, `TabMaxWidth` truncates long labels (the full label stays the tab's name and
  tooltip), and `Prefix` and `Suffix` put content before and after the strip.
- `Button.Loading` and `SplitButton.Loading` show a busy ring and ignore clicks, keys and the
  modal's Enter while keeping focus and their width. Screen readers hear "Loading" as the button's
  status.
- `AlertDialog(title, message)` asks the user to confirm or acknowledge something. `Intent` picks
  info, confirm, warning or destructive, which sets the icon and the confirm button's look; a
  destructive dialog focuses Cancel and does not confirm on Enter from elsewhere. `Confirm`,
  `Cancel` and `Action` set the buttons, Escape cancels, a click outside does nothing, and
  `ConfirmLoading` shows the confirm button busy. Screen readers announce it as an alert dialog
  (`access.RoleAlertDialog`).
- `TextInput.Format` formats the text as the user types while the signal keeps the raw value:
  `TemplateFormat("(999) 999-9999")` fills slots for digits, letters or any character between
  literals that appear as needed, and `NumberFormat` groups digits and places the decimal point in
  the user's locale. `InputFormat.Complete` validates a finished value, `Display` formats a value
  for elsewhere, and `TextInput.Accept` rejects any edit that would make a value it refuses.
- The code editor wraps long lines with `SoftWrap` (at the view's width, a column, or the
  narrower of the two), with `WrapColumn` and `WrappingIndent`. Up, Down, Home and End move by the
  lines as shown, and screen readers read them as lines.
- `Document.SetDiagnostics` underlines errors, warnings, information and hints in every editor of
  the document, with a gutter marker whose shape gives the severity. They follow edits, F8 and
  Shift+F8 go to the next and previous one, and `SetDiagnosticsFor` drops a set computed for an
  older text.
- Editors ask an app's providers for hovers (`Hover`, shown after `HoverDelay` or with Ctrl+K
  Ctrl+I), completions (`Completions`, on a trigger character or Ctrl+Space) and definitions
  (`Definitions`, with F12 or Ctrl+click; another document goes to `OnOpenLocation`). Requests run
  on goroutines and are cancelled by the next edit. `Document.TextVersion`, `UTF16Pos` and
  `OffsetUTF16` help connect a language server.
- Springs: `Style.Spring` and `Div.Spring` move opacity, offset and scale like a spring, set by a
  duration and a bounce, keeping their speed when the target changes on the way. `ease.Spring`
  gives the same feel to any animation.
- `Div.Play` plays keyframes (`Keyframe`, with `Motion.Opacity` for partial opacity), and
  `Sequence`, `Parallel`, `Then` and `Repeat` build timelines for `AnimateTimeline`.
- `Animation.Delay`, `Animation.Stagger` (siblings that appear together enter one after another)
  and `Animation.OnDone`.
- `Theme.Motion` sets the durations of the built-in widgets' hover, enter and exit motion, and a
  spring for their hover transitions; zero values keep today's.
- Under reduced motion, springs, keyframes, timelines, delays and staggers jump to their end (a
  repeating one holds its first frame).
- Editors ask a `CodeActionProvider` (`CodeActions`) for quick fixes, refactorings and source
  actions. A lightbulb in the gutter marks a line with a problem that has a fix; Ctrl+., a click on
  the lightbulb or the hover's Quick fix row opens a menu grouped by kind, preferred actions first
  and unavailable ones dimmed with their reason. An action's edits apply as one undo step, and its
  `Run` handles anything outside the document.
- `Modal.CloseButton` and `Drawer.CloseButton` add a close button to the header, named Close for
  screen readers. It is off by default.
- `Modal.NonModal` and `Drawer.NonModal` show a dialog or a sheet without a backdrop that leaves the
  rest of the window usable: Tab moves in and out of it, F6 cycles through the window, the open
  dialogs and the toasts, and Escape and Enter act on it while it has the focus. A press raises it
  above the others. It shows a close button by default, closes on a press outside only with
  `CloseOnClickOutside`, and screen readers report it as not modal.
- `Context.ToastWithID` shows a toast under an ID, or replaces the toast showing with that ID in
  place, keeping its position and focus and restarting its time; `Context.DismissToast` removes
  it. `Toast.Title` adds a title above the message. Updates are announced at most once every two
  seconds, and a newer one replaces those still waiting.
- `access.Announcement.Source` names the source of an announcement, passed to UI Automation as the
  activity ID.
- `Tree.Expanded` keeps the open nodes in a signal the app can set, and `OnExpand` and
  `OnCollapse` report each node the user or the app opens or closes. `TreeHandle` (`NewTreeHandle`,
  `Tree.Handle`) expands, collapses, reveals and selects nodes from code.
- `ui.Runtime.FrameRequested` reports whether a frame is wanted when the call is made. The frame
  statistics use it to leave frames that follow an idle gap out of the present intervals.
- Dev hooks for measurements: `JAMB_NOACTIVATE=1` shows every window without activating it, and
  `JAMB_POSITION="X,Y"` places the first window's top-left corner at a logical position, negative
  values included. `JAMB_FRAMESTATS=PATH` records every frame of the first window, with its CPU time,
  the interval between presents and the latency of each scripted input event. `JAMB_HEAPLOG=PATH`
  writes the memory counters every `JAMB_HEAPLOG_SECONDS` seconds, for soak runs. The `mark` and
  `repeat N` steps of `JAMB_SCRIPT` loop a sequence (`repeat 0` until the window closes). See
  [docs/tools.md](docs/tools.md).
- `scripts/bench-live.ps1` measures a running demo and hello on a secondary monitor: startup, idle
  CPU and memory, scripted scenarios with frame and input latency, resizing, stress, a second window
  and an optional soak. Its windows never take focus or the cursor.
- `scripts/bench-go.ps1` runs every Go benchmark in the workspace a set number of times, summarizes
  the results with benchstat, and can compare them with an earlier run.
- An advisory benchmark workflow compares the Go benchmarks of each pull request with its base and
  writes the comparison to the job summary. It reports slowdowns and never fails a pull request.
- Go benchmarks for hit testing, key dispatch, text input typing, expanded trees, animations, the
  accessibility tree and its updates, theme and page switches, glyph rasterization, and layout of
  Latin, bidi and complex scripts.

### Fixed

- A stretched flex item keeps its minimum and maximum size on the cross axis, as in CSS. A
  `MinHeight` of 32 px on a child stretched in a 24 px row used to give it 24 px.
- A `Split` measures its own box. It used to find its box by searching the window for its key, so
  of two splits with the same key in different parts of the tree (keys are unique only among
  siblings) both measured the first. When that one was hidden, as in an app that keeps a hidden view
  per document, the shown split had no length and ignored a drag of its divider.
- `ui.TitleBarButtons` take the height of the title bar they are in. They were always 32 px high, so
  in a taller bar they sat in its middle with the bar above and below them, and the close button's
  red did not reach the window's corner. A bar sized by its content still gets 32 px buttons.
- A window's first frame is drawn before the window is shown. Since an earlier change, the frame was
  skipped while the window was hidden, so the window appeared with no content and was painted only
  after it was shown.
- `JAMB_FRAMESTATS` reports the present intervals of a `JAMB_STRESS` run. Before, a stress run had
  no continuous frames, so the intervals were left out.
- The code editor kept the syntax colors of the theme it first painted with: after a switch from
  the dark theme to the light one, its text stayed in the dark theme's pale colors. It now recolors
  the text when the theme changes.

- Ending a drag of a `DataGrid` column's edge over its header also sorted the column. It no longer
  does.

- A key binding on Ctrl+Space typed a space as well as running its action, because Windows sends
  the space with the key. The binding now drops that space.

- `Toast.Dismiss` on a toast that had the focus lost the focus. It now moves the focus on, as the
  toast's close button does.
- A window at a negative position could be placed a pixel off on a monitor scaled above 100%,
  because the scaling rounded toward zero for negative values. Halves now round away from zero, as
  positive values do. A window moved to a monitor with another scale before its creation finished
  now receives the change once it is ready.

### Removed

- **Breaking:** `highlight.Grammar`, `highlight.NewGrammar` and `highlight.ABI`, and jamb's
  dependency on `github.com/tyler-builds/jamb-grammars`. The tree-sitter highlighter moves to the
  `treesitter` package of jamb-grammars, for apps that want colors from a syntax tree; it plugs
  into the editor with `EditorElement.Highlighter`.

## [0.3.0] - 2026-10-19

Nineteen widgets, a Markdown view, more of `Tabs` and `TextInput`, a new look for the visual
inspector, and a fix for fonts kept in memory after their window closed. No API is removed or
renamed.

### Added

- A `markdown` package: `markdown.View(src)` shows CommonMark with GitHub's tables, task lists,
  strikethrough, autolinks and footnotes, parsed with goldmark. Headings, links, lists and task
  states are accessible, text is selectable per block, and raw HTML is shown as text. Links open with
  the system or call `OnLink`; images come from an app's loader or local files under `BaseDir`, and
  the view never downloads anything. It is styled through `markdown.Style`, a theme slot and
  `Styles`. The guide is [docs/markdown.md](docs/markdown.md).
- `markdown/mdcode` colors fenced code blocks with the code editor's tree-sitter grammars. It is a
  separate package, so an app that does not import it links neither the editor nor wazero.
- Markdown tables are data grids that screen readers navigate by row and column, with the header row
  as their column headers, and list items report their position ("2 of 5") and level.
- `AccessGrid`, `AccessCell`, `AccessPosition` and `AccessLevel` on `Div` let any element describe a
  table, a cell, or an item's place in a set and its nesting level to assistive technology.
- `Tabs.OnReselect` runs when the selected tab is activated again, and `Tab.Popup` shows any
  content the app builds in a floating panel under the tab. It opens on a second click on the
  selected tab, or with Down, Shift+F10 or the Menu key, and closes on Escape, a click outside or
  its `close` func.
- `Tabs.Reorderable` lets the user move tabs by dragging them or with Ctrl+Shift+Left and Right,
  with the new position announced, and `OnReorder` reports the new order. The drop line is the
  `DropLine` part of `TabsStyle`.
- `ui.CommandPalette(open)` is a searchable list of commands, open while its signal is true. It
  lists the actions available where focus was (or what `Commands` returns), without hidden ones,
  and keeps those whose names contain every word typed, with the matches highlighted and each
  command's key binding beside it. Up, Down, PageUp and PageDown move the highlight, Enter or a
  click runs the command where focus was and gives focus back, and Escape closes it. Assistive
  technology sees a dialog with a combo box whose highlighted command is the focus. It is styled
  through `ui.CommandPaletteStyle` and `Theme.CommandPalette`, and its texts are
  `Strings.CommandPalette`, `CommandPalettePlaceholder` and `NoMatchingCommands`.
- `ui.Alert` is a message box in the flow of the page, with a title, a description, an icon per
  variant (`Info`, `Success`, `Warning`, `Danger`), an optional action and an optional close button.
  It is a group named by its title; it is not announced, so an app calls `Announce` when it should be.
- `ui.Calendar(value)` is the month of a date picker's calendar on its own, inline on the page,
  and `ui.CalendarRange(from, to)` the month of a date range picker's, choosing a range in two
  steps. They take the pickers' `Min`, `Max`, `DisableDays` and `FirstDay`, their keys (Enter
  chooses, and Escape and Tab are left to the page) and their accessible grid, and are styled
  through `ui.CalendarStyle` and `Theme.Calendar`, with the part names of the pickers' calendars.
- `ui.Carousel(index, slides...)` shows slides one at a time, with previous and next buttons
  and a dot for each slide. `Loop` wraps around, Left and Right move while focus is on the
  controls, and `AutoPlay(d)` moves on by the window's clock, waiting while the pointer or focus
  is on the carousel, with a play button first in the controls that stops and starts it
  ("Pause slides", "Play slides"); under reduced motion it starts stopped. Each slide comes in
  with a short motion. The carousel is a group described as a carousel, the slide shown a group named
  "Slide 2 of 5", and a change the user makes is announced. It is styled through
  `ui.CarouselStyle` and `Theme.Carousel`.
- `ui.DescriptionList` shows terms with their values in a grid of `Columns(n)`, with the term beside
  the value or above it (`Vertical`), optionally `Bordered`. Each item is a group named by its term.
- `ui.Empty(title)` is an empty state: an icon (an inbox by default), a title, an optional
  `Description` and an optional `Action`, centered and muted, for `DataGrid.Empty` and lists
  with no items. It is a group named by its title. It is styled through `ui.EmptyStyle` and
  `Theme.Empty`. The demo's Projects page uses it for its cards and its table.
- `ui.GroupBox` is a titled section for related content, outlined or filled (`Fill`), with an
  optional description. It is a group named by its title.
- `ui.HoverCard(trigger, content)` shows a floating card of rich content once the pointer
  rests on the trigger or keyboard focus reaches it (`OpenDelay`, 500 ms by default). It stays
  open while the pointer is over the trigger or the card, closes `CloseDelay` (300 ms) after it
  leaves, and closes at once on Escape, a click outside or focus leaving. It never takes focus;
  assistive technology reads it as the trigger's description, as a tooltip. It has the look of
  a popover's panel, through `ui.HoverCardStyle` and `Theme.HoverCard`, and `Placement`.
- `ui.Kbd` shows a keyboard shortcut as key caps, from keys written as for `Bind`, with the key
  names a menu shows. It is one static text for assistive technology, such as "Ctrl+Shift+P".
- `ui.OTPInput(label, code, length)` takes a one-time code in a row of single-character boxes, as
  one control with a caret: typing moves on, Backspace and Delete clear, the arrows move, and a
  paste fills the boxes with the characters it takes. It has `Masked`, `Alphanumeric`, `Groups`,
  `OnComplete` and `Validate`, works in a `Field` and a `Form`, and is one text field for assistive
  technology, reported as a password field when masked. It is styled through `ui.OTPInputStyle`
  and `Theme.OTPInput`.
- `ui.RangeSlider(low, high, min, max, step)` sets a range with two thumbs on one track, with the
  track between them filled. The thumbs cannot cross (`MinGap` keeps them apart), a press on the
  track moves the nearer thumb, and each thumb is a Tab stop with a slider's keys. Assistive
  technology sees a group named by the label with two sliders, named by `Strings.Minimum` and
  `Maximum`. It is styled through `ui.RangeSliderStyle` and `Theme.RangeSlider`.
- `ui.Rating(label, value)` is a row of stars for a whole number from 0 to `Max` (5 by default):
  the pointer previews and a click sets the value, a click on the current star clears it, and
  the arrows, Home, End and the digit keys set it. `ReadOnly` shows it without input. Assistive
  technology reads it as a slider with a value such as "3 of 5 stars" (`Strings.RatingValue`).
- `ui.SplitButton(label, onClick, menu)` is a button with an action and an arrow that opens a menu,
  as two joined segments with a button's variants and sizes and an optional `Icon`. The arrow opens
  the menu on a click, Enter, Space or Down, and Alt+Down opens it from either segment. Assistive
  technology sees a group with two buttons: the action and the arrow, named by
  `Strings.MoreOptions`, with its expanded state. It is styled through `ui.SplitButtonStyle` and
  `Theme.SplitButton`.
- `ui.StatusBar()` is the bar along the bottom of a window, with items at its `Start` and its
  `End`, a top border and small muted text. Assistive technology sees a status bar (the new
  `access.RoleStatusBar`, UI Automation's StatusBar control type). It is styled through
  `ui.StatusBarStyle` and `Theme.StatusBar`. The demo's main window has one.
- `ui.Stepper(current, steps...)` shows the progress through the steps of a process: complete
  steps show a check mark, the current one is highlighted, and the later ones are muted. It lays
  out horizontally or with `Vertical()`, and `OnSelect` makes the steps choosable with a click or
  the keyboard. Each step is named for assistive technology by its position, title and state, such
  as "Step 2 of 4: Shipping, current". It is styled through `ui.StepperStyle` and `Theme.Stepper`.
- `ui.Toggle(label, on)` is a button that stays pressed while it is on, such as a Bold button in a
  toolbar, with the sizes and the Ghost or Secondary look of a button, an optional icon and
  `IconOnly`. Assistive technology reads it as a toggle button.
- `ui.ToggleGroup(value, options...)` is a row of joined toggles of which one is selected. It takes
  the options of a radio group, which now have an `Icon`, and behaves as a radio group for the
  keyboard, for assistive technology and in a `Field`.
- `ui.Toolbar(label)` is a row of controls that is one Tab stop: Left and Right move focus
  between its controls, Home and End go to the first and the last, and Tab comes back to the
  control that had focus last. Controls that use those keys, such as a text field, keep them.
  `ui.ToolbarSeparator()` divides groups of controls. Assistive technology sees a tool bar
  named by its label (the new `access.RoleToolBar`, UI Automation's ToolBar control type). It
  is styled through `ui.ToolbarStyle` and `Theme.Toolbar`.
- `TextInput.Masked()` draws a bullet for each character, as a Windows password box does: copy and
  cut do nothing, a word is the whole text, and assistive technology reads a password field whose
  value is the bullets. `Revealable()` adds an eye button that shows and hides the text (Alt+F8 from
  the keyboard), named by `Strings.ShowPassword` or `HidePassword` for what a press does, that
  leaves the focus in the field. Its look is the new `Button` and `ButtonHover` parts of `ui.FieldStyle`.
- `TextInput.Clearable()` shows an X button while the field has text: a click clears the text,
  runs `OnChange` and leaves the focus in the field, and Escape clears a field that has text
  before it does anything else. The button is named by `Strings.Clear`, and the demo's
  Components filter uses it in place of its own button.
- `icons.Pause` and `icons.Play` are new, for the carousel's play button, and `icons.EyeOff` for
  the reveal button of a masked text input.
- `ui.LinkSegs` makes one link of several styled segments.
- `Context.After` runs a function on the UI thread after a delay on the window's clock, which
  `ui/uitest` can advance.
- The notes example shows a Markdown preview beside the editor (Ctrl+Shift+V), and the demo has a
  Markdown page.

### Changed

- Links in rich text (`ui.Link` and segments with `OnClick`) are Tab stops: the focused link shows a
  focus ring and Enter follows it. Links inside headings and list items are kept in the
  accessibility tree.
- The predefined themes are converted from GPUI Kit v0.8.0 (formerly gpui-component). Panel and
  border colors of Asciinema and macOS Classic, and the syntax colors of macOS Classic Dark,
  follow upstream. The Matrix theme, which GPUI Kit no longer has, stays.
- The visual inspector has a new look: a toolbar with the pick toggle (Ctrl+Shift+C), the inspected
  window's title and a filter for the tree, over a resizable split of the element tree and the
  details. Tree rows show the kind, key, part, text and the component's short name on one line. The
  details show the element's component and ancestors, a box model diagram, the accessibility node
  with its warnings as alerts, the style as a table with the overridden values struck through, and
  frame tiles over a sparkline with the 16.7 ms budget.
- The inspector outlines the element of the tree row under the pointer in the inspected window,
  more lightly than the selection. The selected element shows its margin, border and padding as
  tinted bands and its content box filled, as browsers' developer tools do, under an accent outline
  and a label with its kind, key and size.

### Fixed

- The fonts parsed for a window stayed in memory after the window closed, because a process-wide
  map held every parsed font. A closed window, such as a dock window or the inspector, now gives
  its fonts back, and the `ui` tests peak at about 0.5 GB instead of 8 GB.
- An element selected in the inspector's tree is outlined in the inspected window at once, not only
  after the pointer or the focus reaches that window.
- The inspector marks an element with no width or no height with a 2-pixel line and its label,
  where it drew nothing visible before. Its label stays inside the window.
- The inspector shortens long texts by characters, so it no longer cuts a character in two.
- The inspector's pick button follows pick mode when Escape in the inspected window ends it.

## [0.2.0] - 2026-10-15

The additions from the API review of v0.1.0. No API is removed or renamed; the changes below are
the only ones that can affect existing code.

### Added

- `Window.Visible()` reports whether a window is shown, and `Tray.Shown()` whether a tray icon is
  in the notification area. `ui.Window` gains the method; fakes that embed `ui.NopWindow` get it.
- `PinchEvent` carries the handler element's `Width` and `Height`, as `MouseEvent` does.
- `ColorPicker` takes `Disabled`, `FocusHandle`, `LabelledBy` and `Tooltip`, with a
  `TriggerDisabled` style part.
- `DateRangePicker.Validate`, with rules on the new `ui.DateRange` type; a range picker in a `Field`
  shows and announces its error as `DatePicker` does.
- `Tooltip` on `Pagination`, `Breadcrumbs`, `Sidebar`, `Tag` and `Avatar`.
- `SVG` takes `Opacity`, `Fit` and `Radius`, as `Image` does. Without `Fit`, an SVG still follows its
  own `preserveAspectRatio`.
- Charts and the code editor take `Width`, `MinWidth`, `MinHeight`, `MaxWidth`, `MaxHeight`, `Grow`
  and `GridSpan`.
- `Series.Color` sets a series' color, and `CandleSeries.Rising` and `Falling` the candles' colors.
  High contrast ignores them, and pies and Sankey diagrams keep the palette.
- The words and summaries that charts show and announce are `ui.Strings` fields (`Chart...`), so an
  app can translate them. The built-in packs leave them English.
- `ui.SetThemeStyle` and `ui.ThemeStyle` hold the style of any widget type on a `Theme`, so charts,
  the code editor and widgets outside `ui` can be styled app-wide. A chart's or editor's style is the
  built-in default, then the theme's, then the instance's `Styles`; under high contrast their colors
  are ignored.
- `Document.Version()` in `codeedit` is a tracked read: a component that shows it re-renders when the
  document changes, with no subscription by hand.
- `ui/uitest` drives a UI in headless tests: find elements by key, name or role, click, type and press
  keys, step time, use an in-process clipboard, and read text, accessibility states and the drawn
  style. `Runtime.StyleOf` reports how the latest paint drew an element. The guide is
  [docs/testing.md](docs/testing.md).
- `jamb package` checks that the app's `App.AppID` matches `identityName` in jamb.json, and stops an
  installer or MSIX build when they differ. It runs the built app with an internal flag, and skips
  the check with a warning when the app can't run on this machine or predates the flag.

### Changed

- `charts.DataTable` returns a `*DataTableElement`, which has `Key` and the sizing methods. Code that
  stores the result as a `ui.Element` still compiles.
- Chart summaries use the singular for a count of one ("1 link", "1 slice").
- The code editor's `Size`, `Width` and `Height` win over a width or height in its `Box` style, as
  the sizes of the `ui` widgets and charts do. The `Box` style still wins over the default size.

### Fixed

- The tray icon docs named the wrong UUID version for the icon's GUID.

## [0.1.0] - 2026-10-15

jamb is a small, declarative, GPU-rendered desktop UI framework written in plain Go, licensed under
the Apache License, Version 2.0. Windows is the first platform: windows use Win32, and rendering
uses Direct3D 11. An app is a tree of elements built in render functions, and state lives in
signals. This is the first release, and the API may change before 1.0.

### Added

#### Windows and platform

- Native windows, with several windows in one app, and window moves, resizes and fullscreen set from code.
- Custom title bars that the app draws, with the platform still handling the caption buttons.
- Recovery from GPU device loss.
- Following Windows settings: dark and light themes, high contrast, text scale, reduced motion and the caret.
- Opening http, https and mailto links with the default handler.

#### Rendering and text

- Canvas for drawing shapes, a CPU path rasterizer, and SVG images.
- Text shaping and layout, with emoji drawn in color, rich text, and selectable text.
- Fonts registered by the app with `App.Font`, and WebP and animated GIF images.
- Gradients, borders with rounded clipping, shadows, and transitions.
- Frames are drawn only when something changes, so an idle window draws nothing and runs no timers.

#### Layout and styling

- Row and column layout with grow, shrink, align, justify, gap, margins, percentages, clipping and scrolling.
- Grid layout with `Div.Grid`, sidebars, split panes with sized panes, and docking with tab groups,
  splits and floating panels.
- Typed style objects for every built-in widget, with theme-wide styles for each kind of widget.
- Predefined color themes, converted from the theme sets of gpui-component (see NOTICE).

#### Widgets

- Buttons, text fields, text areas, number inputs, check boxes, switches, radio groups and sliders.
- Forms with fields, validation and submit, which moves focus to the first field that needs attention.
- Combo boxes with suggestions, date pickers, date range pickers, time pickers and color pickers.
- Tabs, collapsible sections, accordions and tree views.
- Menus, context menus and a menu bar.
- Variable-height lists, and virtualized data grids with sorting, selection and editable or interactive cells.
- Popovers, drawers, modal dialogs, progress bars, progress rings, toasts, badges, tooltips, avatars,
  tags, skeletons, breadcrumbs and pagination.
- Touchpad and touchscreen pinch gestures, and drag and drop.

#### Accessibility

- A UI Automation tree built from the element tree, with roles, names, states, actions and live updates.
- The Text pattern for text fields and for elements that draw their own text.
- Platform-neutral `access` package, with helpers for tests (`access/accesstest`).
- Development warnings for accessibility problems, such as an interactive element with no name or an
  image with no alternative text, and checks with Axe.Windows.

#### Input and IME

- Keyboard focus through focus handles on every interactive control.
- Input method (IME) composition, drawn in text fields and in elements that draw their own text.
- Actions with default key bindings, contextual bindings, and keymap files in JSON.
- Clipboard text and images.

#### Theming and translations

- Light and dark themes, and a high contrast theme built from the system colors.
- Built-in text translated into Italian and Chinese (Simplified, Traditional and Hong Kong variants),
  with `ui.Strings` for an app to change any of the text.

#### OS integration

- Jump lists with tasks and recent documents, and a single-instance mode that hands command-line
  arguments to the running instance.
- Notification area (tray) icons with menus, and hiding a window to the tray.
- Taskbar progress and badges.
- Windows notifications with activation, and a Start menu shortcut for a notifying app.
- Native dialogs: open, save, folder picker and message box.

#### Developer tools

- `jamb dev`, which watches the app, rebuilds it on change, restarts it and keeps the window geometry.
- A visual inspector in its own window, which shows the layers that set each style value and can
  edit values live.
- Headless tests by default; `JAMB_LIVE_TESTS=1` adds the tests that show a window.
- Environment variables for tracing, accessibility warnings and the inspector, which need no code changes.

#### Packaging

- `jamb package`, which builds a Windows zip, an NSIS installer (`setup.exe`) and an MSIX package.
- A sign command for the release files, and a `jamb.json` file for the package settings.

#### Charts

- A `charts` package drawn with Canvas: line, area, bar (grouped, stacked and horizontal), scatter,
  pie, donut, radar, candlestick and Sankey charts.
- Keyboard navigation and tooltips for each chart, and `DataTable`, which shows a chart's values as a table.
- Lines and areas are downsampled before they are drawn, so large series stay fast.

#### Code editor and grammars

- A `codeedit` code editor with undo and redo, several selections, find and replace, folding by
  indentation, and tree-sitter highlighting that runs in WebAssembly.
- The grammars are in their own module, `github.com/tyler-builds/jamb-grammars`, released with v0.1.0,
  with packages for Go, JavaScript, TypeScript, Python, Rust, C, C++, C#, Java, HTML, CSS, JSON, YAML,
  TOML, Markdown and Bash.

#### Performance

- Lists and data grids build only the rows that are in view.
- Text is held in a persistent rope with a line index.
- Startup and idle figures are measured in [BENCHMARKS.md](BENCHMARKS.md).

[Unreleased]: https://github.com/tyler-builds/jamb/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/tyler-builds/jamb/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/tyler-builds/jamb/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/tyler-builds/jamb/compare/v0.6.4...v0.7.0
[0.6.4]: https://github.com/tyler-builds/jamb/compare/v0.6.3...v0.6.4
[0.6.3]: https://github.com/tyler-builds/jamb/compare/v0.6.2...v0.6.3
[0.6.2]: https://github.com/tyler-builds/jamb/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/tyler-builds/jamb/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/tyler-builds/jamb/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/tyler-builds/jamb/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/tyler-builds/jamb/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/tyler-builds/jamb/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/tyler-builds/jamb/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/tyler-builds/jamb/releases/tag/v0.1.0
