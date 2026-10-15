//go:build windows

package windows

import (
	"syscall"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/win32"
)

// The grid and table patterns of a data grid, and of its cells. A data grid
// (access.RoleDataGrid) has the Grid and Table patterns; each cell
// (access.RoleDataItem with a grid position) has the GridItem and TableItem
// patterns, and a header item (access.RoleHeaderItem) has Invoke, which sorts.
// The IDs are from UIAutomationClient.h and the interfaces from
// UIAutomationCore.idl (SDK 10.0.26100.0). The vtables list the methods in the
// order of the IDL, after IUnknown.
//
// A grid's row count and column count are of the whole data, built or not. GetItem
// answers only for the cells that are built: a client that asks for a row that is
// off screen gets nothing, and scrolls the grid to it with its Scroll pattern.

// Control types, patterns and properties of a data grid.
const (
	ctDataGrid   = 50028 // UIA_DataGridControlTypeId
	ctDataItem   = 50029 // UIA_DataItemControlTypeId
	ctHeaderItem = 50035 // UIA_HeaderItemControlTypeId

	patternGrid      = 10006 // UIA_GridPatternId
	patternGridItem  = 10007 // UIA_GridItemPatternId
	patternTable     = 10012 // UIA_TablePatternId
	patternTableItem = 10013 // UIA_TableItemPatternId

	propIsGridItemPatternAvail  = 30029 // UIA_IsGridItemPatternAvailablePropertyId
	propIsGridPatternAvail      = 30030 // UIA_IsGridPatternAvailablePropertyId
	propIsTablePatternAvail     = 30038 // UIA_IsTablePatternAvailablePropertyId
	propIsTableItemPatternAvail = 30039 // UIA_IsTableItemPatternAvailablePropertyId
	propItemStatus              = 30026 // UIA_ItemStatusPropertyId

	rowMajor = 0 // RowOrColumnMajor_RowMajor
)

// The vtables of the grid interfaces. They are filled by initGridVtbls.
var (
	gridVtbl      [6]uintptr // IGridProvider: GetItem, RowCount, ColumnCount
	tableVtbl     [6]uintptr // ITableProvider: GetRowHeaders, GetColumnHeaders, RowOrColumnMajor
	gridItemVtbl  [8]uintptr // IGridItemProvider: Row, Column, RowSpan, ColumnSpan, ContainingGrid
	tableItemVtbl [5]uintptr // ITableItemProvider: GetRowHeaderItems, GetColumnHeaderItems
)

var (
	ifaceGrid      = iface{iidGridProvider, gridVtbl[:]}
	ifaceTable     = iface{iidTableProvider, tableVtbl[:]}
	ifaceGridItem  = iface{iidGridItemProvider, gridItemVtbl[:]}
	ifaceTableItem = iface{iidTableItemProvider, tableItemVtbl[:]}
)

// The IIDs of the grid interfaces, from the uuid attributes in UIAutomationCore.idl.
// TestGridIIDsMatchTheSDK checks them against the strings.
var (
	iidGridProvider      = guid{0xb17d6187, 0x0907, 0x464b, [8]byte{0xa1, 0x68, 0x0e, 0xf1, 0x7a, 0x15, 0x72, 0xb1}}
	iidTableProvider     = guid{0x9c860395, 0x97b3, 0x490a, [8]byte{0xb5, 0x2a, 0x85, 0x8c, 0xc2, 0x2a, 0xf1, 0x66}}
	iidGridItemProvider  = guid{0xd02541f1, 0xfb81, 0x4d64, [8]byte{0xae, 0x32, 0xf5, 0x20, 0xf8, 0xa6, 0xdb, 0xd1}}
	iidTableItemProvider = guid{0xb9734fa6, 0x771f, 0x4d78, [8]byte{0x9c, 0x90, 0x25, 0x17, 0x99, 0x93, 0x49, 0xcd}}
)

// initGridVtbls fills the vtables of the grid interfaces. It runs once, from initVtbls.
func initGridVtbls(qi, ar, rl uintptr) {
	gridVtbl = [...]uintptr{qi, ar, rl,
		syscall.NewCallback(uiaGridGetItem),
		syscall.NewCallback(uiaGridRowCount),
		syscall.NewCallback(uiaGridColumnCount),
	}
	tableVtbl = [...]uintptr{qi, ar, rl,
		syscall.NewCallback(uiaTableRowHeaders),
		syscall.NewCallback(uiaTableColumnHeaders),
		syscall.NewCallback(uiaTableRowOrColumnMajor),
	}
	gridItemVtbl = [...]uintptr{qi, ar, rl,
		syscall.NewCallback(uiaGridItemRow),
		syscall.NewCallback(uiaGridItemColumn),
		syscall.NewCallback(uiaGridItemRowSpan),
		syscall.NewCallback(uiaGridItemColumnSpan),
		syscall.NewCallback(uiaGridItemContainingGrid),
	}
	tableItemVtbl = [...]uintptr{qi, ar, rl,
		syscall.NewCallback(uiaTableItemRowHeaderItems),
		syscall.NewCallback(uiaTableItemColumnHeaderItems),
	}
}

// gridWalk calls fn for each node under the grid, in tree order, until fn returns
// false. It does not go into a grid inside the grid, whose cells are its own. The
// cells and header items may be children of the grid or of rows, or deeper.
func gridWalk(t *access.Tree, grid *access.Node, fn func(*access.Node) bool) bool {
	for _, id := range grid.Children {
		n := t.Node(id)
		if n == nil {
			continue
		}
		if !fn(n) {
			return false
		}
		if n.Role != access.RoleDataGrid && n.Role != access.RoleCalendar && !gridWalk(t, n, fn) {
			return false
		}
	}
	return true
}

// gridCell returns the built cell at row and column (both 0-based) of the grid, or
// nil: the data item at that 1-based grid position.
func gridCell(t *access.Tree, grid *access.Node, row, col int) *access.Node {
	var cell *access.Node
	gridWalk(t, grid, func(n *access.Node) bool {
		if n.Role == access.RoleDataItem && n.GridRow == row+1 && n.GridColumn == col+1 {
			cell = n
		}
		return cell == nil
	})
	return cell
}

// gridHeaders returns the header items of the grid, in column order.
func gridHeaders(t *access.Tree, grid *access.Node) []*access.Node {
	var hs []*access.Node
	gridWalk(t, grid, func(n *access.Node) bool {
		if n.Role == access.RoleHeaderItem {
			hs = append(hs, n)
		}
		return true
	})
	return hs
}

// columnHeader returns the header item of the 1-based column col of the grid, or nil:
// the one with that GridColumn, or else the col-th header item when it has none.
func columnHeader(t *access.Tree, grid *access.Node, col int) *access.Node {
	hs := gridHeaders(t, grid)
	for _, h := range hs {
		if h.GridColumn == col {
			return h
		}
	}
	if col >= 1 && col <= len(hs) && hs[col-1].GridColumn == 0 {
		return hs[col-1]
	}
	return nil
}

// gridOf returns the data grid that holds node n, which is a cell or a row, or nil.
func gridOf(t *access.Tree, n *access.Node) *access.Node {
	for p := t.Node(n.Parent); p != nil; p = t.Node(p.Parent) {
		if p.Role == access.RoleDataGrid || p.Role == access.RoleCalendar {
			return p
		}
	}
	return nil
}

// providerArray returns a SAFEARRAY of the providers of the nodes ns, or 0 when
// the array cannot be made. The array holds its own references.
func providerArray(c *core, ns []*access.Node) uintptr {
	psa := win32.SafeArrayCreateVector(vtUnknown, 0, uint32(len(ns)))
	if psa == 0 {
		return 0
	}
	for i, n := range ns {
		p := c.provide(n.ID, iidRawElementProviderSimple)
		if p == 0 {
			continue
		}
		win32.SafeArrayPutUnknown(psa, int32(i), p)
		release(p)
	}
	return psa
}

// The Grid pattern. GetItem answers with the built cell at a position, or null.
func uiaGridGetItem(this, row, col, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	n, t, ok := c.current()
	if !ok {
		return hrElementNotAvailable
	}
	cell := gridCell(t, n, int(int32(row)), int(int32(col)))
	if cell == nil {
		putPtr(out, 0)
		return hrOK
	}
	p := c.provide(cell.ID, iidRawElementProviderSimple)
	if p == 0 {
		return hrElementNotAvailable // the cell left the tree since t was published
	}
	putPtr(out, p)
	return hrOK
}

func uiaGridRowCount(this, out uintptr) uintptr {
	return windowOut(this, out, func(n *access.Node) int32 { return int32(n.GridRows) })
}

func uiaGridColumnCount(this, out uintptr) uintptr {
	return windowOut(this, out, func(n *access.Node) int32 { return int32(n.GridColumns) })
}

// The Table pattern. The row headers are empty: a grid's rows have no header
// items. The column headers are the header items of the grid.
func uiaTableRowHeaders(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	putPtr(out, providerArray(c, nil))
	return hrOK
}

func uiaTableColumnHeaders(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	n, t, ok := c.current()
	if !ok {
		return hrElementNotAvailable
	}
	putPtr(out, providerArray(c, gridHeaders(t, n)))
	return hrOK
}

func uiaTableRowOrColumnMajor(this, out uintptr) uintptr {
	return windowOut(this, out, func(*access.Node) int32 { return rowMajor })
}

// The GridItem pattern of a cell: its row and column are 0-based, and it spans one
// of each.
func uiaGridItemRow(this, out uintptr) uintptr {
	return windowOut(this, out, func(n *access.Node) int32 { return int32(n.GridRow - 1) })
}

func uiaGridItemColumn(this, out uintptr) uintptr {
	return windowOut(this, out, func(n *access.Node) int32 { return int32(n.GridColumn - 1) })
}

func uiaGridItemRowSpan(this, out uintptr) uintptr {
	return windowOut(this, out, func(*access.Node) int32 { return 1 })
}

func uiaGridItemColumnSpan(this, out uintptr) uintptr {
	return windowOut(this, out, func(*access.Node) int32 { return 1 })
}

func uiaGridItemContainingGrid(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	n, t, ok := c.current()
	if !ok {
		return hrElementNotAvailable
	}
	g := gridOf(t, n)
	if g == nil {
		putPtr(out, 0)
		return hrOK
	}
	p := c.provide(g.ID, iidRawElementProviderSimple)
	if p == 0 {
		return hrElementNotAvailable // the grid left the tree since t was published
	}
	putPtr(out, p)
	return hrOK
}

// The TableItem pattern of a cell: no row headers, and the header of its column.
func uiaTableItemRowHeaderItems(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	putPtr(out, providerArray(c, nil))
	return hrOK
}

func uiaTableItemColumnHeaderItems(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	n, t, ok := c.current()
	if !ok {
		return hrElementNotAvailable
	}
	g := gridOf(t, n)
	if g == nil {
		putPtr(out, providerArray(c, nil))
		return hrOK
	}
	var mine []*access.Node
	if h := columnHeader(t, g, n.GridColumn); h != nil {
		mine = []*access.Node{h}
	}
	putPtr(out, providerArray(c, mine))
	return hrOK
}
