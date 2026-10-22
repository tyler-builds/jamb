# Status and known limitations

The milestone status, and the known limitations and next steps.

## Milestone status

| Milestone | Status |
|---|---|
| 0: native window, GPU surface, rectangle, text | ✅ |
| 1: Element, Div, Text, style object, tree, row/column layout | ✅ |
| 2: hit testing, hover, buttons, click handlers, signals, redraw scheduling | ✅ (counter and dynamic project cards in the demo) |
| 3: grow/shrink, align, justify, gap, margins, percentages, clipping, vertical and horizontal scrolling | ✅ |
| 4: rounded corners, borders, shadows, icons, images, hover transitions, dark/light themes | ✅ |
| 5: profiling | ✅, see [BENCHMARKS.md](../BENCHMARKS.md) |
| Dev runner (`jamb dev`): watch, rebuild only on change, restart, keep window geometry | ✅ (development state is not preserved yet) |
| Multiple windows and custom title bars (Windows) | ✅ see [Windows](windows.md#windows) and [Custom title bars](windows.md#custom-title-bars) |

## Known limitations and next steps

- **Tree view.** A tree builds only its visible rows, so its accessibility tree is
  flat: the items are siblings under the tree, with `Level` and their position in
  their set, not nested as UI Automation's content view describes. A tree that has
  never been drawn builds only its first row; an assistive technology that expands
  an item out of view should scroll the tree into view first. A tree selects one
  node, and it does not report its open state to the app. Animated expansion and a
  sliding tab indicator are not implemented (both are optional; reduced motion
  needs no change for them).
- **Disabled controls.** In the normal themes a disabled control is dimmed with
  `Opacity` (0.5), as before. WCAG 1.4.3 exempts inactive components from the
  contrast ratio, so the dimmed text is not held to 4.5:1. Under high contrast a
  disabled control is drawn in GrayText (outline and text) and is not dimmed, as
  Microsoft's guidance asks, and that GrayText is kept at 4.5:1 or more on the window.
- **Large text in single-line fields.** A single-line text field scrolls its text
  horizontally when the text is wider than the field, as any input does. At 225%
  the notes title can be longer than its field.
- **Repeating animations at rest.** A repeating `Animate` (the demo's Motion bars)
  keeps the frame loop running at the display's rate, as it must while it moves. Only
  the animated elements repaint; the rest of the page is not rebuilt. Stopping
  repeating animations that are off screen is not done.
- **Focus.** Tab reaches only the rows of a virtualized list (`ListOf`)
  that are built, which are the rows near the viewport; rows beyond them
  are reached by scrolling and then Tab. Tab reveals vertically only,
  not horizontally. A borderless text field has no focus indicator of its
  own, since it has no frame to style. After a click on a `TabStop(false)`
  element, Tab starts from the first element of the order and Shift+Tab
  from the last.
- **Overlays.** They animate in and out only when they set `Enter` and `Exit`
  (tooltips and the command palette do). Escape closes an overlay only when it
  has an `OnClickOutside`; any other overlay handles Escape itself, with
  `OnAction` and a binding or in `OnKeyDown`.
- **Non-modal dialogs.** They are placed by their kind and are not anchored to an
  element or dragged. Their Narrator announcements, and F6 with dialogs open, are
  checked headless only; a hand check on Windows is still owed.
- **Key bindings.** A handler can't pass the key on to the next binding.
  The keymap file's watch is per directory, so other files that change there
  (such as the notes file on each save) cause a cheap re-read of keymap.json,
  which is ignored when its bytes are unchanged. Binding layers are only
  partly like GPUI's (its base, default and user keymaps): the file's layer is
  replaced in place, and otherwise the order in which bindings are added
  decides between them.
- **Toolbars.** A toolbar does not wrap, and does not move controls that do not
  fit into an overflow menu; keep it short enough for its window.
- **Inspector.** An element picked in the window inside a collapsed branch of the
  tree is selected, but its row is not revealed: `ui.Tree` cannot open nodes from
  outside yet. The breadcrumb of the details and the tree filter reach it.
- **Platforms.** Only the Windows adapter exists. macOS (Cocoa + Metal) and
  Linux (Wayland/X11 + Vulkan or GL) adapters plug in behind
  `platform.Window` and `render.Renderer`. Since there is no cgo today,
  macOS will need either a small cgo shim or purego-style Objective-C calls.
- **Windows, multiple windows and custom title bars.** Checked on Windows 11
  with real mouse input: the caption buttons (hover, press, maximize and
  restore), double-clicking the title bar, the hit-test answers for the title
  bar, its buttons, the left border and a corner, and the maximized geometry
  (content inside the work area).
  Checked with scripted input: opening a window from a click, sharing a star
  between two windows, closing a window while the app keeps running, and
  closing the last one. The Snap Layouts flyout was checked by hand on Windows
  11 and works, provided the top DWM frame margin stays 0 (see `applyCustomFrame`
  in `internal/platform/windows/chrome.go`). Not checked automatically, so check
  these by hand: dragging the title bar to move the window and to snap it to an
  edge, Win+ arrows, the drop shadow and the rounded corners.
  Images and glyph textures are shared per device and stay resident after
  the window that used them closes. A lost GPU device is rebuilt
  automatically; see [GPU device loss](architecture.md#gpu-device-loss) for what was checked
  and what was not.
- **Text.** Shaping, bidi display, fallback, selection, clipboard and undo
  work. Still missing:
  - **IME.** Text fields draw the composition inline, through the IMM32
    composition messages; it has not been checked by hand with the Chinese,
    Japanese and Korean input methods yet. Elements that take text with
    `OnTextInput` see only the confirmed text, and assistive technology is not
    told about the composition.
  - **Paragraph direction.** It is always left-to-right, so an Arabic-only
    paragraph is right-to-left inside but left-aligned. Arrow keys move in
    logical order through mixed-direction text.
  - **Emoji.** Drawn in color on Windows, from the color layers (COLR
    version 0) of Segoe UI Emoji, through DirectWrite; they fade with the text
    color's alpha. Skin tones, VS16 and ZWJ sequences (such as woman
    technologist, U+1F469 U+200D U+1F4BB) shape to one emoji. Not drawn in
    color: emoji fonts with only bitmap or SVG glyphs, and COLR version 1
    gradients. Keycaps (a digit, U+FE0F, U+20E3) take the digit from the text
    font, and Windows has no flag emoji, so flags show as letters.
  - **Rendering.** On Windows, glyph masks come from DirectWrite (hinted
    vertically: crossbars and x-height/cap-height edges land on whole pixels
    while stems keep their subpixel position). When the user has ClearType
    on (`SystemParametersInfo` font smoothing of type ClearType), glyphs are
    rasterized as ClearType RGB coverage (BGR panels are honored) into a
    second atlas and drawn with dual-source blending, with an alpha
    correction in the shader driven by the user's DirectWrite gamma,
    contrast and ClearType level. The correction was fitted against
    `IDWriteBitmapRenderTarget` output (`TestClearTypeReference`, about 2.5
    levels mean difference on inked pixels). Grayscale anti-aliasing with
    the contrast-enhancing shader is used when ClearType is off, when the
    device lacks dual-source blending, for icons, and for fonts without a
    file path or on other platforms (the pure-Go rasterizer is unhinted and
    slightly softer). Subpixel text assumes an opaque swap chain and
    pixel-scale drawing, which holds for every window jamb creates.
    `JAMB_TEXT_AA=gray` forces grayscale. Fitted at the default gamma 1.8
    and contrast 0.5; other tuner settings scale the correction rather
    than being fitted.
  - **Spell checking.** Missing. Rich text is display-only; text areas stay single-style.
  - **Shaping cost.** About 0.4 ms per 1,000 characters (measured 2026-10-06). It's cached per
    paragraph, so typing in a long note re-shapes only the edited paragraph
    (about 0.1 ms per keystroke at 11,000 characters, also 2026-10-06).
- **Untested paths.** Non-100% DPI is covered by a headless test
  (`TestScaleProducesPhysicalPixels`) but not on real hardware.
  Multi-monitor `WM_DPICHANGED` has not been exercised either.
- **Paint cost is per visible node.** Paint and hit testing skip off-screen
  subtrees (see the frame pipeline), so a long scroll list costs about its
  visible rows: repainting 10,000 nodes with 73 on screen takes about 0.07 ms
  (72.1 µs in the 2026-10-22 Go suite; 71.5 µs on 2026-10-21).
  A tree whose nodes are all on screen still pays the per-node cost, and a
  container with 10,000 children runs a visibility test on each of them. The
  node itself is about 1 KB; the compact per-node paint struct is the next
  step (see BENCHMARKS.md). `ui.UniformList` and `ui.ListOf` avoid the
  per-child cost for long lists by building only the rows in view. A `ListOf`
  has three caveats: the scrollbar thumb is sized from estimated heights until
  rows have been seen; a slice changed in place is not detected (build a new
  one); and a row whose height changes while it is not built keeps its old
  height until it is built again.
- **Styling.** Border widths are layout, so they don't ease with `Transition`,
  and all sides share one color. Clipping to a rounded shape is exact for one
  rounded container; a nested one keeps only its own corners, and hit testing
  uses the rectangle (see Borders and rounded clipping). Colors, opacity,
  offset and scale ease with `Transition`. Layout
  properties (sizes, padding and so on) animate only through `ui.Animate`,
  which re-lays out each frame. There are no springs yet, no rotation, and
  scale is not hit-tested.
- **Events.** Bubbling only; there's no capture phase.
- **Drag and drop.** Files dragged in from outside the app get no drag
  feedback: `Over`, `Move` and `Leave` do not run, because OLE drag feedback
  for external drags is not implemented. Nothing can be dragged out of the
  app, and OS drops have no drag image. Cursor kinds are unchanged; a drag
  uses the existing not-allowed cursor. File drops arrive through Win32
  `WM_DROPFILES`, which tests can't produce (the drop handle is only valid
  in the process Explorer gives it to); `JAMB_SCRIPT`'s `drop` step covers
  everything after it. Windows refuses drops from Explorer into an app
  running as administrator.
- **Startup.** The ~180 ms startup floor (2026-10-12) is Direct3D driver initialization.
  The framework's own startup cost is about 20 ms. The 2026-10-22 traces put the first
  frame at 204 ms for hello and 221 ms for the demo, warm (196 ms and 215 ms on 2026-10-21).
- **Menus.** Alt+letter mnemonics are not implemented. A menu stays open when a
  key binding moves focus elsewhere. A press outside, Escape or Tab closes it.
  Focus loss does not close menus, because a click on the trigger of a focusable
  dropdown would then reopen it. The submenu delay is a timer, not a triangle.
  The menu bar's labels are not in the Tab order. The roles are in place for
  UI Automation, which is not wired.
- **Dialogs and clipboard.** Native dialogs run on Windows only. Message boxes
  use `MessageBoxW`, not `TaskDialog`. `TaskDialog` needs the common-controls
  version 6 manifest, which `jamb package` adds to a packaged app but a `go run`
  build does not have, so the switch (with a `MessageBoxW` fallback) is future
  work. The clipboard holds plain text and images; HTML is not supported.
  Image copy and paste with other programs has not been checked by hand yet.
  On Windows 11 the file, folder and message box dialogs
  were driven through UI Automation and window messages. They opened owned by
  their window, returned their results, and canceled correctly, and the window
  stayed disabled and kept drawing while a dialog was open. Not checked: dialogs
  in other languages, and high-contrast themes.
- **Packaging.** Windows only (see [Packaging](packaging.md#packaging)). Not done: Store
  submission (the MSIX is uploaded by hand), a machine-wide installer, file
  associations and capabilities beyond `runFullTrust`, localized resources, the
  wide and splash logos and target-size logos of the MSIX, and automatic updates.
  The installer has been run only on Windows 11, and its upgrade over an older
  version has not been tested. The arm64 build is compiled but has not been run.
  The MSIX install steps are for a person to run; the SDK tools check the
  package's structure.
- **Accessibility.** Phases A, B and C, and the automated parts of phase E, of
  [accessibility design](accessibility-design.md) are in place on Windows: the access tree, names and roles, bounds and focus,
  Invoke, Toggle, ExpandCollapse, SelectionItem and Selection, Value, Scroll and
  ScrollItem, Text for text fields and text areas (with the system caret for
  magnifiers), Window for dialogs, focus, menu, property-changed, text-changed
  and notification events, headings and label relations, the dump and the
  development warnings. Open from phase B: a dialog's window opened event is not
  seen by a .NET client (a dialog is announced by its focus, IsDialog and
  IsModal; see [accessibility design](accessibility-design.md)), and virtualized
  rows that are not built cannot be found or realized (see the design's phase B
  open items). Open from phase C: IME composition, the Text pattern for static
  and rich text, annotations and embedded objects (see the design's phase C open
  items). The caption
  buttons (Minimize, Maximize, Close) have their roles and names, but their
  invoke path has not been checked live. Building the tree for the demo's
  dashboard takes about 18 µs, measured with a benchmark before 2026-10-21 (not re-run
  that day; the cost grows with the tree). A UI Automation client that walks the whole tree over and over makes
  the process's private memory rise about 0.05 MB per walk of the demo's 46 nodes;
  the same walk against Windows Notepad grows at the same rate per element, so it
  is UI Automation's own per-client state, not a jamb leak (see
  [accessibility design](accessibility-design.md)). A client that walks past the window's close gets `E_UNEXPECTED`
  from UI Automation. The Axe.Windows rules (AxeWindowsCLI, the engine of
  Accessibility Insights) report no errors in the examples' 18 scanned states, and
  the live checks in `scripts/a11y-check.ps1` pass. Not yet checked with NVDA or
  Narrator: that pass is for a person, with
  [docs/accessibility-testing.md](accessibility-testing.md). Two placeholder
  actions need a decision: the demo's Notifications bell and its sample buttons
  announce a message when pressed. The dev script step `idle` did not return after
  the Components page was scrolled three times (`wait` works); it was not
  investigated.
- **Not attempted (non-goals).** Tray icons, and the optional utility-class
  shorthand (`.Class("flex col gap-2")`). The shorthand was skipped in favor of
  typed styles. Grid layout, once a non-goal, is in (see Grid layout); it is a
  subset of CSS grid, without named areas, explicit placement or subgrids.
