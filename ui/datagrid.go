package ui

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/text"
)

// DataColumn is one column of a DataGrid. Its cells show the text of each row, or
// what Cell returns, and its header shows Header. Set Compare to let the user
// sort by the column, and Resizable to let the user resize it.
type DataColumn[T any] struct {
	// Header is the text of the header, which is the column's name for
	// assistive technology.
	Header string
	// Key identifies the column in a SortState and in the widths the user sets.
	// It is Header when empty, so it must be unique among the columns.
	Key string
	// Text is the text of each cell. It is also the cell's accessible name.
	Text func(row T) string
	// Cell replaces the text of each cell with what it returns (a render slot).
	// The content is hidden from assistive technology, so give Text as well. The
	// content fills the height of its row, so a canvas sized with Full is as tall as
	// the row. CellStyle sets the look of this column's cells.
	Cell func(row T) Element
	// CellStyle is merged over the Cell part for the cells of this column, after the
	// grid's style, so one column can differ: a graph column clears its padding with
	// Clear: ClearPadding. Under high contrast it is plain (see plainStyle).
	CellStyle Style
	// RenderHeader replaces the text of the header with what it returns, for
	// sighted users. The state's Selected is set while the column is sorted.
	RenderHeader func(label string, s PartState) Element
	// Compare orders two rows as strings.Compare does, for sorting. Nil means the
	// column cannot be sorted. A sort is stable: rows that compare equal keep
	// their order.
	Compare func(a, b T) int
	// Width is the width of the column in logical pixels. Zero means 120.
	Width float32
	// MinWidth is the narrowest the user can resize the column to, and MaxWidth
	// the widest (zero means no limit). A zero MinWidth means 40.
	MinWidth, MaxWidth float32
	// Grow is the share of the grid's spare width that the column takes when the
	// columns are narrower than the grid. Zero means the column keeps its width.
	Grow float32
	// Resizable lets the user resize the column, by the edge of its header and
	// with Ctrl+Left and Ctrl+Right on its header (see DataGrid).
	Resizable bool
	// Movable lets the user move the column, by dragging its header and with
	// Ctrl+Shift+Left and Ctrl+Shift+Right on its header (see DataGrid and
	// DataGridElement.ColumnOrder). A column moves among the columns of its Pin.
	Movable bool
	// Pin keeps the column at the start or the end of the grid as the other columns
	// scroll sideways. Zero (PinNone) lets it scroll.
	Pin ColumnPin
	// Group names a group of columns, such as "Address" over Street and City. Columns
	// next to each other with the same Group share one cell, of their width, in a row
	// above the header; a group that a pin or a move splits is drawn in pieces. It is
	// the description of each column's header for assistive technology.
	Group string
	// Align places the text of the column's cells and header: Start (the zero
	// value), Center or End.
	Align Alignment
	// Interactive lets the content of Cell hold controls, such as buttons. The
	// content is not hidden from assistive technology. The controls are not Tab
	// stops: Enter or F2 on the cell moves focus to its first control, Tab and
	// Shift+Tab move through them, and Escape or F2 returns to the grid (see
	// DataGrid).
	Interactive bool
	// Edit makes the cells editable. It saves value as the row's text, or returns an
	// error that says why the value is not accepted; the editor then stays open with
	// the error. The app keeps the data, so it updates its rows in Edit.
	Edit func(row T, value string) error
	// Value is the text that an edit starts with, when it differs from Text, such as
	// the raw number of a formatted one. Nil means Text.
	Value func(row T) string
	// CopyText is the text of a cell when the selection is copied (see
	// DataGridElement.CopyText), when it differs from Text, such as the full value of
	// a shortened one. Nil means Text.
	CopyText func(row T) string
	// Editor replaces the text field of an edit with what it returns, such as a
	// Select, a NumberInput or a CheckBox. value is the pending value, which starts
	// as Value (or Text); change sets the pending value; commit saves it through Edit
	// (and an error keeps the editor open); cancel ends the edit. The grid's keys
	// and focus work as for the text field (see DataGrid): the editor's controls get
	// their keys first, and Enter, Tab and a blur save the pending value, Escape
	// cancels. Give the editor a focusable control, or it cannot take focus.
	Editor func(row T, value string, change func(string), commit, cancel func()) Element
}

// ColumnPin keeps a column of a DataGrid in view as the columns scroll sideways:
// PinStart keeps it at the start of the grid and PinEnd at the end. The pinned
// columns are shown before and after the others, in their order.
type ColumnPin uint8

const (
	PinNone  ColumnPin = iota // the column scrolls with the others
	PinStart                  // the column stays at the start of the grid
	PinEnd                    // the column stays at the end of the grid
)

// key is the column's identity in a SortState and in the widths.
func (c DataColumn[T]) key() string {
	if c.Key != "" {
		return c.Key
	}
	return c.Header
}

// width is the column's width, given the widths the user has set.
func (c DataColumn[T]) width(ws map[string]float32) float32 {
	if w, ok := ws[c.key()]; ok {
		return w
	}
	if c.Width > 0 {
		return c.Width
	}
	return 120
}

// minWidth is the narrowest width the column can be resized to.
func (c DataColumn[T]) minWidth() float32 {
	if c.MinWidth > 0 {
		return c.MinWidth
	}
	return 40
}

// SortState is the sort of a DataGrid: the rows ordered by the column with the
// key Key, in descending order when Desc is set. An empty Key leaves the rows in
// the order they were given.
type SortState struct {
	// Key is the key of the column that the grid is sorted by.
	Key string
	// Desc is true when the sort is descending.
	Desc bool
}

// SortBy returns the Compare of a DataColumn that orders the rows by key, with
// cmp.Compare. The sort is reversed by SortState.Desc, so there is no SortByDesc.
//
//	ui.DataColumn[Run]{Header: "Cost", Text: ..., Compare: ui.SortBy(func(r Run) float32 { return r.Cost })}
func SortBy[T any, K cmp.Ordered](key func(T) K) func(a, b T) int {
	return func(a, b T) int { return cmp.Compare(key(a), key(b)) }
}

// SortByText returns the Compare of a DataColumn that orders the rows by the text
// key, without regard to case: "api" sorts with "Billing". Text with the same
// lowercase form is ordered by its case, so the order is total. The text is
// compared by its Unicode lowercase form, not by a locale's collation.
func SortByText[T any](key func(T) string) func(a, b T) int {
	return func(a, b T) int {
		x, y := key(a), key(b)
		if r := strings.Compare(strings.ToLower(x), strings.ToLower(y)); r != 0 {
			return r
		}
		return strings.Compare(x, y)
	}
}

// SelectionMode says how many rows the user can select at once.
type SelectionMode uint8

const (
	SelectionNone     SelectionMode = iota // rows cannot be selected
	SelectionSingle                        // one row at a time
	SelectionMultiple                      // any number of rows, with Ctrl and Shift
)

// DataGridDensity is the height of the rows and the header: a Comfortable row is
// 36 logical pixels high, a Compact one 26, both at the Text size setting.
type DataGridDensity uint8

const (
	// DataGridComfortable is the default row height, with room around the text.
	DataGridComfortable DataGridDensity = iota
	// DataGridCompact is a smaller row height, for more rows on the screen.
	DataGridCompact
)

// DataGridStyle is the look of the parts of a DataGrid. A part's style merges
// over the built-in defaults, then over the theme's DataGrid, then over the
// Styles of the instance; see docs/styling.md. Grid is the scroll container of
// the rows, and the header scrolls with it. Header is the row of header cells,
// HeaderCell each cell of it, and ResizeHandle the edge a column is resized by.
// Row is each row, RowAlternate every other row when the grid is Striped,
// RowHover a row the pointer is over, and RowSelected a selected row. Cell is each
// cell, and CellFocused the cell that has keyboard focus, in the body or the
// header. Empty is the message shown with no rows. The states are parts of their
// own: GridFocusVisible while the grid has keyboard focus, HeaderCellHover over a
// sortable header, HeaderCellSorted for the column the rows are sorted by, and
// SortIndicator is the arrow of that column.
//
// An editable cell has two more parts: CellEditing is the cell being edited, Editor
// is its text field (a FieldStyle; FrameInvalid is its frame while the last save
// failed), and EditorMessage is the popover under the cell that says why a save failed.
//
// In a tree (see Children) the first column has two parts more: TreeIndent is the
// space of each level above a row, and TreeChevron is the chevron that opens and closes
// a row with children, or the space it leaves for a row without.
//
// With RowHeaders, RowHeader is the header at the start of each row, RowHeaderSelected
// the header of a row selected whole, and Corner the cell above the row headers. A
// column selected whole has HeaderCellSelected on its header cell.
type DataGridStyle struct {
	// Grid is the frame of the grid.
	Grid Style
	// GridFocusVisible is the frame while the grid has keyboard focus.
	GridFocusVisible Style
	// Header is the header row.
	Header Style
	// HeaderCell is each cell of the header row.
	HeaderCell Style
	// HeaderCellHover is a header cell under the pointer.
	HeaderCellHover Style
	// HeaderCellSorted is the header cell of the column that the grid is sorted by.
	HeaderCellSorted Style
	// SortIndicator is the mark that shows the sort direction in a header cell.
	SortIndicator Style
	// ResizeHandle is the handle that resizes a column.
	ResizeHandle Style
	// Row is each row.
	Row Style
	// RowHover is a row under the pointer.
	RowHover Style
	// RowSelected is a selected row.
	RowSelected Style
	// RowAlternate is every other row, for banding.
	RowAlternate Style
	// Cell is each cell of a row.
	Cell Style
	// CellFocused is the cell that has keyboard focus.
	CellFocused Style
	// Empty is the message shown when the grid has no rows.
	Empty Style
	// CellEditing is a cell that is being edited.
	CellEditing Style
	// Editor is the field of the cell editor.
	Editor FieldStyle
	// EditorMessage is the message of the cell editor, such as an error.
	EditorMessage Style
	// DropIndicator is the line that shows where a dragged column lands.
	DropIndicator Style
	// CellPinned is each cell of a pinned column (see DataColumn.Pin).
	CellPinned Style
	// PinnedEdge is the cell, in the body or the header, at the inner edge of the
	// pinned columns, whose border separates them from the columns that scroll.
	PinnedEdge Style
	// HeaderGroup is the cell of a group of columns, above their header cells (see
	// DataColumn.Group).
	HeaderGroup Style
	// CellSelected is a selected cell, when the grid selects cells (see CellSelection).
	CellSelected Style
	// TreeIndent is the space of each level above a row, in the first column of a tree
	// (see Children).
	TreeIndent Style
	// TreeChevron is the chevron that opens and closes a row with children, in the first
	// column of a tree. A row without children leaves its space empty.
	TreeChevron Style
	// RowHeader is the header at the start of each row, which shows the row's number
	// (see RowHeaders).
	RowHeader Style
	// RowHeaderSelected is the header of a row whose every cell is selected, or of a
	// selected row.
	RowHeaderSelected Style
	// HeaderCellSelected is the header cell of a column whose every cell is selected.
	HeaderCellSelected Style
	// Corner is the cell at the start of the header row, above the row headers, which
	// selects every cell (see RowHeaders).
	Corner Style
}

// dataGridGutter is the width kept clear at the right of a DataGrid's body for its
// vertical scrollbar, in logical pixels.
const dataGridGutter = 12

// dataGridRowHeight is the height of a row or a header at the Text size setting 100 percent.
func dataGridRowHeight(d DataGridDensity) float32 {
	if d == DataGridCompact {
		return 26
	}
	return 36
}

// dataGridDefaults sets st to the built-in look of a DataGrid, from the colors of t.
// Under high contrast the selected row is the Highlight color with HighlightText
// on it, and the focus ring is kept.
func dataGridDefaults(st *DataGridStyle, t *Theme, d DataGridDensity, textFactor float32) {
	h := dataGridRowHeight(d) * textFactor
	line := Edges{Bottom: 1}
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	*st = DataGridStyle{
		Grid:             Style{Grow: 1, Background: t.Surface, BorderWidth: 1, BorderColor: t.Border, TextColor: t.Text},
		GridFocusVisible: Style{Shadows: []Shadow{ring}},
		Header: Style{Direction: Row, Align: Center, MinHeight: Px(h), Background: t.Panel,
			BorderEdges: line, BorderColor: t.Border, TextColor: t.TextMuted, FontWeight: FontSemibold},
		// A header cell takes the height of the header row, as a row's cells take the height
		// of their row, so its hover and sorted fills cover the whole cell.
		HeaderCell:       Style{Direction: Row, Align: Center, AlignSelf: Stretch, Gap: 4, Padding: Edges{Left: 8, Right: 8}, NoShrink: true},
		HeaderCellHover:  Style{Background: t.SurfaceHover},
		HeaderCellSorted: Style{TextColor: t.Text},
		SortIndicator:    Style{TextColor: t.Text},
		ResizeHandle:     Style{Width: Px(6), Cursor: input.CursorResizeEW},
		Row: Style{Direction: Row, Align: Center, MinHeight: Px(h), NoShrink: true,
			BorderEdges: line, BorderColor: t.Border, TextColor: t.Text},
		RowHover:     Style{Background: t.SurfaceHover},
		RowSelected:  Style{Background: dataGridSelectionFill(t)},
		RowAlternate: Style{Background: t.Panel},
		// A cell takes the height of its row even with no content, so an empty cell
		// (an interactive column's cell with no control) still has a bounding box.
		// A cell clips what it draws to its column: content wider than the column is cut
		// off at its edge, and never draws over the next column.
		Cell: Style{Direction: Row, Align: Center, AlignSelf: Stretch, Padding: Edges{Left: 8, Right: 8}, NoShrink: true,
			OverflowX: OverflowHidden, OverflowY: OverflowHidden},
		CellFocused: Style{Shadows: []Shadow{ring}},
		Empty:       Style{TextColor: t.TextMuted, Padding: All(16)},
		CellEditing: Style{Background: t.Surface},
		EditorMessage: Style{Direction: Row, Align: Center, Padding: XY(8, 3), Radius: t.Radius,
			Background: t.Surface, BorderWidth: 1, BorderColor: t.Danger, TextColor: t.Danger},
		DropIndicator: Style{Width: Px(2), Background: t.Accent},
		PinnedEdge:    Style{BorderColor: t.Border},
		HeaderGroup: Style{Direction: Row, Align: Center, Justify: Center, AlignSelf: Stretch, Padding: Edges{Left: 8, Right: 8},
			NoShrink: true, OverflowX: OverflowHidden, BorderEdges: Edges{Bottom: 1}, BorderColor: t.Border},
		CellSelected: Style{Background: dataGridSelectionFill(t)},
		// A tree's indent and chevron are 16 logical pixels wide at the Text size 100 percent.
		TreeIndent:  Style{Width: Px(16 * textFactor), NoShrink: true},
		TreeChevron: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(16 * textFactor), NoShrink: true, TextColor: t.TextMuted},
		// A row header is a narrow column of numbers at the start of the rows, in the
		// header's colors, and stays in view as the columns scroll sideways.
		RowHeader: Style{Direction: Row, Align: Center, Justify: End, AlignSelf: Stretch, Padding: Edges{Left: 4, Right: 8},
			NoShrink: true, OverflowX: OverflowHidden, Background: t.Panel, BorderEdges: Edges{Right: 1}, BorderColor: t.Border,
			TextColor: t.TextMuted},
		// A header of a row or a column selected whole is the solid accent, stronger than a
		// selected cell's fill, so that the selected rows and columns are found at a glance.
		RowHeaderSelected:  Style{Background: t.Accent, TextColor: t.AccentText},
		HeaderCellSelected: Style{Background: t.Accent, TextColor: t.AccentText},
		Corner: Style{AlignSelf: Stretch, NoShrink: true, Background: t.Panel, BorderEdges: Edges{Right: 1},
			BorderColor: t.Border},
	}
	if t.HighContrast {
		st.RowSelected = Style{Background: t.Accent, TextColor: t.AccentText}
		st.CellSelected = Style{Background: t.Accent, TextColor: t.AccentText}
		st.RowHeaderSelected = Style{Background: t.Accent, TextColor: t.AccentText}
		st.HeaderCellSelected = Style{Background: t.Accent, TextColor: t.AccentText}
		st.EditorMessage.BorderColor, st.EditorMessage.TextColor = t.Text, t.Text
	}
}

// dataGridSelectionFill is the fill of a selected row or cell: the accent at 0.28
// alpha on a dark theme and at 0.20 on a light one. They are the strongest at which the
// built-in themes keep the focus ring (FocusRing) of a cell at 3:1 on the fill, over a
// plain, striped or hovered row, and muted text (a tree's chevron) at 4.5:1. Against
// the row under them they are about 1.36:1, where AccentSoft was 1.2:1, so a selected
// range is seen at a glance; text keeps more than 10:1 on them (see TestContrast).
func dataGridSelectionFill(t *Theme) Color {
	if t.Dark {
		return t.Accent.Alpha(0.28)
	}
	return t.Accent.Alpha(0.20)
}

// resolveDataGrid sets st to the look of a DataGrid: the defaults, merged with the
// theme's DataGrid and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveDataGrid(ctx *Context, d DataGridDensity, textFactor float32, inst, st *DataGridStyle) {
	t := ctx.Theme()
	dataGridDefaults(st, t, d, textFactor)
	st.layered(ctx,
		themeLayer(&t.DataGrid), instanceLayer(inst))
}

// DataGridElement is a table of rows and columns, for business data. Only the rows
// in view are built, so a grid of 100,000 rows costs what its visible part costs.
// When the columns are wider than the grid, the rows build only the columns in view
// as well, with the pinned columns, the focused one and the one being edited; the
// header is built whole. As for the rows, assistive technology finds by row and
// column only the cells that are built, and the grid reports all its columns.
// Create one with DataGrid.
//
// The grid is one Tab stop. Its keys act on the focused cell, and they follow the
// Windows list and grid conventions and the WAI-ARIA grid pattern
// (https://www.w3.org/WAI/ARIA/apg/patterns/grid/):
//
//   - Left and Right move to the previous and next column. Home and End move to
//     the first and last column of the row; Ctrl+Home and Ctrl+End move to the first
//     and last cell of the grid.
//   - Down and Up move to the next and previous row. Up from the first row moves to
//     the header, and Down from the header moves to the first row.
//   - PageDown and PageUp move by a viewport of rows.
//   - In a single or multiple selection a plain move selects the row it reaches. With
//     Ctrl the focus moves without selecting, as in the Windows list views.
//   - Shift with Up, Down, PageUp or PageDown extends the selection from the anchor
//     row (multiple selection only).
//   - Space toggles the focused row (multiple selection), or selects it (single).
//     Shift+Space extends the selection to the row, and Ctrl+A selects every row.
//   - When the grid selects cells (see CellRanges), the moves select cells in the
//     same way: a plain move selects the cell, Ctrl moves the focus only, and Shift
//     with an arrow, Home, End, PageUp or PageDown extends a rectangle from the
//     anchor cell; Ctrl+Shift extends it and keeps the rest of the selection. Space
//     toggles the focused cell, Shift+Space selects its row, Ctrl+Space its column,
//     and Ctrl+A every cell. After Ctrl+Space, Shift extends whole columns, and after
//     Shift+Space whole rows.
//   - Ctrl+C and Ctrl+Insert copy the selection as tab-separated text, and
//     Ctrl+Shift+C copies it with the headers of its columns (see CopyText). They are
//     the actions of DataGridActions, bound in the grid's key context, "DataGrid",
//     except while a control or an editor in a cell has focus (see DataGridActions).
//   - With RowHeaders, Left from the first column moves to the row's header, and Up
//     from the first row's header to the corner (see RowHeaders).
//   - Enter activates the focused row, and calls OnActivate with it.
//   - On a header cell, Enter and Space sort by its column (see Sort); with
//     HeaderSelects, Space selects the column instead. On a
//     resizable header cell, Ctrl+Left and Ctrl+Right narrow and widen the column by
//     8 logical pixels. On a movable header cell (see DataColumn.Movable),
//     Ctrl+Shift+Left and Ctrl+Shift+Right move the column one place, and its new
//     place is announced; on other header cells they resize, as Ctrl does.
//   - Shift+F10 or the Menu key opens the context menu of the focused cell, or of
//     the header cell (see ContextMenu and HeaderContextMenu). Alt is not used by
//     the grid, so that the host keeps its menu and navigation keys.
//   - On an interactive cell (see DataColumn.Interactive), Enter or F2 moves focus
//     to its first control. In the cell, Tab and Shift+Tab move through its
//     controls and wrap at the ends, as the WAI-ARIA grid pattern allows; Escape or
//     F2 returns focus to the cell, which is still the grid's one Tab stop. Keys
//     that a control uses (the arrows of a text field, Enter on a button) are its own.
//   - On an editable cell (see DataColumn.Edit), Enter or F2 starts an edit, and so
//     does typing a printable character, which replaces the text as in a
//     spreadsheet. Enter saves, and Escape cancels. Tab saves and starts an edit of
//     the next editable cell of the row; at the last one the edit ends and focus
//     stays in the grid. Leaving the editor, by a click elsewhere or by focus
//     moving on, saves the edit, as a spreadsheet does; a save that fails keeps the
//     editor open with its error. Only Escape cancels.
//
// A click selects a row (Ctrl toggles it, Shift extends the selection), or a cell
// when the grid selects cells (Shift selects the rectangle from the anchor, and
// Ctrl+Shift adds it to the rest of the selection; a drag selects the rectangle it
// covers, and a drag held past an edge of the rows in view scrolls them, or the
// columns, and selects what comes into view), and a
// double-click activates it, except on an editable cell, where a double-click
// starts an edit. A click on a control in a cell (see DataColumn.Interactive)
// focuses and operates the control; it moves the cursor to the cell but does not
// select or activate the row. A click on a header sorts by its column: ascending,
// then descending, then not sorted; with HeaderSelects it selects the column, and the
// header's arrow sorts. Dragging the edge of a resizable header
// resizes its column, and a double-click on the edge fits the column to the rows
// in view. Dragging a movable header moves its column: a line shows the side of the
// header under the pointer that it lands on. A column header stays at the top of
// the rows as they scroll, and the header scrolls with the rows when the columns are
// wider than the grid. A pinned column (see DataColumn.Pin) stays at the start or
// the end of the grid as the others scroll under it, and a column that the focus
// moves to is scrolled clear of the pinned ones. Columns with a Group share a cell in
// a row above the header; the keys do not reach that row, and assistive technology
// reads the group as the description of each column's header.
//
// The columns are shown in the order of ColumnOrder, and assistive technology numbers
// them in that order. The order and the widths the user sets can be kept in signals
// (ColumnOrder and ColumnWidths), and OnColumnsChange reports each change the user
// makes to them, so that an app can save and restore them. A DataGridHandle scrolls
// to a row or a column, and focuses a cell, by their keys. LoadMore asks for more
// rows as the user nears the end of them, and LoadingMore shows a row at the end
// while they load.
//
// Sorting orders the rows without changing their keys, and the focus and the
// selection follow their rows by key, so a sort, a scroll, new rows or a saved
// edit keep them. Sorting is stable, and it runs on the UI thread: a sort of 100,000
// rows takes about 8 ms (see docs/accessibility-design-phases-i-m.md, Phase I). Keys must be unique.
//
// With Children the rows form a tree. The rows given to DataGrid are its top level, and
// a row with children shows a chevron in the first column, which opens and closes it.
// On a cell of that column, Right opens a closed row and, on an open row, moves to its
// first child; Left closes an open row and, on a row that is closed or has no children,
// moves to its parent. Elsewhere Left and Right move between columns, as above. The
// * key opens the closed rows among the siblings of the focused row. A row's chevron
// is hidden from assistive technology: the row reports its level, its place among its
// siblings and whether it is open. The rows that are shown are counted for assistive
// technology, and Ctrl+A selects them. A collapse that hides the focused row moves the
// focus to the row above it that is shown. A collapse that hides a row being edited
// saves the edit, as a blur does.
//
// An editing row is built only while it is in view, as the other rows are, because
// the rows of a grid are one window of the list. When the row scrolls out of view its
// editor is not laid out, but the edit, its text and its focus are kept, and the
// editor is laid out again when the row comes back (see docs/accessibility-design-phases-i-m.md).
type DataGridElement[T any] struct {
	rows         []T
	key          func(T) string
	cols         []DataColumn[T]
	name         string
	keyStr       string
	style        Style
	density      DataGridDensity
	striped      bool
	mode         SelectionMode
	sel          *Signal[[]string]
	sort         *Signal[SortState]
	onActivate   func(row T)
	loading      bool
	empty        Element
	styles       DataGridStyle
	focusHandle  *FocusHandle // see FocusHandle
	size         sizing       // see the sizing methods
	order        *Signal[[]string]
	widths       *Signal[map[string]float32]
	onColumns    func(ColumnState)
	gridHandle   *DataGridHandle
	rowMenu      func(row T, column string) []MenuItem
	headerMenu   func(column string) []MenuItem
	hasMore      bool
	loadMore     func()
	loadingMore  bool
	movable      bool
	cellOn       bool // the user selects cells (see CellSelection)
	cellMode     SelectionMode
	cells        *Signal[[]CellKey]
	ranges       *Signal[[]CellRange] // the cells selected, as ranges (see CellRanges)
	selBoth      bool                 // both Selection and CellSelection were called, for the warning
	children     func(row T) []T
	childrenOnly bool              // see SortChildrenOnly
	expanded     *Signal[[]string] // the open rows of a tree, when the app keeps them (see Expanded)
	onExpand     func(key string)
	onCollapse   func(key string)
	headSelects  bool // a click on a column's header selects the column (see HeaderSelects)
	rowHeaders   bool // see RowHeaders
	copyText     func(c DataGridCopy[T]) string
}

// HeaderSelects makes a click on a column's header select the column, as in a
// spreadsheet, when the grid selects cells (see CellRanges): Ctrl+click adds the
// column to the selection or takes it out, Shift+click selects the columns from the
// anchor's, and a drag across the headers selects the columns it crosses. The sort
// moves to the arrow in the header: a sortable column shows it while not sorted too, and
// a click on it sorts. Enter on a header cell sorts, and Space selects its column, with
// Ctrl and Shift as for the click. A drag that starts on a movable column (see
// DataColumn.Movable) that is selected whole moves the column. Without it a click on a
// header sorts, and a grid that selects rows ignores it.
func (g *DataGridElement[T]) HeaderSelects() *DataGridElement[T] { g.headSelects = true; return g }

// RowHeaders shows a header at the start of each row, with the row's number in the order
// shown, and a corner above them that selects every cell (or every row). A click on a
// row's header selects the row: Ctrl+click adds it to the selection or takes it out,
// Shift+click selects the rows from the anchor's, and a drag down the headers selects
// the rows it crosses. In a grid that selects rows the header acts as a click on its row.
// The headers stay in view as the columns scroll sideways. The keys reach them: Left
// from the first column moves to the row's header, where Space selects the row (with
// Ctrl and Shift as for the click), Shift+Up and Shift+Down extend the rows, and Up
// from the first row's header moves to the corner, where Space selects every cell.
// The headers are header items of their rows for assistive technology, and the corner
// is a button named "Select all".
func (g *DataGridElement[T]) RowHeaders() *DataGridElement[T] { g.rowHeaders = true; return g }

// Movable lets the user move every column of the grid, as DataColumn.Movable does
// for one column.
func (g *DataGridElement[T]) Movable() *DataGridElement[T] { g.movable = true; return g }

// dgColDrag is the payload of a column's header that is dragged: the grid it
// belongs to, and the key of the column.
type dgColDrag struct {
	grid any
	key  string
}

// dgHeadOps are the actions of the header cells beyond the sort and the resize:
// changed reports a change of the layout of the columns, widthOf is the width of a
// column at the Text size setting 100 percent, movable says whether a column can be
// moved, accepts whether the column from can be put beside the column to, and drop
// puts it there (after it when after is set). mark is where a dragged column would
// land, setMark sets it, and stopDrag stops a press from dragging the header.
// post runs a function at the start of the next frame.
type dgHeadOps[T any] struct {
	changed  func()
	widthOf  func(cl DataColumn[T]) float32
	movable  func(cl DataColumn[T]) bool
	accepts  func(from, to string) bool
	drop     func(from, to string, after bool)
	mark     dgColMark
	setMark  func(dgColMark)
	stopDrag func()
	post     func(func())
	// startN and endN are the numbers of the columns pinned to the start and the end.
	startN, endN int
	// runs are the pieces of the row of the groups of columns, or nil without it.
	runs []dgGroupRun
	// The selection from the headers (see HeaderSelects and RowHeaders). selects says a
	// press on a header cell selects its column, and pickCol selects it. cellSel says the
	// grid selects cells, and whole reports whether column c is selected whole. rowHeadW
	// is the width of the corner, zero without row headers; corner selects everything,
	// and onCorner says the cursor is on it.
	selects  bool
	cellSel  bool
	pickCol  func(c int, mods input.Modifiers)
	whole    func(c int) bool
	rowHeadW float32
	corner   func()
	onCorner bool
}

// dgGroupRun is a piece of the row of the groups of columns: the columns [from, to)
// that are next to each other with the same group (empty for columns with none)
// and the same pin.
type dgGroupRun struct {
	group    string
	pin      ColumnPin
	from, to int
}

// groupRuns returns the pieces of the row of the groups of the columns cols.
func groupRuns[T any](cols []DataColumn[T]) []dgGroupRun {
	var runs []dgGroupRun
	for c, cl := range cols {
		if k := len(runs) - 1; k >= 0 && runs[k].group == cl.Group && runs[k].pin == cl.Pin {
			runs[k].to = c + 1
			continue
		}
		runs = append(runs, dgGroupRun{group: cl.Group, pin: cl.Pin, from: c, to: c + 1})
	}
	return runs
}

// dgColMark is where a dragged column would land: beside the column with the key,
// after it when after is set. It draws the drop indicator on that side.
type dgColMark struct {
	key   string
	after bool
}

// LoadMore asks fn for more rows while hasMore is true: fn runs when the rows near
// the end come into view, within a viewport of rows of the last one, or when Down,
// PageDown or Ctrl+End is pressed on the last row. It runs once for each number of
// rows, so it runs again only after the rows have changed, and it runs after the
// frame, not while the grid is built. In a tree the rows counted are every row given,
// children included, so opening or closing a row does not ask again. fn adds the rows
// to the data, with LoadingMore shown while they load.
func (g *DataGridElement[T]) LoadMore(hasMore bool, fn func()) *DataGridElement[T] {
	g.hasMore, g.loadMore = hasMore, fn
	return g
}

// LoadingMore shows a row at the end of the grid, with a progress ring and "Loading
// more rows", while on is true, and announces it when it starts. The row is not one
// of the grid's rows for assistive technology.
func (g *DataGridElement[T]) LoadingMore(on bool) *DataGridElement[T] {
	g.loadingMore = on
	return g
}

// The sizing methods give the grid's outer box a size, over its Style. See Sizing
// in docs/styling.md.
func (g *DataGridElement[T]) Width(l Length) *DataGridElement[T] { g.size.setWidth(l); return g }

// Height sets the height of the data grid.
func (g *DataGridElement[T]) Height(l Length) *DataGridElement[T] { g.size.setHeight(l); return g }

// MinWidth sets the smallest width of the data grid.
func (g *DataGridElement[T]) MinWidth(l Length) *DataGridElement[T] { g.size.setMinWidth(l); return g }

// MinHeight sets the smallest height of the data grid.
func (g *DataGridElement[T]) MinHeight(l Length) *DataGridElement[T] {
	g.size.setMinHeight(l)
	return g
}

// MaxWidth sets the largest width of the data grid.
func (g *DataGridElement[T]) MaxWidth(l Length) *DataGridElement[T] { g.size.setMaxWidth(l); return g }

// MaxHeight sets the largest height of the data grid.
func (g *DataGridElement[T]) MaxHeight(l Length) *DataGridElement[T] {
	g.size.setMaxHeight(l)
	return g
}

// Grow makes the data grid take a share of the free space on its parent's main axis.
func (g *DataGridElement[T]) Grow(f float32) *DataGridElement[T] { g.size.setGrow(f); return g }

// GridSpan sets how many columns and rows the data grid covers in its parent's grid.
func (g *DataGridElement[T]) GridSpan(cols, rows int) *DataGridElement[T] {
	g.size.setSpan(cols, rows)
	return g
}

// FocusHandle lets code focus the grid (see NewFocusHandle). A Field that wraps
// the control gives it its own handle, which takes precedence.
func (g *DataGridElement[T]) FocusHandle(h *FocusHandle) *DataGridElement[T] {
	g.focusHandle = h
	return g
}

// DataGrid creates a table of rows, with a column for each of cols. key gives
// the key of each row, which must be unique and stable: the focus, the selection
// and the sort follow it.
//
// With no size, the grid takes the height its parent leaves it, and builds the
// rows that fit in it, as a grid with a Height does.
//
//	ui.DataGrid(runs, func(r Run) string { return r.ID },
//		ui.DataColumn[Run]{Header: "Name", Text: func(r Run) string { return r.Name }, Compare: byName, Grow: 1},
//		ui.DataColumn[Run]{Header: "Status", Text: func(r Run) string { return r.Status }}).
//		Selection(ui.SelectionMultiple, picked).Sort(order)
func DataGrid[T any](rows []T, key func(T) string, cols ...DataColumn[T]) *DataGridElement[T] {
	return &DataGridElement[T]{rows: rows, key: key, cols: cols}
}

func (*DataGridElement[T]) isElement()           {}
func (g *DataGridElement[T]) elementKey() string { return g.keyStr }

// Key gives the grid a stable identity among its siblings.
func (g *DataGridElement[T]) Key(k string) *DataGridElement[T] { g.keyStr = k; return g }

// Label names the grid for assistive technology, such as "Deploy runs".
func (g *DataGridElement[T]) Label(name string) *DataGridElement[T] { g.name = name; return g }

// Style is merged into the grid's outer box: size it with Height, Grow or MaxHeight.
func (g *DataGridElement[T]) Style(s Style) *DataGridElement[T] { return g.styleOf(&s) }

func (g *DataGridElement[T]) styleOf(s *Style) *DataGridElement[T] {
	g.style.mergeFrom(s, false)
	return g
}

// LayoutSpring moves the grid to a new place in the layout as the spring s, as
// DivElement.LayoutSpring does. It is the same as Style(Style{LayoutSpring: s}).
func (g *DataGridElement[T]) LayoutSpring(s Spring) *DataGridElement[T] {
	return g.styleOf(&Style{LayoutSpring: s})
}

// Density sets the height of the rows: DataGridComfortable (the default) or DataGridCompact.
func (g *DataGridElement[T]) Density(d DataGridDensity) *DataGridElement[T] { g.density = d; return g }

// Striped draws every other row in the RowAlternate part.
func (g *DataGridElement[T]) Striped() *DataGridElement[T] { g.striped = true; return g }

// Selection sets how the user selects rows, and keeps the keys of the selected
// rows in keys: the grid sets it when the selection changes, and a value set in
// keys selects those rows. A nil keys keeps the selection inside the grid.
func (g *DataGridElement[T]) Selection(mode SelectionMode, keys *Signal[[]string]) *DataGridElement[T] {
	if g.cellOn {
		g.selBoth = true
		g.cellOn, g.cellMode, g.cells, g.ranges = false, SelectionNone, nil, nil
	}
	g.mode, g.sel = mode, keys
	return g
}

// CellKey names a cell of a DataGrid: the key of its row and the key of its column.
type CellKey struct {
	// Row is the key of the cell's row.
	Row string
	// Column is the key of the cell's column.
	Column string
}

// CellSelection lets the user select cells rather than rows, and keeps the selected
// cells in cells: the grid sets it when the selection changes, and a value set in
// cells selects those cells. A nil cells keeps the selection inside the grid. The
// cells follow their rows and columns by key, through a sort or a move.
//
// It replaces Selection: the grid selects rows or cells, not both, and the later of
// the two calls is the one that applies (the development warnings say so).
//
// The grid keeps the selection as ranges (see CellRanges), and cells is a copy of it
// with a key for each cell: Ctrl+A on 100,000 rows of 4 columns sets cells to 400,000
// keys. A grid of many rows should keep its selection with CellRanges, which never
// lists the cells; the two can be given together, and the grid keeps both up to date.
func (g *DataGridElement[T]) CellSelection(mode SelectionMode, cells *Signal[[]CellKey]) *DataGridElement[T] {
	g.cellsOf(mode)
	g.cells = cells
	return g
}

// CellRanges lets the user select cells, rows and columns, as CellSelection does, and
// keeps the selection in ranges as blocks of cells (see CellRange): the grid sets it
// when the selection changes, and a value set in ranges selects those cells. A nil
// ranges keeps the selection inside the grid. Selecting every cell, a row or a column
// adds one range, whatever the number of rows, so a grid of many rows keeps its
// selection here rather than in the cells of CellSelection.
//
// A range of every row (KeySet{All: true}), as Ctrl+A, Ctrl+Space or a column's header
// make (see HeaderSelects), has the rows that are added later too; a range of some rows
// lists their keys, and follows them through a sort, as a selection of cells does.
func (g *DataGridElement[T]) CellRanges(mode SelectionMode, ranges *Signal[[]CellRange]) *DataGridElement[T] {
	g.cellsOf(mode)
	g.ranges = ranges
	return g
}

// cellsOf makes the grid select cells in the mode mode, in place of rows.
func (g *DataGridElement[T]) cellsOf(mode SelectionMode) {
	if g.mode != SelectionNone || g.sel != nil {
		g.selBoth = true
		g.mode, g.sel = SelectionNone, nil
	}
	g.cellOn, g.cellMode = true, mode
}

// Sort keeps the sort in s: the grid sets it when the user sorts, and the rows are
// ordered by it. A nil s keeps the sort inside the grid. A column sorts only when
// its Compare is set.
func (g *DataGridElement[T]) Sort(s *Signal[SortState]) *DataGridElement[T] { g.sort = s; return g }

// Children makes the rows a tree: the rows given to DataGrid are the top level, and fn
// returns the children of a row, nil or empty for a row with none. Keys must be unique
// across the whole tree. A row with children shows a chevron that opens and closes it,
// and its children are shown under it while it is open (see Expanded). The rows and
// their children are read on each build of the grid, as a flat grid reads its rows, so a
// change made in place shows; fn therefore runs for each row on each build, and should be
// cheap, such as a field read. The grid computes no totals: a group row is an ordinary row
// that the app builds, with its summed values.
//
// The children of a row are sorted within their level by the sort (see Sort), so a
// group sorts among the groups and a child among its siblings.
//
//	ui.DataGrid(groups, func(g Group) string { return g.Key },
//		ui.DataColumn[Group]{Header: "Name", Text: func(g Group) string { return g.Name }},
//		ui.DataColumn[Group]{Header: "Memory", Text: func(g Group) string { return g.Memory }}).
//		Children(func(g Group) []Group { return g.Members })
func (g *DataGridElement[T]) Children(fn func(row T) []T) *DataGridElement[T] {
	g.children = fn
	return g
}

// SortChildrenOnly keeps the top rows of a tree in the order they are given, and
// sorts only the rows under them, each among its siblings. The top rows are then
// sections, such as Task Manager's Apps, Background processes and Windows processes,
// which stay in their place while the rows in them sort. It applies to a grid with
// Children; a grid without them sorts all its rows, as before.
func (g *DataGridElement[T]) SortChildrenOnly() *DataGridElement[T] {
	g.childrenOnly = true
	return g
}

// Expanded keeps the keys of the open rows of a tree in keys: the grid sets it when a
// row is opened or closed, and a value the app sets in keys opens those rows and closes
// the others. A nil keys keeps the open rows inside the grid. The keys are kept in the
// order the rows were opened; closing a row keeps the keys of the rows below it, so
// opening it again shows them open. A DataGridHandle that scrolls to a row opens the
// rows above it (see ScrollToRow).
func (g *DataGridElement[T]) Expanded(keys *Signal[[]string]) *DataGridElement[T] {
	g.expanded = keys
	return g
}

// OnExpand runs fn with the key of a row after it is opened, by a click on its chevron,
// a key, the accessibility actions or a DataGridHandle. It does not run for the keys the
// app sets in Expanded.
func (g *DataGridElement[T]) OnExpand(fn func(key string)) *DataGridElement[T] {
	g.onExpand = fn
	return g
}

// OnCollapse runs fn with the key of a row after it is closed, from the same sources as
// OnExpand. It does not run for the keys the app sets in Expanded.
func (g *DataGridElement[T]) OnCollapse(fn func(key string)) *DataGridElement[T] {
	g.onCollapse = fn
	return g
}

// OnActivate runs fn with the row that the user activates, by Enter or a
// double-click.
func (g *DataGridElement[T]) OnActivate(fn func(row T)) *DataGridElement[T] {
	g.onActivate = fn
	return g
}

// Loading shows a progress ring over the rows while on is true. The rows stay where
// they are, so the grid keeps its focus and selection while it loads.
func (g *DataGridElement[T]) Loading(on bool) *DataGridElement[T] { g.loading = on; return g }

// Empty sets what the grid shows with no rows, under its header. Without it the
// grid shows "No rows".
func (g *DataGridElement[T]) Empty(el Element) *DataGridElement[T] { g.empty = el; return g }

// Styles sets the look of the parts of the grid, for this instance only. Set
// fields win over the theme's DataGrid and the defaults; see DataGridStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (g *DataGridElement[T]) Styles(st DataGridStyle) *DataGridElement[T] { return g.stylesOf(&st) }

func (g *DataGridElement[T]) stylesOf(st *DataGridStyle) *DataGridElement[T] {
	g.styles.mergeFrom(st, false)
	return g
}

// ContextMenu opens a menu on the cells of the grid, built by build with the row and
// the key of the column. A right-press opens it at the pointer: on a row that is not
// selected it first moves the focus to the cell and selects the row, and on a selected
// row it moves the focus and keeps the selection, as in File Explorer. Shift+F10 or
// the Menu key opens it for the focused cell. An empty result opens nothing.
func (g *DataGridElement[T]) ContextMenu(build func(row T, column string) []MenuItem) *DataGridElement[T] {
	g.rowMenu = build
	return g
}

// HeaderContextMenu opens a menu on the header cells, built by build with the key of
// the column: on a right-press on a header cell, and with Shift+F10 or the Menu key
// while the focus is on the header. An empty result opens nothing.
func (g *DataGridElement[T]) HeaderContextMenu(build func(column string) []MenuItem) *DataGridElement[T] {
	g.headerMenu = build
	return g
}

// DataGridHandle scrolls a DataGrid and moves its focus from code, by the keys of
// the rows and the columns. Create one with NewDataGridHandle and pass it to
// DataGrid.Handle. A request takes effect when the grid is next built, on the next
// frame; a key that names no row or column does nothing. Call it on the UI thread.
type DataGridHandle struct {
	inst           *componentInstance
	row, col       string
	hasRow, hasCol bool
	focus          bool
	copy           func(headers bool) // the copy of the grid's last build
}

// NewDataGridHandle creates a handle to pass to DataGrid.Handle.
func NewDataGridHandle() *DataGridHandle { return &DataGridHandle{} }

// ScrollToRow scrolls the least amount that shows the row with the key key, in
// the order the rows are sorted in.
func (h *DataGridHandle) ScrollToRow(key string) {
	h.row, h.hasRow = key, true
	h.request()
}

// ScrollToColumn scrolls sideways the least amount that shows the column with the
// key key.
func (h *DataGridHandle) ScrollToColumn(key string) {
	h.col, h.hasCol = key, true
	h.request()
}

// FocusCell moves the grid's focused cell to the row with the key rowKey, in the
// column with the key colKey (the focused column when colKey names none), scrolls
// it into view and gives the grid keyboard focus. It does not change the selection.
func (h *DataGridHandle) FocusCell(rowKey, colKey string) {
	h.row, h.hasRow, h.col, h.hasCol, h.focus = rowKey, true, colKey, true, true
	h.request()
}

// Copy copies the grid's selection to the clipboard now, as Ctrl+C does, or with the
// headers of its columns first, as Ctrl+Shift+C does, when withHeaders is set. It is
// for a toolbar button or a menu of the app's that takes focus from the grid. It does
// nothing before the grid is first built.
func (h *DataGridHandle) Copy(withHeaders bool) {
	if h.copy != nil {
		h.copy(withHeaders)
	}
}

func (h *DataGridHandle) request() {
	if h.inst != nil {
		h.inst.invalidate()
	}
}

// Handle connects h, so that code can scroll the grid and move its focus.
func (g *DataGridElement[T]) Handle(h *DataGridHandle) *DataGridElement[T] {
	g.gridHandle = h
	return g
}

// ColumnState is the layout of the columns of a DataGrid that the user can change:
// the keys of the columns in the order they are shown, and the widths the user has
// set, by key, in logical pixels at the Text size setting 100 percent. Save it to
// restore the columns with ColumnOrder and ColumnWidths.
type ColumnState struct {
	// Order is the keys of the columns, in the order they are shown.
	Order []string
	// Widths is the width of each column the user has resized, by key.
	Widths map[string]float32
}

// ColumnOrder keeps the order of the columns in keys: the grid sets it when the
// user moves a column, and a value set in keys shows the columns in that order.
// A key that is not a column's is dropped, and the columns that keys does not name
// follow in the order they are given, so a saved order survives new columns. A nil
// keys keeps the order inside the grid.
func (g *DataGridElement[T]) ColumnOrder(keys *Signal[[]string]) *DataGridElement[T] {
	g.order = keys
	return g
}

// ColumnWidths keeps the widths the user sets in ws, by column key, in logical
// pixels at the Text size setting 100 percent: the grid sets it when the user
// resizes a column, and a value set in ws sizes the columns it names. A nil ws
// keeps the widths inside the grid.
func (g *DataGridElement[T]) ColumnWidths(ws *Signal[map[string]float32]) *DataGridElement[T] {
	g.widths = ws
	return g
}

// OnColumnsChange runs fn with the layout of the columns after the user changes it:
// when a drag of a column's edge ends, at each step of a resize by the keyboard,
// after an auto-fit, and after a column is moved. It does not run while the edge is
// dragged, so fn can save the layout.
func (g *DataGridElement[T]) OnColumnsChange(fn func(ColumnState)) *DataGridElement[T] {
	g.onColumns = fn
	return g
}

// viewCols returns the columns in the order they are shown: the order of the keys
// in order, and then the columns it does not name, in the order they are given;
// the columns pinned to the start come first, and those pinned to the end last.
func (g *DataGridElement[T]) viewCols(order []string) []DataColumn[T] {
	out := g.orderCols(order)
	if slices.ContainsFunc(out, func(c DataColumn[T]) bool { return c.Pin != PinNone }) {
		rank := func(p ColumnPin) int { return [...]int{PinNone: 1, PinStart: 0, PinEnd: 2}[min(p, PinEnd)] }
		out = slices.Clone(out)
		slices.SortStableFunc(out, func(a, b DataColumn[T]) int { return rank(a.Pin) - rank(b.Pin) })
	}
	return out
}

// orderCols returns the columns in the order of the keys in order, and then the
// columns it does not name, in the order they are given.
func (g *DataGridElement[T]) orderCols(order []string) []DataColumn[T] {
	if len(order) == 0 {
		return g.cols
	}
	at := make(map[string]int, len(g.cols))
	for i, c := range g.cols {
		at[c.key()] = i
	}
	used := make([]bool, len(g.cols))
	out := make([]DataColumn[T], 0, len(g.cols))
	for _, k := range order {
		if i, ok := at[k]; ok && !used[i] {
			used[i] = true
			out = append(out, g.cols[i])
		}
	}
	for i, c := range g.cols {
		if !used[i] {
			out = append(out, c)
		}
	}
	return out
}

// dgCursor is the focused cell: the key of its row (empty on the header), the
// position of that row when the key is gone, and the column, by key and by the
// position it had, for when the key is gone.
type dgCursor struct {
	key    string
	pos    int
	col    int
	colKey string
	header bool
	// rowHead puts the cursor on the row's header, or on the corner with header (see
	// RowHeaders). col stays the column it had.
	rowHead bool
}

// The drags of a selection (see dgShared.selDrag).
const (
	dgDragNone uint8 = iota
	dgDragCells
	dgDragRows
	dgDragCols
)

// dgSpan says whether a range of a cell selection has every row (rows) and every
// column (cols), as the selection of a column or of a row does.
type dgSpan struct{ rows, cols bool }

// dgEdit is the edit of a cell: the key of its row ("" when no cell is being
// edited), its column (by key, and by position for when the key is gone), and the
// error of the last save, which keeps the editor open.
type dgEdit struct {
	key    string
	col    int
	colKey string
	err    string
	seq    int // the number of the edit, so that a blur from an older editor is ignored
}

// dgOps are the actions that the cells of a grid call. pick moves the cursor to a
// cell without selecting its row, and edit starts an edit of a cell. save, cancel
// and blur are the editor's: save saves its text and returns focus to the cell,
// cancel ends the edit and returns focus, and blur ends it when focus leaves.
type dgOps struct {
	pick, edit func(q, cl int)
	save       func(text string)
	cancel     func()
	blur       func(seq int)
}

// dgRowCtx is what the rows of a grid are built from: the look, the data, the
// cursor and the edit, and the actions of the cells (see dgOps).
type dgRowCtx[T any] struct {
	t                    *Theme
	ctx                  *Context // the grid's render, for the layers of the editor's style (see StyleLayer)
	st                   *DataGridStyle
	sh                   *dgShared[T]
	rows                 []T
	cols                 []DataColumn[T]
	xs                   []float32
	selected             map[string]bool
	rowH                 float32
	cursorPos, cursorCol int
	onHeader, hasFocus   bool
	startN, endN         int     // the numbers of the columns pinned to the start and the end
	built                []bool  // the columns whose cells are built, or nil for all of them
	cells                *dgSel  // the selected cells, when the grid selects cells
	rowHeadW             float32 // the width of the row headers, or zero without them
	onRowHead            bool    // the cursor is on a row's header (see dgCursor.rowHead)
	ed                   dgEdit
	click                func(q, cl int, mods input.Modifiers)
	colAt                func(q int, x float32) int
	activate             func(q int)
	ops                  dgOps
	store                *Signal[[]string] // the open rows of a tree, or nil (see Expanded)
	memoOK               bool              // the rows can be kept from one build to the next (see rowFor)
}

// dgLive is the closures of the grid's last build, which the handlers of its rows
// call. A row kept from an earlier build (see rowFor) then acts on the grid as it is
// now, not as it was when the row was built.
type dgLive struct {
	click     func(q, cl int, mods input.Modifiers)
	colAt     func(q int, x float32) int
	activate  func(q int)
	ops       dgOps
	cursorCol int
	setOpen   func(key string, v bool)
	// rowHead selects the row at q, from its header, with the modifiers mods.
	rowHead func(q int, mods input.Modifiers)
	// press is a mouse press on the cell at (q, c): a click, which starts a drag.
	press func(q, c int, mods input.Modifiers)
}

// dgRowLook is what every row of a grid is built from, beyond the row itself: when
// it changes, no row is kept (see rowFor and same).
type dgRowLook struct {
	theme        *Theme
	highContrast bool
	st           DataGridStyle
	cols         []dgColLook
	xs           []float32
	built        []bool
	startN, endN int
	striped      bool
	mode         SelectionMode
	tree         bool
	rowHeadW     float32
}

// dgColLook is what a row's cell is built from, of its column.
type dgColLook struct {
	key       string
	grow      float32
	align     Alignment
	pin       ColumnPin
	cellStyle Style
	text      bool
}

// same reports whether the rows built from l and o are built the same.
func (l *dgRowLook) same(o *dgRowLook) bool {
	if l.theme != o.theme || l.highContrast != o.highContrast || l.startN != o.startN || l.endN != o.endN ||
		l.striped != o.striped || l.mode != o.mode || l.tree != o.tree || l.rowHeadW != o.rowHeadW ||
		!slices.Equal(l.xs, o.xs) || !slices.Equal(l.built, o.built) || len(l.cols) != len(o.cols) ||
		!sameDataGridStyle(&l.st, &o.st) {
		return false
	}
	for c := range l.cols {
		a, b := &l.cols[c], &o.cols[c]
		if a.key != b.key || a.grow != b.grow || a.align != b.align || a.pin != b.pin || a.text != b.text ||
			!sameStyle(&a.cellStyle, &b.cellStyle) {
			return false
		}
	}
	return true
}

// sameDataGridStyle reports whether a and b have the same styles for every part but
// Editor, the style of an editing cell's editor; a row being edited is never kept.
func sameDataGridStyle(a, b *DataGridStyle) bool {
	bs := b.styles()
	for i, pa := range a.styles() {
		if !sameStyle(pa, bs[i]) {
			return false
		}
	}
	return true
}

// styles returns the styles of the parts of s, all but Editor.
func (s *DataGridStyle) styles() [28]*Style {
	return [28]*Style{&s.Grid, &s.GridFocusVisible, &s.Header, &s.HeaderCell, &s.HeaderCellHover,
		&s.HeaderCellSorted, &s.SortIndicator, &s.ResizeHandle, &s.Row, &s.RowHover, &s.RowSelected,
		&s.RowAlternate, &s.Cell, &s.CellFocused, &s.Empty, &s.CellEditing, &s.EditorMessage,
		&s.DropIndicator, &s.CellPinned, &s.PinnedEdge, &s.HeaderGroup, &s.CellSelected, &s.TreeIndent,
		&s.TreeChevron, &s.RowHeader, &s.RowHeaderSelected, &s.HeaderCellSelected, &s.Corner}
}

// sameStyle reports whether a and b are the same style. The slices are compared by
// their elements; the other fields, which are numbers, strings, colors and other
// plain values (TestSameStyleCoversStyle checks that they are), by their memory. Two
// styles that differ only in memory that holds no value, such as padding, or in
// strings with equal text held in different memory, are taken as different, which
// costs a row that is built again, never a wrong one.
func sameStyle(a, b *Style) bool {
	if !slices.Equal(a.GridColumns, b.GridColumns) || !slices.Equal(a.GridRows, b.GridRows) ||
		!slices.Equal(a.Shadows, b.Shadows) {
		return false
	}
	x, y := *a, *b
	x.GridColumns, x.GridRows, x.Shadows = nil, nil, nil
	y.GridColumns, y.GridRows, y.Shadows = nil, nil, nil
	size := unsafe.Sizeof(x)
	return bytes.Equal(unsafe.Slice((*byte)(unsafe.Pointer(&x)), size), unsafe.Slice((*byte)(unsafe.Pointer(&y)), size))
}

// dgRowSig is what a row was built from, beyond dgRowLook and the text of its cells:
// a row whose signature and texts are the same is built the same, but for its
// position, which place sets.
type dgRowSig struct {
	gen        int  // dgShared.lookGen when the row was built
	selected   bool // the row is selected
	cursor     int  // the column of the cursor in the row, or -1
	focused    bool // the cursor's cell is drawn focused
	odd        bool // the row is an odd one of a striped grid
	level      int  // the level of a tree row
	kids, open bool // a tree row has children, and is open
	num        int  // the number the row's header shows, or zero without row headers
	join       bool // the row and the one below it are selected (see joinBelow)
}

// dgRowMemo is a built row of a grid: its element, what it was built from, and the
// elements whose position place updates when the row is kept at another position.
// q is the row's position at its last build or placing, which its handlers act on.
type dgRowMemo struct {
	el        *ComponentElement
	inst      *componentInstance // the instance the element was built by
	row       *DivElement
	cells     []*DivElement
	sig       dgRowSig
	texts     []string
	seen      int // dgShared.builds at the row's last build or keeping
	q         int
	pos, size int // the position of the row in its set, for assistive technology
}

// live reports whether the row's element is still built by its instance, so that
// returning it again keeps the row's nodes.
func (m *dgRowMemo) live() bool {
	return m.el != nil && m.inst != nil && !m.inst.disposed && m.inst.el == m.el && m.inst.root != nil && m.inst.root.div == m.row
}

// place moves a kept row to position q, the pos-th of size in its set.
func (m *dgRowMemo) place(q, pos, size int) {
	m.q = q
	if a := m.row.acc; a != nil {
		a.gridRow, a.setPos, a.setSize = q+1, pos, size
	}
	for _, c := range m.cells {
		c.acc.gridRow = q + 1
	}
	m.pos, m.size = pos, size
}

// dgShared is the state of a grid that lives with it: the order of its rows, the
// drag of a column edge, and what the paint of its list reports back.
type dgShared[T any] struct {
	// The keys are made once per slice of rows, and the order once per sort.
	keyed, ordered bool
	ident          unsafe.Pointer
	n              int
	keys           []string       // key of each row of the data
	sortedKeys     bool           // the keys are in strictly increasing order, so unique
	by             SortState      // the sort the order is made for
	keepRoots      bool           // the order keeps the top rows of a tree (see SortChildrenOnly)
	order          []int          // the row of the data at each displayed position
	pos            map[string]int // displayed position by key, made when needed (see posOf)
	dup            string         // a key that is used twice, for the warning
	data           []T            // the rows of the last build, for an edit to find its row

	// A flat grid keeps shadow, a copy of the rows its keys were made for, while
	// shadowed (see makeKeys), and midKeys for the keys of the rows a change made.
	// keysFlat says that keys is makeKeys's own, not the keys of a tree; descents is
	// the number of keys that do not come after the key before them. orderBuf holds
	// the order, from one slice of rows to the next.
	shadow []T
	// live is the closures of the last build (see dgLive). rowMemo is the built rows by
	// key (see rowFor); look is what they were built from, and lookGen counts its
	// changes. texts is a buffer for the text of a row's cells.
	live     dgLive
	rowMemo  map[string]*dgRowMemo
	look     dgRowLook
	lookGen  int
	builds   int // the builds of the grid
	texts    []string
	shadowed bool
	midKeys  []string
	keysFlat bool
	descents int
	orderBuf []int
	// counts is how many rows have each key, kept while the grid still checks for a
	// repeated key (see countKeys); dups is the number of keys used more than once,
	// and dupKey the last key that became one of them.
	counts map[string]int32
	dups   int
	dupKey string

	// The tree of the rows, when the grid has Children (see dgTree), with the identity
	// of the rows it was made from. open is the keys of the open rows the order was made
	// for, and openSet the same as a set.
	tree      *dgTree[T]
	treeIdent unsafe.Pointer
	open      []string
	openSet   map[string]bool

	// focus is the grid's body, and editFocus the editor of an edit. draft is the
	// text of the editor. ctl is set by a press on a control in a cell, so that the
	// row does not select or activate on it. seq counts the edits; wantFocus is set
	// while a custom editor waits to be painted, to take focus.
	focus, editFocus *FocusHandle
	draft            *Signal[string]
	ctl              bool
	seq              int
	wantFocus        bool

	selOK    bool
	selIdent unsafe.Pointer
	selN     int
	selSet   map[string]bool

	// The index of the selected ranges, made again only when they change (see
	// rangesSelection), and whether the warning about Selection and CellSelection has
	// been given. cellsIdent and cellsN are the cells of CellSelection last read or
	// written, so that a value the app sets there is told from the grid's own.
	rangeOK    bool
	rangeIdent unsafe.Pointer
	rangeN     int
	rangeSel   *dgSel
	cellsIdent unsafe.Pointer
	cellsN     int
	// rangesIdent and rangesN are the ranges last written or read with them.
	rangesIdent unsafe.Pointer
	rangesN     int
	warnedSel   bool

	// The displayed positions built by the last frame: [builtLo, builtHi).
	builtLo, builtHi int
	// viewW is the width of the viewport at the last paint, and pressed is set by
	// a mouse press on a row, so that the click that follows does not select twice.
	viewW   float32
	pressed bool
	// headPress is set by a mouse press on a header cell or the corner, so that the
	// click that follows does not act a second time (see HeaderSelects).
	headPress bool
	// selDrag is the drag of a selection that a press started: over the cells, down
	// the row headers or across the column headers. dragQ and dragC are where it has
	// reached, and dragKeep keeps the rest of the selection (a press with Ctrl).
	selDrag      uint8
	dragQ, dragC int
	dragKeep     bool
	dragAt       time.Time // the frame of the last step of dragScroll

	// The column edge being dragged.
	dragging     bool
	dragKey      string
	dragX, dragW float32
	dragFrom     float32 // the width at the press, at the Text size setting 100 percent
	// edgePress is set from a press on a column's edge until the frame after its
	// release, so that the click the release makes on the header cell does not sort.
	edgePress bool

	// A column the cursor moved to is scrolled into view at the next paint, from
	// revealX0 to revealX1 in the content.
	reveal             bool
	revealX0, revealX1 float32
	viewH              float32 // the height of the viewport at the last paint
	warned             bool

	// When the columns are wider than the view, only the columns that scroll in
	// [colFrom, colTo) are built in the rows (colSet once it is known), with the
	// pinned columns, the cursor's and the editing one. visW is the width of the
	// rows the view shows, at the last paint.
	colFrom, colTo int
	colSet         bool
	visW           float32

	// asked is set once LoadMore has asked for rows, when there were askedN of them;
	// loadingMore is whether the last build showed the LoadingMore row.
	asked       bool
	askedN      int
	loadingMore bool
}

// update makes the keys of the rows when the rows have changed, and the order of
// them for the sort by when either has changed. While checkDup is set, a key used
// twice is noted in dup (see countKeys).
//
// The table of positions by key is not made here: posOf makes it on its first
// lookup, and posNear finds a row near where it was without it. So a change to a
// long list of rows costs a call of key for each row and an order, not a map.
func (sh *dgShared[T]) update(rows []T, key func(T) string, cols []DataColumn[T], by SortState, checkDup bool) {
	n := len(rows)
	ident := unsafe.Pointer(unsafe.SliceData(rows))
	if !sh.keyed || sh.ident != ident || sh.n != n {
		sh.keyed, sh.ident, sh.n = true, ident, n
		sh.makeKeys(rows, key, checkDup)
		sh.ordered = false
	} else if !checkDup {
		sh.counts, sh.dups = nil, 0
	}
	if sh.ordered && sh.by == by {
		return
	}
	sh.ordered, sh.by = true, by
	order := slices.Grow(sh.orderBuf[:0], n)[:n]
	for i := range order {
		order[i] = i
	}
	for _, c := range cols {
		if by.Key != "" && c.key() == by.Key && c.Compare != nil {
			cmp := c.Compare
			slices.SortStableFunc(order, func(a, b int) int {
				r := cmp(rows[a], rows[b])
				if by.Desc {
					return -r
				}
				return r
			})
			break
		}
	}
	sh.order, sh.orderBuf = order, order
	sh.tree = nil
	sh.pos = nil
}

// dgShadowMax is the largest row, in bytes, whose rows a grid keeps a copy of to
// find what a change to them changed (see makeKeys). Larger rows are keyed again.
const dgShadowMax = 128

// makeKeys makes the keys of rows, the rows of a new slice. It keeps a copy of the
// rows it made them for, and compares the new rows with it, byte for byte, from
// the start and from the end: the rows that are the same at both ends keep their
// keys, and key runs only for the rows between. A row whose bytes are the same is
// the same value, whose key is the same, since keys are stable. So an insert, a
// removal or a move among 100,000 rows calls key a few times, not 100,000, and
// changes the count of repeated keys (see countKey) and of keys out of order for
// those rows alone.
//
// Rows larger than dgShadowMax are not copied, and every change keys them all.
func (sh *dgShared[T]) makeKeys(rows []T, key func(T) string, checkDup bool) {
	n := len(rows)
	size := unsafe.Sizeof(*new(T))
	if !sh.keysFlat || !sh.shadowed {
		keys := slices.Grow(sh.keys[:0:0], n)[:n]
		if sh.keysFlat {
			keys = slices.Grow(sh.keys[:0], n)[:n]
		}
		for i := range rows {
			keys[i] = key(rows[i])
		}
		sh.keys, sh.keysFlat = keys, true
		sh.descents = dgDescents(keys, 1, n-1)
		sh.sortedKeys = sh.descents == 0
		sh.counts, sh.dups = nil, 0
		sh.shadowed = size > 0 && size <= dgShadowMax
		sh.shadow = nil
		if sh.shadowed {
			sh.shadow = append(sh.shadow, rows...)
		}
		sh.noteDup(checkDup)
		return
	}
	old := sh.shadow
	oldN := len(old)
	head := dgSameHead(old, rows)
	tail := dgSameTail(old[head:], rows[head:])
	oldEnd, newEnd := oldN-tail, n-tail
	sh.descents -= dgDescents(sh.keys, max(1, head), min(oldN-1, oldEnd))
	counting := checkDup && sh.counts != nil
	if counting {
		for _, k := range sh.keys[head:oldEnd] {
			sh.countKey(k, -1)
		}
	}
	mid := sh.midKeys[:0]
	for _, r := range rows[head:newEnd] {
		mid = append(mid, key(r))
	}
	sh.keys = slices.Replace(sh.keys, head, oldEnd, mid...)
	clear(mid)
	sh.midKeys = mid[:0]
	sh.shadow = slices.Replace(sh.shadow, head, oldEnd, rows[head:newEnd]...)
	sh.descents += dgDescents(sh.keys, max(1, head), min(n-1, newEnd))
	sh.sortedKeys = sh.descents == 0
	if counting {
		for _, k := range sh.keys[head:newEnd] {
			sh.countKey(k, 1)
		}
	}
	sh.noteDup(checkDup)
}

// noteDup notes in dup a key that the rows use twice, while check is set. Keys in
// strictly increasing order are unique and are not counted; others are counted
// once, and makeKeys keeps the counts in step with each change after that. The
// grid warns once (see build), and counts no more after that.
func (sh *dgShared[T]) noteDup(check bool) {
	sh.dup = ""
	if !check || sh.sortedKeys {
		sh.counts, sh.dups = nil, 0
		return
	}
	if sh.counts == nil {
		sh.counts, sh.dups = make(map[string]int32, len(sh.keys)), 0
		for _, k := range sh.keys {
			sh.countKey(k, 1)
		}
	}
	if sh.dups == 0 {
		return
	}
	if sh.counts[sh.dupKey] < 2 {
		for k, c := range sh.counts {
			if c > 1 {
				sh.dupKey = k
				break
			}
		}
	}
	sh.dup = sh.dupKey
}

// countKey adds d to the count of the key k.
func (sh *dgShared[T]) countKey(k string, d int32) {
	c := sh.counts[k] + d
	switch {
	case d > 0 && c == 2:
		sh.dups++
		sh.dupKey = k
	case d < 0 && c == 1:
		sh.dups--
	}
	if c <= 0 {
		delete(sh.counts, k)
		return
	}
	sh.counts[k] = c
}

// dgDescents counts the positions i from lo to hi, both included, where keys[i]
// does not come after keys[i-1]: the keys are in strictly increasing order when
// there are none.
func dgDescents(keys []string, lo, hi int) int {
	d := 0
	for i := max(lo, 1); i <= hi && i < len(keys); i++ {
		if keys[i-1] >= keys[i] {
			d++
		}
	}
	return d
}

// dgRowBytes returns the memory of rows.
func dgRowBytes[T any](rows []T) []byte {
	if len(rows) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(rows))), len(rows)*int(unsafe.Sizeof(rows[0])))
}

// dgSameHead returns the number of rows at the start of a and b that are the same,
// byte for byte. It compares blocks of memory first, then the rows of the block
// that differs.
func dgSameHead[T any](a, b []T) int {
	m := min(len(a), len(b))
	size := int(unsafe.Sizeof(*new(T)))
	ab, bb := dgRowBytes(a[:m]), dgRowBytes(b[:m])
	const block = 4096
	i := 0
	for i+block <= len(ab) && bytes.Equal(ab[i:i+block], bb[i:i+block]) {
		i += block
	}
	h := i / size
	for h < m && bytes.Equal(ab[h*size:(h+1)*size], bb[h*size:(h+1)*size]) {
		h++
	}
	return h
}

// dgSameTail returns the number of rows at the end of a and b that are the same,
// byte for byte.
func dgSameTail[T any](a, b []T) int {
	m := min(len(a), len(b))
	size := int(unsafe.Sizeof(*new(T)))
	ab, bb := dgRowBytes(a[len(a)-m:]), dgRowBytes(b[len(b)-m:])
	const block = 4096
	j := len(ab) // ab[j:] and bb[j:] are the same
	for j-block >= 0 && bytes.Equal(ab[j-block:j], bb[j-block:j]) {
		j -= block
	}
	t := (len(ab) - j) / size
	for t < m {
		lo := len(ab) - (t+1)*size
		if !bytes.Equal(ab[lo:lo+size], bb[lo:lo+size]) {
			break
		}
		t++
	}
	return t
}

// makePos makes the table of displayed positions by key, and notes in dup a key
// that is used twice. A tree notes its repeated keys among all its rows, not among
// the shown ones, when it is made (see dgTree).
func (sh *dgShared[T]) makePos() {
	sh.pos = make(map[string]int, len(sh.order))
	dup := ""
	for p, i := range sh.order {
		k := sh.keys[i]
		if _, ok := sh.pos[k]; ok && dup == "" {
			dup = k
		}
		sh.pos[k] = p
	}
	if sh.tree == nil {
		sh.dup = dup
	}
}

// posOf returns the displayed position of the row with key k.
func (sh *dgShared[T]) posOf(k string) (int, bool) {
	if sh.pos == nil {
		sh.makePos()
	}
	q, ok := sh.pos[k]
	return q, ok
}

// posNear is posOf for a row that was at the position hint: it looks at the few
// positions either side of hint first, which finds the row after rows were added or
// removed near it, or after it moved a little, without the table of positions.
func (sh *dgShared[T]) posNear(k string, hint int) (int, bool) {
	n := len(sh.order)
	for d := range 5 {
		if q := hint - d; q >= 0 && q < n && sh.keys[sh.order[q]] == k {
			return q, true
		}
		if q := hint + d; d > 0 && q >= 0 && q < n && sh.keys[sh.order[q]] == k {
			return q, true
		}
	}
	return sh.posOf(k)
}

// selection returns the set of the keys in sel, made again only when sel changes.
func (sh *dgShared[T]) selection(sel []string) map[string]bool {
	ident := unsafe.Pointer(unsafe.SliceData(sel))
	if sh.selOK && sh.selIdent == ident && sh.selN == len(sel) {
		return sh.selSet
	}
	set := make(map[string]bool, len(sel))
	for _, k := range sel {
		set[k] = true
	}
	sh.selOK, sh.selIdent, sh.selN, sh.selSet = true, ident, len(sel), set
	return set
}

// expand builds the grid when it is placed in the tree.
func (g *DataGridElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		return g.build(ctx)
	})
}

// alignOf is the flex alignment of a column's content: Start for the zero value.
func alignOf(a Alignment) Alignment {
	if a == 0 {
		return Start
	}
	return a
}

// textWidth is the width of s on one line at the Text size, for auto-fit.
func textWidth(rt *Runtime, s string) float32 {
	return rt.text.Layout(s, text.Style{Size: rt.theme.FontSize * rt.textFactor, NoWrap: true}, 1e6).Width
}

// autoFitWidth is the width that fits column i to its header and to the Text of
// the rows built by the last frame. A Cell slot is not measured. In a tree, the first
// column also fits the indent of each row and its chevron (see DataGridStyle).
func autoFitWidth[T any](rt *Runtime, st *DataGridStyle, sh *dgShared[T], rows []T, cols []DataColumn[T], i int) float32 {
	cl := cols[i]
	w := textWidth(rt, cl.Header) + 40 // the padding, the sort arrow and the edge
	for q := sh.builtLo; q < sh.builtHi && q < sh.n; q++ {
		lead := float32(0)
		if sh.tree != nil && i == 0 {
			lead = float32(sh.tree.level[sh.order[q]]-1)*st.TreeIndent.Width.Value + st.TreeChevron.Width.Value
		}
		if cl.Text != nil {
			w = max(w, textWidth(rt, cl.Text(rows[sh.order[q]]))+16+lead)
		} else {
			w = max(w, 16+lead)
		}
	}
	return max(w, cl.minWidth())
}

// build returns the grid's outer box: the body, which is the scrolling list of the
// rows with the header over its top, and the message and the progress ring over it.
func (g *DataGridElement[T]) build(ctx *Context) Element {
	t := ctx.Theme()
	rt := ctx.rt
	strs := ctx.Strings()
	inst := ctx.inst // the grid's own component: a width change rebuilds it
	var st DataGridStyle
	resolveDataGrid(ctx, g.density, rt.textFactor, &g.styles, &st)
	sh := Local(ctx, "grid", &dgShared[T]{}).Peek()
	cur := Local(ctx, "cursor", dgCursor{})
	anchor := Local(ctx, "anchor", "")
	focused := Local(ctx, "focused", false)
	edit := Local(ctx, "edit", dgEdit{})
	widths := g.widths
	if widths == nil {
		widths = Local(ctx, "widths", map[string]float32(nil))
	}
	colOrder := g.order
	if colOrder == nil {
		colOrder = Local(ctx, "order", []string(nil))
	}
	handle := Local(ctx, "handle", NewListHandle()).Peek()
	colMark := Local(ctx, "colMark", dgColMark{})
	sel := g.sel
	if sel == nil {
		sel = Local(ctx, "selection", []string(nil))
	}
	by := g.sort
	if by == nil {
		by = Local(ctx, "sort", SortState{})
	}
	rangeSel := g.ranges
	if rangeSel == nil {
		rangeSel = Local(ctx, "ranges", []CellRange(nil))
	}
	anchorCol := Local(ctx, "anchorCol", "")
	anchorSpan := Local(ctx, "anchorSpan", dgSpan{})
	selBase := Local(ctx, "selBase", []CellRange(nil))

	// The columns are in the order they are shown, so that a column's index is its
	// place on the screen and its column for assistive technology.
	cols := g.viewCols(colOrder.Get())
	nc := len(cols)
	colPos := make(map[string]int, nc)
	colKeys := make([]string, nc)
	for i, cl := range cols {
		colPos[cl.key()] = i
		colKeys[i] = cl.key()
	}
	// colOf is the index of the column with key, or of the one at i when the key is
	// gone, so that the cursor and an edit follow their column when it moves.
	colOf := func(key string, i int) int {
		if c, ok := colPos[key]; ok && key != "" {
			return c
		}
		return min(max(i, 0), max(nc-1, 0))
	}
	sortState := by.Get()
	store := g.openStore(ctx)
	rows := g.refresh(sh, store, cols, sortState, rt.warn != nil && !sh.warned)
	n := len(sh.order)
	sh.data = rows
	if sh.focus == nil {
		sh.focus, sh.editFocus, sh.draft = NewFocusHandle(), NewFocusHandle(), NewSignal("")
	}
	if g.focusHandle != nil {
		sh.focus = g.focusHandle
	}
	if g.selBoth && !sh.warnedSel && rt.warn != nil {
		sh.warnedSel = true
		rt.warn("a DataGrid has both Selection and CellSelection; it selects rows or cells, " +
			"and the later call applies. Call one of them.")
	}
	if sh.dup != "" && !sh.warned && rt.warn != nil {
		sh.warned = true
		rt.warn(fmt.Sprintf("duplicate key %q among the rows of a DataGrid; keys must be unique. "+
			"Derive keys from a unique field such as an ID.", sh.dup))
	}
	// The order is read from sh, not copied: a key handler may open or close a tree row
	// before the next frame (see setOpen), and then the handler reads the rows as shown.
	keyAt := func(q int) string { return sh.keys[sh.order[q]] }
	selected := sh.selection(sel.Get())
	var cellsOn *dgSel
	if g.cellOn {
		rs := rangeSel.Get()
		if g.cells != nil {
			// A value the app has set in the cells of CellSelection selects those cells,
			// and one it has set in the ranges is listed in the cells: each is told from the
			// grid's own by the slice the grid last wrote.
			cs := g.cells.Get()
			switch rid := unsafe.Pointer(unsafe.SliceData(rs)); {
			case unsafe.Pointer(unsafe.SliceData(cs)) != sh.cellsIdent || len(cs) != sh.cellsN:
				sh.cellsIdent, sh.cellsN = unsafe.Pointer(unsafe.SliceData(cs)), len(cs)
				rs = rangesOfCells(cs)
				sh.rangesIdent, sh.rangesN = unsafe.Pointer(unsafe.SliceData(rs)), len(rs)
				rangeSel.Set(rs)
			case rid != sh.rangesIdent || len(rs) != sh.rangesN:
				sh.rangesIdent, sh.rangesN = rid, len(rs)
				cs = cellsOfRanges(rs, n, keyAt, colKeys)
				sh.cellsIdent, sh.cellsN = unsafe.Pointer(unsafe.SliceData(cs)), len(cs)
				g.cells.Set(cs)
			}
		}
		cellsOn = sh.rangesSelection(rs)
	}
	rowH := dataGridRowHeight(g.density) * rt.textFactor
	// The header is a row high, or two with a row of the groups of columns above it.
	headerH := rowH
	if slices.ContainsFunc(g.cols, func(c DataColumn[T]) bool { return c.Group != "" }) {
		headerH = 2 * rowH
	}

	// The cursor follows its row by key, so that a sort or new rows keep it on
	// the same cell.
	cursor := cur.Get()
	if store != nil && cursor.key != "" {
		// A collapse that hides the focused row moves the focus to the row above it that
		// is shown, in the same column.
		if k, q, ok := sh.shownKey(cursor.key); ok && k != cursor.key {
			cursor.key, cursor.pos = k, q
			cur.Set(cursor)
		}
	}
	p := min(max(cursor.pos, 0), max(n-1, 0))
	if cursor.key != "" {
		if q, ok := sh.posNear(cursor.key, cursor.pos); ok {
			p = q
		}
	}
	col := colOf(cursor.colKey, cursor.col)
	onHeader := cursor.header
	onRowHead := cursor.rowHead && g.rowHeaders
	hasFocus := focused.Get()
	ed := edit.Get()
	ed.col = colOf(ed.colKey, ed.col)
	anchorPos := func() int {
		if k := anchor.Peek(); k != "" {
			if q, ok := sh.posOf(k); ok {
				return q
			}
		}
		return p
	}

	// The width of each column, and where each one starts. The row headers come first,
	// as wide as the longest number (see RowHeaders).
	var rowHeadW float32
	if g.rowHeaders {
		rowHeadW = max(textWidth(rt, strings.Repeat("0", len(strconv.Itoa(max(n, 1)))))+18, 32*rt.textFactor)
	}
	ws := widths.Get()
	cw := make([]float32, nc)
	xs := make([]float32, nc+1)
	xs[0] = rowHeadW
	for i, cl := range cols {
		cw[i] = cl.width(ws) * rt.textFactor // the columns grow with the Text size, as the rows do
		xs[i+1] = xs[i] + cw[i]
	}
	total := xs[nc]
	// The pinned columns are the first startN and the last endN, startW and endW wide.
	startN, endN := 0, 0
	for _, cl := range cols {
		switch cl.Pin {
		case PinStart:
			startN++
		case PinEnd:
			endN++
		}
	}
	startW, endW := xs[startN], total-xs[nc-endN]
	// When the columns are wider than the view, the rows build only the columns in
	// view (see dgShared.colFrom), so that a grid of many columns costs what its
	// visible part costs. The header is built whole.
	virtual := sh.colSet && sh.visW > 0 && total > sh.visW
	var built []bool
	if virtual {
		built = make([]bool, nc)
		for c, cl := range cols {
			built[c] = cl.Pin != PinNone || (c >= sh.colFrom && c < sh.colTo)
		}
		if nc > 0 {
			built[col] = true
			if ed.key != "" {
				built[ed.col] = true
			}
		}
	}
	sortCol := -1
	for i, cl := range cols {
		if sortState.Key != "" && cl.key() == sortState.Key && cl.Compare != nil {
			sortCol = i
		}
	}

	// setWidth sets column i to v pixels wide at the Text size in use, and reports
	// whether the width changed. The width is kept at the size of 100 percent, so that
	// it scales with the text.
	setWidth := func(i int, v float32) bool {
		cl := cols[i]
		u := max(v/rt.textFactor, cl.minWidth())
		if cl.MaxWidth > 0 {
			u = min(u, cl.MaxWidth)
		}
		if u == cl.width(ws) {
			return false
		}
		next := make(map[string]float32, nc)
		for k, x := range ws {
			next[k] = x
		}
		next[cl.key()] = u
		widths.Set(next)
		return true
	}
	// columnsChanged tells OnColumnsChange the layout of the columns, after a change
	// that the user has made.
	columnsChanged := func() {
		if g.onColumns == nil {
			return
		}
		vc := g.viewCols(colOrder.Peek())
		keys := make([]string, len(vc))
		for i, cl := range vc {
			keys[i] = cl.key()
		}
		g.onColumns(ColumnState{Order: keys, Widths: widths.Peek()})
	}
	// sortBy turns the sort by column i through ascending, descending and none.
	sortBy := func(i int) {
		cl := cols[i]
		if cl.Compare == nil {
			return
		}
		s := by.Peek()
		switch {
		case s.Key != cl.key():
			by.Set(SortState{Key: cl.key()})
		case !s.Desc:
			by.Set(SortState{Key: cl.key(), Desc: true})
		default:
			by.Set(SortState{})
		}
	}
	activate := func(q int) {
		if g.onActivate != nil && n > 0 {
			g.onActivate(rows[sh.order[q]])
		}
	}
	// askMore asks LoadMore for more rows, once for each number of rows. It runs fn
	// after the frame, since it is called while the rows are built. The rows counted
	// are the rows given: for a tree, every row of it, children included, so that
	// opening a row does not ask again (see Children).
	askMore := func() {
		if g.loadMore == nil || !g.hasMore || (sh.asked && sh.askedN == len(rows)) {
			return
		}
		sh.asked, sh.askedN = true, len(rows)
		rt.Post(g.loadMore)
	}
	selectKeys := func(ks []string) {
		if g.mode != SelectionNone {
			sel.Set(ks)
		}
	}
	rangeKeys := func(a, b int) []string {
		lo, hi := min(a, b), max(a, b)
		out := make([]string, 0, hi-lo+1)
		for q := lo; q <= hi; q++ {
			out = append(out, keyAt(q))
		}
		return out
	}
	toggleKey := func(k string) {
		cur := sel.Peek()
		next := make([]string, 0, len(cur)+1)
		found := false
		for _, x := range cur {
			if x == k {
				found = true
				continue
			}
			next = append(next, x)
		}
		if !found {
			next = append(next, k)
		}
		sel.Set(next)
	}
	// place moves the cursor to row q (on the header when header is set) and to
	// column cl, and brings both into view. The count of the rows is read now, since a
	// tree's open rows may have changed since the build (see setOpen).
	place := func(q, cl int, header bool) {
		k := ""
		if !header && q >= 0 && q < len(sh.order) {
			k = keyAt(q)
		}
		ck := ""
		if cl >= 0 && cl < nc {
			ck = cols[cl].key()
		}
		cur.Set(dgCursor{key: k, pos: q, col: cl, colKey: ck, header: header})
		if nc > 0 && cols[cl].Pin == PinNone { // a pinned column is always in view
			sh.reveal = true
			sh.revealX0, sh.revealX1 = xs[cl], xs[cl+1]
		}
		if !header && len(sh.order) > 0 {
			handle.ScrollToItem(q)
		} else {
			handle.request()
		}
	}
	// placeHead moves the cursor to the header of row q, or to the corner with header
	// (see RowHeaders), and keeps its column. The row headers are always in view.
	placeHead := func(q int, header bool) {
		c := cur.Peek()
		k := ""
		if !header && q >= 0 && q < len(sh.order) {
			k = keyAt(q)
		}
		cur.Set(dgCursor{key: k, pos: q, col: c.col, colKey: c.colKey, header: header, rowHead: true})
		if !header && len(sh.order) > 0 {
			handle.ScrollToItem(q)
		} else {
			handle.request()
		}
	}
	// focusCol moves the cursor to column cl, in its row or on the header, and scrolls
	// the column into view but not the rows: a click on a column's header leaves the
	// rows where they are.
	focusCol := func(cl int) {
		c := cur.Peek()
		cur.Set(dgCursor{key: c.key, pos: c.pos, col: cl, colKey: cols[cl].key(), header: c.header})
		if cols[cl].Pin == PinNone {
			sh.reveal = true
			sh.revealX0, sh.revealX1 = xs[cl], xs[cl+1]
		}
		handle.request()
	}
	movable := func(cl DataColumn[T]) bool { return cl.Movable || g.movable }
	// moveColumn moves the column at from of vc, the columns in the order they are
	// shown, to the index to, and announces where it is now.
	moveColumn := func(vc []DataColumn[T], from, to int) bool {
		if from < 0 || from >= len(vc) || to < 0 || to >= len(vc) || to == from || vc[to].Pin != vc[from].Pin {
			return false // a column moves among the columns of its pin only
		}
		keys := make([]string, 0, len(vc))
		for i, cl := range vc {
			if i != from {
				keys = append(keys, cl.key())
			}
		}
		keys = slices.Insert(keys, to, vc[from].key())
		colOrder.Set(keys)
		columnsChanged()
		Announce(strs.ColumnMoved(vc[from].Header, to+1, len(vc)), access.Polite)
		return true
	}
	// indexOf is the index of the column with key among vc, or -1.
	indexOf := func(vc []DataColumn[T], key string) int {
		return slices.IndexFunc(vc, func(cl DataColumn[T]) bool { return cl.key() == key })
	}
	// dropColumn puts the dragged column from beside the column to, after it when
	// after is set. The columns are read from the signal, as a key handler reads them.
	dropColumn := func(from, to string, after bool) {
		vc := g.viewCols(colOrder.Peek())
		f, t := indexOf(vc, from), indexOf(vc, to)
		if f < 0 || t < 0 {
			return
		}
		if after {
			t++
		}
		if f < t {
			t-- // the column leaves its place before it is put back
		}
		moveColumn(vc, f, t)
	}
	// stepColumn moves the column with key one place in direction dir, and scrolls it
	// into view at its new place.
	stepColumn := func(key string, dir int) {
		vc := g.viewCols(colOrder.Peek())
		f := indexOf(vc, key)
		if !moveColumn(vc, f, f+dir) || vc[f].Pin != PinNone {
			return
		}
		x := rowHeadW
		for _, k := range colOrder.Peek()[:f+dir] {
			x += cw[colPos[k]]
		}
		sh.reveal = true
		sh.revealX0, sh.revealX1 = x, x+cw[colPos[key]]
		handle.request()
	}
	// A request of the grid's handle is applied now, with the rows and columns of
	// this build. A focus request moves the cursor after the build, as a key would.
	if h := g.gridHandle; h != nil {
		h.inst = inst
		var q int
		rowOK := false
		if h.hasRow {
			q, rowOK = sh.posOf(h.row)
		}
		c, colOK := colPos[h.col]
		colOK = colOK && h.hasCol
		switch {
		case h.focus && rowOK:
			if !colOK {
				c = col
			}
			rt.Post(func() {
				place(q, c, false)
				sh.focus.Focus()
			})
		case h.focus:
		default:
			if rowOK {
				handle.ScrollToItem(q)
			}
			if colOK && cols[c].Pin == PinNone {
				sh.reveal = true
				sh.revealX0, sh.revealX1 = xs[c], xs[c+1]
				handle.request()
			}
		}
		h.hasRow, h.hasCol, h.focus = false, false, false
	}
	// The cells of a cell selection (see CellRanges). The anchor of the cells is the anchor row and anchorCol; anchorSpan says whether
	// the anchor's range has every row or every column, as the selection of a column or
	// a row does, so that Shift extends whole columns or rows from it. selBase is the
	// rest of the selection beside the anchor's range, which Ctrl+Shift keeps.
	curSel := func() *dgSel { return sh.rangesSelection(rangeSel.Peek()) }
	setRanges := func(rs []CellRange) {
		if g.cellMode == SelectionNone {
			return
		}
		rangeSel.Set(rs)
		if g.cells != nil {
			// The cells of CellSelection are the ranges listed cell by cell.
			sh.rangesIdent, sh.rangesN = unsafe.Pointer(unsafe.SliceData(rs)), len(rs)
			cs := cellsOfRanges(rs, len(sh.order), keyAt, colKeys)
			sh.cellsIdent, sh.cellsN = unsafe.Pointer(unsafe.SliceData(cs)), len(cs)
			g.cells.Set(cs)
		}
	}
	setAnchor := func(q, c int, span dgSpan, base []CellRange) {
		anchor.Set(keyAt(q))
		anchorCol.Set(cols[c].key())
		anchorSpan.Set(span)
		selBase.Set(base)
	}
	// rect returns the range from (q0, c0) to (q1, c1): their rows and columns, or every
	// row or every column as span says.
	rect := func(q0, c0, q1, c1 int, span dgSpan) CellRange {
		r := CellRange{Rows: allKeys, Columns: allKeys}
		if !span.rows {
			lo, hi := min(q0, q1), max(q0, q1)
			ks := make([]string, 0, hi-lo+1)
			for q := lo; q <= hi; q++ {
				ks = append(ks, keyAt(q))
			}
			r.Rows = keysIn(ks...)
		}
		if !span.cols {
			lo, hi := min(c0, c1), max(c0, c1)
			r.Columns = keysIn(slices.Clone(colKeys[lo : hi+1])...)
		}
		return r
	}
	// extend selects from the anchor to (q, c), in whole rows or columns when the
	// anchor's span or force says so. keep keeps the rest of the selection (Ctrl+Shift).
	extend := func(q, c int, force dgSpan, keep bool) {
		span := anchorSpan.Peek()
		span.rows, span.cols = span.rows || force.rows, span.cols || force.cols
		r := rect(anchorPos(), colOf(anchorCol.Peek(), c), q, c, span)
		var next []CellRange
		if keep {
			next = slices.Clone(selBase.Peek())
		}
		setRanges(append(next, r))
	}
	// pick selects the range r alone, with the anchor at (q, c).
	pick := func(q, c int, r CellRange, span dgSpan) {
		setRanges([]CellRange{r})
		setAnchor(q, c, span, nil)
	}
	// toggle adds the range r to the selection, or takes it out when every cell of it is
	// selected (whole says so), and puts the anchor at (q, c).
	toggle := func(q, c int, r CellRange, span dgSpan, whole bool) {
		cur := rangeSel.Peek()
		next, base := append(slices.Clone(cur), r), cur
		if whole {
			next = rangesMinus(cur, r)
			base = next
		}
		setRanges(next)
		setAnchor(q, c, span, base)
	}
	cellRange := func(q, c int) CellRange {
		return CellRange{Rows: keysIn(keyAt(q)), Columns: keysIn(cols[c].key())}
	}
	rowRange := func(q int) CellRange { return CellRange{Rows: keysIn(keyAt(q)), Columns: allKeys} }
	colRange := func(c int) CellRange { return CellRange{Rows: allKeys, Columns: keysIn(cols[c].key())} }
	selectAll := func() { setRanges([]CellRange{{Rows: allKeys, Columns: allKeys}}) }
	// moveCell moves to the cell at (q, c) and selects as moveRow does for rows: a
	// plain move selects the cell, Ctrl moves the focus only, Shift extends the
	// rectangle from the anchor cell, and Ctrl+Shift extends it and keeps the rest.
	moveCell := func(q, c int, ctrl, shift bool) {
		switch {
		case shift && g.cellMode == SelectionMultiple:
			extend(q, c, dgSpan{}, ctrl)
		case ctrl:
			// The anchor stays where it is.
		case g.cellMode != SelectionNone:
			pick(q, c, cellRange(q, c), dgSpan{})
		}
		place(q, c, false)
	}
	// moveRow moves to row q and column cl, and selects as the selection mode and
	// the modifiers say: a plain move selects the row, Ctrl moves the focus only,
	// and Shift extends the selection from the anchor.
	moveRow := func(q, cl int, ctrl, shift bool) {
		if g.cellOn {
			moveCell(q, cl, ctrl, shift)
			return
		}
		k := keyAt(q)
		switch {
		case shift && g.mode == SelectionMultiple:
			selectKeys(rangeKeys(anchorPos(), q))
		case ctrl:
			// The anchor stays where it is.
		case g.mode != SelectionNone:
			selectKeys([]string{k})
			anchor.Set(k)
		}
		place(q, cl, false)
	}
	// click selects the row at q for a press with the modifiers mods, or the cell at
	// (q, cl) when the grid selects cells.
	click := func(q, cl int, mods input.Modifiers) {
		if g.cellOn {
			ctrl := mods&input.ModCtrl != 0
			switch {
			case g.cellMode == SelectionMultiple && mods&input.ModShift != 0:
				extend(q, cl, dgSpan{}, ctrl)
			case g.cellMode == SelectionMultiple && ctrl:
				toggle(q, cl, cellRange(q, cl), dgSpan{}, curSel().has(keyAt(q), cols[cl].key()))
			case g.cellMode != SelectionNone:
				pick(q, cl, cellRange(q, cl), dgSpan{})
			}
			place(q, cl, false)
			return
		}
		k := keyAt(q)
		switch {
		case g.mode == SelectionMultiple && mods&input.ModShift != 0:
			selectKeys(rangeKeys(anchorPos(), q))
		case g.mode == SelectionMultiple && mods&input.ModCtrl != 0:
			toggleKey(k)
			anchor.Set(k)
		case g.mode != SelectionNone:
			selectKeys([]string{k})
			anchor.Set(k)
		}
		place(q, cl, false)
	}
	// pickRow selects the row at q from its header (see RowHeaders), with the modifiers
	// mods: alone, added or taken out with Ctrl, or the rows from the anchor's with
	// Shift. The anchor's range then has every column, so Shift extends whole rows. In
	// a grid that selects rows it is a click on the row.
	pickRow := func(q int, mods input.Modifiers) {
		c := colOf(cur.Peek().colKey, cur.Peek().col)
		ctrl, shift := mods&input.ModCtrl != 0, mods&input.ModShift != 0
		switch {
		case n == 0 || nc == 0:
		case !g.cellOn:
			click(q, c, mods)
		case g.cellMode != SelectionMultiple:
			pick(q, c, cellRange(q, c), dgSpan{})
		case shift:
			extend(q, c, dgSpan{cols: true}, ctrl)
		case ctrl:
			toggle(q, c, rowRange(q), dgSpan{cols: true}, curSel().wholeRow(keyAt(q)))
		default:
			pick(q, c, rowRange(q), dgSpan{cols: true})
		}
	}
	// pickColumn selects column c from its header (see HeaderSelects), as pickRow does a
	// row. The anchor's range then has every row, so Shift extends whole columns.
	pickColumn := func(c int, mods input.Modifiers) {
		if n == 0 || nc == 0 || !g.cellOn || g.cellMode != SelectionMultiple {
			return
		}
		q := min(max(cur.Peek().pos, 0), n-1)
		ctrl, shift := mods&input.ModCtrl != 0, mods&input.ModShift != 0
		switch {
		case shift:
			extend(q, c, dgSpan{rows: true}, ctrl)
		case ctrl:
			toggle(q, c, colRange(c), dgSpan{rows: true}, curSel().wholeColumn(cols[c].key()))
		default:
			pick(q, c, colRange(c), dgSpan{rows: true})
		}
	}
	// selectEverything selects every cell, or every row of a grid that selects rows, as
	// Ctrl+A and the corner of the row headers do.
	selectEverything := func() {
		switch {
		case n == 0:
		case g.cellOn && g.cellMode == SelectionMultiple:
			selectAll()
		case !g.cellOn && g.mode == SelectionMultiple:
			selectKeys(rangeKeys(0, n-1))
		}
	}
	// startEdit starts an edit of the cell at displayed position q and column cl,
	// with text in the editor. The editor takes focus when it is built.
	startEdit := func(q, cl int, text string) {
		sh.draft.Set(text)
		sh.seq++
		edit.Set(dgEdit{key: keyAt(q), col: cl, colKey: cols[cl].key(), seq: sh.seq})
		if cols[cl].Editor != nil {
			sh.wantFocus = true
			rt.requestFrame()
		} else {
			sh.editFocus.Focus()
		}
	}
	// editAt starts an edit of the cell at (q, cl) with the text it shows, or with
	// its Value when it has one.
	editAt := func(q, cl int) {
		text := ""
		if c := cols[cl]; c.Value != nil {
			text = c.Value(rows[sh.order[q]])
		} else if c.Text != nil {
			text = c.Text(rows[sh.order[q]])
		}
		startEdit(q, cl, text)
	}
	// commitEdit saves the text of the editor to the row being edited. An error
	// keeps the editor open with its message, which is announced. It reports
	// whether the edit was saved.
	commitEdit := func(text string) bool {
		e := edit.Peek()
		if e.key == "" {
			return false
		}
		q, ok := sh.posOf(e.key)
		if !ok {
			edit.Set(dgEdit{})
			return false
		}
		c := colOf(e.colKey, e.col)
		if err := cols[c].Edit(sh.data[sh.order[q]], text); err != nil {
			edit.Set(dgEdit{key: e.key, col: c, colKey: e.colKey, err: err.Error(), seq: e.seq})
			Announce(err.Error(), access.Assertive)
			return false
		}
		edit.Set(dgEdit{})
		return true
	}
	// saveEdit saves the edit and returns focus to the cell, when it was saved.
	saveEdit := func(text string) {
		if commitEdit(text) {
			sh.focus.FocusNow()
		}
	}
	// endEdit ends the edit without saving it, and Escape returns focus to the cell.
	endEdit := func(refocus bool) {
		edit.Set(dgEdit{})
		if refocus {
			sh.focus.FocusNow()
		}
	}
	// refocusEditor gives focus back to the editor of the edit that is open.
	refocusEditor := func() {
		if e := edit.Peek(); cols[colOf(e.colKey, e.col)].Editor != nil {
			sh.wantFocus = true
			rt.requestFrame()
		} else {
			sh.editFocus.Focus()
		}
	}
	// blurEdit saves the edit numbered seq when focus leaves its editor, as a
	// spreadsheet does. A save that fails keeps the editor open with its error, and
	// the editor takes focus back when the window has it. An editor that is not the
	// open edit (its edit has ended, or the next one has started) does nothing.
	blurEdit := func(seq int) {
		if e := edit.Peek(); e.key == "" || e.seq != seq {
			return
		}
		if !commitEdit(sh.draft.Peek()) && rt.focused.Peek() {
			refocusEditor()
		}
	}
	// saveHidden saves the edit numbered seq with text after the frame, when a collapse
	// has hidden its row: the row is found in the tree, and saved as a blur saves it. The
	// editor is not shown for a hidden row, so an error is announced and the edit ends,
	// and the collapse is not held back.
	saveHidden := func(seq int, text string) {
		rt.Post(func() {
			e := edit.Peek()
			if e.key == "" || e.seq != seq {
				return
			}
			edit.Set(dgEdit{})
			if i, ok := sh.tree.at[e.key]; ok {
				if err := cols[colOf(e.colKey, e.col)].Edit(sh.tree.rows[i], text); err != nil {
					Announce(err.Error(), access.Assertive)
				}
			}
		})
	}
	if store != nil && ed.key != "" {
		if _, shown := sh.posOf(ed.key); !shown {
			if _, ok := sh.tree.at[ed.key]; ok {
				saveHidden(ed.seq, sh.draft.Peek())
			}
		}
	}
	// tabEdit saves the edit and starts one in the next editable cell of the row
	// in direction dir. At the last editable cell the edit ends, and focus stays
	// in the grid on that cell.
	tabEdit := func(dir int) {
		e := edit.Peek()
		if !commitEdit(sh.draft.Peek()) {
			return
		}
		if q, ok := sh.posOf(e.key); ok {
			for c := colOf(e.colKey, e.col) + dir; c >= 0 && c < nc; c += dir {
				if cols[c].Edit != nil {
					place(q, c, false)
					editAt(q, c)
					return
				}
			}
		}
		sh.focus.FocusNow()
	}
	// controls returns the focusable controls of the cell at (q, cl), in tree
	// order, or nil when the cell is not built.
	controls := func(q, cl int) []*node {
		c := dgCellNode(rt.byID[sh.focus.id], q+1, cl+1)
		if c == nil {
			return nil
		}
		return dgControls(c, nil)
	}
	// cycle moves focus to the next control of the cursor cell, or to the previous
	// one when back is set, and wraps around the cell.
	cycle := func(back bool) {
		ctl := controls(p, col)
		if len(ctl) == 0 {
			return
		}
		i := -1
		for j, c := range ctl {
			if c.id == rt.focusID {
				i = j
			}
		}
		switch {
		case back && i <= 0:
			i = len(ctl) - 1
		case back:
			i--
		default:
			i = (i + 1) % len(ctl)
		}
		rt.setFocus(ctl[i].id)
	}
	// enter moves focus to the first control of the cell at (q, cl). It reports
	// whether the cell has one.
	enter := func(q, cl int) bool {
		ctl := controls(q, cl)
		if len(ctl) == 0 {
			return false
		}
		rt.setFocus(ctl[0].id)
		return true
	}
	// cellAction is what Enter and F2 do on the cell at (q, cl): an editable cell
	// starts an edit, and an interactive one moves focus to its first control.
	cellAction := func(q, cl int) bool {
		switch {
		case cols[cl].Edit != nil:
			editAt(q, cl)
			return true
		case cols[cl].Interactive:
			return enter(q, cl)
		}
		return false
	}
	// cellKeys handles a key press while focus is on a control of a cell or on an
	// editor, rather than on the grid. Tab moves through the controls of the cell
	// or saves the edit and moves to the next editable cell. Escape and F2 return
	// to the grid. Every other key belongs to the control, or to the editor.
	// cursorNow returns the cell the cursor is on now. A key handler can run before the
	// grid is built again, and the render's copy of the cursor would then be old: two arrow
	// keys before the next frame must move two cells, so the handlers read the signal.
	cursorNow := func() (p, col int, onHeader bool) {
		c := cur.Peek()
		return min(max(c.pos, 0), max(n-1, 0)), colOf(c.colKey, c.col), c.header
	}
	cellKeys := func(e *KeyEvent) bool {
		switch e.Key {
		case input.KeyTab:
			if e.Repeat {
				return true
			}
			dir := 1
			if e.Mods&input.ModShift != 0 {
				dir = -1
			}
			if ed.key != "" {
				tabEdit(dir)
			} else {
				cycle(dir < 0)
			}
			return true
		case input.KeyEscape:
			// A custom editor's own keys came first: an open list takes Escape to close
			// itself. What reaches the grid cancels the edit.
			if edit.Peek().key != "" {
				if !e.Repeat {
					endEdit(true)
				}
				return true
			}
			if e.Repeat {
				return false
			}
			sh.focus.FocusNow()
			return true
		case input.KeyF2:
			if edit.Peek().key != "" || e.Repeat {
				return false
			}
			sh.focus.FocusNow()
			return true
		case input.KeyEnter:
			// The text field saves on Enter by its own binding, so this is a custom
			// editor's Enter that its controls did not take.
			if edit.Peek().key == "" || e.Repeat {
				return false
			}
			saveEdit(sh.draft.Peek())
			return true
		}
		return false
	}
	// typeText starts an edit of an editable cell with the character typed, which
	// replaces its text. While an edit is starting, the text is added to its draft.
	typeText := func(s string) {
		p, col, onHeader := cursorNow()
		if edit.Peek().key != "" {
			sh.draft.Set(sh.draft.Peek() + s)
			return
		}
		if s == "*" && store != nil && !onHeader && n > 0 {
			g.openSiblings(store, sh, p)
			return
		}
		r, _ := utf8.DecodeRuneInString(s)
		// A custom editor is started by Enter or F2, not by typing: its value is not text.
		if !sh.focus.Focused() || onHeader || cur.Peek().rowHead || n == 0 || nc == 0 || cols[col].Edit == nil || cols[col].Editor != nil ||
			r == ' ' || !unicode.IsPrint(r) {
			return
		}
		startEdit(p, col, s)
	}

	// moveHead moves the cursor to the header of row q (see RowHeaders). Shift extends
	// whole rows from the anchor, and Ctrl+Shift keeps the rest of the selection; a plain
	// move only moves. In a grid that selects rows the move selects as on the row.
	moveHead := func(q int, ctrl, shift bool) {
		c := colOf(cur.Peek().colKey, cur.Peek().col)
		switch {
		case !g.cellOn:
			moveRow(q, c, ctrl, shift)
		case shift && g.cellMode == SelectionMultiple:
			extend(q, c, dgSpan{cols: true}, ctrl)
		}
		placeHead(q, false)
	}
	// headKeys handles a key press while the cursor is on a row's header, or on the
	// corner when onHeader is set (see RowHeaders), and reports whether it used the key.
	// The keys it does not use act as they do on the first column.
	headKeys := func(e *KeyEvent, p int, onHeader, ctrl, shift bool, step int) bool {
		switch e.Key {
		case input.KeyLeft:
		case input.KeyRight:
			switch {
			case nc == 0:
			case onHeader:
				place(p, 0, true)
			case n > 0:
				moveRow(p, 0, ctrl, shift)
			}
		case input.KeyUp:
			switch {
			case onHeader || n == 0:
			case p == 0:
				placeHead(0, true)
			default:
				moveHead(p-1, ctrl, shift)
			}
		case input.KeyDown:
			switch {
			case n == 0:
			case onHeader:
				placeHead(0, false)
			case p+1 < n:
				moveHead(p+1, ctrl, shift)
			default:
				askMore()
			}
		case input.KeyPageDown, input.KeyPageUp:
			switch {
			case n == 0:
			case e.Key == input.KeyPageUp && !onHeader:
				moveHead(max(p-step, 0), ctrl, shift)
			case e.Key == input.KeyPageDown:
				if !onHeader && p == n-1 {
					askMore()
				}
				moveHead(min(p+step, n-1), ctrl, shift)
			}
		case input.KeyHome, input.KeyEnd:
			if !ctrl {
				return false // Home and End move to the first and last cell of the row
			}
			if n > 0 && e.Key == input.KeyEnd {
				moveHead(n-1, ctrl, shift)
			} else if n > 0 {
				moveHead(0, ctrl, shift)
			}
		case input.KeySpace, input.KeyEnter:
			switch {
			case e.Repeat:
			case onHeader:
				selectEverything()
			case n == 0:
			case e.Key == input.KeyEnter:
				activate(p)
			default:
				pickRow(p, e.Mods)
			}
		case input.KeyF2:
		default:
			return false
		}
		return true
	}

	// keys handles a key press on the grid, and reports whether it used the key.
	keys := func(e *KeyEvent) bool {
		p, col, onHeader := cursorNow()
		if e.Mods&input.ModAlt != 0 {
			return false
		}
		if !sh.focus.Focused() {
			return cellKeys(e)
		}
		if edit.Peek().key != "" {
			// The editor is about to take focus, or the focus is back on the grid after a
			// save that failed; Escape ends the edit either way.
			if e.Key == input.KeyEscape && !e.Repeat {
				endEdit(true)
			}
			return true
		}
		ctrl := e.Mods&input.ModCtrl != 0
		shift := e.Mods&input.ModShift != 0
		step := max(int((sh.viewH-headerH)/rowH), 1)
		if g.rowHeaders && cur.Peek().rowHead && headKeys(e, p, onHeader, ctrl, shift, step) {
			return true
		}
		switch e.Key {
		case input.KeyLeft, input.KeyRight:
			dir := 1
			if e.Key == input.KeyLeft {
				dir = -1
			}
			if onHeader && ctrl && shift && nc > 0 && movable(cols[col]) {
				stepColumn(cols[col].key(), dir)
				return true
			}
			if onHeader && ctrl {
				if nc > 0 && cols[col].Resizable && setWidth(col, cw[col]+8*float32(dir)) {
					columnsChanged()
				}
				return true
			}
			if store != nil && !onHeader && col == 0 && nc > 0 && n > 0 && !ctrl && !shift {
				if to, moves, used := g.treeKey(store, sh, p, dir); used {
					if moves {
						place(to, col, false)
					}
					return true
				}
			}
			if g.rowHeaders && dir < 0 && col == 0 && !ctrl && !shift {
				placeHead(p, onHeader) // Left from the first column moves to the row's header
				return true
			}
			if onHeader && shift && g.headSelects && g.cellOn && nc > 0 {
				// Shift+Left and Shift+Right on a header extend the columns selected.
				c := min(max(col+dir, 0), nc-1)
				place(p, c, true)
				pickColumn(c, input.ModShift)
				return true
			}
			switch {
			case nc == 0:
			case g.cellOn && !onHeader && n > 0:
				moveCell(p, min(max(col+dir, 0), nc-1), ctrl, shift)
			default:
				place(p, min(max(col+dir, 0), nc-1), onHeader)
			}
		case input.KeyHome, input.KeyEnd:
			if nc == 0 {
				return true
			}
			last := e.Key == input.KeyEnd
			switch {
			case ctrl && n > 0 && last:
				if !onHeader && p == n-1 {
					askMore()
				}
				moveRow(n-1, nc-1, ctrl, shift && g.cellOn)
			case ctrl && n > 0:
				moveRow(0, 0, ctrl, shift && g.cellOn)
			case g.cellOn && !onHeader && n > 0 && last:
				moveCell(p, nc-1, false, shift)
			case g.cellOn && !onHeader && n > 0:
				moveCell(p, 0, false, shift)
			case last:
				place(p, nc-1, onHeader)
			default:
				place(p, 0, onHeader)
			}
		case input.KeyDown:
			switch {
			case n == 0:
			case onHeader:
				moveRow(0, col, ctrl, shift)
			case p+1 < n:
				moveRow(p+1, col, ctrl, shift)
			default:
				askMore() // Down on the last row asks for more
			}
		case input.KeyUp:
			switch {
			case onHeader || n == 0:
			case p == 0:
				place(p, col, true)
			default:
				moveRow(p-1, col, ctrl, shift)
			}
		case input.KeyPageDown:
			if n > 0 && !onHeader && p == n-1 {
				askMore()
			}
			if n > 0 {
				moveRow(min(p+step, n-1), col, ctrl, shift)
			}
		case input.KeyPageUp:
			if n > 0 && !onHeader {
				moveRow(max(p-step, 0), col, ctrl, shift)
			}
		case input.KeyF2:
			if e.Repeat || onHeader || n == 0 {
				return false
			}
			return cellAction(p, col)
		case input.KeyEnter, input.KeySpace:
			if e.Repeat {
				return true
			}
			if onHeader {
				// With HeaderSelects, Space selects the column and Enter sorts.
				if e.Key == input.KeySpace && g.headSelects && g.cellOn {
					pickColumn(col, e.Mods)
				} else {
					sortBy(col)
				}
				return true
			}
			if n == 0 {
				return true
			}
			if e.Key == input.KeyEnter && cellAction(p, col) {
				return true
			}
			k := keyAt(p)
			switch {
			case e.Key == input.KeyEnter:
				activate(p)
			case g.cellOn && g.cellMode == SelectionMultiple && shift:
				pick(p, col, rowRange(p), dgSpan{cols: true}) // the row
			case g.cellOn && g.cellMode == SelectionMultiple && ctrl:
				pick(p, col, colRange(col), dgSpan{rows: true}) // the column
			case g.cellOn && g.cellMode == SelectionMultiple:
				toggle(p, col, cellRange(p, col), dgSpan{}, curSel().has(k, cols[col].key()))
			case g.cellOn && g.cellMode == SelectionSingle:
				pick(p, col, cellRange(p, col), dgSpan{})
			case g.cellOn:
			case g.mode == SelectionMultiple && shift:
				selectKeys(rangeKeys(anchorPos(), p))
			case g.mode == SelectionMultiple:
				toggleKey(k)
				anchor.Set(k)
			case g.mode == SelectionSingle:
				selectKeys([]string{k})
				anchor.Set(k)
			}
		case input.KeyA:
			if g.cellOn && ctrl && g.cellMode == SelectionMultiple && n > 0 && nc > 0 {
				selectAll()
				return true
			}
			if !ctrl || g.mode != SelectionMultiple || n == 0 {
				return false
			}
			selectKeys(rangeKeys(0, n-1))
		default:
			return false
		}
		return true
	}

	// The rows the list builds in this frame are noted for the auto-fit.
	sh.builtLo, sh.builtHi = 0, 0
	ops := dgOps{
		pick:   func(q, cl int) { place(q, cl, false) },
		edit:   editAt,
		save:   saveEdit,
		cancel: func() { endEdit(true) },
		blur:   blurEdit,
	}
	// colAt returns the column of the cell of row q under the window's x. The cells are
	// sized by their flex layout, where a Grow column takes the spare width, so the
	// column is found in the laid-out cells and not from the widths. The pinned cells
	// are drawn over the others, so they are looked at first. A point right of the
	// last cell is in the last column.
	colAt := func(q int, x float32) int {
		body := rt.byID[sh.focus.id]
		rect := func(c int) (scene.Rect, bool) {
			cell := dgCellNode(body, q+1, c+1)
			if cell == nil {
				return scene.Rect{}, false
			}
			return rt.nodeRect(cell), true
		}
		for _, c := range append(seq(nc-endN, nc), seq(0, startN)...) {
			if r, ok := rect(c); ok && x >= r.X && x < r.X+r.W {
				return c
			}
		}
		if virtual {
			// Not every cell is built, and no column grows, so the column is found from
			// the widths.
			b := rt.nodeRect(body)
			be := body.style.borderEdges()
			cx := x - b.X - be.Left - body.style.Padding.Left + body.st.scrollX
			for c := startN; c < nc-1; c++ {
				if cx < xs[c+1] {
					return c
				}
			}
			return max(nc-1, 0)
		}
		for c := startN; c < nc-1; c++ {
			if r, ok := rect(c); ok && x < r.X+r.W {
				return c
			}
		}
		return max(nc-1, 0)
	}
	// cellAt returns the displayed position and the column of the cell under the
	// window's (x, y), and reports whether there is one.
	cellAt := func(x, y float32) (q, c int, ok bool) {
		for _, nd := range rt.hitTest(x, y) {
			d := nd.div
			switch {
			case d == nil || d.acc == nil:
			case d.part == "DataGrid.Cell" && d.acc.gridRow > 0:
				return d.acc.gridRow - 1, d.acc.gridCol - 1, true
			case d.part == "DataGrid.Row" && d.acc.gridRow > 0:
				return d.acc.gridRow - 1, colAt(d.acc.gridRow-1, x), true
			}
		}
		return 0, 0, false
	}
	// dragPoint returns the row and the column of the cells under the window's (x, y),
	// for a drag of the selection: a point past an edge of the rows in view, or outside
	// the grid, takes the nearest row and column in view, so that the drag follows the
	// pointer on one axis while the other is outside. The rows are found from their
	// height and the scroll, not by a hit test, so a row that is not built is found too.
	dragPoint := func(x, y float32) (q, c int, ok bool) {
		body := rt.byID[sh.focus.id]
		if body == nil || body.st == nil || n == 0 || nc == 0 || sh.builtHi <= sh.builtLo {
			return 0, 0, false
		}
		r := rt.nodeRect(body)
		be := body.style.borderEdges()
		top, bottom := r.Y+be.Top+headerH, r.Bottom()-be.Bottom
		y = min(max(y, top), max(bottom-1, top))
		q = min(max(int((y-top+body.st.scrollY)/rowH), 0), n-1)
		left, right := r.X+be.Left+rowHeadW, r.X+be.Left+sh.visW
		x = min(max(x, left), max(right-1, left))
		// The cells of any built row lie where those of every row do.
		c = colAt(min(max(q, sh.builtLo), sh.builtHi-1), x)
		return q, c, true
	}
	// cursorTo moves the cursor to (q, c), on the header when header is set, without
	// scrolling: a drag of the selection scrolls by itself (see dragScroll).
	cursorTo := func(q, c int, header bool) {
		k := ""
		if !header {
			k = keyAt(q)
		}
		cur.Set(dgCursor{key: k, pos: q, col: c, colKey: cols[c].key(), header: header})
		handle.request()
	}
	// startDrag starts a drag of the selection of the kind kind from (q, c), after a
	// press with the modifiers mods has selected there. Only a multiple selection of
	// cells drags.
	startDrag := func(kind uint8, q, c int, mods input.Modifiers) {
		if g.cellOn && g.cellMode == SelectionMultiple {
			sh.selDrag, sh.dragQ, sh.dragC, sh.dragKeep = kind, q, c, mods&input.ModCtrl != 0
			sh.dragAt = time.Time{}
		}
	}
	// dragTo extends the drag of the selection to the window's (x, y), as Shift+click
	// there would: over the cells a rectangle, down the row headers whole rows, and across
	// the column headers whole columns. The grid's body holds the pointer while the
	// button is down, so a drag ends when it no longer does.
	dragTo := func(x, y float32) {
		if sh.selDrag != dgDragNone && rt.captureID != sh.focus.id {
			sh.selDrag = dgDragNone
		}
		if sh.selDrag == dgDragNone {
			return
		}
		q, cl, ok := dragPoint(x, y)
		if !ok {
			return
		}
		c := colOf(cur.Peek().colKey, cur.Peek().col)
		switch sh.selDrag {
		case dgDragCells:
			if q != sh.dragQ || cl != sh.dragC {
				sh.dragQ, sh.dragC = q, cl
				extend(q, cl, dgSpan{}, sh.dragKeep)
				cursorTo(q, cl, false)
			}
		case dgDragRows:
			if q != sh.dragQ {
				sh.dragQ = q
				extend(q, c, dgSpan{cols: true}, sh.dragKeep)
				cursorTo(q, c, false)
			}
		case dgDragCols:
			if cl != sh.dragC {
				sh.dragC = cl
				p := min(max(cur.Peek().pos, 0), n-1)
				extend(p, cl, dgSpan{rows: true}, sh.dragKeep)
				cursorTo(p, cl, cur.Peek().header)
			}
		}
	}
	// dragScroll scrolls the rows, and the columns, toward the pointer while a drag of
	// the selection holds it past an edge of the rows in view, as a spreadsheet does:
	// from nothing at the edge to dragScrollSpeed a zone beyond it, as a drag of an
	// element scrolls (see edgeSpeed). It runs at each paint of the body nd, and asks for
	// the next frame while it scrolls; the selection then extends to the row or column
	// that came into view.
	dragScroll := func(nd *node) {
		if sh.selDrag == dgDragNone || rt.captureID != nd.id {
			return
		}
		dt := float32(0.016)
		if !sh.dragAt.IsZero() {
			dt = min(max(float32(rt.lastFrame.Sub(sh.dragAt).Seconds()), 0), 0.1)
		}
		sh.dragAt = rt.lastFrame
		r := rt.nodeRect(nd)
		be := nd.style.borderEdges()
		moved := false
		step := func(pos, lo, hi, maxScroll float32, target, scroll *float32) {
			v := edgeSpeed(pos, lo-dragScrollZone, hi+dragScrollZone)
			if next := min(max(*target+v*dragScrollSpeed*dt, 0), max(maxScroll, 0)); v != 0 && next != *target {
				*target, *scroll = next, next
				moved = true
			}
		}
		if sh.selDrag != dgDragCols {
			step(rt.mouseY, r.Y+be.Top+headerH, r.Bottom()-be.Bottom, nd.lay.ContentH-nd.lay.H, &nd.st.targetY, &nd.st.scrollY)
		}
		if sh.selDrag != dgDragRows && total > sh.visW {
			lo := r.X + be.Left + startW
			step(rt.mouseX, lo, lo+sh.visW-startW-endW, nd.lay.ContentW-nd.lay.W, &nd.st.targetX, &nd.st.scrollX)
		}
		if moved {
			rt.animating, rt.needPaint, rt.scrolled = true, true, true
			rt.Post(func() { dragTo(rt.mouseX, rt.mouseY) })
		}
	}
	dragEnd := func() { sh.selDrag = dgDragNone }
	// pressRow selects the row at q for a press on its header (see RowHeaders), and moves
	// the cursor to the row, in its column.
	pressRow := func(q int, mods input.Modifiers) {
		if q >= n || nc == 0 {
			return
		}
		pickRow(q, mods)
		c := colOf(cur.Peek().colKey, cur.Peek().col)
		place(q, c, false)
		startDrag(dgDragRows, q, c, mods)
	}
	// pressColumn selects column c for a press on its header (see HeaderSelects).
	pressColumn := func(c int, mods input.Modifiers) {
		if c >= nc || n == 0 {
			return
		}
		pickColumn(c, mods)
		focusCol(c)
		startDrag(dgDragCols, cur.Peek().pos, c, mods)
	}
	// menu builds the context menu of the grid. The menu is built before the press
	// reaches the rows, so a press finds its cell itself, and moves the cursor there
	// and selects the row as the rule of File Explorer says. The keyboard opens the
	// menu of the cell, or the header cell, that the cursor is on.
	menu := func() []MenuItem {
		if nc == 0 {
			return nil
		}
		if !rt.keyboardModality {
			q, c, ok := cellAt(rt.mouseX, rt.mouseY)
			if !ok || g.rowMenu == nil || q >= n {
				return nil
			}
			switch {
			case g.cellOn && g.cellMode != SelectionNone && !curSel().has(keyAt(q), cols[c].key()):
				click(q, c, 0)
			case !g.cellOn && g.mode != SelectionNone && !sh.selection(sel.Peek())[keyAt(q)]:
				click(q, c, 0)
			default:
				place(q, c, false)
			}
			return g.rowMenu(rows[sh.order[q]], cols[c].key())
		}
		p, c, onHeader := cursorNow()
		switch {
		case onHeader && g.headerMenu != nil:
			return g.headerMenu(cols[c].key())
		case onHeader || g.rowMenu == nil || n == 0:
			return nil
		}
		return g.rowMenu(rows[sh.order[p]], cols[c].key())
	}
	// copySelection puts the selection on the clipboard as text (see CopyText), with the
	// headers of its columns first when headers is set. A grid that selects rows copies
	// its rows, and with nothing selected the copy is the focused cell.
	copySelection := func(headers bool) {
		if n == 0 || nc == 0 {
			return
		}
		var rs []CellRange
		switch {
		case g.cellOn:
			rs = rangeSel.Peek()
		case g.mode != SelectionNone && len(sel.Peek()) > 0:
			rs = []CellRange{{Rows: keysIn(sel.Peek()...), Columns: allKeys}}
		}
		src := dgCopySource[T]{n: len(sh.order), row: func(q int) T { return rows[sh.order[q]] }, key: keyAt, cols: cols, sel: rs}
		c, cells := copyOf(src, headers, g.copyText != nil)
		if cells == 0 {
			q := min(max(cur.Peek().pos, 0), n-1)
			src.sel = []CellRange{cellRange(q, colOf(cur.Peek().colKey, cur.Peek().col))}
			c, cells = copyOf(src, headers, g.copyText != nil)
		}
		var text string
		switch {
		case g.copyText != nil:
			text = g.copyText(c)
		case cells > dataGridCopyMax:
			Announce(strs.CopyTooLarge(cells, dataGridCopyMax), access.Assertive)
			return
		default:
			text = c.Text()
		}
		if text != "" {
			rt.setClipboard(text)
		}
	}
	if h := g.gridHandle; h != nil {
		h.copy = copySelection
	}
	rc := dgRowCtx[T]{t: t, ctx: ctx, st: &st, sh: sh, rows: rows, cols: cols, xs: xs, selected: selected, rowH: rowH,
		cursorPos: p, cursorCol: col, onHeader: onHeader, hasFocus: hasFocus, ed: ed, startN: startN, endN: endN,
		built: built, cells: cellsOn, rowHeadW: rowHeadW, onRowHead: onRowHead, click: click, colAt: colAt, activate: activate, ops: ops, store: store}
	sh.live = dgLive{click: click, colAt: colAt, activate: activate, ops: ops, cursorCol: col,
		setOpen: func(key string, v bool) { g.setOpen(store, sh, key, v) },
		rowHead: pressRow,
		press: func(q, c int, mods input.Modifiers) {
			click(q, c, mods)
			startDrag(dgDragCells, q, c, mods)
		}}
	rc.memoOK = g.prepareRowMemo(&rc)
	// A row within a viewport of rows of the end asks for more rows when it is built.
	near := n - max(int(sh.viewH/rowH), 1)
	list := UniformList(n, func(q int) Element {
		if sh.builtHi == 0 {
			sh.builtLo = q
		}
		sh.builtHi = q + 1
		if q >= near {
			askMore()
		}
		return g.rowFor(&rc, q)
	}).Key(g.keyStr).Handle(handle)
	// The rows report their positions themselves: a tree row's is among its siblings,
	// and a kept row's is updated by place.
	list.ownPos = true
	if g.loadingMore && n > 0 {
		// The row of LoadingMore follows the rows. Its ring names it, and its text is
		// for sighted users.
		list.tail = Div().styleOf(&st.Row).styleOf(&Style{Gap: 8, Padding: Edges{Left: 8}}).
			Child(ProgressRing().Size(RingSmall).Label(strs.LoadingMore)).
			Child(Div().AccessHidden().Child(Text(strs.LoadingMore).Truncate()))
		if !sh.loadingMore {
			Announce(strs.LoadingMore, access.Polite)
		}
	}
	sh.loadingMore = g.loadingMore && n > 0
	// The scrollbar is drawn over the grid's right edge, so the rows end before it:
	// the body keeps dataGridGutter of padding there, and the rows are as wide as
	// the body less its border and that gutter, or as wide as the columns.
	width := max(total, sh.viewW-2-dataGridGutter)
	list.rowWidth = width
	list.decorate = func(el *DivElement) {
		focus := Style{Shadows: []Shadow{{Spread: 2, Color: t.FocusRing}}}.Merge(st.GridFocusVisible)
		el.asPart("DataGrid.Grid").Role(access.RoleDataGrid).ScrollX().
			styleOf(&st.Grid).styleOf(&Style{Padding: Edges{Top: headerH, Right: dataGridGutter}}).
			FocusHandle(sh.focus).focusVisibleOf(&focus).
			OnKeyDown(func(e *KeyEvent) {
				if keys(e) {
					e.StopPropagation()
				}
			}).
			KeyContext("DataGrid").
			// The body holds the pointer from a press until its release: a drag of the
			// selection follows it (see dragTo), and the release clears what the press set
			// for the click that follows, after that click, even when no click follows.
			CapturePointer().
			OnMouseMove(func(e *MouseEvent) { dragTo(e.X, e.Y) }).
			OnMouseUp(func(*MouseEvent) {
				dragEnd()
				rt.Post(func() { sh.pressed, sh.headPress = false, false })
			}).
			OnAction(dataGridCopy, func() { copySelection(false) }).
			OnAction(dataGridCopyHeaders, func() { copySelection(true) }).
			OnFocus(func() { focused.Set(true) }).
			OnBlur(func() { focused.Set(false) }).
			OnTextInput(typeText)
		if g.mode != SelectionNone || (g.cellOn && g.cellMode != SelectionNone) {
			el.SelectionContainer()
		}
		if g.name != "" {
			el.Label(g.name)
		}
		el.AccessGrid(n, nc)
		el.access().multiSelect = g.mode == SelectionMultiple || (g.cellOn && g.cellMode == SelectionMultiple)
	}
	list.before = func(nd *node) {
		sh.viewH = nd.lay.H
		dragScroll(nd)
		// A custom editor takes focus when it is painted, on its first control; until
		// then the focus is not inside the cell, so the blur check below waits. When
		// focus has left the editing cell, the edit is saved, as a text field's is.
		if e := edit.Peek(); e.key != "" && cols[colOf(e.colKey, e.col)].Editor != nil {
			if q, ok := sh.posOf(e.key); ok {
				if cell := dgCellNode(nd, q+1, colOf(e.colKey, e.col)+1); cell != nil {
					switch {
					case sh.wantFocus:
						sh.wantFocus = false
						if ctl := dgControls(cell, nil); len(ctl) > 0 {
							rt.setFocus(ctl[0].id)
						}
					case !rt.focusInside(cell.id):
						blurEdit(e.seq)
					}
				}
			}
		}
		if nd.lay.W != sh.viewW {
			// The rows and the header are as wide as the grid or the columns, so a
			// new width rebuilds them.
			sh.viewW = nd.lay.W
			inst.invalidate()
			rt.listStale = true
		}
		sh.visW = dgVisibleWidth(nd)
		if sh.reveal {
			sh.reveal = false
			// The column is brought into the part of the view that the pinned columns
			// leave free.
			x0, x1 := sh.revealX0, sh.revealX1
			switch {
			case x0 < nd.st.targetX+startW:
				nd.st.targetX = x0 - startW
			case x1 > nd.st.targetX+sh.visW-endW:
				nd.st.targetX = x1 - sh.visW + endW
			}
			rt.animating = true
		}
		// The columns in view, at the current offset and at the scroll target, with two
		// more on each side. When the built columns do not cover them, the rows are
		// built again.
		if total <= sh.visW || nc-endN <= startN {
			return
		}
		lo := min(nd.st.scrollX, nd.st.targetX) + startW
		hi := max(nd.st.scrollX, nd.st.targetX) + sh.visW - endW
		vf := startN
		for vf < nc-endN-1 && xs[vf+1] <= lo {
			vf++
		}
		vt := vf
		for vt < nc-endN && xs[vt] < hi {
			vt++
		}
		const ov = 2
		if !virtual || vf < sh.colFrom || vt > sh.colTo || sh.colFrom < vf-3*ov || sh.colTo > vt+3*ov {
			sh.colFrom, sh.colTo, sh.colSet = max(vf-ov, startN), min(vt+ov, nc-endN), true
			inst.invalidate()
			rt.listStale = true
		}
	}
	// The pieces of the row of the groups of columns.
	var runs []dgGroupRun
	if headerH > rowH {
		runs = groupRuns(cols)
	}
	if startN+endN > 0 || rowHeadW > 0 {
		// The pinned cells, the row headers and the corner, and the row of LoadingMore, are moved by the sideways
		// scroll before they are painted, so that they stay where they are on the
		// screen. The cells are drawn over the cells that scroll under them, and they
		// are not culled at the place their layout gives them.
		list.pin = func(nd *node, sx float32) {
			// The end of the rows stops short of the body's right padding, the gutter the
			// scrollbar takes, so the end-pinned cells are moved over it to the inner edge.
			endOff := sx - max(nd.lay.ContentW-nd.lay.W, 0) + nd.style.Padding.Right
			move := func(c *node, off float32) {
				c.style.OffsetX, c.div.style.OffsetX = off, off
				c.keepGen = rt.paintGen
			}
			pinOf := func(c int) ColumnPin {
				if c < 0 || c >= nc {
					return PinNone
				}
				return cols[c].Pin
			}
			offset := func(c *node, p ColumnPin) {
				switch p {
				case PinStart:
					move(c, sx)
				case PinEnd:
					move(c, endOff)
				}
			}
			if len(nd.children) > 1 {
				// The header is the row of header cells, or a column of the row of the
				// groups, whose pieces are those of groupRuns, and that row.
				hdr := nd.children[1]
				if headerH > rowH && len(hdr.children) == 2 {
					pieces := hdr.children[0].children
					if rowHeadW > 0 && len(pieces) > 0 {
						move(pieces[0], sx) // the space over the row headers
						pieces = pieces[1:]
					}
					for i, c := range pieces {
						if i < len(runs) {
							offset(c, runs[i].pin)
						}
					}
					hdr = hdr.children[1]
				}
				i := 0
				for _, c := range hdr.children {
					switch {
					case c.div == nil:
					case c.div.part == "DataGrid.HeaderCell":
						offset(c, pinOf(i))
						i++
					case c.div.part == "DataGrid.Corner":
						move(c, sx)
					}
				}
			}
			for _, w := range nd.children[0].children {
				if w.key == "jamb.list.tail" && len(w.children) > 0 {
					move(w.children[0], sx)
					continue
				}
				for _, r := range w.children {
					for _, c := range r.children {
						switch d := c.div; {
						case d == nil:
						case d.part == "DataGrid.Cell" && d.acc != nil:
							offset(c, pinOf(d.acc.gridCol-1))
						case d.part == "DataGrid.RowHeader":
							move(c, sx)
						}
					}
				}
			}
		}
	}
	ho := dgHeadOps[T]{
		changed: columnsChanged,
		widthOf: func(cl DataColumn[T]) float32 { return cl.width(widths.Peek()) },
		movable: movable,
		accepts: func(from, to string) bool {
			return from != to && cols[colOf(from, -1)].Pin == cols[colOf(to, -1)].Pin
		},
		drop: dropColumn,
		mark: colMark.Get(),
		setMark: func(m dgColMark) {
			if colMark.Peek() != m {
				colMark.Set(m)
			}
		},
		// A press on the edge of a column resizes it: it does not drag the header.
		stopDrag: func() {
			if !rt.drag.active {
				rt.drag = dragState{}
			}
		},
		post:   rt.Post,
		startN: startN,
		endN:   endN,
		runs:   runs,
		// The selection from the headers.
		selects:  g.headSelects && g.cellOn && g.cellMode == SelectionMultiple,
		cellSel:  g.cellOn,
		pickCol:  pressColumn,
		whole:    func(c int) bool { return g.cellOn && curSel().wholeColumn(cols[c].key()) },
		rowHeadW: rowHeadW,
		corner: func() {
			selectEverything()
			sh.focus.Focus()
		},
		onCorner: onRowHead && onHeader,
	}
	list.header = g.headerRow(&st, sh, rows, cols, cw, width, sortState, sortCol, col, onHeader, hasFocus,
		t.HighContrast, sortBy, setWidth, func(i int) float32 { return autoFitWidth(rt, &st, sh, rows, cols, i) }, ho, strs)

	// The column may shrink below its rows, so that the list windows them to the space
	// instead of building them all (see Tree).
	root := g.size.apply(Div().Column().MinHeight(Px(0)).styleOf(&g.style)).Child(list)
	if g.rowMenu != nil || g.headerMenu != nil {
		// The menu hangs from the grid's outer box, which does not scroll, so that it
		// opens at the pointer however far the rows are scrolled.
		root.ContextMenu(menu)
	}
	if n == 0 {
		msg := g.empty
		if msg == nil {
			msg = Text(strs.NoRows)
		}
		root.Child(Div().asPart("DataGrid.Empty").Absolute().Top(headerH).Left(0).Right(0).styleOf(&st.Empty).Child(msg))
	}
	if g.loading {
		root.Child(Div().Absolute().Top(0).Right(0).Bottom(0).Left(0).Column().Align(Center).Justify(Center).
			Child(ProgressRing().Label(strs.Loading)))
	}
	return root
}

// row builds the row at displayed position q: a list item whose cells are the
// cells of the columns. A selected row is drawn in RowSelected: its fill (the Highlight
// color under high contrast) is the selection, and its text keeps its weight.
// prepareRowMemo makes ready the rows kept from one build to the next (see rowFor),
// and reports whether rows can be kept at all. A grid whose cells hold content of
// the app's own, an editor or controls, or that selects cells, builds every row
// each time: the closures its cells capture cannot be compared. When what every row
// is built from has changed, no row is kept. Rows whose instances are gone are
// forgotten.
func (g *DataGridElement[T]) prepareRowMemo(rc *dgRowCtx[T]) bool {
	sh := rc.sh
	sh.builds++
	for k, m := range sh.rowMemo {
		if m.el != nil && !m.live() || m.el == nil && m.seen < sh.builds-1 {
			delete(sh.rowMemo, k)
		}
	}
	if g.cellOn {
		return false
	}
	look := dgRowLook{theme: rc.t, highContrast: rc.t.HighContrast, xs: rc.xs, built: rc.built,
		startN: rc.startN, endN: rc.endN, striped: g.striped, mode: g.mode, tree: sh.tree != nil, rowHeadW: rc.rowHeadW}
	copyLarge(&look.st, rc.st)
	look.cols = make([]dgColLook, len(rc.cols))
	for c, cl := range rc.cols {
		if cl.Cell != nil || cl.Edit != nil || cl.Editor != nil || cl.Interactive {
			return false
		}
		look.cols[c] = dgColLook{key: cl.key(), grow: cl.Grow, align: cl.Align, pin: cl.Pin,
			cellStyle: cl.CellStyle, text: cl.Text != nil}
	}
	if !look.same(&sh.look) {
		copyLarge(&sh.look, &look)
		sh.lookGen++
	}
	return true
}

// rowFor returns the element of the row at position q. A row that would be built
// the same as at the last build, from the same row of text in the same state, is
// kept: its element is returned again, so that its instance keeps its nodes, and
// only its position is updated (see dgRowMemo.place). So a grid that renders again,
// for a new slice of rows, a click or a move of the cursor, builds only the rows
// that changed.
//
// A row's handlers act on the position the row was last built or placed at, through
// the closures of the last build (see dgLive), so a kept row acts as a new one would.
func (g *DataGridElement[T]) rowFor(rc *dgRowCtx[T], q int) Element {
	sh := rc.sh
	i := sh.order[q]
	k := sh.keys[i]
	sig := dgRowSig{gen: sh.lookGen, selected: rc.selected[k], cursor: -1, odd: g.striped && q%2 == 1}
	if !rc.onHeader && q == rc.cursorPos {
		sig.cursor, sig.focused = rc.cursorCol, rc.hasFocus
		if rc.onRowHead {
			sig.cursor = -2
		}
	}
	if rc.rowHeadW > 0 {
		sig.num = q + 1
	}
	sig.join = sig.selected && rc.selected[rc.keyBelow(q)]
	pos, size := q+1, sh.n
	if t := sh.tree; t != nil {
		pos, size = t.pos[i], t.size[i]
		sig.level, sig.kids, sig.open = t.level[i], len(t.kids[i]) > 0, sh.openSet[k]
	}
	texts := sh.texts[:0]
	for _, cl := range rc.cols {
		s := ""
		if cl.Text != nil {
			s = cl.Text(rc.rows[i])
		}
		texts = append(texts, s)
	}
	sh.texts = texts
	old := sh.rowMemo[k]
	if rc.memoOK && rc.ed.key != k && old != nil && old.sig == sig && slices.Equal(old.texts, texts) && old.live() {
		old.place(q, pos, size)
		old.seen = sh.builds
		return old.el
	}
	m := &dgRowMemo{sig: sig, q: q, pos: pos, size: size, seen: sh.builds}
	m.texts = texts // read by row; cloned below when the row is recorded
	r := g.row(rc, q, m)
	if !rc.memoOK {
		return r
	}
	m.texts, m.row = slices.Clone(texts), r
	if sh.rowMemo == nil {
		sh.rowMemo = map[string]*dgRowMemo{}
	}
	sh.rowMemo[k] = m
	m.el = Component(func(ctx *Context) Element {
		m.inst = ctx.inst
		return r
	}).Key(k)
	return m.el
}

func (g *DataGridElement[T]) row(rc *dgRowCtx[T], q int, m *dgRowMemo) *DivElement {
	st, sh, rows, cols, xs, selected := rc.st, rc.sh, rc.rows, rc.cols, rc.xs, rc.selected
	cursorPos, cursorCol, onHeader, hasFocus := rc.cursorPos, rc.cursorCol, rc.onHeader, rc.hasFocus
	ed := rc.ed
	// The handlers act on the row where it is now (m.q), through the closures of the
	// grid's last build (sh.live), since the row may be kept for later builds.
	live := &sh.live
	rt := rc.ctx.rt
	i := sh.order[q]
	k := sh.keys[i]
	v := rows[i]
	on := selected[k]
	r := Div().asPart("DataGrid.Row").Key(k).styleOf(&st.Row)
	if g.striped && q%2 == 1 {
		r.styleOf(&st.RowAlternate)
	}
	if on {
		r.styleOf(&st.RowSelected)
		if rc.selected[rc.keyBelow(q)] {
			// The separator between two selected rows is not drawn, so that the selection
			// reads as one block: the row's fill covers it (see joinBelow). Its width goes
			// to the padding, so the cells keep their height.
			if bw := r.style.borderEdges().Bottom; bw > 0 {
				pad := r.style.Padding
				pad.Bottom += bw
				r.styleOf(&Style{Clear: ClearBorder}).styleOf(&Style{Padding: pad})
			}
		}
	} else {
		r.hoverOf(&st.RowHover)
	}
	if !rc.onHeader && q == rc.cursorPos {
		r.ZIndex(dgZFocus) // the list draws the cursor's row over the others (see joinBelow)
	}
	// A row reports its place in the grid, for the hit test, and its place in the set for
	// assistive technology: among the shown rows, or among its siblings in a tree. In a
	// tree it also reports its level, and whether it is open when it has children.
	// The closures of a tree row capture only what they use, not rc: a captured parameter
	// is moved to the heap for every row, flat rows included.
	tree := rc.sh.tree
	if tree == nil {
		r.listItem(q+1, sh.n)
	} else {
		r.listItem(tree.pos[i], tree.size[i]).AccessLevel(tree.level[i])
		if len(tree.kids[i]) > 0 {
			key := k
			r.AccessExpanded(sh.openSet[k])
			r.access().setExpanded = func(v bool) { live.setOpen(key, v) }
		}
	}
	r.access().gridRow = q + 1
	if g.mode != SelectionNone {
		r.AccessSelected(on)
	}
	r.OnMouseDown(func(e *MouseEvent) {
		switch e.Button {
		case input.ButtonLeft:
			sh.pressed, sh.ctl = true, false
			live.press(m.q, live.colAt(m.q, e.X), e.Mods)
		case input.ButtonRight:
			// A right-press focuses the grid before its context menu opens, as File
			// Explorer does, so that the menu's actions (such as actions.GridCopy) run on
			// the grid, and focus returns to it when the menu closes.
			if !sh.focus.Focused() && !rt.focusInside(sh.focus.id) {
				sh.focus.FocusNow()
			}
		}
	})
	r.OnClick(func() {
		if sh.pressed {
			sh.pressed = false
			return
		}
		// A click from an assistive technology selects the row, in the column it had.
		live.click(m.q, live.cursorCol, 0)
	})
	r.OnDoubleClick(func() {
		if !sh.ctl {
			live.activate(m.q)
		}
	})
	if g.mode != SelectionNone {
		r.access().selectWith = func(ctrl bool) {
			mods := input.Modifiers(0)
			if ctrl {
				mods = input.ModCtrl
			}
			live.click(m.q, live.cursorCol, mods)
		}
	}
	if rc.rowHeadW > 0 {
		r.Child(g.rowHeader(rc, q, m, k, on))
	}
	var names []string
	joins := false // the cell being built joins the selected cell below it (see joinBelow)
	// skip is the width of the columns not built since the last cell, which a spacer
	// hidden from assistive technology takes.
	var skip float32
	spacer := func() {
		if skip > 0 {
			r.Child(Div().AccessHidden().styleOf(&Style{Basis: Px(skip), NoShrink: true}))
			skip = 0
		}
	}
	for c, cl := range cols {
		s := ""
		if cl.Text != nil {
			s = m.texts[c]
			names = append(names, s)
		}
		if rc.built != nil && !rc.built[c] {
			skip += xs[c+1] - xs[c]
			continue
		}
		spacer()
		cell := Div().asPart("DataGrid.Cell").styleOf(&st.Cell).
			styleOf(&Style{Basis: Px(xs[c+1] - xs[c]), Grow: cl.Grow, Justify: alignOf(cl.Align)})
		cs := cl.CellStyle
		if rc.t.HighContrast {
			cs = plainStyle(cs)
		}
		cell.styleOf(&cs)
		cell.Role(access.RoleDataItem).Label(s).AccessCell(q+1, c+1)
		m.cells = append(m.cells, cell)
		if cl.Pin != PinNone {
			cell.styleOf(&st.CellPinned)
			pinCell(cell, st, cl.Pin, c == rc.startN-1 || c == len(cols)-rc.endN)
		}
		if g.cellOn && g.cellMode != SelectionNone {
			// When the grid selects cells, the cells are the items of the selection, and
			// a selected cell is drawn in CellSelected: its fill is the selection.
			ci, picked := c, rc.cells.has(k, cl.key())
			if picked {
				cell.styleOf(&st.CellSelected)
				if below := rc.keyBelow(q); below != "" && rc.cells.has(below, cl.key()) {
					joins = true
				}
			}
			cell.AccessSelected(picked)
			cell.access().selectWith = func(ctrl bool) {
				mods := input.Modifiers(0)
				if ctrl {
					mods = input.ModCtrl
				}
				live.click(m.q, ci, mods)
			}
			cell.OnClick(func() {
				if sh.pressed {
					sh.pressed = false
					return
				}
				// A click from an assistive technology selects the cell.
				live.click(m.q, ci, 0)
			})
		}
		a := cell.access()
		if !onHeader && !rc.onRowHead && q == cursorPos && c == cursorCol {
			if cl.Pin == PinNone {
				cell.ZIndex(dgZFocus)
			}
			a.highlighted = true
			if hasFocus {
				cell.styleOf(&st.CellFocused)
			}
		}
		editing := ed.key == k && ed.col == c
		if tree != nil && c == 0 {
			// The first column of a tree starts with an indent for each level above the row,
			// and the chevron of a row with children, or the space of it for a leaf, so that
			// the text of every row lines up. The chevron is a click target of its own: a
			// press on it does not select the row.
			for range tree.level[i] - 1 {
				cell.Child(Div().AccessHidden().asPart("DataGrid.TreeIndent").styleOf(&st.TreeIndent))
			}
			chev := Div().AccessHidden().asPart("DataGrid.TreeChevron").styleOf(&st.TreeChevron)
			if len(tree.kids[i]) > 0 {
				glyph := icons.ChevronRight
				open := sh.openSet[k]
				if open {
					glyph = icons.ChevronDown
				}
				key := k
				chev.Child(Icon(glyph).Size(12)).Cursor(CursorPointer).
					OnMouseDown(func(e *MouseEvent) {
						if e.Button == input.ButtonLeft {
							sh.pressed, sh.ctl = true, true
							e.StopPropagation()
						}
					}).
					OnClick(func() { live.setOpen(key, !sh.openSet[key]) })
			}
			cell.Child(chev)
		}
		if cl.Interactive || cl.Edit != nil {
			// The controls of the cell are not Tab stops: the grid is the one stop. The
			// cell's key context keeps the grid's copy keys from its controls and its
			// editor (see dataGridBindings).
			cell.tabSkip = true
			cell.KeyContext("DataGridCell")
			ci := c
			cell.OnMouseDown(func(e *MouseEvent) {
				// A press on a control, or in the editor, which has focus, moves the
				// cursor to the cell and leaves the row alone.
				if e.Button == input.ButtonLeft && !sh.focus.Focused() {
					sh.pressed, sh.ctl = true, true
					live.ops.pick(m.q, ci)
					e.StopPropagation()
				}
			})
		}
		if cl.Edit != nil {
			// The Value pattern of an editable cell sets its value through Edit, on the UI
			// thread. A cell that cannot be edited has no Value pattern (see access.Node).
			a.value = s
			a.setValue = func(x string) {
				if err := cl.Edit(v, x); err != nil {
					Announce(err.Error(), access.Assertive)
				}
			}
			ci := c
			cell.OnDoubleClick(func() {
				if sh.focus.Focused() {
					live.ops.edit(m.q, ci)
				}
			})
		}
		switch {
		case editing && cl.Editor != nil:
			cell.styleOf(&st.CellEditing)
			cell.Child(cl.Editor(v, sh.draft.Get(), func(x string) { sh.draft.Set(x) },
				func() { live.ops.save(sh.draft.Peek()) }, live.ops.cancel))
		case editing:
			cell.styleOf(&st.CellEditing)
			cell.Child(g.editor(rc, cl))
		case cl.Cell != nil && cl.Interactive:
			cell.Child(cl.Cell(v))
		case cl.Cell != nil:
			// A cell aligned to the start fills its width, so the content can take a
			// percentage of it (as a progress bar does). Other cells keep the content's
			// width, so their Justify places it. The wrapper fills the height of the row,
			// so content sized with Full does too.
			wrap := Div().AccessHidden().AlignSelf(Stretch)
			if alignOf(cl.Align) == Start {
				wrap.Grow(1)
			}
			cell.Child(wrap.Child(cl.Cell(v)))
		default:
			cell.Child(Text(s).Truncate())
		}
		if editing && ed.err != "" {
			// The message of a failed save hangs under the editing cell as an overlay: it is
			// not clipped by the cell or the grid, and it flips above at the bottom edge.
			cell.Child(Div().asPart("DataGrid.EditorMessage").styleOf(&st.EditorMessage).
				Overlay(BelowStart).OverlayGap(4).
				Child(Div().Row().Align(Center).Gap(6).
					Child(Icon(icons.Alert).Size(14).Color(st.EditorMessage.TextColor)).
					Child(Text(ed.err).NoWrap())))
		}
		if joins {
			joinBelow(cell, st, true)
			joins = false
		}
		r.Child(cell)
	}
	spacer()
	if len(names) > 0 {
		r.Label(strings.Join(names, ", "))
	}
	return r
}

// keyBelow returns the key of the row shown under the row at q, or "" for the last.
func (rc *dgRowCtx[T]) keyBelow(q int) string {
	if q+1 >= len(rc.sh.order) {
		return ""
	}
	return rc.sh.keys[rc.sh.order[q+1]]
}

// joinBelow makes d, a selected cell or row header whose neighbor below is selected
// too, reach over the separator under its row (the Row part's bottom border), so that
// the selection's fill is one block with no line between its rows. Its padding grows by
// as much, so its content stays where it was. A cell at the edge of the selection does
// not join, so the edge stays crisp.
//
// A cell's fill is translucent, and over the separator it would show the line through
// it, so the cell is first filled with the grid's background and its row's fill, as a
// pinned cell is (fillFromParent): over the separator it then looks as it does over the
// rest of its row, striped or hovered. That fill is opaque, so the focused cell and its
// row are drawn over their neighbors (see dgZFocus), and its ring is not covered. A row
// header's fill is opaque already.
func joinBelow(d *DivElement, st *DataGridStyle, opaque bool) {
	bw := st.Row.borderEdges().Bottom
	if bw <= 0 {
		return
	}
	pad := d.style.Padding
	pad.Bottom += bw
	d.styleOf(&Style{Margin: Edges{Bottom: -bw}, Padding: pad})
	if opaque && !d.fillFromParent {
		d.fillFromParent, d.fillBase = true, st.Grid.Background
	}
}

// The paint order of the parts of a grid, among their siblings: the focused cell over
// the other cells of its row and the cursor's row over the other rows (dgZFocus), so
// that the focused cell's ring is not covered by an opaque neighbor (see joinBelow); the
// pinned cells and the row headers over both (dgZPinned), as the cells scroll under them.
const (
	dgZFocus  = 1
	dgZPinned = 2
)

// rowHeader returns the header of the row with the key k at position q (see
// RowHeaders): its number, a header item of the row for assistive technology. It is
// drawn in RowHeaderSelected while the row is selected whole, which on says for a grid
// that selects rows. A press selects the row, and a drag down the headers the rows it
// crosses (see DataGridElement.RowHeaders).
func (g *DataGridElement[T]) rowHeader(rc *dgRowCtx[T], q int, m *dgRowMemo, k string, on bool) *DivElement {
	st, sh, live := rc.st, rc.sh, &rc.sh.live
	num := strconv.Itoa(q + 1)
	whole := on
	if g.cellOn {
		whole = rc.cells.wholeRow(k)
	}
	rh := Div().asPart("DataGrid.RowHeader").styleOf(&st.RowHeader).styleOf(&Style{Basis: Px(rc.rowHeadW)}).ZIndex(dgZPinned)
	if whole {
		rh.styleOf(&st.RowHeaderSelected)
		below := rc.keyBelow(q)
		if g.cellOn && below != "" && rc.cells.wholeRow(below) || !g.cellOn && rc.selected[below] {
			joinBelow(rh, st, false)
		}
	}
	rh.Role(access.RoleHeaderItem).Label(num).AccessCell(q+1, 0)
	if rc.onRowHead && !rc.onHeader && q == rc.cursorPos {
		rh.access().highlighted = true
		if rc.hasFocus {
			rh.styleOf(&st.CellFocused)
		}
	}
	if g.cellOn && g.cellMode == SelectionMultiple {
		// The header is an item of the selection, selected while its row is selected whole.
		rh.AccessSelected(whole)
		rh.access().selectWith = func(ctrl bool) {
			mods := input.Modifiers(0)
			if ctrl {
				mods = input.ModCtrl
			}
			live.rowHead(m.q, mods)
		}
	}
	rh.Child(Text(num).NoWrap()).
		OnMouseDown(func(e *MouseEvent) {
			if e.Button == input.ButtonLeft {
				sh.pressed, sh.ctl = true, false
				live.rowHead(m.q, e.Mods)
				e.StopPropagation()
			}
		}).
		OnClick(func() {
			if sh.pressed {
				sh.pressed = false
				return
			}
			live.rowHead(m.q, 0) // a click from an assistive technology
		})
	return rh
}

// editor returns the content of an editing cell: its text editor. The editor takes
// the grid's keys as described in DataGrid, and its frame is invalid while the last
// save failed. The message of that save is drawn by row, under the cell.
func (g *DataGridElement[T]) editor(rc *dgRowCtx[T], cl DataColumn[T]) Element {
	var fe FieldStyle
	resolveField(rc.ctx, fieldLayout{size: sizeSmall}, &rc.st.Editor, &fe)
	if rc.ed.err != "" {
		// The editor has focus while it is invalid, so the invalid frame also takes the
		// place of the focus frame: the Accent border would hide the danger color.
		fe.Frame.mergeFrom(&fe.FrameInvalid, false)
		copyLarge(&fe.FrameFocusWithin, &fe.FrameInvalid)
	}
	ti := TextInput(rc.sh.draft).FocusHandle(rc.sh.editFocus).Label(cl.Header).stylesOf(&fe).
		OnSubmit(func(text string) { rc.ops.save(text) })
	ti.widget = "DataGrid.Editor"
	ti.onCancel = rc.ops.cancel
	ti.onBlur = func() { rc.ops.blur(rc.ed.seq) }
	// The message is the editor's description: the cell's own name hides the children
	// of the cell, so a message in the cell is not exposed by itself.
	ti.description = rc.ed.err
	return ti.styleOf(&Style{Grow: 1})
}

// pinCell makes d, a cell of a pinned column in the body or the header, opaque in
// the state of its row and drawn over the cells that scroll under it. The cell at
// the inner edge of its region, edge, has the PinnedEdge border on that side.
func pinCell(d *DivElement, st *DataGridStyle, pin ColumnPin, edge bool) {
	d.ZIndex(dgZPinned)
	d.fillFromParent, d.fillBase = true, st.Grid.Background
	if edge {
		side := Edges{Right: 1}
		if pin == PinEnd {
			side = Edges{Left: 1}
		}
		d.styleOf(&Style{BorderEdges: side}).styleOf(&st.PinnedEdge)
	}
}

// dgVisibleWidth is the width of the rows that the grid's body n shows: its width
// less its border and padding.
func dgVisibleWidth(n *node) float32 {
	be := n.style.borderEdges()
	return n.lay.W - be.Left - be.Right - n.style.Padding.Left - n.style.Padding.Right
}

// seq returns the integers from lo up to hi, hi not included.
func seq(lo, hi int) []int {
	out := make([]int, 0, max(hi-lo, 0))
	for i := lo; i < hi; i++ {
		out = append(out, i)
	}
	return out
}

// dgCellNode returns the built cell at the 1-based row and column of the grid whose
// body is body, or nil when it is not built.
func dgCellNode(body *node, row, col int) *node {
	if body == nil {
		return nil
	}
	var found *node
	var walk func(n *node)
	walk = func(n *node) {
		for _, c := range n.children {
			if found != nil || c.flags&flagNoDisplay != 0 {
				return
			}
			if d := c.div; d != nil && d.part == "DataGrid.Cell" && d.acc != nil && d.acc.gridRow == row && d.acc.gridCol == col {
				found = c
				return
			}
			walk(c)
		}
	}
	walk(body)
	return found
}

// dgControls appends to out the focusable nodes under n, in tree order: the
// controls of a cell. An editor is one control.
func dgControls(n *node, out []*node) []*node {
	for _, c := range n.children {
		if c.flags&flagNoDisplay != 0 {
			continue
		}
		if c.kind == kindEditor || (c.div != nil && c.div.focusable) {
			out = append(out, c)
			continue
		}
		out = dgControls(c, out)
	}
	return out
}

// headerRow returns the header: a row of header cells, one for each column. The
// list keeps it at the top of its viewport, and it scrolls with the columns.
func (g *DataGridElement[T]) headerRow(st *DataGridStyle, sh *dgShared[T], rows []T, cols []DataColumn[T], cw []float32,
	width float32, sorted SortState, sortCol, col int, onHeader, hasFocus, highContrast bool,
	sortBy func(i int), setWidth func(i int, v float32) bool, fit func(i int) float32, ho dgHeadOps[T],
	strs *Strings) *DivElement {
	changed, widthOf := ho.changed, ho.widthOf
	h := Div().asPart("DataGrid.Header").styleOf(&st.Header).Width(Px(width))
	if ho.rowHeadW > 0 {
		// The corner, over the row headers, selects every cell (see RowHeaders).
		cn := Div().asPart("DataGrid.Corner").styleOf(&st.Corner).styleOf(&Style{Basis: Px(ho.rowHeadW)}).ZIndex(dgZPinned)
		cn.Role(access.RoleButton).Label(strs.SelectAll).hoverOf(&st.HeaderCellHover)
		if ho.onCorner {
			cn.access().highlighted = true
			if hasFocus {
				cn.styleOf(&st.CellFocused)
			}
		}
		h.Child(cn.OnMouseDown(func(e *MouseEvent) {
			if e.Button == input.ButtonLeft {
				sh.headPress = true
				ho.corner()
			}
		}).OnClick(func() {
			if sh.headPress {
				sh.headPress = false
				return
			}
			ho.corner()
		}))
	}
	for c, cl := range cols {
		sortable := cl.Compare != nil
		isSorted := c == sortCol
		name := cl.Header
		status := ""
		if isSorted {
			status = strs.SortedAscending
			if sorted.Desc {
				status = strs.SortedDescending
			}
			name += ", " + status
		}
		hc := Div().asPart("DataGrid.HeaderCell").styleOf(&st.HeaderCell).
			styleOf(&Style{Basis: Px(cw[c]), Grow: cl.Grow, Justify: alignOf(cl.Align)})
		hc.Role(access.RoleHeaderItem).Label(name).AccessCell(0, c+1)
		if cl.Group != "" {
			hc.Description(cl.Group)
		}
		if cl.Pin != PinNone {
			pinCell(hc, st, cl.Pin, c == ho.startN-1 || c == len(cols)-ho.endN)
		}
		a := hc.access()
		a.status = status
		if onHeader && c == col && !ho.onCorner {
			a.highlighted = true
			if hasFocus {
				hc.styleOf(&st.CellFocused)
			}
		}
		if sortable || ho.selects {
			hc.hoverOf(&st.HeaderCellHover).Cursor(CursorPointer)
		}
		if isSorted {
			hc.styleOf(&st.HeaderCellSorted)
		}
		whole := ho.cellSel && ho.whole(c)
		if whole {
			hc.styleOf(&st.HeaderCellSelected)
		}
		if cl.RenderHeader != nil {
			hc.Child(Div().AccessHidden().Child(cl.RenderHeader(cl.Header, PartState{Selected: isSorted, HighContrast: highContrast})))
		} else {
			hc.Child(Text(cl.Header).Truncate())
		}
		ci := c
		switch {
		case isSorted && !ho.selects:
			glyph := icons.ChevronUp
			if sorted.Desc {
				glyph = icons.ChevronDown
			}
			ind := Div().asPart("DataGrid.SortIndicator").styleOf(&st.SortIndicator)
			if whole {
				// On a selected column's header the arrow takes the header's text color.
				ind.styleOf(&Style{TextColor: st.HeaderCellSelected.TextColor})
			}
			hc.Child(ind.Child(Icon(glyph).Size(12)))
		case sortable && ho.selects:
			// When a press on the header selects its column, the sort is the arrow's: a
			// sortable column shows it while not sorted too, in the header's color.
			glyph := icons.ChevronsUpDown
			ind := Div().asPart("DataGrid.SortIndicator").AccessHidden().Cursor(CursorPointer)
			if isSorted {
				glyph = icons.ChevronUp
				if sorted.Desc {
					glyph = icons.ChevronDown
				}
				ind.styleOf(&st.SortIndicator)
			}
			if whole {
				ind.styleOf(&Style{TextColor: st.HeaderCellSelected.TextColor})
			}
			if alignOf(cl.Align) == Start {
				// The arrow is at the end of the header cell, where it is found at once.
				hc.Child(Div().Grow(1))
			}
			hc.Child(ind.Child(Icon(glyph).Size(12)).
				OnMouseDown(func(e *MouseEvent) {
					if e.Button == input.ButtonLeft {
						sh.headPress = false
						ho.stopDrag()
						e.StopPropagation()
					}
				}).
				OnClick(func() { sortBy(ci) }))
		}
		if ho.selects {
			hc.AccessSelected(whole)
			hc.access().selectWith = func(ctrl bool) {
				mods := input.Modifiers(0)
				if ctrl {
					mods = input.ModCtrl
				}
				ho.pickCol(ci, mods)
			}
			move := ho.movable(cl)
			hc.OnMouseDown(func(e *MouseEvent) {
				if e.Button != input.ButtonLeft {
					return
				}
				// A drag from a movable column that is selected whole moves it; any
				// other drag selects the columns it crosses.
				drags := move && e.Mods == 0 && ho.whole(ci)
				sh.headPress = true
				ho.pickCol(ci, e.Mods)
				if !drags {
					ho.stopDrag()
				}
			})
		}
		if sortable {
			// With HeaderSelects a mouse press has selected the column, and the click is
			// assistive technology's Invoke, which sorts.
			hc.OnClick(func() {
				if sh.headPress {
					sh.headPress = false
					return
				}
				if !sh.edgePress {
					sortBy(ci)
				}
			})
		}
		if g.headerMenu != nil {
			key := cl.key()
			hc.ContextMenu(func() []MenuItem { return g.headerMenu(key) })
		}
		// Every header cell is a drop target for a moved column, a fixed one too, so a
		// column can be put beside it; the half of the cell under the pointer picks the
		// side. The line that shows the side changes no weight.
		key := cl.key()
		if m := ho.mark; m.key == key {
			line := Div().asPart("DataGrid.DropIndicator").Absolute().Top(0).Bottom(0).styleOf(&st.DropIndicator)
			if m.after {
				line.Right(-1)
			} else {
				line.Left(-1)
			}
			hc.Child(line)
		}
		hc.OnDrop(Drop(func(p dgColDrag, e *DropEvent) {
			ho.setMark(dgColMark{})
			ho.drop(p.key, key, e.LocalX > e.Width/2)
		}).
			Accept(func(p dgColDrag) bool { return p.grid == any(sh) && ho.accepts(p.key, key) }).
			Move(func(p dgColDrag, e *DropEvent) { ho.setMark(dgColMark{key: key, after: e.LocalX > e.Width/2}) }).
			Leave(func() { ho.setMark(dgColMark{}) }))
		if ho.movable(cl) {
			// The dragged header is dimmed, except under high contrast, where it keeps
			// its colors.
			source := Style{Opacity: 0.5}
			if highContrast {
				source = Style{}
			}
			label := cl.Header
			hc.draggingOf(&source).OnDrag(Drag(dgColDrag{grid: sh, key: key}).Preview(func() Element {
				return Div().Padding(6).styleOf(&st.HeaderCell).Background(st.Header.Background).Border(1, st.Header.BorderColor).
					Child(Text(label))
			}).End(func(DragEnd) {
				ho.setMark(dgColMark{})
				sh.headPress = false // a drop runs no click to clear it
			}))
		}
		if cl.Resizable {
			ci, key, cl := c, cl.key(), cl
			hc.Child(Div().asPart("DataGrid.ResizeHandle").styleOf(&st.ResizeHandle).Absolute().Top(0).Bottom(0).Right(0).
				CapturePointer().
				OnMouseDown(func(e *MouseEvent) {
					if e.Button == input.ButtonLeft {
						sh.dragging, sh.dragKey, sh.dragX, sh.dragW = true, key, e.X, cw[ci]
						sh.dragFrom = widthOf(cl)
						sh.edgePress = true
						ho.stopDrag()
						e.StopPropagation()
					}
				}).
				OnMouseMove(func(e *MouseEvent) {
					if sh.dragging && sh.dragKey == key {
						setWidth(ci, sh.dragW+e.X-sh.dragX)
					}
				}).
				// The end of the drag is the change the layout of the columns reports, not
				// each move of it.
				OnMouseUp(func(*MouseEvent) {
					if sh.dragging && sh.dragKey == key && widthOf(cl) != sh.dragFrom {
						changed()
					}
					sh.dragging = false
					ho.post(func() { sh.edgePress = false })
				}).
				OnDoubleClick(func() {
					if setWidth(ci, fit(ci)) {
						changed()
					}
				}))
		}
		h.Child(hc)
	}
	if ho.runs == nil {
		return h.Absolute().Top(0).Left(0)
	}
	// The row of the groups is above the header cells. Its pieces are as wide as their
	// columns: each takes their widths and their shares of the spare width. It is
	// for sighted users: each header cell is described by its group.
	gr := Div().styleOf(&st.Header).AccessHidden()
	if ho.rowHeadW > 0 {
		gr.Child(Div().styleOf(&st.Corner).styleOf(&Style{Basis: Px(ho.rowHeadW)}).ZIndex(dgZPinned))
	}
	for _, run := range ho.runs {
		var basis, grow float32
		for c := run.from; c < run.to; c++ {
			basis += cw[c]
			grow += cols[c].Grow
		}
		piece := Div().styleOf(&Style{Basis: Px(basis), Grow: grow, NoShrink: true, AlignSelf: Stretch})
		if run.group != "" {
			piece.asPart("DataGrid.HeaderGroup").styleOf(&st.HeaderGroup).Child(Text(run.group).Truncate())
		}
		if run.pin != PinNone {
			pinCell(piece, st, run.pin, run.to-1 == ho.startN-1 || run.from == len(cols)-ho.endN)
		}
		gr.Child(piece)
	}
	return Div().Column().Absolute().Top(0).Left(0).Width(Px(width)).Child(gr).Child(h)
}
