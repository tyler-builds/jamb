# Navigation widgets

Tabs, collapsible sections, trees, the sidebar, the other navigation controls, group boxes,
description lists, status bars and toolbars.

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
ui.Tabs(view, tabs...).Reorderable().OnReorder(func(order []string) { save(order) }) // the user moves tabs
ui.Tabs(doc, tabs...).OverflowMenu().TabMaxWidth(ui.Px(180)). // a menu of every tab; long labels cut short
	Suffix(ui.IconButton(icons.Plus).Label("New tab").OnClick(newTab)) // beside the strip, not scrolled
ui.Tab[string]{Value: "sales", Label: "Sales", Panel: salesPanel,
	Popup: func(close func()) ui.Element { return tabActions(close) }} // anything, in a panel under the tab

ui.Collapsible("Advanced options", open, content).Key("advanced") // *ui.Signal[bool]
ui.Accordion(sectionA, sectionB, sectionC).SingleOpen()          // one section open at a time

ui.Tree(nodes, selected).Key("files").Label("Project files"). // *ui.Signal[string]: the selected node's key
	Style(ui.Style{MaxHeight: ui.Px(320)}).OnChange(func(key string) { ... })
// ui.TreeNode{Key: "src", Label: "src", Icon: icons.Folder,
//	Children: func() []ui.TreeNode { return loadSrc() }}   // nil Children: a leaf
th := ui.NewTreeHandle()                                    // code opens, closes, reveals and selects nodes
ui.Tree(nodes, selected).Key("files").Handle(th).Expanded(openKeys). // *ui.Signal[[]string]: the open keys
	OnExpand(func(key string) { ... }).OnCollapse(func(key string) { ... })
th.Reveal("src/main.go") // opens the nodes above it and scrolls it into view
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
  of a tab list (`Bar`, `List`, `Tab`, `TabSelected`, `TabHover`, `TabDisabled`,
  `TabFocusVisible`, `Indicator`, `Panel`, `DropLine`, `ScrollButton`, `MenuButton`) are set with `Styles` or the theme, and
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
  `Reorderable()` lets the user move the tabs: drag a tab onto the half of
  another tab it should go beside, where a line (the `DropLine` part) shows
  where it lands, or press Ctrl+Shift+Left or Ctrl+Shift+Right on the focused
  tab, as the Dock does for its panels. Assistive technology hears the new
  position, such as "Sales moved to position 2 of 3" (`Strings.TabMoved`). The
  Tabs keeps the user's order; a tab it has not seen before goes after the
  others. `OnReorder` gets the values in their new order, so an app can save
  it and pass the tabs in that order next time. A disabled tab cannot be
  dragged, but a tab can be put beside it.
  When the tabs are wider than the strip, the strip scrolls sideways, as
  TabView's does, and tabs that fit are drawn exactly as before. Two scroll
  buttons appear after the strip ("Scroll tabs back" and "Scroll tabs forward",
  `Strings.ScrollTabsBack` and `ScrollTabsForward`): each scrolls by three
  quarters of the strip, is disabled at its end, and takes no focus, so Tab
  does not reach it and the arrow keys, Home and End move the strip as they move
  the selection. They work under high contrast, where a fade at the edges would
  be lost, and without a wheel. `ScrollButtons(false)` leaves them out. A
  vertical wheel over the strip scrolls it sideways until it reaches an end, and
  then the page scrolls on. Whenever the selection changes, by a click, a key,
  the menu or code, the selected tab is scrolled into view with a little of the
  next one showing; with reduced motion the strip jumps. An open `Popup` closes
  when the wheel scrolls the strip. `OverflowMenu()` adds a button ("All tabs",
  `Strings.AllTabs`) that opens a menu of every tab in order, with its whole
  label: the selected tab is checked and a disabled tab cannot be chosen.
  Choosing a tab selects it as a click does, then scrolls to it and focuses it;
  the button is one Tab stop after the tab list. `TabMaxWidth` caps each tab's
  width: a longer label ends with an ellipsis and shows whole in its tooltip,
  and its accessible name stays the whole label (with `RenderTab` only the width
  applies). `Prefix` and `Suffix` put elements before and after the strip, in
  the same row (the `Bar` part), outside the tab list and outside what scrolls.
  A segmented strip stays as wide as its tabs until it reaches the width it has.
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
  while it is open. A tree with no size takes the height its parent leaves it,
  and shows the rows that fit in it; a tree in a parent with no bounded height
  shows every row, so size the tree with `Style` to bound it. The tree is one
  focusable element. Its keys act on the
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
- **Tree from code.** A `TreeHandle` (`NewTreeHandle`, passed with `Tree.Handle`)
  opens and closes nodes by key with `Expand` and `Collapse`, opens or closes all of
  them with `ExpandAll` and `CollapseAll`, and opens the nodes above a key and scrolls
  it into view with `Reveal`. `Select` does the same and selects the node, without
  running `OnChange`. A request applies on the next frame, in the order it was made,
  and a request made before the first frame applies then. A key that is not shown
  does nothing, except that `Reveal` and `Select` open the nodes above it. Collapsing
  the node that holds the selection moves the selection to the collapsed node and runs
  `OnChange`; `CollapseAll` moves it to the top-level node that holds it. `ExpandAll`
  and `Reveal` ask for the children of the nodes they open, so a lazily loaded tree
  loads all of them below those nodes.
- **Open state.** `Expanded` keeps the open keys in a `Signal[[]string]`: the tree
  sets it when a node opens or closes, and a value the app sets opens and closes the
  nodes. `OnExpand` and `OnCollapse` run once for each node that changes, from a
  click, the keys, `*`, an accessibility action or a `TreeHandle`, and not for a value
  the app sets.

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

Steppers. A stepper shows the progress through the steps of a process, such as the
pages of a checkout. Its look is `ui.StepperStyle`, set on the theme as
`Theme.Stepper` or with `Styles`, and it follows high contrast and the Text size.

```go
ui.Stepper(step, // *ui.Signal[int]: the current step, from 0
	ui.Step{Title: "Account", Description: "Sign in or create one"},
	ui.Step{Title: "Shipping", Icon: icons.Home},
	ui.Step{Title: "Payment"},
).Key("checkout").Label("Checkout steps").
	Vertical().                                  // stacked, with the line running down between the indicators
	OnSelect(func(i int) { step.Set(i) })        // the steps can be chosen; the app sets the signal
```

- **States.** The steps before the current one are complete: their indicator is
  filled with Accent and shows a check mark. The current step has an Accent outline
  on AccentSoft, and the later steps an outlined circle with their number, or their
  `Icon`, in the muted text color. The line after a complete step is Accent. No state
  changes the font weight. Under high contrast a complete indicator is Highlight, the
  current one has a thicker Highlight outline, and the others are outlined in the text
  color. A current step equal to the number of steps marks every step complete.
- **Width.** A horizontal stepper fits the width it is given: when its steps do not
  fit, they shrink down to their indicators, and their titles and descriptions end
  with an ellipsis. A vertical stepper wraps its texts instead.
- **Choosing.** Without `OnSelect` the stepper only shows the progress, and it takes
  no focus. With it, a click, or Enter or Space on the focused step, runs `OnSelect`
  with the step's index. The signal is not set by the stepper, so the app decides
  whether a step can be reached yet. The stepper is one Tab stop, at the current step.
  Left and Right (Up and Down when `Vertical`) move the focus to the previous and next
  step without choosing it and without wrapping, and Home and End go to the first and
  last step, as in a Sidebar. `Disabled(true)` shows the steps of a stepper with
  `OnSelect` dimmed (GrayText under high contrast), and they cannot be focused or
  chosen.
- **Accessibility.** The stepper is a list named by `Label` (default "Steps"). Each
  step is a list item named by its position, title and state, such as "Step 2 of 4:
  Shipping, current", "Step 1 of 4: Account, completed" or "Step 3 of 4: Payment, not
  started" (the `StepCompleted`, `StepCurrent` and `StepUpcoming` messages of
  `ui.Strings`). The current step is selected in the list, as the current location of
  a breadcrumb trail is. The lines are hidden from assistive technology. `Tooltip(s)`
  shows s over the stepper, and is read as the list's description.

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

## Group boxes

`ui.GroupBox(title)` is a titled section that holds related content, such as the settings
of one feature. Its look is `ui.GroupBoxStyle`: per app in `Theme.GroupBox`, or per
instance with `Styles`. It follows high contrast and the Text size.

```go
ui.GroupBox("Notifications").Description("How you hear about changes.").
	Child(ui.Checkbox("Email", email)).
	Child(ui.Checkbox("Desktop", desktop))
ui.GroupBox("Storage").Fill().Children(usageBar, cleanButton) // filled, without a border
```

- **Look.** The box is outlined in the border color by default; `Fill()` fills it with a
  raised surface color instead, without a border. The title is semibold and the
  description muted, in the `Header` part, and the children are laid out in a column in the
  `Content` part. A box with neither a title nor a description has no header. Under high
  contrast both looks are the window color outlined in the text color.
- **Accessibility.** The box is a group named by its title, with the description as its
  description, so assistive technology reads the title as it enters the group.

## Description lists

`ui.DescriptionList()` shows terms with their values, such as the details of a record.
Its look is `ui.DescriptionListStyle`: per app in `Theme.DescriptionList`, or per instance
with `Styles`. It follows high contrast and the Text size.

```go
ui.DescriptionList().
	ItemText("Owner", "Dana Lee").
	ItemText("Region", "West").
	Item("Status", ui.BadgeText("Active").Variant(ui.BadgeSuccess)) // any element as the value
ui.DescriptionList().Columns(2).Vertical().Bordered(). // two columns, terms above values, with lines
	ItemText("Created", "May 4").ItemText("Updated", "June 12")
```

- **Layout.** The items are laid out in a grid, row by row, in `Columns(n)` columns of the
  same width (1 by default). By default each term is in a column of its own, 120 pixels
  wide (the `Width` of the `Term` part), beside its value; `Vertical()` puts the term above
  the value instead. The term is muted text, and the value takes the rest of the item.
- **Bordered.** `Bordered()` outlines the list and draws a line between its items, which are
  padded. The lines are the border edges of each `Item`: one above every row but the first
  and one before every column but the first. Under high contrast the lines and the terms are
  the text color.
- **Accessibility.** Each item is a group named by its term, with the value inside it, so
  assistive technology reads the value with its term.

## Status bars

`ui.StatusBar()` is the bar along the bottom of a window, which shows the state of what
the window holds, such as the current page, a count or the position of the caret. Its
look is `ui.StatusBarStyle`: per app in `Theme.StatusBar`, or per instance with `Styles`.
It follows high contrast and the Text size.

```go
ui.Div().Column().Size(ui.Full, ui.Full).
	Child(page).                                        // content that grows, such as Grow(1)
	Child(ui.StatusBar().
		Start(ui.Text("Saved")).                        // at the left, taking the free room
		End(ui.Text("Ln 12, Col 4"), ui.Text("UTF-8"))) // at the right
```

- **Look.** A compact strip in the panel color with a border along its top, and 12-pixel
  muted text that its items inherit. The `Start` items are in a row that takes the free
  room, and the `End` items in a row at the right, 12 pixels apart. Under high contrast the
  strip is the window color, and its border and text are the text color.
- **Layout.** The bar fills the width of its container and does not shrink. Put it last in
  the window's column, after content that grows.
- **Accessibility.** The bar is a status bar (UI Automation's StatusBar control type) that
  holds its items, so a screen reader can find it by its type. It needs no name; `Label`
  names it when a window has two. It is not a live region: a change to its text is not
  announced, so an app announces a change the user should hear about with `Announce`.

## Toolbars

`ui.Toolbar(label)` is a row of controls, such as the formatting buttons above an editor,
named by `label` for assistive technology. `ui.ToolbarSeparator()` draws a thin line
between groups of controls. Its look is `ui.ToolbarStyle`: per app in `Theme.Toolbar`, or
per instance with `Styles`. It follows high contrast and the Text size.

```go
ui.Toolbar("Formatting").
	Child(ui.Toggle("Bold", bold).Small()).
	Child(ui.Toggle("Italic", italic).Small()).
	Child(ui.ToolbarSeparator()).
	Child(ui.Button("Undo").Ghost().Small().OnClick(undo))
```

- **Keys.** The toolbar follows the WAI-ARIA toolbar pattern
  (https://www.w3.org/WAI/ARIA/apg/patterns/toolbar/). It is one Tab stop: Tab enters it at
  the control that had focus last (the first one at the start), and the next Tab leaves it.
  Left and Right move focus to the previous and the next control, wrapping at the ends, and
  Home and End move it to the first and the last. Separators and disabled controls are
  passed over. A control that uses those keys keeps them, as the pattern says: a text field
  moves its caret with them, and a slider or a toggle group changes its value.
- **Look.** The controls are in a row 2 pixels apart, on the surface color and outlined in
  the border color. The toolbar does not restyle its controls, so give its buttons and
  toggles the looks that suit it, usually `Ghost()` (the default of a toggle) and `Small()`.
  Under high contrast the outline and the separators are the text color.
- **Layout.** The toolbar fills the width of its container. It does not wrap or move
  controls that do not fit into a menu; keep it short enough for the window.
- **Accessibility.** The toolbar is a tool bar (UI Automation's ToolBar control type) named
  by its label, with its controls and separators inside it.

## Carousels

`ui.Carousel(index, slides...)` shows slides one at a time, such as featured items or the
steps of a tour. `index` is a `*ui.Signal[int]`, the slide shown, from 0. Its look is
`ui.CarouselStyle`: per app in `Theme.Carousel`, or per instance with `Styles`. It follows
high contrast, the Text size and reduced motion.

```go
ui.Carousel(slide, first, second, third). // any elements as the slides
	Label("Featured products").
	Loop().                      // the buttons, the keys and AutoPlay wrap around
	AutoPlay(5 * time.Second).   // moves on by itself, on the window's clock
	Height(ui.Px(240)).          // the viewport takes the room the controls leave
	OnChange(func(i int) { ... })
```

- **Controls.** Under the slide are a previous and a next `IconButton` and a dot for each
  slide, which goes to that slide. Without `Loop` the buttons are disabled at the first
  and the last slide. Left and Right go to the previous and the next slide while focus is
  on the buttons or the dots, and keep the focus on the dot of the slide shown when it was
  on a dot. Only that dot is a Tab stop, so the carousel takes three Tab stops whatever
  the number of slides (four with the play button of `AutoPlay`). The dot of the slide shown is a pill in the accent color, wider
  than the others, so it is not told by its color alone.
- **Slides.** Only the slide shown is built, as only a tab's selected panel is, so a slide
  that keeps state of its own keeps it in signals outside the carousel. A new slide comes
  in from the side it is on with a short slide and fade; reduced motion turns it off. An
  index out of range shows the nearest slide, and an index set from outside is shown
  without `OnChange` or an announcement.
- **AutoPlay.** `AutoPlay(d)` moves to the next slide every `d`, on the window's clock (see
  `Context.After`), and runs `OnChange`. It waits while the pointer is over the carousel or
  focus is inside it, and goes on once they leave; it stops at the last slide unless the
  carousel loops. It does not run in a `Disabled` carousel.
- **Play button.** With `AutoPlay`, the controls start with a button that stops and starts
  it, as WCAG's Pause, Stop, Hide asks. It is the first of the controls in the Tab order,
  as the WAI-ARIA pattern places its rotation control, and a button named "Pause slides"
  while the slides move and "Play slides" while they are stopped (`Strings.PauseSlides`,
  `Strings.PlaySlides`). Its name says what it does, so it reports no pressed state. A press
  stops AutoPlay until the next press. While the user asks for reduced motion, AutoPlay
  starts stopped and the button shows Play, so the user can still start it; the slides
  then change without their motion. The app does not control the button: there is no
  signal for it.
- **Accessibility.** The carousel follows the WAI-ARIA carousel pattern as far as jamb's
  roles go: it is a group named by its `Label` and described as a carousel
  (`Strings.Carousel`; an unnamed carousel is named "Carousel"). The slide shown is a
  group named by its position, such as "Slide 2 of 5" (`Strings.SlideOf`), and the other
  slides are not in the accessibility tree. The buttons are named "Previous slide" and
  "Next slide", and the dots, "Go to slide 3" (`Strings.GoToSlide`), are in a group
  named "Slides" in which the dot of the slide shown is selected. When the user changes
  the slide, its position is announced politely; AutoPlay changes it without an
  announcement, so a screen reader is not interrupted. `Tooltip(s)` shows s over the
  carousel and is read as its description, in place of "Carousel".
- **Other.** `Disabled(true)` dims the carousel and disables its buttons and dots;
  `Key`, `FocusHandle` (which focuses the dot of the slide shown) and the sizing methods
  are as for the other widgets.
