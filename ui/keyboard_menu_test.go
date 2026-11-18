package ui_test

import (
	"strconv"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// openMenu returns the bounds and the name of the open menu, failing when none is open.
func openMenu(t *testing.T, u *uitest.UI) (ui.Rect, string) {
	t.Helper()
	var found *access.Node
	u.Tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleMenu {
			found = n
		}
		return true
	})
	if found == nil {
		t.Fatal("no menu is open")
	}
	b := found.Bounds
	return ui.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}, found.Name
}

// checkKeyboardMenu presses Shift+F10 on what has focus and checks that the menu
// opens just below the item that has accessibility focus, at its left edge or
// inside it, and takes its name.
func checkKeyboardMenu(t *testing.T, u *uitest.UI, item string) {
	t.Helper()
	f := u.Focused()
	if n := f.Node(); n.Name != item {
		t.Fatalf("focus on %s %q, want %q", n.Role, n.Name, item)
	}
	row := f.Bounds()
	u.Press("shift+f10")
	u.Settle() // the menu slides in
	m, name := openMenu(t, u)
	if name != item {
		t.Errorf("the menu is named %q, want the item's name %q", name, item)
	}
	if bottom := row.Y + row.H; m.Y < bottom-1 || m.Y > bottom+8 {
		t.Errorf("the menu's top is at y=%.1f, want just below the item (%+v, bottom %.1f)", m.Y, row, bottom)
	}
	if m.X < row.X-1 || m.X > row.X+row.W {
		t.Errorf("the menu's left is at x=%.1f, want within the item %+v", m.X, row)
	}
}

// A menu opened from the keyboard on a tree opens below the selected row and is
// named after it, as Windows places a menu opened with Shift+F10 at the focused
// item, rather than at the corner of the tree.
func TestTreeKeyboardMenuOpensAtTheRow(t *testing.T) {
	var nodes []ui.TreeNode
	for i := range 12 {
		nodes = append(nodes, ui.TreeNode{Key: "t" + strconv.Itoa(i), Label: "Table " + strconv.Itoa(i)})
	}
	sel := ui.NewSignal("t3")
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Width(ui.Px(280)).Grow(1).Child(
			ui.Tree(nodes, sel).Key("tree").Label("Object Explorer").Grow(1).
				ContextMenu(func(key string) []ui.MenuItem {
					return []ui.MenuItem{{Label: "Refresh " + key, OnClick: func() {}}}
				}))
	}, uitest.Size(600, 600))
	u.Get("tree").Focus()
	checkKeyboardMenu(t, u, "Table 3")
	if !u.HasRole(access.RoleMenuItem, "Refresh t3") {
		t.Error("the menu is not the selected row's")
	}
}

// The same holds for a list box: the menu opens below the focused item.
func TestListBoxKeyboardMenuOpensAtTheItem(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	sel := ui.NewSignal([]string{"c"})
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Width(ui.Px(280)).Grow(1).Child(
			ui.ListBox(items, func(s string) string { return s }, sel,
				func(s string, _ ui.PartState) ui.Element { return ui.Text("item " + s) }).
				Key("lb").Label("Items").Grow(1).
				ContextMenu(func(s string) []ui.MenuItem { return []ui.MenuItem{{Label: "Open " + s, OnClick: func() {}}} }))
	}, uitest.Size(600, 600))
	u.Get("lb").Focus()
	u.Press("down")
	checkKeyboardMenu(t, u, "item d")
}

// And for a data grid: the menu opens below the cell the cursor is on.
func TestDataGridKeyboardMenuOpensAtTheCell(t *testing.T) {
	type rowT struct{ id, name string }
	var rows []rowT
	for i := range 12 {
		rows = append(rows, rowT{strconv.Itoa(i), "Customer " + strconv.Itoa(i)})
	}
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Grow(1).Child(
			ui.DataGrid(rows, func(r rowT) string { return r.id },
				ui.DataColumn[rowT]{Header: "ID", Text: func(r rowT) string { return r.id }, Width: 60},
				ui.DataColumn[rowT]{Header: "Name", Text: func(r rowT) string { return r.name }, Grow: 1}).
				Key("grid").Label("Customers").Grow(1).
				ContextMenu(func(r rowT, col string) []ui.MenuItem {
					return []ui.MenuItem{{Label: "Copy " + col + " of " + r.id, OnClick: func() {}}}
				}))
	}, uitest.Size(600, 600))
	u.Get("grid").Focus()
	u.Press("down")
	u.Press("down")
	u.Press("down")
	u.Press("right")
	checkKeyboardMenu(t, u, "Customer 3")
}
