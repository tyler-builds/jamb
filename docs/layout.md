# Layout

Shrinking, grid layout, split panes and docking. Flex layout, sizing and scrolling are set on each element; see [the builder API](concepts.md#api-guide).

## Shrinking and minimum sizes

When a row is narrower than its content, its items shrink in proportion to their
`Basis` and `Shrink`. An item that shrinks is held to an automatic minimum, which
follows CSS Flexbox's automatic minimum size of flex items
([CSS Flexbox, 4.5](https://www.w3.org/TR/css-flexbox-1/#min-size-auto)):

- **Text keeps its widest word.** The minimum is the min-content width of the text:
  the width of its longest unbreakable run, which is a word, or a part of a hyphenated
  word. A text in a shrinking row therefore wraps between words, and never between letters.
- **Containers keep their children's min-content widths.** A column or a wrapping row
  takes the largest of its children's, and a row takes their sum, with the gaps and the padding.
- **Opt-outs.** `Truncate()` sets the minimum to zero, so the text stays on one line
  and shrinks with an ellipsis. `NoWrap()` keeps the full width of the text, since it
  is one line. Any explicit `MinWidth` replaces the automatic minimum. An item that
  clips its overflow (`ScrollX`, `ScrollY`, `ClipContent`) has no automatic minimum.
- **A fixed `Width` is the item's floor**, as it was before this rule: the item does
  not shrink below it. CSS takes the smaller of the width and the content instead, and
  jamb does not. The exception is a text field (`TextInput`, `TextArea` and the frame of
  a `NumberInput`): its fixed width is a ceiling, and in a shrinking row it keeps the
  room for six digits and no more, so it does not collapse to a letter's width. A
  `MinWidth` replaces that minimum.
- **Bars and sliders** (`ProgressBar`, also when indeterminate, and `Slider`) keep a
  default minimum of 160 px, so they stay visible in a container that has no width of
  its own. `MinWidth` replaces it.
- **A word longer than its container breaks inside the word**, so a URL stays within
  its row or column. A word that fits its container does not break, even when a fixed
  item beside it takes the room. The row then overflows, as in CSS, so give such items room.
- The minimums of text grow with the Text size (`JAMB_TEXT_SCALE`). A control whose label
  is wider than a fixed `Width` should use `MinWidth` instead, so the control grows with
  its label.
- **A stretched item keeps its cross-axis limits.** An item that stretches across a row
  (its height) or a column (its width) takes the line's size, but never less than its
  `MinHeight` or `MinWidth`, nor more than its `MaxHeight` or `MaxWidth`, as in CSS. A
  row 24 px tall holds a child with `MinHeight(ui.Px(32))` at 32 px.

## Grid layout

`Div().Grid(columns...)` lays the children out in a grid instead of a row or a
column, one per cell, row by row. It saves the nested rows of forms and dashboards:

```go
ui.Div().Grid(ui.Auto, ui.Fr(1), ui.Px(120)).Gap(12).
	Child(ui.Text("Name")).Child(ui.TextInput(name)).Child(ui.Button("Check")).
	Child(ui.Text("Notes")).Child(ui.TextArea(notes).GridSpan(2, 1))
```

- **Columns.** `ui.Px` and `ui.Percent` take that width. `ui.Auto` fits the widest
  item in the column, and shrinks towards the narrowest wrap of its text when the
  grid is too narrow. `ui.Fr(n)` takes n shares of what the others leave.
- **Rows** are as tall as their tallest item. `GridRows(...)` sizes the first rows
  the same way (an `Fr` row shares the grid's height, when it has one); the rest
  fit their content.
- **Spans.** `GridSpan(cols, rows)` (on a `Div`, `Text` and every widget with the
  sizing methods) covers more cells. An item that does not fit in the rest of its
  row starts the next one, and the cells it skipped stay empty. Items that span
  tracks do not size `Auto` columns; a row grows to fit an item spanning into it.
- **Cells.** Items fill their cells unless they have a size of their own. A
  widget's default width (a text field's 240 px) gives way to the cell; a width
  set with `Width` is kept. `AlignSelf`, or `Align` on the grid, places an item
  in a taller row. `Gap` separates rows and columns.

The style fields are `GridColumns`, `GridRows`, `ColSpan` and `RowSpan`, so a theme
or a `Style` value can set them too. It is a subset of CSS grid: there are no named
areas, explicit line placement, dense packing or subgrids.

## Split panes

`ui.Split(panes...)` lays out panes side by side, with a divider between each two that
the user drags, or moves with the keys, to give one pane more of the space. Each pane is
made with `ui.Pane(content)` and takes a size, or it fills the space the sized panes
leave. Add `Orientation(ui.SplitVertical)` to stack the panes. The contents are any
elements, so a split can hold another split. Its look is `ui.SplitStyle` (the divider,
its hover, drag and focus states, the grip and the panes; see the Styling section), and
its sizing methods are the same as the other widgets'.

```go
ui.Split(
	ui.Pane(sidebar).Size(ui.Px(200)).Min(ui.Px(150)).Max(ui.Percent(40)),
	ui.Pane(editor).Min(ui.Px(300)), // no Size: fills what is left
	ui.Pane(inspector).Size(ui.Percent(20)).Min(ui.Px(180)),
).Orientation(ui.SplitHorizontal)
```

**Sizes** are `ui.Px` (logical pixels) or `ui.Percent`. A percentage is of the length the
panes have, which is the split's length less each divider's bar (1 px by default). `Min` and `Max`
take either unit as well. Any other length (`ui.Auto`, `ui.Fr`) panics, as a programming
error. When `Min` is larger than `Max`, `Min` wins.

- **Layout.** Sized panes take their size, clamped to their limits. The fill panes share
  the rest equally, or in proportion to the weights that a drag between two of them sets.
  Each fill pane stays within its own limits.
- **Window resize.** A pixel pane keeps its pixels, a percentage pane keeps its share, and
  a fill pane takes the change.
- **Too little space.** When the sized panes need more than the length, the fill panes go
  to their `Min` first, then the sized panes shrink toward theirs, in proportion to how far
  each is above its `Min`. Content that still does not fit is clipped.
- **Too much space.** When no pane can take the space that is left over, the last visible
  pane is drawn with it. Its stored size does not change, so the space is only a display
  of the last pane.

**Dragging** a divider moves space between the two panes beside it, each in its own unit,
and stops when either pane reaches a limit. The fill panes share what the sized panes leave,
in proportion to their weights. So a drag that changes a sized pane changes every fill pane,
each in proportion to its weight. A drag between two fill panes moves space only between
those two: their weights change, and their sum stays the same. A drag between two sized
panes moves space only between those two.

**The sizes can be saved.** `SizeSignal` gives the split a signal that the app owns. The
split sets it as the user drags or uses the keys, and reads it on every render, so the app
can also set it. The signal holds `ui.Px` or `ui.Percent` lengths, and each pane keeps the
unit it has.

```go
sidebarSize := ui.NewSignal(ui.Px(200)) // holds ui.Px or ui.Percent
ui.Split(
	ui.Pane(sidebar).SizeSignal(sidebarSize).Min(ui.Px(150)),
	ui.Pane(editor),
).OnChange(func() { save(sidebarSize.Peek()) })
```

`OnChange` runs after the user changes a size, from the keyboard or the pointer, and only
when a size actually changes. It does not run for a collapse.

**Collapse.** A pane marked `Collapsible` collapses to zero size, and restores its previous
size. `Collapsed` stores the state in a signal the app owns, and also makes the pane
collapsible. Without it the split keeps the state itself.

```go
collapsed := ui.NewSignal(false)
ui.Pane(files).Size(ui.Percent(30)).Collapsible().Collapsed(collapsed) // Enter collapses
```

The divider that collapses a pane is the one between it and the nearest fill pane, on the
pane's side of the split. With no fill pane, the divider after the pane collapses it, and
the last pane collapses through the divider before it. A divider collapses one pane. While
a pane is collapsed, its divider and the divider beside it do not move.

**Identity.** `Key` gives a pane an identity among the panes of its split, so its content
keeps its state when panes are added, removed or reordered. The keys of one split's panes
must differ. A pane without a key is known by its position.

The divider is 1 px wide by default (`Divider`, its `Width` in a horizontal split and its
`Height` in a vertical one), and it takes only that much of the layout: the panes meet at the
bar, which reads as the border between them. Its hit area is 9 px, centered on the bar, and lies
over the edges of the panes beside it, so it is easy to grab; a press in those few pixels of a
pane drags the divider, as in VS Code and Zed. That includes the 4 px of a pane's scroll bar
nearest the bar; the rest of the scroll bar takes its presses as usual. The grip is a short
pill over its middle. `Grip` on the split sets when it shows: only while the pointer is over
the divider or it is dragged or focused (`ui.SplitGripOnHover`, the default), always
(`ui.SplitGripShown`), or never (`ui.SplitGripHidden`). Without the grip the divider is a plain
line between the panes, like a border. The dock's dividers use `SplitGripOnHover` too.

```go
ui.Split(files, editor)                           // a line, with the grip on hover
ui.Split(files, editor).Grip(ui.SplitGripShown)   // the grip is always drawn
```

- **Pointer.** Dragging the divider moves it. The pointer is captured while the button is
  held, so a drag that leaves the divider still moves it. The cursor is the horizontal or
  vertical resize cursor. A double-click collapses or restores the pane that the divider
  collapses.
- **Keyboard.** The divider is one Tab stop. Left and Right move it in a horizontal split,
  and Up and Down in a vertical one. A step is 10 logical pixels, or 5 percent of the
  length, for the pane that the divider resizes; Shift makes it 50 pixels or 10 percent.
  Home and End move the divider to its limits, and Enter collapses or restores the pane
  that the divider collapses. The keys follow the window splitter pattern of the WAI-ARIA
  Authoring Practices (https://www.w3.org/WAI/ARIA/apg/patterns/windowsplitter/), which
  makes Enter the toggle of the collapsible pane; Home and End are the pattern's optional
  keys. Shift with an arrow is the larger step, as the pattern does not define one.
- **Accessibility.** Each divider is a focusable element with the Separator role. It is named by
  `Label` on the split when that is given. Otherwise, when both panes beside it have a
  `Label`, it is named after them, such as "Resize Sidebar and Editor"; any other divider
  is "Resize panes". It has the RangeValue pattern. Its
  value is the size of the pane it resizes, in that pane's unit, and its minimum and
  maximum are the least and the greatest sizes that pane can take at this divider. The
  small and large changes are the arrow and Shift steps. A change of the size is a change
  of the Range value, which the accessibility tree publishes as a property change, and
  assistive technology can set the value. A pane with a `Label` is a group with that name.
  Microsoft has no splitter control type, so the divider uses Separator, which the Windows
  Separator control type defines
  (https://technet.microsoft.com/en-us/ee671643(v=vs.71)). The control type that a splitter
  exposes in other frameworks was not verified for this release.
- **Layout.** A split fills its container by default, in both directions. In a column with
  other children, give it a `Height` or a `Grow` so it has room. The split measures its
  length after each layout, and a split whose length changed renders again in the same
  frame, so its panes always match the length it has.

The demo's Components page has a "Split panes" section with samples of a fraction split,
a stacked split, a nested split, a collapsible pane, a pixel pane with limits, and three
panes.

## Docking

`ui.Dock(layout, panels)` shows panels that the user rearranges as in an IDE: a
panel is dragged over a group to join it as a tab, or to a side of it to make a
group beside it, or to an edge of the dock; a group can float in a window of its
own. The layout is a `ui.DockLayout` signal that the app owns, so the arrangement
is saved with the settings and loaded back.

```go
var defaultLayout = ui.DockLayout{Root: ui.DockNode{
	Sizes: []float64{0.25, 0.75},
	Children: []ui.DockNode{
		{Panels: []string{"explorer"}, Active: "explorer"},
		{Vertical: true, Sizes: []float64{0.7, 0.3}, Children: []ui.DockNode{
			{Panels: []string{"editor"}, Active: "editor"},
			{Panels: []string{"output"}, Active: "output"},
		}},
	},
}}

panels := map[string]ui.DockPanel{
	"explorer": {Title: "Explorer", Content: explorer, Closable: true},
	"editor":   {Title: "Editor", Content: editor},
	"output":   {Title: "Output", Content: output, Closable: true},
}
layout := ui.NewSignal(loadLayout(defaultLayout)) // the saved layout, or the default

ui.Dock(layout, panels).OnChange(func(l ui.DockLayout) { saveLayout(l) }).
	Floating(app.OpenDockWindow)

// Reset layout, and reopen a panel that was closed:
layout.Set(defaultLayout.Clone())
layout.Set(layout.Peek().Show("output"))
```

**The layout model.** A `DockLayout` is a tree of `DockNode`s and a list of
`DockFloat`s. A node with `Children` is a split: its children sit side by side, or
one above the other when `Vertical` is set, and `Sizes` holds each child's share of
the split, which need not sum to 1. A node without children is a tab group: `Panels`
lists its panels in tab order and `Active` is the one it shows. A floating group is
a tab group with an `ID`, and the position and size of its window. The layout is a
value: its methods `Move`, `Close`, `Show`, `Focus`, `Float`, `Dock`, `MoveSide` and `Clone`
return a new layout and never change their receiver. They also put the layout in order: an empty group goes, a split with one
child is replaced by it, and a split inside a split of the same direction is merged
into it. The layout marshals to JSON with `encoding/json`. A panel ID in a loaded
layout that the dock does not have is dropped when the dock is shown, and a panel
that the layout does not hold stays hidden until `Show` shows it.

**Dragging.** Drag a tab over a group: the group shows the zone the panel will land
in, the center to add it as a tab or a side to make a group beside it. Drag a tab to
one of the four strips along the dock's edges, which appear while a panel is dragged,
to put the panel along that edge. A drop that would change nothing shows no preview.

**Keyboard.** The tab's menu, opened with the context menu key or Shift+F10, has
Move left, right, up and down (into the group on that side), Split right and Split
down, Float or Dock back, and Close panel (for a closable panel). Ctrl+Shift with an
arrow key moves the focused panel to the group on that side. Left and Right, Home and
End select the tabs of a group. Ctrl+Tab and Ctrl+Shift+Tab select the next and
previous panel of a group, and Ctrl+F6 moves focus to the next group. The dividers
are `Split`s, so they take the arrow keys, Home and End as a split does.

**Floating windows.** `Floating` takes the function that opens a window for a group.
`app.OpenDockWindow` opens it with `app.Open`, since the `ui` package cannot import
`app`. A tab released outside the dock, where no group of any dock window takes it,
floats its panel in a window whose top-left corner is by the pointer, kept on its
monitor. A tab released over a group of another window of the dock moves into that
group, so panels move between windows by dragging as well as with Float and Dock
back. A floating window shows its group. Its position and size are recorded in the
layout as the user moves and resizes it: the layout takes the new place at once, and
`OnChange` runs once the window has been still for half a second, or at once when the
window closes. Closing a window, by the user or by the dock, docks its panels back
into the main window, so no panel is lost with its window. Set `QuitWhenClosed` on
the main window, so that closing it closes the floating windows too, and their
positions are saved as they close.

A floating window is part of the dock's window. It shows that window's theme, text and
zoom, whatever the opener sets, and keeps them in step both ways: `SetTheme`, `SetThemes`,
`SetStrings` and a zoom in either window apply to both, and the dock window's `OnZoom`
hears of a zoom made in a floating one. The dock window's key bindings and keymap file apply
in the floating window, and an action that nothing in the floating window handles goes on
to the dock window: from the dock up to its root, as if the panel were still docked. So the
app's shortcuts and its command palette work from a floated panel, and they act in the
dock's window. A floating window's own bindings are not used.

**State.** A panel keeps its state wherever it goes: to another group, split off,
dropped on an edge, floated into a window and docked back, or put back by a layout
reset. Its components keep their `Local` state, their signals and their `OnDispose`
functions. Its text fields keep their text, undo history and selection, its scroll
containers their offsets, and it keeps the focus when it had it. `OnDispose` runs
only when the panel closes: with `Close`, or when a layout no longer holds the
panel. A panel that moves to another window renders once more there, with its state
kept. A dock that its parent builds again for the same layout keeps its panels the
same way. A signal that a panel reads still re-renders it, in whichever window it is
in. The state is kept by the panel's place in the dock, not by its place in the tree,
so give two docks in one window different `Key`s and keep one layout signal per dock.
The layout itself is a signal the app owns: save it in `OnChange`, which also runs
once a divider has been dragged and let go.

The dock's kept state lives while the layout is shown: while at least one `Dock` for
it is mounted, or one of its floating windows is open, in any window of the app. Hiding
a dock (unmounting all of its `Dock` elements) disposes its panels, so their
`OnDispose` functions run once, as a component's do when it is removed. The state is
released at the end of the frame in which the last of them goes, so a dock that the same
frame mounts again keeps its panels. A release first runs a pending `OnChange`, and then
the state is dropped. A layout whose floating windows are all closed and whose docks are
all hidden holds no state.

**Styling.** `ui.DockStyle` is the look of the groups, tabs, panels and the drop
preview, and `Theme.Dock` sets it for every dock. The dividers take `Theme.Split`.
The parts are listed in the widget table of docs/styling.md.

**Limits.**
- A new floating window opens with its top-left corner 24 by 16 logical pixels up and
  left of the pointer. With the native title bar, which lies above the tabs, the tab
  lands a little below the pointer's line; drop it again to move it.
- A drop on another window shows no preview before the release. It is found by each
  window's content area under the pointer, so where two floating windows overlap at
  the release point, the drop goes to one of them.
- Ctrl+F6 moves between groups, not F6, which moves between the window's regions:
  the window, non-modal dialogs and the toast region (see
  [Non-modal dialogs](widgets-overlays.md#non-modal-dialogs)).
- Every panel of the layout is built when the dock is shown, not when it is first
  selected.
- A split keeps each pane at least 48 logical pixels, and the edge strips are 24
  logical pixels wide.
- A floating group has no sides: a panel dropped in it joins its tabs.
- The floating groups are not shown in a dock without `Floating`.
