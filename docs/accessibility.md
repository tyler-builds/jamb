# Accessibility design

Status: accepted. Phases A (Foundation) and B (Controls) are done, with the
changes listed under "Phase A: what changed from this design" and "Phase B: what
changed from this design". Phases C to E have not started. This document
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
  tooltip, scroll area, heading, tab list, tab, tab panel.
- **Actions**: click (invoke), focus, expand, collapse, toggle, select, set
  value, scroll into view, increment, decrement.
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

- `Div().Role(access.RoleCheckBox).Label("Notifications").Checked(on)`
- `Div().Description("…")`, `Div().Hidden()` (leave out of the tree),
  `Div().Heading(2)`
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
| D. System settings | Windows high contrast, the Text size setting, reduced animations, a contrast audit of the built-in themes, caret tracking for magnifiers |
| E. Verification | Accessibility Insights for Windows checks on the examples, a manual NVDA and Narrator script, automated tests that read the real UI Automation tree |

## Testing

- Unit tests on the access tree built from ui elements.
- Automated checks that read the live UI Automation tree of the examples the
  way a screen reader does.
- [Accessibility Insights for Windows](https://accessibilityinsights.io/)
  (free, from Microsoft) for automated rule checks.
- Manual passes with NVDA (free) and Narrator (built into Windows).

## Phase A: what changed from this design

Phase A is in place on Windows (`access`, `ui/access.go`,
`platform/windows/uia*.go`). These are the places where the work differed from
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
  disabled, as UI Automation expects.
- **Expand, select and toggle run on the UI thread.** An expand, collapse or
  select that asks for the state the element already has does nothing. Otherwise
  the element's click handler runs, so an expandable element's handler must
  change its own state, as a click does. A submenu item is the exception: its
  expansion is not a click, so it opens and closes its submenu directly. Toggle
  always runs the click.
- **Selection is declared by the app.** `Selected(on)` gives an item its state,
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

### Phase B: open items

- **Window events are not delivered in the live check.** Window opened and closed
  are raised, for the dialog and for the window's root, and the dialog has the
  Window pattern. A .NET client that subscribed to them, on the window or at the
  desktop root, received neither, while the menu events, raised on the same root,
  were received. The cause is not isolated. Until it is, a dialog is announced by
  its focus, its IsDialog and its IsModal properties.
- **An action requested through UI Automation may not run while a context menu
  opened from the keyboard is open.** The live check was: focus a project card,
  press Shift+F10, then call Expand on the menu's Status item. The request was
  posted to the UI thread, but no frame ran it, while key presses and dropdown
  menus were not affected. The cause is not isolated. The likely place to look
  is the frame request and wake path (`ui.Runtime.requestFrame` and
  `platform/windows/window.go`), not the access tree.
- ItemContainer and VirtualizedItem, as above. A client cannot find an unbuilt
  row.
- A context menu's own items are not exposed as a dropdown: a context menu opens
  from the keyboard or a pointer press on the element it hangs from, and that
  element has no expand state.
- The mixed (indeterminate) state is in the model and the adapter, but no public
  API sets it.
- RangeValue is not implemented: no slider exists in the framework yet.
- The Text pattern for text fields and text areas is Phase C. Until then, a
  text area exposes its Value only, as a text field does.
- Screen reader checks (NVDA, Narrator, Accessibility Insights) are Phase E.
