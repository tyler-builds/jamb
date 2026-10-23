package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
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
}

// merge returns s with the set fields of o applied on top, part by part.
func (s DataGridStyle) merge(o DataGridStyle) DataGridStyle {
	return DataGridStyle{
		Grid:             s.Grid.Merge(o.Grid),
		GridFocusVisible: s.GridFocusVisible.Merge(o.GridFocusVisible),
		Header:           s.Header.Merge(o.Header),
		HeaderCell:       s.HeaderCell.Merge(o.HeaderCell),
		HeaderCellHover:  s.HeaderCellHover.Merge(o.HeaderCellHover),
		HeaderCellSorted: s.HeaderCellSorted.Merge(o.HeaderCellSorted),
		SortIndicator:    s.SortIndicator.Merge(o.SortIndicator),
		ResizeHandle:     s.ResizeHandle.Merge(o.ResizeHandle),
		Row:              s.Row.Merge(o.Row),
		RowHover:         s.RowHover.Merge(o.RowHover),
		RowSelected:      s.RowSelected.Merge(o.RowSelected),
		RowAlternate:     s.RowAlternate.Merge(o.RowAlternate),
		Cell:             s.Cell.Merge(o.Cell),
		CellFocused:      s.CellFocused.Merge(o.CellFocused),
		Empty:            s.Empty.Merge(o.Empty),
		CellEditing:      s.CellEditing.Merge(o.CellEditing),
		Editor:           s.Editor.merge(o.Editor),
		EditorMessage:    s.EditorMessage.Merge(o.EditorMessage),
		DropIndicator:    s.DropIndicator.Merge(o.DropIndicator),
		CellPinned:       s.CellPinned.Merge(o.CellPinned),
		PinnedEdge:       s.PinnedEdge.Merge(o.PinnedEdge),
		HeaderGroup:      s.HeaderGroup.Merge(o.HeaderGroup),
		CellSelected:     s.CellSelected.Merge(o.CellSelected),
		TreeIndent:       s.TreeIndent.Merge(o.TreeIndent),
		TreeChevron:      s.TreeChevron.Merge(o.TreeChevron),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s DataGridStyle) plain() DataGridStyle {
	return DataGridStyle{
		Grid:             plainStyle(s.Grid),
		GridFocusVisible: plainStyle(s.GridFocusVisible),
		Header:           plainStyle(s.Header),
		HeaderCell:       plainStyle(s.HeaderCell),
		HeaderCellHover:  plainStyle(s.HeaderCellHover),
		HeaderCellSorted: plainStyle(s.HeaderCellSorted),
		SortIndicator:    plainStyle(s.SortIndicator),
		ResizeHandle:     plainStyle(s.ResizeHandle),
		Row:              plainStyle(s.Row),
		RowHover:         plainStyle(s.RowHover),
		RowSelected:      plainStyle(s.RowSelected),
		RowAlternate:     plainStyle(s.RowAlternate),
		Cell:             plainStyle(s.Cell),
		CellFocused:      plainStyle(s.CellFocused),
		Empty:            plainStyle(s.Empty),
		CellEditing:      plainStyle(s.CellEditing),
		Editor:           s.Editor.plain(),
		EditorMessage:    plainStyle(s.EditorMessage),
		DropIndicator:    plainStyle(s.DropIndicator),
		CellPinned:       plainStyle(s.CellPinned),
		PinnedEdge:       plainStyle(s.PinnedEdge),
		HeaderGroup:      plainStyle(s.HeaderGroup),
		CellSelected:     plainStyle(s.CellSelected),
		TreeIndent:       plainStyle(s.TreeIndent),
		TreeChevron:      plainStyle(s.TreeChevron),
	}
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

// dataGridDefaults returns the built-in look of a DataGrid, from the colors of t.
// Under high contrast the selected row is the Highlight color with HighlightText
// on it, and the focus ring is kept.
func dataGridDefaults(t *Theme, d DataGridDensity, textFactor float32) DataGridStyle {
	h := dataGridRowHeight(d) * textFactor
	line := Edges{Bottom: 1}
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	st := DataGridStyle{
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
		RowSelected:  Style{Background: t.AccentSoft},
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
		CellSelected: Style{Background: t.AccentSoft},
		// A tree's indent and chevron are 16 logical pixels wide at the Text size 100 percent.
		TreeIndent:  Style{Width: Px(16 * textFactor), NoShrink: true},
		TreeChevron: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(16 * textFactor), NoShrink: true, TextColor: t.TextMuted},
	}
	if t.HighContrast {
		st.RowSelected = Style{Background: t.Accent, TextColor: t.AccentText}
		st.CellSelected = Style{Background: t.Accent, TextColor: t.AccentText}
		st.EditorMessage.BorderColor, st.EditorMessage.TextColor = t.Text, t.Text
	}
	return st
}

// resolveDataGrid returns the look of a DataGrid: the defaults, merged with the
// theme's DataGrid and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveDataGrid(ctx *Context, d DataGridDensity, textFactor float32, inst DataGridStyle) DataGridStyle {
	t := ctx.Theme()
	return layered(ctx, dataGridDefaults(t, d, textFactor), DataGridStyle.merge, DataGridStyle.plain,
		themeLayer(t.DataGrid), instanceLayer(inst))
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
//   - When the grid selects cells (see CellSelection), the moves select cells in the
//     same way: a plain move selects the cell, Ctrl moves the focus only, and Shift
//     with an arrow, Home, End, PageUp or PageDown extends a rectangle from the
//     anchor cell. Space toggles the focused cell, Shift+Space selects the cells of
//     its row, Ctrl+Space those of its column, and Ctrl+A every cell.
//   - Enter activates the focused row, and calls OnActivate with it.
//   - On a header cell, Enter and Space sort by its column (see Sort). On a
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
// when the grid selects cells (Shift selects the rectangle from the anchor), and a
// double-click activates it, except on an editable cell, where a double-click
// starts an edit. A click on a control in a cell (see DataColumn.Interactive)
// focuses and operates the control; it moves the cursor to the cell but does not
// select or activate the row. A click on a header sorts by its column: ascending,
// then descending, then not sorted. Dragging the edge of a resizable header
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
	rows        []T
	key         func(T) string
	cols        []DataColumn[T]
	name        string
	keyStr      string
	style       Style
	density     DataGridDensity
	striped     bool
	mode        SelectionMode
	sel         *Signal[[]string]
	sort        *Signal[SortState]
	onActivate  func(row T)
	loading     bool
	empty       Element
	styles      DataGridStyle
	focusHandle *FocusHandle // see FocusHandle
	size        sizing       // see the sizing methods
	order       *Signal[[]string]
	widths      *Signal[map[string]float32]
	onColumns   func(ColumnState)
	gridHandle  *DataGridHandle
	rowMenu     func(row T, column string) []MenuItem
	headerMenu  func(column string) []MenuItem
	hasMore     bool
	loadMore    func()
	loadingMore bool
	movable     bool
	cellOn      bool // the user selects cells (see CellSelection)
	cellMode    SelectionMode
	cells       *Signal[[]CellKey]
	selBoth     bool // both Selection and CellSelection were called, for the warning
	children    func(row T) []T
	expanded    *Signal[[]string] // the open rows of a tree, when the app keeps them (see Expanded)
	onExpand    func(key string)
	onCollapse  func(key string)
}

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
func (g *DataGridElement[T]) Style(s Style) *DataGridElement[T] { g.style = g.style.Merge(s); return g }

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
		g.cellOn, g.cellMode, g.cells = false, SelectionNone, nil
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
// the two calls is the one that applies (the development warnings say so). Ctrl+A
// selects every cell, so with many rows cells holds a key for each cell of each row.
func (g *DataGridElement[T]) CellSelection(mode SelectionMode, cells *Signal[[]CellKey]) *DataGridElement[T] {
	if g.mode != SelectionNone || g.sel != nil {
		g.selBoth = true
		g.mode, g.sel = SelectionNone, nil
	}
	g.cellOn, g.cellMode, g.cells = true, mode, cells
	return g
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
func (g *DataGridElement[T]) Styles(st DataGridStyle) *DataGridElement[T] {
	g.styles = g.styles.merge(st)
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
}

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
	st                   DataGridStyle
	sh                   *dgShared[T]
	rows                 []T
	cols                 []DataColumn[T]
	xs                   []float32
	selected             map[string]bool
	rowH                 float32
	cursorPos, cursorCol int
	onHeader, hasFocus   bool
	startN, endN         int              // the numbers of the columns pinned to the start and the end
	built                []bool           // the columns whose cells are built, or nil for all of them
	cells                map[CellKey]bool // the selected cells, when the grid selects cells
	ed                   dgEdit
	click                func(q, cl int, mods input.Modifiers)
	colAt                func(q int, x float32) int
	activate             func(q int)
	ops                  dgOps
	store                *Signal[[]string] // the open rows of a tree, or nil (see Expanded)
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
	order          []int          // the row of the data at each displayed position
	pos            map[string]int // displayed position by key, made when needed (see posOf)
	dup            string         // a key that is used twice, for the warning
	data           []T            // the rows of the last build, for an edit to find its row

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

	// The set of the selected cells, made again only when the cells change (see
	// cellSelection), and whether the warning about Selection and CellSelection has
	// been given.
	cellOK    bool
	cellIdent unsafe.Pointer
	cellN     int
	cellSet   map[CellKey]bool
	warnedSel bool

	// The displayed positions built by the last frame: [builtLo, builtHi).
	builtLo, builtHi int
	// viewW is the width of the viewport at the last paint, and pressed is set by
	// a mouse press on a row, so that the click that follows does not select twice.
	viewW   float32
	pressed bool

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
// them for the sort by when either has changed. A key used twice is noted in dup.
func (sh *dgShared[T]) update(rows []T, key func(T) string, cols []DataColumn[T], by SortState) {
	n := len(rows)
	ident := unsafe.Pointer(unsafe.SliceData(rows))
	if !sh.keyed || sh.ident != ident || sh.n != n {
		sh.keyed, sh.ident, sh.n = true, ident, n
		sh.keys = make([]string, n)
		sh.sortedKeys = true
		for i := range rows {
			sh.keys[i] = key(rows[i])
			if i > 0 && sh.keys[i-1] >= sh.keys[i] {
				sh.sortedKeys = false
			}
		}
		sh.ordered = false
	}
	if sh.ordered && sh.by == by {
		return
	}
	sh.ordered, sh.by = true, by
	order := make([]int, n)
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
	sh.order = order
	sh.tree = nil
	sh.pos, sh.dup = nil, ""
	// Keys in strictly increasing order are unique, so the table of positions can
	// wait for its first lookup; otherwise it is made now, to find a repeated key.
	if !sh.sortedKeys {
		sh.makePos()
	}
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

// cellSelection returns the set of the cells in cells, made again only when cells
// changes.
func (sh *dgShared[T]) cellSelection(cells []CellKey) map[CellKey]bool {
	ident := unsafe.Pointer(unsafe.SliceData(cells))
	if sh.cellOK && sh.cellIdent == ident && sh.cellN == len(cells) {
		return sh.cellSet
	}
	set := make(map[CellKey]bool, len(cells))
	for _, k := range cells {
		set[k] = true
	}
	sh.cellOK, sh.cellIdent, sh.cellN, sh.cellSet = true, ident, len(cells), set
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
func autoFitWidth[T any](rt *Runtime, st DataGridStyle, sh *dgShared[T], rows []T, cols []DataColumn[T], i int) float32 {
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
	st := resolveDataGrid(ctx, g.density, rt.textFactor, g.styles)
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
	cellSel := g.cells
	if cellSel == nil {
		cellSel = Local(ctx, "cells", []CellKey(nil))
	}
	anchorCol := Local(ctx, "anchorCol", "")

	// The columns are in the order they are shown, so that a column's index is its
	// place on the screen and its column for assistive technology.
	cols := g.viewCols(colOrder.Get())
	nc := len(cols)
	colPos := make(map[string]int, nc)
	for i, cl := range cols {
		colPos[cl.key()] = i
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
	rows := g.refresh(sh, store, cols, sortState)
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
	var cellsOn map[CellKey]bool
	if g.cellOn {
		cellsOn = sh.cellSelection(cellSel.Get())
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
		if q, ok := sh.posOf(cursor.key); ok {
			p = q
		}
	}
	col := colOf(cursor.colKey, cursor.col)
	onHeader := cursor.header
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

	// The width of each column, and where each one starts.
	ws := widths.Get()
	cw := make([]float32, nc)
	xs := make([]float32, nc+1)
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
		var x float32
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
	// The cells of a cell selection (see CellSelection). cellAt names the cell at
	// (q, c); the anchor of the cells is the anchor row and anchorCol.
	cellKeyAt := func(q, c int) CellKey { return CellKey{Row: keyAt(q), Column: cols[c].key()} }
	setAnchor := func(q, c int) {
		anchor.Set(keyAt(q))
		anchorCol.Set(cols[c].key())
	}
	selectCells := func(ks []CellKey) {
		if g.cellMode != SelectionNone {
			cellSel.Set(ks)
		}
	}
	// rectCells returns the cells of the rectangle from (q0, c0) to (q1, c1), in the
	// order they are shown, row by row.
	rectCells := func(q0, c0, q1, c1 int) []CellKey {
		qa, qb, ca, cb := min(q0, q1), max(q0, q1), min(c0, c1), max(c0, c1)
		out := make([]CellKey, 0, (qb-qa+1)*(cb-ca+1))
		for q := qa; q <= qb; q++ {
			for c := ca; c <= cb; c++ {
				out = append(out, cellKeyAt(q, c))
			}
		}
		return out
	}
	// fromAnchor returns the cells from the anchor cell to (q, c).
	fromAnchor := func(q, c int) []CellKey {
		return rectCells(anchorPos(), colOf(anchorCol.Peek(), c), q, c)
	}
	toggleCell := func(k CellKey) {
		cur := cellSel.Peek()
		next := make([]CellKey, 0, len(cur)+1)
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
		cellSel.Set(next)
	}
	// moveCell moves to the cell at (q, c) and selects as moveRow does for rows: a
	// plain move selects the cell, Ctrl moves the focus only, and Shift extends the
	// rectangle from the anchor cell.
	moveCell := func(q, c int, ctrl, shift bool) {
		switch {
		case shift && g.cellMode == SelectionMultiple:
			selectCells(fromAnchor(q, c))
		case ctrl:
			// The anchor stays where it is.
		case g.cellMode != SelectionNone:
			selectCells([]CellKey{cellKeyAt(q, c)})
			setAnchor(q, c)
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
			k := cellKeyAt(q, cl)
			switch {
			case g.cellMode == SelectionMultiple && mods&input.ModShift != 0:
				selectCells(fromAnchor(q, cl))
			case g.cellMode == SelectionMultiple && mods&input.ModCtrl != 0:
				toggleCell(k)
				setAnchor(q, cl)
			case g.cellMode != SelectionNone:
				selectCells([]CellKey{k})
				setAnchor(q, cl)
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
		if !sh.focus.Focused() || onHeader || n == 0 || nc == 0 || cols[col].Edit == nil || cols[col].Editor != nil ||
			r == ' ' || !unicode.IsPrint(r) {
			return
		}
		startEdit(p, col, s)
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
				if e.Key == input.KeySpace || e.Key == input.KeyEnter {
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
				selectCells(rectCells(p, 0, p, nc-1)) // the cells of the row
			case g.cellOn && g.cellMode == SelectionMultiple && ctrl:
				selectCells(rectCells(0, col, n-1, col)) // the cells of the column
			case g.cellOn && g.cellMode == SelectionMultiple:
				toggleCell(cellKeyAt(p, col))
				setAnchor(p, col)
			case g.cellOn && g.cellMode == SelectionSingle:
				selectCells([]CellKey{cellKeyAt(p, col)})
				setAnchor(p, col)
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
				selectCells(rectCells(0, 0, n-1, nc-1))
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
			case g.cellOn && g.cellMode != SelectionNone && !sh.cellSelection(cellSel.Peek())[cellKeyAt(q, c)]:
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
	rc := dgRowCtx[T]{t: t, ctx: ctx, st: st, sh: sh, rows: rows, cols: cols, xs: xs, selected: selected, rowH: rowH,
		cursorPos: p, cursorCol: col, onHeader: onHeader, hasFocus: hasFocus, ed: ed, startN: startN, endN: endN,
		built: built, cells: cellsOn, click: click, colAt: colAt, activate: activate, ops: ops, store: store}
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
		return g.row(rc, q)
	}).Key(g.keyStr).Handle(handle)
	// A tree row's position is among its siblings, which the rows report themselves.
	list.ownPos = store != nil
	if g.loadingMore && n > 0 {
		// The row of LoadingMore follows the rows. Its ring names it, and its text is
		// for sighted users.
		list.tail = Div().Style(st.Row).Style(Style{Gap: 8, Padding: Edges{Left: 8}}).
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
			Style(st.Grid).Style(Style{Padding: Edges{Top: headerH, Right: dataGridGutter}}).
			FocusHandle(sh.focus).FocusVisible(focus).
			OnKeyDown(func(e *KeyEvent) {
				if keys(e) {
					e.StopPropagation()
				}
			}).
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
	if startN+endN > 0 {
		// The pinned cells, and the row of LoadingMore, are moved by the sideways
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
					for i, c := range hdr.children[0].children {
						if i < len(runs) {
							offset(c, runs[i].pin)
						}
					}
					hdr = hdr.children[1]
				}
				i := 0
				for _, c := range hdr.children {
					if c.div != nil && c.div.part == "DataGrid.HeaderCell" {
						offset(c, pinOf(i))
						i++
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
						if d := c.div; d != nil && d.part == "DataGrid.Cell" && d.acc != nil {
							offset(c, pinOf(d.acc.gridCol-1))
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
	}
	list.header = g.headerRow(st, sh, rows, cols, cw, width, sortState, sortCol, col, onHeader, hasFocus,
		t.HighContrast, sortBy, setWidth, func(i int) float32 { return autoFitWidth(rt, st, sh, rows, cols, i) }, ho, strs)

	// The column may shrink below its rows, so that the list windows them to the space
	// instead of building them all (see Tree).
	root := g.size.apply(Div().Column().MinHeight(Px(0)).Style(g.style)).Child(list)
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
		root.Child(Div().asPart("DataGrid.Empty").Absolute().Top(headerH).Left(0).Right(0).Style(st.Empty).Child(msg))
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
func (g *DataGridElement[T]) row(rc dgRowCtx[T], q int) Element {
	st, sh, rows, cols, xs, selected := rc.st, rc.sh, rc.rows, rc.cols, rc.xs, rc.selected
	cursorPos, cursorCol, onHeader, hasFocus := rc.cursorPos, rc.cursorCol, rc.onHeader, rc.hasFocus
	click, colAt, activate, ed, ops := rc.click, rc.colAt, rc.activate, rc.ed, rc.ops
	i := sh.order[q]
	k := sh.keys[i]
	v := rows[i]
	on := selected[k]
	r := Div().asPart("DataGrid.Row").Key(k).Style(st.Row)
	if g.striped && q%2 == 1 {
		r.Style(st.RowAlternate)
	}
	if on {
		r.Style(st.RowSelected)
	} else {
		r.Hover(st.RowHover)
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
			key, store := k, rc.store
			r.AccessExpanded(sh.openSet[k])
			r.access().setExpanded = func(v bool) { g.setOpen(store, sh, key, v) }
		}
	}
	r.access().gridRow = q + 1
	if g.mode != SelectionNone {
		r.AccessSelected(on)
	}
	r.OnMouseDown(func(e *MouseEvent) {
		if e.Button == input.ButtonLeft {
			sh.pressed, sh.ctl = true, false
			click(q, colAt(q, e.X), e.Mods)
		}
	})
	r.OnClick(func() {
		if sh.pressed {
			sh.pressed = false
			return
		}
		// A click from an assistive technology selects the row, in the column it had.
		click(q, cursorCol, 0)
	})
	r.OnDoubleClick(func() {
		if !sh.ctl {
			activate(q)
		}
	})
	if g.mode != SelectionNone {
		r.access().selectWith = func(ctrl bool) {
			mods := input.Modifiers(0)
			if ctrl {
				mods = input.ModCtrl
			}
			click(q, cursorCol, mods)
		}
	}
	var names []string
	// skip is the width of the columns not built since the last cell, which a spacer
	// hidden from assistive technology takes.
	var skip float32
	spacer := func() {
		if skip > 0 {
			r.Child(Div().AccessHidden().Style(Style{Basis: Px(skip), NoShrink: true}))
			skip = 0
		}
	}
	for c, cl := range cols {
		s := ""
		if cl.Text != nil {
			s = cl.Text(v)
			names = append(names, s)
		}
		if rc.built != nil && !rc.built[c] {
			skip += xs[c+1] - xs[c]
			continue
		}
		spacer()
		cell := Div().asPart("DataGrid.Cell").Style(st.Cell).
			Style(Style{Basis: Px(xs[c+1] - xs[c]), Grow: cl.Grow, Justify: alignOf(cl.Align)})
		cs := cl.CellStyle
		if rc.t.HighContrast {
			cs = plainStyle(cs)
		}
		cell.Style(cs)
		cell.Role(access.RoleDataItem).Label(s).AccessCell(q+1, c+1)
		if cl.Pin != PinNone {
			cell.Style(st.CellPinned)
			pinCell(cell, st, cl.Pin, c == rc.startN-1 || c == len(cols)-rc.endN)
		}
		if g.cellOn && g.cellMode != SelectionNone {
			// When the grid selects cells, the cells are the items of the selection, and
			// a selected cell is drawn in CellSelected: its fill is the selection.
			ci, picked := c, rc.cells[CellKey{Row: k, Column: cl.key()}]
			if picked {
				cell.Style(st.CellSelected)
			}
			cell.AccessSelected(picked)
			cell.access().selectWith = func(ctrl bool) {
				mods := input.Modifiers(0)
				if ctrl {
					mods = input.ModCtrl
				}
				click(q, ci, mods)
			}
			cell.OnClick(func() {
				if sh.pressed {
					sh.pressed = false
					return
				}
				// A click from an assistive technology selects the cell.
				click(q, ci, 0)
			})
		}
		a := cell.access()
		if !onHeader && q == cursorPos && c == cursorCol {
			a.highlighted = true
			if hasFocus {
				cell.Style(st.CellFocused)
			}
		}
		editing := ed.key == k && ed.col == c
		if tree != nil && c == 0 {
			// The first column of a tree starts with an indent for each level above the row,
			// and the chevron of a row with children, or the space of it for a leaf, so that
			// the text of every row lines up. The chevron is a click target of its own: a
			// press on it does not select the row.
			for range tree.level[i] - 1 {
				cell.Child(Div().AccessHidden().asPart("DataGrid.TreeIndent").Style(st.TreeIndent))
			}
			chev := Div().AccessHidden().asPart("DataGrid.TreeChevron").Style(st.TreeChevron)
			if len(tree.kids[i]) > 0 {
				glyph := icons.ChevronRight
				open := sh.openSet[k]
				if open {
					glyph = icons.ChevronDown
				}
				key, store := k, rc.store
				chev.Child(Icon(glyph).Size(12)).Cursor(CursorPointer).
					OnMouseDown(func(e *MouseEvent) {
						if e.Button == input.ButtonLeft {
							sh.pressed, sh.ctl = true, true
							e.StopPropagation()
						}
					}).
					OnClick(func() { g.setOpen(store, sh, key, !open) })
			}
			cell.Child(chev)
		}
		if cl.Interactive || cl.Edit != nil {
			// The controls of the cell are not Tab stops: the grid is the one stop.
			cell.tabSkip = true
			ci := c
			cell.OnMouseDown(func(e *MouseEvent) {
				// A press on a control, or in the editor, which has focus, moves the
				// cursor to the cell and leaves the row alone.
				if e.Button == input.ButtonLeft && !sh.focus.Focused() {
					sh.pressed, sh.ctl = true, true
					ops.pick(q, ci)
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
					ops.edit(q, ci)
				}
			})
		}
		switch {
		case editing && cl.Editor != nil:
			cell.Style(st.CellEditing)
			cell.Child(cl.Editor(v, sh.draft.Get(), func(x string) { sh.draft.Set(x) },
				func() { ops.save(sh.draft.Peek()) }, ops.cancel))
		case editing:
			cell.Style(st.CellEditing)
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
			cell.Child(Div().asPart("DataGrid.EditorMessage").Style(st.EditorMessage).
				Overlay(BelowStart).OverlayGap(4).
				Child(Div().Row().Align(Center).Gap(6).
					Child(Icon(icons.Alert).Size(14).Color(st.EditorMessage.TextColor)).
					Child(Text(ed.err).NoWrap())))
		}
		r.Child(cell)
	}
	spacer()
	if len(names) > 0 {
		r.Label(strings.Join(names, ", "))
	}
	return r
}

// editor returns the content of an editing cell: its text editor. The editor takes
// the grid's keys as described in DataGrid, and its frame is invalid while the last
// save failed. The message of that save is drawn by row, under the cell.
func (g *DataGridElement[T]) editor(rc dgRowCtx[T], cl DataColumn[T]) Element {
	fe := resolveField(rc.ctx, fieldLayout{size: sizeSmall}, rc.st.Editor)
	if rc.ed.err != "" {
		// The editor has focus while it is invalid, so the invalid frame also takes the
		// place of the focus frame: the Accent border would hide the danger color.
		fe.Frame = fe.Frame.Merge(fe.FrameInvalid)
		fe.FrameFocusWithin = fe.FrameInvalid
	}
	ti := TextInput(rc.sh.draft).FocusHandle(rc.sh.editFocus).Label(cl.Header).Styles(fe).
		OnSubmit(func(text string) { rc.ops.save(text) })
	ti.widget = "DataGrid.Editor"
	ti.onCancel = rc.ops.cancel
	ti.onBlur = func() { rc.ops.blur(rc.ed.seq) }
	// The message is the editor's description: the cell's own name hides the children
	// of the cell, so a message in the cell is not exposed by itself.
	ti.description = rc.ed.err
	return ti.Style(Style{Grow: 1})
}

// pinCell makes d, a cell of a pinned column in the body or the header, opaque in
// the state of its row and drawn over the cells that scroll under it. The cell at
// the inner edge of its region, edge, has the PinnedEdge border on that side.
func pinCell(d *DivElement, st DataGridStyle, pin ColumnPin, edge bool) {
	d.ZIndex(1)
	d.fillFromParent, d.fillBase = true, st.Grid.Background
	if edge {
		side := Edges{Right: 1}
		if pin == PinEnd {
			side = Edges{Left: 1}
		}
		d.Style(Style{BorderEdges: side}).Style(st.PinnedEdge)
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
func (g *DataGridElement[T]) headerRow(st DataGridStyle, sh *dgShared[T], rows []T, cols []DataColumn[T], cw []float32,
	width float32, sorted SortState, sortCol, col int, onHeader, hasFocus, highContrast bool,
	sortBy func(i int), setWidth func(i int, v float32) bool, fit func(i int) float32, ho dgHeadOps[T],
	strs *Strings) *DivElement {
	changed, widthOf := ho.changed, ho.widthOf
	h := Div().asPart("DataGrid.Header").Style(st.Header).Width(Px(width))
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
		hc := Div().asPart("DataGrid.HeaderCell").Style(st.HeaderCell).
			Style(Style{Basis: Px(cw[c]), Grow: cl.Grow, Justify: alignOf(cl.Align)})
		hc.Role(access.RoleHeaderItem).Label(name)
		if cl.Group != "" {
			hc.Description(cl.Group)
		}
		if cl.Pin != PinNone {
			pinCell(hc, st, cl.Pin, c == ho.startN-1 || c == len(cols)-ho.endN)
		}
		a := hc.access()
		a.status = status
		if onHeader && c == col {
			a.highlighted = true
			if hasFocus {
				hc.Style(st.CellFocused)
			}
		}
		if sortable {
			hc.Hover(st.HeaderCellHover).Cursor(CursorPointer)
		}
		if isSorted {
			hc.Style(st.HeaderCellSorted)
		}
		if cl.RenderHeader != nil {
			hc.Child(Div().AccessHidden().Child(cl.RenderHeader(cl.Header, PartState{Selected: isSorted, HighContrast: highContrast})))
		} else {
			hc.Child(Text(cl.Header).Truncate())
		}
		if isSorted {
			glyph := icons.ChevronUp
			if sorted.Desc {
				glyph = icons.ChevronDown
			}
			hc.Child(Div().asPart("DataGrid.SortIndicator").Style(st.SortIndicator).Child(Icon(glyph).Size(12)))
		}
		if sortable {
			ci := c
			hc.OnClick(func() {
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
			line := Div().asPart("DataGrid.DropIndicator").Absolute().Top(0).Bottom(0).Style(st.DropIndicator)
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
			hc.Dragging(source).OnDrag(Drag(dgColDrag{grid: sh, key: key}).Preview(func() Element {
				return Div().Padding(6).Style(st.HeaderCell).Background(st.Header.Background).Border(1, st.Header.BorderColor).
					Child(Text(label))
			}).End(func(DragEnd) { ho.setMark(dgColMark{}) }))
		}
		if cl.Resizable {
			ci, key, cl := c, cl.key(), cl
			hc.Child(Div().asPart("DataGrid.ResizeHandle").Style(st.ResizeHandle).Absolute().Top(0).Bottom(0).Right(0).
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
	gr := Div().Style(st.Header).AccessHidden()
	for _, run := range ho.runs {
		var basis, grow float32
		for c := run.from; c < run.to; c++ {
			basis += cw[c]
			grow += cols[c].Grow
		}
		piece := Div().Style(Style{Basis: Px(basis), Grow: grow, NoShrink: true, AlignSelf: Stretch})
		if run.group != "" {
			piece.asPart("DataGrid.HeaderGroup").Style(st.HeaderGroup).Child(Text(run.group).Truncate())
		}
		if run.pin != PinNone {
			pinCell(piece, st, run.pin, run.to-1 == ho.startN-1 || run.from == len(cols)-ho.endN)
		}
		gr.Child(piece)
	}
	return Div().Column().Absolute().Top(0).Left(0).Width(Px(width)).Child(gr).Child(h)
}
