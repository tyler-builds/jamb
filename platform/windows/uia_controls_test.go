//go:build windows

package windows

import (
	"fmt"
	"math"
	"runtime"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/win32"
)

// newDouble returns a float64 on the heap, for an out parameter that COM writes
// (see newVariant).
//
//go:noinline
func newDouble() *float64 { return new(float64) }

// testHwnd gives each test that makes pattern objects its own window handle, since
// the registry keeps the pattern objects of a node per window.
var testHwnd uintptr = 0x7200

// patternOn returns a new reference to the pattern of the given ID that the node
// id of the bridge has, and fails the test when it has none.
func patternOn(t *testing.T, b *access.Bridge, id access.ID, pattern int32) uintptr {
	t.Helper()
	testHwnd++
	v := create(kindNode, id, b, testHwnd, iidRawElementProviderSimple)
	t.Cleanup(func() { releaseCall(v) })
	p, hr := getOut(comMethod(v, 4), v, uintptr(pattern))
	if hr != hrOK || p == 0 {
		t.Fatalf("node %d has no pattern %d (HRESULT 0x%08X)", id, pattern, uint32(hr))
	}
	return p
}

// guidText formats an IID the way the IDL writes a uuid attribute.
func guidText(g guid) string {
	d := g.Data4
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x", g.Data1, g.Data2, g.Data3,
		d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
}

// TestPatternIIDsMatchTheSDK checks the interface IIDs against the uuid attributes
// in UIAutomationCore.idl, from the Windows SDK 10.0.26100.0.
func TestPatternIIDsMatchTheSDK(t *testing.T) {
	cases := []struct {
		name string
		g    guid
		uuid string
	}{
		{"IRawElementProviderSimple", iidRawElementProviderSimple, "d6dd68d1-86fd-4332-8666-9abedea2d24c"},
		{"IRawElementProviderFragment", iidRawElementProviderFragment, "f7063da8-8359-439c-9297-bbc5299a7d87"},
		{"IRawElementProviderFragmentRoot", iidRawElementProviderFragmentRoot, "620ce2a5-ab8f-40a9-86cb-de3c75599b58"},
		{"IInvokeProvider", iidInvokeProvider, "54fcb24b-e18e-47a2-b4d3-eccbe77599a2"},
		{"IToggleProvider", iidToggleProvider, "56d00bd0-c4f4-433c-a836-1a52a57e0892"},
		{"IExpandCollapseProvider", iidExpandCollapseProvider, "d847d3a5-cab0-4a98-8c32-ecb45c59ad24"},
		{"ISelectionItemProvider", iidSelectionItemProvider, "2acad808-b2d4-452d-a407-91ff1ad167b2"},
		{"ISelectionProvider", iidSelectionProvider, "fb8b03af-3bdf-48d4-bd36-1a65793be168"},
		{"IValueProvider", iidValueProvider, "c7935180-6fb3-4201-b174-7df73adbf64a"},
		{"IRangeValueProvider", iidRangeValueProvider, "36dc7aef-33e6-4691-afe1-2be7274b3d33"},
		{"IScrollProvider", iidScrollProvider, "b38b8077-1fc3-42a5-8cae-d40c2215055a"},
		{"IScrollItemProvider", iidScrollItemProvider, "2360c714-4bf1-4b26-ba65-9b21316127eb"},
	}
	for _, c := range cases {
		if got := guidText(c.g); got != c.uuid {
			t.Errorf("%s has IID %s, the SDK has %s", c.name, got, c.uuid)
		}
	}
}

func TestVariantsOfTheNewTypes(t *testing.T) {
	if v := variantDouble(2.5); v.vt != vtR8 || math.Float64frombits(v.data[0]) != 2.5 {
		t.Errorf("variantDouble(2.5) = tag %d value %v", v.vt, math.Float64frombits(v.data[0]))
	}
	if v := variantUnknown(0x1234); v.vt != vtUnknown || v.data[0] != 0x1234 {
		t.Errorf("variantUnknown = tag %d value %#x", v.vt, v.data[0])
	}
	if v := variantUnknownArray(0x5678); v.vt != vtUnknown|vtArray || v.data[0] != 0x5678 {
		t.Errorf("variantUnknownArray = tag %#x value %#x, want VT_UNKNOWN|VT_ARRAY", v.vt, v.data[0])
	}
}

// TestUnknownArrayHoldsTheProviders checks the SAFEARRAY of IUnknown that
// DescribedBy and GetSelection return: its elements are the views that were put
// in, and the array keeps its own references.
func TestUnknownArrayHoldsTheProviders(t *testing.T) {
	b := testTree(0, node(access.RootID, 0, 2, 3), node(2, access.RootID), node(3, access.RootID))
	a := create(kindNode, 2, b, 0x7101, iidRawElementProviderSimple)
	defer releaseCall(a)
	c := create(kindNode, 3, b, 0x7101, iidRawElementProviderSimple)
	defer releaseCall(c)
	psa := win32.SafeArrayCreateVector(vtUnknown, 0, 2)
	if psa == 0 {
		t.Fatal("SafeArrayCreateVector failed")
	}
	defer win32.SafeArrayDestroy(psa)
	win32.SafeArrayPutUnknown(psa, 0, a)
	win32.SafeArrayPutUnknown(psa, 1, c)
	if ub := win32.SafeArrayUBound(psa); ub != 1 {
		t.Fatalf("upper bound %d, want 1", ub)
	}
	var got uintptr
	if hr := win32.SafeArrayGetUnknown(psa, 1, &got); hr != hrOK || got == 0 {
		t.Fatalf("SafeArrayGetUnknown HRESULT 0x%08X", uint32(hr))
	}
	defer releaseCall(got)
	if id := coreOf(got).id; id != 3 {
		t.Errorf("element 1 is node %d, want 3", id)
	}
}

// TestPatternsFollowRolesAndStates checks which patterns each kind of node has,
// by asking for every pattern of each node.
func TestPatternsFollowRolesAndStates(t *testing.T) {
	nodes := []*access.Node{
		{ID: access.RootID, Role: access.RoleWindow, Children: []access.ID{2, 3, 5, 6, 7}},
		{ID: 2, Role: access.RoleSwitch, Parent: access.RootID, States: access.StateToggle},
		{ID: 3, Role: access.RoleButton, Parent: access.RootID, States: access.StateExpandable,
			Actions: []access.Action{access.ActionClick}},
		{ID: 4, Role: access.RoleButton, Parent: 5, States: access.StateSelectable},
		{ID: 5, Role: access.RoleGroup, Parent: access.RootID, States: access.StateSelectionContainer,
			Children: []access.ID{4}},
		{ID: 6, Role: access.RoleTextField, Parent: access.RootID},
		{ID: 7, Role: access.RoleScrollArea, Parent: access.RootID, Scroll: access.Scroll{Container: true},
			Children: []access.ID{8}},
		{ID: 8, Role: access.RoleListItem, Parent: 7, Actions: []access.Action{access.ActionScrollIntoView}},
	}
	b := access.NewBridge()
	b.Publish(access.NewTree(0, nodes), access.Update{})
	want := map[access.ID][]int32{
		2: {patternToggle},
		3: {patternInvoke, patternExpandCollapse},
		4: {patternSelectionItem},
		5: {patternSelection},
		6: {patternValue},
		7: {patternScroll},
		8: {patternScrollItem},
	}
	all := []int32{patternInvoke, patternToggle, patternExpandCollapse, patternSelectionItem,
		patternSelection, patternValue, patternScroll, patternScrollItem}
	testHwnd++
	for id := access.ID(2); id <= 8; id++ {
		v := create(kindNode, id, b, testHwnd, iidRawElementProviderSimple)
		for _, p := range all {
			got, hr := getOut(comMethod(v, 4), v, uintptr(p))
			wanted := false
			for _, w := range want[id] {
				wanted = wanted || w == p
			}
			if hr != hrOK || (got != 0) != wanted {
				t.Errorf("node %d has pattern %d = %v (HRESULT 0x%08X), want %v", id, p, got != 0, uint32(hr), wanted)
			}
			if got != 0 {
				releaseCall(got)
			}
		}
		releaseCall(v)
	}
}

// TestInvokeFollowsTogglesAndDisabledButtons checks that a toggle button has
// Toggle and not Invoke (Axe.Windows reports both together), and that a disabled
// button keeps Invoke with no action.
func TestInvokeFollowsTogglesAndDisabledButtons(t *testing.T) {
	nodes := []*access.Node{
		{ID: access.RootID, Role: access.RoleWindow, Children: []access.ID{2, 3, 4}},
		{ID: 2, Role: access.RoleButton, Parent: access.RootID, States: access.StateToggle | access.StateChecked,
			Actions: []access.Action{access.ActionClick, access.ActionToggle}},
		{ID: 3, Role: access.RoleButton, Parent: access.RootID, States: access.StateDisabled},
		{ID: 4, Role: access.RoleButton, Parent: access.RootID, Actions: []access.Action{access.ActionClick}},
	}
	b := access.NewBridge()
	b.Publish(access.NewTree(0, nodes), access.Update{})
	want := map[access.ID][]int32{
		2: {patternToggle},
		3: {patternInvoke},
		4: {patternInvoke},
	}
	testHwnd++
	for id := access.ID(2); id <= 4; id++ {
		v := create(kindNode, id, b, testHwnd, iidRawElementProviderSimple)
		for _, p := range []int32{patternInvoke, patternToggle} {
			got, hr := getOut(comMethod(v, 4), v, uintptr(p))
			wanted := false
			for _, w := range want[id] {
				wanted = wanted || w == p
			}
			if hr != hrOK || (got != 0) != wanted {
				t.Errorf("node %d has pattern %d = %v (HRESULT 0x%08X), want %v", id, p, got != 0, uint32(hr), wanted)
			}
			if got != 0 {
				releaseCall(got)
			}
		}
		releaseCall(v)
	}
}

func TestTogglePatternTogglesTheNode(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleSwitch, Parent: access.RootID, States: access.StateToggle | access.StateChecked}
	b := testTree(0, node(access.RootID, 0, 2), n)
	var got []access.Request
	b.SetHooks(nil, func(id access.ID, r access.Request) { got = append(got, r) })
	p := patternOn(t, b, 2, patternToggle)

	state, hr := getOut(comMethod(p, 4), p)
	if hr != hrOK || int32(state) != toggleStateOn {
		t.Fatalf("ToggleState = %d (HRESULT 0x%08X), want on", int32(state), uint32(hr))
	}
	if hr, _, _ := syscall.SyscallN(comMethod(p, 3), p); hr != hrOK {
		t.Fatalf("Toggle HRESULT 0x%08X", uint32(hr))
	}
	if len(got) != 1 || got[0].Action != access.ActionToggle {
		t.Errorf("Toggle requested %v, want one toggle", got)
	}
}

func TestToggleStateValues(t *testing.T) {
	cases := map[string]struct {
		s    access.State
		want int32
	}{
		"off":           {access.StateToggle, toggleStateOff},
		"on":            {access.StateToggle | access.StateChecked, toggleStateOn},
		"indeterminate": {access.StateToggle | access.StateMixed, toggleStateIndeterminate},
	}
	for name, c := range cases {
		if got := toggleStateOf(c.s); got != c.want {
			t.Errorf("%s: toggleStateOf = %d, want %d", name, got, c.want)
		}
	}
}

// TestExpandCollapsePatternOpensAndCloses checks the requests that Expand and
// Collapse make, and the state they read.
func TestExpandCollapsePatternOpensAndCloses(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleButton, Parent: access.RootID,
		States: access.StateExpandable | access.StateExpanded}
	b := testTree(0, node(access.RootID, 0, 2), n)
	var got []access.Action
	b.SetHooks(nil, func(id access.ID, r access.Request) { got = append(got, r.Action) })
	p := patternOn(t, b, 2, patternExpandCollapse)

	state, _ := getOut(comMethod(p, 5), p)
	if int32(state) != expandCollapseExpanded {
		t.Errorf("ExpandCollapseState = %d, want expanded", int32(state))
	}
	syscall.SyscallN(comMethod(p, 3), p) // Expand
	syscall.SyscallN(comMethod(p, 4), p) // Collapse
	if len(got) != 2 || got[0] != access.ActionExpand || got[1] != access.ActionCollapse {
		t.Errorf("requests %v, want expand then collapse", got)
	}
}

// TestSelectionItemAndContainer checks the item's state, its container, and the
// container's selection.
func TestSelectionItemAndContainer(t *testing.T) {
	chosen := &access.Node{ID: 3, Role: access.RoleButton, Parent: 2, States: access.StateSelectable | access.StateSelected}
	other := &access.Node{ID: 4, Role: access.RoleButton, Parent: 2, States: access.StateSelectable}
	group := &access.Node{ID: 2, Role: access.RoleGroup, Parent: access.RootID,
		States: access.StateSelectionContainer, Children: []access.ID{3, 4}}
	b := testTree(0, node(access.RootID, 0, 2), group, chosen, other)
	var got []access.Action
	b.SetHooks(nil, func(id access.ID, r access.Request) { got = append(got, r.Action) })

	item := patternOn(t, b, 3, patternSelectionItem)
	selected, _ := getOut(comMethod(item, 6), item)
	if int32(selected) != 1 {
		t.Errorf("IsSelected of the chosen item = %d, want 1", int32(selected))
	}
	container, hr := getOut(comMethod(item, 7), item)
	if hr != hrOK || container == 0 || coreOf(container).id != 2 {
		t.Fatalf("SelectionContainer = %#x (HRESULT 0x%08X), want node 2", container, uint32(hr))
	}
	releaseCall(container)
	if hr, _, _ := syscall.SyscallN(comMethod(item, 5), item); uint32(hr) != hrInvalidOperation {
		t.Errorf("RemoveFromSelection HRESULT 0x%08X, want UIA_E_INVALIDOPERATION", uint32(hr))
	}

	sel := patternOn(t, b, 2, patternSelection)
	psa, hr := getOut(comMethod(sel, 3), sel)
	if hr != hrOK || psa == 0 {
		t.Fatalf("GetSelection HRESULT 0x%08X, array %#x", uint32(hr), psa)
	}
	if ub := win32.SafeArrayUBound(psa); ub != 0 {
		t.Fatalf("GetSelection has %d items, want 1", ub+1)
	}
	var first uintptr
	win32.SafeArrayGetUnknown(psa, 0, &first)
	if first == 0 || coreOf(first).id != 3 {
		t.Errorf("the selected item is not node 3")
	}
	releaseCall(first)
	win32.SafeArrayDestroy(psa)
	if multi, _ := getOut(comMethod(sel, 4), sel); multi != 0 {
		t.Error("CanSelectMultiple is true for a single-selection container")
	}
	if len(got) != 0 {
		t.Errorf("reading the patterns made requests: %v", got)
	}
}

func TestValuePatternSetsAndReadsTheText(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleTextField, Parent: access.RootID, Value: "draft"}
	b := testTree(0, node(access.RootID, 0, 2), n)
	var got access.Request
	b.SetHooks(nil, func(id access.ID, r access.Request) { got = r })
	p := patternOn(t, b, 2, patternValue)

	bstr, hr := getOut(comMethod(p, 4), p)
	if hr != hrOK || bstrString(bstr) != "draft" {
		t.Errorf("Value = %q (HRESULT 0x%08X), want draft", bstrString(bstr), uint32(hr))
	}
	win32.SysFreeString(bstr)
	if ro, _ := getOut(comMethod(p, 5), p); int32(ro) != 0 {
		t.Error("IsReadOnly is true for an editable field")
	}

	text := append(utf16.Encode([]rune("café")), 0)
	hr2, _, _ := syscall.SyscallN(comMethod(p, 3), p, uintptr(unsafe.Pointer(&text[0])))
	runtime.KeepAlive(text)
	if uint32(hr2) != hrOK || got.Action != access.ActionSetValue || got.Value != "café" {
		t.Errorf("SetValue requested %+v (HRESULT 0x%08X), want the text café", got, uint32(hr2))
	}
}

// TestComboBoxHasTheValuePattern checks that a combo box has the Value pattern,
// which reports the label it has chosen, and not the Text pattern, which is for
// text fields and text areas.
func TestComboBoxHasTheValuePattern(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleComboBox, Parent: access.RootID, Value: "Louisiana",
		States: access.StateExpandable}
	b := testTree(0, node(access.RootID, 0, 2), n)
	p := patternOn(t, b, 2, patternValue)

	bstr, hr := getOut(comMethod(p, 4), p)
	if hr != hrOK || bstrString(bstr) != "Louisiana" {
		t.Errorf("Value = %q (HRESULT 0x%08X), want Louisiana", bstrString(bstr), uint32(hr))
	}
	win32.SysFreeString(bstr)
	if ro, _ := getOut(comMethod(p, 5), p); int32(ro) != 0 {
		t.Error("IsReadOnly is true for an editable combo box")
	}

	testHwnd++
	v := create(kindNode, 2, b, testHwnd, iidRawElementProviderSimple)
	defer releaseCall(v)
	if got, hr := getOut(comMethod(v, 4), v, patternText); hr != hrOK || got != 0 {
		t.Errorf("a combo box answered the text pattern: %x, HRESULT 0x%08X", got, uint32(hr))
	}
}

// TestRangePatternNeedsARange checks that only a node with a Range has the
// RangeValue pattern, and that a slider with one has it.
func TestRangePatternNeedsARange(t *testing.T) {
	slider := &access.Node{ID: 2, Role: access.RoleSlider, Parent: access.RootID, Range: &access.Range{Max: 10}}
	spinner := &access.Node{ID: 3, Role: access.RoleSpinner, Parent: access.RootID}
	if !supports(kindRange, slider) {
		t.Error("a slider with a Range does not support RangeValue")
	}
	if supports(kindRange, spinner) {
		t.Error("a spin box without a Range supports RangeValue")
	}
	b := testTree(0, node(access.RootID, 0, 2, 3), slider, spinner)
	patternOn(t, b, 2, patternRangeValue)
}

// TestRangeValuePatternReadsAndSetsTheValue checks the getters of the RangeValue
// pattern, and that SetValue requests the new value. SetValue takes a double,
// which the vtable reaches through the trampoline, so the test calls the method
// that the trampoline jumps to with the double's bits (see uiaSetRangeValue).
func TestRangeValuePatternReadsAndSetsTheValue(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleSlider, Parent: access.RootID,
		Range: &access.Range{Value: 30, Min: 5, Max: 100, SmallChange: 1, LargeChange: 10}}
	b := testTree(0, node(access.RootID, 0, 2), n)
	var got []access.Request
	b.SetHooks(nil, func(id access.ID, r access.Request) { got = append(got, r) })
	p := patternOn(t, b, 2, patternRangeValue)

	double := func(index int) float64 {
		out := newDouble()
		syscall.SyscallN(comMethod(p, index), p, uintptr(unsafe.Pointer(out)))
		runtime.KeepAlive(out)
		return *out
	}
	cases := []struct {
		name  string
		index int
		want  float64
	}{
		{"Value", 4, 30},
		{"Maximum", 6, 100},
		{"Minimum", 7, 5},
		{"LargeChange", 8, 10},
		{"SmallChange", 9, 1},
	}
	for _, c := range cases {
		if v := double(c.index); v != c.want {
			t.Errorf("%s = %v, want %v", c.name, v, c.want)
		}
	}
	if ro, _ := getOut(comMethod(p, 5), p); int32(ro) != 0 {
		t.Error("IsReadOnly is true for a slider without the read-only state")
	}

	if hr := uiaSetRangeValue(p, uintptr(math.Float64bits(42.5))); uint32(hr) != hrOK {
		t.Fatalf("SetValue HRESULT 0x%08X", uint32(hr))
	}
	last := got[len(got)-1]
	if last.Action != access.ActionSetRangeValue || last.X != 42.5 {
		t.Errorf("SetValue requested %+v, want 42.5", last)
	}
}

// TestRangePropertiesOfASlider checks the VARIANTs of the RangeValue properties,
// and that a node without a Range has none of them.
func TestRangePropertiesOfASlider(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleSlider, States: access.StateReadOnly,
		Range: &access.Range{Value: 30, Min: 5, Max: 100, SmallChange: 1, LargeChange: 10}}
	cases := map[int32]float64{propRangeValue: 30, propRangeMinimum: 5, propRangeMaximum: 100,
		propRangeLargeChange: 10, propRangeSmallChange: 1}
	for id, want := range cases {
		if v := nodeProperty(id, n, false); v.vt != vtR8 || math.Float64frombits(v.data[0]) != want {
			t.Errorf("property %d has tag %d and value %v, want %v", id, v.vt, math.Float64frombits(v.data[0]), want)
		}
	}
	if v := nodeProperty(propRangeIsReadOnly, n, false); v.vt != vtBool || variantBoolOf(v) != -1 {
		t.Error("RangeValue IsReadOnly is not VARIANT_TRUE for a read-only slider")
	}
	if v := nodeProperty(propIsRangeValueAvail, n, false); variantBoolOf(v) != -1 {
		t.Error("IsRangeValuePatternAvailable is not VARIANT_TRUE for a slider with a Range")
	}
	button := &access.Node{ID: 3, Role: access.RoleButton}
	if v := nodeProperty(propRangeValue, button, false); v.vt != vtEmpty {
		t.Errorf("RangeValue of a button has tag %d, want VT_EMPTY", v.vt)
	}
	if v := nodeProperty(propIsRangeValueAvail, button, false); variantBoolOf(v) != 0 {
		t.Error("IsRangeValuePatternAvailable is true for a button")
	}
}

func TestWideStringReadsToTheTerminator(t *testing.T) {
	s := append(utf16.Encode([]rune("Save")), 0, 'x')
	if got := wideString(uintptr(unsafe.Pointer(&s[0]))); got != "Save" {
		t.Errorf("wideString = %q, want Save", got)
	}
	if got := wideString(0); got != "" {
		t.Errorf("wideString(null) = %q, want empty", got)
	}
	runtime.KeepAlive(s)
}

// TestScrollPatternReadsAndScrolls checks the Scroll pattern's getters and the
// requests of its methods. SetScrollPercent takes doubles, which the vtable
// reaches through the trampoline, so the test calls the callback it jumps to with
// the doubles' bits.
func TestScrollPatternReadsAndScrolls(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleScrollArea, Parent: access.RootID,
		Scroll: access.Scroll{Container: true, CanY: true, Y: 40, ViewY: 25, X: 0, ViewX: 100}}
	b := testTree(0, node(access.RootID, 0, 2), n)
	var got []access.Request
	b.SetHooks(nil, func(id access.ID, r access.Request) { got = append(got, r) })
	p := patternOn(t, b, 2, patternScroll)

	double := func(index int) float64 {
		out := newDouble()
		syscall.SyscallN(comMethod(p, index), p, uintptr(unsafe.Pointer(out)))
		runtime.KeepAlive(out)
		return *out
	}
	if v := double(5); v != -1 {
		t.Errorf("HorizontalScrollPercent of a container that scrolls only vertically = %v, want -1", v)
	}
	if v := double(6); v != 40 {
		t.Errorf("VerticalScrollPercent = %v, want 40", v)
	}
	if v := double(7); v != 100 {
		t.Errorf("HorizontalViewSize = %v, want 100", v)
	}
	if v := double(8); v != 25 {
		t.Errorf("VerticalViewSize = %v, want 25", v)
	}
	if v, _ := getOut(comMethod(p, 9), p); int32(v) != 0 {
		t.Error("HorizontallyScrollable is true for a container that does not scroll horizontally")
	}
	if v, _ := getOut(comMethod(p, 10), p); int32(v) != 1 {
		t.Error("VerticallyScrollable is false for a container that scrolls vertically")
	}

	if hr := uiaSetScrollPercent(p, uintptr(math.Float64bits(25)), uintptr(math.Float64bits(-1))); uint32(hr) != hrOK {
		t.Fatalf("SetScrollPercent HRESULT 0x%08X", uint32(hr))
	}
	last := got[len(got)-1]
	if last.Action != access.ActionSetScrollPercent || last.X != 25 || last.Y != -1 {
		t.Errorf("SetScrollPercent requested %+v, want 25 and -1", last)
	}
}

func TestScrollAmountsBecomeFractionsOfTheView(t *testing.T) {
	cases := map[int32]float64{scrollLargeDecrement: -0.9, scrollSmallDecrement: -0.1, 2: 0,
		scrollLargeIncrement: 0.9, scrollSmallIncrement: 0.1}
	for amount, want := range cases {
		if got := scrollFraction(amount); got != want {
			t.Errorf("scrollFraction(%d) = %v, want %v", amount, got, want)
		}
	}
}

func TestScrollItemPatternScrollsIntoView(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleListItem, Parent: access.RootID,
		Actions: []access.Action{access.ActionScrollIntoView}}
	b := testTree(0, node(access.RootID, 0, 2), n)
	var got []access.Action
	b.SetHooks(nil, func(id access.ID, r access.Request) { got = append(got, r.Action) })
	p := patternOn(t, b, 2, patternScrollItem)
	syscall.SyscallN(comMethod(p, 3), p)
	if len(got) != 1 || got[0] != access.ActionScrollIntoView {
		t.Errorf("ScrollIntoView requested %v", got)
	}
}

// TestRelationPropertiesAreProviders checks LabeledBy, which is one provider, and
// DescribedBy, which is an array of them, and that a node with no description
// has neither.
func TestRelationPropertiesAreProviders(t *testing.T) {
	field := &access.Node{ID: 2, Role: access.RoleTextField, Parent: access.RootID,
		LabelledBy: []access.ID{3}, DescribedBy: []access.ID{3, 4}}
	b := testTree(0, node(access.RootID, 0, 2, 3, 4), field, node(3, access.RootID), node(4, access.RootID))
	testHwnd++
	c := &core{kind: kindNode, id: 2, hwnd: testHwnd, bridge: b}

	labeled, hr := c.property(propLabeledBy)
	if hr != hrOK || labeled.vt != vtUnknown {
		t.Fatalf("LabeledBy tag %d (HRESULT 0x%08X), want VT_UNKNOWN", labeled.vt, uint32(hr))
	}
	if p := variantUnknownValue(labeled); p == 0 || coreOf(p).id != 3 {
		t.Errorf("LabeledBy is not node 3")
	} else {
		releaseCall(p)
	}

	described, hr := c.property(propDescribedBy)
	if hr != hrOK || described.vt != vtUnknown|vtArray {
		t.Fatalf("DescribedBy tag %#x (HRESULT 0x%08X), want VT_UNKNOWN|VT_ARRAY", described.vt, uint32(hr))
	}
	psa := variantUnknownValue(described)
	if ub := win32.SafeArrayUBound(psa); ub != 1 {
		t.Errorf("DescribedBy has %d elements, want 2", ub+1)
	}
	win32.SafeArrayDestroy(psa)

	testHwnd++
	plain := &core{kind: kindNode, id: 3, hwnd: testHwnd, bridge: b}
	none, hr := plain.property(propDescribedBy)
	if hr != hrOK || none.vt != vtEmpty {
		t.Errorf("DescribedBy of a node with no description has tag %d, want VT_EMPTY", none.vt)
	}
}

// variantUnknownValue returns the pointer that a VT_UNKNOWN or array VARIANT holds.
func variantUnknownValue(v variant) uintptr { return uintptr(v.data[0]) }

// TestStatePropertiesOfTheNewPatterns checks the VARIANTs of the properties that
// clients read without a pattern, and the IsXPatternAvailable properties.
func TestStatePropertiesOfTheNewPatterns(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleSwitch, Parent: access.RootID,
		States: access.StateToggle | access.StateChecked | access.StateExpandable | access.StateExpanded |
			access.StateSelectable | access.StateSelected}
	if v := nodeProperty(propToggleState, n, false); v.vt != vtI4 || variantI4Of(v) != toggleStateOn {
		t.Errorf("ToggleState = %d, want on", variantI4Of(v))
	}
	if v := nodeProperty(propExpandCollapseState, n, false); variantI4Of(v) != expandCollapseExpanded {
		t.Errorf("ExpandCollapseState = %d, want expanded", variantI4Of(v))
	}
	if v := nodeProperty(propIsSelected, n, false); variantBoolOf(v) != -1 {
		t.Error("IsSelected is not VARIANT_TRUE")
	}
	if v := nodeProperty(propIsToggleAvailable, n, false); variantBoolOf(v) != -1 {
		t.Error("IsTogglePatternAvailable is not VARIANT_TRUE for a toggle")
	}
	if v := nodeProperty(propIsScrollAvailable, n, false); variantBoolOf(v) != 0 {
		t.Error("IsScrollPatternAvailable is true for a node that does not scroll")
	}
	heading := &access.Node{ID: 3, Role: access.RoleHeading, Heading: 2}
	if v := nodeProperty(propHeadingLevel, heading, false); variantI4Of(v) != headingLevel1+1 {
		t.Errorf("HeadingLevel of a level-2 heading = %d, want HeadingLevel2 (%d)", variantI4Of(v), headingLevel1+1)
	}
	field := &access.Node{ID: 4, Role: access.RoleTextArea, Value: "body", States: access.StateReadOnly}
	v := nodeProperty(propValue, field, false)
	if got := bstrString(variantBSTROf(v)); got != "body" {
		t.Errorf("Value = %q, want body", got)
	}
	win32.SysFreeString(variantBSTROf(v))
	if ro := nodeProperty(propIsReadOnly, field, false); variantBoolOf(ro) != -1 {
		t.Error("IsReadOnly is not VARIANT_TRUE for a read-only field")
	}
}

// TestNotificationProcessingForPoliteness checks the mapping that the design
// records: polite announcements are queued, and assertive ones interrupt.
func TestNotificationProcessingForPoliteness(t *testing.T) {
	if got := notificationProcessingOf(access.Polite); got != notificationAll {
		t.Errorf("polite = %d, want All (%d)", got, notificationAll)
	}
	if got := notificationProcessingOf(access.Assertive); got != notificationImportantMostRecent {
		t.Errorf("assertive = %d, want ImportantMostRecent (%d)", got, notificationImportantMostRecent)
	}
}

// TestEventsWithoutAClientDropObjects checks that a snapshot with changes,
// opened and closed popups and announcements is handled when no client listens:
// nothing is raised, and the objects of the removed nodes are dropped.
func TestEventsWithoutAClientDropObjects(t *testing.T) {
	const hwnd = 0x7104
	b := testTree(0, node(access.RootID, 0, 2), node(2, access.RootID))
	w := &window{hwnd: hwnd, uiaRoot: rootView(hwnd, b, iidRawElementProviderSimple)}
	defer releaseCall(w.uiaRoot)
	releaseCall(cachedObject(kindNode, 9, b, hwnd, iidRawElementProviderSimple))
	if n := cachedObjectCount(hwnd, 9); n != 1 {
		t.Fatalf("node 9 has %d registered objects before the snapshot, want 1", n)
	}
	w.onAccess(b, access.Update{
		Removed:       []access.ID{9},
		Properties:    []access.PropertyChange{{ID: 2, Property: access.PropertyName}},
		Opened:        []access.Popup{{ID: 2, Role: access.RoleDialog}},
		Closed:        []access.Popup{{ID: 5, Role: access.RoleMenu}},
		Announcements: []access.Announcement{{Text: "Saved", Politeness: access.Polite}},
	})
	if n := cachedObjectCount(hwnd, 9); n != 0 {
		t.Errorf("the object of removed node 9 is still registered")
	}
}

// cachedObjectCount returns how many registry objects of node id a window has.
func cachedObjectCount(hwnd uintptr, id access.ID) int {
	regMu.Lock()
	defer regMu.Unlock()
	n := 0
	for _, kind := range objKinds {
		if _, ok := objs[objKey{hwnd, kind, id}]; ok {
			n++
		}
	}
	return n
}

// TestPropertyIDsOfTheAccessProperties checks that each access property maps to
// the UI Automation property that a client watches for it.
func TestPropertyIDsOfTheAccessProperties(t *testing.T) {
	cases := map[access.Property]int32{
		access.PropertyName:       propName,
		access.PropertyValue:      propValue,
		access.PropertyToggle:     propToggleState,
		access.PropertyExpanded:   propExpandCollapseState,
		access.PropertySelected:   propIsSelected,
		access.PropertyEnabled:    propIsEnabled,
		access.PropertyRangeValue: propRangeValue,
	}
	for prop, want := range cases {
		if got := propertyID(prop); got != want {
			t.Errorf("propertyID(%d) = %d, want %d", prop, got, want)
		}
	}
}

// TestWindowPatternIsForDialogs checks that a dialog has the Window pattern, which
// reports it as modal, and that other nodes do not.
func TestWindowPatternIsForDialogs(t *testing.T) {
	dialog := &access.Node{ID: 2, Role: access.RoleDialog, Name: "Confirm", Parent: access.RootID}
	button := &access.Node{ID: 3, Role: access.RoleButton, Name: "OK", Parent: access.RootID}
	b := testTree(0, node(access.RootID, 0, 2, 3), dialog, button)
	testHwnd++
	dv := create(kindNode, 2, b, testHwnd, iidRawElementProviderSimple)
	defer releaseCall(dv)
	bv := create(kindNode, 3, b, testHwnd, iidRawElementProviderSimple)
	defer releaseCall(bv)
	if w, _ := getOut(comMethod(dv, 4), dv, patternWindow); w == 0 {
		t.Fatal("a dialog has no Window pattern")
	} else {
		modal, hr := getOut(comMethod(w, 8), w)
		if hr != hrOK || int32(modal) != 1 {
			t.Errorf("IsModal of a dialog = %d (HRESULT 0x%08X), want true", int32(modal), uint32(hr))
		}
		releaseCall(w)
	}
	if w, _ := getOut(comMethod(bv, 4), bv, patternWindow); w != 0 {
		t.Error("a button has the Window pattern")
	}
}
