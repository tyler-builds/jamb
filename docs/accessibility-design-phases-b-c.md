# Accessibility design: phases B and C

The decisions of phases B and C. The goals, the architecture and the phase index are in [accessibility-design.md](accessibility-design.md).

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
