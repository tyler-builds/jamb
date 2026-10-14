# Accessibility design

Status: accepted. Phases A (Foundation), B (Controls) and C (Text) are done, with
the changes listed under "Phase A: what changed from this design", "Phase B: what
changed from this design" and "Phase C: what changed from this design". Phase D
(system settings) is done: high contrast, text size, reduced motion,
the text caret, the contrast audit, and the fix for the `idle` script step (see
"Phase D: what changed from this design").
Phase F (form controls: check boxes with a mixed state, switches, radio groups,
selects, sliders and number inputs, and the RangeValue pattern) is done, with the
changes listed under "Phase F: what changed from this design".
Phase E's automated checks are done (see "Phase E: what changed from this
design"); its manual screen reader pass is pending, and is described in
[accessibility-testing.md](accessibility-testing.md). This document
describes how jamb exposes its UI
to assistive technology (screen readers, magnifiers, voice control) and the
order in which the work is done.

## Goals

- Apps built with jamb are usable with a screen reader by default: built-in
  widgets report the right role, name and state with no app code.
- Custom elements can describe themselves with a small API.
- No cost when no assistive technology is running.
- One platform-neutral model, with one adapter per operating system, so macOS
  and Linux support later reuse everything above the adapter.
- Developers are warned about the common mistakes (an icon button without a
  label, an image without a description) while they develop.

Accessibility standards (for example Section 508, which references WCAG 2.0 AA,
or EN 301 549) apply to finished apps, not to a toolkit. jamb's job is to make
meeting them achievable and the default; whether an app conforms depends on the
app.

## Prior art

jamb draws its own UI, like egui, Bevy and GPUI. Toolkits of this kind generally
follow [AccessKit](https://github.com/AccessKit/accesskit): the toolkit
describes its UI as a tree of nodes, and a platform adapter translates the tree
to the operating system's accessibility API. GPUI depends on AccessKit; its
Windows support is not complete yet
([zed-industries/zed#41138](https://github.com/zed-industries/zed/issues/41138)).

AccessKit is written in Rust; using it from Go would require cgo, and jamb is
pure Go. jamb therefore follows AccessKit's architecture in Go instead of
binding to it.

## Architecture

```
ui runtime ──builds──▶ access tree (package access) ──snapshot──▶ platform adapter ──▶ OS API
   ▲                                                                  │
   └──────────────────── actions (click, focus, expand…) ◀────────────┘
```

### The access package

A platform-neutral package with no dependencies. It defines:

- **Node**: a stable ID, a role, a name, an optional description and value,
  states (focused, disabled, checked, mixed, expanded, selected, read-only,
  required, invalid), bounds in window coordinates, the actions it supports,
  its children, relations (labelled by, described by, controls), heading level,
  and list position (position in set and set size, so a row of a virtualized
  list can report "40 of 10,000" without the other rows existing).
- **Roles**: window, group, button, check box, switch, radio button, text
  field, multi-line text field, static text, image, link, list, list item,
  menu bar, menu, menu item (plain, check, radio), separator, dialog,
  tooltip, scroll area, heading, tab list, tab, tab panel, combo box, slider,
  spin box (the last three from phase F).
- **Actions**: click (invoke), focus, expand, collapse, toggle, select, set
  value, scroll into view, increment, decrement, set range value (phase F).
- **Tree updates**: the nodes that changed and the focused node. Updates are
  incremental.
- **Announcements**: text to speak, polite or assertive, for status messages
  that have no focused element ("Saved", "3 results").

### Building the tree

The ui runtime builds the tree from its own nodes:

- Built-in widgets map to roles automatically: buttons, text fields and text
  areas, menus and menu bars, modals (dialog, named by the title), lists,
  tooltips, links in rich text, images and icons.
- Plain text becomes the name of the element that contains it, or static text
  when it stands alone. Purely visual elements (decorative icons, dividers,
  layout containers without semantics) are left out of the tree.
- Virtualized lists (`UniformList`, `ListOf`) expose the rows that are built,
  each with its position in the full list and the list's size.
- Overlays are attached where they belong in reading order, not at the end of
  the window.

The tree is built only after an assistive technology first asks for it, and is
then updated from the runtime's existing change tracking, so frames cost nothing
extra when no assistive technology is running.

### Public API

Small and explicit, on the elements that already exist, for example:

- `Div().Role(access.RoleCheckBox).Label("Notifications").AccessChecked(on)`
- `Div().Description("…")`, `Div().AccessHidden()` (leave out of the tree),
  `Div().Heading(2)`
- `Div().Display(false)` (not shown, not drawn, not focusable and out of the tree,
  with its state kept). It is not `AccessHidden()`, which only leaves the element out of
  the tree and keeps it drawn and reachable by mouse and keyboard.
- `Image(src).Alt("…")`, `IconButton(icon).Label("…")`
- `ui.Announce(text, access.Polite)`

The exact names are settled during implementation and documented in the
README.

### Windows adapter (UI Automation)

- The window answers `WM_GETOBJECT` with a fragment root provider; nodes are
  fragment providers (navigation, bounds, focus, runtime IDs).
- Control patterns by role and state: Invoke, Toggle, ExpandCollapse,
  Selection and SelectionItem, Value, RangeValue, Scroll and ScrollItem, and
  Text (phase C).
- Events: focus changed, property changed (name, value, toggle and expand
  state), structure changed, menu opened and closed, and notification events
  for announcements.
- Implemented in pure Go with the COM calling technique jamb already uses for
  Direct3D and the file dialogs.
- Threading: UI Automation may call from other threads. The adapter answers
  from an immutable snapshot of the tree, and posts actions back to the UI
  thread, as AccessKit does.

## Developer tooling

- A dump of the access tree (a `JAMB_SCRIPT` step and a runtime method), for
  tests and debugging.
- Development warnings for unlabeled interactive elements and images without
  alt text.
- Test helpers to assert roles, names and states in unit tests.

## Phases

| Phase | Delivers |
|---|---|
| A. Foundation | access package, tree building, Windows fragment providers (navigation, names, roles, bounds, focus), Invoke for buttons, focus events, tree dump, development warnings |
| B. Controls | Toggle, ExpandCollapse, Selection, Value for single-line fields, Scroll, virtualized list positions, dialog and menu events, announcements, headings |
| C. Text | UI Automation Text pattern for text fields and text areas: caret, selection, reading by character, word and line, text change events |
| D. System settings | Windows high contrast, the Text size setting, reduced animations, a contrast audit of the built-in themes, caret tracking for magnifiers, and the text caret blink and width |
| E. Verification | Accessibility Insights for Windows checks on the examples, a manual NVDA and Narrator script, automated tests that read the real UI Automation tree |
| F. Form controls | Check boxes with a mixed state, switches, radio groups, selects, sliders and number inputs; the RangeValue pattern; the Windows ComboBox, Slider and Spinner control types |
| G. Navigation and layout | Tabs with automatic activation, Ctrl+Tab, and lazily built panels; collapsible sections and an accordion; a tree view that builds only its visible rows, with Tree and TreeItem control types, levels and positions in their sets |
| M. Date and time | `ui.DatePicker` and `ui.TimePicker`, the calendar with the Calendar control type and the Grid and Table patterns, and the keys of the WAI-ARIA date picker dialog pattern |
| Split panes | `ui.Split`: a focusable divider with the Separator role and the RangeValue pattern, keys of the window splitter pattern, panes as named groups |
| K. Forms | `ui.Field` (label, help text, required marker, error), validators on the controls, `ui.NewForm` with the submit flow, the error summary, and an async submit; IsRequiredForForm and IsDataValidForForm |

## Testing

- Unit tests on the access tree built from ui elements.
- Automated checks that read the live UI Automation tree of the examples the
  way a screen reader does.
- [Accessibility Insights for Windows](https://accessibilityinsights.io/)
  (free, from Microsoft) for automated rule checks.
- Manual passes with NVDA (free) and Narrator (built into Windows).

## Phase A: what changed from this design

Phase A is in place on Windows (`access`, `ui/access.go`,
`internal/platform/windows/uia*.go`). These are the places where the work differed from
the design above, and why.

- **The tree is rebuilt when something changed, not per subtree.** The runtime
  marks the tree dirty when a component re-renders, when layout or paint runs
  (paint moves nodes when they scroll), and when focus moves. The next frame
  then builds the whole tree and publishes it with a `Diff` against the last
  one, so only changed nodes are reported. Building per subtree would need the
  runtime to track which nodes changed, which it does not do yet. The cost is
  measured: about 18 µs and 281 allocations for the demo's dashboard (about 47
  nodes). A frame of the paint stress test takes about 300 µs, and with a client
  attached each painted frame builds the tree once. That is cheap at this size,
  but it grows with the tree, so per-subtree updates are the next step if large
  trees need it.
- **One provider object per node.** The adapter answers from the immutable
  snapshot, as the design says. It also keeps one COM object per node in a
  registry, so that UI Automation sees the same element each time, and releases
  the registry's reference when the node leaves the tree. The design did not
  specify this.
- **Per-window and per-snapshot caches.** The host provider of a window is made
  once and held by the registry, and the root node is rebuilt when the snapshot
  changes, not on each call. Navigation and bounds no longer allocate on the hot
  paths; `uia_alloc_test.go` holds them to zero.
- **The window is the host of its fragment root.** The root's
  `get_HostRawElementProvider` returns `UiaHostProviderFromHwnd`. Without it UI
  Automation placed the fragment root one level below a native element for the
  window, so every node sat one level too deep in a client's tree. Found by
  walking the tree from PowerShell.
- **`IRawElementProviderFragmentRoot` derives from `IUnknown`**, as the SDK
  headers declare, not from `IRawElementProviderFragment`. Its vtable is
  `QueryInterface`, `AddRef`, `Release`, `ElementProviderFromPoint`, `GetFocus`,
  and the fragment methods are on the fragment view.
- **`ElementProviderFromPoint` takes its coordinates in XMM registers.** Its two
  doubles do not reach a `syscall.NewCallback` function, so a small amd64
  assembly stub moves them into the integer registers first (no cgo). The live
  check covered it: a client's `ElementFromPoint` over a button returned that
  button.
- **Public API.** Added `Checked` on `DivElement`, which the design's example
  uses and a switch needs. `Heading` is defined in `access` but has no public
  method yet, as the design allows. Icon elements have no `Label` or `Alt`:
  they are decorative, and an icon's meaning comes from its button's name.
- **Clickable elements with no role are buttons.** A `DivElement` with `OnClick`
  and no role is exposed as a button, because a clickable element that a screen
  reader calls a group cannot be operated.
- **The placeholder is not a name.** A text field's name comes from `Label`
  alone, so a field with only a placeholder is reported by the warnings.
- **Caption buttons have a click handler** that calls the window's minimize,
  maximize and close, so that UI Automation can invoke them. The native hit test
  still handles the mouse.

Open after Phase A:

- **Memory growth with a client walking the tree.** Private memory of the demo
  rises about 0.05 MB per full walk of its 46 nodes (109 to 138 MB over 600
  walks in 27 seconds, with and without a client-side collection). With no client
  the idle demo stays at 106 MB for two minutes. The Go heap is not the cause
  (its live size is flat after each collection) and the adapter's own objects are
  bounded (47 live objects for the demo). Per-call garbage on the hot paths was
  removed and is guarded by a test (`uia_alloc_test.go`). The rest tracks the
  number of new element objects that a client creates: a loop that re-reads one
  element's property stays flat, while a loop that makes a new element each time
  grows about 1.2 KB per element. That points at per-client-element state inside
  UI Automation or OLE, which the provider cannot release. The same walk against
  Windows Notepad grows at the same rate (101.8 to 135.3 MB over 600 walks of its
  49 nodes, about 1.1 KB per element read), so this is how UI Automation behaves
  under repeated full-tree walks, not a leak in jamb. Screen readers read what
  changes rather than re-walking the whole tree, so the growth seen in practice
  is far smaller.
- **`E_UNEXPECTED` in a client walk.** Reproduced once, when a walk continued
  after the demo's window had closed: `FindAll` failed with `E_UNEXPECTED`. Three
  runs in which the walk stayed inside the window's life had no errors. The
  likely source is the disconnect at `WM_DESTROY` (`UiaDisconnectProvider`), so
  a client that outlives its window gets an error from UI Automation. A walk that
  races a page change can also get `UIA_E_ELEMENTNOTAVAILABLE` for a node that
  just left the tree, which is the expected answer for a removed node.

## Phase B: what changed from this design

Phase B is in place on Windows: the control patterns, the events, the relations,
headings, announcements, and the warnings for them. These are the decisions made
in building it, and why.

- **Patterns follow roles and states.** Toggle for check boxes, switches and
  check menu items, and for buttons made toggle buttons with `Checked`.
  ExpandCollapse for elements with `Expanded`: disclosure rows, dropdown
  triggers, submenu items and the labels of a menu bar. SelectionItem for
  elements with `Selected`, which includes radio buttons and radio menu items.
  Selection on the groups that hold them (`SelectionContainer`, tab lists, and
  menus that hold radio items). Value on text fields and text areas. Scroll on
  scroll containers, and ScrollItem on the rows and controls inside one. Invoke
  is unchanged. A pattern is present on a disabled element, which reports it
  disabled, as UI Automation expects. (Phase E changed Invoke for toggles and for
  disabled buttons: see "Phase E: what changed from this design".)
- **Expand, select and toggle run on the UI thread.** An expand, collapse or
  select that asks for the state the element already has does nothing. Otherwise
  the element's click handler runs, so an expandable element's handler must
  change its own state, as a click does. A submenu item is the exception: its
  expansion is not a click, so it opens and closes its submenu directly. Toggle
  always runs the click.
- **Selection is declared by the app.** `AccessSelected(on)` gives an item its state,
  and `SelectionContainer()` marks the group. An item reports the nearest
  container above it. A tab list, and a menu with radio items, are containers
  without a call. A container reports one selection: CanSelectMultiple and
  IsSelectionRequired are false. RemoveFromSelection is refused with
  `UIA_E_INVALIDOPERATION`, since a choice is replaced, not cleared.
- **Value.** A text field's and a text area's text is its Value. SetValue sets the
  bound signal through the editor's own edit path, so `OnChange` runs as it does
  for typing, and the undo history is kept. IsReadOnly is false, because no field
  is read-only yet.
- **Scroll.** A scroll container reports the position and the visible part of
  each axis, in percent. SetScrollPercent moves to a percent of the scrollable
  range, and a negative value leaves an axis alone. Scroll with a ScrollAmount
  moves a tenth of the view for a small amount and nine tenths for a large one.
  That is an approximation, since UI Automation's amounts are not defined in
  pixels. ScrollIntoView reveals the least amount, as `ListHandle.ScrollToItem`
  does.
- **Virtualized lists.** Rows report their position in the full list, as in
  Phase A, and ScrollItem works for the rows that are built. ItemContainer and
  VirtualizedItem are not implemented: a client cannot name a row that is not
  built, so it cannot ask for that row to be realized. A row that declares its
  own role (a list item with a selection) is the item itself, so it is not nested
  in a second list item.
- **Events.** They are derived from the snapshot's `Diff`, once per published
  frame, and raised only when a client listens. Property-changed events cover
  Name, Value, ToggleState, ExpandCollapseState, IsSelected and IsEnabled, with
  the new value; the old value is empty, since the adapter keeps no history.
  Menu opened and closed are raised for each menu that appears or goes away.
  Window opened and closed are raised for dialogs, but see the open items.
- **Announcements.** Each `ui.Announce` is a Notification event on the window.
  Polite maps to `NotificationProcessing_All`: presented when possible, and
  every message is delivered, so a burst of status messages is heard in order
  (the examples debounce their own repeats). Assertive maps to
  `ImportantMostRecent`: presented as soon as possible, and a newer message
  supersedes an older one. `ImportantAll` is avoided because
  [Microsoft's documentation](https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-notificationprocessing)
  warns it can flood the user, and `ImportantCurrentThenMostRecent` exists only
  from Windows build 26100.
- **A Window pattern for dialogs.** It is not in the design's list. A dialog has
  the Window pattern, so that it reports IsModal, IsTopmost and
  WindowInteractionState. Close and SetVisualState are not supported: the
  application closes its dialogs.
- **Relations and headings.** `Heading(level)` maps to HeadingLevel1 to 9.
  `LabelledBy(keys...)` and `DescribedBy(keys...)` name the elements by their
  `Key`. A node's LabeledBy is its first label, and DescribedBy is an array of
  providers. TextInput and TextArea take the same two methods. A key that matches
  no exposed element is a development warning.
- **Menu items and dropdowns.** A menu item's name is its label, and the shortcut
  is shown but not read as part of the name. A DropdownMenu's trigger button is
  inert in the tree: the menu's wrapper is the control, takes the button's name,
  and has the expanded state. The wrapper is focusable, and a focus request on it
  moves keyboard focus to the button.
- **Developer warnings** added: a check box, switch or radio button with no
  `Checked` call, a selection item outside a selection container, and a
  LabelledBy or DescribedBy key that matches nothing. Each element is reported
  once.
- **Adapter notes.** A COM out parameter must not be on the goroutine stack. A
  callback can grow the stack, which moves the variable, and the result is
  written to the old copy: a test's Name read returned an empty VARIANT for this
  reason. `newVariant` and `newWord` return heap pointers for that purpose. For an
  interface array, `SafeArrayPutElement` takes the interface pointer itself, not
  the address of a variable that holds it. SetScrollPercent takes two doubles, so
  it goes through the amd64 trampoline, as ElementProviderFromPoint does; the other
  architectures report E_NOTIMPL.
- **The examples.** The demo: its switches are labelled by their row titles; the
  page titles and section titles are headings; the filter chips, the navigation
  and the theme options are selection items; the project list is a selection
  container, and each card is a focusable list item; the star is a toggle button;
  the Activity build rows expand; the result count of a filter is announced once
  the typing pauses for 600 ms; a created project and a copied text are announced.
  The notes app: note rows are selection items in the sidebar's selection
  container; pin and theme are toggles; tag chips are a selection group; a
  finished save is announced once per save; an import, an export or a deletion is
  announced as its status message is shown.

### Phase B: verification

The unit tests cover each pattern's mapping, the events derived from snapshots,
the list positions, the relations, the announcements, the warnings, and the
adapter's VARIANT and SAFEARRAY types. The examples' tests check the roles and
states on each page and that the actions change the app, with zero warnings.

Live checks were made against the built demo and notes apps, with the .NET
UI Automation client, using a temporary notes file. For each pattern the client
read the state, performed the action, and read the result:

- Toggle: the Settings switch flips, the theme follows, and the property-changed
  event carries the new state. A project's star toggles, and the Dark theme check
  item in a dropdown toggles the theme and closes the menu.
- ExpandCollapse: an Activity build row and the dropdown trigger expand and
  collapse, with a property-changed event; the dropdown raises a menu-opened
  event, and closing it raises a menu-closed event.
- Selection: the Done filter chip is selected and the list shows its one project;
  a project card is selected and reports the list as its container, and
  GetSelection returns it. A note row is selected exclusively, a tag chip is
  selected in its group, and RemoveFromSelection is refused.
- Value: SetValue on the search field filters the projects; the Title field of a
  note takes a value.
- Scroll and ScrollItem: SetScrollPercent(50) lands at 50%, and the rows change;
  ScrollIntoView brings a built row into the list's bounds.
- Events: the notification events for a created project, a note's save, and a
  deleted note; the Name field's LabeledBy is its label.
- Focus, ElementFromPoint and the tree walk were re-checked from Phase A. The
  project card takes keyboard focus, and the dropdown trigger takes focus through
  its wrapper.

Memory was measured with a walker that reads each element's name and control
type, in a loop of 400 walks of the demo's tree (about 50 nodes):

| Walk | Per element read | Growth over 400 walks |
|---|---|---|
| Demo, names only | 1.27 to 1.34 KB | 25 to 27 MB |
| Demo, GetCurrentPattern on each supported pattern | 1.48 to 1.57 KB | 29 to 31 MB |
| Notepad, names only (Phase A's baseline) | 1.15 KB | 22 MB |
| Notepad, GetCurrentPattern on each supported pattern | 2.1 KB | 41 MB |

A pattern query adds about 0.1 to 0.2 KB in the demo, and about 0.5 KB in
Notepad, so the adapter's share is below the native provider's. The growth is
not released by a collection in the client either, so it is UI Automation's own
per-client state, as Phase A concluded. The zero-allocation guard
(`uia_alloc_test.go`) covers the new hot paths: the pattern lookup, the state
reads, and the scroll and selection reads.

### Native modal loops and posted work

A UI Automation request reaches the UI thread through `ui.Runtime.Post`, which
requests a frame and wakes the window. The runtime and the wake path were not
the cause: a headless test that opens the context menu with Shift+F10 and posts
an Expand from another goroutine runs the request on the next frame
(`TestPostRunsWithKeyboardContextMenuOpen`).

The cause was the window's message loop. Shift+F10 arrives as WM_SYSKEYDOWN,
since F10 is a system key. `wndProc` handled the key and then fell through to
`DefWindowProc` (`internal/platform/windows/window.go`, the WM_SYSKEYDOWN case), which
opened the window's system menu. That menu runs its own message loop, and the
loop of `Run` does not run while it does. The wake-up message was taken by the
menu's loop, so no frame rendered until the menu closed. A lone Alt (the window
has no menu bar in the demo) and a title-bar drag (a move loop) stalled the same
way. Dialogs already avoided this with a timer that renders frames while their
loop runs (`modal` in `internal/platform/windows/dialog.go`).

The fix moves that timer into `startFrames` and `stopFrames`, and starts it on
WM_ENTERMENULOOP and on WM_ENTERSIZEMOVE, as well as for dialogs. `stopFrames`
runs the posted queue when the loop ends. The regression test,
`TestPostRunsDuringNativeMenuLoop` (`internal/platform/windows/modal_test.go`), opens the
system menu on the UI thread and posts a function from another goroutine. It
fails without the fix ("the posted function did not run while the menu loop was
open") and passes with it.

Live checks against the demo, with a UI Automation client, before and after the
fix:

| State | Before | After |
| --- | --- | --- |
| Shift+F10 on a card, then Expand Status | not expanded | Expanded, Active, Paused and Done shown |
| Right-click on a card, then Expand Status | expanded | expanded |
| Lone Alt, then Invoke the "Done" filter | list stayed at 6 items | list filtered to 1 |
| Drag the title bar, Invoke "Done" while held | list stayed at 6 items | list filtered to 1, window moved |
| No Alt, Invoke "Done" (control, unfixed build only) | list filtered to 1 | not measured |

The idle, animating and minimized states are covered by the same frame path. A
minimized window renders nothing, and a function posted while it is minimized
runs when it is restored, since the restore sends WM_SIZE, which renders a
frame. That state was not driven live.

### Window events for dialogs

Phase B raises WindowOpened on a dialog's element and WindowClosed on the
window's root. Findings:

- Microsoft describes `WindowOpenedEvent` as the event "raised when a window is
  opened", and says it is "used by UI Automation providers" while clients use
  the WindowPattern field
  ([WindowPatternIdentifiers.WindowOpenedEvent](https://learn.microsoft.com/en-us/dotnet/api/system.windows.automation.windowpatternidentifiers.windowopenedevent)).
  The page does not say that an event raised on a non-HWND fragment reaches
  clients. The Win32 event-id reference could not be fetched (HTTP 404), so the
  rules for fragment windows are not confirmed from primary sources.
- A live check with a UI Automation client (a listener in C#, with
  Automation.AddAutomationEventHandler, pumping messages) subscribed to
  WindowOpened and WindowClosed at the window's root and at the desktop, and to
  MenuOpened at the window's root. It opened the "New project" modal by Invoke
  and closed it with Escape. WindowOpened arrived at neither scope. WindowClosed
  arrived twice, at the desktop only, and its source element could no longer be
  read. The window-scoped listener received nothing, and no menu was opened in
  that run.
- Dialog announcement is described in Microsoft's IsDialog documentation: a
  screen reader speaks the dialog's title, then the focused control, which is why
  focus is what matters
  ([AutomationProperties.IsDialog](https://learn.microsoft.com/en-us/uwp/api/windows.ui.xaml.automation.automationproperties.isdialog?view=winrt-26100)).
  Narrator relies on FocusChanged events, and does not announce changes that do
  not have focus
  ([Microsoft UIA notification guidance](https://learn.microsoft.com/tr-tr/archive/blogs/winuiautomation/can-your-desktop-app-leverage-the-new-uia-notification-event-in-order-to-have-narrator-say-exactly-what-your-customers-need)).
  These two statements came from search summaries, not from the pages
  themselves, so they need checking.

The supported approach is therefore focus. A dialog must take keyboard focus,
and its root must have ControlType Window, IsDialog and IsModal, as the
`ui.Modal` root reports. The adapter's WindowOpened and WindowClosed raises are
kept, since WindowClosed reaches a desktop listener. Whether a fragment
WindowOpened can be delivered at all, and whether a focused modal is announced by
Narrator and NVDA, are left for Phase E, with Narrator testing.

### Phase B: open items

- **Window opened is not delivered to a .NET client for a dialog.** The adapter
  raises WindowOpened on the dialog's element and WindowClosed on the window's
  root. In the live check (see [Window events for dialogs](#window-events-for-dialogs))
  a client listening for WindowOpened at the window and at the desktop received
  nothing, and WindowClosed reached only the desktop listener. Menu events on the
  same root were received. The cause is not isolated, so the event code is
  unchanged. Until it is, a dialog is announced by its focus, its IsDialog and its
  IsModal properties: focus moves into the dialog, and the dialog's Window pattern
  reports IsModal.
- ItemContainer and VirtualizedItem, as above. A client cannot find an unbuilt
  row.
- A context menu's own items are not exposed as a dropdown: a context menu opens
  from the keyboard or a pointer press on the element it hangs from, and that
  element has no expand state.
- The mixed (indeterminate) state is in the model and the adapter, but no public
  API sets it. Closed in phase F: `CheckboxTri` sets it (see "Phase F").
- RangeValue is not implemented: no slider exists in the framework yet. Closed in
  phase F: `Slider` and `NumberInput` expose it (see "Phase F").
- The Text pattern for text fields and text areas was Phase C, and is done (see
  below).
- Screen reader checks: the Accessibility Insights rules are done in Phase E; the
  NVDA and Narrator passes are pending (see
  [accessibility-testing.md](accessibility-testing.md)).

## Phase C: what changed from this design

Phase C is in place on Windows for text fields and text areas: the Text pattern
(ITextProvider, ITextProvider2 for the caret, and ITextRangeProvider), the text
events, and the system caret that magnifiers follow. The model is in
`access/text.go` and `access/textrange.go`, the content is built in
`ui/textaccess.go`, and the adapter is `internal/platform/windows/uiatext.go`.

### Design

- **The snapshot carries the text.** Each text node has an `access.TextInfo` in
  the snapshot: the content, the selection, where the layout is drawn in physical
  pixels, the clip, and the font attributes. The content (`access.TextContent`)
  holds the text, the grapheme points, the x of the caret at each point, the
  visual lines (with their top, height and right edge), the paragraph starts and
  the word runs. The ui runtime builds the content on the UI thread, once for each
  change of the text or the layout, and keeps it on the editor's state; a
  selection change or a scroll does not rebuild it. Nothing a UI Automation call
  reads is shared with the runtime's mutable state.
- **Offset model: points.** A point is a position between two grapheme clusters,
  held as an index into `Points` (the byte offset of each cluster boundary).
  Endpoints of a range are points, so a range never splits a grapheme cluster,
  a surrogate pair or a combining sequence. UI Automation exposes no offsets, so
  the point is an internal unit. UTF-16 is counted only where UI Automation
  counts it: GetText's `maxLength` cuts at a UTF-16 code-unit count and never
  inside a surrogate pair (`access.UTF16Prefix`). A byte offset from the editor
  rounds down to the point of its grapheme cluster.
- **Range lifetime.** A range is a COM object in the registry, made per call with
  native memory from the process heap, reference counted, and freed when its last
  reference is released (`kindTextRange`). A range keeps the `TextInfo` of the
  snapshot it was made from, so its text and rectangles are the ones the client
  was told about, even after the field has changed. Select acts on the current
  text: it sends the two endpoints as byte offsets, and the runtime clamps them to
  its text on a rune boundary. A range whose node has left the tree still answers
  from its snapshot; Select and ScrollIntoView then report ElementNotAvailable.
- **Events.** `access.Diff` reports the fields whose text changed
  (`Update.TextChanged`) and whose selection or caret moved
  (`Update.TextSelection`), once per published frame, so a burst of typing in one
  frame is one event. The adapter raises TextChanged (`UIA_Text_TextChangedEventId`,
  20015) and TextSelectionChanged (`UIA_Text_TextSelectionChangedEventId`, 20014)
  only when a client listens. The Value property change is kept for text fields
  and text areas.
- **Caret for magnifiers.** The window's `SetTextInputRect` already receives the
  insertion point once per frame, for the IME. `placeCaret` keeps a system caret
  there: `CreateCaret` makes a one-pixel solid caret at the field's height,
  `HideCaret` hides it at once so it never draws over jamb's own caret, and
  `SetCaretPos` moves it. The caret is destroyed when no field has focus, and made
  again at the new height when the height changes. Live, `GetGUIThreadInfo`
  reports the caret rectangle in client coordinates, and a screenshot shows only
  jamb's caret.

### Semantics, and the documentation they follow

Microsoft's rules are cited where they apply. Where the documentation is silent,
the choice is stated.

- **Move**
  (<https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextrangeprovider-move>).
  A degenerate range moves its insertion point and stays degenerate. Any other
  range moves its start back to the start of its unit, moves that by the count,
  and ends one unit later, so the result is whole units. The count returned is the
  number of units moved, negative for a backward move, and a move that cannot
  happen leaves the range unchanged. The documentation says a backward count is
  negative and is silent on the sign of MoveEndpointByUnit's return; the same sign
  convention is used there.
- **MoveEndpointByUnit** and **MoveEndpointByRange**
  (<https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextrangeprovider-moveendpointbyunit>,
  <https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextrangeprovider-moveendpointbyrange>).
  The endpoint moves to the next unit boundary in its direction. If it passes the
  other endpoint, the other moves with it, so the range stays ordered and becomes
  degenerate.
- **ExpandToEnclosingUnit**
  (<https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextrangeprovider-expandtoenclosingunit>).
  The start moves back to the start of its unit and the end forward to the end of
  the unit of its last grapheme; a range that is already whole units is unchanged.
  A page is the whole text, as a document is. A degenerate range expands to the
  unit at its position.
- **CompareEndpoints** returns a negative value, zero or a positive value
  (<https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextrangeprovider-compareendpoints>).
  The magnitude is not defined, and the adapter returns -1, 0 or 1.
- **Compare** compares the endpoints, not the text, and a range of another
  snapshot is not the same
  (<https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextrangeprovider-compare>).
- **FindText** returns the first match, or the last with `backward`, within the
  range, with optional case folding (simple Unicode folding). A match starts and
  ends on a cluster boundary. No match, and an empty search, return no range
  (<https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextrangeprovider-findtext>).
- **GetBoundingRectangles** returns one rectangle for each visual line the range
  touches, in screen coordinates, clipped to the field. A degenerate range, and a
  range that is off the visible part of the field, has none
  (<https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextrangeprovider-getboundingrectangles>).
- **GetText** returns the text between the endpoints, whatever their order, cut
  to `maxLength` UTF-16 code units when it is not -1
  (<https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextrangeprovider-gettext>).
- **Select** selects the range and replaces any selection; a degenerate range moves
  the insertion point
  (<https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextrangeprovider-select>).
  The field takes focus, since its selection is shown only when it has it.
- **AddToSelection** and **RemoveFromSelection** return UIA_E_NOTSUPPORTED: the
  field has one selection (SupportedTextSelection_Single).
- **GetCaretRange** (ITextProvider2,
  <https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/nf-uiautomationcore-itextprovider2-getcaretrange>)
  returns a degenerate range at the caret, and isActive is true when the field has
  keyboard focus.
- **GetSelection** returns the selection as one range, a degenerate one at the
  insertion point when nothing is selected
  (<https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-implementingtextandtextrange>).
  **Deviation:** a field that does not have focus returns no range, because its
  selection is not shown; the documentation does not cover this case.
- **Characters** are grapheme clusters, which is what the caret moves over. A CR
  LF pair is one cluster.
- **Lines** are the visual lines of the layout. A line includes its line break, a
  soft wrap belongs to the line after it, and the end of a text after a line break
  is an empty line of its own. The rectangles of a line run from the start of its
  text to its right edge, which excludes trailing spaces.
- **Word units** follow Windows and Notepad (`text.WordStarts`). A unit is a run
  of word characters, or a run of punctuation, followed by the spaces after it,
  so "A small note-taking app." has the units "A ", "small ", "note", "-",
  "taking ", "app", ". ". A line break is a unit of its own, and the spaces
  before it belong to the word before. The Text pattern and the editor use the
  same units: a degenerate range moved by one word lands at the start of the next
  unit, where Ctrl+Right lands, and Move(Word, -1) lands where Ctrl+Left lands.
  ExpandToEnclosingUnit(Word) of a degenerate range gives the unit that holds the
  position, including its trailing spaces. A non-degenerate range moves from the
  start of its unit, and Move(Word, 1) followed by Expand(Word) gives the next
  unit. Ctrl+Delete and Ctrl+Backspace delete to the same places as the keys.
  The rule was checked against Notepad (below), which the Windows edit controls
  follow; Microsoft's documentation does not state the punctuation rules, so
  those are Notepad's observed behaviour.

  Notepad (Windows 11), jamb's notes body, and the same text, "A small
  note-taking app. Next line" with a line break and "Second line, ok.":

  | Operation | Notepad | jamb |
  | --- | --- | --- |
  | Ctrl+Right from 0, repeated | "A ", "small ", "note", "-", "taking ", "app", ". ", "Next ", "line  ", break | the same |
  | Ctrl+Left from the end, repeated (the start of each unit passed) | "ok", ", ", "line", "Second ", break, "line  ", "Next ", ". ", "app" | the same |
  | Move(Word, 1) from 0, then Expand(Word) | "A " then "small ", "note", "-", "taking ", "app", ". ", "Next ", "line  ", break | the same |
  | Move(Word, -1) from the end | the Ctrl+Left stops | the same |
  | Ctrl+Right from inside "sm" (after Right x4) | "small " | the same |
  | Move(Word, 3) from 0, then Expand(Word) | moves to 12 (the start of "-"), expands to "-" | the same |
  | MoveEndpointByUnit(End, Word, 2) from 0 | "A small " | the same |
  | Ctrl+Delete at 0 | deletes "A " | the same |
  | Ctrl+Backspace at the end | deletes "." | the same |

  The two runs were compared line for line (44 lines each, identical), and the
  mid-word run matched as well. Intentional differences: none measured. Not
  checked: the leading spaces of a text, and a space typed after a line break.
- **Paragraphs** end with their line break; the last one may be empty.
- **Document, page and format** are the whole text: a plain text has one format,
  and a text field has one page.
- **Attributes.** IsReadOnly is always false, since no field is read-only yet. The
  font name, size (in points, at 96 per inch), weight and italic are reported,
  and a colour is not, so ForegroundColor and any other attribute is
  NotSupported. A field's attributes are the same throughout, so an attribute
  does not depend on the range.
- **Visible ranges** of a field are its whole text. Scrolling does not split them.
- **Events.** The text events follow
  <https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-implementingtextandtextrange>:
  the text-changed event after any text change, and the selection event whenever
  the selection or the caret moves.

### Verification

Unit tests cover the points and their conversions (ASCII, combining marks,
surrogate pairs, CR LF and Arabic), the expansion, movement and endpoint rules
against hand-worked tables, the word units against Notepad's stops and the editor's keys,
the wrapped lines and rectangles against the layout, selection requests, the
events from snapshots, range validity, the vtable layout, the refcounts, and the
zero-allocation hot paths (`uia_alloc_test.go`).

Live checks were made against the notes body (a text area with two lines), the
demo's search field, and a UI Automation client. Two client harnesses were used:
PowerShell's .NET UI Automation client, and a compiled C# handler for the events,
since PowerShell script blocks called from the event thread did not run.

| Operation | Notes body | Demo search field |
| --- | --- | --- |
| DocumentRange, GetText | the two-line text | empty |
| GetSelection at the caret | degenerate | degenerate |
| Move by Character, 3 | moved 3 | none (empty text) |
| Move by Word, 1, then Expand to Word | "Hello" before the Windows word units; under them the expected result is moved 1, "notes " (not re-run on this text) | none |
| Expand to Line at the caret | "Hello notes body" with its break | none |
| Move by Line, 1 | moved 1 | none |
| GetBoundingRectangles | two rectangles, one per line: x 759, y 411 (113 by 24) and y 435 (528 by 24), screen pixels | none |
| FindText "second" | found | none |
| FindText "LINE", backward, ignoring case | "line" | none |
| Select the found word, then GetSelection | "second" | none |
| RangeFromPoint on line 1, then Expand to Line | "Hello notes body" | none |
| Typing, Ctrl+Left and Shift+Left (script) | the text and the selection follow | "Beta" typed, caret moves 27 px |

Events, counted with the C# handler: typing three characters in one frame gave one
TextChanged; typing, Ctrl+Left, Shift+Left and Select gave four
TextSelectionChanged; the Value property changed once.

System caret, from GetGUIThreadInfo (client coordinates): the body's first line
(348,244) when the body has focus; (395,268) with "second" selected (the caret at
its end); (373,268) after "XYZ" replaces it; (348,268) after Ctrl+Left; (461,244)
after Shift+Left selects the line break. Focus on the title moved it to (348,110),
focus on a button destroyed it (no caret), and focus on the body restored it. A
magnified screenshot shows jamb's single caret and no second caret.

Phase A and B were re-checked on the notes app: a raw tree walk reached 23
elements, the Pin note toggle went from Off to On (by Invoke, which Phase E removed
from toggles, so the check now uses Toggle), and Invoke on New note added
the note's fields.

### Memory

Each operation was run 5000 times after a warm-up of 3000, with a client-side
collection every 500, against the notes body (about 110 characters) in jamb and a
Notepad document of the same text. The growth below is the process's private
memory per call, after warm-up.

| Operation | jamb before | jamb after | Notepad |
| --- | --- | --- | --- |
| GetSelection | 0.029 | 0.100 at 5000 calls; 0.012 over 20,000 (collector pacing, see below) | -0.018 |
| DocumentRange | 0.006 | 0.030 | -0.030 |
| Clone | 0.051 | 0.029 | -0.018 |
| Move (Word, 1) | 0.029 | 0.264 (one step of 1.2 MB, then flat) | -0.029 |
| ExpandToEnclosingUnit (Line) | 0.219 | 0.014 | 0.027 |
| GetText | 0.683 | 0.019 | -0.002 |
| GetBoundingRectangles | 0.265 | 0.194 (one step, then flat) | 0 |
| FindText | 0.046 | 0.022 | -0.023 |

KB per call. "Before" is the adapter before the change described below, measured
with the same protocol.

The cause was the adapter's own Go garbage, not a leak. A forced collection in
the adapter (a temporary instrument, removed afterwards) showed that the Go heap
was constant once collected: after the fix, 22,000 DocumentRange calls left the
heap at the same size and object count, and the registry of cores at its
baseline. Before the fix the steady rise was garbage that the collector had not
yet run over. Three sources made it: GetText built a `[]rune` and a `[]uint16`
for every call, and every range object allocated its core, its views slice and a
pointer to its range. GetText now writes the BSTR in place (`bstrOf`,
`win32.SysAllocStringN`), the views live in the core, the range is stored under
the registry lock rather than in a pointer, and freed cores are reused with a new
handle (`freeCores`, `recycle`), so a steady range object makes no Go allocation.
A view that outlived its object cannot reach a reused core, since the reused
core has a new handle. The scaling check after the fix: a 20-character body
gives 0.017 KB per GetText call, and a 20,000-character body 0.005 KB, both flat;
before, 0.26 and 1.94.

Warm-up matters for the figures. The collector runs when the heap reaches its
goal, so a short run can show a rise that is only waiting for a collection; the
forced-collection run is what separates the two. The one steps of 0.9 to 1.2 MB
in the table are arena growth in the Go heap, after which the samples are flat.
The `SAFEARRAY`s are owned by UI Automation once returned, and a test
(`TestRangeArrayOwnsItsRanges`) checks that the array keeps its ranges alive and
frees them when destroyed.

### Open items

- **IME composition.** GetActiveComposition and GetConversionTarget
  (ITextEditProvider) and UiaRaiseTextEditTextChangedEvent are not implemented.
  ITextEditProvider was skipped as not trivial; the composition text is not
  exposed as a range.
- **Static and rich text.** The Text pattern is only on text fields and text
  areas. A static text node's name carries its text, and the links of rich text
  are children of it. A Text pattern on read-only rich text is not built.
- **Annotations and embedded objects.** RangeFromAnnotation is not supported, and
  RangeFromChild returns no range, since a field has no child with text. The
  framework has no embedded objects.
- **Attributes.** Colours, per-span attributes and mixed values are NotSupported.
  No field is read-only, so IsReadOnly is false. A password flag exists
  (`access.StatePassword`, reported as IsPassword) and no API sets it yet.
- **Cost.** Building the content is linear in the text, about 2.4 ms for a 22 KB
  wrapped text, and runs on each change to the text; grapheme segmentation is most
  of it. A large text that changes on every keystroke may need incremental
  segmentation.
- **Collector pacing.** The adapter's per-call Go garbage was reduced (see the
  Memory section). Between collections the private memory still rises by the
  garbage made since the last one, so a short run shows a slope that a long run
  does not. Notepad makes no Go garbage per call, which is why its column is flat.
- **Visible ranges and scrolling.** Visible ranges are the whole text, and a
  rectangle outside the field is dropped rather than scrolled into view.

## Phase D: what changed from this design

Phase D reads four Windows settings and applies them to every window, and checks
the built-in themes' contrast. The settings are read, never written. Sources are
linked where each setting is described.

### Settings and where they are read

| Setting | Read with | Source |
|---|---|---|
| High contrast on, and its colors | `SystemParametersInfo(SPI_GETHIGHCONTRAST)` (HCF_HIGHCONTRASTON); `GetSysColor` for the 15 colors in `platform.SystemColors` | [SystemParametersInfo](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-systemparametersinfoa), [GetSysColor](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getsyscolor) |
| Text size, 100 to 225% | `RegGetValueW` on `HKCU\Software\Microsoft\Accessibility`, value `TextScaleFactor` (DWORD) | [Accessible text requirements](https://learn.microsoft.com/en-us/windows/apps/design/accessibility/accessible-text-requirements) describes the setting and its range. The registry location is not in the official documentation: it comes from community answers, so verify it on a PC (see the manual steps in [accessibility-testing.md](accessibility-testing.md#high-contrast-and-text-size)). |
| Animation effects off | `SPI_GETCLIENTAREAANIMATION` (BOOL) | [SystemParametersInfo](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-systemparametersinfoa) |
| Caret blink time (INFINITE = no blink) | `GetCaretBlinkTime` | [GetCaretBlinkTime](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getcaretblinktime) |
| Text cursor thickness | `SPI_GETCARETWIDTH` (DWORD, pixels; default 1) | [SystemParametersInfo](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-systemparametersinfoa) |

The settings are read at start, and again on `WM_SETTINGCHANGE` and
`WM_SYSCOLORCHANGE`, which the window procedure turns into
`platform.EventSettingsChanged`. The app package (`app/settings.go`) converts the
platform's values, applies the dev overrides, and gives the result to
`ui.Runtime.SetSystemSettings`. Values are clamped: text scale to 100 to 225, caret
width to 1 to 20. `internal/platform/linux` and `internal/platform/darwin` report
`platform.DefaultSettings()`, since no adapter reads the settings there yet.

### API

- `ui.SystemSettings` (HighContrast, Colors, TextScale, ReducedMotion, CaretBlinkOff,
  CaretBlink, CaretWidth) and `rt.SetSystemSettings`, `rt.SystemSettings`.
- `ctx.HighContrast()`, `ctx.ReducedMotion()`, `ctx.TextScale()` (percent), and
  `rt.HighContrast()`, `rt.ReducedMotion()`, `rt.TextScale()`.
- `ui.HighContrastTheme(colors, base)`, `ui.HighContrastBlack()` and
  `ui.HighContrastWhite()` (representative schemes, used by the dev override; not
  read from a system).
- `Theme.HighContrast` (true for the high contrast theme) and `ctx.AppTheme()` /
  `rt.AppTheme()`, the theme the app set with `SetTheme`. Under high contrast the
  active theme is the high contrast one, so the app's own selection must use
  `AppTheme`.

### High contrast

Microsoft's guidance for contrast themes is at
[Contrast themes](https://learn.microsoft.com/en-us/windows/apps/design/accessibility/high-contrast-themes):
the system colors are the palette, text uses WindowText, selected and interacted
UI uses Highlight with HighlightText, buttons use ButtonText and ButtonFace,
hyperlinks use HotLight, disabled UI uses GrayText, and borders keep their
boundaries. The guidance also says not to hard-code colors in a contrast theme.

`ui.HighContrastTheme` maps the tokens this way:

| Token | System color |
|---|---|
| Background, Sidebar, Surface, SurfaceHover | Window |
| SurfaceActive (selection, pressed) | Highlight |
| Text, TextMuted, TextSubtle, Border, BorderStrong, Scrollbar, Success, Warning, Danger | WindowText (status is shown by text and icons) |
| Accent, AccentHover, AccentActive, FocusRing | Highlight |
| AccentText | HighlightText |
| AccentSoft | Window (no tint) |
| Scrim | Window (opaque) |
| Link (hyperlinks) | HotLight |
| Disabled (text, icons, borders of disabled controls) | GrayText |

Radius and FontSize come from the app's theme. High contrast is automatic and
has no opt-out: no clear reason for one was found, and an app that picks its own
colors can use `ctx.HighContrast()`.

Painting follows the same rules automatically: a blurred or offset shadow is not
drawn, a crisp ring (a focus ring) is; a gradient is drawn as its first color, solid;
an icon takes the text color (`fg`) instead of its own color; images stay images.

The demo adds three things that the framework cannot know: the Dashboard's banner
picture is replaced by a plain panel with theme text, since its gradient cannot
hold the text's contrast; chart bars use the solid color; and chips use the
surface instead of a tint (`tintOf`, `barFill` in `examples/demo/views/kit.go`).
The selected navigation item uses `AccentText` for its label, and the theme switch
selects the app's theme (`ctx.AppTheme()`).

The parts of the built-in widgets (`Tabs`, `Select`, `Slider`, `NumberInput`) follow
the same rules. The theme's and an instance's part styles lose their colors,
gradients, shadows and opacity under high contrast, and keep their sizes, spacing,
fonts, radius and border widths, so the built-in system colors and focus ring stay.
A render slot's content is hidden from assistive technology, so it cannot change
what the widget announces. See [styling.md](styling.md).

Custom graphics that show a state were checked under high contrast, since a color
that carries a state has to be replaced by something that does not depend on color.
Fixed: the progress bar (an outlined empty track, a solid Highlight fill, the
percentage as text); the switch (on: a Highlight track with an AccentText knob; off:
an outlined track with a text-colored knob; both before, the off track was the same
Highlight as the on one); a starred project and a pinned note (a filled star, in the
demo's `starIcon` and the notes' pin, so the state shows in the shape); the own
message in a comment thread (a Highlight edge); the selected filter (an outline); the
Danger button's label (the window color, since its fill is the text color). Not
changed: the chart bars, badges, activity icons and stat tiles, whose meaning is in
their height, text or position, not in their color.

Verified in screenshots (black and white schemes) and by
`ui/system_settings_test.go`: the theme mapping, shadows and gradients dropped while
a crisp ring stays, icons take the text color, and the switch back to the app's theme. Tints are replaced in the demo (tintOf), not by the framework, since a framework rule would also hide chart bars and selections.

### Contrast audit of the built-in themes

`ui/contrast_test.go` computes WCAG 2 contrast ratios for each pair the built-in
widgets draw, in both themes, and fails when an enforced pair falls below its
minimum: 4.5:1 for text (WCAG 2.1
[1.4.3](https://www.w3.org/TR/WCAG21/#contrast-minimum)) and 3:1 for interactive
graphics (WCAG 2.1 [1.4.11](https://www.w3.org/TR/WCAG21/#non-text-contrast)).
Translucent tokens are composited over their surface first. Disabled text is
recorded but exempt (WCAG 1.4.3 exempts inactive components). Decorative lines are
recorded without a minimum. A pair that is a known gap fails if it starts to pass,
so the entry is removed.

Token changes (before, then after, with the minimum):

| Pair | Theme | Before | After | Minimum | Change |
|---|---|---|---|---|---|
| TextSubtle on Background, Sidebar, Surface, SurfaceHover, SurfaceActive | dark | 4.04, 3.90, 3.67, 3.40, 3.08 | 6.01, 5.80, 5.45, 5.05, 4.58 | 4.5 | TextSubtle #71717a to #8e8e96 |
| TextSubtle on the same surfaces | light | 2.46, 2.33, 2.56, 2.33, 2.02 | 5.79, 5.50, 6.04, 5.50, 4.76 | 4.5 | TextSubtle #a1a1aa to #62626a |
| Placeholder (TextSubtle) on Surface | dark / light | 3.67 / 2.56 | 5.45 / 6.04 | 4.5 | (same token) |
| AccentText on Accent | dark | 4.47 | 4.56 | 4.5 | Accent #6366f1 to #6265ed |
| AccentText on AccentHover | dark | 3.73 | 5.00 | 4.5 | AccentHover #7376f4 to #5b5ee7 |
| AccentText on AccentActive | dark | 5.50 | 5.50 | 4.5 | none |
| White label on Danger, hover, pressed | dark | 3.76, 3.46, 4.53 | 5.01, 4.56, 5.94 | 4.5 | Danger #ef4444 to #d62828 |
| White label on Danger, hover, pressed | light | 4.83, 4.43, 5.74 | 5.15, 4.66, 6.10 | 4.5 | Danger #dc2626 to #d12a2a |
| BorderStrong (control border) on Background, Sidebar, Surface | dark | 1.73, 1.67, 1.57 | 3.43, 3.31, 3.11 | 3 | BorderStrong #3a3a41 to #66666d; controls now draw BorderStrong at rest |
| BorderStrong on the same surfaces | light | 1.42, 1.34, 1.48 | 3.28, 3.12, 3.43 | 3 | BorderStrong #d4d4d8 to #8a8a92 |
| Scrollbar thumb at rest on Surface | dark | 1.21 | 3.23 at alpha 0.35, 5.24 at 0.5 | 3 | Scrollbar #ffffff33 to #ffffff; rest alpha 0.35 to 0.5 (scrollbar.go) |
| Scrollbar thumb at rest on Surface | light | 1.21 | 2.44 at alpha 0.35, 3.98 at 0.5 | 3 | Scrollbar #0000003d to #000000; rest alpha 0.5 |
| Link text on Background, Sidebar, Surface | dark | Accent: 4.38, 4.22, 3.97 | Link token: 9.80, 9.46, 8.89 (all five surfaces 7.47 to 9.80) | 4.5 | new Link token (#a5b4fc); Accent kept for fills |
| Link text on the light surfaces | light | Accent passed | Link token: 6.23 to 7.90 | 4.5 | new Link token (#4338ca) |
| Disabled text | both | TextSubtle (no change in normal themes) | Disabled token; GrayText under high contrast | exempt (1.4.3) | normal themes dim with Opacity as before |
| Demo badge text (Accent.Lighten(0.3) on AccentSoft) | light | 2.84 | text now Text (passes) | 4.5 | demo badge uses t.Text |
| Input and secondary-button border at rest | both | Border (1.22 to 1.30) | BorderStrong; hover TextMuted | 3 | widgets.go |

The values above are the test output (the ratios listed in the test file), rounded.


Changed after the audit, at the user's direction:

- **Links** use their own token, `Theme.Link` (dark #a5b4fc, light #4338ca; under
  high contrast HotLight). The audit's link pairs, on every surface, reach 6.2:1 or
  more in both themes (see the `Link_on_*` rows of the test). Accent stays the
  primary button fill.
- **Disabled controls** use `Theme.Disabled`: in the normal themes it equals
  TextSubtle and disabled controls keep their `Opacity` dimming (WCAG 1.4.3 exempts
  inactive components). Under high contrast it is GrayText, and disabled buttons and
  menu items are drawn in it with a GrayText outline, not dimmed.

Still recorded, not enforced: Danger, Success and Warning as text (the built-in
widgets do not use them as text; Danger text on the dark Surface is 3.54), and the
Scrim and the hovered scrollbar thumb (informational).

Both themes keep their character: the dark theme stays dark, and the light theme
stays light. AccentHover in the dark theme is now darker than Accent, since a lighter
hover cannot hold a white label at 4.5:1.

### Text size

The Text size setting scales text only. `Runtime.textStyle` multiplies the font
size by `textFactor` (percent / 100), and line heights follow, since they are
multiples of the size. Zoom and the display scale are separate and multiply as before:
a test checks that the logical text height is the same at any zoom and device scale
for the same text size, and that 225% is 2.25 times 100%.

Fixed heights became minimum heights, so content grows instead of overlapping.
Changed in the framework (`ui/widgets.go`, `ui/menu.go`, `ui/menubar.go`): buttons,
text fields, menu rows and menu bar items. Scaled in the framework:
`ui/vlist.go` estimates an unmeasured row at 32 px times the text factor, and a
multi-line field's minimum height scales with `ctx.TextScale()`. The rows of
`ListOf` and the virtual list are measured, so they follow their content.

Changed in the examples, with `layoutScale` (the text size, capped at 150%) for
wide controls:

- Demo: the sidebar width, the search width, navigation rows and badges are minimum
  heights; the title bar is a minimum height; the header wraps onto a second line
  when it is narrow (breadcrumb first, then search, bell and New Project), so no label
  is cut.
- Notes: the header bars and the menu bar are minimum heights; the menu bar's
  height and the content's top padding scale together; the "Add tag" field widens.

Checked at 150% and 225% in screenshots (`docs/screenshots`): demo Components,
Projects, Activity, Settings and Dashboard; notes. Known: a single-line field
(the notes title) scrolls its text horizontally when the text is wider than the
field, as any input does. Recorded as a limitation in the README.

### Reduced motion

With reduced motion (`ctx.ReducedMotion()`):

- A `Transition` jumps to its target (`animate` in `ui/paint.go`).
- `Enter` and `Exit` are not run; an element that leaves is not kept as a ghost
  (`retireNode`, `motion`).
- A scroll jumps to its target (`scrollStep`); the hover is refreshed after a jump.
- A finite `Animate` renders its end value and asks for no frame.
- A repeating `Animate` renders its first frame (progress 0) and asks for no frame,
  so it stops. Apps that want a different still frame can check `ctx.ReducedMotion()`.

Caret: the blink uses the system's blink time (`caretVisible`), and the blink
stays solid when the system's blink is off. The caret width is 1.5 logical pixels
times the system's cursor thickness; the system value is in device pixels, and the
scale of the display applies to it as to the rest of the UI.

### Dev overrides

`JAMB_HIGH_CONTRAST`, `JAMB_TEXT_SCALE` and `JAMB_REDUCED_MOTION` are documented
in `app/dev.go` and in the README. They simulate the settings for one run and
change no system setting. Normal use reads the real settings only; a variable
replaces its setting only when it is set.

### Part 1: the idle step

The Phase E open item below ("The `idle` script step") is fixed. The cause was not in
the idle step: the Components page never settles because a hover feedback loop
keeps the runtime animating. A hover style that lifts an element (here the
"Hover: scale and lift" card) also moved its hit box, since hit testing used the
painted box (`node.abs`, which includes the element's offset). When the pointer
rests on the lifted card's edge, the card leaves the pointer, the hover ends, the
card drops back under the pointer, and the hover begins again. The transition
never ends, and `Settled()` stays false.

The fix: a separate hit box (`node.hit`), set in `paintNode` to the layout box
moved only by the element's own base offset (`n.div.style`), not by its hover,
focus, press styles, transitions or motion. `hit()` tests that box. A static
`Style.Offset` still moves the hit area, so a click where the element is painted
still reaches it. Tests: `ui/hit_test.go` (the hover does not move the hit box and
the runtime settles; a static offset does move it) and
`examples/demo/views/components_idle_test.go` (the Components page settles after a
scroll with the pointer at the card's edge). Both fail without the fix.

Evidence: the script from the task (`wait 1500; key ctrl+4; wait 800; scroll 700 500 600`
three times, `idle`, `shot`, `exit`) completes. The probe found the loop at pointer
(600, 400) in the real app: the script timed out at 25 s before the fix and completes
in about 3 s after it. A sweep of 84 pointer positions in the headless runtime settled
at every point after the fix. With a trace (`JAMB_TRACE=1`) after the idle step, the
frames are the five repeating Motion bars only (`rendered=5`, no layout, no build),
at the display's rate, as repeating animations require.

### Keyboard context menus (Shift+F10)

Shift+F10 and the Menu key open a context menu from the keyboard. Two faults stopped
a keyboard or screen reader user from moving through it, and both are fixed.

- **F10 was handled by Windows, not the app.** F10 is a system key. The window
  passed its key-down to `DefWindowProc`, which turns F10 into the menu bar's keyboard
  mode, a native menu loop. That loop took the keys that followed (Down, Tab) and
  left only on Escape, so the app never saw them: after Shift+F10 the menu opened,
  but Down did nothing. Found with `GetGUIThreadInfo` (the thread's `menuOwner` was
  the window) and the window's key messages. The fix is in `internal/platform/windows/window.go`
  (`ownsF10`): F10 without Alt is the application's, and the default handling is not
  run for it. Alt+F10 keeps the system's handling.
- **The focus did not move to the menu's items.** The menu's highlight lived in the
  menu's own state, so the access tree reported the menu as focused, with no item. A
  screen reader then heard the menu's name, not the item. Now a menu that has focus
  passes it to its highlighted row (`highlightedRow` in `ui/access.go`, the flag is
  set by the menu row). The adapter raises a focus event for each change of focused
  node, so each Down and Up is announced. Dropdown menus and the menu bar use the same
  rows, so they behave the same way.

Tests: `ui/menu_focus_test.go` (Shift+F10 focuses the first item; Down and Up move the
focus item by item; a pointer-opened menu is named and focused; a dropdown follows the
same rule; Escape returns the focus to the element). They fail without the change.
`scripts/a11y-check.ps1` has a live check that focuses a project card, presses
Shift+F10 and Down through `SendKeys`, and checks by UI Automation that the focused
element is "Open", then "Open in window", then "Escape" returns it to the card. It
failed on the build without the F10 fix.

The Axe state 11 (the card context menu with the Status submenu) is reached by the
same keys: Shift+F10, Down three times to Status, Right.

### Phase D verification

AxeWindowsCLI scans of the 18 Phase E states, with no overrides, with
`JAMB_HIGH_CONTRAST=1`, and with `JAMB_TEXT_SCALE=225`. Errors per state; zero in
all 54 scans.

| State | Normal | High contrast | Text 225% |
|---|---|---|---|
| 1 Dashboard | 0 | 0 | 0 |
| 2 Projects (default) | 0 | 0 | 0 |
| 3 Activity, build row expanded, filter "build" | 0 | 0 | 0 |
| 4 Components, top | 0 | 0 | 0 |
| 5 Components, scrolled about 600 px | 0 | 0 | 0 |
| 6 Components, scrolled about 1200 px | 0 | 0 | 0 |
| 7 Components, scrolled about 2400 px (clamped to the bottom) | 0 | 0 | 0 |
| 8 Components, dropdown open | 0 | 0 | 0 |
| 9 Settings | 0 | 0 | 0 |
| 10 New Project dialog | 0 | 0 | 0 |
| 11 Card context menu, Status submenu (Shift+F10, Down three times, Right) | 0 | 0 | 0 |
| 12 Project detail window (both top-level windows scanned) | 0 | 0 | 0 |
| 13 Notes default | 0 | 0 | 0 |
| 14 Command palette | 0 | 0 | 0 |
| 15 File menu | 0 | 0 | 0 |
| 16 Edit menu | 0 | 0 | 0 |
| 17 View menu | 0 | 0 | 0 |
| 18 Tag editor, New tag focused | 0 | 0 | 0 |

Scroll positions are approximate pixel targets, set by the scroll percentage of
the live viewport; the 2400 px state is clamped to the bottom of the page. The
scan driver is not in the repository.

`scripts/a11y-check.ps1` (20 checks: the 18 of Phase E, plus the Shift+F10 check on
a project card and the F10 check on the notes menu bar), each run with two Axe scans:
normal `Summary: 20 passed, 0 failed`; `-HighContrast` `Summary: 20 passed, 0 failed`;
`-TextScale 225` `Summary: 20 passed, 0 failed`.

Tests: `gofmt -l .` prints nothing; `go vet ./... ./examples/demo/... ./examples/notes/... ./cmd/jamb/...`
is clean; `go test -count=1 ./... ./examples/demo/... ./examples/notes/... ./cmd/jamb/...`
passes in every package; `GOOS=linux go build ./access/ ./ui/ ./app/ ./text/` builds.

The Axe scans ran before the plain-F10 menu bar change. That change only adds a
case for F10 with no modifiers, which none of the 18 states presses.

## Phase E: what changed from this design

Phase E's automated parts are in place: the Axe.Windows rules (the engine of
Accessibility Insights) run on both examples in every significant state through
AxeWindowsCLI, the errors they found are fixed, a live
UI Automation check runs in `scripts/a11y-check.ps1`, and the manual screen reader
script is in [accessibility-testing.md](accessibility-testing.md). The NVDA and
Narrator pass is for a person to run, and is pending.

### Tools

- **AxeWindowsCLI 2.4.2** (the Axe.Windows rules engine), from the official
  release at github.com/microsoft/axe-windows. The winget package
  `Microsoft.AxeWindows` is that release's MSI. Its install needs elevation, because
  the MSI's .NET 6 runtime dependency is installed machine-wide, so the signed zip
  was used instead, unpacked to `%LOCALAPPDATA%\Programs\AxeWindowsCLI`. Every
  executable and DLL in it carries a valid Microsoft or .NET Authenticode
  signature.
- **Accessibility Insights for Windows 1.1.2924.1**, the latest stable release
  from github.com/microsoft/accessibility-insights-windows. It is not in winget.
  Its MSI is a machine-wide install that needs elevation, so the MSI was extracted
  with an administrative install to `%LOCALAPPDATA%\Programs\AccessibilityInsights`.
  The extracted app is signed by Microsoft and starts. It is not registered: there
  is no Start menu entry and no uninstall entry.

The scans use AxeWindowsCLI. The Accessibility Insights app was installed and
started, but its own assessment and FastPass tests were not run, since they are
interactive. The CLI runs the same rules non-interactively.

Axe.Windows 2.4.2 has 154 rules: 76 at Error, 16 at Warning and 62 at
NeedsReview. The CLI reports errors. No scan log had warning or needs-review
output.

### Scan results

Errors per state, as reported by AxeWindowsCLI, before and after the fixes. The
demo's states are on the Projects page unless noted.

| State | Before | After |
| --- | --- | --- |
| Demo, Dashboard | 1 | 0 |
| Demo, Projects (default) | 7 | 0 |
| Demo, Activity (one build row expanded, filter "build") | 1 | 0 |
| Demo, Components, top | 7 | 0 |
| Demo, Components, scrolled 600 px | 7 | 0 |
| Demo, Components, scrolled 1200 px | 7 | 0 |
| Demo, Components, scrolled 2400 px | 7 | 0 |
| Demo, Components, dropdown open | 8 | 0 |
| Demo, Settings | 1 | 0 |
| Demo, New Project dialog | 8 | 0 |
| Demo, context menu with the Status submenu open | 8 | 0 |
| Demo, project detail window (comments) | 8 | 0 |
| Notes, default | 2 | 0 |
| Notes, command palette open | 2 | 0 |
| Notes, File menu open | 3 | 0 |
| Notes, Edit menu open | 3 | 0 |
| Notes, View menu open | 3 | 0 |
| Notes, tag editor (New tag field focused) | 2 | 0 |

Three rules produced all the errors before the fixes:

- **ButtonInvokeAndTogglePatterns** (Error, WCAG 4.1.2): a toggle button exposed
  both Invoke and Toggle. It hit every star, the Pin note toggle and the Toggle
  theme button.
- **ButtonShouldHavePatterns** (Error, WCAG 4.1.2): a button had none of Invoke,
  Toggle or ExpandCollapse. It hit the disabled Create project button and the
  disabled Components sample, the Components sample buttons with no action, and
  the Notifications bell, which had no action.
- **NameNotNull** (Error, Section 508 502.3.1): a focusable menu panel had no name.
  It hit the Components dropdown menu, the context menu and the notes menu bar's
  File, Edit and View popups.

### Fixes

Framework (the generic problems):

- `internal/platform/windows/uia.go`: a toggle has Toggle and not Invoke. A disabled button
  keeps its Invoke pattern, which the design already says a disabled element does.
  The pattern is still absent for an enabled element with no action.
- `ui/menu.go`, `ui/menubar.go` and `ui/widgets.go`: a menu panel takes a name. A
  dropdown's menu takes its trigger's name, a menu bar popup takes its menu's
  name, and a context menu takes the name of the element it hangs from, or
  "Actions" when that element has none. `ButtonElement.accessName` gives the
  trigger's name, in the same order the access tree uses: Label, text, tooltip.

Examples (app content):

- `examples/demo/views/app.go`: the Notifications bell announces "No new
  notifications" when it is pressed. The demo has no notifications feature, so the
  text is a placeholder to confirm.
- `examples/demo/views/components.go`: the five enabled sample buttons (Primary,
  Secondary, Ghost, Danger, Small) announce "<name> pressed" through a `pressed`
  helper. They had no action. The text is a placeholder to confirm.

Regression tests: `ui/access_controls_test.go` (a request on a disabled button does
nothing, menu panels are named, a context menu is named by its element);
`internal/platform/windows/uia_controls_test.go` (`TestInvokeFollowsTogglesAndDisabledButtons`).
Two existing tests looked up unnamed menus, and now look up "Actions", the name
a context menu gets. A request on a disabled button already does nothing, since a
disabled element has no request target (the rule that makes targets, in
`ui/access.go`, leaves out a disabled element that has no actions), so that test
needed no code change.

### Justified exceptions

None. Every error the rules raised was a real defect, and each is fixed above. The
two placeholder actions are content decisions, listed under open items.

### Live checks

`scripts/a11y-check.ps1` builds both examples, runs them (the notes app with a
temporary notes file), checks them through UI Automation and scans them with
AxeWindowsCLI. It ran 18 checks, all passing, on this tree:

- Names and roles on the navigation, the toolbar and the notes menu bar.
- Invoke opens a page; a toggle has Toggle and no Invoke (the star and the Pin
  note).
- Expand and collapse on an Activity build row, the dropdown trigger and the File
  menu, with the menu named after its trigger.
- Selection on the filter chips, with the group reporting its one selection.
- Value on the search field and the note title, with SetValue.
- Focus moves to the search field, and a focus event is raised for it.
- Scroll: the Components page scrolls to 50 percent with SetScrollPercent.
- A disabled button keeps Invoke (Create project); Cancel closes the dialog.
- Settings switches toggle with Toggle.
- The note body: its text holds the welcome sentence; FindText finds "small";
  Select selects it; the selection and the word unit are "small"; the word has one
  bounding rectangle.
- AxeWindowsCLI reports no errors on the demo's Settings page and on the notes
  editor.

The script was also run against the commit before this phase (extracted from git,
with the same script). It failed seven checks: the star and Pin toggles (Invoke
present), the dropdown and File menus (not named), Create project (no Invoke), and
the two Axe scans (2 and 3 errors). Those are the defects the fixes address.

### Open items

- **Manual passes.** NVDA and Narrator are for a person to run, with
  [accessibility-testing.md](accessibility-testing.md).
- **The two placeholder actions.** The Notifications bell and the sample buttons
  now announce something. Confirm the wording, or give the bell a real action.
- **The `idle` script step.** Fixed in phase D (see "Part 1: the idle step"). The
  cause was a hover feedback loop, not the idle step.
- **Scanner coverage.** The CLI reports errors only, and this phase did not run
  the Accessibility Insights assessment, which needs a person.

## Phase F: what changed from this design

Phase F adds the form controls to the framework and closes the two open items of
phase B that were about them: the mixed state had no public API, and RangeValue
was not implemented. The decisions and their sources are below.

- **Roles and patterns.** A switch is a check box with the Toggle pattern: its
  ControlType is CheckBox and its LocalizedControlType is "switch", as phase B
  chose. A check box in the mixed state reports ToggleState Indeterminate. A select
  is a ComboBox with ExpandCollapse and Value; the Value is the chosen label and is
  read-only. Its list is a List whose items have SelectionItem, under a
  SelectionContainer, and the highlighted item has accessibility focus, as a menu's
  highlighted row does. A radio group is a Group that is a SelectionContainer, and
  its radio buttons are RadioButton with SelectionItem. A slider is a Slider with
  RangeValue. A number input is a Spinner that holds its Edit, with RangeValue.
- **Control types from Microsoft.** The control type identifiers are ComboBox
  50003, Slider 50015 and Spinner 50016
  ([Control Type Identifiers](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-controltype-ids)).
  The Spinner page describes a spinner as a tree of an edit (0 or 1) and two
  buttons, with RangeValue for a numeric range
  ([Spinner Control Type](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-supportspinnercontroltype)).
  The Slider page requires RangeValue for a numerical range
  ([Slider Control Type](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-supportslidercontroltype)).
  The number input therefore follows the Spinner structure. The slider needs no
  Thumb child; the Slider page lists thumbs and buttons as the structure of the
  control, and jamb draws its thumb without exposing it.
- **RangeValue.** `access.Range` (Value, Min, Max, SmallChange, LargeChange) is set
  on the node, and Diff raises a range-value property change. ActionSetRangeValue
  sets the value through the element, which clamps and snaps it. The Windows adapter
  implements the provider interface IRangeValueProvider (IID
  36dc7aef-33e6-4691-afe1-2be7274b3d33). The property IDs come from
  UIAutomationClient.h in the Windows 10.0.26100.0 SDK: the pattern is 10003, the
  availability property 30033, and the value, read-only, minimum, maximum, large
  change and small change properties 30047 to 30052 in that order, with Minimum at
  30049 and Maximum at 30050.
- **Mixed state API.** `CheckboxTri(label, state)` takes a `*Signal[CheckState]`
  with `CheckOff`, `CheckOn` and `CheckMixed`. `Checkbox(label, checked)` takes a
  bool and cannot be mixed. A click on a mixed box makes it on, as Windows check
  boxes do. A mixed box is not checked: its state is the mixed one.
- **Keys of the controls.** Space toggles a check box and a switch, and chooses a
  radio button. Enter does not: the runtime runs a click for Enter only on an
  element that is not `spaceOnly`, and check boxes, switches and radio buttons
  are, so Enter goes on to the modal's default button or the form, as on Windows.
- **Radio group focus.** One Tab stop: the selected option, or the first enabled
  one when none is selected. The others are reached with the arrows, which move the
  focus and select the option they reach. Up and Left go back, and Down and Right go
  on. The group wraps and skips disabled options. The Windows behavior was not
  checked against a source in this phase; the choice is a design decision.
- **Select.** It reuses the overlay of a menu: an Overlay below the field, which
  flips when there is no room, a click outside that closes it, and AutoFocus. It
  also reuses the menu's focus return (menuOpened and menuClosed), so focus goes
  back to the field however the list was opened. Its keys are its own, since a list
  does not wrap and has no submenus. Closed, Up and Down choose the neighbouring
  option, and Home and End the ends, as the task asked for a Windows combo box.
  Type-ahead matches the typed letters from the current option, and a pause of one
  second starts a new search.
- **Runtime fix.** A radio button is a selection item by its role, but the runtime
  ran the Select action only for items with the Selected flag. A radio button set
  with Checked could not be selected through the bridge. The check now includes the
  radio roles, which also covers menu radio items.
- **Slider.** A click or drag sets the value to the nearest step. The arrows move by
  one step, Page Up and Page Down by a tenth of the range rounded up to steps, and
  Home and End go to the ends. `MouseEvent` now carries the size of the element that
  handles it (Width and Height), which the slider needs to map a position to a value.
- **NumberInput validation.** Chosen and documented in the README: a number outside
  the range is clamped on commit; text that is not a number (or not a whole number
  for an int) is not applied, and Enter shows the field as invalid with a message
  that gives the range. Leaving the field with invalid text reverts it to the value.
  Up and Down step the value. The editor's own Up and Down are replaced for a number
  field only.
- **Switch in the demo.** The Settings switches are `ui.Switch`, and the demo's
  hand-made toggle is removed. The UI zoom slider sets the framework's zoom.

### Phase F: verification

- Unit tests (`ui/form_controls_test.go`, 21 tests): the states and the mixed
  state; Space and click toggles; disabled controls; roving focus and the arrows;
  the select's open, close, Enter, Escape, Tab, F4, Alt+Down, type-ahead, Home and
  End, and focus return; the slider's keys, pointer and SetValue; the number input's
  steps, clamping, invalid text, blur and buttons; the warning for an unnamed select;
  and a high contrast and 225 percent build with no warnings and no fixed heights on
  the fields.
- Adapter tests in `internal/platform/windows` for the control types, the RangeValue pattern
  and its values, the combo box Value, and the property mapping.
- The demo's tests check the Components page's form controls and the Settings
  switch.
- `scripts/a11y-check.ps1` runs 27 checks in the normal, high contrast and 225
  percent modes, and all pass. The new checks read a check box's Toggle and its
  indeterminate state, a radio button's SelectionItem, a select's ComboBox with
  ExpandCollapse and Value and its open list, a slider's RangeValue with SetValue,
  and a number input's Spinner with RangeValue and Edit. Axe scans report zero errors
  for the Settings page, the Components page scrolled to the form controls, and the
  same page with a select open, in all three modes.
- The Axe scan found one defect in this phase: a disabled switch named "Unavailable
  switch" repeated its control type in its name, which the Name rule forbids. Its
  name is now "Unavailable sync". The rule is general: a name must not contain the
  LocalizedControlType of its control.
- Screenshots of the form controls (dark, light, high contrast and 225 percent), the
  open select and the Settings page are in `docs/screenshots`.

- **Follow-up fixes after review.** Under high contrast a disabled slider, check
  box, switch, select and number input are drawn in GrayText throughout: the slider's
  fill, thumb and readout, the check box's fill and mark, the switch's track and knob,
  the select's text, placeholder and chevron, and the number input's frame and text.
  A selected radio button is one Highlight ring with one solid dot. It had a second,
  text-colored dot, which made a blob. The check box and radio button were also
  checked at 150 percent, with the selected radio button drawn as a clean ring.
  Enter on a check box or a switch now goes on to the modal default. Select and
  radio group have `OnChange`, which the tests cover.
- **Reduced motion.** A test opens and closes a select and toggles a switch: under
  reduced motion none of the three asks for animation frames, and without it each of
  them does.
- **The Notes text-selection check.** It failed once in the first runs. A harness
  that reran the check on 20 fresh Notes processes showed the cause: 19 of 20 runs
  read the selection before the next frame published it, which is 6 to 40 ms after
  the Select. The app is correct; the check read too early. The check now polls for
  the selection, with a 3-second timeout, and 20 of 20 fresh runs pass.
- **The Menus sample at 225 percent.** Its box had a fixed height of 64 px, so its
  wrapped text overlapped the caption. The box now has a fixed width and a minimum
  height, and grows with its text.

### Phase F: open items

- **Radio group and select keys on Windows** were not compared with a source either.
  The arrow and type-ahead rules are design decisions, listed above.
- **A select's value is read-only** to assistive technology. SetValue is ignored on
  a combo box, which is what a read-only Value pattern expects.
- **The spin box's buttons** do not have the ScrollAmount automation IDs that the
  Spinner page suggests, so an automation test cannot tell them apart by ID.
- **Narrator and NVDA** passes are pending, as for phase E, and are for a person to
  run.
- **Existing layout.** The Menus sample on the Components page has a fixed height,
  so its text overlaps at 225 percent. This predates phase F and was not changed.

## Phase G: what changed from this design

Phase G adds the navigation and layout widgets: tabs, collapsible sections, an
accordion and a tree view. The decisions and their sources are below. Where a
source states a keyboard convention, the entry cites it; the others are design
decisions, and are marked as such.

- **Access roles.** `access.RoleTree` and `access.RoleTreeItem` are appended to the
  role enum, so the existing values do not change. They map to the Tree control
  type (50023) and the TreeItem control type (50024), as UIAutomationClient.h names
  them ([Control Type Identifiers](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-controltype-ids)).
  Tab lists and tabs were already mapped to Tab (50018) and TabItem (50019).
- **Tabs.** `ui.Tabs(selected, tabs...)` takes `Tab[T]` values, each with a value, a
  label, an optional icon, a disabled flag and a `Panel` func. Only the selected
  panel is built, unless the tabs call `KeepMounted()` (see Panel state). The tab
  list is one Tab stop, at the selected tab. This follows
  [Microsoft's TabView guidance](https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/tab-view),
  which says that when focus moves into the tab strip, "the selected TabViewItem
  gains focus".
- **Tab activation (deviation).** Left and Right select the neighbouring enabled tab
  at once: automatic activation, as the task asked. TabView's guidance says the arrows
  move focus and not the selection, and that Space or Enter selects. The design keeps
  automatic activation and records the difference. Home and End go to the first and
  last enabled tab. Arrows do not wrap, as in TabView.
- **Ctrl+Tab (decided).** Ctrl+Tab and Ctrl+Shift+Tab select the next and previous
  enabled tab and wrap around. TabView's guidance lists them as its adjacent-tab
  shortcuts and says the tab list "is looped". The shortcut is bound on the whole
  tab group, so it works from the tab list and from the panel, and focus moves to
  the selected tab. The framework bound Ctrl+Tab nowhere else: it moved focus before,
  and inside a tab group it now selects a tab. Tab itself is not changed.
- **Tab accessibility.** The tab list is a Tab control with the selection-container
  state. Each tab is a TabItem with SelectionItem and Select, and it has no Invoke
  pattern. The panel is a Pane named by its tab's label, and LabelledBy names it by
  the tab too. A tab list reports IsSelectionRequired, since it always shows its
  selected tab. The adapter used to report that a selection may be empty for every
  container; the change is for tab lists only. The Axe scans found three defects
  that this phase fixed: a TabItem supported Invoke, a tab list did not report
  IsSelectionRequired, and a tab list's name repeated its control type word (so the
  demo's labels avoid "tab").
- **Panel state (opt-in).** By default a panel is rebuilt when its tab is selected
  again, and its state is not kept. `KeepMounted()` keeps each panel once its tab has
  been selected. A kept panel is hidden with `Display(false)` while other tabs are
  selected, so it is not in the tree, is not focusable and is not in the Tab order.
  Its panel keys are per tab (`<tabs key>/panel:<value>`), so they stay unique. The
  selected panel is named and labelled as before.
- **Collapsible.** The header is a button with the Expanded state, and a click toggles
  it. Enter and Space run the click, as they do for a focusable element with OnClick,
  so both open and close the section. This matches the Phase B semantics, where an
  expandable row's click toggles it. The content is a group named by its title, and
  LabelledBy names it by the header. The content is not built while the section is
  closed, unless the section calls `KeepMounted()`. Then it is built on the first
  opening and hidden while the section is closed, and it is not in the tree then.
- **Expander UIA type (decided).** Microsoft's Expander page says the header is the
  part a user activates, and that the content area is the region it shows
  ([Expander](https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/expander)).
  The WinUI ExpanderAutomationPeer implements IExpandCollapseProvider on the control
  itself ([ExpanderAutomationPeer](https://learn.microsoft.com/en-us/windows/windows-app-sdk/api/winrt/microsoft.ui.xaml.automation.peers.expanderautomationpeer)).
  The Expander page does not name a UIA control type. jamb therefore uses a Button for
  the header, which is what a user activates, with ExpandCollapse, and a Group for the
  content. This is a design decision.
- **Accordion.** Sections are independent by default. `SingleOpen` makes opening one
  section close the others, as Microsoft's guidance on groups of expanders describes
  (the same Expander page: the Expanding event can close the other expanders, and the
  page says to offer it as an option). A section's `OnChange` runs when the accordion
  closes it too.
- **Tree view keys.** The keys follow the WAI-ARIA tree view pattern
  ([Tree View Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/treeview/)): Down and
  Up move to the next and previous visible node without wrapping; Right opens a closed
  node and otherwise moves to its first child; Left closes an open node and otherwise
  moves to the parent; Home and End go to the ends; typing letters selects the next node
  whose label starts with them, and a pause of a second starts a new search; `*` opens
  the closed siblings, which the pattern marks as optional. The pattern's multi-select
  keys are not implemented, since the selection is single. Enter and Space open and
  close a node that has children. The pattern gives Enter the job of activating a node,
  and this tree has no activation of its own, so that is a design decision. The
  Microsoft TreeView page fetched for this phase has no keyboard section, so the keys
  were not checked against a Windows source. Selection follows the arrows; that is also
  a design decision.
- **Tree view focus.** The tree is one focusable element, so Tab reaches it once. Its
  keys act on the selected node, which is also the highlighted row. The access layer
  passes focus to a highlighted row inside a tree, as it already does for a menu and a
  list, so a screen reader hears the selected row, and the focus event moves with the
  selection. The rows are keyed by node key, so a row keeps its identity as the list
  scrolls.
- **Tree view accessibility.** Each row is a TreeItem named by its label, with
  ExpandCollapse, SelectionItem, the Selected and Expanded states, its Level, and its
  position among its siblings (PositionInSet and SizeOfSet). The tree is a Tree with
  the selection-container state. Every tree item has ExpandCollapse, as the TreeItem
  page requires, and a leaf reports the LeafNode state (3). That page says tree items
  "are leaf nodes when they are not expanded or collapsed". The chevron is inert, so
  it is not a second button in the tree.
- **Tree view structure (deviation).** The UIA Tree page describes the content view as
  nested TreeItems. The rows of a virtualized tree are siblings under the Tree, with
  Level. Nesting them would mean building the children of the open parents in the
  access tree, which the virtualization exists to avoid. This is a deviation, and it is
  listed in the README's known limitations.
- **Virtualization.** The tree is a `UniformList` of the visible rows, flattened from
  the open nodes. Only the built rows are in the access tree: the 10,000-node tree
  builds 18 tree items at a time, for a 300 px tree with 26 px rows and the list's
  overscan. Children are asked for only while a node
  is open. A tree that has never been drawn builds only its first row, so an
  assistive technology should scroll the tree into view before it expands an item out
  of view. The live check does this.
- **Framework finding (not fixed).** A `UniformList` returned directly by a component
  that sits under a parent node panics when its row count changes. The list is its own
  instance, and the instance's root is not found among the parent's children. The
  panic is pre-existing: a plain list reproduces it on HEAD. The tree avoids it by
  returning a column that holds the list. The fix belongs to the instance code and was
  not in scope for this phase.
- **Runtime and adapter changes.** `UniformListElement` has two unexported hooks, a
  `decorate` func for the scroll container and an `ownPos` flag for rows that set their
  own positions. `highlightedRow` accepts a tree. `absorbsText` names a tree item by its
  text. `actions` leaves the Click action off a tab. The adapter maps the tree control
  types, reports LeafNode for a tree item with no children, and supports ExpandCollapse
  on every tree item.

### Phase G: verification

- Unit tests in `ui/navigation_test.go` (18 tests): the panels built only while
  selected; arrow, Home and End selection that skips disabled tabs and does not wrap;
  one Tab stop; Ctrl+Tab and Ctrl+Shift+Tab from the tab list and from the panel, with
  focus moving; the tab accessibility tree, the Select action and the absence of Invoke;
  the high contrast label colour, the 225 percent height, and reduced motion; the
  collapsible's click, Space and Enter, its actions and its disabled state; the
  accordion's single-open and independent modes; the tree's keys (Right, Left, Up, Down,
  Home, End, Enter, Space), `*` and type-ahead; levels and positions; the item actions;
  a 10,000-node tree that builds at most 40 items, keeps the selection, and gives focus
  to the selected row; and high contrast, 225 percent and reduced motion for the tree.
- Adapter tests in `internal/platform/windows/uia_tree_test.go` (3 tests): the Tree and TreeItem
  control types with their identifiers, the ExpandCollapse states of tree items
  including LeafNode, and that other roles have no ExpandCollapse state.
- Demo tests in `examples/demo/views/navigation_test.go` (3 tests): the roles and states
  of the Components samples, with no warnings; the large tree builds only its visible
  rows; the project window opens on its Overview tab. The Settings and chat tests were
  updated for the tabs: the shortcuts test clicks the Keyboard shortcuts tab, and the
  chat rig clicks the Comments tab first.
- `scripts/a11y-check.ps1` runs 32 checks, and all pass in the normal, high contrast
  and 225 percent modes. Five are new: a tab list's Tab, TabItem and SelectionItem, with
  Select moving the panel; a collapsible's ExpandCollapse; a tree whose items expand,
  with their level and position in their set (read through the client's raw property
  reader, since the .NET client rejects these property IDs); Down, Right, Left and Up
  moving the focus through the tree by SendKeys, with focus events; and the AxeWindowsCLI
  scan of the navigation section.
- Axe: zero errors for the Settings page, the Components page scrolled to the navigation
  section and to the form controls, and the same page with a select open, in the three
  modes. For the project window, zero errors for both tabs (Overview and Comments) in
  the three modes. Those scans used `AxeWindowsCLI --processid`, which scanned both
  windows of the process.
- Screenshots in `docs/screenshots`: `navigation-layout.png` (dark),
  `navigation-layout-light.png`, `navigation-layout-hc.png` (high contrast),
  `navigation-layout-225.png` (225 percent), `tree-view.png` (both trees),
  `settings-tabs.png`, `project-window-tabs.png` and `project-window-comments.png`.

### Phase G: open items

- **Animated expansion and a sliding tab indicator** are optional and are not
  implemented. Reduced motion has nothing to skip for them yet.
- **Narrator and NVDA** passes for the navigation widgets are pending, as for earlier
  phases, and are for a person to run.
- **Tree multi-select** is not implemented. The open state of a tree is kept by the
  tree, and the app cannot set or read it; `OnChange` reports the selection only.
- **Live level and position reads** go through the .NET client's internal
  `UiaCoreApi.UiaGetPropertyValue`, found by reflection. A Windows or .NET update could
  change it. The Go adapter tests check the values independently.
- **Tab panel state** is not kept across tab changes (see above).

## Phase H: what changed from this design

Phase H adds the feedback widgets: a progress bar, a busy indicator, toasts and
badges. It also fixes two list bugs that the tree work exposed. The decisions and
their sources are below. Where a source states a convention, the entry cites it; the
others are design decisions and are marked as such.

- **Progress bar roles (sourced).** `access.RoleProgressBar` is appended to the role
  enum, so the existing values do not change. It maps to the ProgressBar control type
  (50012). A determinate bar's range is read-only. Microsoft's ProgressBar control type
  page says a progress bar that takes a numeric range must implement the RangeValue
  pattern, that its `IsReadOnly` is TRUE, and that SmallChange and LargeChange are not
  required because progress bars are read-only
  ([ProgressBar control type](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-supportprogressbarcontroltype)).
  The range has no setter, so the bar offers no SetValue action.
- **Indeterminate bars and busy rings (sourced, then decided).** Microsoft's progress
  guidance defines a determinate state, which shows a percentage, and an indeterminate
  state, which shows that work is under way with no known end. It says a progress
  control is read only and not interactive
  ([Guidelines for progress controls](https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/progress-controls)).
  An indeterminate bar therefore exposes no range, since there is no value to read. The
  busy ring is the ProgressRing of that guidance. It is exposed as a ProgressBar with no
  range, which is a design choice. The Microsoft pages reviewed here do not say what a
  WinUI ProgressRing exposes, so that mapping is not verified against Windows' own
  control.
- **Name of the busy indicator (decided).** The API is `ProgressRing`, not `Spinner`,
  because `RoleSpinner` is the number input's spin box (see Phase C). A second meaning
  of "spinner" would be ambiguous. The ring is named "Loading…" unless `Label` gives
  another name.
- **Reduced motion (decided).** With reduced motion on, an indeterminate bar is drawn
  in the middle of its track, and a busy ring is drawn with all its dots still. Neither
  asks for frames.
- **Badges (decided).** A count of zero draws nothing. A badge is a pill with its text,
  and under high contrast it is outlined, so the variant is also told by its text. A
  badge attached to an element with `Badged` adds its text to the element's name, as in
  "Projects, 6 items". The attached badge is hidden from the access tree, so the element
  is read once, with its count. The naming rule is a design decision; no Microsoft page
  was consulted for it. A standalone badge is a group named by its text.
- **Toast keyboard access (sourced).** Microsoft's keyboard guidance says F6 is
  commonly used to move focus between application panes, that Shift+F6 traverses the
  same cycle in reverse, and that "you must implement F6 navigation explicitly in your
  app; it is not provided automatically". Each region should have a name
  ([Keyboard accessibility](https://learn.microsoft.com/en-us/windows/apps/design/accessibility/keyboard-accessibility)).
  The Toaster's region is a focusable group named "Notifications". F6 and Shift+F6
  toggle focus between the region and the element that had it. Tab reaches the actions
  and close buttons inside the toasts. Escape on a focused toast dismisses it, and focus
  moves to the next toast or back to the saved element. Escape in the region returns
  focus.
- **Toasts do not steal focus (sourced in part).** Focus stays where it was when a
  toast appears. The keyboard guidance above asks that focus not move unexpectedly. The
  InfoBar guidance is the natural source for in-app notifications, but the page was not
  retrieved in this phase (the URL tried returned 404), so the rule is also a design
  decision.
- **Announcement politeness (existing mapping).** Each toast is announced through
  `ui.Announce` with the mapping the framework already has: polite for info and success,
  assertive for warning and error. The adapter raises a UIA Notification event for each
  announcement. The event's arguments do not carry the text, so the live check counts the
  events and reads the toast's text from its group.
- **Auto-dismiss (decided).** Info toasts stay 5 seconds, success 4, warning 8, and an
  error stays until it is dismissed. The time does not run while the pointer is over the
  toast or keyboard focus is in it. Each of these can be changed (`Duration`, `Sticky`).
  The pause follows the usual reading of WCAG 2.2.1 (Timing Adjustable); that
  criterion's text was not retrieved in this phase.
- **Maximum stack (decided).** Three toasts show at most. When a new toast would pass
  the limit, the oldest toast that is neither hovered nor holding keyboard focus goes. A
  toast a user is reading or using is kept, and if every toast is in use the stack grows
  for the time being. `Toaster().Max(n)` changes the limit.
- **Badge contrast (measured).** The text of each badge variant is measured against its
  fill, in the pairs `badgePairs` in `ui/contrast_test.go`. The test enforces 4.5:1 for
  dark and light, and `TestBadgeContrastHighContrast` checks both high contrast schemes.
  The results are in the table below. No token failed, so none changed.

  | Variant | Dark | Light | High contrast (black, white) |
  |---|---|---|---|
  | Neutral (Text on SurfaceActive) | 13.55 | 13.96 | 21.00, 21.00 |
  | Accent (AccentText on Accent) | 4.56 | 6.29 | 21.00, 21.00 |
  | Success (dark text on Success) | 8.58 | 5.38 | 21.00, 21.00 |
  | Warning (dark text on Warning) | 9.10 | 5.56 | 21.00, 21.00 |
  | Danger (AccentText on Danger) | 5.01 | 5.15 | 21.00, 21.00 |

  The dark Accent badge is the closest to the target at 4.56:1. A high contrast badge is
  outlined in the text color on Surface.
- **Progress fill contrast (measured).** A determinate bar's fill (Accent by default)
  is a graphical object and needs 3:1 against its track (SurfaceActive; Surface under
  high contrast). `progressPairs` checks it: dark 3.27, light 4.96, high contrast black
  14.37 and white 8.59. An app `Color` is the app's responsibility, as the README says.
  The demo's project colors are a light palette, which falls below 3:1 on the light
  track (1.3 to 2.4). The demo darkens them by 0.35 in the light theme only; every
  palette color then clears 3:1 (darkening only raises the ratio against a light track).
- **List fixes found while testing.** A UniformList or ListOf returned directly from a
  component under a parent panicked when its row count changed. The list rebuilt its own
  instance, and the component that owned the list kept the replaced node, so the next
  rebuild could not find its slot. The owner now follows the node. Separately, a ListOf
  that fits its viewport scrolled to its end when it grew, which hid its first rows: a
  list follows its end only when it overflows or is a Bottom list. Regression tests are
  in `ui/list_test.go`.

### Phase H: verification

- Unit tests in `ui/feedback_test.go` (13 tests): the determinate range and its
  read-only state, the readout and clamping, the indeterminate bar with no range, the
  ring's name and bounds, reduced motion for both (no frames asked), the high-contrast
  outline, badge counts and the 99+ cap, a count of zero drawing nothing, the attached
  badge's name and its hidden node, the dot, and the high-contrast badge.
- Unit tests in `ui/toast_test.go` (16 tests): announcements by variant (politeness),
  the region and toasts named, no focus steal, auto-dismiss timing on a controlled clock,
  pause while hovered and while focused, the sticky error, the max stack, the action that
  runs and dismisses, the close button, F6 in and out, Escape on a toast and in the
  region, the frame request while toasts run, and the icons differing by shape.
- Regression tests in `ui/list_test.go`: `TestListRowCountChangesUnderParent` (four
  shapes: under a parent, as the root, under If, and under Map) and
  `TestListOfRowCountChangesUnderParent`. Each changes the count to 100, 0, 40 and back,
  and the ListOf test also grows a short list from 3 to 100. Both failed before the fix
  (the first with an index-out-of-range panic in `rerender`, the second with a missing
  first row) and pass after it.
- Demo tests in `examples/demo/views/feedback_test.go` (3 tests): Undo removes the new
  project and clears the selection; the bell's unread count and Mark all read; the
  inbox's limit. `access_test.go` now expects the Projects item to read "Projects, 6
  items".
- `scripts/a11y-check.ps1` runs 42 checks, and all pass in the normal, high contrast and
  225 percent modes. Eleven are new: the determinate bar's read-only RangeValue; the
  indeterminate bar and the busy ring with no RangeValue; a toast as a group named by its
  text with a Notification event; an action run from its button; F6 in and out with focus
  events; Escape dismissing a toast; the bell's name with the unread count; and four
  AxeWindowsCLI scans (the Feedback section with a toast visible, the notifications list
  open from the bell, the Projects page, and the Dashboard page), each with zero errors in
  the three modes. The Projects item's name in the earlier checks changed to "Projects,
  6 items".
- Screenshots in `docs/screenshots`: `feedback.png` (dark), `feedback-light.png`,
  `feedback-hc.png` (high contrast), `feedback-225.png` (225 percent), `toasts.png` (the
  stack), `notifications.png` (the bell's list), `projects.png` and `dashboard.png` (the
  changed pages). They were taken with `JAMB_SCRIPT` `shot`.

### Phase H: open items

- **Narrator and NVDA** passes for the feedback widgets are pending, as for earlier
  phases, and are for a person to run. In particular, the busy ring's UIA type and the
  announcement of a toast's text (the Notification event carries no text) need a screen
  reader to confirm.
- **The ProgressRing peer** is mapped to the ProgressBar control type by this design. It
  is not verified against Windows' own control.
- **The busy ring's size** is fixed in pixels (16, 24 and 40), as icon sizes are, and
  does not grow with the Text size. Apps that need it larger pass a larger `Size`.
- **Progress fills** take the theme's accent by default, or an app color through `Color`,
  whose contrast against the track is the app's responsibility (see above).
- **Toast removal** no longer drops a focused or hovered toast; the oldest one that is
  in neither state goes (see Maximum stack).
- **F6 with more regions** is future work. F6 toggles between the toast region and the
  content. A window with more regions needs the full cycle that Microsoft describes.

## Phase I: what changed from this design

Phase I adds `ui.DataGrid`, a virtualized table, with its UI Automation mapping: the `DataGrid` control type, the Grid and Table patterns, the GridItem and TableItem patterns on its cells, and `HeaderItem` headers that sort. The grid's keyboard model and mouse model are in the README (Data grids). This section records the decisions that bind the accessibility side.

### Design

**One Tab stop, one focus.** The body of the grid (a scroll container with the role `DataGrid`) is the only focusable element. The focused cell is a logical position (a row key and a column), not a focused element. The bridge reports the highlighted cell as the focus, as it does for a menu's highlighted item and a tree's selected row, through `highlightedRow`. So a screen reader reads the cell the keys moved to, and the focus event fires when the cell changes. The grid's `OnFocus` and `OnBlur` update the focus ring and the focused cell's style.

**Header inside the body.** The header is an absolute child of the body, after the rows. Its vertical position follows the scroll offset in the paint (`stickList`), and its horizontal position follows the body's scroll, so the header is a child of the grid in the access tree (`data grid` → `header item`s and `list item`s). The rows start below a padding the grid sets to the header's height.

**Roles and patterns.** Mapping, from the SDK's `UIAutomationClient.h` and `UIAutomationCore.idl` (10.0.26100.0):

| Access role | UI Automation control type | Patterns |
| --- | --- | --- |
| `RoleDataGrid` (the body) | `DataGrid` (50028) | Grid (`RowCount` and `ColumnCount` of the data, `GetItem` of a built cell), Table (`GetColumnHeaders`, `RowOrColumnMajor` row-major), Selection (`CanSelectMultiple` from the mode), Scroll |
| `RoleListItem` (a row) | `ListItem` | SelectionItem (`Select`, `AddToSelection`, `RemoveFromSelection` in a multiple selection), ScrollItem |
| `RoleDataItem` (a cell) | `DataItem` (50029) | GridItem (`Row`, `Column`, spans of 1, `ContainingGrid`), TableItem (the column's header item) |
| `RoleHeaderItem` (a header cell) | `HeaderItem` (50035) | Invoke (sorts, when the column is sortable) |

**Sort state.** Microsoft's guidance defines a `DataGrid`'s header and items, and says that `ItemStatus` (UIA_ItemStatusPropertyId, 30026) must be supported when a status changes dynamically: [DataGrid control type](https://learn.microsoft.com/en-gb/windows/win32/winauto/uiauto-supportdatagridcontroltype), [DataItem control type](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-supportdataitemcontroltype), [control patterns overview](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-controlpatternsoverview). Those pages do not define a sort-direction property. The sort state is therefore exposed in two places: the header's name ("Duration, sorted descending"), which every screen reader speaks, and its `ItemStatus` ("sorted descending"). Both are checked by the live script. This is a choice, not a documented requirement; see open items.

**Keyboard model.** Keys follow the WAI-ARIA grid pattern (https://www.w3.org/WAI/ARIA/apg/patterns/grid/): arrows move the cell, Home and End move within a row, Ctrl+Home and Ctrl+End to the first and last cell, PageUp and PageDown by a viewport. Selection follows the Microsoft keyboard-interactions page (https://learn.microsoft.com/en-us/windows/apps/develop/input/keyboard-interactions): Space invokes, Enter activates, Shift+arrow selects continuously, Ctrl+A selects all. The rule that Ctrl+arrow moves the focus without selecting follows the Windows shortcut guides that describe Ctrl+arrow followed by Space to select individual items; no Microsoft page states it for a list view, so it is marked for verification. Keyboard column resizing is Ctrl+Left and Ctrl+Right on a resizable header cell, which no Windows convention defines; Alt is not used, so that menus and host shortcuts keep working.

**Rows that are not built.** A cell is built only when it is in view, and `GetItem` answers only for built cells (it returns null otherwise). A client that needs a row off screen scrolls the grid to it with the Scroll pattern, which the body has, or with ScrollItem on a row, which the rows offer in a scroll container. `RowCount` is the whole data, so the scrollbar and the count agree.

**Selection that follows keys.** Row keys identify the selection and the focus, so a sort or new rows keep both on the same data. An assistive technology's `Select` on a row runs the row's click, which selects the row alone; `AddToSelection` runs the row's Ctrl-click, which adds it, and `RemoveFromSelection` takes it out, in a multiple-selection container only (a single-selection container refuses it, as before).

### Verification

- Unit tests in `ui` (`datagrid_test.go`, `datagrid_style_test.go`, `datagrid_select_test.go`, `datagrid_width_test.go`, `datagrid_density_test.go`, `datagrid_more_test.go`): row virtualization (no more than 40 rows built of 100,000), stable sorting with ties and the cycle through ascending, descending and none, focus and selection kept across a sort, the keyboard model including the header and resizing, Space and Shift+Space, mouse selection with Ctrl and Shift, activation by Enter and double-click, mouse and keyboard resize, the double-click fit, sticky header, horizontal scroll with the header, empty and loading states, the custom empty slot, the access tree (roles, `RowCount`, `ColumnCount`, the cell positions, header sort state and ItemStatus, selection and multi-select state, `AddToSelection` and `RemoveFromSelection`), the style drift test, precedence, `Clear`, high contrast, text size, reduced motion and the variants.
- Platform tests in `internal/platform/windows` (`uiagrid_test.go`): the four IIDs against the SDK strings (`TestGridIIDsMatchTheSDK`), the control types and pattern IDs, and which node has which pattern. The `CanSelectMultiple` callback is checked live, because a unit test cannot call the COM object.
- Live checks in `scripts/a11y-check.ps1`: on the Projects table, the Grid and Table patterns (`RowCount` 6, `ColumnCount` 5, `GetItem(0, 0)` with GridItem and TableItem, `GetColumnHeaders` of 5), a header's Invoke that sorts (the first row becomes Dev Runner) and that gives its name and `ItemStatus`, SelectionItem `Select`, `AddToSelection` and `RemoveFromSelection` with `CanSelectMultiple`, and Ctrl+Home, Down and Right with the cell's row and column read back from the focused element and focus events counted. On the Components data grid: `RowCount` 100,000, `ColumnCount` 6, a sort by Duration with its state, and a two-row multi-selection. Both grids are scanned by AxeWindowsCLI.
- Results (the final script, `-HighContrast`, `-TextScale 225` and normal): 52 of 52 checks pass in each mode in the final runs. The Axe scans of the Projects table and of the Components data grid report zero errors in every mode.
- Benchmarks (`ui`, Windows, on the desktop PC described in BENCHMARKS.md): the first frame of a grid of 100,000 rows, with runtime setup, about 12 ms; a frame after one key press (a rebuild, layout and paint of the rows in view), about 0.2 ms; a sort of 100,000 rows and the frame that shows it, about 8 ms. For comparison, `ListOf` builds 100,000 rows in about 13 ms.

**A finding in an existing check.** The notifications scan of the live script failed in some full runs before the fix below, in both the normal and the 225 percent modes, with three errors: "An on-screen element must not have a null BoundingRectangle property." The three elements are a dismissed warning toast's group, its text and its button, which are still in the notifications group of the scan's snapshot with no properties: they had been removed while the scan read them. The warning toast leaves after eight seconds, and the scan started as it left. The Components page now holds a 100,000-row grid, which makes a scan take longer and widens that window. The unmodified commit passed the 225 percent run twice. The script then waited for the warning toast to leave before it opened the notifications list. That wait hid the race rather than fixing it. The wait is removed, and the cause and the framework changes are in "Removed elements" below; the scan still fails in some runs there.

### Open items

- **Multi-column sort.** Not built. A single `Sort` signal holds one column; Shift+click would need a list of sorts and a way to show their order in the header and in ItemStatus.
- **Frozen first column.** Not built. A frozen column would be a second body that scrolls on one axis only, and its focus and its scroll sync would need their own tests.
- **Off-thread sort.** Not built. The sort of 100,000 rows measures about 8 ms, under the 50 ms threshold at which the brief asks for a worker thread. A sort on a slower machine, or a `Compare` that does real work, would need it.
- **Sort state in UI Automation.** Microsoft's documentation does not define a sort-direction property. The header's name and `ItemStatus` both carry the state. Confirm with an assistive technology (Narrator, NVDA) that the header is announced as sorted, and whether `ItemStatus` should carry it at all.
- **Ctrl+arrow focus.** The rule that Ctrl+arrow moves the focus without selecting is from the Windows shortcut guides, not from a Microsoft list-view page. Check it against the Win32 list-view keyboard documentation or a screen reader, and adjust the table in the README if it differs.
- **Column resize.** Ctrl+Left and Ctrl+Right on a header cell is a choice. The live script does not resize columns; keyboard and mouse resizing are tested in `ui` only.
- **Stale elements in a scan.** Resolved. The framework's answers for a removed node were corrected (see "Removed elements"). The remaining failures came from scanning a screen that changed during the scan; the check now closes every toast before it scans (see "Removed elements").
- **`CanSelectMultiple` in a unit test.** The unit test for the mode checks the flag only. The live script is the test of the callback.
- **Header height.** The grid sets the body's padding to the header's density height. A `Header` style that sets another `MinHeight` moves the header off that padding (documented in `docs/styling.md`).

## Removed elements

This section records a follow-up to Phase I: how the adapter answers a provider whose node has left the tree, and what a walking client sees.

### Cause

- **Answers that did not check the node.** Most entry points resolve their node against the snapshot first (`core.current` in `internal/platform/windows/uia.go`) and return UIA_E_ELEMENTNOTAVAILABLE for a node that is gone. Four did not: `get_FragmentRoot` and `GetEmbeddedFragmentRoots` (`internal/platform/windows/uiacom.go`) returned success for a removed node, and `RemoveFromSelection` returned UIA_E_INVALIDOPERATION. `RangeFromEnclosingElement` (`internal/platform/windows/uiatext.go`, `uiaRangeEnclosingElement`) returned the provider of a removed field.
- **A registry object for an absent node.** `cachedObject` (`internal/platform/windows/uiacom.go`) made an object for any node ID it was given. A call that had read an older snapshot could make an object after the removal had dropped the node's objects; nothing dropped that object again, so the registry kept it until the window closed.
- **No disconnect.** A removed node's provider stayed connected to UI Automation, which asked it again on every call and kept the client's references to it.
- **What a client saw.** A walk reads an element's properties one call at a time. An element can leave the tree between two reads: UI Automation reports the failed reads as empty values, not as exceptions. Here `IsOffscreen` was answered before the removal, so the element read as on-screen with no bounds. In the notifications scan the three such elements were a dismissed warning toast's group, its text and its button, and their snapshot holds no properties.

### Fix

- **Every entry point checks the node.** The methods that did not check now do (`uiacom.go`, `uiatext.go`), and `navigate` takes its target from the same snapshot as its check, so the two agree.
- **No object for an absent node.** `cachedObject` returns 0 for a node that is not in the snapshot. The check and the making run under `regMu`. The runtime publishes a snapshot before it drops the objects of the nodes it removed, and that drop also takes `regMu`, so an object is either made before the drop (and dropped by it) or never made. A caller that gets 0 for a node it read from the snapshot returns UIA_E_ELEMENTNOTAVAILABLE.
- **Disconnect on removal.** `dropObjects` (a removal) and `dropAllObjects` (the window's destruction) call `UiaDisconnectProvider` for each node's provider (`disconnect` in `uiacom.go`). Pattern objects are not disconnected, since they are not elements; they answer from the snapshot as before. The call runs outside `regMu`, since UI Automation may release references into the adapter while it runs.
- **Structure change.** A removal already raises StructureChanged with ChildrenInvalidated on the window (`onAccess`), so a client re-walks. A ChildRemoved on the removed element's parent was not added: the window-level event is enough for a client to drop its cached elements, and the parent's runtime ID is not kept per node.

### Answers for a removed node

Microsoft's guidance on destroyed elements is that their provider resources "should be released by calling the UiaDisconnectProvider" ([server-side providers, Disconnecting Providers](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-serversideprovider)), and a disconnected provider's client requests "receive the UIA_E_ELEMENTNOTAVAILABLE error code" ([UiaDisconnectProvider](https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiadisconnectprovider)). The how-to page named in the task (`uiauto-howto-implement-uia-providers`) returned 404 when fetched, so it is not cited.

| Entry point | Answer for a removed node |
| --- | --- |
| `IRawElementProviderSimple::GetPropertyValue`, any property | UIA_E_ELEMENTNOTAVAILABLE |
| `IRawElementProviderSimple::GetPatternProvider`, any pattern | UIA_E_ELEMENTNOTAVAILABLE |
| `get_ProviderOptions` | S_OK, the provider's constant (not a property of the element) |
| `get_HostRawElementProvider` | S_OK, NULL: a fragment other than the root returns NULL, per the server-side page |
| `IRawElementProviderFragment::Navigate` | UIA_E_ELEMENTNOTAVAILABLE |
| `GetRuntimeId`, `get_BoundingRectangle`, `SetFocus` | UIA_E_ELEMENTNOTAVAILABLE |
| `get_FragmentRoot`, `GetEmbeddedFragmentRoots` | UIA_E_ELEMENTNOTAVAILABLE (was S_OK) |
| Invoke, Toggle, ExpandCollapse (incl. state), SelectionItem (Select, AddToSelection, RemoveFromSelection, IsSelected, SelectionContainer) | UIA_E_ELEMENTNOTAVAILABLE (RemoveFromSelection was UIA_E_INVALIDOPERATION) |
| Selection, Value, RangeValue, Scroll, ScrollItem, Window, Grid, Table, GridItem, TableItem | UIA_E_ELEMENTNOTAVAILABLE |
| Text: DocumentRange, GetSelection, GetVisibleRanges | UIA_E_ELEMENTNOTAVAILABLE |
| Text range: Select, ScrollIntoView, EnclosingElement | UIA_E_ELEMENTNOTAVAILABLE (EnclosingElement was S_OK) |
| Text range reads: GetText, GetBoundingRectangles, Move, Clone, Compare | the range's snapshot, unchanged (the Phase C design: a range is data, and its text and rectangles are the ones the client was told about) |

The SetVisualState and Close methods of IWindowProvider still answer UIA_E_NOTSUPPORTED for any node, as before.

### Verification

- **Reproduction.** `TestRemovedNodeProvidersReturnElementNotAvailable` (`internal/platform/windows/uia_removed_test.go`) obtains every provider of a window with one node of each pattern kind, removes all of the nodes through `Publish` and the adapter's listener, and then calls each entry point above on the providers the client holds. `TestProviderOfAnAbsentNodeIsNotMade` checks the registry rule. Before the change, the reproduction failed on five answers: `get_FragmentRoot` (S_OK), `GetEmbeddedFragmentRoots` (S_OK), `RemoveFromSelection` (UIA_E_INVALIDOPERATION), `RangeFromEnclosingElement` (S_OK), and no disconnect (0 calls for 11 removed nodes); the absent-node test made a provider. After the change both pass, and every other answer in the table was already correct.
- **Unit tests.** `TestEventsWithoutAClientDropObjects` registered a node that was never in a snapshot; the node is now in the tree first, since the registry no longer makes such an object. The zero-allocation guards (`uia_alloc_test.go`) pass; the changed paths add no Go allocation, and a disconnect runs once per removed node.
- **Live script, normal runs** (`scripts/a11y-check.ps1`, the wait removed): 52 of 52 in run 1, 51 of 52 in runs 2, 3 and 4 (the notifications scan reports three errors, "An on-screen element must not have a null BoundingRectangle property"), and 52 of 52 in run 5. The `-HighContrast` run and the `-TextScale 225` run each pass 52 of 52. The notifications scan is therefore not reliable without the wait.
- **Stress.** A client (a temporary script outside the repository) walked the window's raw tree for 60 seconds, reading each element's `IsOffscreen` and `BoundingRectangle`, while the demo showed about 30 toasts, changed the Activity filter 20 times and switched pages 10 times. The client made 406 walks and read 53,490 elements. The app stayed alive, wrote nothing to stderr, and had no panic. No read threw anything other than UIA_E_ELEMENTNOTAVAILABLE. A second run of the same length made 397 walks and read 51,609 elements, with no errors and 37 null-bounds reads. In the first run there were 32 reads where an element answered `IsOffscreen` as on-screen, and its name and control type then read as empty. A re-read of the name also came back empty for each of them, which is what a removed element gives. The stress did not record the order of each removal against the reads, so the per-read timing is not proven here; the deterministic test above covers the answers after a removal.

### Open items

- **Scanning a settled screen.** A client that reads an element's properties across the element's removal sees an empty value for the reads after the removal, and the scanner then reports an on-screen element with no bounds. The framework cannot make a removed element report properties, since the documented answer is UIA_E_ELEMENTNOTAVAILABLE, and a screen reader that meets this re-reads the element or drops it. A rule scan, though, should run on a screen that is not changing: the check now closes every toast before it opens the notifications list and scans, instead of waiting for an auto-dismiss. With that, five consecutive normal runs passed 52 of 52. Whether AxeWindowsCLI should treat an element whose `IsOffscreen` was read before its removal as missing rather than on-screen is a question for its maintainers.
- **Memory.** A second 60-second stress run (397 walks, 51,609 element reads) took the demo's private memory from 158.9 MB to 231.0 MB, about 1.4 KB per element read. That is inside the 1.1 to 1.5 KB per read that Phase A attributes to UI Automation's own client-side state. No control run without the change was made, so the figure is not a before-and-after comparison. The per-call guards hold, and a disconnect is one UI Automation call per removed node.

## Phase J: what changed from this design

Phase J adds to `ui.DataGrid` the interactive cell (`DataColumn.Interactive`), the editable cell (`DataColumn.Edit`, with `DataColumn.Value` for the text an edit starts with), and the sort helpers `ui.SortBy` and `ui.SortByText`. The keyboard and pointer model is in the README (Data grids, "Interactive and editable cells"); this section records the decisions that bind the accessibility side and the sources they follow.

**Sources.**

- The WAI-ARIA grid pattern (https://www.w3.org/WAI/ARIA/apg/patterns/grid/), in its "Editing and Navigating Inside a Cell" section. Enter and F2 place focus on the first widget of a cell that holds widgets. Tab and Shift+Tab move to the next and previous widget, and the pattern allows that movement to wrap inside the cell or within the grid. This grid wraps inside the cell. Escape restores grid navigation, and "may also undo edits", which this grid does by cancelling the edit. The pattern does not name the two states or define what happens at the last widget; the names used in the README are this grid's.
- The Microsoft UI Automation DataItem control type (https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-supportdataitemcontroltype). A data item may host edit controls, and it must support the Value pattern when its primary text is editable. Its name identifies the item. The page gives no guidance on announcing an edit mode.
- The Microsoft control type identifiers (https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-controltype-ids): DataItem is 50029, Edit is 50004.
- The Microsoft keyboard guidance already cited in the README (https://learn.microsoft.com/en-us/windows/apps/develop/input/keyboard-interactions).

**Mapping.**

| Element | Role and name | Patterns and states |
| --- | --- | --- |
| Interactive cell | DataItem, named by `Text` when set, else by its children | Its controls are exposed as children; they are not Tab stops, and the grid's one stop reaches them by Enter or F2 |
| Editable cell, not editing | DataItem, named by `Text` | Value pattern: its value is `Value` (or `Text`); `IsReadOnly` is false; SetValue runs `Edit` on the UI thread. A refused value is announced assertively and changes nothing |
| Read-only cell | DataItem, named by `Text` | No Value pattern (not required: see the DataItem guidance below) |
| Editor of an edit | Text field, named by the column's header | Value pattern (`SetValue`) through its text; its description is the error of the last save, while the save has failed |
| Error of a failed save | Not an element of its own: the editor's description (help text) and an assertive announcement | The invalid frame of the `Editor` style is drawn; a popover under the cell shows the same text for sighted users |
| Edit mode | None announced. Focus moves into the editor, which announces its name and value | The editor is the focus target; no live-region change is made for "editing" |

The Value pattern follows Microsoft's DataItem guidance (uiauto-supportdataitemcontroltype, "Required Control Patterns"): IValueProvider is required when the data item's primary text is editable, and the page's table gives it as "Depends" otherwise. An editable cell therefore exposes it; a read-only cell does not, which the guidance does not require. The adapter offers the pattern on a node whose role has it (a text field, a combo box) or that offers the Set Value action (`hasValueNode` in `internal/platform/windows/uia.go`). The editor of an edit is a text field, so it exposes the pattern too.

**Layout.** A grid cell takes the height of its row, even with no content. A cell that is centred in its row with nothing in it had no height, and its bounding rectangle was null. AxeWindowsCLI reported that as "An on-screen element must not have a null BoundingRectangle property" for the empty Actions cells of a focused grid. The cell is now stretched to the row (`AlignSelf: Stretch` in the default `Cell` style).

**Accessibility decisions, with the reason.**

- The grid stays the one Tab stop. A cell's controls are marked `tabSkip`, which leaves them out of the Tab order and out of the modal's focus walk; the grid's own keys route Tab inside the cell.
- The message of a failed save is the editor's description (set on the access node from the text field, which has no key to resolve). A key that matches no exposed element is recorded as an issue of the element that names it, and the message text is inside a labelled cell, so it is not exposed by itself.
- A pointer press on a control does not select the row, so that a click on a button is not a row click. The cell records that the press was on a control, and the row's click and double-click ignore it.

**Verification.** The unit tests in `ui/datagrid_cells_test.go` cover the keyboard flow (Enter, F2, Tab, Shift+Tab, Escape, arrows inside a field), the pointer, the editing flow (start by Enter, F2, double-click and typing; save; a failed save; cancel; Tab to the next editable cell; the blur save and a failed blur save, with and without window focus; the keyed selection; a row that scrolls away and back), the access tree, the Set Value action (a save, a refused value announced assertively, no action on a read-only cell), the custom editor (F2 focus, its own keys first, Enter, Escape, a blur, a refused save), the popover and the clipping of cells, the `Value` start text and the `SortBy` helpers. `testDataGridEditParts` checks the style parts against the drift test. `internal/platform/windows/uiavalue_test.go` checks which nodes get the Value pattern. The live checks are in `scripts/a11y-check.ps1` (Components and Projects grids, including SetValue on a label cell without an edit, and an invalid SetValue that changes nothing). The Axe scans of the grids (with the editor open, with its error shown, and with focus on a Retry button) reported zero errors.

**Open items.**

- **An editing row that scrolls out of view.** The rows of a grid are one window of a virtual list, which is contiguous, so the row is not laid out outside the window. The edit, its text and its focus are kept, and the editor is laid out again when the row comes back (`TestEditKeepsItsTextWhenItsRowScrollsAway`). Focus therefore stays in an editor that is not on screen, and keys typed meanwhile go to it. Moving focus out of the grid when the row leaves the view would change the keyboard model, so it is not made here; the README asks the user to scroll the row back first. Whether the access tree reports the editor while it is out of view is not checked.
- **Moving focus out of an editing row that scrolls away.** Focus stays in an editor that is not on screen (see the bullet above), so the keys typed meanwhile go to it. A grid could move focus to the cell when its editor leaves the view; that would change the keyboard model, so it is not made here.
- **A custom editor and the Value pattern of its cell.** A custom editor (`DataColumn.Editor`, a Select, a NumberInput or a CheckBox) is the app's control: its accessibility is that of the control it returns. The grid focuses the first focusable control of the editor when it opens, saves a change of focus out of the cell, and gives the failed-save message to the cell as a popover. The grid does not set the description of the custom control, since the control is not the grid's.
- **Focus on a DataItem.** The Microsoft page describes SetFocus on a data item moving focus to its edit control. The grid's `ActionFocus` does not do that yet.

## Phase K: what changed from this design

Phase K adds forms: `ui.Field` (a label above or beside a control, with help text, a required marker and an error), validators on the controls (`Validate`), `ui.NewForm` (the submit flow, with an error summary and an async submit), and the Windows properties that report a form's state. The API and the look are in the README (Forms) and in docs/styling.md. This section records the decisions that bind the accessibility side and the sources they follow.

**Sources.**

- WCAG 2.2 error identification (https://www.w3.org/WAI/WCAG22/Understanding/error-identification.html) and error suggestion (https://www.w3.org/WAI/WCAG22/Understanding/error-suggestion.html): an error is identified in text, not by color alone, and it is described to the user.
- The W3C WAI forms tutorial (https://www.w3.org/WAI/tutorials/forms/): each control has a label, help text is associated with its control, and errors are announced.
- The GOV.UK Design System error summary (https://design-system.service.gov.uk/components/error-summary/) and error message (https://design-system.service.gov.uk/components/error-message/): the summary sits above the form and links to each field, the message sits above its input, and the focus moves to the first field with an error.
- Microsoft's automation element property identifiers (https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-automation-element-propids), checked against UIAutomationClient.h in the Windows SDK 10.0.26100.0: `IsRequiredForForm` is 30025 ("required to be filled out on a form"), `IsDataValidForForm` is 30103 ("valid for the form rule associated with the automation element"; its documented default is FALSE), `HelpText` is 30013, `DescribedBy` is 30105, and `FullDescription` is 30159.

**Decisions.**

- **Timing.** No message appears before the user has changed a field and left it, or before a submit. A field's message updates on each change once it is shown, so it clears as soon as the value is valid. On submit, every field is checked and every message is shown. An error the app sets with `Error` is shown at once.
- **Name, relations and description.** The control's accessible name is the label (`Label`), and `LabelledBy` refers to the label's text. `DescribedBy` refers to the help text and the error, and `HelpText` carries the help text and the error together, since the screen readers that speak a description read `HelpText`. A group of controls (a `Field` with `Check` and no control of its own) is a group named by its label.
- **Required.** The state is `StateRequired`, which the adapter answers as `IsRequiredForForm`. The asterisk is drawn and hidden from the tree, so the state is the only announcement.
- **Invalid.** The state is `StateInvalid` while a message is shown. The adapter answers `IsDataValidForForm` false for an invalid field, true for a required valid one, and leaves it empty for an element that is neither, since that is not a form field.
- **Focus and summary.** On a failed submit, focus moves to the first field with a message, and the form announces the count assertively ("2 fields need attention."). The summary is a group named by its title, and each item is a link that moves focus to its field.
- **Async submit.** While the submit runs, `Submit` does nothing, and the button is disabled. The result reaches the UI thread through `Context.Post`, which requests a frame, so the result is shown without an input event.
- **Enter.** In a single-line field of a form, Enter submits the form. A check box, switch or radio button keeps Enter for the modal's default button, as it did before.
- **Validators on empty input.** `MinLength` and `Pattern` pass the empty text, so `Required` decides whether a field is filled in; an optional field is not reported as too short.

**Verification.**

- Unit tests in `ui` (`form_test.go`, `form_group_test.go`, `form_parts_test.go`): each validator and its message, the first failing validator, the field's wiring (`LabelledBy`, `DescribedBy`, `Description`, `StateRequired`, `StateInvalid`), the required marker outside the tree, the timing rules (no error before the user acts, an error after an edit and a blur, not after a visit alone, an app error at once), the submit (the first invalid field gets focus, the count is announced, the summary's links move focus), the form's message from a submit and from an async submit, the disabled state while the submit runs, Enter in a field against Enter on a check box in a modal, `Reset`, the controls' own checks (select, radio group, check box "must accept"), a group's check and its focus handle, the styles' precedence, `Clear` and high contrast, the shared message look of a number input and a field, and the drift test for every new part.
- Platform tests in `internal/platform/windows` (`uia_forms_test.go`): the two property IDs against the SDK header, and the answers for a required, an invalid, a required-and-invalid and an ordinary element.
- Demo tests in `examples/demo/views` (`components_forms_test.go`): the New Project error and focus, the channels rule of the preferences, and the Forms summary and the server's refusal.
- Live checks in `scripts/a11y-check.ps1`: New Project with an empty name (focus on Name, `IsRequiredForForm` true, `IsDataValidForForm` false, `HelpText` "This field is required.", the announcement by a Notification event, the summary's count, an Axe scan of the errors, and then a valid name submitted and undone); the Forms summary's link moving focus to Name with the same properties, an Axe scan with the summary shown, and the background save with Save disabled and the server's refusal shown. The check "New Project opens a dialog" now asserts that Create is enabled and that no error is shown before Create is pressed.
- Results (final, after the fixes below): `-TextScale 225` passed 66 of 66 in five runs out of five; the normal mode passed 66 of 66 in two runs out of two; `-HighContrast` passed 66 of 66 once. No Axe scan reported an error in any run.
- Earlier, before the fixes below, the 225 percent runs failed on the label editor's keyboard checks. The cause was in the grid, not the timing of the script: see "A grid key handler read a stale cursor" below.
**Fixes made after the first live runs.**

- **A grid key handler read a stale cursor.** The grid took the cursor (row, column and header) from the render, and its key handlers used that copy. Two keys delivered before the next frame, such as Ctrl+Home, Right, Right and F2 sent by a fast keyboard or by SendKeys, all acted on the cell of the last render, so the cursor moved once and F2 opened the editor of the wrong cell (or none). The handlers now read the cursor from its signal when they run (`cursorNow` in `ui/datagrid.go`). Regression tests: `TestArrowKeysBeforeAFrameEachMoveTheCursor` and `TestArrowKeysAtLargeTextScrollTheCellIntoView` (`ui/datagrid_fastkeys_test.go`), which send the keys without frames at 225% text. The failing timeouts were "Label editor to take focus", not a timing problem in the script.
- **A modal reflowed to the window.** A modal's panel is at most the window's height less 24 px, its title and footer do not shrink, and its body scrolls (`ui/modal.go`). A focus the code asks for (`FocusHandle.Focus`, which Submit, the focus to the first invalid field and the summary links use) now scrolls the element into view, as Tab does. Tests: `ui/modal_reflow_test.go`. The 225% screenshot `docs/screenshots/modal-new-project-errors-225.png` shows the footer on screen, with Cancel and Create wrapped to fit the panel.

**Open items.**

- **The manual screen reader pass** (NVDA and Narrator, the script in docs/accessibility-testing.md) is pending for the forms. Check, in particular, whether the help text and the error are read once or twice: `HelpText` and `DescribedBy` both carry them, and a reader that speaks both would repeat them. The choice is kept for now, so that a reader that speaks only one of the two still gets the text.
- **`FullDescription`** (30159) is not answered. The help text and the error are in `HelpText`.
- **Switch and Slider.** They have no `Validate`, and a `Field` around them is a group, not a control. Their labels are not wired as names.
- **The grid editor's popover** keeps its own look (a bordered popover), so the grid and the forms share the Danger text and the alert icon, not the same box.
- **Quiet hours.** The check that the start and the end differ is a demo rule, not a framework rule; a range is always valid for a number input, which clamps its value.

## Phase L: what changed from this design

Phase L adds `ui.Combobox`, an editable text field with a list of suggestions. Its keys, its options and its look are in the README (Combobox) and in docs/styling.md. This section records the decisions that bind the accessibility side and the sources they follow.

**Focus stays in the text, and the highlight is the focused element.** The APG combobox pattern keeps DOM focus in the combobox and conveys the highlighted option through `aria-activedescendant`: "DOM focus stays on the combobox. The focused option is conveyed by setting `aria-activedescendant`" (https://www.w3.org/WAI/ARIA/apg/patterns/combobox/). The framework's equivalent is the access tree's focus target. A menu already passes focus to the row it highlights (`highlightedRow`), and a combo box extends that: when the text has focus and its list highlights a row, the focused node in the tree is that row (`comboRow` in ui/access.go). The text keeps the keyboard, and the UI Automation focus event moves to the highlighted ListItem, which a screen reader announces. This is a change to the tree only; the Windows adapter is unchanged.

**The mapping of the control.** The field is a ComboBox, with ExpandCollapse from its Expanded state, a Value that is its text, and a Name from its label. Its text is an Edit with the Value pattern. The list is a List with its suggestions as ListItems, each SelectionItem, with the highlighted one marked as the focus. These are the mappings the select and the menus already use (`RoleComboBox`, `RoleList`, `RoleListItem`), so the Windows side gained no new control type.

**Announcements.** When the suggestions for a query arrive, their count is announced politely, after 300 ms with no further change: "5 suggestions", "1 suggestion" or "No results" (`countText`). A search that is still running shows a `ProgressRing` labelled "Loading suggestions", which the assistive technology reads as a busy indicator. The count is not announced for a stale result (see SuggestAsync in the README).

**Keys that the assistive technology uses.** Expand and Collapse run the field's own handlers through the access layer's `setExpanded` hook, so they open and close the list without a click. A click in the text opens the list and does not close it, since the press places the caret. Select on a suggestion runs its click, which chooses it.

**Errors and restricted text.** A failed async request shows an error row (`ErrorText`, default "Couldn't load suggestions") and announces it politely at once; the error row replaces the empty slot, since the empty slot would say there is nothing to find. A restricted field that reverts or clears unlisted text says so in the same way, as "Not in the list; cleared" or "Not in the list; reverted to Apple". The same text is the field's description, so the change is read with the field, and the same text is shown under the field in the error's look.

**Scrolling and the rows.** `MaxVisible` sets the height of the list, not the number of rows: every suggestion is a ListItem in the tree, and the keys scroll the highlighted row into view. The rows are not virtualized, as the lists of the framework that take a fixed set of rows are not, so the suggest function should narrow its results to a few dozen.

**Sources.** The keys follow the APG combobox pattern for list autocomplete, with the gaps the APG leaves noted in the README (Tab while the list is open; Enter and Escape when it is hidden). Microsoft's AutoSuggestBox guidance (https://learn.microsoft.com/windows/apps/develop/ui/controls/auto-suggest-box) describes navigating the list with the keys and updating the text from the highlighted item; it does not describe the focus model used here, and this is a difference, not a conflict.

**Verification.** `scripts/a11y-check.ps1` drives the header combobox through UI Automation: it expands on typing, Down reaches the Project Alpha ListItem as the focused element, Enter chooses it and the value follows, and Escape closes the list and then clears the text. An AxeWindowsCLI scan runs with the list open. The script passed 69 of 69 checks in normal mode, with `-HighContrast` and with `-TextScale 225`. In one normal run the Feedback scan reported a null BoundingRectangle on the window's "System" menu item, which belongs to the window chrome and not to the combo box; the next run passed, and the report was not reproduced.

**Open items.**

- **The manual screen reader pass** (NVDA and Narrator) for the combobox is not yet run. Check that the highlighted suggestion is read once as the focus moves, and that the count announcement is not repeated with the suggestion's own name.
- **Narrator's handling** of a focus event on a row of a combo box's list has not been checked on a device.
- **Retry** is not offered on an error. A button in the list would be reached by the mouse only, since the keys stay in the text; editing the text asks again.

## Phase M: what changed from this design

Phase M adds `ui.DatePicker` and `ui.TimePicker`, and the Calendar control type they need for the calendar. Their keys, values and look are in the README (Date and time) and in docs/styling.md. This section records the decisions that bind the accessibility side and the sources they follow.

**The calendar is a Calendar.** The popup has the role `access.RoleCalendar`, which maps to UI Automation's Calendar control type (`UIA_CalendarControlTypeId`, 50001). Microsoft's Calendar requirements say a calendar supports the Grid and Table patterns and has "a DataGrid, with a header, header items for the days of the week, and list items for the days" (https://learn.microsoft.com/en-us/dotnet/framework/ui-automation/ui-automation-support-for-the-calendar-control-type). The framework's grid already gives a RoleDataGrid the Grid and Table patterns, so the calendar reuses that path: the adapter's `supports` and `gridOf` accept a calendar as they accept a data grid. Each day is a data item at a grid position, not a list item. The Microsoft page shows days as list items; the task asked for data items, and the grid cells are the ones the adapter already gives the GridItem pattern, so this is a deliberate difference. The report notes it for review.

**Focus is on the calendar, and the highlighted day is the focus.** A calendar opened from the field takes keyboard focus, as a select's list does: the popup is the focused element, and its highlighted day is the focus target (`highlightedRow`, which now includes the calendar). The screen reader reads the day with its long date, such as "Tuesday, October 13, 2026", and the field keeps its ComboBox role with Expanded. Enter or Space chooses and closes the calendar and returns focus to the field; Escape and Tab close it. These follow the WAI-ARIA date picker dialog pattern (https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/examples/datepicker-dialog/), with two differences noted in the README: Tab closes the calendar rather than moving focus within it, and the navigation buttons are for the pointer.

**The field is a ComboBox.** The text field under the ComboBox is an Edit, and the root is a ComboBox with ExpandCollapse from its Expanded state and a Value that is its text. The time picker is the same shape. Microsoft's ComboBox control type covers an editable field that opens a list, so the framework's combo box mapping serves both. The DatePicker's calendar button is a Button named "Choose date".

**Names.** The calendar and the list of times are named by their field, as a combo box's list is, so the calendar is "Due date" and the list of times is "Start time". A name must not contain the control type: AxeWindowsCLI reports "The Name property must not include the element's control type" for a calendar named "Due date calendar", and the name was changed for that reason. The list of times, and a calendar with no field name, are named "Times" and "Date" when the field has no name.

**Days, names and states.** A day's name is its long date from the locale (on Windows, `GetDateFormatEx`), a day that cannot be chosen is disabled, the chosen day is selected (SelectionItem), and today has the description "Today". The weekday names are header items. The month and year are announced politely when the keys or the navigation buttons change the month (`Announce`, as the combo box announces its count). The "Today" description and the "Choose date" and "Show times" names are English, which is a localization gap.

**The list of times.** The time picker's list is a List with each time a ListItem, the highlighted time is the focus target through the combo box's highlight (as in Phase L), and the time the field holds is selected. Up and Down step the time when the list is closed, so a screen reader user can change the time without opening the list.

**Verification.** The unit tests check the roles, names, the grid size (6 by 7), the selected and disabled states, the description of today, and the month announcement (`datepicker_test.go`, `timepicker_test.go`). The UI Automation mapping is tested in `internal/platform/windows/uia_calendar_test.go`. `scripts/a11y-check.ps1` has checks for the date picker's ComboBox, its calendar taking focus on Alt+Down with Right and Enter, Escape returning focus, the time picker's list, and AxeWindowsCLI scans of the Date and time section and of the calendar open. The keyboard checks passed in none of the live runs of this session: the foreground window was another app (Windows Search), so every focus wait timed out. The Axe scan of the section passed in the normal run. The results are in the report, not here.

**Open items.**

- **The focus-dependent checks** of the date and time pickers, and the rest of the script's focus checks, need a run with the app in the foreground. They are unverified live.
- **The manual screen reader pass** for the calendar (NVDA and Narrator) is not yet run. Check that the day is read once as focus moves, that the month is announced without repeating the day, and that "unavailable" is read for a disabled day.
- **Localization** of the "Today" description and the "Choose date" and "Show times" names is not done.

## Split panes: what changed from this design

`ui.Split` is in place on Windows. Its divider is a focusable element in the access
tree with the Separator role (`access.RoleSeparator`), the name given by `Label`
(by default "Resize panes"), and a Range: the value is the size of the pane the divider
resizes, in that pane's unit (pixels, or percent of the length the panes have), and the
minimum and maximum are the least and the greatest sizes that pane can take at the
divider. The Windows Separator control type has no pattern of its own, and the
RangeValue pattern is the one that carries the value; the Microsoft reference is the
Separator control type page (https://technet.microsoft.com/en-us/ee671643(v=vs.71)).
The keys follow the WAI-ARIA window splitter pattern
(https://www.w3.org/WAI/ARIA/apg/patterns/windowsplitter/), in which Enter toggles
the primary pane and Home and End are optional.

Checked on the platform side: the RangeValue set goes to the same
`ActionSetRangeValue` the sliders use, and a change of the size is a change of the
Range, so it is published as a RangeValue property change. The unit tests read
the access tree and the published updates, and the notes app's test sets the value
through the access bridge.

Not verified: how other frameworks expose a splitter (WPF, WinUI or Chromium), which
the earlier research did not reach. A screen reader pass on the split panes (NVDA and
Narrator) and the live scripted checks in `scripts/a11y-check.ps1` have not been run
for this change. The divider's Range minimum and maximum are updated at the split's
next render, so after a window resize they can lag by a render until the next
interaction; the value is not affected.
