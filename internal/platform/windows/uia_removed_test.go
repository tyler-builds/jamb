//go:build windows

package windows

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/tyler-builds/jamb/access"
)

// removedNodes is a window with one node of each kind of pattern, so that every
// provider entry point has a provider to answer for.
func removedNodes() []*access.Node {
	return []*access.Node{
		{ID: access.RootID, Role: access.RoleWindow, Children: []access.ID{2, 3, 4, 6, 7, 8, 9, 11, 12}},
		{ID: 2, Role: access.RoleButton, Parent: access.RootID, Actions: []access.Action{access.ActionClick}},
		{ID: 3, Role: access.RoleCheckBox, Parent: access.RootID, States: access.StateToggle},
		{ID: 4, Role: access.RoleList, Parent: access.RootID, Children: []access.ID{5},
			States: access.StateSelectionContainer | access.StateMultiSelectable},
		{ID: 5, Role: access.RoleListItem, Parent: 4, States: access.StateSelectable | access.StateSelected,
			Actions: []access.Action{access.ActionClick, access.ActionScrollIntoView}},
		{ID: 6, Role: access.RoleTextField, Parent: access.RootID, Name: "Search", Value: "hello",
			Text: &access.TextInfo{Content: asciiText("hello"), OriginX: 10, OriginY: 20, Scale: 1,
				Clip: access.Rect{W: 500, H: 500}}},
		{ID: 7, Role: access.RoleSlider, Parent: access.RootID,
			Range: &access.Range{Value: 5, Min: 0, Max: 10, SmallChange: 1, LargeChange: 2}},
		{ID: 8, Role: access.RoleScrollArea, Parent: access.RootID, Scroll: access.Scroll{Container: true, CanY: true, Y: 50, ViewY: 50},
			Actions: []access.Action{access.ActionScrollIntoView}},
		{ID: 9, Role: access.RoleDataGrid, Parent: access.RootID, Children: []access.ID{10}, GridRows: 1, GridColumns: 1},
		{ID: 10, Role: access.RoleDataItem, Parent: 9, GridRow: 1, GridColumn: 1},
		{ID: 11, Role: access.RoleDialog, Parent: access.RootID},
		{ID: 12, Role: access.RoleTreeItem, Parent: access.RootID, States: access.StateExpandable},
	}
}

// removeAll publishes a snapshot with no nodes but the root, as the runtime does,
// so that the adapter's listener drops the objects of the nodes that left.
func removeAll(b *access.Bridge) {
	old := b.Tree()
	root := &access.Node{ID: access.RootID, Role: access.RoleWindow}
	next := access.NewTree(0, []*access.Node{root})
	b.Publish(next, access.Diff(old, next))
}

// rawOut returns the address of a word for a call that writes a pointer-sized
// value. The caller keeps the word alive until the call returns.
func rawOut(p *uintptr) uintptr { return uintptr(unsafe.Pointer(p)) }

// TestRemovedNodeProvidersReturnElementNotAvailable is the reproduction of the
// stale-element report. A client obtains the provider of every node and of each
// pattern, the nodes are removed, and then every entry point is called on each
// provider it still holds. Each call must return UIA_E_ELEMENTNOTAVAILABLE, the
// answer of Microsoft's provider documentation for an element that is gone, except
// for the two provider constants that the adapter answers for any node: the
// provider options and the host provider (see the server-side provider page).
func TestRemovedNodeProvidersReturnElementNotAvailable(t *testing.T) {
	nodes := removedNodes()
	b := testTree(0, nodes...)
	const hwnd = 0x7400
	w := &window{hwnd: hwnd, uiaRoot: rootView(hwnd, b, iidRawElementProviderSimple)}
	b.Listen(func(u access.Update) { w.onAccess(b, u) })
	defer func() {
		b.Listen(nil)
		dropAllObjects(hwnd)
		releaseCall(w.uiaRoot)
	}()

	// node returns the simple view of node id, and its fragment view.
	nodeOf := func(id access.ID) (simple, fragment uintptr) {
		simple = cachedObject(kindNode, id, b, hwnd, iidRawElementProviderSimple)
		if simple == 0 {
			t.Fatalf("no provider for node %d", id)
		}
		t.Cleanup(func() { releaseCall(simple) })
		return simple, coreOf(simple).viewFor(iidRawElementProviderFragment)
	}
	// pattern returns the pattern object that a node's GetPatternProvider gives.
	pattern := func(simple uintptr, id int32) uintptr {
		p := newWord()
		hr := uiaGetPatternProvider(simple, uintptr(id), rawOut(p))
		runtime.KeepAlive(p)
		if hr != hrOK || *p == 0 {
			t.Fatalf("pattern %d: HRESULT 0x%08X", id, uint32(hr))
		}
		t.Cleanup(func() { releaseCall(*p) })
		return *p
	}

	button, buttonFrag := nodeOf(2)
	checkBox, _ := nodeOf(3)
	list, _ := nodeOf(4)
	item, itemFrag := nodeOf(5)
	field, _ := nodeOf(6)
	slider, _ := nodeOf(7)
	scroll, _ := nodeOf(8)
	grid, _ := nodeOf(9)
	cell, cellFrag := nodeOf(10)
	dialog, _ := nodeOf(11)
	treeItem, _ := nodeOf(12)

	invoke := pattern(button, patternInvoke)
	toggle := pattern(checkBox, patternToggle)
	expand := pattern(treeItem, patternExpandCollapse)
	selItem := pattern(item, patternSelectionItem)
	selection := pattern(list, patternSelection)
	scrollItem := pattern(item, patternScrollItem)
	value := pattern(field, patternValue)
	textPattern := pattern(field, patternText)
	rangeValue := pattern(slider, patternRangeValue)
	scrollPattern := pattern(scroll, patternScroll)
	gridPattern := pattern(grid, patternGrid)
	tablePattern := pattern(grid, patternTable)
	gridItem := pattern(cell, patternGridItem)
	tableItem := pattern(cell, patternTableItem)
	winPattern := pattern(dialog, patternWindow)

	// A text range of the field, made before the removal, and the element that
	// the range returns.
	docRange := newWord()
	if hr := uiaTextDocumentRange(textPattern, rawOut(docRange)); hr != hrOK || *docRange == 0 {
		t.Fatalf("DocumentRange before the removal: HRESULT 0x%08X", uint32(hr))
	}
	rangeView := *docRange
	t.Cleanup(func() { releaseCall(rangeView) })
	runtime.KeepAlive(docRange)

	// ids are the nodes that the removal takes away, for the registry checks.
	ids := []access.ID{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	var disconnected []uintptr
	saved := disconnectProvider
	disconnectProvider = func(p uintptr) uintptr {
		disconnected = append(disconnected, p)
		return hrOK
	}
	defer func() { disconnectProvider = saved }()

	removeAll(b)

	// The registry drops the objects of the removed nodes (kept or not).
	for _, id := range ids {
		if n := cachedObjectCount(hwnd, id); n != 0 {
			t.Errorf("node %d still has %d registered objects after its removal", id, n)
		}
	}

	prop := func(v uintptr, id int32) uintptr {
		val := newVariant()
		hr := uiaGetPropertyValue(v, uintptr(id), uintptr(unsafe.Pointer(val)))
		runtime.KeepAlive(val)
		return hr
	}
	bounds := new([4]float64)
	outOf := func(fn func(this, out uintptr) uintptr, v uintptr) uintptr {
		p := newWord()
		hr := fn(v, rawOut(p))
		runtime.KeepAlive(p)
		return hr
	}
	// The objects are kept alive by the test, so the calls below reach the
	// removed nodes through the same views that a client holds.
	checks := []struct {
		name string
		want uintptr
		call func() uintptr
	}{
		// IRawElementProviderSimple of a removed node.
		{"Simple GetPropertyValue Name", hrElementNotAvailable, func() uintptr { return prop(button, propName) }},
		{"Simple GetPropertyValue ControlType", hrElementNotAvailable, func() uintptr { return prop(button, propControlType) }},
		{"Simple GetPropertyValue IsOffscreen", hrElementNotAvailable, func() uintptr { return prop(button, propIsOffscreen) }},
		{"Simple GetPropertyValue IsEnabled", hrElementNotAvailable, func() uintptr { return prop(button, propIsEnabled) }},
		{"Simple GetPropertyValue BoundingRectangle", hrElementNotAvailable, func() uintptr { return prop(button, 30001) }},
		{"Simple GetPropertyValue LabeledBy", hrElementNotAvailable, func() uintptr { return prop(item, propLabeledBy) }},
		{"Simple GetPropertyValue DescribedBy", hrElementNotAvailable, func() uintptr { return prop(item, propDescribedBy) }},
		{"Simple GetPatternProvider Invoke", hrElementNotAvailable, func() uintptr {
			return outOf(func(this, out uintptr) uintptr { return uiaGetPatternProvider(this, patternInvoke, out) }, button)
		}},
		{"Simple GetPatternProvider Text", hrElementNotAvailable, func() uintptr {
			return outOf(func(this, out uintptr) uintptr { return uiaGetPatternProvider(this, patternText, out) }, field)
		}},
		{"Simple GetPatternProvider Grid", hrElementNotAvailable, func() uintptr {
			return outOf(func(this, out uintptr) uintptr { return uiaGetPatternProvider(this, patternGrid, out) }, grid)
		}},
		{"Simple GetPatternProvider Window", hrElementNotAvailable, func() uintptr {
			return outOf(func(this, out uintptr) uintptr { return uiaGetPatternProvider(this, patternWindow, out) }, dialog)
		}},
		{"Simple ProviderOptions (provider constant)", hrOK, func() uintptr { return outOf(uiaProviderOptions, button) }},
		{"Simple HostRawElementProvider (NULL for a fragment)", hrOK, func() uintptr { return outOf(uiaHostProvider, button) }},
		// IRawElementProviderFragment of a removed node.
		{"Fragment Navigate parent", hrElementNotAvailable, func() uintptr {
			return outOf(func(this, out uintptr) uintptr { return uiaNavigate(this, navParent, out) }, buttonFrag)
		}},
		{"Fragment Navigate first child", hrElementNotAvailable, func() uintptr {
			return outOf(func(this, out uintptr) uintptr { return uiaNavigate(this, navFirstChild, out) }, scroll)
		}},
		{"Fragment Navigate last child", hrElementNotAvailable, func() uintptr {
			return outOf(func(this, out uintptr) uintptr { return uiaNavigate(this, navLastChild, out) }, grid)
		}},
		{"Fragment Navigate next sibling", hrElementNotAvailable, func() uintptr {
			return outOf(func(this, out uintptr) uintptr { return uiaNavigate(this, navNextSibling, out) }, itemFrag)
		}},
		{"Fragment Navigate previous sibling", hrElementNotAvailable, func() uintptr {
			return outOf(func(this, out uintptr) uintptr { return uiaNavigate(this, navPreviousSibling, out) }, cellFrag)
		}},
		{"Fragment GetRuntimeId", hrElementNotAvailable, func() uintptr { return outOf(uiaRuntimeID, buttonFrag) }},
		{"Fragment BoundingRectangle", hrElementNotAvailable, func() uintptr {
			return uiaBoundingRectangle(buttonFrag, uintptr(unsafe.Pointer(bounds)))
		}},
		{"Fragment FragmentRoot", hrElementNotAvailable, func() uintptr { return outOf(uiaFragmentRoot, itemFrag) }},
		{"Fragment EmbeddedFragmentRoots", hrElementNotAvailable, func() uintptr { return outOf(uiaEmbeddedFragmentRoots, cellFrag) }},
		{"Fragment SetFocus", hrElementNotAvailable, func() uintptr { return uiaSetFocus(buttonFrag) }},
		// Patterns of a removed node.
		{"Invoke Invoke", hrElementNotAvailable, func() uintptr { return uiaInvoke(invoke) }},
		{"Toggle Toggle", hrElementNotAvailable, func() uintptr { return uiaToggle(toggle) }},
		{"Toggle ToggleState", hrElementNotAvailable, func() uintptr { return outOf(uiaToggleState, toggle) }},
		{"ExpandCollapse Expand", hrElementNotAvailable, func() uintptr { return uiaExpand(expand) }},
		{"ExpandCollapse Collapse", hrElementNotAvailable, func() uintptr { return uiaCollapse(expand) }},
		{"ExpandCollapse ExpandCollapseState", hrElementNotAvailable, func() uintptr { return outOf(uiaExpandCollapseState, expand) }},
		{"SelectionItem Select", hrElementNotAvailable, func() uintptr { return uiaSelect(selItem) }},
		{"SelectionItem AddToSelection", hrElementNotAvailable, func() uintptr { return uiaAddToSelection(selItem) }},
		{"SelectionItem RemoveFromSelection", hrElementNotAvailable, func() uintptr { return uiaRemoveFromSelection(selItem) }},
		{"SelectionItem IsSelected", hrElementNotAvailable, func() uintptr { return outOf(uiaIsSelected, selItem) }},
		{"SelectionItem SelectionContainer", hrElementNotAvailable, func() uintptr { return outOf(uiaSelectionContainer, selItem) }},
		{"Selection GetSelection", hrElementNotAvailable, func() uintptr { return outOf(uiaGetSelection, selection) }},
		{"Selection CanSelectMultiple", hrElementNotAvailable, func() uintptr { return outOf(uiaCanSelectMultiple, selection) }},
		{"Selection IsSelectionRequired", hrElementNotAvailable, func() uintptr { return outOf(uiaIsSelectionRequired, selection) }},
		{"Value Value", hrElementNotAvailable, func() uintptr { return outOf(uiaValue, value) }},
		{"Value IsReadOnly", hrElementNotAvailable, func() uintptr { return outOf(uiaIsReadOnly, value) }},
		{"Value SetValue", hrElementNotAvailable, func() uintptr { return uiaSetValue(value, 0) }},
		{"RangeValue Value", hrElementNotAvailable, func() uintptr { return outOf(uiaRangeValue, rangeValue) }},
		{"RangeValue Maximum", hrElementNotAvailable, func() uintptr { return outOf(uiaRangeMaximum, rangeValue) }},
		{"RangeValue SetValue", hrElementNotAvailable, func() uintptr { return uiaSetRangeValue(rangeValue, 0) }},
		{"Scroll HorizontalScrollPercent", hrElementNotAvailable, func() uintptr { return outOf(uiaHorizontalScrollPercent, scrollPattern) }},
		{"Scroll VerticallyScrollable", hrElementNotAvailable, func() uintptr { return outOf(uiaVerticallyScrollable, scrollPattern) }},
		{"Scroll Scroll", hrElementNotAvailable, func() uintptr { return uiaScroll(scrollPattern, 0, 0) }},
		{"Scroll SetScrollPercent", hrElementNotAvailable, func() uintptr { return uiaSetScrollPercent(scrollPattern, 0, 0) }},
		{"ScrollItem ScrollIntoView", hrElementNotAvailable, func() uintptr { return uiaScrollIntoView(scrollItem) }},
		{"Window IsModal", hrElementNotAvailable, func() uintptr { return outOf(uiaIsModal, winPattern) }},
		{"Window WindowVisualState", hrElementNotAvailable, func() uintptr { return outOf(uiaWindowVisualState, winPattern) }},
		{"Grid GetItem", hrElementNotAvailable, func() uintptr {
			p := newWord()
			hr := uiaGridGetItem(gridPattern, 0, 0, rawOut(p))
			runtime.KeepAlive(p)
			return hr
		}},
		{"Grid RowCount", hrElementNotAvailable, func() uintptr { return outOf(uiaGridRowCount, gridPattern) }},
		{"Grid ColumnCount", hrElementNotAvailable, func() uintptr { return outOf(uiaGridColumnCount, gridPattern) }},
		{"Table ColumnHeaders", hrElementNotAvailable, func() uintptr { return outOf(uiaTableColumnHeaders, tablePattern) }},
		{"Table RowHeaders", hrElementNotAvailable, func() uintptr { return outOf(uiaTableRowHeaders, tablePattern) }},
		{"GridItem Row", hrElementNotAvailable, func() uintptr { return outOf(uiaGridItemRow, gridItem) }},
		{"GridItem ContainingGrid", hrElementNotAvailable, func() uintptr { return outOf(uiaGridItemContainingGrid, gridItem) }},
		{"TableItem ColumnHeaderItems", hrElementNotAvailable, func() uintptr { return outOf(uiaTableItemColumnHeaderItems, tableItem) }},
		{"Text DocumentRange", hrElementNotAvailable, func() uintptr { return outOf(uiaTextDocumentRange, textPattern) }},
		{"Text GetSelection", hrElementNotAvailable, func() uintptr { return outOf(uiaTextSelection, textPattern) }},
		{"Text GetVisibleRanges", hrElementNotAvailable, func() uintptr { return outOf(uiaTextVisibleRanges, textPattern) }},
		// The range that was made before the removal answers from its snapshot,
		// as the text range design says, except for the calls that act on the
		// element, which report the removal.
		{"Range Select", hrElementNotAvailable, func() uintptr { return uiaRangeSelect(rangeView) }},
		{"Range ScrollIntoView", hrElementNotAvailable, func() uintptr { return uiaRangeScrollIntoView(rangeView, 1) }},
		{"Range EnclosingElement", hrElementNotAvailable, func() uintptr { return outOf(uiaRangeEnclosingElement, rangeView) }},
	}
	for _, c := range checks {
		if hr := c.call(); hr != c.want {
			t.Errorf("%s = 0x%08X, want 0x%08X", c.name, uint32(hr), uint32(c.want))
		}
	}

	// The removal raised the structure change, and disconnected the providers of
	// the removed nodes: a disconnected provider answers the client with
	// UIA_E_ELEMENTNOTAVAILABLE without calling into the adapter.
	if len(disconnected) != len(ids) {
		t.Errorf("UiaDisconnectProvider was called %d times for %d removed nodes", len(disconnected), len(ids))
	}
}

// TestProviderOfAnAbsentNodeIsNotMade checks the race that the registry must
// close: a provider requested for a node that has already left the snapshot is
// not made, so the registry cannot keep an object that no removal will drop.
func TestProviderOfAnAbsentNodeIsNotMade(t *testing.T) {
	b := testTree(0, removedNodes()...)
	const hwnd = 0x7401
	before := registrySize()
	if v := cachedObject(kindNode, 99, b, hwnd, iidRawElementProviderSimple); v != 0 {
		releaseCall(v)
		t.Fatal("a provider was made for node 99, which is not in the snapshot")
	}
	if n := registrySize(); n != before {
		t.Errorf("the registry grew by %d objects for an absent node", n-before)
	}
	if n := cachedObjectCount(hwnd, 99); n != 0 {
		t.Errorf("node 99 has %d registered objects", n)
	}
}
