# Navigation widgets

Tabs, collapsible sections, trees, the sidebar, and the other navigation controls.

## Navigation and layout

Tabs, collapsible sections, an accordion and a tree view. Each is bound to
signals or to a node key, as the form controls are, and follows high contrast,
the Text size and reduced motion. Their looks are `ui.CollapsibleStyle` (with an
`Accordion`'s sections) and `ui.TreeStyle`, and a section's header or a tree row can
be drawn with `RenderHeader`, `RenderRow` or `RenderLabel`.

```go
ui.Tabs(section,                                  // *ui.Signal[T], for any comparable T
	ui.Tab[string]{Value: "general", Label: "General", Icon: icons.Settings,
		Panel: func() ui.Element { return generalPanel() }},   // built only while selected (or kept: KeepMounted)
	ui.Tab[string]{Value: "usage", Label: "Usage", Disabled: true, Panel: usagePanel},
).Key("settings-tabs").Label("Settings sections").KeepMounted().OnChange(func(v string) { ... })
ui.Tabs(filter, tabs...).Variant(ui.TabsSegmented)   // one row of segments, the selected one filled
ui.Tabs(view, tabs...).Styles(ui.TabsStyle{Indicator: ui.Style{Height: ui.Px(3)}}) // parts: see Styling
ui.Tabs(count, tabs...).RenderTab(func(tab ui.Tab[int], s ui.PartState) ui.Element { ... }) // your content in each tab
ui.Tabs(view, tabs...).OnReselect(func(v string) { refresh(v) }) // the selected tab clicked again
ui.Tab[string]{Value: "sales", Label: "Sales", Panel: salesPanel,
	Popup: func(close func()) ui.Element { return tabActions(close) }} // anything, in a panel under the tab

ui.Collapsible("Advanced options", open, content).Key("advanced") // *ui.Signal[bool]
ui.Accordion(sectionA, sectionB, sectionC).SingleOpen()          // one section open at a time

ui.Tree(nodes, selected).Key("files").Label("Project files"). // *ui.Signal[string]: the selected node's key
	Style(ui.Style{MaxHeight: ui.Px(320)}).OnChange(func(key string) { ... })
// ui.TreeNode{Key: "src", Label: "src", Icon: icons.Folder,
//	Children: func() []ui.TreeNode { return loadSrc() }}   // nil Children: a leaf
```

- **Tabs.** The tab list is one Tab stop, at the selected tab, as Microsoft's
  TabView guidance describes: Tab enters the strip at the selected tab and Tab
  again goes to the panel. Left and Right select the previous and next enabled
  tab at once (automatic activation, as the design chose; WinUI's TabView moves
  focus on the arrows and selects with Space or Enter). They do not wrap. Home
  and End go to the first and last enabled tab. Space and Enter select the
  focused tab. A disabled tab is shown but skipped, and it is GrayText under
  high contrast. Ctrl+Tab and Ctrl+Shift+Tab select the next and previous
  enabled tab and wrap around, as TabView does
  ([Tab view keyboard navigation](https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/tab-view)).
  They work from the tab list and from the panel. Inside a tab group, Ctrl+Tab
  is therefore bound, and focus moves to the selected tab. Only the selected
  panel is built, so the state of the other panels is discarded when the
  selection moves. `KeepMounted()` keeps them: each panel is built the first time
  its tab is selected, and then stays in the tree, hidden with `Display` while
  other tabs are selected. Its component instances, Local state, text, selection
  and scroll positions survive switching tabs, and switching does not render it.
  It renders again when the code that makes the tabs runs again, and when its own
  state changes. A hidden panel is not in the tree, the Tab order or the hit test.
  The project window keeps its Overview and Comments tabs this way, so a draft
  comment and the scroll position survive. Without `KeepMounted()`, keep the data in
  signals, as the other demo samples do.
  The selected tab has a line under it, in Accent (Highlight under high contrast),
  so the selection is not carried by color alone. No state makes the text bold.
  Tabs take the height their parent gives them, so a panel can fill a window.
  `Variant(ui.TabsSegmented)` draws the tabs as one row of segments instead, with
  the selected segment filled in Accent (Highlight under high contrast). Both
  variants keep the same keys, keyboard behavior and accessible names. The parts
  of a tab list (`List`, `Tab`, `TabSelected`, `TabHover`, `TabDisabled`,
  `TabFocusVisible`, `Indicator`, `Panel`) are set with `Styles` or the theme, and
  `RenderTab` draws each tab's content yourself. A tab's accessible name stays its
  `Label`, so a count the user must hear belongs in the label.
  `OnReselect` runs when the selected tab is activated again, by a click, Enter
  or Space, for an action such as a refresh. A tab's `Popup` builds anything the
  app wants, such as a menu of actions or a rename field, in a floating panel
  under the tab, styled as a Popover. It opens when the selected tab is clicked
  again, and with Down, Shift+F10 or the Menu key while the tab has focus. Focus
  moves into it when it opens; Escape, a click outside or the `close` func passed
  to `Popup` closes it, and Escape and `close` return focus to the tab. While focus
  is in it, the arrow keys and Ctrl+Tab belong to its content and do not switch
  tabs. The tab reports Expanded or Collapsed, Expand and Collapse from assistive
  technology open and close it, and the panel is a dialog named by the tab. A
  change of selection closes it.
- **Collapsible.** The header is a button with the Expanded state. Space and
  Enter open and close it, as a click does, and its chevron points right when closed
  and down when open. The content is not built while the section is closed. With
  `KeepMounted()` it is built on the first opening and then kept hidden while the
  section is closed, so its state survives closing and opening.
  Assistive technology reads the header as expandable and expanded, and the
  content as a group named by the header.
- **Accordion.** Sections open and close on their own. `SingleOpen` makes
  opening one section close the others, as the Expander guidance describes for a
  group ([Expander](https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/expander)).
  A section's `OnChange` runs when it is closed by the accordion too.
- **Tree view.** Only the rows in view are built, so a tree of tens of thousands
  of nodes costs what its visible part costs. A node's children are asked for only
  while it is open. Size the tree with `Style`: a tree with no bounded height
  shows every row. The tree is one focusable element. Its keys act on the
  selected node, which follows the keyboard (single selection). Down and Up move
  to the next and previous visible node, without wrapping. Home and End go to the
  first and last visible node. Right opens a closed node, and on an open node
  moves to its first child. Left closes an open node, and otherwise moves to the
  parent. Enter and Space open and close a node with children. `*` opens the
  closed siblings of the selected node, and typing letters selects the next node
  whose label starts with them. These follow the WAI-ARIA tree view pattern
  ([Tree View Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/treeview/)). A click
  selects a node, and a click on its chevron opens or closes it without selecting.
  A double-click opens or closes it. Keys are kept by node key, so focus and the
  selection stay with their node as the rows scroll.

Breadcrumbs and pagination. A breadcrumb trail shows where the user is, and a
pagination control moves through pages of results. Their looks are
`ui.BreadcrumbsStyle` and `ui.PaginationStyle`, and both follow high contrast and
the Text size.

```go
ui.Breadcrumbs(
	ui.Crumb{Label: "Projects", Icon: icons.Folder, OnClick: showProjects},
	ui.Crumb{Label: "Reports", OnClick: showReports},
	ui.Crumb{Label: "Q3 summary"},                       // the last location is the current one
).Key("path").Label("Folder path").MaxItems(4)          // "..." opens a menu of the hidden locations

ui.Pagination(page, pageCount).Key("results").Label("Result pages"). // *ui.Signal[int], pages from 1
	Siblings(1).OnChange(func(n int) { ... })
ui.Pagination(page, pageCount).Compact()                 // Previous, "Page 3 of 20", Next
```

- **Breadcrumbs.** The trail is a group named by its `Label` (default "Breadcrumb").
  A location with an `OnClick` is a link in the Tab order, and Enter and Space choose
  it, as a click does. A location without one is plain text. The last location is the
  current one: it is plain text, it is selected in the group, and it cannot be chosen.
  The chevrons between locations are hidden from assistive technology. `MaxItems(n)`
  keeps the first location and the last n-1, and puts a "..." button between them that
  opens a menu of the hidden locations. A hidden location without an `OnClick` is
  listed as disabled. A value below 2 counts as 2. `Tooltip(s)` shows s over the trail,
  and is read as the group's description.
- **Pagination.** Each page is a button named "Page n", in the Tab order, and Enter
  and Space choose it. The current page is named "Page n, current page", it is
  selected, and it is filled in Accent (Highlight under high contrast). It is not a
  button. Previous and Next are named "Previous page" and "Next page", and they are
  disabled at the ends. The first and the last page are always shown, with `Siblings(n)`
  pages on each side of the current page (default 1) and a "..." where pages are
  skipped. Up to seven pages are shown in full. The page is held between 1 and the
  page count when it is drawn. `Compact()` shows only Previous, "Page 3 of 20" and
  Next. The control is a group named by its `Label` (default "Pagination"). Left and
  Right do not move between pages. `Tooltip(s)` shows s over the control, and is read
  as the group's description.

## Sidebar

`ui.Sidebar(selected, items...)` is a vertical navigation pane, as the left pane of
WinUI's NavigationView. Its look is `ui.SidebarStyle`, set on the theme as
`Theme.Sidebar` or with `Styles`, and it follows high contrast, the Text size and
reduced motion.

```go
ui.Sidebar(page, // *ui.Signal[string]: the selected item's key
	ui.SidebarHeading("Mail"),
	ui.SidebarItem{Key: "inbox", Label: "Inbox", Icon: icons.Inbox, Badge: 4},
	ui.SidebarItem{Key: "sent", Label: "Sent", Icon: icons.Home, Disabled: true},
	ui.SidebarHeading("Tags"),
	ui.SidebarItem{Key: "archive", Label: "Archive", Icon: icons.Search},
).Key("main-nav").Label("Main navigation").
	Collapsed(collapsed).Toggle(). // *ui.Signal[bool]: icons only, with a toggle button at the top
	Header(header).Footer(account). // above the items, and pinned at the bottom
	Width(ui.Px(260)).              // the expanded width; 240 px at the Text size of 100 percent by default
	OnChange(func(key string) { ... })
```

- **Selection.** A click on an item, or Enter or Space on it, selects it and runs
  `OnChange`. Up and Down move the focus to the previous and next enabled item,
  without wrapping, and skip the headings and the disabled items. Home and End go
  to the first and last enabled item. Focus follows the keys, but the selection
  does not change until Enter, Space or a click, as NavigationView does.
- **Tab stop.** The pane is one Tab stop, at the selected item, or at the first
  enabled item when the selected one is not among them.
- **Collapsed.** While the `Collapsed` signal is true, the pane is 48 px wide and
  shows only the icons. Each item shows its label as a tooltip, and each heading is
  a thin rule. Give every item an icon, since an item with none shows nothing. The
  header and the footer are drawn as they are.
- **Accessibility.** The pane is a group named by `Label` (by default
  "Navigation"). Its items are list items that report the selected and disabled
  states, and an item with a badge is named with its count, as in "Inbox, 4". Its
  headings are headings. The toggle is a button named "Collapse navigation" or
  "Expand navigation", which reports the expanded state. `Tooltip(s)` shows s over
  the pane and is read as its description; an item of a collapsed pane shows its own
  label instead.
- **Layout.** The pane takes the height of its row, so put it in a row beside the
  content. Its items scroll when they do not fit, and the header and the footer stay
  in place.
