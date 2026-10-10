//go:build windows

package windows

import (
	"sync"
	"unicode/utf16"
	"unsafe"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/win32"
	"github.com/tyler-builds/jamb/platform"
)

// The constants below are from UIAutomationClient.h and UIAutomationCoreApi.h
// in the Windows 10 SDK (10.0.26100.0).
const (
	// Control types.
	ctButton      = 50000
	ctCheckBox    = 50002
	ctEdit        = 50004
	ctHyperlink   = 50005
	ctImage       = 50006
	ctListItem    = 50007
	ctList        = 50008
	ctMenu        = 50009
	ctMenuBar     = 50010
	ctMenuItem    = 50011
	ctRadioButton = 50013
	ctTab         = 50018
	ctTabItem     = 50019
	ctText        = 50020
	ctToolTip     = 50022
	ctCustom      = 50025
	ctGroup       = 50026
	ctDocument    = 50030
	ctWindow      = 50032
	ctPane        = 50033
	ctSeparator   = 50038

	// Properties.
	propControlType          = 30003
	propLocalizedControlType = 30004
	propName                 = 30005
	propAcceleratorKey       = 30006
	propAccessKey            = 30007
	propHasKeyboardFocus     = 30008
	propIsKeyboardFocusable  = 30009
	propIsEnabled            = 30010
	propAutomationID         = 30011
	propClassName            = 30012
	propHelpText             = 30013
	propIsControlElement     = 30016
	propIsContentElement     = 30017
	propIsPassword           = 30019
	propIsOffscreen          = 30022
	propOrientation          = 30023
	propPositionInSet        = 30152
	propSizeOfSet            = 30153
	propLevel                = 30154
	propIsDialog             = 30174

	// Patterns and events.
	patternInvoke               = 10000
	eventStructureChanged       = 20002
	eventAutomationFocusChanged = 20005

	// ProviderOptions.
	providerClientSide             = 0x1
	providerServerSide             = 0x2
	providerNonClientArea          = 0x4
	providerOverride               = 0x8
	providerOwnsSetFocus           = 0x10
	providerUseComThreading        = 0x20
	providerRefuseNonClientSupport = 0x40
	providerHasNativeIAccessible   = 0x80
	providerUseClientCoordinates   = 0x100

	// NavigateDirection.
	navParent          = 0
	navNextSibling     = 1
	navPreviousSibling = 2
	navFirstChild      = 3
	navLastChild       = 4

	// StructureChangeType_ChildrenInvalidated.
	structureChildrenInvalidated = 2

	uiaAppendRuntimeID = 3
	uiaRootObjectID    = -25 // the lParam of WM_GETOBJECT that asks for the root object

	// VARIANT types and values.
	vtEmpty    = 0
	vtI4       = 3
	vtBSTR     = 8
	vtBool     = 11
	variantOn  = -1 // VARIANT_TRUE
	variantOff = 0  // VARIANT_FALSE
)

// HRESULTs. A failure has the top bit set.
const (
	hrOK                  = 0
	hrNotImpl             = 0x80004001
	hrNoInterface         = 0x80004002
	hrPointer             = 0x80004003
	hrFail                = 0x80004005
	hrElementNotAvailable = 0x80040201 // UIA_E_ELEMENTNOTAVAILABLE
)

// controlType returns the UI Automation control type of a role.
//
//	role                      control type      LocalizedControlType
//	window                    Window            window
//	dialog                    Window            dialog (IsDialog is set)
//	group                     Group             group
//	button                    Button            button
//	check box, switch         CheckBox          check box, switch
//	radio button              RadioButton       radio button
//	text field, text area     Edit              text field, text area
//	static text, heading      Text              static text, heading
//	image                     Image             image
//	link                      Hyperlink         link
//	list, list item           List, ListItem    list, list item
//	menu bar, menu            MenuBar, Menu     menu bar, menu
//	menu item (all kinds)     MenuItem          menu item, menu item check, menu item radio
//	separator                 Separator         separator
//	tooltip                   ToolTip           tooltip
//	scroll area, tab panel    Pane              scroll area, tab panel
//	tab list, tab             Tab, TabItem      tab list, tab
//	unknown                   Pane              unknown
//
// A text area is an Edit, not a Document: a Document is for read-only rich
// text, and the ui runtime's text areas are editable. Unknown roles are Panes,
// which carry no pattern and so no behavior a client would expect.
func controlType(r access.Role) int32 {
	switch r {
	case access.RoleWindow, access.RoleDialog:
		return ctWindow
	case access.RoleGroup:
		return ctGroup
	case access.RoleButton:
		return ctButton
	case access.RoleCheckBox, access.RoleSwitch:
		return ctCheckBox
	case access.RoleRadioButton:
		return ctRadioButton
	case access.RoleTextField, access.RoleTextArea:
		return ctEdit
	case access.RoleStaticText, access.RoleHeading:
		return ctText
	case access.RoleImage:
		return ctImage
	case access.RoleLink:
		return ctHyperlink
	case access.RoleList:
		return ctList
	case access.RoleListItem:
		return ctListItem
	case access.RoleMenuBar:
		return ctMenuBar
	case access.RoleMenu:
		return ctMenu
	case access.RoleMenuItem, access.RoleMenuItemCheck, access.RoleMenuItemRadio:
		return ctMenuItem
	case access.RoleSeparator:
		return ctSeparator
	case access.RoleTooltip:
		return ctToolTip
	case access.RoleScrollArea, access.RoleTabPanel:
		return ctPane
	case access.RoleTabList:
		return ctTab
	case access.RoleTab:
		return ctTabItem
	}
	return ctPane
}

// variant is a VARIANT. The type tag is at offset 0 and the value at offset 8,
// so the struct is 24 bytes on amd64. The value is kept as words so the struct
// is 8-byte aligned.
type variant struct {
	vt   uint16
	_    [3]uint16
	data [2]uint64
}

func variantI4(n int32) variant {
	v := variant{vt: vtI4}
	*(*int32)(unsafe.Pointer(&v.data[0])) = n
	return v
}

func variantBool(b bool) variant {
	v := variant{vt: vtBool}
	if b {
		*(*int16)(unsafe.Pointer(&v.data[0])) = variantOn
	} else {
		*(*int16)(unsafe.Pointer(&v.data[0])) = variantOff
	}
	return v
}

// variantBSTR returns a VARIANT that holds a new BSTR. The VARIANT owns the
// BSTR once UI Automation has it, so it is freed by UI Automation.
func variantBSTR(s string) variant {
	v := variant{vt: vtBSTR}
	*(*uintptr)(unsafe.Pointer(&v.data[0])) = bstrOf(s)
	return v
}

// bstrOf returns a new BSTR holding s. The caller frees it with SysFreeString,
// unless it is given to UI Automation in a VARIANT.
func bstrOf(s string) uintptr {
	return win32.SysAllocStringLen(utf16.Encode([]rune(s)))
}

// nodeProperty returns the VARIANT of property id for node n. focused says
// whether n has keyboard focus. Properties the node does not have are VT_EMPTY.
func nodeProperty(id int32, n *access.Node, focused bool) variant {
	switch id {
	case propControlType:
		return variantI4(controlType(n.Role))
	case propLocalizedControlType:
		return variantBSTR(n.Role.String())
	case propName:
		return variantBSTR(n.Name)
	case propAutomationID:
		return variantBSTR(n.Key)
	case propHelpText:
		return variantBSTR(n.Description)
	case propIsEnabled:
		return variantBool(!n.States.Has(access.StateDisabled))
	case propHasKeyboardFocus:
		return variantBool(focused)
	case propIsKeyboardFocusable:
		return variantBool(n.States.Has(access.StateFocusable))
	case propIsOffscreen:
		return variantBool(n.States.Has(access.StateOffscreen))
	case propIsControlElement, propIsContentElement:
		return variantBool(true)
	case propPositionInSet:
		if n.PosInSet > 0 {
			return variantI4(int32(n.PosInSet))
		}
	case propSizeOfSet:
		if n.SetSize > 0 {
			return variantI4(int32(n.SetSize))
		}
	case propLevel:
		if n.Level > 0 {
			return variantI4(int32(n.Level))
		}
	case propIsDialog:
		if n.Role == access.RoleDialog {
			return variantBool(true)
		}
	}
	return variant{}
}

// runtimeIDOf returns the runtime ID of a node: UiaAppendRuntimeId, then the
// high and low 32 bits of the node ID.
func runtimeIDOf(id access.ID) [3]int32 {
	return [3]int32{uiaAppendRuntimeID, int32(uint32(id >> 32)), int32(uint32(id))}
}

// newRuntimeID returns a new SAFEARRAY of the runtime ID of id, which the
// caller owns, or 0 on failure.
func newRuntimeID(id access.ID) uintptr {
	vals := runtimeIDOf(id)
	psa := win32.SafeArrayCreateVector(vtI4, 0, uint32(len(vals)))
	if psa == 0 {
		return 0
	}
	for i, v := range vals {
		if win32.SafeArrayPutInt32(psa, int32(i), v) != hrOK {
			win32.SafeArrayDestroy(psa)
			return 0
		}
	}
	return psa
}

// screenRect converts a rectangle in client pixels to screen pixels, given the
// screen position of the client area's top-left corner. An empty rectangle is
// all zeros.
func screenRect(origin win32.POINT, r access.Rect) (x, y, w, h float64) {
	if r.Empty() {
		return 0, 0, 0, 0
	}
	return float64(origin.X) + float64(r.X), float64(origin.Y) + float64(r.Y), float64(r.W), float64(r.H)
}

// clientPoint converts a screen point to client pixels, given the screen
// position of the client area's top-left corner.
func clientPoint(origin win32.POINT, x, y float64) (float64, float64) {
	return x - float64(origin.X), y - float64(origin.Y)
}

// clientOrigin returns the screen position of the top-left corner of the client
// area of hwnd, in physical pixels. It is zero if hwnd is not a window.
// boundsMu guards boundsScratch, which clientOrigin and clientSize use for the
// structures the window calls fill in. A local would escape to the heap through
// the call, once per call.
var (
	boundsMu      sync.Mutex
	boundsScratch struct {
		pt win32.POINT
		r  win32.RECT
	}
)

func clientOrigin(hwnd uintptr) win32.POINT {
	boundsMu.Lock()
	defer boundsMu.Unlock()
	boundsScratch.pt = win32.POINT{}
	win32.ClientToScreen(hwnd, &boundsScratch.pt)
	return boundsScratch.pt
}

// clientSize returns the size of the client area of hwnd in physical pixels.
func clientSize(hwnd uintptr) (w, h int32) {
	boundsMu.Lock()
	defer boundsMu.Unlock()
	win32.GetClientRectTo(hwnd, &boundsScratch.r)
	r := boundsScratch.r
	return r.Right - r.Left, r.Bottom - r.Top
}

// navigateTarget returns the node that navigation in direction dir reaches from
// node id, or 0 for none. The walk over the children allocates nothing, since
// a client navigates on every node it visits.
func navigateTarget(t *access.Tree, id access.ID, dir int32) access.ID {
	switch dir {
	case navFirstChild:
		return edgeChild(t, t.Node(id), false)
	case navLastChild:
		return edgeChild(t, t.Node(id), true)
	case navParent, navNextSibling, navPreviousSibling:
		if id == access.RootID {
			return 0
		}
		parent := access.RootID
		if n := t.Node(id); n != nil && n.Parent != 0 {
			parent = n.Parent
		}
		if dir == navParent {
			if parent == access.RootID || t.Node(parent) != nil {
				return parent
			}
			return 0
		}
		return siblingOf(t, t.Node(parent), id, dir == navNextSibling)
	}
	return 0
}

// edgeChild returns the first (or last) child of n that is in t, or 0.
func edgeChild(t *access.Tree, n *access.Node, last bool) access.ID {
	if n == nil {
		return 0
	}
	if last {
		for i := len(n.Children) - 1; i >= 0; i-- {
			if t.Node(n.Children[i]) != nil {
				return n.Children[i]
			}
		}
		return 0
	}
	for _, c := range n.Children {
		if t.Node(c) != nil {
			return c
		}
	}
	return 0
}

// siblingOf returns the child of parent that follows (or precedes) id among the
// children that are in t, or 0. It is 0 when id is not one of them.
func siblingOf(t *access.Tree, parent *access.Node, id access.ID, next bool) access.ID {
	if parent == nil {
		return 0
	}
	var prev access.ID
	found := false
	for _, c := range parent.Children {
		if t.Node(c) == nil {
			continue
		}
		if found {
			return c // the one after id
		}
		if c == id {
			if !next {
				return prev
			}
			found = true
			continue
		}
		prev = c
	}
	return 0
}

// hitTestTree returns the deepest node under the client point (x, y) in the
// subtree of id, or 0. Children are searched in reverse paint order, so the one
// painted last, which is on top, wins. A node's descendants are searched even
// when they lie outside its bounds, as overlays do.
func hitTestTree(t *access.Tree, id access.ID, x, y float32) access.ID {
	n := t.Node(id)
	if n == nil {
		return 0
	}
	for i := len(n.Children) - 1; i >= 0; i-- {
		c := t.Node(n.Children[i])
		if c == nil {
			continue
		}
		if hit := hitTestTree(t, c.ID, x, y); hit != 0 {
			return hit
		}
		if c.Bounds.Contains(x, y) {
			return c.ID
		}
	}
	return 0
}

// hitTestPoint returns what the client point (x, y) hits in a client area of w
// by h pixels: 0 when the point is outside the area, access.RootID when no node
// is under it, and otherwise the deepest node under it.
func hitTestPoint(t *access.Tree, x, y float64, w, h int32) access.ID {
	if x < 0 || y < 0 || x >= float64(w) || y >= float64(h) {
		return 0
	}
	if id := hitTestTree(t, access.RootID, float32(x), float32(y)); id != 0 {
		return id
	}
	return access.RootID
}

// current returns the snapshot node that c stands for, and the snapshot. The
// root answers as the window: its node is the tree's root node, named by the
// window title when the node has no name of its own. ok is false when the node
// is no longer in the snapshot. The snapshot is read only; it is immutable.
func (c *core) current() (n *access.Node, t *access.Tree, ok bool) {
	t = c.bridge.Tree()
	if c.kind == kindRoot {
		return c.rootNode(t), t, true
	}
	n = t.Node(c.id)
	return n, t, n != nil
}

// rootNode returns the root's node for the snapshot t. It is made once for each
// snapshot, not for each call, because UI Automation asks for the root's
// properties and children very often. The window title is read at that time, so
// a title change is seen with the next snapshot.
func (c *core) rootNode(t *access.Tree) *access.Node {
	if s := c.root.Load(); s != nil && s.tree == t {
		return &s.node
	}
	s := &rootSnap{tree: t, node: access.Node{ID: access.RootID, Role: access.RoleWindow}}
	if src := t.Node(access.RootID); src != nil {
		s.node = *src
	}
	if s.node.Name == "" {
		s.node.Name = win32.GetWindowText(c.hwnd)
	}
	c.root.Store(s)
	return &s.node
}

// navigate returns a new reference to the fragment that dir reaches from c, or
// 0 for none.
func (c *core) navigate(dir int32) (uintptr, uintptr) {
	if _, _, ok := c.current(); !ok {
		return 0, hrElementNotAvailable
	}
	id := navigateTarget(c.bridge.Tree(), c.id, dir)
	if id == 0 {
		return 0, hrOK
	}
	return c.provide(id, iidRawElementProviderFragment), hrOK
}

// runtimeID returns a new SAFEARRAY that the caller owns. The root has no
// runtime ID and returns 0.
func (c *core) runtimeID() (uintptr, uintptr) {
	if c.kind == kindRoot {
		return 0, hrOK
	}
	if _, _, ok := c.current(); !ok {
		return 0, hrElementNotAvailable
	}
	psa := newRuntimeID(c.id)
	if psa == 0 {
		return 0, hrFail
	}
	return psa, hrOK
}

// boundingRect returns the screen rectangle of c in physical pixels. The root's
// is the client area.
func (c *core) boundingRect() (x, y, w, h float64, hr uintptr) {
	n, _, ok := c.current()
	if !ok {
		return 0, 0, 0, 0, hrElementNotAvailable
	}
	origin := clientOrigin(c.hwnd)
	if c.kind == kindRoot {
		cw, ch := clientSize(c.hwnd)
		x, y, w, h = screenRect(origin, access.Rect{W: float32(cw), H: float32(ch)})
		return x, y, w, h, hrOK
	}
	x, y, w, h = screenRect(origin, n.Bounds)
	return x, y, w, h, hrOK
}

// property returns the VARIANT of property id for c.
func (c *core) property(id int32) (variant, uintptr) {
	n, t, ok := c.current()
	if !ok {
		return variant{}, hrElementNotAvailable
	}
	return nodeProperty(id, n, t != nil && t.Focus == n.ID), hrOK
}

// pattern returns a new reference to the Invoke pattern of c when the node
// supports ActionClick, and 0 otherwise.
func (c *core) pattern(id int32) (uintptr, uintptr) {
	if c.kind != kindNode || id != patternInvoke {
		return 0, hrOK
	}
	n, _, ok := c.current()
	if !ok {
		return 0, hrElementNotAvailable
	}
	if !n.Has(access.ActionClick) {
		return 0, hrOK
	}
	return cachedObject(kindPattern, c.id, c.bridge, c.hwnd, iidInvokeProvider), hrOK
}

// setFocus asks the runtime to move keyboard focus to c. The root has nothing to
// focus and does nothing.
func (c *core) setFocus() uintptr {
	if c.kind == kindRoot {
		return hrOK
	}
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	c.bridge.Perform(c.id, access.ActionFocus)
	return hrOK
}

// invoke asks the runtime to click c. The runtime runs the click on the UI
// thread, so this returns at once.
func (c *core) invoke() uintptr {
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	c.bridge.Perform(c.id, access.ActionClick)
	return hrOK
}

// focused returns a new reference to the node with keyboard focus, or 0.
func (c *core) focused() (uintptr, uintptr) {
	t := c.bridge.Tree()
	if t == nil || t.Focus == 0 || t.Node(t.Focus) == nil {
		return 0, hrOK
	}
	return c.provide(t.Focus, iidRawElementProviderFragment), hrOK
}

// The window's accessibility. The window answers WM_GETOBJECT with its root
// object, and listens for the snapshots that its runtime publishes. Both run on
// the UI thread.

// answerAccess answers WM_GETOBJECT for UI Automation. It returns false, and
// the window procedure uses DefWindowProc, when the request is for another
// object, or when the handler has no access bridge yet.
func (w *window) answerAccess(wp, lp uintptr) (uintptr, bool) {
	if int32(lp) != uiaRootObjectID {
		return 0, false
	}
	ah, ok := w.handler.(platform.AccessHandler)
	if !ok {
		return 0, false
	}
	b := ah.AccessBridge()
	if b == nil {
		return 0, false
	}
	if w.uiaRoot == 0 {
		// The window keeps one reference to its root, released when the
		// window is destroyed. UI Automation takes its own reference when it
		// receives the provider.
		w.uiaRoot = rootView(w.hwnd, b, iidRawElementProviderSimple)
		w.uiaBridge = b
		b.Listen(func(u access.Update) { w.onAccess(b, u) })
		b.Activate()
	}
	return win32.UiaReturnRawElementProvider(w.hwnd, wp, lp, w.uiaRoot), true
}

// onAccess reports a published snapshot to UI Automation. It raises nothing when
// no client listens, and does nothing for announcements (see the design).
func (w *window) onAccess(b *access.Bridge, u access.Update) {
	for _, id := range u.Removed {
		dropObjects(w.hwnd, id)
	}
	if w.uiaRoot == 0 || !win32.UiaClientsAreListening() {
		return
	}
	if u.FocusChanged && u.Focus != 0 && b.Tree().Node(u.Focus) != nil {
		if p := create(kindNode, u.Focus, b, w.hwnd, iidRawElementProviderSimple); p != 0 {
			win32.UiaRaiseAutomationEvent(p, eventAutomationFocusChanged)
			release(p)
		}
	}
	if u.Structure {
		win32.UiaRaiseStructureChangedEvent(w.uiaRoot, structureChildrenInvalidated, 0, 0)
	}
}

// endAccess ends the window's accessibility when it is destroyed. It stops the
// listener and releases the window's reference to its root.
func (w *window) endAccess() {
	if w.uiaBridge != nil {
		w.uiaBridge.Listen(nil)
		w.uiaBridge = nil
	}
	if w.uiaRoot == 0 {
		return
	}
	win32.UiaReturnRawElementProvider(w.hwnd, 0, 0, 0)
	win32.UiaDisconnectProvider(w.uiaRoot)
	dropAllObjects(w.hwnd)
	forgetRoot(w.hwnd)
	forgetHost(w.hwnd)
	release(w.uiaRoot)
	w.uiaRoot = 0
}
