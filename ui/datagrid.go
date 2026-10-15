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
	// The content is hidden from assistive technology, so give Text as well.
	Cell func(row T) Element
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
	Key  string
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
	DataGridComfortable DataGridDensity = iota
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
type DataGridStyle struct {
	Grid             Style
	GridFocusVisible Style
	Header           Style
	HeaderCell       Style
	HeaderCellHover  Style
	HeaderCellSorted Style
	SortIndicator    Style
	ResizeHandle     Style
	Row              Style
	RowHover         Style
	RowSelected      Style
	RowAlternate     Style
	Cell             Style
	CellFocused      Style
	Empty            Style
	CellEditing      Style
	Editor           FieldStyle
	EditorMessage    Style
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
		Header: Style{Direction: Row, Align: Center, MinHeight: Px(h), Background: t.Sidebar,
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
		RowAlternate: Style{Background: t.Sidebar},
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
	}
	if t.HighContrast {
		st.RowSelected = Style{Background: t.Accent, TextColor: t.AccentText}
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
//   - Enter activates the focused row, and calls OnActivate with it.
//   - On a header cell, Enter and Space sort by its column (see Sort). On a
//     resizable header cell, Ctrl+Left and Ctrl+Right narrow and widen the column by
//     8 logical pixels. Alt is not used by the grid, so that the host keeps its
//     menu and navigation keys.
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
// A click selects a row (Ctrl toggles it, Shift extends the selection), and a
// double-click activates it, except on an editable cell, where a double-click
// starts an edit. A click on a control in a cell (see DataColumn.Interactive)
// focuses and operates the control; it moves the cursor to the cell but does not
// select or activate the row. A click on a header sorts by its column: ascending,
// then descending, then not sorted. Dragging the edge of a resizable header
// resizes its column, and a double-click on the edge fits the column to the rows
// in view. A column header stays at the top of the rows as they scroll, and the
// header scrolls with the rows when the columns are wider than the grid.
//
// Sorting orders the rows without changing their keys, and the focus and the
// selection follow their rows by key, so a sort, a scroll, new rows or a saved
// edit keep them. Sorting is stable, and it runs on the UI thread: a sort of 100,000
// rows takes about 8 ms (see docs/accessibility.md, Phase I). Keys must be unique.
//
// An editing row is built only while it is in view, as the other rows are, because
// the rows of a grid are one window of the list. When the row scrolls out of view its
// editor is not laid out, but the edit, its text and its focus are kept, and the
// editor is laid out again when the row comes back (see docs/accessibility.md).
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
}

// The sizing methods give the grid's outer box a size, over its Style. See Sizing
// in docs/styling.md.
func (g *DataGridElement[T]) Width(l Length) *DataGridElement[T]    { g.size.setWidth(l); return g }
func (g *DataGridElement[T]) Height(l Length) *DataGridElement[T]   { g.size.setHeight(l); return g }
func (g *DataGridElement[T]) MinWidth(l Length) *DataGridElement[T] { g.size.setMinWidth(l); return g }
func (g *DataGridElement[T]) MinHeight(l Length) *DataGridElement[T] {
	g.size.setMinHeight(l)
	return g
}
func (g *DataGridElement[T]) MaxWidth(l Length) *DataGridElement[T] { g.size.setMaxWidth(l); return g }
func (g *DataGridElement[T]) MaxHeight(l Length) *DataGridElement[T] {
	g.size.setMaxHeight(l)
	return g
}
func (g *DataGridElement[T]) Grow(f float32) *DataGridElement[T] { g.size.setGrow(f); return g }
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
	g.mode, g.sel = mode, keys
	return g
}

// Sort keeps the sort in s: the grid sets it when the user sorts, and the rows are
// ordered by it. A nil s keeps the sort inside the grid. A column sorts only when
// its Compare is set.
func (g *DataGridElement[T]) Sort(s *Signal[SortState]) *DataGridElement[T] { g.sort = s; return g }

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

// dgCursor is the focused cell: the key of its row (empty on the header), the
// position of that row when the key is gone, and the column.
type dgCursor struct {
	key    string
	pos    int
	col    int
	header bool
}

// dgEdit is the edit of a cell: the key of its row ("" when no cell is being
// edited), its column, and the error of the last save, which keeps the editor open.
type dgEdit struct {
	key string
	col int
	err string
	seq int // the number of the edit, so that a blur from an older editor is ignored
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
	ed                   dgEdit
	click                func(q, cl int, mods input.Modifiers)
	colAt                func(q int, x float32) int
	activate             func(q int)
	ops                  dgOps
}

// dgShared is the state of a grid that lives with it: the order of its rows, the
// drag of a column edge, and what the paint of its list reports back.
type dgShared[T any] struct {
	// The keys are made once per slice of rows, and the order once per sort.
	keyed, ordered bool
	ident          unsafe.Pointer
	n              int
	keys           []string       // key of each row of the data
	by             SortState      // the sort the order is made for
	order          []int          // the row of the data at each displayed position
	pos            map[string]int // displayed position by key
	dup            string         // a key that is used twice, for the warning
	data           []T            // the rows of the last build, for an edit to find its row

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

	// A column the cursor moved to is scrolled into view at the next paint, from
	// revealX0 to revealX1 in the content.
	reveal             bool
	revealX0, revealX1 float32
	viewH              float32 // the height of the viewport at the last paint
	warned             bool
}

// update makes the keys of the rows when the rows have changed, and the order of
// them for the sort by when either has changed. A key used twice is noted in dup.
func (sh *dgShared[T]) update(rows []T, key func(T) string, cols []DataColumn[T], by SortState) {
	n := len(rows)
	ident := unsafe.Pointer(unsafe.SliceData(rows))
	if !sh.keyed || sh.ident != ident || sh.n != n {
		sh.keyed, sh.ident, sh.n = true, ident, n
		sh.keys = make([]string, n)
		for i := range rows {
			sh.keys[i] = key(rows[i])
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
	sh.pos = make(map[string]int, n)
	sh.dup = ""
	for p, i := range order {
		k := sh.keys[i]
		if _, ok := sh.pos[k]; ok && sh.dup == "" {
			sh.dup = k
		}
		sh.pos[k] = p
	}
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
// the rows built by the last frame. A Cell slot is not measured.
func autoFitWidth[T any](rt *Runtime, sh *dgShared[T], rows []T, cols []DataColumn[T], i int) float32 {
	cl := cols[i]
	w := textWidth(rt, cl.Header) + 40 // the padding, the sort arrow and the edge
	if cl.Text != nil {
		for q := sh.builtLo; q < sh.builtHi && q < sh.n; q++ {
			w = max(w, textWidth(rt, cl.Text(rows[sh.order[q]]))+16)
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
	widths := Local(ctx, "widths", map[string]float32(nil))
	handle := Local(ctx, "handle", NewListHandle()).Peek()
	sel := g.sel
	if sel == nil {
		sel = Local(ctx, "selection", []string(nil))
	}
	by := g.sort
	if by == nil {
		by = Local(ctx, "sort", SortState{})
	}

	rows, cols, n := g.rows, g.cols, len(g.rows)
	nc := len(cols)
	sortState := by.Get()
	sh.update(rows, g.key, cols, sortState)
	sh.data = rows
	if sh.focus == nil {
		sh.focus, sh.editFocus, sh.draft = NewFocusHandle(), NewFocusHandle(), NewSignal("")
	}
	if g.focusHandle != nil {
		sh.focus = g.focusHandle
	}
	if sh.dup != "" && !sh.warned && rt.warn != nil {
		sh.warned = true
		rt.warn(fmt.Sprintf("duplicate key %q among the rows of a DataGrid; keys must be unique. "+
			"Derive keys from a unique field such as an ID.", sh.dup))
	}
	order := sh.order
	keyAt := func(q int) string { return sh.keys[order[q]] }
	selected := sh.selection(sel.Get())
	rowH := dataGridRowHeight(g.density) * rt.textFactor

	// The cursor follows its row by key, so that a sort or new rows keep it on
	// the same cell.
	cursor := cur.Get()
	p := min(max(cursor.pos, 0), max(n-1, 0))
	if q, ok := sh.pos[cursor.key]; ok && cursor.key != "" {
		p = q
	}
	col := min(max(cursor.col, 0), max(nc-1, 0))
	onHeader := cursor.header
	hasFocus := focused.Get()
	ed := edit.Get()
	anchorPos := func() int {
		if k := anchor.Peek(); k != "" {
			if q, ok := sh.pos[k]; ok {
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
	sortCol := -1
	for i, cl := range cols {
		if sortState.Key != "" && cl.key() == sortState.Key && cl.Compare != nil {
			sortCol = i
		}
	}

	// setWidth sets column i to v pixels wide at the Text size in use. The width is
	// kept at the size of 100 percent, so that it scales with the text.
	setWidth := func(i int, v float32) {
		cl := cols[i]
		u := max(v/rt.textFactor, cl.minWidth())
		if cl.MaxWidth > 0 {
			u = min(u, cl.MaxWidth)
		}
		if u == cl.width(ws) {
			return
		}
		next := make(map[string]float32, nc)
		for k, x := range ws {
			next[k] = x
		}
		next[cl.key()] = u
		widths.Set(next)
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
			g.onActivate(rows[order[q]])
		}
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
	// column cl, and brings both into view.
	place := func(q, cl int, header bool) {
		k := ""
		if !header && q >= 0 && q < n {
			k = keyAt(q)
		}
		cur.Set(dgCursor{key: k, pos: q, col: cl, header: header})
		if nc > 0 {
			sh.reveal = true
			sh.revealX0, sh.revealX1 = xs[cl], xs[cl+1]
		}
		if !header && n > 0 {
			handle.ScrollToItem(q)
		} else {
			handle.request()
		}
	}
	// moveRow moves to row q and column cl, and selects as the selection mode and
	// the modifiers say: a plain move selects the row, Ctrl moves the focus only,
	// and Shift extends the selection from the anchor.
	moveRow := func(q, cl int, ctrl, shift bool) {
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
	// click selects the row at q for a press with the modifiers mods.
	click := func(q, cl int, mods input.Modifiers) {
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
		edit.Set(dgEdit{key: keyAt(q), col: cl, seq: sh.seq})
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
			text = c.Value(rows[order[q]])
		} else if c.Text != nil {
			text = c.Text(rows[order[q]])
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
		q, ok := sh.pos[e.key]
		if !ok {
			edit.Set(dgEdit{})
			return false
		}
		if err := cols[e.col].Edit(sh.data[sh.order[q]], text); err != nil {
			edit.Set(dgEdit{key: e.key, col: e.col, err: err.Error(), seq: e.seq})
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
		if cols[edit.Peek().col].Editor != nil {
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
	// tabEdit saves the edit and starts one in the next editable cell of the row
	// in direction dir. At the last editable cell the edit ends, and focus stays
	// in the grid on that cell.
	tabEdit := func(dir int) {
		e := edit.Peek()
		if !commitEdit(sh.draft.Peek()) {
			return
		}
		if q, ok := sh.pos[e.key]; ok {
			for c := e.col + dir; c >= 0 && c < nc; c += dir {
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
		return min(max(c.pos, 0), max(n-1, 0)), min(max(c.col, 0), max(nc-1, 0)), c.header
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
		step := max(int((sh.viewH-rowH)/rowH), 1)
		switch e.Key {
		case input.KeyLeft, input.KeyRight:
			dir := 1
			if e.Key == input.KeyLeft {
				dir = -1
			}
			if onHeader && ctrl {
				if nc > 0 && cols[col].Resizable {
					setWidth(col, cw[col]+8*float32(dir))
				}
				return true
			}
			if nc > 0 {
				place(p, min(max(col+dir, 0), nc-1), onHeader)
			}
		case input.KeyHome, input.KeyEnd:
			if nc == 0 {
				return true
			}
			last := e.Key == input.KeyEnd
			switch {
			case ctrl && n > 0 && last:
				moveRow(n-1, nc-1, ctrl, false)
			case ctrl && n > 0:
				moveRow(0, 0, ctrl, false)
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
	// column is found in the laid-out cells and not from the widths. A point right of
	// the last cell is in the last column.
	colAt := func(q int, x float32) int {
		body := rt.byID[sh.focus.id]
		for c := range nc - 1 {
			cell := dgCellNode(body, q+1, c+1)
			if cell == nil {
				continue
			}
			if r := rt.nodeRect(cell); x < r.X+r.W {
				return c
			}
		}
		return max(nc-1, 0)
	}
	rc := dgRowCtx[T]{t: t, ctx: ctx, st: st, sh: sh, rows: rows, cols: cols, xs: xs, selected: selected, rowH: rowH,
		cursorPos: p, cursorCol: col, onHeader: onHeader, hasFocus: hasFocus, ed: ed,
		click: click, colAt: colAt, activate: activate, ops: ops}
	list := UniformList(n, func(q int) Element {
		if sh.builtHi == 0 {
			sh.builtLo = q
		}
		sh.builtHi = q + 1
		return g.row(rc, q)
	}).Key(g.keyStr).Handle(handle)
	// The scrollbar is drawn over the grid's right edge, so the rows end before it:
	// the body keeps dataGridGutter of padding there, and the rows are as wide as
	// the body less its border and that gutter, or as wide as the columns.
	width := max(total, sh.viewW-2-dataGridGutter)
	list.rowWidth = width
	list.decorate = func(el *DivElement) {
		focus := Style{Shadows: []Shadow{{Spread: 2, Color: t.FocusRing}}}.Merge(st.GridFocusVisible)
		el.asPart("DataGrid.Grid").Role(access.RoleDataGrid).ScrollX().
			Style(st.Grid).Style(Style{Padding: Edges{Top: rowH, Right: dataGridGutter}}).
			FocusHandle(sh.focus).FocusVisible(focus).
			OnKeyDown(func(e *KeyEvent) {
				if keys(e) {
					e.StopPropagation()
				}
			}).
			OnFocus(func() { focused.Set(true) }).
			OnBlur(func() { focused.Set(false) }).
			OnTextInput(typeText)
		if g.mode != SelectionNone {
			el.SelectionContainer()
		}
		if g.name != "" {
			el.Label(g.name)
		}
		a := el.access()
		a.gridRows, a.gridCols = n, nc
		a.multiSelect = g.mode == SelectionMultiple
	}
	list.before = func(nd *node) {
		sh.viewH = nd.lay.H
		// A custom editor takes focus when it is painted, on its first control; until
		// then the focus is not inside the cell, so the blur check below waits. When
		// focus has left the editing cell, the edit is saved, as a text field's is.
		if e := edit.Peek(); e.key != "" && cols[e.col].Editor != nil {
			if q, ok := sh.pos[e.key]; ok {
				if cell := dgCellNode(nd, q+1, e.col+1); cell != nil {
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
		if !sh.reveal {
			return
		}
		sh.reveal = false
		x0, x1 := sh.revealX0, sh.revealX1
		switch {
		case x0 < nd.st.targetX:
			nd.st.targetX = x0
		case x1 > nd.st.targetX+nd.lay.W:
			nd.st.targetX = x1 - nd.lay.W
		}
		rt.animating = true
	}
	list.header = g.headerRow(st, sh, rows, cols, cw, width, sortState, sortCol, col, onHeader, hasFocus,
		t.HighContrast, sortBy, setWidth, func(i int) float32 { return autoFitWidth(rt, sh, rows, cols, i) }, strs)

	root := g.size.apply(Div().Column().Style(g.style)).Child(list)
	if n == 0 {
		msg := g.empty
		if msg == nil {
			msg = Text(strs.NoRows)
		}
		root.Child(Div().asPart("DataGrid.Empty").Absolute().Top(rowH).Left(0).Right(0).Style(st.Empty).Child(msg))
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
	r.listItem(q+1, sh.n)
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
	for c, cl := range cols {
		s := ""
		if cl.Text != nil {
			s = cl.Text(v)
			names = append(names, s)
		}
		cell := Div().asPart("DataGrid.Cell").Style(st.Cell).
			Style(Style{Basis: Px(xs[c+1] - xs[c]), Grow: cl.Grow, Justify: alignOf(cl.Align)})
		cell.Role(access.RoleDataItem).Label(s)
		a := cell.access()
		a.gridRow, a.gridCol = q+1, c+1
		if !onHeader && q == cursorPos && c == cursorCol {
			a.highlighted = true
			if hasFocus {
				cell.Style(st.CellFocused)
			}
		}
		editing := ed.key == k && ed.col == c
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
			// width, so their Justify places it.
			wrap := Div().AccessHidden()
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
	sortBy func(i int), setWidth func(i int, v float32), fit func(i int) float32, strs *Strings) *DivElement {
	h := Div().asPart("DataGrid.Header").Style(st.Header).Absolute().Top(0).Left(0).Width(Px(width))
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
			hc.OnClick(func() { sortBy(ci) })
		}
		if cl.Resizable {
			ci, key := c, cl.key()
			hc.Child(Div().asPart("DataGrid.ResizeHandle").Style(st.ResizeHandle).Absolute().Top(0).Bottom(0).Right(0).
				CapturePointer().
				OnMouseDown(func(e *MouseEvent) {
					if e.Button == input.ButtonLeft {
						sh.dragging, sh.dragKey, sh.dragX, sh.dragW = true, key, e.X, cw[ci]
						e.StopPropagation()
					}
				}).
				OnMouseMove(func(e *MouseEvent) {
					if sh.dragging && sh.dragKey == key {
						setWidth(ci, sh.dragW+e.X-sh.dragX)
					}
				}).
				OnMouseUp(func(*MouseEvent) { sh.dragging = false }).
				OnDoubleClick(func() { setWidth(ci, fit(ci)) }))
		}
		h.Child(hc)
	}
	return h
}
