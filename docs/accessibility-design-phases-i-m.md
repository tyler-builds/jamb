# Accessibility design: phases I to M

The decisions of phases I to M, and the split panes. The goals, the architecture and the phase index are in [accessibility-design.md](accessibility-design.md).

## Phase I: what changed from this design

Phase I adds `ui.DataGrid`, a virtualized table, with its UI Automation mapping: the `DataGrid` control type, the Grid and Table patterns, the GridItem and TableItem patterns on its cells, and `HeaderItem` headers that sort. The grid's keyboard model and mouse model are in [widgets-data.md](widgets-data.md#data-grids). This section records the decisions that bind the accessibility side.

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
- **Ctrl+arrow focus.** The rule that Ctrl+arrow moves the focus without selecting is from the Windows shortcut guides, not from a Microsoft list-view page. Check it against the Win32 list-view keyboard documentation or a screen reader, and adjust the table in [widgets-data.md](widgets-data.md#data-grids) if it differs.
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

Phase J adds to `ui.DataGrid` the interactive cell (`DataColumn.Interactive`), the editable cell (`DataColumn.Edit`, with `DataColumn.Value` for the text an edit starts with), and the sort helpers `ui.SortBy` and `ui.SortByText`. The keyboard and pointer model is in [widgets-data.md](widgets-data.md#data-grids) ("Interactive and editable cells"); this section records the decisions that bind the accessibility side and the sources they follow.

**Sources.**

- The WAI-ARIA grid pattern (https://www.w3.org/WAI/ARIA/apg/patterns/grid/), in its "Editing and Navigating Inside a Cell" section. Enter and F2 place focus on the first widget of a cell that holds widgets. Tab and Shift+Tab move to the next and previous widget, and the pattern allows that movement to wrap inside the cell or within the grid. This grid wraps inside the cell. Escape restores grid navigation, and "may also undo edits", which this grid does by cancelling the edit. The pattern does not name the two states or define what happens at the last widget; the names used in [widgets-data.md](widgets-data.md#data-grids) are this grid's.
- The Microsoft UI Automation DataItem control type (https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-supportdataitemcontroltype). A data item may host edit controls, and it must support the Value pattern when its primary text is editable. Its name identifies the item. The page gives no guidance on announcing an edit mode.
- The Microsoft control type identifiers (https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-controltype-ids): DataItem is 50029, Edit is 50004.
- The Microsoft keyboard guidance already cited in [widgets-data.md](widgets-data.md#data-grids) (https://learn.microsoft.com/en-us/windows/apps/develop/input/keyboard-interactions).

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

- **An editing row that scrolls out of view.** The rows of a grid are one window of a virtual list, which is contiguous, so the row is not laid out outside the window. The edit, its text and its focus are kept, and the editor is laid out again when the row comes back (`TestEditKeepsItsTextWhenItsRowScrollsAway`). Focus therefore stays in an editor that is not on screen, and keys typed meanwhile go to it. Moving focus out of the grid when the row leaves the view would change the keyboard model, so it is not made here; the [data grid section](widgets-data.md#data-grids) asks the user to scroll the row back first. Whether the access tree reports the editor while it is out of view is not checked.
- **Moving focus out of an editing row that scrolls away.** Focus stays in an editor that is not on screen (see the bullet above), so the keys typed meanwhile go to it. A grid could move focus to the cell when its editor leaves the view; that would change the keyboard model, so it is not made here.
- **A custom editor and the Value pattern of its cell.** A custom editor (`DataColumn.Editor`, a Select, a NumberInput or a CheckBox) is the app's control: its accessibility is that of the control it returns. The grid focuses the first focusable control of the editor when it opens, saves a change of focus out of the cell, and gives the failed-save message to the cell as a popover. The grid does not set the description of the custom control, since the control is not the grid's.
- **Focus on a DataItem.** The Microsoft page describes SetFocus on a data item moving focus to its edit control. The grid's `ActionFocus` does not do that yet.

## Phase K: what changed from this design

Phase K adds forms: `ui.Field` (a label above or beside a control, with help text, a required marker and an error), validators on the controls (`Validate`), `ui.NewForm` (the submit flow, with an error summary and an async submit), and the Windows properties that report a form's state. The API and the look are in [widgets-forms.md](widgets-forms.md#forms) and in docs/styling.md. This section records the decisions that bind the accessibility side and the sources they follow.

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

Phase L adds `ui.Combobox`, an editable text field with a list of suggestions. Its keys, its options and its look are in [widgets-selection.md](widgets-selection.md#combobox) and in docs/styling.md. This section records the decisions that bind the accessibility side and the sources they follow.

**Focus stays in the text, and the highlight is the focused element.** The APG combobox pattern keeps DOM focus in the combobox and conveys the highlighted option through `aria-activedescendant`: "DOM focus stays on the combobox. The focused option is conveyed by setting `aria-activedescendant`" (https://www.w3.org/WAI/ARIA/apg/patterns/combobox/). The framework's equivalent is the access tree's focus target. A menu already passes focus to the row it highlights (`highlightedRow`), and a combo box extends that: when the text has focus and its list highlights a row, the focused node in the tree is that row (`comboRow` in ui/access.go). The text keeps the keyboard, and the UI Automation focus event moves to the highlighted ListItem, which a screen reader announces. This is a change to the tree only; the Windows adapter is unchanged.

**The mapping of the control.** The field is a ComboBox, with ExpandCollapse from its Expanded state, a Value that is its text, and a Name from its label. Its text is an Edit with the Value pattern. The list is a List with its suggestions as ListItems, each SelectionItem, with the highlighted one marked as the focus. These are the mappings the select and the menus already use (`RoleComboBox`, `RoleList`, `RoleListItem`), so the Windows side gained no new control type.

**Announcements.** When the suggestions for a query arrive, their count is announced politely, after 300 ms with no further change: "5 suggestions", "1 suggestion" or "No results" (`countText`). A search that is still running shows a `ProgressRing` labelled "Loading suggestions", which the assistive technology reads as a busy indicator. The count is not announced for a stale result (see SuggestAsync in [widgets-selection.md](widgets-selection.md#combobox)).

**Keys that the assistive technology uses.** Expand and Collapse run the field's own handlers through the access layer's `setExpanded` hook, so they open and close the list without a click. A click in the text opens the list and does not close it, since the press places the caret. Select on a suggestion runs its click, which chooses it.

**Errors and restricted text.** A failed async request shows an error row (`ErrorText`, default "Couldn't load suggestions") and announces it politely at once; the error row replaces the empty slot, since the empty slot would say there is nothing to find. A restricted field that reverts or clears unlisted text says so in the same way, as "Not in the list; cleared" or "Not in the list; reverted to Apple". The same text is the field's description, so the change is read with the field, and the same text is shown under the field in the error's look.

**Scrolling and the rows.** `MaxVisible` sets the height of the list, not the number of rows: every suggestion is a ListItem in the tree, and the keys scroll the highlighted row into view. The rows are not virtualized, as the lists of the framework that take a fixed set of rows are not, so the suggest function should narrow its results to a few dozen.

**Sources.** The keys follow the APG combobox pattern for list autocomplete, with the gaps the APG leaves noted in [widgets-selection.md](widgets-selection.md#combobox) (Tab while the list is open; Enter and Escape when it is hidden). Microsoft's AutoSuggestBox guidance (https://learn.microsoft.com/windows/apps/develop/ui/controls/auto-suggest-box) describes navigating the list with the keys and updating the text from the highlighted item; it does not describe the focus model used here, and this is a difference, not a conflict.

**Verification.** `scripts/a11y-check.ps1` drives the header combobox through UI Automation: it expands on typing, Down reaches the Project Alpha ListItem as the focused element, Enter chooses it and the value follows, and Escape closes the list and then clears the text. An AxeWindowsCLI scan runs with the list open. The script passed 69 of 69 checks in normal mode, with `-HighContrast` and with `-TextScale 225`. In one normal run the Feedback scan reported a null BoundingRectangle on the window's "System" menu item, which belongs to the window chrome and not to the combo box; the next run passed, and the report was not reproduced.

**Open items.**

- **The manual screen reader pass** (NVDA and Narrator) for the combobox is not yet run. Check that the highlighted suggestion is read once as the focus moves, and that the count announcement is not repeated with the suggestion's own name.
- **Narrator's handling** of a focus event on a row of a combo box's list has not been checked on a device.
- **Retry** is not offered on an error. A button in the list would be reached by the mouse only, since the keys stay in the text; editing the text asks again.

## Phase M: what changed from this design

Phase M adds `ui.DatePicker` and `ui.TimePicker`, and the Calendar control type they need for the calendar. Their keys, values and look are in [widgets-selection.md](widgets-selection.md#date-and-time) and in docs/styling.md. This section records the decisions that bind the accessibility side and the sources they follow.

**The calendar is a Calendar.** The popup has the role `access.RoleCalendar`, which maps to UI Automation's Calendar control type (`UIA_CalendarControlTypeId`, 50001). Microsoft's Calendar requirements say a calendar supports the Grid and Table patterns and has "a DataGrid, with a header, header items for the days of the week, and list items for the days" (https://learn.microsoft.com/en-us/dotnet/framework/ui-automation/ui-automation-support-for-the-calendar-control-type). The framework's grid already gives a RoleDataGrid the Grid and Table patterns, so the calendar reuses that path: the adapter's `supports` and `gridOf` accept a calendar as they accept a data grid. Each day is a data item at a grid position, not a list item. The Microsoft page shows days as list items; the task asked for data items, and the grid cells are the ones the adapter already gives the GridItem pattern, so this is a deliberate difference. The report notes it for review.

**Focus is on the calendar, and the highlighted day is the focus.** A calendar opened from the field takes keyboard focus, as a select's list does: the popup is the focused element, and its highlighted day is the focus target (`highlightedRow`, which now includes the calendar). The screen reader reads the day with its long date, such as "Tuesday, October 13, 2026", and the field keeps its ComboBox role with Expanded. Enter or Space chooses and closes the calendar and returns focus to the field; Escape and Tab close it. These follow the WAI-ARIA date picker dialog pattern (https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/examples/datepicker-dialog/), with two differences noted in [widgets-selection.md](widgets-selection.md#date-and-time): Tab closes the calendar rather than moving focus within it, and the navigation buttons are for the pointer.

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

## Status bars and toolbars: what changed from this design

`ui.StatusBar` and `ui.Toolbar` add two roles. `access.RoleStatusBar` maps to UI
Automation's StatusBar control type (`UIA_StatusBarControlTypeId`, 50017), and
`access.RoleToolBar` to its ToolBar control type (`UIA_ToolBarControlTypeId`, 50021). No
existing role fitted: a group would hide what each bar is, and these are the control
types UI Automation defines for them. Neither bar has a pattern of its own; their items
are their children, each with its own role, and a toolbar's separators are separators.
Microsoft's pages for the two control types
(https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-supportstatusbarcontroltype
and https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-supporttoolbarcontroltype)
were not re-read for this change; the optional Grid pattern of a status bar of cells is
not implemented.

**Names.** A status bar has no name by default, and `Label` gives it one. A name such as
"Status bar" would repeat the control type, which AxeWindowsCLI reports (see Phase M). A
toolbar is named by the label it is built with, such as "Formatting".

**A status bar is not a live region.** A change to its text is not announced. An app that
changes it after an action announces the change with `Announce`, as for an alert.

**Toolbar keys.** The toolbar follows the WAI-ARIA toolbar pattern
(https://www.w3.org/WAI/ARIA/apg/patterns/toolbar/): one Tab stop, Left and Right between
its controls with wrapping, Home and End to the ends, and Tab back to the control that
had focus last. The pattern's guidance that a control which needs the arrow keys keeps
them is followed by dispatch order: a text field's caret keys are bindings, which run
before the toolbar's key handler, and a slider or a toggle group stops the keys it uses.
The remembered control is the last one that had focus by any means, including a click.
The runtime keeps it on the toolbar's element (`rovingGroup`); the Tab order lists only
that control, or the first one, so a screen reader's Tab navigation matches the keys.

**Verification.** The unit tests check the roles, names and children of both bars
(`ui/statusbar_test.go`, `ui/toolbar_test.go`), the Tab stop, the arrow keys, the
remembered control and a text field that keeps its keys, and the control type mapping is
in `internal/platform/windows/uia_test.go`. A screen reader pass and the live scripted
checks in `scripts/a11y-check.ps1` have not been run for this change.
