# Accessibility

This page is the guide for app authors. The design and the decision log behind it, split by phase, are in [accessibility-design.md](accessibility-design.md).

On Windows, jamb exposes its UI to screen readers and other assistive
technology through UI Automation. The design is in
[accessibility-design.md](accessibility-design.md). Nothing is built until an
assistive technology asks for the window's UI, so an app run without one pays
nothing extra. The exception is a development run with warnings on (see below),
which builds the tree to check it.

What is automatic, with no app code:

- Buttons, including icon-only buttons, are named by their text. A clickable
  element with no role is a button too.
- Text fields and text areas have the role. Give them a name with `Label`.
- Plain text is the name of its container, or a static text node when it stands
  alone. Decorative icons and layout containers with no role or name are left
  out, so their children sit in the parent.
- Menus, menu bars and menu items carry their roles, checked and disabled
  states, and nesting level. A `Modal` is a dialog named by its title.
- A `UniformList` or `ListOf` exposes its built rows as list items, each with
  its position in the full list ("40 of 10,000"). Only the rows that are built
  are in the tree.
- Tooltips are nodes under the element they hang from, so an overlay reads in
  place rather than at the end of the window.

The controls, with no app code, once their roles and states are set:

- A check box, switch or radio button has its check state. A `Checked` element
  is a toggle, so Toggle works, and a `ui.IconButton(...).AccessChecked(on)` is a
  toggle button with its state.
- A `DropdownMenu` trigger, a menu item with a submenu, and a menu bar label
  expand and collapse. A menu of radio items is a selection group.
- A `Tabs` group is a Tab control: its tabs are tab items with SelectionItem, and
  its panel is a pane named by its tab. Tabs have no Invoke pattern, since they are
  selected. A `Collapsible` header is a button that expands and collapses, and its
  content is a group named by the header. A `Tree` is a tree whose items expand
  and collapse, with their level and position in their set. Every tree item has
  ExpandCollapse, and a leaf reports that it has no children. Only the rows in view
  are in the tree.
- A `TextInput` or `TextArea` has its text as its value, which assistive
  technology can read and set. It also has the UI Automation Text pattern: a
  screen reader reads and moves by character, word, line and paragraph, reads
  the selection and the caret, selects and finds text, and gets the bounds of
  each visual line. Text and selection changes raise events. The system caret
  follows the insertion point, so screen magnifiers track typing. The points
  and the word rules are described in
  [accessibility design](accessibility-design-phases-b-c.md#phase-c-what-changed-from-this-design).
- A scroll container reports its position and its visible part, and can be
  scrolled and scrolled to a percent. A row or control inside one can be
  scrolled into view.
- A `ProgressBar` is a progress bar whose range is read-only: assistive technology
  reads its value and cannot set it. An indeterminate bar and a `ProgressRing` are
  progress bars with no range. The Toaster's region is a group named "Notifications",
  and each toast is a group named by its text. A badge attached to an element is
  part of that element's name (see Feedback above).

The API, on the elements that already exist:

```go
ui.Div().Role(access.RoleCheckBox).Label("Notifications").AccessChecked(on)
ui.Div().AccessExpanded(open)                    // a row that opens and closes; its click toggles it
ui.Div().Role(access.RoleButton).AccessSelected(on).OnClick(choose) // one item of a selection
ui.Div().Label("Status").SelectionContainer() // the group that holds the items
ui.Div().Heading(2).Child(ui.Text("Appearance"))
ui.Text("Name").Key("name-label")          // a label, named by key
ui.TextInput(name).Label("Name").LabelledBy("name-label").DescribedBy("name-help")
ui.ListOf(items, key, row).SelectionContainer() // also UniformList
ui.Div().Description("Saves the file to disk")
ui.Div().AccessHidden() // leaves the element and its children out of the tree
ui.Div().Display(false) // hidden: not drawn, not focusable, out of the tree, state kept (see the API guide)
ui.IconButton(icons.Bell).Label("Notifications")
ui.TextInput(query).Label("Search projects") // also TextArea
ui.Image(avatar).Alt("Account avatar")
ui.Announce("Saved", access.Polite) // spoken as a notification; Assertive interrupts
ui.CheckboxTri("All", all)            // a check box in the mixed state is set by its CheckState
ui.Slider(v, 0, 100, 5).Label("Volume") // a slider: RangeValue, with the Range of the value
ui.ProgressBar(0.45).Label("Upload")    // a progress bar: read-only RangeValue
ui.ProgressRing().Label("Sending reply") // a busy indicator: no range, named "Loading…" by default
ui.Div().Label("Projects").Badged(ui.Badge(6).Noun("items")) // named "Projects, 6 items"
ui.Div().Role(access.RoleDataGrid).AccessGrid(3, 2)     // a table of 3 rows of data (the header row not counted) and 2 columns
ui.Div().Role(access.RoleHeaderItem).AccessCell(0, 2)   // the header of column 2
ui.Div().Role(access.RoleDataItem).AccessCell(1, 2)     // the cell in row 1, column 2
ui.Div().Role(access.RoleListItem).AccessPosition(2, 5) // "2 of 5"
ui.Div().Role(access.RoleListItem).AccessLevel(2)       // an item of a list nested in another
```

`AccessGrid`, `AccessCell`, `AccessPosition` and `AccessLevel` describe a table or a
set built from plain elements, as `DataGrid` and `Tree` describe theirs. A data grid
with `AccessGrid` has the Grid and Table patterns, so a screen reader moves through it
by row and column (NVDA's Ctrl+Alt+arrows, Narrator's table keys) and reads each
cell's column header. Its cells are the data items under it with `AccessCell`, at any
depth (a row element with no role of its own is left out of the tree), and its
column headers are the header items under it. Rows and columns count from 1, and the
header row is not one of the rows: a header item takes row 0. A cell's name is its
text, and the links in it stay links of their own. `AccessGrid` applies to a data
grid or a calendar and `AccessCell` to a data item or a header item; on another role
they are not reported, and the development warnings say so, as they do for a data
item in row 0. `AccessPosition` and `AccessLevel` apply to an item of any role. All
four panic on numbers that describe no place, such as a position past the size.

`Expanded` and `Selected` declare the state; the element's click handler makes
the change, so a click on an expandable row must toggle it. Expand and select
run that handler only when the state is not already the one asked for.
`SelectionContainer` marks the group that holds the selected items, and a tab
list, or a menu with radio items, is one without a call. `Heading` takes a level
from 1 to 6. `LabelledBy` and `DescribedBy` name an element by the `Key` of
other elements, such as a visible label above a field.

Announcements are spoken as notifications. A polite one waits its turn, in
order, after the speech in progress; an assertive one is spoken as soon as
possible, and a newer assertive one replaces it. Announce
the result of an action or a status change that has no focus to carry it, such
as "Saved" or "3 results", and keep repeated messages to one per change: a burst
of keystrokes should be announced once it pauses.

A name comes from the first of these that is set: the `Label`, then the text
inside a control, then its tooltip. `Description` (or the tooltip, when it is
not the name) is the help text read after the name. Set `Label` where an
element has no visible text, and on every text field, since a field has no
name of its own. The placeholder is not used as a name.

`access.Role` and the other names in `access` (states, actions, the tree and
its updates) are the platform-neutral model; `access/accesstest` has helpers
that find a node by its role and name in a unit test.

Development warnings report what a user cannot identify: an interactive element
with no accessible name, an image without `Alt`, a check box, switch or radio
button with no `Checked` call, a selection item outside a selection container,
an `AccessGrid` or `AccessCell` on a role it does not apply to, and a `LabelledBy`
or `DescribedBy` key that matches no element. Each element is
reported once, to stderr, in runs with `JAMB_TRACE=1`, under `jamb dev`, or with
`JAMB_A11Y_WARN=1`.

To see the tree as text, use the `a11ydump PATH` script step, or call
`rt.AccessTree().Dump()` in a test. Each line is a role, its name and states,
its bounds in physical pixels, and its ID.

The automated checks, the Accessibility Insights scans and the live UI Automation
check are in [Tools](tools.md#tools-for-humans-and-coding-agents). The manual pass with
Narrator and NVDA, with the expected results for each task in the demo and the
notes app, is in [docs/accessibility-testing.md](accessibility-testing.md).
