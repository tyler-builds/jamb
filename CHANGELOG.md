# Changelog

All notable changes to jamb are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
version numbers follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `ListOf` and `UniformList` take `Label`, which names the list for assistive technology.
- `Chart.TabStop(false)` leaves a chart and its legend out of the Tab order, for a sparkline
  that decorates a number. A click still focuses it, and assistive technology still reads it.

### Fixed

- A keyed component or composite element, such as a chart, kept answering to its key in tests
  and in the inspector only until a component inside it rendered again. A chart's key was lost
  on the first frame of its enter animation.

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

[Unreleased]: https://github.com/tyler-builds/jamb/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/tyler-builds/jamb/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/tyler-builds/jamb/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/tyler-builds/jamb/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/tyler-builds/jamb/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/tyler-builds/jamb/releases/tag/v0.1.0
