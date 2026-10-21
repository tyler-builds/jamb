//go:build windows

package windows

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/win32"
)

// testTree publishes a snapshot of nodes on a new bridge. The nodes must include
// the root, access.RootID, which lists the top-level nodes as its children.
func testTree(focus access.ID, nodes ...*access.Node) *access.Bridge {
	b := access.NewBridge()
	b.Publish(access.NewTree(focus, nodes), access.Update{})
	return b
}

// node returns a node with the given ID, parent and children.
func node(id, parent access.ID, children ...access.ID) *access.Node {
	return &access.Node{ID: id, Parent: parent, Children: children, Role: access.RoleGroup}
}

func variantI4Of(v variant) int32 { return *(*int32)(unsafe.Pointer(&v.data[0])) }

func variantBoolOf(v variant) int16 { return *(*int16)(unsafe.Pointer(&v.data[0])) }

func variantBSTROf(v variant) uintptr { return *(*uintptr)(unsafe.Pointer(&v.data[0])) }

// bstrString copies a BSTR into a string.
func bstrString(p uintptr) string {
	n := win32.SysStringLen(p)
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(ptr(p)), n))
}

func TestVariantLayout(t *testing.T) {
	if n := unsafe.Sizeof(variant{}); n != 24 {
		t.Fatalf("VARIANT is %d bytes, want 24", n)
	}
	if v := variantI4(-7); v.vt != vtI4 || variantI4Of(v) != -7 {
		t.Errorf("variantI4(-7) = tag %d value %d, want tag %d value -7", v.vt, variantI4Of(v), vtI4)
	}
	if v := variantBool(true); v.vt != vtBool || variantBoolOf(v) != -1 {
		t.Errorf("variantBool(true) = tag %d value %d, want tag %d value VARIANT_TRUE", v.vt, variantBoolOf(v), vtBool)
	}
	if v := variantBool(false); v.vt != vtBool || variantBoolOf(v) != 0 {
		t.Errorf("variantBool(false) = tag %d value %d, want tag %d value VARIANT_FALSE", v.vt, variantBoolOf(v), vtBool)
	}
	if v := (variant{}); v.vt != vtEmpty {
		t.Errorf("zero variant tag = %d, want VT_EMPTY", v.vt)
	}
}

func TestBSTRAllocAndFree(t *testing.T) {
	for _, s := range []string{"", "Save", "café"} {
		v := variantBSTR(s)
		if v.vt != vtBSTR {
			t.Fatalf("variantBSTR(%q) tag = %d, want VT_BSTR", s, v.vt)
		}
		p := variantBSTROf(v)
		if got := bstrString(p); got != s {
			t.Errorf("BSTR holds %q, want %q", got, s)
		}
		win32.SysFreeString(p)
	}
}

func TestRuntimeID(t *testing.T) {
	if got, want := runtimeIDOf(0x0000000200000003), [3]int32{3, 2, 3}; got != want {
		t.Errorf("runtimeIDOf = %v, want %v", got, want)
	}
	psa := newRuntimeID(0x0000000100000000 | 0x2A)
	if psa == 0 {
		t.Fatal("newRuntimeID returned 0")
	}
	defer win32.SafeArrayDestroy(psa)
	want := [3]int32{3, 1, 0x2A}
	for i := range want {
		var got int32
		if hr := win32.SafeArrayGetInt32(psa, int32(i), &got); hr != hrOK {
			t.Fatalf("SafeArrayGetInt32(%d) HRESULT 0x%08X", i, uint32(hr))
		}
		if got != want[i] {
			t.Errorf("runtime ID element %d = %d, want %d", i, got, want[i])
		}
	}
}

func TestScreenRect(t *testing.T) {
	origin := win32.POINT{X: 100, Y: 200}
	x, y, w, h := screenRect(origin, access.Rect{X: 10, Y: 20, W: 30, H: 40})
	if x != 110 || y != 220 || w != 30 || h != 40 {
		t.Errorf("screenRect = %v %v %v %v, want 110 220 30 40", x, y, w, h)
	}
	if x, y, w, h := screenRect(origin, access.Rect{X: 10, Y: 20}); x != 0 || y != 0 || w != 0 || h != 0 {
		t.Errorf("empty rectangle = %v %v %v %v, want zeros", x, y, w, h)
	}
	if cx, cy := clientPoint(origin, 150, 260); cx != 50 || cy != 60 {
		t.Errorf("clientPoint = %v %v, want 50 60", cx, cy)
	}
}

func TestControlTypeMapping(t *testing.T) {
	cases := []struct {
		role access.Role
		want int32
	}{
		{access.RoleWindow, ctWindow},
		{access.RoleDialog, ctWindow},
		{access.RoleAlertDialog, ctWindow},
		{access.RoleGroup, ctGroup},
		{access.RoleButton, ctButton},
		{access.RoleCheckBox, ctCheckBox},
		{access.RoleSwitch, ctCheckBox},
		{access.RoleRadioButton, ctRadioButton},
		{access.RoleTextField, ctEdit},
		{access.RoleTextArea, ctEdit},
		{access.RoleComboBox, ctComboBox},
		{access.RoleSlider, ctSlider},
		{access.RoleSpinner, ctSpinner},
		{access.RoleStaticText, ctText},
		{access.RoleHeading, ctText},
		{access.RoleImage, ctImage},
		{access.RoleLink, ctHyperlink},
		{access.RoleList, ctList},
		{access.RoleListItem, ctListItem},
		{access.RoleMenuBar, ctMenuBar},
		{access.RoleMenu, ctMenu},
		{access.RoleMenuItem, ctMenuItem},
		{access.RoleMenuItemCheck, ctMenuItem},
		{access.RoleMenuItemRadio, ctMenuItem},
		{access.RoleSeparator, ctSeparator},
		{access.RoleTooltip, ctToolTip},
		{access.RoleScrollArea, ctPane},
		{access.RoleTabList, ctTab},
		{access.RoleTab, ctTabItem},
		{access.RoleTabPanel, ctPane},
		{access.RoleStatusBar, 50017},
		{access.RoleToolBar, 50021},
		{access.RoleUnknown, ctPane},
	}
	for _, c := range cases {
		if got := controlType(c.role); got != c.want {
			t.Errorf("controlType(%v) = %d, want %d", c.role, got, c.want)
		}
	}
}

func TestNodeProperties(t *testing.T) {
	n := &access.Node{
		ID: 7, Role: access.RoleDialog, Name: "Confirm", Key: "confirm", Description: "Asks first",
		States: access.StateFocusable | access.StateDisabled, PosInSet: 3, SetSize: 10,
	}
	if v := nodeProperty(propControlType, n, false); v.vt != vtI4 || variantI4Of(v) != ctWindow {
		t.Errorf("ControlType = tag %d value %d, want Window", v.vt, variantI4Of(v))
	}
	if v := nodeProperty(propIsDialog, n, false); v.vt != vtBool || variantBoolOf(v) != -1 {
		t.Errorf("IsDialog for a dialog is not VARIANT_TRUE")
	}
	if v := nodeProperty(propIsEnabled, n, false); variantBoolOf(v) != 0 {
		t.Errorf("IsEnabled for a disabled node is not VARIANT_FALSE")
	}
	if v := nodeProperty(propHasKeyboardFocus, n, true); variantBoolOf(v) != -1 {
		t.Errorf("HasKeyboardFocus for the focused node is not VARIANT_TRUE")
	}
	if v := nodeProperty(propPositionInSet, n, false); v.vt != vtI4 || variantI4Of(v) != 3 {
		t.Errorf("PositionInSet = tag %d value %d, want 3", v.vt, variantI4Of(v))
	}
	if v := nodeProperty(propLevel, n, false); v.vt != vtEmpty {
		t.Errorf("Level for a node without a level has tag %d, want VT_EMPTY", v.vt)
	}
	if v := nodeProperty(propIsDialog, &access.Node{Role: access.RoleButton}, false); v.vt != vtEmpty {
		t.Errorf("IsDialog for a button has tag %d, want VT_EMPTY", v.vt)
	}
	if v := nodeProperty(propClassName, n, false); v.vt != vtEmpty {
		t.Errorf("ClassName has tag %d, want VT_EMPTY", v.vt)
	}
	name := nodeProperty(propName, n, false)
	if got := bstrString(variantBSTROf(name)); got != "Confirm" {
		t.Errorf("Name = %q, want %q", got, "Confirm")
	}
	win32.SysFreeString(variantBSTROf(name))
	lct := nodeProperty(propLocalizedControlType, n, false)
	if lct.vt != vtBSTR || bstrString(variantBSTROf(lct)) != "dialog" {
		t.Errorf("LocalizedControlType tag %d, want a BSTR holding %q", lct.vt, "dialog")
	}
	win32.SysFreeString(variantBSTROf(lct))
}

// A non-modal dialog is a dialog, but not a modal window: IsModal is
// VARIANT_FALSE, while IsDialog still reports it.
func TestNonModalDialogProperties(t *testing.T) {
	n := &access.Node{ID: 7, Role: access.RoleDialog, Name: "Properties", States: access.StateNonModal}
	if v := nodeProperty(propIsDialog, n, false); v.vt != vtBool || variantBoolOf(v) != -1 {
		t.Errorf("IsDialog for a non-modal dialog is not VARIANT_TRUE")
	}
	if v := nodeProperty(propWindowIsModal, n, false); v.vt != vtBool || variantBoolOf(v) != 0 {
		t.Errorf("Window.IsModal for a non-modal dialog is not VARIANT_FALSE")
	}
	if v := nodeProperty(propWindowIsModal, &access.Node{ID: 8, Role: access.RoleDialog}, false); variantBoolOf(v) != -1 {
		t.Errorf("Window.IsModal for a modal dialog is not VARIANT_TRUE")
	}
}

// An alert dialog is a Window (50032) that is a dialog and modal, with the
// Window pattern, as a dialog is, and its own localized control type.
func TestAlertDialogProperties(t *testing.T) {
	n := &access.Node{ID: 7, Role: access.RoleAlertDialog, Name: "Delete 3 files?"}
	if v := nodeProperty(propControlType, n, false); v.vt != vtI4 || variantI4Of(v) != 50032 {
		t.Errorf("ControlType = tag %d value %d, want Window (50032)", v.vt, variantI4Of(v))
	}
	if v := nodeProperty(propIsDialog, n, false); v.vt != vtBool || variantBoolOf(v) != -1 {
		t.Errorf("IsDialog for an alert dialog is not VARIANT_TRUE")
	}
	if v := nodeProperty(propWindowIsModal, n, false); v.vt != vtBool || variantBoolOf(v) != -1 {
		t.Errorf("Window.IsModal for an alert dialog is not VARIANT_TRUE")
	}
	if !supports(kindWindow, n) {
		t.Errorf("an alert dialog has no Window pattern")
	}
	lct := nodeProperty(propLocalizedControlType, n, false)
	if lct.vt != vtBSTR || bstrString(variantBSTROf(lct)) != "alert dialog" {
		t.Errorf("LocalizedControlType tag %d, want a BSTR holding %q", lct.vt, "alert dialog")
	}
	win32.SysFreeString(variantBSTROf(lct))
}

func TestNavigateTarget(t *testing.T) {
	// 1 (root): 2, 3, 6, and 7, which is not in the snapshot.
	// 2: 4, 5. 3 and 6 are top-level nodes; 6 has parent 0, which means the root.
	root := node(access.RootID, 0, 2, 3, 6, 7)
	tree := access.NewTree(0, []*access.Node{
		root,
		node(2, access.RootID, 4, 5),
		node(3, access.RootID),
		node(6, 0),
		node(4, 2),
		node(5, 2),
	})
	cases := []struct {
		name string
		id   access.ID
		dir  int32
		want access.ID
	}{
		{"root first child", access.RootID, navFirstChild, 2},
		{"root last child skips a missing node", access.RootID, navLastChild, 6},
		{"root has no parent", access.RootID, navParent, 0},
		{"root has no sibling", access.RootID, navNextSibling, 0},
		{"top-level node's parent is the root", 2, navParent, access.RootID},
		{"node with parent 0 is a child of the root", 6, navParent, access.RootID},
		{"next sibling", 2, navNextSibling, 3},
		{"previous sibling of the first child", 2, navPreviousSibling, 0},
		{"previous sibling", 3, navPreviousSibling, 2},
		{"last top-level node has no next sibling", 6, navNextSibling, 0},
		{"first child", 2, navFirstChild, 4},
		{"last child", 2, navLastChild, 5},
		{"leaf has no child", 4, navFirstChild, 0},
		{"child's parent", 4, navParent, 2},
		{"child's sibling", 4, navNextSibling, 5},
		{"node not in the snapshot", 99, navFirstChild, 0},
	}
	for _, c := range cases {
		if got := navigateTarget(tree, c.id, c.dir); got != c.want {
			t.Errorf("%s: navigateTarget(%d, %d) = %d, want %d", c.name, c.id, c.dir, got, c.want)
		}
	}
}

func TestHitTest(t *testing.T) {
	// 2 is painted first, 3 over it. 8 is a child of 2 that lies outside 2.
	tree := access.NewTree(0, []*access.Node{
		{ID: access.RootID, Children: []access.ID{2, 3}},
		{ID: 2, Parent: access.RootID, Children: []access.ID{4, 8}, Bounds: access.Rect{W: 100, H: 100}},
		{ID: 3, Parent: access.RootID, Bounds: access.Rect{X: 50, Y: 50, W: 100, H: 100}},
		{ID: 4, Parent: 2, Bounds: access.Rect{X: 10, Y: 10, W: 20, H: 20}},
		{ID: 8, Parent: 2, Bounds: access.Rect{X: 150, Y: 150, W: 50, H: 50}},
	})
	cases := []struct {
		name string
		x, y float64
		want access.ID
	}{
		{"deepest node", 15, 15, 4},
		{"topmost sibling", 60, 60, 3},
		{"parent only", 5, 5, 2},
		{"descendant outside its parent", 160, 160, 8},
		{"no node under the point", 200, 10, access.RootID},
		{"outside the window", 400, 10, 0},
		{"negative", -1, 10, 0},
	}
	for _, c := range cases {
		if got := hitTestPoint(tree, c.x, c.y, 300, 300); got != c.want {
			t.Errorf("%s: hitTestPoint(%v, %v) = %d, want %d", c.name, c.x, c.y, got, c.want)
		}
	}
}

// vtable calls on a view. Each returns the HRESULT.

// qiCall calls QueryInterface and stores the view in out. The view is written to
// the heap first: a callback can grow the goroutine stack, which would move a
// variable on it, so COM never gets a pointer to the stack.
func qiCall(v uintptr, iid guid, out *uintptr) uintptr {
	p := newWord()
	hr, _, _ := syscall.SyscallN(comMethod(v, 0), v, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	*out = *p
	return hr
}

// getOut calls a COM method that takes the view obj, the arguments and an out
// pointer, and returns what it wrote and the HRESULT. The out pointer is on the
// heap, for the reason given at qiCall.
func getOut(method, obj uintptr, args ...uintptr) (uintptr, uintptr) {
	p := newWord()
	hr, _, _ := syscall.SyscallN(method, append(append([]uintptr{obj}, args...), uintptr(unsafe.Pointer(p)))...)
	runtime.KeepAlive(p)
	return *p, hr
}

func addRefCall(v uintptr) uintptr {
	r, _, _ := syscall.SyscallN(comMethod(v, 1), v)
	return r
}

func releaseCall(v uintptr) uintptr {
	r, _, _ := syscall.SyscallN(comMethod(v, iUnknownRelease), v)
	return r
}

// propCall calls GetPropertyValue (index 5 of IRawElementProviderSimple).
func propCall(v uintptr, id int32) (variant, uintptr) {
	out := newVariant()
	hr, _, _ := syscall.SyscallN(comMethod(v, 5), v, uintptr(id), uintptr(unsafe.Pointer(out)))
	runtime.KeepAlive(out) // the pointer was given as a uintptr, which does not keep it alive
	return *out, hr
}

func TestQueryInterfaceAndRefcounts(t *testing.T) {
	b := testTree(0, node(access.RootID, 0, 2), node(2, access.RootID))
	before := registrySize()

	v := create(kindNode, 2, b, 0x7001, iidRawElementProviderSimple)
	if v == 0 {
		t.Fatal("create returned 0")
	}
	if got := registrySize(); got != before+1 {
		t.Fatalf("registry has %d objects after create, want %d", got, before+1)
	}

	var frag uintptr
	if hr := qiCall(v, iidRawElementProviderFragment, &frag); hr != hrOK || frag == 0 {
		t.Fatalf("QueryInterface(Fragment) = 0x%08X, view %#x", uint32(hr), frag)
	}
	if frag == v {
		t.Error("QueryInterface returned the same view for a different interface")
	}
	// v, frag and unk each hold a reference, so the count is 3. AddRef
	// reports the new count, 4.
	var unk uintptr
	if hr := qiCall(frag, iidIUnknown, &unk); hr != hrOK || unk == 0 {
		t.Fatalf("QueryInterface(IUnknown) = 0x%08X", uint32(hr))
	}
	if n := addRefCall(v); n != 4 {
		t.Errorf("count after three references and an AddRef = %d, want 4", n)
	}
	releaseCall(v)

	var none uintptr
	if hr := qiCall(v, iidInvokeProvider, &none); hr != hrNoInterface || none != 0 {
		t.Errorf("QueryInterface for an interface the node lacks = 0x%08X, view %#x; want E_NOINTERFACE and 0", uint32(hr), none)
	}

	if n := releaseCall(unk); n != 2 {
		t.Errorf("release of IUnknown view: count %d, want 2", n)
	}
	releaseCall(frag)
	if n := releaseCall(v); n != 0 {
		t.Errorf("last release returned %d, want 0", n)
	}
	if got := registrySize(); got != before {
		t.Errorf("registry has %d objects after the last release, want %d", got, before)
	}
}

func TestRootIsOnePerWindow(t *testing.T) {
	b := testTree(0, node(access.RootID, 0))
	const hwnd = 0x7002
	before := registrySize()

	a := rootView(hwnd, b, iidRawElementProviderSimple)
	again := rootView(hwnd, b, iidRawElementProviderSimple)
	if a == 0 || again != a {
		t.Fatalf("second rootView = %#x, want the first %#x", again, a)
	}
	if registrySize() != before+1 {
		t.Errorf("two rootView calls made %d objects, want 1", registrySize()-before)
	}
	releaseCall(a)
	releaseCall(again)
	if _, ok := roots[hwnd]; ok || registrySize() != before {
		t.Error("the root of a released window is still registered")
	}
}

func TestMissingNodeIsElementNotAvailable(t *testing.T) {
	b := testTree(0, node(access.RootID, 0, 2), node(2, access.RootID))
	v := create(kindNode, 9, b, 0x7003, iidRawElementProviderSimple) // 9 is not in the snapshot
	defer releaseCall(v)

	if _, hr := propCall(v, propName); hr != hrElementNotAvailable {
		t.Errorf("GetPropertyValue on a missing node = 0x%08X, want UIA_E_ELEMENTNOTAVAILABLE", uint32(hr))
	}
	if hr, _, _ := syscall.SyscallN(comMethod(v, 4), v, patternInvoke, uintptr(unsafe.Pointer(newWord()))); uint32(hr) != hrElementNotAvailable {
		t.Errorf("GetPatternProvider on a missing node = 0x%08X, want UIA_E_ELEMENTNOTAVAILABLE", uint32(hr))
	}
}

func TestNodePropertyThroughVtable(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleButton, Name: "Save", Key: "save", Parent: access.RootID}
	b := testTree(2, node(access.RootID, 0, 2), n)
	v := create(kindNode, 2, b, 0x7004, iidRawElementProviderSimple)
	defer releaseCall(v)

	name, hr := propCall(v, propName)
	if hr != hrOK || name.vt != vtBSTR {
		t.Fatalf("Name: HRESULT 0x%08X, tag %d", uint32(hr), name.vt)
	}
	if got := bstrString(variantBSTROf(name)); got != "Save" {
		t.Errorf("Name = %q, want %q", got, "Save")
	}
	win32.SysFreeString(variantBSTROf(name)) // the caller owns it once UI Automation has it

	if focus, _ := propCall(v, propHasKeyboardFocus); variantBoolOf(focus) != -1 {
		t.Error("HasKeyboardFocus for the focused node is not VARIANT_TRUE")
	}
	var opts int32
	if hr, _, _ := syscall.SyscallN(comMethod(v, 3), v, uintptr(unsafe.Pointer(&opts))); hr != hrOK || opts != providerServerSide {
		t.Errorf("ProviderOptions = %#x (HRESULT 0x%08X), want ServerSideProvider", opts, uint32(hr))
	}
}

func TestInvokePatternPerformsClick(t *testing.T) {
	n := &access.Node{ID: 2, Role: access.RoleButton, Actions: []access.Action{access.ActionClick}, Parent: access.RootID}
	plain := &access.Node{ID: 3, Role: access.RoleStaticText, Parent: access.RootID}
	b := testTree(0, node(access.RootID, 0, 2, 3), n, plain)
	var got []access.Action
	var ids []access.ID
	b.SetHooks(nil, func(id access.ID, r access.Request) {
		ids = append(ids, id)
		got = append(got, r.Action)
	})
	v := create(kindNode, 2, b, 0x7005, iidRawElementProviderSimple)
	defer releaseCall(v)

	pattern, _ := getOut(comMethod(v, 4), v, patternInvoke)
	if pattern == 0 {
		t.Fatal("GetPatternProvider(Invoke) on a clickable node returned NULL")
	}
	if hr, _, _ := syscall.SyscallN(comMethod(pattern, 3), pattern); hr != hrOK {
		t.Fatalf("Invoke HRESULT 0x%08X", uint32(hr))
	}
	if len(got) != 1 || got[0] != access.ActionClick || ids[0] != 2 {
		t.Errorf("Invoke performed %v on %v, want click on 2", got, ids)
	}
	releaseCall(pattern)

	pv := create(kindNode, 3, b, 0x7005, iidRawElementProviderSimple)
	defer releaseCall(pv)
	none, _ := getOut(comMethod(pv, 4), pv, patternInvoke)
	if none != 0 {
		t.Error("GetPatternProvider(Invoke) on a node without a click returned a pattern")
	}
}

func TestRootNavigatesAndReportsRuntimeIDs(t *testing.T) {
	b := testTree(0, node(access.RootID, 0, 2), &access.Node{ID: 2, Parent: access.RootID, Role: access.RoleButton})
	root := rootView(0x7006, b, iidRawElementProviderFragment)
	defer releaseCall(root)

	child, hr := getOut(comMethod(root, 3), root, navFirstChild)
	if hr != hrOK || child == 0 {
		t.Fatalf("Navigate(FirstChild) = 0x%08X, view %#x", uint32(hr), child)
	}
	defer releaseCall(child)

	if psa, hr := getOut(comMethod(root, 4), root); hr != hrOK || psa != 0 {
		t.Errorf("GetRuntimeId on the root = 0x%08X, array %#x; want NULL", uint32(hr), psa)
	}
	psa, hr := getOut(comMethod(child, 4), child)
	if hr != hrOK || psa == 0 {
		t.Fatalf("GetRuntimeId on a node = 0x%08X, array %#x", uint32(hr), psa)
	}
	var first, last int32
	win32.SafeArrayGetInt32(psa, 0, &first)
	win32.SafeArrayGetInt32(psa, 2, &last)
	if first != 3 || last != 2 {
		t.Errorf("runtime ID of node 2 starts %d and ends %d, want 3 and 2", first, last)
	}
	win32.SafeArrayDestroy(psa)
}

func TestAnswerAccessGating(t *testing.T) {
	// WM_GETOBJECT for the root object asks for it; any other object id is not
	// the root.
	rootID := uintptr(0xFFFFFFE7)  // -25, UiaRootObjectId, in the low 32 bits
	otherID := uintptr(0xFFFFFFFC) // -4, OBJID_CLIENT

	plain := &window{handler: struct{ platform.Handler }{}}
	if _, ok := plain.answerAccess(0, rootID); ok {
		t.Error("answerAccess answered for a handler without an access bridge")
	}
	noBridge := &window{handler: accessHandler{}}
	if _, ok := noBridge.answerAccess(0, rootID); ok {
		t.Error("answerAccess answered before the runtime has a bridge")
	}
	w := &window{handler: accessHandler{b: access.NewBridge()}}
	if _, ok := w.answerAccess(0, otherID); ok {
		t.Error("answerAccess answered for an object that is not the root")
	}
	if w.uiaRoot != 0 || w.uiaBridge != nil {
		t.Error("a request for another object created a root or a listener")
	}
	w.endAccess() // nothing to end; must not panic
}

// accessHandler is a Handler that exposes an access bridge. Only AccessBridge
// is used by the tests; the embedded nil Handler is never called.
type accessHandler struct {
	platform.Handler
	b *access.Bridge
}

func (h accessHandler) AccessBridge() *access.Bridge { return h.b }

// A window's root is not shared with a bridge other than its own, and once the
// window is forgotten, a new window with the same handle gets a new root, while
// the old root lives on until it is released.
func TestRootIsNotSharedAcrossBridges(t *testing.T) {
	const hwnd = 0x7007
	a := rootView(hwnd, testTree(0, node(access.RootID, 0)), iidRawElementProviderSimple)
	b := rootView(hwnd, testTree(0, node(access.RootID, 0)), iidRawElementProviderSimple)
	if a == 0 || b == 0 || a == b {
		t.Fatalf("roots for two bridges are %#x and %#x; want two different roots", a, b)
	}
	releaseCall(a)
	releaseCall(b)

	c := rootView(hwnd, testTree(0, node(access.RootID, 0)), iidRawElementProviderSimple)
	forgetRoot(hwnd)
	d := rootView(hwnd, testTree(0, node(access.RootID, 0)), iidRawElementProviderSimple)
	if c == 0 || d == 0 || c == d {
		t.Fatalf("after forgetRoot the handle got %#x, the old root %#x; want a new root", d, c)
	}
	releaseCall(c) // must not unregister d
	if _, ok := roots[hwnd]; !ok || cores[roots[hwnd]] == nil {
		t.Error("releasing the forgotten root unregistered the new root")
	}
	releaseCall(d)
}

// TestNodeObjectsAreCachedAndDropped checks that a node is one object for all
// the queries that UI Automation makes, and that the registry lets go of it when
// the node leaves the tree: the object is freed once UI Automation has released
// its references too.
func TestNodeObjectsAreCachedAndDropped(t *testing.T) {
	count := func() int {
		regMu.Lock()
		defer regMu.Unlock()
		return len(cores)
	}
	b := testTree(0, node(access.RootID, 0, 2), node(2, access.RootID))
	base := count()

	v1 := cachedObject(kindNode, 2, b, 0x7f00, iidRawElementProviderSimple)
	v2 := cachedObject(kindNode, 2, b, 0x7f00, iidRawElementProviderSimple)
	if v1 == 0 || v1 != v2 {
		t.Fatalf("two queries for node 2 gave %#x and %#x; want the same object", v1, v2)
	}
	if n := count(); n != base+1 {
		t.Fatalf("registry holds %d objects, want %d", n-base, 1)
	}
	releaseCall(v1)
	releaseCall(v2)
	if n := count(); n != base+1 {
		t.Fatalf("the object was freed while the registry still holds it (%d new)", n-base)
	}
	dropObjects(0x7f00, 2)
	if n := count(); n != base {
		t.Fatalf("the object of a removed node was not freed (%d left)", n-base)
	}

	// The same node ID in another window is another object.
	w1 := cachedObject(kindNode, 2, b, 0x7f00, iidRawElementProviderSimple)
	w2 := cachedObject(kindNode, 2, b, 0x7f01, iidRawElementProviderSimple)
	if w1 == w2 {
		t.Fatal("two windows share the object of node 2")
	}
	releaseCall(w1)
	releaseCall(w2)
	dropObjects(0x7f00, 2)
	dropObjects(0x7f01, 2)
}
