# Accessibility design: phases F to H

The decisions of phases F, G and H. The goals, the architecture and the phase index are in [accessibility-design.md](accessibility-design.md).

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
- **NumberInput validation.** Chosen and documented in [widgets-forms.md](widgets-forms.md): a number outside
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
  listed in [the known limitations](status.md#known-limitations-and-next-steps).
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
  14.37 and white 8.59. An app `Color` is the app's responsibility, as [widgets-feedback.md](widgets-feedback.md#feedback) says.
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
