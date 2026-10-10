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
	simpleVtbl   [7]uintptr // IRawElementProviderSimple
	fragmentVtbl [9]uintptr // IRawElementProviderFragment
	rootVtbl     [5]uintptr // IRawElementProviderFragmentRoot (its base is IUnknown)
	invokeVtbl   [4]uintptr // IInvokeProvider
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
)

// The interface IIDs, from the uuid attributes in UIAutomationCore.idl.
var (
	iidIUnknown                       = guid{0, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidRawElementProviderSimple       = guid{0xd6dd68d1, 0x86fd, 0x4332, [8]byte{0x86, 0x66, 0x9a, 0xbe, 0xde, 0xa2, 0xd2, 0x4c}}
	iidRawElementProviderFragment     = guid{0xf7063da8, 0x8359, 0x439c, [8]byte{0x92, 0x97, 0xbb, 0xc5, 0x29, 0x9a, 0x7d, 0x87}}
	iidRawElementProviderFragmentRoot = guid{0x620ce2a5, 0xab8f, 0x40a9, [8]byte{0x86, 0xcb, 0xde, 0x3c, 0x75, 0x59, 0x9b, 0x58}}
	iidInvokeProvider                 = guid{0x54fcb24b, 0xe18e, 0x47a2, [8]byte{0xb4, 0xd3, 0xec, 0xcb, 0xe7, 0x75, 0x99, 0xa2}}
)

// coreKind says what an object stands for.
type coreKind uint8

const (
	kindRoot    coreKind = iota // the window's fragment root
	kindNode                    // a node of the access tree
	kindPattern                 // the Invoke pattern of a node
)

// kindIfaces lists the interfaces that each kind of object implements.
var kindIfaces = [...][]*iface{
	kindRoot:    {&ifaceSimple, &ifaceFragment, &ifaceFragmentRoot},
	kindNode:    {&ifaceSimple, &ifaceFragment},
	kindPattern: {&ifaceInvoke},
}

// core is the Go-side state of an object. Its views share it.
type core struct {
	refs   atomic.Int32
	handle uintptr
	kind   coreKind
	id     access.ID // the node, for node and pattern objects; access.RootID for the root
	hwnd   uintptr   // the window
	bridge *access.Bridge
	views  []comView
	// root is the window's node as of the snapshot it was made from, for the
	// root object only (see rootNode).
	root atomic.Pointer[rootSnap]
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
	c := &core{handle: lastHandle, kind: kind, id: id, hwnd: hwnd, bridge: b}
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

// objs holds one object for each node and kind, so that a node is the same
// object each time UI Automation asks for it. UI Automation keeps the objects it
// has been given, so making a new object per query would grow the registry with
// every walk of the tree. The registry owns one reference to each object, which
// dropObjects releases when the node leaves the tree.
var objs = map[objKey]uintptr{} // to the handle of the object

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
	for _, kind := range []coreKind{kindNode, kindPattern} {
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
