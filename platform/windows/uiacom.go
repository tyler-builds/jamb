package windows

import (
	"math"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/win32"
)

// The UI Automation providers are COM objects that UI Automation calls from
// other threads, and holds past the call that created them. They are kept
// outside the Go heap, as the file dialogs are, and they are called through
// their vtables.
//
// An object is one or more views. A view is an interface pointer: two words in
// memory from the process heap, the address of the vtable and the handle of
// the object's core. The vtable is a Go array of callback addresses, which
// does not move. A core is the Go-side state of an object: its reference count,
// what it stands for, and its views. The cores are kept in a map keyed by
// handle, so no Go pointer is stored outside the Go heap. A core whose count
// reaches zero leaves the map, and its views are freed.
//
// Each vtable is filled once, because syscall.NewCallback has a process-wide
// limit. The vtable indices are counted from the declaration order in
// UIAutomationCore.idl, with IUnknown at 0 to 2.

const wordSize = unsafe.Sizeof(uintptr(0))

// Vtables. They are filled by initVtbls.
var (
	simpleVtbl     [7]uintptr  // IRawElementProviderSimple
	fragmentVtbl   [9]uintptr  // IRawElementProviderFragment
	rootVtbl       [5]uintptr  // IRawElementProviderFragmentRoot (its base is IUnknown)
	invokeVtbl     [4]uintptr  // IInvokeProvider
	windowVtbl     [12]uintptr // IWindowProvider
	toggleVtbl     [5]uintptr  // IToggleProvider
	expandVtbl     [6]uintptr  // IExpandCollapseProvider
	selItemVtbl    [8]uintptr  // ISelectionItemProvider
	selectionVtbl  [6]uintptr  // ISelectionProvider
	valueVtbl      [6]uintptr  // IValueProvider
	rangeVtbl      [10]uintptr // IRangeValueProvider
	scrollVtbl     [11]uintptr // IScrollProvider
	scrollItemVtbl [4]uintptr  // IScrollItemProvider
)

// iface is an interface an object implements, with its vtable.
type iface struct {
	iid  guid
	vtbl []uintptr
}

var (
	ifaceSimple       = iface{iidRawElementProviderSimple, simpleVtbl[:]}
	ifaceFragment     = iface{iidRawElementProviderFragment, fragmentVtbl[:]}
	ifaceFragmentRoot = iface{iidRawElementProviderFragmentRoot, rootVtbl[:]}
	ifaceInvoke       = iface{iidInvokeProvider, invokeVtbl[:]}
	ifaceWindow       = iface{iidWindowProvider, windowVtbl[:]}
	ifaceToggle       = iface{iidToggleProvider, toggleVtbl[:]}
	ifaceExpand       = iface{iidExpandCollapseProvider, expandVtbl[:]}
	ifaceSelectItem   = iface{iidSelectionItemProvider, selItemVtbl[:]}
	ifaceSelection    = iface{iidSelectionProvider, selectionVtbl[:]}
	ifaceValue        = iface{iidValueProvider, valueVtbl[:]}
	ifaceRange        = iface{iidRangeValueProvider, rangeVtbl[:]}
	ifaceScroll       = iface{iidScrollProvider, scrollVtbl[:]}
	ifaceScrollItem   = iface{iidScrollItemProvider, scrollItemVtbl[:]}
)

// The interface IIDs, from the uuid attributes in UIAutomationCore.idl. The
// test TestPatternIIDsMatchTheSDK checks them against the strings.
var (
	iidIUnknown                       = guid{0, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidRawElementProviderSimple       = guid{0xd6dd68d1, 0x86fd, 0x4332, [8]byte{0x86, 0x66, 0x9a, 0xbe, 0xde, 0xa2, 0xd2, 0x4c}}
	iidRawElementProviderFragment     = guid{0xf7063da8, 0x8359, 0x439c, [8]byte{0x92, 0x97, 0xbb, 0xc5, 0x29, 0x9a, 0x7d, 0x87}}
	iidRawElementProviderFragmentRoot = guid{0x620ce2a5, 0xab8f, 0x40a9, [8]byte{0x86, 0xcb, 0xde, 0x3c, 0x75, 0x59, 0x9b, 0x58}}
	iidInvokeProvider                 = guid{0x54fcb24b, 0xe18e, 0x47a2, [8]byte{0xb4, 0xd3, 0xec, 0xcb, 0xe7, 0x75, 0x99, 0xa2}}
	iidWindowProvider                 = guid{0x987df77b, 0xdb06, 0x4d77, [8]byte{0x8f, 0x8a, 0x86, 0xa9, 0xc3, 0xbb, 0x90, 0xb9}}
	iidToggleProvider                 = guid{0x56d00bd0, 0xc4f4, 0x433c, [8]byte{0xa8, 0x36, 0x1a, 0x52, 0xa5, 0x7e, 0x08, 0x92}}
	iidExpandCollapseProvider         = guid{0xd847d3a5, 0xcab0, 0x4a98, [8]byte{0x8c, 0x32, 0xec, 0xb4, 0x5c, 0x59, 0xad, 0x24}}
	iidSelectionItemProvider          = guid{0x2acad808, 0xb2d4, 0x452d, [8]byte{0xa4, 0x07, 0x91, 0xff, 0x1a, 0xd1, 0x67, 0xb2}}
	iidSelectionProvider              = guid{0xfb8b03af, 0x3bdf, 0x48d4, [8]byte{0xbd, 0x36, 0x1a, 0x65, 0x79, 0x3b, 0xe1, 0x68}}
	iidValueProvider                  = guid{0xc7935180, 0x6fb3, 0x4201, [8]byte{0xb1, 0x74, 0x7d, 0xf7, 0x3a, 0xdb, 0xf6, 0x4a}}
	iidRangeValueProvider             = guid{0x36dc7aef, 0x33e6, 0x4691, [8]byte{0xaf, 0xe1, 0x2b, 0xe7, 0x27, 0x4b, 0x3d, 0x33}}
	iidScrollProvider                 = guid{0xb38b8077, 0x1fc3, 0x42a5, [8]byte{0x8c, 0xae, 0xd4, 0x0c, 0x22, 0x15, 0x05, 0x5a}}
	iidScrollItemProvider             = guid{0x2360c714, 0x4bf1, 0x4b26, [8]byte{0xba, 0x65, 0x9b, 0x21, 0x31, 0x61, 0x27, 0xeb}}
)

// coreKind says what an object stands for. Each kind of pattern has its own
// object, so that a node with two patterns has two objects, one per pattern.
type coreKind uint8

const (
	kindRoot       coreKind = iota // the window's fragment root
	kindNode                       // a node of the access tree
	kindInvoke                     // the Invoke pattern of a node
	kindToggle                     // the Toggle pattern of a node
	kindExpand                     // the ExpandCollapse pattern of a node
	kindSelectItem                 // the SelectionItem pattern of a node
	kindSelection                  // the Selection pattern of a container
	kindValue                      // the Value pattern of a text field or combo box
	kindScroll                     // the Scroll pattern of a scroll container
	kindScrollItem                 // the ScrollItem pattern of a node
	kindWindow                     // the Window pattern of a dialog
	kindText                       // the Text pattern of a text field or text area
	kindRange                      // the RangeValue pattern of a slider or spin box
	kindTextRange                  // a range of the text of a text field; made per call, not kept by its node
)

// patternKinds lists the kinds of pattern objects. Like nodes, each pattern
// object is kept per node until the node leaves the tree.
var patternKinds = []coreKind{kindInvoke, kindToggle, kindExpand, kindSelectItem, kindSelection,
	kindValue, kindScroll, kindScrollItem, kindWindow, kindText, kindRange}

// kindIfaces lists the interfaces that each kind of object implements.
var kindIfaces = [...][]*iface{
	kindRoot:       {&ifaceSimple, &ifaceFragment, &ifaceFragmentRoot},
	kindNode:       {&ifaceSimple, &ifaceFragment},
	kindInvoke:     {&ifaceInvoke},
	kindToggle:     {&ifaceToggle},
	kindExpand:     {&ifaceExpand},
	kindSelectItem: {&ifaceSelectItem},
	kindSelection:  {&ifaceSelection},
	kindValue:      {&ifaceValue},
	kindRange:      {&ifaceRange},
	kindScroll:     {&ifaceScroll},
	kindScrollItem: {&ifaceScrollItem},
	kindWindow:     {&ifaceWindow},
	kindText:       {&ifaceText, &ifaceText2},
	kindTextRange:  {&ifaceTextRange},
}

// core is the Go-side state of an object. Its views share it.
type core struct {
	refs    atomic.Int32
	handle  uintptr
	kind    coreKind
	id      access.ID // the node, for node and pattern objects; access.RootID for the root
	hwnd    uintptr   // the window
	bridge  *access.Bridge
	views   []comView
	viewBuf [3]comView // the backing array of views, so that a core needs no second allocation
	// root is the window's node as of the snapshot it was made from, for the
	// root object only (see rootNode).
	root atomic.Pointer[rootSnap]
	// rng is the range of a text range object (see uiatext.go).
	rng access.TextRange // read and set under regMu
}

// rootSnap is the root node of one snapshot, with the title the window had when
// the snapshot was taken.
type rootSnap struct {
	tree *access.Tree
	node access.Node
}

// comView is one view of a core: the interface and the address of the view.
type comView struct {
	iid  guid
	addr uintptr
}

// The registry. Its lock also orders the reference count against the map: a
// count only reaches zero, and leaves the map, under the lock, so that a lookup
// under the lock never finds a core that is being freed.
var (
	regMu      sync.Mutex
	cores      = map[uintptr]*core{}   // by handle
	roots      = map[uintptr]uintptr{} // window handle to the handle of its root object
	lastHandle uintptr
)

var vtblOnce sync.Once

// initVtbls makes the callbacks and fills the vtables. It runs once.
func initVtbls() {
	vtblOnce.Do(func() {
		qi := syscall.NewCallback(uiaQueryInterface)
		ar := syscall.NewCallback(uiaAddRef)
		rl := syscall.NewCallback(uiaRelease)
		initTextVtbls(qi, ar, rl)
		simpleVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaProviderOptions),
			syscall.NewCallback(uiaGetPatternProvider),
			syscall.NewCallback(uiaGetPropertyValue),
			syscall.NewCallback(uiaHostProvider),
		}
		fragmentVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaNavigate),
			syscall.NewCallback(uiaRuntimeID),
			syscall.NewCallback(uiaBoundingRectangle),
			syscall.NewCallback(uiaEmbeddedFragmentRoots),
			syscall.NewCallback(uiaSetFocus),
			syscall.NewCallback(uiaFragmentRoot),
		}
		rootVtbl = [...]uintptr{qi, ar, rl,
			newElementFromPointSlot(),
			syscall.NewCallback(uiaGetFocus),
		}
		invokeVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaInvoke),
		}
		toggleVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaToggle),
			syscall.NewCallback(uiaToggleState),
		}
		expandVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaExpand),
			syscall.NewCallback(uiaCollapse),
			syscall.NewCallback(uiaExpandCollapseState),
		}
		selItemVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaSelect),
			syscall.NewCallback(uiaAddToSelection),
			syscall.NewCallback(uiaRemoveFromSelection),
			syscall.NewCallback(uiaIsSelected),
			syscall.NewCallback(uiaSelectionContainer),
		}
		selectionVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaGetSelection),
			syscall.NewCallback(uiaCanSelectMultiple),
			syscall.NewCallback(uiaIsSelectionRequired),
		}
		valueVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaSetValue),
			syscall.NewCallback(uiaValue),
			syscall.NewCallback(uiaIsReadOnly),
		}
		// IRangeValueProvider's SetValue takes a double, so it goes through a
		// trampoline (see uia_amd64.s). Its IsReadOnly is the Value pattern's.
		rangeVtbl = [...]uintptr{qi, ar, rl,
			newSetRangeValueSlot(),
			syscall.NewCallback(uiaRangeValue),
			syscall.NewCallback(uiaIsReadOnly),
			syscall.NewCallback(uiaRangeMaximum),
			syscall.NewCallback(uiaRangeMinimum),
			syscall.NewCallback(uiaRangeLargeChange),
			syscall.NewCallback(uiaRangeSmallChange),
		}
		// IScrollProvider's SetScrollPercent takes two doubles, so it goes through
		// the same trampoline as ElementProviderFromPoint (see uia_amd64.s).
		scrollVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaScroll),
			newSetScrollPercentSlot(),
			syscall.NewCallback(uiaHorizontalScrollPercent),
			syscall.NewCallback(uiaVerticalScrollPercent),
			syscall.NewCallback(uiaHorizontalViewSize),
			syscall.NewCallback(uiaVerticalViewSize),
			syscall.NewCallback(uiaHorizontallyScrollable),
			syscall.NewCallback(uiaVerticallyScrollable),
		}
		scrollItemVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaScrollIntoView),
		}
		windowVtbl = [...]uintptr{qi, ar, rl,
			syscall.NewCallback(uiaSetVisualState),
			syscall.NewCallback(uiaCloseWindow),
			syscall.NewCallback(uiaWaitForInputIdle),
			syscall.NewCallback(uiaCanMaximize),
			syscall.NewCallback(uiaCanMinimize),
			syscall.NewCallback(uiaIsModal),
			syscall.NewCallback(uiaWindowVisualState),
			syscall.NewCallback(uiaWindowInteractionState),
			syscall.NewCallback(uiaIsTopmost),
		}
	})
}

// create makes a new object of kind for node id (the root for access.RootID),
// and returns its view for iid. The view holds the one reference that the
// caller releases. It returns 0 if the kind has no such interface, or if memory
// runs out.
func create(kind coreKind, id access.ID, b *access.Bridge, hwnd uintptr, iid guid) uintptr {
	regMu.Lock()
	defer regMu.Unlock()
	return createLocked(kind, id, b, hwnd, iid)
}

// createLocked is create for a caller that holds regMu.
func createLocked(kind coreKind, id access.ID, b *access.Bridge, hwnd uintptr, iid guid) uintptr {
	initVtbls()
	ifs := kindIfaces[kind]
	if !hasIface(ifs, iid) {
		return 0
	}
	lastHandle++
	var c *core
	if n := len(freeCores); n > 0 {
		c = freeCores[n-1]
		freeCores = freeCores[:n-1]
		*c = core{}
	} else {
		c = new(core)
	}
	c.handle, c.kind, c.id, c.hwnd, c.bridge = lastHandle, kind, id, hwnd, b
	c.views = c.viewBuf[:0]
	c.refs.Store(1)
	for _, i := range ifs {
		addr := allocView(i.vtbl, c.handle)
		if addr == 0 {
			c.free()
			return 0
		}
		c.views = append(c.views, comView{i.iid, addr})
	}
	cores[c.handle] = c
	if kind == kindRoot {
		roots[hwnd] = c.handle
	}
	return c.viewFor(iid)
}

// rootView returns the view for iid of the root object of the window hwnd, with
// a new reference. The root is made if the window has none, so each window has
// one root object. A root that belongs to another bridge is not reused.
func rootView(hwnd uintptr, b *access.Bridge, iid guid) uintptr {
	regMu.Lock()
	defer regMu.Unlock()
	if c := cores[roots[hwnd]]; c != nil && c.bridge == b {
		v := c.viewFor(iid)
		if v != 0 {
			c.refs.Add(1)
		}
		return v
	}
	return createLocked(kindRoot, access.RootID, b, hwnd, iid)
}

// forgetRoot removes the root of the window hwnd from roots. The window is
// destroyed, but the root lives on while UI Automation holds it; a window made
// later with the same handle gets a root of its own.
func forgetRoot(hwnd uintptr) {
	regMu.Lock()
	defer regMu.Unlock()
	delete(roots, hwnd)
}

// hasIface reports whether one of ifs is the interface iid.
func hasIface(ifs []*iface, iid guid) bool {
	for _, i := range ifs {
		if i.iid == iid {
			return true
		}
	}
	return false
}

// allocView makes a view with the vtable vtbl for the object with the given
// handle. It returns 0 if memory runs out.
func allocView(vtbl []uintptr, handle uintptr) uintptr {
	p := win32.HeapAlloc(win32.GetProcessHeap(), win32.HEAP_ZERO_MEMORY, 2*wordSize)
	if p != 0 {
		*(*uintptr)(ptr(p)) = uintptr(unsafe.Pointer(&vtbl[0]))
		*(*uintptr)(ptr(p + wordSize)) = handle
	}
	return p
}

// viewFor returns the view of c for iid, or 0. IUnknown is answered by the
// first view, since every view of an object answers for the whole object.
func (c *core) viewFor(iid guid) uintptr {
	if len(c.views) == 0 {
		return 0
	}
	if iid == iidIUnknown {
		return c.views[0].addr
	}
	for _, v := range c.views {
		if v.iid == iid {
			return v.addr
		}
	}
	return 0
}

// free frees the views of c. The core must already have left the registry.
func (c *core) free() {
	heap := win32.GetProcessHeap()
	for _, v := range c.views {
		win32.HeapFree(heap, 0, v.addr)
	}
	c.views = nil
}

// coreOf returns the core of the object that view v belongs to, or nil. The
// caller holds a reference to the object, so the core stays alive after coreOf
// returns.
func coreOf(v uintptr) *core {
	h := *(*uintptr)(ptr(v + wordSize))
	regMu.Lock()
	defer regMu.Unlock()
	return cores[h]
}

// registrySize returns the number of live objects.
func registrySize() int {
	regMu.Lock()
	defer regMu.Unlock()
	return len(cores)
}

// enter finds the object for a method that fills an out parameter.
func enter(this, out uintptr) (*core, uintptr) {
	c := coreOf(this)
	switch {
	case c == nil:
		return nil, hrFail
	case out == 0:
		return nil, hrPointer
	}
	return c, hrOK
}

// putPtr stores v in the out parameter out, a pointer-sized result.
func putPtr(out, v uintptr) { *(*uintptr)(ptr(out)) = v }

// provide returns a new reference to the provider of node id for iid. The root
// is the window's root object.
func (c *core) provide(id access.ID, iid guid) uintptr {
	if id == access.RootID {
		return rootView(c.hwnd, c.bridge, iid)
	}
	return cachedObject(kindNode, id, c.bridge, c.hwnd, iid)
}

// freeCores holds the cores of freed objects, for createLocked to reuse, so that a
// range object makes no Go allocation in steady state. A reused core takes a new
// handle, so a view of the object it was before no longer finds it in cores.
var freeCores []*core

// maxFreeCores bounds freeCores.
const maxFreeCores = 256

// recycle keeps a core whose object was freed, once its views are freed.
func recycle(c *core) {
	regMu.Lock()
	if len(freeCores) < maxFreeCores {
		freeCores = append(freeCores, c)
	}
	regMu.Unlock()
}

// objs holds one object for each node and kind, so that a node is the same
// object each time UI Automation asks for it. UI Automation keeps the objects it
// has been given, so making a new object per query would grow the registry with
// every walk of the tree. The registry owns one reference to each object, which
// dropObjects releases when the node leaves the tree.
var objs = map[objKey]uintptr{} // to the handle of the object

// objKinds lists the kinds of object that a node has in objs.
var objKinds = append([]coreKind{kindNode}, patternKinds...)

type objKey struct {
	hwnd uintptr // the window, since node IDs are only unique within one window
	kind coreKind
	id   access.ID
}

// cachedObject returns a view of the object of kind for node id, with a new
// reference. The first call makes the object.
func cachedObject(kind coreKind, id access.ID, b *access.Bridge, hwnd uintptr, iid guid) uintptr {
	regMu.Lock()
	defer regMu.Unlock()
	k := objKey{hwnd, kind, id}
	if h, ok := objs[k]; ok {
		if c := cores[h]; c != nil {
			v := c.viewFor(iid)
			if v != 0 {
				c.refs.Add(1)
			}
			return v
		}
	}
	v := createLocked(kind, id, b, hwnd, iid)
	if v != 0 {
		c := cores[lastHandle] // the object createLocked just made
		c.refs.Add(1)          // the registry's reference
		objs[k] = c.handle
	}
	return v
}

// dropObjects releases the registry's references to the objects of node id. It
// runs when the node has left the tree.
func dropObjects(hwnd uintptr, id access.ID) {
	for _, kind := range objKinds {
		k := objKey{hwnd, kind, id}
		regMu.Lock()
		var c *core
		if h, ok := objs[k]; ok {
			c = cores[h]
			delete(objs, k)
		}
		regMu.Unlock()
		if c != nil {
			dropRef(c)
		}
	}
}

// dropAllObjects releases every reference the registry holds. It runs when the
// window is destroyed.
func dropAllObjects(hwnd uintptr) {
	regMu.Lock()
	var list []*core
	for k, h := range objs {
		if k.hwnd != hwnd {
			continue
		}
		if c := cores[h]; c != nil {
			list = append(list, c)
		}
		delete(objs, k)
	}
	regMu.Unlock()
	for _, c := range list {
		dropRef(c)
	}
}

// dropRef releases one reference that the registry holds on c.
func dropRef(c *core) {
	regMu.Lock()
	n := c.refs.Add(-1)
	if n == 0 {
		delete(cores, c.handle)
	}
	regMu.Unlock()
	if n == 0 {
		c.free()
		recycle(c)
	}
}

// IUnknown, shared by every vtable. The callbacks find the object from the view
// they are called on, so one set of callbacks serves every view.

func uiaQueryInterface(this, riid, ppv uintptr) uintptr {
	if ppv == 0 || riid == 0 {
		return hrPointer
	}
	putPtr(ppv, 0)
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	// The caller holds a reference, so the count is not zero and the core is
	// not being freed; the reference added here is taken under the lock below.
	iid := *(*guid)(ptr(riid))
	regMu.Lock()
	v := c.viewFor(iid)
	if v != 0 {
		c.refs.Add(1)
	}
	regMu.Unlock()
	if v == 0 {
		return hrNoInterface
	}
	putPtr(ppv, v)
	return hrOK
}

func uiaAddRef(this uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return 0
	}
	return uintptr(c.refs.Add(1))
}

// uiaRelease drops a reference. The last reference removes the object from the
// registry and frees its views.
func uiaRelease(this uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return 0
	}
	regMu.Lock()
	n := c.refs.Add(-1)
	if n == 0 {
		delete(cores, c.handle)
		if c.kind == kindRoot && roots[c.hwnd] == c.handle {
			delete(roots, c.hwnd)
		}
	}
	regMu.Unlock()
	if n == 0 {
		c.free()
		recycle(c)
	}
	return uintptr(n)
}

// IRawElementProviderSimple.

func uiaProviderOptions(this, out uintptr) uintptr {
	if _, hr := enter(this, out); hr != hrOK {
		return hr
	}
	*(*int32)(ptr(out)) = providerServerSide
	return hrOK
}

func uiaGetPatternProvider(this, id, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.pattern(int32(id))
	if hr != hrOK {
		return hr
	}
	putPtr(out, v)
	return hrOK
}

func uiaGetPropertyValue(this, id, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.property(int32(id))
	if hr != hrOK {
		return hr
	}
	*(*variant)(ptr(out)) = v
	return hrOK
}

func uiaHostProvider(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	if c.kind == kindRoot {
		// The window is the host. UI Automation builds the window's element
		// from it and makes the fragment root its child, so the window's
		// children are the nodes of the tree. With no host, the fragment root
		// would appear as a second window, one level below the window the
		// client asked for, and the nodes would sit one level too deep.
		h := hostFor(c.hwnd)
		if h == 0 {
			putPtr(out, 0)
			return hrOK
		}
		// The caller takes a reference, which it releases when done.
		syscall.SyscallN(comMethod(h, 1), h) // AddRef
		putPtr(out, h)
		return hrOK
	}
	putPtr(out, 0)
	return hrOK
}

// hosts holds the host provider of each window, which UI Automation makes once
// per window. The registry holds one reference to it; forgetHost releases it.
// Making a host on each query left one native object behind per query.
var (
	hostMu sync.Mutex
	hosts  = map[uintptr]uintptr{} // window handle to its host provider
)

// hostFor returns the host provider of the window hwnd, without a new reference,
// making it on first use. It returns 0 if UI Automation cannot make one.
func hostFor(hwnd uintptr) uintptr {
	hostMu.Lock()
	defer hostMu.Unlock()
	if h, ok := hosts[hwnd]; ok {
		return h
	}
	return makeHost(hwnd)
}

// makeHost asks UI Automation for the host provider of hwnd and records it. The
// caller holds hostMu. The variable it writes to is local to this function, so
// the cache hit in hostFor does not allocate.
func makeHost(hwnd uintptr) uintptr {
	var h uintptr
	if win32.UiaHostProviderFromHwnd(hwnd, uintptr(unsafe.Pointer(&h))) != hrOK || h == 0 {
		return 0
	}
	hosts[hwnd] = h
	return h
}

// forgetHost releases the registry's reference to the host provider of hwnd.
func forgetHost(hwnd uintptr) {
	hostMu.Lock()
	h, ok := hosts[hwnd]
	delete(hosts, hwnd)
	hostMu.Unlock()
	if ok {
		syscall.SyscallN(comMethod(h, iUnknownRelease), h)
	}
}

// IRawElementProviderFragment.

func uiaNavigate(this, dir, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.navigate(int32(dir))
	if hr != hrOK {
		return hr
	}
	putPtr(out, v)
	return hrOK
}

func uiaRuntimeID(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	psa, hr := c.runtimeID()
	if hr != hrOK {
		return hr
	}
	putPtr(out, psa)
	return hrOK
}

// uiaBoundingRectangle writes a UiaRect, four doubles: left, top, width, height.
func uiaBoundingRectangle(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	x, y, w, h, hr := c.boundingRect()
	if hr != hrOK {
		return hr
	}
	r := (*[4]float64)(ptr(out))
	r[0], r[1], r[2], r[3] = x, y, w, h
	return hrOK
}

func uiaEmbeddedFragmentRoots(this, out uintptr) uintptr {
	if _, hr := enter(this, out); hr != hrOK {
		return hr
	}
	putPtr(out, 0)
	return hrOK
}

func uiaSetFocus(this uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.setFocus()
}

// uiaFragmentRoot answers the FragmentRoot property of a fragment. Every
// fragment belongs to the window's root.
func uiaFragmentRoot(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	putPtr(out, c.provide(access.RootID, iidRawElementProviderFragmentRoot))
	return hrOK
}

// uiaElementFromPoint answers IRawElementProviderFragmentRoot::ElementProviderFromPoint.
// Its coordinates are doubles, passed in XMM1 and XMM2, which are reached through
// the trampoline in uia_amd64.s; x and y arrive here as the bit patterns of
// those doubles, in screen pixels.
func uiaElementFromPoint(this, x, y, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	putPtr(out, 0)
	origin := clientOrigin(c.hwnd)
	cw, ch := clientSize(c.hwnd)
	cx, cy := clientPoint(origin, math.Float64frombits(uint64(x)), math.Float64frombits(uint64(y)))
	id := hitTestPoint(c.bridge.Tree(), cx, cy, cw, ch)
	if id == 0 {
		return hrOK
	}
	putPtr(out, c.provide(id, iidRawElementProviderFragment))
	return hrOK
}

// IRawElementProviderFragmentRoot.

func uiaGetFocus(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.focused()
	if hr != hrOK {
		return hr
	}
	putPtr(out, v)
	return hrOK
}

// IInvokeProvider.

func uiaInvoke(this uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.invoke()
}

// The pattern interfaces. Each method runs its core method, which answers from
// the snapshot and posts an action to the UI thread, as the Invoke pattern does.

// boolOf returns the BOOL value of b for an out parameter.
func boolOf(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// IToggleProvider.

func uiaToggle(this uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.toggle()
}

func uiaToggleState(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.toggleState()
	if hr != hrOK {
		return hr
	}
	*(*int32)(ptr(out)) = v
	return hrOK
}

// IExpandCollapseProvider.

func uiaExpand(this uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.expand(true)
}

func uiaCollapse(this uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.expand(false)
}

func uiaExpandCollapseState(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.expandState()
	if hr != hrOK {
		return hr
	}
	*(*int32)(ptr(out)) = v
	return hrOK
}

// ISelectionItemProvider.

func uiaSelect(this uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.selectItem()
}

// uiaAddToSelection selects the item: a container holds one selection.
func uiaAddToSelection(this uintptr) uintptr { return uiaSelect(this) }

// uiaRemoveFromSelection is refused: a selection is always one item, which
// another item replaces.
func uiaRemoveFromSelection(this uintptr) uintptr {
	if coreOf(this) == nil {
		return hrFail
	}
	return hrInvalidOperation
}

func uiaIsSelected(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.isSelected()
	if hr != hrOK {
		return hr
	}
	*(*int32)(ptr(out)) = boolOf(v)
	return hrOK
}

func uiaSelectionContainer(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.selectionContainer()
	if hr != hrOK {
		return hr
	}
	putPtr(out, v)
	return hrOK
}

// ISelectionProvider.

func uiaGetSelection(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.selection()
	if hr != hrOK {
		return hr
	}
	putPtr(out, v)
	return hrOK
}

// uiaCanSelectMultiple reports that a container holds one selection.
func uiaCanSelectMultiple(this, out uintptr) uintptr {
	if _, hr := enter(this, out); hr != hrOK {
		return hr
	}
	*(*int32)(ptr(out)) = 0
	return hrOK
}

// uiaIsSelectionRequired reports whether a selection must hold an item. A tab list
// always shows its selected tab, so it is required; any other container may be empty.
func uiaIsSelectionRequired(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	n, _, ok := c.current()
	if !ok {
		return hrElementNotAvailable
	}
	v := int32(0)
	if n.Role == access.RoleTabList {
		v = 1
	}
	*(*int32)(ptr(out)) = v
	return hrOK
}

// IValueProvider.

func uiaSetValue(this, val uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.setValue(val)
}

func uiaValue(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.value()
	if hr != hrOK {
		return hr
	}
	putPtr(out, v)
	return hrOK
}

func uiaIsReadOnly(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	v, hr := c.isReadOnly()
	if hr != hrOK {
		return hr
	}
	*(*int32)(ptr(out)) = boolOf(v)
	return hrOK
}

// IRangeValueProvider. SetValue's double is passed in XMM1, which the trampoline
// moves into the integer register the callback reads (see uia_amd64.s).

func uiaSetRangeValue(this, val uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.setRangeValue(math.Float64frombits(uint64(val)))
}

// rangeNumberOut answers one getter of IRangeValueProvider: f picks the number
// from the range of the node that the object stands for.
func rangeNumberOut(this, out uintptr, f func(r access.Range) float64) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	r, hr := c.rangeOf()
	if hr != hrOK {
		return hr
	}
	*(*float64)(ptr(out)) = f(r)
	return hrOK
}

func uiaRangeValue(this, out uintptr) uintptr {
	return rangeNumberOut(this, out, func(r access.Range) float64 { return r.Value })
}

func uiaRangeMaximum(this, out uintptr) uintptr {
	return rangeNumberOut(this, out, func(r access.Range) float64 { return r.Max })
}

func uiaRangeMinimum(this, out uintptr) uintptr {
	return rangeNumberOut(this, out, func(r access.Range) float64 { return r.Min })
}

func uiaRangeLargeChange(this, out uintptr) uintptr {
	return rangeNumberOut(this, out, func(r access.Range) float64 { return r.LargeChange })
}

func uiaRangeSmallChange(this, out uintptr) uintptr {
	return rangeNumberOut(this, out, func(r access.Range) float64 { return r.SmallChange })
}

// IScrollProvider. Its amounts are ScrollAmount values, and the percents are
// doubles (the trampoline moves them into the integer registers).

func uiaScroll(this, horizontal, vertical uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.scroll(int32(horizontal), int32(vertical))
}

func uiaSetScrollPercent(this, horizontal, vertical uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.setScrollPercent(math.Float64frombits(uint64(horizontal)), math.Float64frombits(uint64(vertical)))
}

// scrollOut answers one getter of IScrollProvider: f picks the value from the
// node's scroll state.
func scrollOut(this, out uintptr, f func(s scrollState) float64) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	s, hr := c.scrollState()
	if hr != hrOK {
		return hr
	}
	*(*float64)(ptr(out)) = f(s)
	return hrOK
}

func uiaHorizontalScrollPercent(this, out uintptr) uintptr {
	return scrollOut(this, out, func(s scrollState) float64 { return s.percentX() })
}

func uiaVerticalScrollPercent(this, out uintptr) uintptr {
	return scrollOut(this, out, func(s scrollState) float64 { return s.percentY() })
}

func uiaHorizontalViewSize(this, out uintptr) uintptr {
	return scrollOut(this, out, func(s scrollState) float64 { return s.viewX() })
}

func uiaVerticalViewSize(this, out uintptr) uintptr {
	return scrollOut(this, out, func(s scrollState) float64 { return s.viewY() })
}

func uiaHorizontallyScrollable(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	s, hr := c.scrollState()
	if hr != hrOK {
		return hr
	}
	*(*int32)(ptr(out)) = boolOf(s.canX())
	return hrOK
}

func uiaVerticallyScrollable(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	s, hr := c.scrollState()
	if hr != hrOK {
		return hr
	}
	*(*int32)(ptr(out)) = boolOf(s.canY())
	return hrOK
}

// IScrollItemProvider.

func uiaScrollIntoView(this uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.scrollIntoView()
}

// IWindowProvider. A dialog is a modal window: it is always topmost, and it is
// closed by the application, not through the pattern, so Close and SetVisualState
// are not supported.

// windowOut answers one getter of IWindowProvider, a BOOL or an enum value, from
// the node that the object stands for.
func windowOut(this, out uintptr, f func(n *access.Node) int32) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	n, _, ok := c.current()
	if !ok {
		return hrElementNotAvailable
	}
	*(*int32)(ptr(out)) = f(n)
	return hrOK
}

func uiaSetVisualState(this, state uintptr) uintptr {
	if coreOf(this) == nil {
		return hrFail
	}
	return hrNotSupported
}

func uiaCloseWindow(this uintptr) uintptr {
	if coreOf(this) == nil {
		return hrFail
	}
	return hrNotSupported
}

func uiaWaitForInputIdle(this, out uintptr) uintptr {
	return windowOut(this, out, func(*access.Node) int32 { return 1 })
}

func uiaCanMaximize(this, out uintptr) uintptr {
	return windowOut(this, out, func(*access.Node) int32 { return 0 })
}

func uiaCanMinimize(this, out uintptr) uintptr {
	return windowOut(this, out, func(*access.Node) int32 { return 0 })
}

func uiaIsModal(this, out uintptr) uintptr {
	return windowOut(this, out, func(n *access.Node) int32 { return boolOf(n.Role == access.RoleDialog) })
}

func uiaWindowVisualState(this, out uintptr) uintptr {
	return windowOut(this, out, func(*access.Node) int32 { return windowVisualNormal })
}

func uiaWindowInteractionState(this, out uintptr) uintptr {
	return windowOut(this, out, func(*access.Node) int32 { return windowReadyForInput })
}

func uiaIsTopmost(this, out uintptr) uintptr {
	return windowOut(this, out, func(n *access.Node) int32 { return boolOf(n.Role == access.RoleDialog) })
}
