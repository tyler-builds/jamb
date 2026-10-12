package ui

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// dgButtonRig returns a rig of the grid of items with an interactive Action column,
// whose cells hold a Retry button that counts its presses, and a Name column of
// text. The row's activation is onAct.
func dgButtonRig(items []dgItem, sel *Signal[[]string], presses *int, onAct func(dgItem)) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		cols := []DataColumn[dgItem]{
			{Header: "Name", Text: func(x dgItem) string { return x.Name }, Grow: 1},
			{Header: "Action", Text: func(x dgItem) string { return "" }, Interactive: true, Width: 120,
				Cell: func(x dgItem) Element {
					return Button("Retry").Small().OnClick(func() { *presses++ })
				}},
		}
		g := DataGrid(items, func(x dgItem) string { return x.ID }, cols...).Label("Runs").
			Selection(SelectionMultiple, sel)
		if onAct != nil {
			g.OnActivate(onAct)
		}
		return g
	})
}

// dgEditRig returns a rig of the grid of rows, whose Name and N columns are editable.
// Name refuses an empty value, and N accepts 0 to 2, so that a save can fail. A save
// writes the row into rows.
func dgEditRig(rows *Signal[[]dgItem], sel *Signal[[]string], onAct func(dgItem)) *accessRig {
	return dgEditRigStyled(rows, sel, onAct, DataGridStyle{})
}

// dgEditRigStyled is dgEditRig with the look st for the grid's parts.
func dgEditRigStyled(rows *Signal[[]dgItem], sel *Signal[[]string], onAct func(dgItem), st DataGridStyle) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		save := func(x dgItem, edit func(*dgItem)) {
			rows.Update(func(xs []dgItem) []dgItem {
				out := slices.Clone(xs)
				for i := range out {
					if out[i].ID == x.ID {
						edit(&out[i])
					}
				}
				return out
			})
		}
		cols := []DataColumn[dgItem]{
			{Header: "Name", Text: func(x dgItem) string { return x.Name }, Grow: 1,
				Compare: func(a, b dgItem) int { return strings.Compare(a.Name, b.Name) },
				Edit: func(x dgItem, v string) error {
					if v == "" {
						return errors.New("Name is required")
					}
					save(x, func(r *dgItem) { r.Name = v })
					return nil
				}},
			{Header: "N", Text: func(x dgItem) string { return strconv.Itoa(x.N) }, Width: 80,
				Edit: func(x dgItem, v string) error {
					n, err := strconv.Atoi(v)
					if err != nil || n < 0 || n > 2 {
						return errors.New("N must be 0 to 2")
					}
					save(x, func(r *dgItem) { r.N = n })
					return nil
				}},
		}
		g := DataGrid(rows.Get(), func(x dgItem) string { return x.ID }, cols...).Label("Runs").
			Selection(SelectionMultiple, sel).Styles(st)
		if onAct != nil {
			g.OnActivate(onAct)
		}
		return g
	})
}

// testDataGridEditParts checks that the parts of an editing cell reach the nodes
// that draw them: CellEditing the cell being edited, Editor the frame of its field,
// and EditorMessage the message of a save that failed. It records the parts in
// applied, for the drift test.
func testDataGridEditParts(t *testing.T, applied map[string]bool) {
	open := func(t *testing.T, r *accessRig) {
		focusNode(r, dgBody(t, r))
		dgPress(r, input.KeyEnter)
	}
	t.Run("CellEditing", func(t *testing.T) {
		applied["DataGrid.CellEditing"] = true
		r := dgEditRigStyled(NewSignal(dgItems(3)), NewSignal([]string(nil)), nil,
			DataGridStyle{CellEditing: Style{Background: markBG, TextColor: markFG}})
		open(t, r)
		if !anyMarked(r.rt, "DataGrid.Cell", false) {
			t.Error("the style does not reach the cell being edited")
		}
	})
	t.Run("Editor", func(t *testing.T) {
		applied["DataGrid.Editor"] = true
		r := dgEditRigStyled(NewSignal(dgItems(3)), NewSignal([]string(nil)), nil,
			DataGridStyle{Editor: FieldStyle{Frame: Style{Background: markBG}}})
		open(t, r)
		if !anyMarked(r.rt, "DataGrid.Editor.Frame", false) {
			t.Error("the Editor style does not reach the editor's frame")
		}
	})
	t.Run("EditorMessage", func(t *testing.T) {
		applied["DataGrid.EditorMessage"] = true
		r := dgEditRigStyled(NewSignal(dgItems(3)), NewSignal([]string(nil)), nil,
			DataGridStyle{EditorMessage: Style{Background: markBG, TextColor: markFG}})
		open(t, r)
		r.press(input.KeyA, input.ModCtrl)
		dgPress(r, input.KeyDelete)
		dgPress(r, input.KeyEnter)
		if !anyMarked(r.rt, "DataGrid.EditorMessage", false) {
			t.Error("the style does not reach the message of a failed save")
		}
	})
}

// dgEditor returns the text editor of the grid's cell being edited, or nil.
func dgEditor(r *accessRig) *node {
	ns := partNodes(r.rt, "DataGrid.Editor.Text")
	if len(ns) == 0 {
		return nil
	}
	return ns[0]
}

// dgPress presses and releases k without modifiers.
func dgPress(r *accessRig, k input.Key) {
	r.press(k, 0)
}

// dgCellControls returns the controls of the built cell at row and col (0-based),
// in tree order, or nil when the cell is not built.
func dgCellControls(t *testing.T, r *accessRig, row, col int) []*node {
	t.Helper()
	c := dgCellNode(dgBody(t, r), row+1, col+1)
	if c == nil {
		return nil
	}
	return dgControls(c, nil)
}

// dgReplace selects all the text of the editor and types s over it.
func dgReplace(r *accessRig, s string) {
	r.press(input.KeyA, input.ModCtrl)
	r.typeText(s)
}

func TestSortByOrdersByKey(t *testing.T) {
	items := []dgItem{{ID: "a", N: 2}, {ID: "b", N: 0}, {ID: "c", N: 2}, {ID: "d", N: 1}}
	slices.SortStableFunc(items, SortBy(func(x dgItem) int { return x.N }))
	var ids []string
	for _, x := range items {
		ids = append(ids, x.ID)
	}
	if got := strings.Join(ids, ","); got != "b,d,a,c" {
		t.Errorf("SortBy by N orders %s, want b,d,a,c (ties keep their order)", got)
	}
	if SortBy(func(x dgItem) float32 { return 1.5 })(dgItem{}, dgItem{}) != 0 {
		t.Error("equal keys do not compare equal")
	}
}

func TestSortByTextIgnoresCase(t *testing.T) {
	names := []string{"billing", "API", "Worker", "api", "Auth"}
	slices.SortStableFunc(names, SortByText(func(s string) string { return s }))
	// Equal lowercase forms keep a total order by their case: "API" before "api".
	if got := strings.Join(names, ","); got != "API,api,Auth,billing,Worker" {
		t.Errorf("SortByText orders %s, want API,api,Auth,billing,Worker", got)
	}
}

func TestInteractiveCellKeyboardFlow(t *testing.T) {
	presses := 0
	sel := NewSignal([]string(nil))
	r := dgButtonRig(dgItems(5), sel, &presses, nil)
	body := dgBody(t, r)
	focusNode(r, body)
	dgPress(r, input.KeyRight) // the Action column
	ctl := dgCellControls(t, r, 0, 1)
	if len(ctl) != 1 {
		t.Fatalf("%d controls in the cell, want 1", len(ctl))
	}
	button := ctl[0]
	dgPress(r, input.KeyEnter)
	if r.rt.focusID != button.id {
		t.Fatalf("Enter on an interactive cell: focus %d, want its control %d", r.rt.focusID, button.id)
	}
	dgPress(r, input.KeyEnter) // Enter on the focused button presses it
	if presses != 1 {
		t.Errorf("Enter on the focused Retry: %d presses, want 1", presses)
	}
	if got := sel.Peek(); len(got) != 0 {
		t.Errorf("entering the cell selected %v, want nothing", got)
	}
	// With one control, Tab wraps to the same control; it never leaves the cell.
	dgPress(r, input.KeyTab)
	if r.rt.focusID != button.id {
		t.Errorf("Tab from the only control left the cell (focus %d)", r.rt.focusID)
	}
	dgPress(r, input.KeyEscape)
	if r.rt.focusID != body.id {
		t.Errorf("Escape: focus %d, want the grid body %d", r.rt.focusID, body.id)
	}
	dgPress(r, input.KeyF2)
	if r.rt.focusID != button.id {
		t.Error("F2 on the interactive cell did not enter it")
	}
	dgPress(r, input.KeyF2)
	if r.rt.focusID != body.id {
		t.Error("a second F2 did not return to the grid")
	}
}

func TestInteractiveCellTabCyclesItsControls(t *testing.T) {
	presses := 0
	r := newAccessRig(func(ctx *Context) Element {
		cols := []DataColumn[dgItem]{
			{Header: "Name", Text: func(x dgItem) string { return x.Name }, Grow: 1},
			{Header: "Actions", Text: func(x dgItem) string { return "" }, Interactive: true, Width: 200,
				Cell: func(x dgItem) Element {
					return Div().Row().Gap(4).
						Child(Button("Retry").Small().OnClick(func() { presses++ })).
						Child(Button("Open").Small())
				}},
		}
		return DataGrid(dgItems(3), func(x dgItem) string { return x.ID }, cols...).Label("Runs")
	})
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyRight)
	ctl := dgCellControls(t, r, 0, 1)
	if len(ctl) != 2 {
		t.Fatalf("%d controls in the cell, want 2", len(ctl))
	}
	focusNode(r, ctl[0])
	dgPress(r, input.KeyTab)
	if r.rt.focusID != ctl[1].id {
		t.Errorf("Tab from Retry: focus %d, want Open %d", r.rt.focusID, ctl[1].id)
	}
	dgPress(r, input.KeyTab)
	if r.rt.focusID != ctl[0].id {
		t.Errorf("Tab from the last control: focus %d, want the first, which wraps inside the cell", r.rt.focusID)
	}
	dgPress(r, input.KeyTab)
	r.press(input.KeyTab, input.ModShift)
	if r.rt.focusID != ctl[0].id {
		t.Errorf("Shift+Tab from Open: focus %d, want Retry %d", r.rt.focusID, ctl[0].id)
	}
	dgPress(r, input.KeyEnter)
	if presses != 1 {
		t.Errorf("Enter on Retry after Tab: %d presses, want 1", presses)
	}
}

func TestInteractiveCellArrowsBelongToTheControl(t *testing.T) {
	text := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		cols := []DataColumn[dgItem]{
			{Header: "Name", Text: func(x dgItem) string { return x.Name }, Grow: 1},
			{Header: "Note", Text: func(x dgItem) string { return "" }, Interactive: true, Width: 160,
				Cell: func(x dgItem) Element { return TextInput(text).Label("Note") }},
		}
		return DataGrid(dgItems(4), func(x dgItem) string { return x.ID }, cols...).Label("Runs")
	})
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyRight)
	dgPress(r, input.KeyEnter)
	ctl := dgCellControls(t, r, 0, 1)
	if len(ctl) != 1 || r.rt.focusID != ctl[0].id {
		t.Fatal("Enter on the cell did not focus its text field")
	}
	ed := ctl[0]
	r.typeText("abc")
	dgPress(r, input.KeyLeft)
	dgPress(r, input.KeyDown)
	if got := dgFocusCell(r); got == nil || got.div.acc.gridRow != 1 || got.div.acc.gridCol != 2 {
		t.Error("arrow keys in the field moved the cursor off its cell")
	}
	if r.rt.focusID != ed.id {
		t.Error("arrow keys left the text field")
	}
	if text.Peek() != "abc" {
		t.Errorf("text %q, want abc", text.Peek())
	}
}

func TestInteractiveCellPointerDoesNotSelectTheRow(t *testing.T) {
	presses := 0
	activated := 0
	sel := NewSignal([]string(nil))
	r := dgButtonRig(dgItems(5), sel, &presses, func(dgItem) { activated++ })
	focusNode(r, dgBody(t, r))
	target := dgCellControls(t, r, 1, 1)
	if len(target) != 1 {
		t.Fatalf("%d controls in the cell of row 1, want 1", len(target))
	}
	dgClick(r, target[0], 0)
	if presses != 1 {
		t.Errorf("a click on Retry: %d presses, want 1", presses)
	}
	if got := sel.Peek(); len(got) != 0 {
		t.Errorf("a click on a control selected %v, want nothing", got)
	}
	dgDoubleClick(r, target[0])
	if activated != 0 {
		t.Errorf("a double-click on a control activated the row %d times", activated)
	}
	// A click on the name of a row selects it, as it always has.
	dgClick(r, dgCell(r, 1, 0), 0)
	if got := sel.Peek(); len(got) != 1 || got[0] != "i1" {
		t.Errorf("a click on the name selected %v, want i1", got)
	}
}

func TestEditableCellStartsByEnterF2AndTyping(t *testing.T) {
	rows := NewSignal(dgItems(4))
	r := dgEditRig(rows, NewSignal([]string(nil)), nil)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter)
	ed := dgEditor(r)
	if ed == nil || r.rt.focusID != ed.id {
		t.Fatal("Enter on an editable cell did not start an edit with focus in the editor")
	}
	dgPress(r, input.KeyEscape)
	dgPress(r, input.KeyF2)
	if dgEditor(r) == nil {
		t.Fatal("F2 did not start an edit")
	}
	dgPress(r, input.KeyEscape)
	focusNode(r, dgBody(t, r))
	r.typeText("Q")
	if dgEditor(r) == nil {
		t.Fatal("typing a character did not start an edit")
	}
	dgPress(r, input.KeyEnter)
	if got := rows.Peek()[0].Name; got != "Q" {
		t.Errorf("typing replaced the text with %q, want Q", got)
	}
}

func TestEditCommitSavesAndKeepsSelectionAndFocus(t *testing.T) {
	rows := NewSignal(dgItems(4))
	sel := NewSignal([]string{"i0"})
	r := dgEditRig(rows, sel, nil)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter)
	dgReplace(r, "Saved")
	dgPress(r, input.KeyEnter)
	if got := rows.Peek()[0].Name; got != "Saved" {
		t.Errorf("saved name %q, want Saved", got)
	}
	if got := sel.Peek(); len(got) != 1 || got[0] != "i0" {
		t.Errorf("selection after a save %v, want i0", got)
	}
	if dgEditor(r) != nil {
		t.Error("the editor is still open after a save")
	}
	if r.rt.focusID != dgBody(t, r).id {
		t.Error("focus did not return to the cell after a save")
	}
}

func TestEditErrorKeepsTheEditorOpenAndAnnounces(t *testing.T) {
	rows := NewSignal(dgItems(3))
	r := dgEditRig(rows, NewSignal([]string(nil)), nil)
	var said []access.Announcement
	r.bridge.Listen(func(u access.Update) { said = append(said, u.Announcements...) })
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter)
	r.press(input.KeyA, input.ModCtrl)
	dgPress(r, input.KeyDelete)
	dgPress(r, input.KeyEnter)
	ed := dgEditor(r)
	if ed == nil || r.rt.focusID != ed.id {
		t.Fatal("an invalid save closed the editor or moved focus")
	}
	if got := rows.Peek()[0].Name; got == "" {
		t.Error("an invalid value was saved")
	}
	if n := len(partNodes(r.rt, "DataGrid.EditorMessage")); n != 1 {
		t.Fatalf("%d error messages, want 1", n)
	}
	if f := r.tree().Find(access.RoleTextField, "Name"); f == nil || f.Description != "Name is required" {
		t.Errorf("the editor does not describe the error: %+v", f)
	}
	announced := false
	for _, a := range said {
		announced = announced || (a.Text == "Name is required" && a.Politeness == access.Assertive)
	}
	if !announced {
		t.Errorf("the error was not announced assertively: %+v", said)
	}
	// A valid value saves and closes the editor.
	r.typeText("ok")
	dgPress(r, input.KeyEnter)
	if dgEditor(r) != nil {
		t.Error("the editor stayed open after a valid save")
	}
}

func TestEditEscapeCancels(t *testing.T) {
	rows := NewSignal(dgItems(3))
	r := dgEditRig(rows, NewSignal([]string(nil)), nil)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter)
	dgReplace(r, "zzz")
	dgPress(r, input.KeyEscape)
	if dgEditor(r) != nil {
		t.Error("Escape left the editor open")
	}
	if got := rows.Peek()[0].Name; got != "Item 000000" {
		t.Errorf("Escape saved %q", got)
	}
	if r.rt.focusID != dgBody(t, r).id {
		t.Error("Escape did not return focus to the cell")
	}
}

func TestEditTabMovesToTheNextEditableCell(t *testing.T) {
	rows := NewSignal(dgItems(3))
	r := dgEditRig(rows, NewSignal([]string(nil)), nil)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter) // Name of row 0
	dgReplace(r, "T")
	dgPress(r, input.KeyTab) // saves Name, and starts an edit of N in the same row
	if got := rows.Peek()[0].Name; got != "T" {
		t.Errorf("Tab saved %q, want T", got)
	}
	if dgEditor(r) == nil {
		t.Fatal("Tab did not start an edit of the next editable cell")
	}
	if c := dgFocusCell(r); c == nil || c.div.acc.gridCol != 2 {
		t.Fatal("the cursor did not move to the next editable cell")
	}
	dgPress(r, input.KeyTab) // N is the last editable cell: the edit ends, focus stays in the grid
	if dgEditor(r) != nil {
		t.Error("Tab at the last editable cell left the editor open")
	}
	if r.rt.focusID != dgBody(t, r).id {
		t.Error("Tab at the last editable cell did not stay in the grid")
	}
}

// TestEditBlurSavesTheText checks that leaving the editor by a click elsewhere saves
// the text, as a spreadsheet does, and that Escape is the only cancel of an edit.
func TestEditBlurSavesTheText(t *testing.T) {
	rows := NewSignal(dgItems(3))
	r := dgEditRig(rows, NewSignal([]string(nil)), nil)
	r.rt.WindowFocus(true)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter)
	r.typeText("!")
	dgClick(r, dgCell(r, 2, 0), 0) // a click elsewhere in the grid
	if dgEditor(r) != nil {
		t.Error("a click elsewhere left the editor open after a save")
	}
	if got := rows.Peek()[0].Name; got != "!Item 000000" {
		t.Errorf("the blur saved %q, want the typed text", got)
	}
}

// TestEditBlurKeepsAFailedSave checks a blur whose save fails: the editor stays open
// with the error, and focus goes back to it while the window has focus. With the
// window out of focus, the editor stays open and focus does not come back.
func TestEditBlurKeepsAFailedSave(t *testing.T) {
	rows := NewSignal(dgItems(3))
	r := dgEditRig(rows, NewSignal([]string(nil)), nil)
	r.rt.WindowFocus(true)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter)
	r.press(input.KeyA, input.ModCtrl)
	dgPress(r, input.KeyDelete) // an empty name is refused
	dgClick(r, dgCell(r, 2, 0), 0)
	if ed := dgEditor(r); ed == nil || r.rt.focusID != ed.id {
		t.Fatal("a failed blur save did not keep the editor open with focus in it")
	}
	if n := len(partNodes(r.rt, "DataGrid.EditorMessage")); n != 1 {
		t.Errorf("%d error messages after a failed blur save, want 1", n)
	}
	if rows.Peek()[0].Name == "" {
		t.Error("an empty name was saved on blur")
	}

	// Out of focus, the same failure keeps the editor, and focus stays where it is.
	r = dgEditRig(rows, NewSignal([]string(nil)), nil)
	r.rt.WindowFocus(true)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter)
	r.press(input.KeyA, input.ModCtrl)
	dgPress(r, input.KeyDelete)
	r.rt.WindowFocus(false)
	dgClick(r, dgCell(r, 2, 0), 0)
	if dgEditor(r) == nil {
		t.Fatal("a failed blur save closed the editor while the window was out of focus")
	}
	if ed := dgEditor(r); r.rt.focusID == ed.id {
		t.Error("focus came back to the editor while the window was out of focus")
	}
}

func TestEditDoubleClickStartsAnEditAndKeepsTheRow(t *testing.T) {
	rows := NewSignal(dgItems(3))
	activated := 0
	r := dgEditRig(rows, NewSignal([]string(nil)), func(dgItem) { activated++ })
	focusNode(r, dgBody(t, r))
	dgDoubleClick(r, dgCell(r, 1, 0))
	if dgEditor(r) == nil {
		t.Fatal("a double-click on an editable cell did not start an edit")
	}
	if activated != 0 {
		t.Errorf("a double-click on an editable cell activated the row %d times", activated)
	}
}

func TestEditingCellIsExposedInTheTree(t *testing.T) {
	rows := NewSignal(dgItems(3))
	r := dgEditRig(rows, NewSignal([]string(nil)), nil)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter)
	tr := r.tree()
	if tr.Find(access.RoleTextField, "Name") == nil {
		t.Error("the editor is not exposed as a text field named for its column")
	}
	if tr.Find(access.RoleDataItem, "Item 000000") == nil {
		t.Error("the edited cell is not a named data item")
	}
}

func TestInteractiveCellExposesItsControls(t *testing.T) {
	presses := 0
	r := dgButtonRig(dgItems(3), NewSignal([]string(nil)), &presses, nil)
	tr := r.tree()
	if tr.Find(access.RoleButton, "Retry") == nil {
		t.Error("the Retry button of a cell is not in the access tree")
	}
	if tr.Find(access.RoleDataItem, "Item 000000") == nil {
		t.Error("the cell of an interactive column is not a named data item")
	}
}

// TestEditStartsWithValueWhenTextDiffers checks that an edit starts with the Value of
// the row, when the cell shows something else: a formatted text, such as "12 tasks".
func TestEditStartsWithValueWhenTextDiffers(t *testing.T) {
	rows := NewSignal(dgItems(2))
	r := newAccessRig(func(ctx *Context) Element {
		cols := []DataColumn[dgItem]{
			{Header: "Tasks", Text: func(x dgItem) string { return "tasks: " + strconv.Itoa(x.N) },
				Value: func(x dgItem) string { return strconv.Itoa(x.N) },
				Edit:  func(x dgItem, v string) error { return nil }},
		}
		return DataGrid(rows.Get(), func(x dgItem) string { return x.ID }, cols...).Label("Runs")
	})
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter)
	ed := dgEditor(r)
	if ed == nil {
		t.Fatal("Enter did not start the edit")
	}
	if text := r.tree().Find(access.RoleTextField, "Tasks"); text == nil || text.Value != "0" {
		t.Errorf("the edit starts with %+v, want the Value 0", text)
	}
}

// TestEditKeepsItsTextWhenItsRowScrollsAway checks what the grid promises for an edit
// whose row leaves the view: the edit, its text and its focus are kept, and the
// editor is laid out again when the row comes back.
func TestEditKeepsItsTextWhenItsRowScrollsAway(t *testing.T) {
	rows := NewSignal(dgItems(60))
	r := dgEditRig(rows, NewSignal([]string(nil)), nil)
	body := dgBody(t, r)
	focusNode(r, body)
	dgPress(r, input.KeyEnter)
	dgReplace(r, "kept")
	r.rt.scrollBy(body, 0, 100000)
	r.frame()
	if ed := dgEditor(r); ed == nil || r.rt.focusID != ed.id {
		t.Fatal("scrolling the edited row out of view lost the edit or its focus")
	}
	r.rt.scrollBy(body, 0, -100000)
	r.frame()
	ed := dgEditor(r)
	if ed == nil || r.rt.focusID != ed.id {
		t.Fatal("the editor did not come back with focus when its row did")
	}
	if got := ed.editor.value.Peek(); got != "kept" {
		t.Errorf("the editor came back with %q, want the text it had", got)
	}
}

// dgCustomRig returns a rig of the grid of rows whose Pick column is edited by a
// custom editor: a focusable control that takes Space as a change to "picked", and
// takes Enter only when takeEnter is set. Pick saves its value into choice, and
// refuses "bad".
func dgCustomRig(rows *Signal[[]dgItem], choice *Signal[string], takeEnter bool) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		cols := []DataColumn[dgItem]{
			{Header: "Name", Text: func(x dgItem) string { return x.Name }, Grow: 1},
			{Header: "Pick", Text: func(x dgItem) string { return choice.Peek() }, Width: 160,
				Edit: func(x dgItem, v string) error {
					if v == "bad" {
						return errors.New("bad is refused")
					}
					choice.Set(v)
					return nil
				},
				Editor: func(x dgItem, value string, change func(string), commit, cancel func()) Element {
					return Div().Focusable().Child(Text(value)).OnKeyDown(func(e *KeyEvent) {
						switch {
						case e.Key == input.KeyB && !e.Repeat:
							change("bad")
							e.StopPropagation()
						case e.Key == input.KeySpace && !e.Repeat:
							change("picked")
							e.StopPropagation()
						case e.Key == input.KeyEnter && takeEnter:
							e.StopPropagation()
						}
					})
				}},
		}
		return DataGrid(rows.Get(), func(x dgItem) string { return x.ID }, cols...).Label("Runs")
	})
}

// TestCustomEditorKeysAndFocus checks a custom editor: F2 gives focus to its control,
// its own keys come first, Enter saves the pending value when the control does not
// take it, Escape cancels, a blur saves, and typing does not start it.
func TestCustomEditorKeysAndFocus(t *testing.T) {
	rows := NewSignal(dgItems(3))
	choice := NewSignal("first")
	r := dgCustomRig(rows, choice, false)
	r.rt.WindowFocus(true)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyRight)
	r.typeText("x")
	if dgEditor(r) != nil || len(dgCellControls(t, r, 0, 1)) != 0 {
		t.Fatal("typing started a custom editor")
	}
	dgPress(r, input.KeyF2)
	ctl := dgCellControls(t, r, 0, 1)
	if len(ctl) != 1 || r.rt.focusID != ctl[0].id {
		t.Fatal("F2 did not give focus to the custom editor's control")
	}
	dgPress(r, input.KeySpace) // the control changes the pending value
	dgPress(r, input.KeyEnter) // not taken by the control: the grid saves it
	if got := choice.Peek(); got != "picked" {
		t.Errorf("Enter saved %q, want the pending value picked", got)
	}
	if r.rt.focusID != dgBody(t, r).id || dgCellControls(t, r, 0, 1) != nil {
		t.Error("the save did not return focus to the cell")
	}

	// Escape cancels, and a blur saves.
	dgPress(r, input.KeyF2)
	dgPress(r, input.KeySpace)
	dgPress(r, input.KeyEscape)
	if dgCellControls(t, r, 0, 1) != nil || choice.Peek() != "picked" {
		t.Errorf("Escape did not cancel; choice %q", choice.Peek())
	}
	focusNode(r, dgBody(t, r))
	choice.Set("first")
	dgPress(r, input.KeyF2)
	dgPress(r, input.KeySpace)
	dgClick(r, dgCell(r, 2, 0), 0)
	if choice.Peek() != "picked" {
		t.Errorf("a blur saved %q, want picked", choice.Peek())
	}
}

// TestCustomEditorControlTakesEnter checks the precedence: a control that takes Enter
// keeps it, so the grid does not save until a later key does.
func TestCustomEditorControlTakesEnter(t *testing.T) {
	rows := NewSignal(dgItems(3))
	choice := NewSignal("first")
	r := dgCustomRig(rows, choice, true)
	r.rt.WindowFocus(true)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyRight)
	dgPress(r, input.KeyF2)
	dgPress(r, input.KeySpace)
	dgPress(r, input.KeyEnter)
	if dgCellControls(t, r, 0, 1) == nil || choice.Peek() != "first" {
		t.Error("Enter that the control took also saved, or closed the editor")
	}
	dgPress(r, input.KeyTab) // Tab is not the control's: the grid saves the pending value
	if choice.Peek() != "picked" {
		t.Errorf("Tab saved %q, want picked", choice.Peek())
	}
}

// TestCustomEditorSaveErrorKeepsItOpen checks that a custom editor's refused save
// keeps the editor open with focus in its control, and that its message is shown.
func TestCustomEditorSaveErrorKeepsItOpen(t *testing.T) {
	rows := NewSignal(dgItems(3))
	choice := NewSignal("first")
	r := dgCustomRig(rows, choice, false)
	r.rt.WindowFocus(true)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyRight)
	dgPress(r, input.KeyF2)
	dgPress(r, input.KeyB) // the control changes the pending value to "bad"
	dgPress(r, input.KeyEnter)
	ctl := dgCellControls(t, r, 0, 1)
	if len(ctl) != 1 || choice.Peek() != "first" {
		t.Fatal("a refused save closed the custom editor or saved its value")
	}
	if r.rt.focusID != ctl[0].id {
		t.Error("a refused save moved focus out of the custom editor")
	}
	if n := len(partNodes(r.rt, "DataGrid.EditorMessage")); n != 1 {
		t.Errorf("%d messages after a refused save, want 1", n)
	}
}

// TestEditableCellExposesItsValuePattern checks the Set Value action of an editable
// cell: it sets the value through Edit, an invalid value is refused and announced
// assertively, and a read-only cell offers no Set Value action.
func TestEditableCellExposesItsValuePattern(t *testing.T) {
	rows := NewSignal(dgItems(3))
	r := dgEditRig(rows, NewSignal([]string(nil)), nil)
	var said []access.Announcement
	r.bridge.Listen(func(u access.Update) { said = append(said, u.Announcements...) })
	cell := r.tree().Find(access.RoleDataItem, "Item 000000")
	if cell == nil || !slices.Contains(cell.Actions, access.ActionSetValue) {
		t.Fatal("an editable cell does not offer Set Value")
	}
	r.bridge.Do(cell.ID, access.Request{Action: access.ActionSetValue, Value: "Set by UIA"})
	r.frame()
	if got := rows.Peek()[0].Name; got != "Set by UIA" {
		t.Errorf("Set Value saved %q, want the value set", got)
	}
	cell = r.tree().Find(access.RoleDataItem, "Set by UIA")
	if cell == nil {
		t.Fatal("the cell does not show the value set")
	}
	r.bridge.Do(cell.ID, access.Request{Action: access.ActionSetValue, Value: ""})
	r.rt.markAccess()
	r.frame()
	if got := rows.Peek()[0].Name; got != "Set by UIA" {
		t.Errorf("an invalid Set Value changed the value to %q", got)
	}
	announced := false
	for _, a := range said {
		announced = announced || (a.Text == "Name is required" && a.Politeness == access.Assertive)
	}
	if !announced {
		t.Errorf("the refused value was not announced assertively: %+v", said)
	}
	if dgEditor(r) != nil {
		t.Error("Set Value opened the editor")
	}

	ro := dgButtonRig(dgItems(2), NewSignal([]string(nil)), new(int), nil)
	if n := ro.tree().Find(access.RoleDataItem, "Item 000000"); n == nil || slices.Contains(n.Actions, access.ActionSetValue) {
		t.Error("a read-only cell offers Set Value")
	}
}

// TestEditMessageIsAPopoverOnTheCell checks that a failed save's message is an
// overlay of the editing cell, and that the cell clips what it draws.
func TestEditMessageIsAPopoverOnTheCell(t *testing.T) {
	rows := NewSignal(dgItems(3))
	r := dgEditRig(rows, NewSignal([]string(nil)), nil)
	focusNode(r, dgBody(t, r))
	dgPress(r, input.KeyEnter)
	r.press(input.KeyA, input.ModCtrl)
	dgPress(r, input.KeyDelete)
	dgPress(r, input.KeyEnter)
	msg := partNodes(r.rt, "DataGrid.EditorMessage")
	if len(msg) != 1 {
		t.Fatalf("%d messages, want 1", len(msg))
	}
	if msg[0].div.overlay != BelowStart {
		t.Errorf("the message is not an overlay below the cell (placement %v)", msg[0].div.overlay)
	}
	if msg[0].parent == nil || msg[0].parent.div == nil || msg[0].parent.div.part != "DataGrid.Cell" {
		t.Error("the message is not a child of the editing cell")
	}
	for _, c := range partNodes(r.rt, "DataGrid.Cell") {
		if c.style.OverflowX != OverflowHidden || c.style.OverflowY != OverflowHidden {
			t.Fatalf("a cell does not clip its content (overflow %v, %v)", c.style.OverflowX, c.style.OverflowY)
		}
	}
}
