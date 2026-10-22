# Data widgets

Variable-height lists, uniform lists and data grids.

## Uniform lists

Long lists build only the rows on screen. Every row must share one height;
the list measures the first row and positions the rest from it:

```go
h := ui.NewListHandle()
ui.UniformList(len(events), func(i int) ui.Element { return eventRow(events[i]) }).
    Style(ui.Style{Grow: 1}).Handle(h)  // Style sizes the scroll container
h.ScrollToItem(5000)                    // scrolls the least amount that shows row 5000
```

## Variable-height lists

When rows differ in height, use `ui.ListOf`. It takes the items, a key for each
item and a function that builds a row:

```go
h := ui.NewListHandle()
ui.ListOf(notes, func(n *store.Note) string { return n.ID }, func(_ int, n *store.Note) ui.Element {
    return noteRow(n)
}).Style(ui.Style{Grow: 1}).Handle(h)
h.ScrollToItem(i)  // the least scroll that shows item i completely
h.ScrollToEnd()    // the last item
h.AtEnd()          // within a pixel of the end; false before the first paint
```

`AtEnd` reads the scroll position without tracking it, so a component that
shows a control only while the list is away from its end should read the state
through `OnAtEnd` instead. It is called on the first paint and then on each
change, from the paint that makes the change:

```go
atEnd := ui.Local(ctx, "atEnd", false)
ui.ListOf(...).Bottom().Handle(h).OnAtEnd(atEnd.Set) // then: if !atEnd.Get() { show "Jump to latest" }
```

Only the rows near the viewport are built. A row's height is measured once it
is built and remembered by its key while the item stays in the list. A row not yet seen is estimated from the
mean of the measured heights (32 px before any is measured). The scroll position
stays on the item at the top of the viewport, so measuring rows above it, or
inserting and removing items above it, does not make the view jump. Items are
compared by slice identity: a new slice means the items changed, and a slice
changed in place is not detected. Keys must be unique among the items; a
duplicate gets a warning once and falls back to its position.

`Bottom()` is for chat-like lists. Rows shorter than the viewport sit at its
bottom, and a list that is at its end stays there when items are appended or a
row grows.

`ScrollToItem` eases toward its item when the item is within a viewport of the
current position. Further away, the list jumps there at once and builds only
the rows at the destination.

Prefer `UniformList` when every row has the same height. Its scroll step is
cheaper and its scrollbar thumb is exact from the start.

## Data grids

`ui.DataGrid` shows rows and columns of business data. Like `UniformList`, it builds only the rows in view, so a grid of 100,000 rows costs what its visible rows cost. Its header stays at the top as the rows scroll, and the header scrolls with the rows when the columns are wider than the grid. With no size, a grid takes the height its parent leaves it, as a `Tree` does, so a grid in a full-height column needs no `Height`; a grid in a parent with no bounded height shows every row.

```go
type run struct {
	ID, Service, Status string
	Cost                float32
}

picked := ui.NewSignal([]string(nil)) // the keys of the selected rows
order := ui.NewSignal(ui.SortState{}) // the column and direction of the sort

ui.DataGrid(runs, func(r run) string { return r.ID },
	ui.DataColumn[run]{Header: "Run", Text: func(r run) string { return r.ID },
		Compare: func(a, b run) int { return strings.Compare(a.ID, b.ID) }, Width: 130, Resizable: true},
	ui.DataColumn[run]{Header: "Service", Text: func(r run) string { return r.Service },
		Compare: func(a, b run) int { return strings.Compare(a.Service, b.Service) }, Grow: 1, Resizable: true},
	ui.DataColumn[run]{Header: "Cost", Text: func(r run) string { return fmt.Sprintf("$%.2f", r.Cost) },
		Compare: func(a, b run) int { return cmp.Compare(a.Cost, b.Cost) }, Align: ui.End, Width: 100, Resizable: true},
).Label("Deploy runs").
	Selection(ui.SelectionMultiple, picked).
	Sort(order).
	Density(ui.DataGridCompact).Striped().
	OnActivate(func(r run) { openRun(r.ID) }).
	Style(ui.Style{Height: ui.Px(420)})
```

`ui.DataColumn[T]` is the column type. The name is `DataColumn` because `ui.Column` is the layout direction (`Div().Column()`).

| Field | Meaning |
| --- | --- |
| `Header` | The header text, and the column's name for assistive technology |
| `Key` | Identifies the column in a `SortState` and in the widths the user sets. `Header` when empty |
| `Text` | The text of each cell, and the cell's accessible name |
| `Cell` | A render slot: replaces the text of each cell (hidden from assistive technology, so set `Text` too) |
| `RenderHeader` | A render slot: replaces the header text. `PartState.Selected` is set while the column is sorted |
| `Compare` | Orders two rows for sorting, as `strings.Compare` does. Nil means the column cannot be sorted. A sort is stable |
| `Width`, `MinWidth`, `MaxWidth` | The width at the 100 percent Text size (120 when zero), and the limits for resizing (40 and no limit when zero). Columns grow with the Text size, as the rows do |
| `Grow` | The share of the grid's spare width the column takes when the columns are narrower than the grid |
| `Resizable` | Lets the user resize the column (see the keyboard and mouse below) |
| `Align` | `ui.Start` (the zero value), `ui.Center` or `ui.End` for the cells and the header |
| `Interactive` | The `Cell` holds controls, such as buttons. Its content is not hidden from assistive technology (see Interactive and editable cells below) |
| `Edit` | Makes the cells editable: `Edit(row, value) error` saves the value, or returns the error that keeps the editor open. Set Value (assistive technology) runs it too |
| `Editor` | Replaces the text field of an edit with a custom editor, such as a `ui.Select` (see Custom editors below) |
| `Value` | The text an edit starts with, when it differs from `Text` (a raw number for a formatted one). Nil means `Text` |
| `Movable` | Lets the user move the column, by dragging its header or with Ctrl+Shift+Left and Right on it (see the keyboard and mouse below) |
| `Pin` | `ui.PinStart` or `ui.PinEnd` keeps the column at the start or the end of the grid as the other columns scroll sideways. The pinned columns are shown before and after the others; a column moves only among the columns of its pin |
| `Group` | Columns next to each other with the same group share one cell, of their width, in a row above the header (`HeaderGroup`). A group that a pin or a move splits is drawn in pieces. For assistive technology the group is the description of each column's header, and the row of groups is hidden |

The `Compare` of a column is easier to write with `ui.SortBy(key)`, which orders by any `cmp.Ordered` key, and `ui.SortByText(key)`, which orders text without regard to case. `SortState.Desc` reverses the order, so there is no descending helper.

The grid's methods:

- `Selection(mode, keys)`: `ui.SelectionNone`, `ui.SelectionSingle` or `ui.SelectionMultiple`. The keys of the selected rows are kept in `keys`, so the app reads the selection and can set it. A nil `keys` keeps the selection in the grid.
- `CellSelection(mode, cells)`: selects cells instead of rows, kept in `cells` as `ui.CellKey{Row, Column}` keys, which follow their rows and columns through a sort or a move. It replaces `Selection`: the later of the two calls applies, and the development warnings say so. The cells are the items of the selection for assistive technology, and a selected cell has the `CellSelected` fill. Ctrl+A makes a key for every cell, so with 100,000 rows `cells` is a large slice.
- `ColumnOrder(keys)`, `ColumnWidths(ws)` and `OnColumnsChange(fn)`: the order of the columns (by key) and the widths the user sets (by key, at the 100 percent Text size) kept in signals, so an app can save and restore them. A saved order survives new columns: an unknown key is dropped and the columns it does not name follow in their given order. `fn` gets a `ui.ColumnState{Order, Widths}` after each change the user makes: the end of a drag of a column's edge, each keyboard resize step, an auto-fit, and a move. It does not run on each move of a drag. The focus and an open edit follow their column when it moves.
- `Movable()`: lets the user move every column, as `DataColumn.Movable` does for one.
- `Handle(h)`: a `ui.NewDataGridHandle()` scrolls the grid from code by key: `ScrollToRow(key)` in the sorted order, `ScrollToColumn(key)`, and `FocusCell(rowKey, colKey)`, which moves the focused cell, scrolls it into view and focuses the grid without changing the selection. A request applies at the grid's next build; a key that names nothing does nothing.
- `ContextMenu(build)` and `HeaderContextMenu(build)`: menus built with the row and the column key, or the column key of a header. A right-press on a row that is not selected moves the focus there and selects the row; on a selected row it keeps the selection, as File Explorer does. Shift+F10 or the Menu key opens the menu of the focused cell, or of the header cell while the focus is on the header. An empty result opens nothing.
- `LoadMore(hasMore, fn)` and `LoadingMore(on)`: while `hasMore` is true, `fn` runs once when the rows within a viewport of the end are built, or when Down, PageDown or Ctrl+End is pressed on the last row, and again only after the number of rows changes. It runs after the frame, never while the grid is built. `LoadingMore(true)` shows a row after the last one with a small progress ring and "Loading more rows" (`Strings.LoadingMore`), announced politely; it is not one of the grid's rows for assistive technology.
- `Sort(s)`: the `ui.SortState` (`Key` and `Desc`) in `s`. A click on a header cycles the column through ascending, descending and not sorted. A nil `s` keeps the sort in the grid.
- `OnActivate(fn)`: runs `fn` with the row, on Enter or a double-click.
- The look of the states is a fill or a line, not a change of weight: a selected row has the `RowSelected` fill, a sorted column has its arrow (`SortIndicator`), and the text keeps its weight.
- `Density(d)`: `ui.DataGridComfortable` (36 logical pixels a row, the default) or `ui.DataGridCompact` (26). `Striped()` draws every other row in `RowAlternate`.
- `Loading(on)`: shows a progress ring over the rows. The rows, the focus and the selection stay.
- `Empty(el)`: what the grid shows with no rows. The default is "No rows". A `ui.Empty` state fits here (see [widgets-feedback.md](widgets-feedback.md#empty-states)).
- `Label(name)`, `Key(k)`, `Style(s)` and `Styles(st)`: as for the other widgets. `Styles` takes a `ui.DataGridStyle`; see [docs/styling.md](styling.md).

Rows are identified by their keys, which must be unique and stable. The focus, the selection and the sort follow the key, so a sort, a scroll or new rows keep the focused cell and the selected rows on the same data.

**Keyboard.** The grid is one Tab stop. Its keys act on the focused cell, and they follow the WAI-ARIA grid pattern (https://www.w3.org/WAI/ARIA/apg/patterns/grid/) and the Windows keyboard conventions for selection (https://learn.microsoft.com/en-us/windows/apps/develop/input/keyboard-interactions):

| Key | Action |
| --- | --- |
| Left, Right | Previous and next column |
| Home, End | First and last column of the row |
| Ctrl+Home, Ctrl+End | First and last cell of the grid |
| Down, Up | Next and previous row. Up from the first row moves to the header; Down from the header moves to the first row |
| PageDown, PageUp | By a viewport of rows |
| Space | Toggles the row (multiple), or selects it (single). On a header cell, sorts by the column |
| Shift+Space | Selects the rows from the anchor row to the focused row (multiple) |
| Shift+Down, Up, PageDown, PageUp | Extends the selection from the anchor (multiple). A single selection follows the move |
| Ctrl+Down, Up and the others | Moves the focus without selecting (multiple). The anchor stays |
| Ctrl+A | Selects every row (multiple) |
| Enter | Activates the row (`OnActivate`). On a header cell, sorts by the column. On an editable cell, starts an edit. On an interactive cell, moves focus to its first control |
| F2 | On an editable cell, starts an edit. On an interactive cell, moves focus to its first control; in a control, returns to the grid |
| Tab, Shift+Tab | In an interactive cell, moves through its controls and wraps inside the cell. In an edit, saves and starts an edit of the next (previous) editable cell of the row; at the last one the edit ends and focus stays in the grid |
| Escape | In a control of an interactive cell, returns to the grid. In an edit, cancels it |
| A printable character | On an editable cell with the text field, starts an edit with the character, which replaces the text as in a spreadsheet |
| Ctrl+Left, Ctrl+Right | On a resizable header cell, narrows or widens the column by 8 logical pixels |
| Ctrl+Shift+Left, Ctrl+Shift+Right | On a movable header cell, moves the column one place among the columns of its pin, and announces its new place ("Status moved to position 2 of 5", `Strings.ColumnMoved`). On a header cell that cannot move, resizes as Ctrl does |
| Shift+F10, Menu | Opens the context menu of the focused cell, or of the header cell (`ContextMenu`, `HeaderContextMenu`) |

With `CellSelection` the keys select cells:

| Key | Action |
| --- | --- |
| Arrows, Home, End, PageDown, PageUp | Move to the cell and select it |
| Ctrl with a move | Moves the focus without selecting. The anchor stays |
| Shift with a move | Selects the rectangle of cells from the anchor cell to the focused one (multiple) |
| Space | Toggles the focused cell (multiple), or selects it (single) |
| Shift+Space | Selects the cells of the focused row (multiple) |
| Ctrl+Space | Selects the cells of the focused column (multiple) |
| Ctrl+A | Selects every cell (multiple) |

**Interactive and editable cells.** A cell that holds controls, or an editor, is still the grid's one Tab stop: the controls and the editor are not in the Tab order, and the grid's keys reach them through the focus. The model follows the WAI-ARIA grid pattern (https://www.w3.org/WAI/ARIA/apg/patterns/grid/), which describes two states without naming them: grid navigation, in which the grid holds focus, and the cell's own widgets, which Enter or F2 move focus into. In the second state Tab and Shift+Tab move to the next and previous widget, which the pattern lets wrap inside the cell or leave the grid; this grid wraps inside the cell. Escape (the pattern's "restores grid navigation") or F2 returns to grid navigation on the same cell. The keys a control uses (the arrows of a text field, Enter on a button) belong to the control. An edit saves on Enter and on Tab, and when focus leaves its editor (a click elsewhere in the grid), as a spreadsheet does. Escape is the only cancel. This blur save is a choice of this grid, not a rule of the pattern.

![The Status header under the pointer, which fills its whole header cell, and a clicked cell in a selected row, whose text is not bold](screenshots/data-grid.png)

![An editable cell with its editor open](screenshots/data-grid-editing.png)

![A save that failed: the editor keeps the invalid frame, and the message is a popover under it](screenshots/data-grid-validation.png)

![A status chosen in a Select editor (a custom editor)](screenshots/data-grid-choice.png)

![A failed save at 225 percent text: the cell clips to its column, and the popover stays on the window](screenshots/data-grid-validation-225.png)

![The Open button of a cell, focused from the keyboard](screenshots/data-grid-actions.png)

A pointer press on a control focuses it and operates it, and moves the cursor to the cell; it does not select or activate the row. A double-click on an editable cell starts an edit and does not activate the row; elsewhere a double-click activates the row, as before.

An editing cell's editor is a text field named for its column, so assistive technology announces it as an edit field and reports its value. A save that fails keeps the editor open with the invalid frame of its `Editor` style. The message of the failure is a popover under the cell (`EditorMessage`, which flips above at the bottom edge of the window and is not clipped by the grid); it is the editor's description for assistive technology, and it is announced assertively.

**Custom editors.** `DataColumn.Editor` replaces the text field of an edit with what it returns, such as a `ui.Select`: `Editor(row, value, change, commit, cancel) ui.Element`. `value` is the pending value (it starts as `Value`, or `Text`); `change(v)` sets it; `commit()` saves it through `Edit`; `cancel()` ends the edit. The editor needs a focusable control, which takes focus when the editor is first painted. Keys go to the editor's controls first: a control that takes Enter or Escape keeps it (an open list closes with Escape). What the controls do not take is the grid's: Enter and Tab save the pending value, Escape cancels, and a blur saves it. Typing does not start a custom editor; Enter and F2 do.

**Set Value.** An editable cell has the Value pattern, whose value is the cell's text (or `Value`). Its SetValue runs `Edit` on the UI thread, without an edit: a value that `Edit` refuses is announced assertively and does not change the cell. A cell that is not editable has no Value pattern, since Microsoft's DataItem guidance requires it only for an editable primary text (https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-supportdataitemcontroltype). Focus stays in the editor, which takes the place of the cell's content until the edit ends. The data is the app's: `Edit` updates the app's rows, and the grid draws them again, keeping the focus and the selection on their keys.

An editing row is laid out only while it is in view, as the other rows are. If its row scrolls out of view, the edit, its text and its focus are kept, and the editor is laid out again when the row comes back. While it is out of view the focus stays in the editor, so keys typed then go to that editor; a grid with an edit open should be scrolled back to its row before typing.

The cell's `Text`, when it is set, is still the cell's accessible name in an interactive or editable column; the controls are listed under the cell.

Alt is not used by the grid, so the host keeps its menu and navigation keys.

**Mouse.** A click selects a row: Ctrl toggles it and Shift extends the selection from the anchor. With `CellSelection` a click selects a cell, Ctrl+click toggles it, and Shift+click selects the rectangle from the anchor cell. A double-click activates the row. A click on a header cell sorts. Dragging the edge of a resizable header cell resizes its column, and the cursor is the east-west resize cursor over the edge. A double-click on the edge fits the column to the text of the rows in view. Dragging a movable header cell moves its column: a line (`DropIndicator`) shows the side of the header under the pointer that it lands on, and a press that does not move 4 px is still a click that sorts. A right-press opens the context menu (see `ContextMenu`).

**Pinned columns.** A pinned column stays at its end of the grid while the others scroll under it. Its cells are filled with the grid's background and then their row's fill, so a hovered, selected or striped row shows through them; `CellPinned` styles them, and `PinnedEdge` is the cell at the inner edge of the pinned columns, with a 1 px border by default. A column that the focus moves to, or that `ScrollToColumn` names, is scrolled clear of the pinned columns.

**Many columns.** When the columns are wider than the grid, the rows build only the columns in view, two more on each side, the pinned columns, the focused column and the one being edited; the rest of each row is a spacer hidden from assistive technology. The header is built whole, so every column header is reported. This does not apply while the columns are narrower than the grid, where `Grow` shares out the spare width.

**Sorting.** A sort of 100,000 rows takes about 8 ms in the benchmark (`BenchmarkDataGridSort100k`), which is under the 50 ms at which a sort would move to a worker thread, so the sort runs on the UI thread. Only one column sorts at a time: Shift+click does not add a second sort (see `docs/accessibility-design-phases-i-m.md`, Phase I).

**Accessibility.** The grid is a `DataGrid` control with the Grid and Table patterns. Its `RowCount` is every row of the data, built or not, and its `ColumnCount` is the number of columns. Each cell is a `DataItem` with the GridItem and TableItem patterns, and each row is a `ListItem` with the SelectionItem pattern. Each header cell is a `HeaderItem` that Invoke sorts, with its sort state in its name ("Duration, sorted descending") and in its ItemStatus. Selection uses the Selection pattern, which says whether it takes many items; with `CellSelection` the cells are its items, and the rows are not. The columns are numbered in the order they are shown, so a moved or pinned column changes its column number. GetItem answers only for the cells that are built: a row out of view, or a column out of view in a grid wider than its view, gives nothing, and the client scrolls the grid to it. See `docs/accessibility-design-phases-i-m.md`, Phase I.
