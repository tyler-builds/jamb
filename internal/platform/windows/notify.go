//go:build windows

package windows

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/win32"
)

// OS notifications are Windows toasts, built through the Windows Runtime objects of
// windows.ui.notifications.h and windows.data.xml.dom.h, which are called through
// their vtables as the jump list is. The runtime objects are made with
// RoGetActivationFactory, which needs COM on the calling thread: the UI thread
// has it from OleInitialize.
//
// Vtable indices count from 0 with IUnknown at 0 to 2 and IInspectable at 3 to 5,
// so a method of a WinRT interface starts at 6. They are taken from the SDK's
// cppwinrt headers, which carry the abi structs in declaration order:
// impl/windows.ui.notifications.0.h (IToastNotificationManagerStatics at 1249,
// IToastNotifier at 1280, and the rest of the toast interfaces) and
// impl/windows.data.xml.dom.0.h (IXmlDocumentIO). The interface IDs are the
// uuid values in the same files.

const (
	// inspectableMethod is the first method index of a WinRT interface.
	inspectableMethod = 6

	// IToastNotificationManagerStatics.
	tmsCreateToastNotifier       = inspectableMethod
	tmsCreateToastNotifierWithID = inspectableMethod + 1
	// IToastNotificationManagerStatics2.
	tmsGetHistory = inspectableMethod
	// IToastNotifier.
	tnShow = inspectableMethod
	// IToastNotificationHistory.
	thRemove = inspectableMethod + 4
	thClear  = inspectableMethod + 5
	// IToastNotificationFactory.
	tfCreateToastNotification = inspectableMethod
	// IToastNotification.
	tnAddActivated = inspectableMethod + 5
	tnAddFailed    = inspectableMethod + 7
	// IToastNotification2.
	tn2PutTag = inspectableMethod
	// IXmlDocumentIO.
	xdioLoadXML = inspectableMethod
	// IToastActivatedEventArgs and IToastFailedEventArgs.
	taGetArguments = inspectableMethod
	tfGetErrorCode = inspectableMethod
	// IActivationFactory.
	afActivateInstance = inspectableMethod
	// IPersistFile: IPersist's GetClassID is 3, so IsDirty is 4, Load 5 and Save 6. The
	// zig MinGW headers in objidl.h (IPersistFileVtbl) give the same order.
	pfLoad = 5
	pfSave = 6

	// The modes that IPersistFile::Load opens a shortcut with: STGM_READ, and
	// STGM_READWRITE for a shortcut that is then saved.
	stgmRead      = 0x0
	stgmReadWrite = 0x2

	// shortcutPathChars is the room for a shortcut's target path, in UTF-16 characters.
	shortcutPathChars = 1024
	// findDataWSize is the size of WIN32_FIND_DATAW, which GetPath fills in (minwinbase.h).
	findDataWSize = 592

	// eNoInterface is E_NOINTERFACE.
	eNoInterface = 0x80004002
)

// The class and interface identifiers of the notification objects.
var (
	iidActivationFactory = guid{0x00000035, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidPersistFile       = guid{0x0000010B, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidXmlDocumentIO     = guid{0x6CD0E74E, 0xEE65, 0x4489, [8]byte{0x9E, 0xBF, 0xCA, 0x43, 0xE8, 0x7B, 0xA6, 0x37}}
	iidXmlDocument       = guid{0xF7F3A506, 0x1E87, 0x42D6, [8]byte{0xBC, 0xFB, 0xB8, 0xC8, 0x09, 0xFA, 0x54, 0x94}}
	iidToastStatics      = guid{0x50AC103F, 0xD235, 0x4598, [8]byte{0xBB, 0xEF, 0x98, 0xFE, 0x4D, 0x1A, 0x3A, 0xD4}}
	iidToastStatics2     = guid{0x7AB93C52, 0x0E48, 0x4750, [8]byte{0xBA, 0x9D, 0x1A, 0x41, 0x13, 0x98, 0x18, 0x47}}
	iidToastFactory      = guid{0x04124B20, 0x82C6, 0x4229, [8]byte{0xB1, 0x09, 0xFD, 0x9E, 0xD4, 0x66, 0x2B, 0x53}}
	iidToastNotifier     = guid{0x75927B93, 0x03F3, 0x41EC, [8]byte{0x91, 0xD3, 0x6E, 0x5B, 0xAC, 0x1B, 0x38, 0xE7}}
	iidToastNotification = guid{0x997E2675, 0x059E, 0x4E60, [8]byte{0x8B, 0x06, 0x17, 0x60, 0x91, 0x7C, 0x8B, 0x80}}
	iidToastNotif2       = guid{0x9DFB9FD1, 0x143A, 0x490E, [8]byte{0x90, 0xBF, 0xB9, 0xFB, 0xA7, 0x13, 0x2D, 0xE7}}
	iidToastHistory      = guid{0x5CADDC63, 0x01D3, 0x4C97, [8]byte{0x98, 0x6F, 0x05, 0x33, 0x48, 0x3F, 0xEE, 0x14}}
	iidActivatedArgs     = guid{0xE3BF92F3, 0xC197, 0x436F, [8]byte{0x82, 0x65, 0x06, 0x25, 0x82, 0x4F, 0x8D, 0xAC}}
	iidFailedArgs        = guid{0x35176862, 0xCFD4, 0x44F8, [8]byte{0xAD, 0x64, 0xF5, 0x00, 0xFD, 0x89, 0x6C, 0x3B}}
)

// pkeyAppUserModelID is PKEY_AppUserModel_ID, which a shortcut carries so that
// its target's notifications and taskbar button use the application ID.
var pkeyAppUserModelID = propertyKey{guid{0x9F4C2855, 0x9F79, 0x4B39, [8]byte{0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}}, 5}

// The event handlers are TypedEventHandler delegates: Activated is called with the
// toast and an IInspectable, and Failed with the toast and IToastFailedEventArgs.
// The IIDs are the parameterized IIDs of those delegates, computed from the
// signatures that cppwinrt's base.h builds (TypedEventHandler's generic ID is
// 9DE1C534-6AE1-11E0-84E1-18A905BCC53F; ToastNotification's default interface is
// IToastNotification).
const toastSignature = "rc(Windows.UI.Notifications.ToastNotification;{997e2675-059e-4e60-8b06-1760917c8b80})"

// The delegate kinds, which are also the handles of their objects.
const (
	eventActivated = iota
	eventFailed
)

// eventIIDs are the IIDs of the two delegates, written out so that the package links no
// crypto package (whose import would add the 32 MB buffer of Go's FIPS module to every
// app; see BENCHMARKS.md). TestEventIIDsMatchTheirSignatures computes them from the
// signatures, as the Windows Runtime does: TypedEventHandler<ToastNotification, Object>
// is AB54DE2D-97D9-5528-B6AD-105AFE156530, which Windows documents, and
// TypedEventHandler<ToastNotification, ToastFailedEventArgs> is
// 062A90FB-319A-5920-83AF-01D5F464765C.
var eventIIDs = [2]guid{
	{0xAB54DE2D, 0x97D9, 0x5528, [8]byte{0xB6, 0xAD, 0x10, 0x5A, 0xFE, 0x15, 0x65, 0x30}},
	{0x062A90FB, 0x319A, 0x5920, [8]byte{0x83, 0xAF, 0x01, 0xD5, 0xF4, 0x64, 0x76, 0x5C}},
}

// Each delegate has one object, made once and kept for the life of the process: the
// process holds one reference to it, and the references the runtime takes and
// drops balance around that. The object's view stores its kind as the handle.
var (
	toastVtbls     [2][4]uintptr // IUnknown, then Invoke
	toastVtblsOnce sync.Once
	toastRefs      [2]int32
	toastViews     [2]uintptr
	toastViewsMu   sync.Mutex
)

// toastCallbacks are the Go functions that the event handlers call. They run on
// the thread that raises the event, which is not the UI thread.
type toastCallbacks struct {
	activated func(arguments string)
	failed    func(hr uint32)
}

var toastCB atomic.Pointer[toastCallbacks]

// SetToastCallbacks sets the functions called when a toast is activated, with the
// arguments of the activated action, and when a toast fails, with its HRESULT.
// They run on the thread that Windows raises the event on, so they must only post
// the work to the UI thread.
func SetToastCallbacks(activated func(arguments string), failed func(hr uint32)) {
	toastCB.Store(&toastCallbacks{activated, failed})
}

// The factories and the shown toasts are only touched on the UI thread.
var (
	toastManagerObj uintptr // IToastNotificationManagerStatics
	toastStaticsV2  uintptr // IToastNotificationManagerStatics2
	toastFactoryObj uintptr // IToastNotificationFactory
	xmlFactoryObj   uintptr // IActivationFactory of XmlDocument
	// shownToasts keeps the toasts that have been shown, so their events are still
	// delivered; the oldest are released beyond shownKeep.
	shownToasts []uintptr
)

const shownKeep = 32

// handlerKind reads the kind of a handler object, which its view holds as the handle.
func handlerKind(this uintptr) int { return int(*(*uintptr)(ptr(this + wordSize))) }

// handlerQueryInterface answers for IUnknown and the delegate's own IID.
func handlerQueryInterface(this, riid, out uintptr) uintptr {
	kind := handlerKind(this)
	iid := *(*guid)(ptr(riid))
	if iid != iidIUnknown && iid != eventIIDs[kind] {
		*(*uintptr)(ptr(out)) = 0
		return eNoInterface
	}
	atomic.AddInt32(&toastRefs[kind], 1)
	*(*uintptr)(ptr(out)) = this
	return 0
}

func handlerAddRef(this uintptr) uintptr {
	return uintptr(atomic.AddInt32(&toastRefs[handlerKind(this)], 1))
}

// handlerRelease drops a reference. The process's own reference is never dropped,
// so the count does not reach zero and the object is not freed.
func handlerRelease(this uintptr) uintptr {
	return uintptr(atomic.AddInt32(&toastRefs[handlerKind(this)], -1))
}

// handlerInvoke is the Invoke of both delegates: (sender, args). It reads the
// arguments while the event is raised, then passes them to the callbacks.
func handlerInvoke(this, sender, args uintptr) uintptr {
	cb := toastCB.Load()
	if cb == nil || args == 0 {
		return 0
	}
	switch handlerKind(this) {
	case eventActivated:
		var out uintptr
		if err := comCall("QueryInterface", args, iUnknownQueryInterface, uintptr(unsafe.Pointer(&iidActivatedArgs)), uintptr(unsafe.Pointer(&out))); err != nil {
			return 0
		}
		var h uintptr
		err := comCall("get_Arguments", out, taGetArguments, uintptr(unsafe.Pointer(&h)))
		release(out)
		if err == nil && cb.activated != nil {
			text := win32.HSTRINGText(h)
			cb.activated(text)
		}
		win32.DeleteHString(h)
	case eventFailed:
		var out uintptr
		if err := comCall("QueryInterface", args, iUnknownQueryInterface, uintptr(unsafe.Pointer(&iidFailedArgs)), uintptr(unsafe.Pointer(&out))); err != nil {
			return 0
		}
		var code int32
		err := comCall("get_ErrorCode", out, tfGetErrorCode, uintptr(unsafe.Pointer(&code)))
		release(out)
		if err == nil && cb.failed != nil {
			cb.failed(uint32(code))
		}
	}
	return 0
}

// handlerView returns the delegate object of kind, made on first use. It is called
// on the UI thread.
func handlerView(kind int) (uintptr, error) {
	toastViewsMu.Lock()
	defer toastViewsMu.Unlock()
	if toastViews[kind] != 0 {
		return toastViews[kind], nil
	}
	toastVtblsOnce.Do(func() {
		toastVtbls = [2][4]uintptr{
			{syscall.NewCallback(handlerQueryInterface), syscall.NewCallback(handlerAddRef), syscall.NewCallback(handlerRelease), syscall.NewCallback(handlerInvoke)},
			{syscall.NewCallback(handlerQueryInterface), syscall.NewCallback(handlerAddRef), syscall.NewCallback(handlerRelease), syscall.NewCallback(handlerInvoke)},
		}
	})
	view := allocView(toastVtbls[kind][:], uintptr(kind))
	if view == 0 {
		return 0, fmt.Errorf("platform/windows: no memory for a notification event handler")
	}
	atomic.StoreInt32(&toastRefs[kind], 1)
	toastViews[kind] = view
	return view, nil
}

// rtFactory returns the activation factory of the runtime class class, as iid.
func rtFactory(class string, iid *guid) (uintptr, error) {
	var out uintptr
	if hr := hresult(win32.RoGetActivationFactory(class, unsafe.Pointer(iid), &out)); hr.failed() {
		return 0, fmt.Errorf("platform/windows: RoGetActivationFactory(%s) failed with HRESULT 0x%08X", class, uint32(hr))
	}
	return out, nil
}

// queryInterface returns obj as the interface iid. The caller releases it.
func queryInterface(what string, obj uintptr, iid *guid) (uintptr, error) {
	var out uintptr
	if err := comCall("QueryInterface "+what, obj, iUnknownQueryInterface, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out))); err != nil {
		return 0, err
	}
	return out, nil
}

// toastManager returns the ToastNotificationManager statics and their second
// interface, which has the history. They are made once.
func toastManager() (uintptr, uintptr, error) {
	if toastManagerObj == 0 {
		f, err := rtFactory("Windows.UI.Notifications.ToastNotificationManager", &iidToastStatics)
		if err != nil {
			return 0, 0, err
		}
		toastManagerObj = f
	}
	if toastStaticsV2 == 0 {
		v2, err := queryInterface("IToastNotificationManagerStatics2", toastManagerObj, &iidToastStatics2)
		if err != nil {
			return 0, 0, err
		}
		toastStaticsV2 = v2
	}
	return toastManagerObj, toastStaticsV2, nil
}

// newToastDocument returns an XmlDocument holding xml, as IXmlDocument. The caller
// releases it.
func newToastDocument(xml string) (uintptr, error) {
	if xmlFactoryObj == 0 {
		f, err := rtFactory("Windows.Data.Xml.Dom.XmlDocument", &iidActivationFactory)
		if err != nil {
			return 0, err
		}
		xmlFactoryObj = f
	}
	var insp uintptr
	if err := comCall("ActivateInstance", xmlFactoryObj, afActivateInstance, uintptr(unsafe.Pointer(&insp))); err != nil {
		return 0, err
	}
	defer release(insp)
	io, err := queryInterface("IXmlDocumentIO", insp, &iidXmlDocumentIO)
	if err != nil {
		return 0, err
	}
	defer release(io)
	s, err := win32.HSTRING(xml)
	if err != nil {
		return 0, err
	}
	err = comCall("LoadXml", io, xdioLoadXML, s)
	win32.DeleteHString(s)
	if err != nil {
		return 0, err
	}
	return queryInterface("IXmlDocument", insp, &iidXmlDocument)
}

// ShowToast shows a toast of the XML content, under the Application User Model ID
// appID, with the tag (empty for none). A packaged process has the identity of its
// package, so appID is then empty and the notifier takes no ID. The toast's events
// are delivered to the callbacks set by SetToastCallbacks. It must run on the UI
// thread, which has COM.
func ShowToast(appID, content, tag string) error {
	mgr, _, err := toastManager()
	if err != nil {
		return err
	}
	var notifier uintptr
	if appID == "" {
		err = comCall("CreateToastNotifier", mgr, tmsCreateToastNotifier, uintptr(unsafe.Pointer(&notifier)))
	} else {
		var id uintptr
		if id, err = win32.HSTRING(appID); err != nil {
			return err
		}
		err = comCall("CreateToastNotifierWithId", mgr, tmsCreateToastNotifierWithID, id, uintptr(unsafe.Pointer(&notifier)))
		win32.DeleteHString(id)
	}
	if err != nil {
		return err
	}
	defer release(notifier)

	doc, err := newToastDocument(content)
	if err != nil {
		return err
	}
	defer release(doc)

	if toastFactoryObj == 0 {
		if toastFactoryObj, err = rtFactory("Windows.UI.Notifications.ToastNotification", &iidToastFactory); err != nil {
			return err
		}
	}
	var toast uintptr
	if err := comCall("CreateToastNotification", toastFactoryObj, tfCreateToastNotification, doc, uintptr(unsafe.Pointer(&toast))); err != nil {
		return err
	}
	// The toast is kept by shownToasts when it is shown, and released here otherwise.
	released := false
	defer func() {
		if !released {
			release(toast)
		}
	}()

	if tag != "" {
		t2, err := queryInterface("IToastNotification2", toast, &iidToastNotif2)
		if err != nil {
			return err
		}
		defer release(t2)
		s, err := win32.HSTRING(tag)
		if err != nil {
			return err
		}
		err = comCall("put_Tag", t2, tn2PutTag, s)
		win32.DeleteHString(s)
		if err != nil {
			return err
		}
	}

	// The tokens identify the subscriptions for removal, which is never needed: the
	// toast is released when it is no longer kept.
	var token int64
	activated, err := handlerView(eventActivated)
	if err != nil {
		return err
	}
	if err := comCall("add_Activated", toast, tnAddActivated, activated, uintptr(unsafe.Pointer(&token))); err != nil {
		return err
	}
	failed, err := handlerView(eventFailed)
	if err != nil {
		return err
	}
	if err := comCall("add_Failed", toast, tnAddFailed, failed, uintptr(unsafe.Pointer(&token))); err != nil {
		return err
	}

	if err := comCall("Show", notifier, tnShow, toast); err != nil {
		return err
	}
	released = true
	shownToasts = append(shownToasts, toast)
	if len(shownToasts) > shownKeep {
		release(shownToasts[0])
		shownToasts = shownToasts[1:]
	}
	return nil
}

// history returns the notification history of the process's identity, as
// IToastNotificationHistory. The caller releases it.
func history() (uintptr, error) {
	_, v2, err := toastManager()
	if err != nil {
		return 0, err
	}
	var h uintptr
	if err := comCall("get_History", v2, tmsGetHistory, uintptr(unsafe.Pointer(&h))); err != nil {
		return 0, err
	}
	return h, nil
}

// RemoveToast removes the toast with the tag from the notification center.
func RemoveToast(tag string) error {
	h, err := history()
	if err != nil {
		return err
	}
	defer release(h)
	s, err := win32.HSTRING(tag)
	if err != nil {
		return err
	}
	defer win32.DeleteHString(s)
	return comCall("Remove", h, thRemove, s)
}

// ClearToasts removes the process's toasts from the notification center.
func ClearToasts() error {
	h, err := history()
	if err != nil {
		return err
	}
	defer release(h)
	return comCall("Clear", h, thClear)
}

// WriteShortcut writes the shortcut at path to the program target, with the
// Application User Model ID appID, which Windows reads to match the process's
// notifications to the shortcut. The file is replaced if it exists.
func WriteShortcut(path, target, appID string) error {
	link, err := createCOM("IShellLinkW", &clsidShellLink, &iidShellLinkW)
	if err != nil {
		return err
	}
	defer release(link)
	var live []*uint16
	defer func() { runtime.KeepAlive(live) }()
	if err := comCall("SetPath", link, slSetPath, wide(&live, target)); err != nil {
		return err
	}
	if err := comCall("SetWorkingDirectory", link, slSetWorkingDirectory, wide(&live, filepath.Dir(target))); err != nil {
		return err
	}
	if err := setStringProperty(link, pkeyAppUserModelID, appID, &live); err != nil {
		return err
	}
	pf, err := queryInterface("IPersistFile", link, &iidPersistFile)
	if err != nil {
		return err
	}
	defer release(pf)
	// The second argument of Save is TRUE, which saves the file at the path given.
	return comCall("Save", pf, pfSave, wide(&live, path), 1)
}

// InitializeCOM initializes COM on the calling thread, for code that runs before the UI
// thread exists, such as the installer's shortcut stamp. The UI thread gets it from its
// window.
func InitializeCOM() { win32.OleInitialize() }

// ReadShortcut returns the target path and the Application User Model ID of the shortcut
// at path. The ID is empty when the shortcut has none. The shortcut is loaded but not
// resolved, so a target that is missing or on an unavailable drive does not delay the read.
func ReadShortcut(path string) (target, appID string, err error) {
	var live []*uint16
	defer func() { runtime.KeepAlive(live) }()
	link, pf, err := loadShortcut(path, stgmRead, &live)
	if err != nil {
		return "", "", err
	}
	defer release(pf)
	defer release(link)
	buf := make([]uint16, shortcutPathChars)
	var fd [findDataWSize]byte
	if err := comCall("GetPath", link, slGetPath, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&fd[0])), 0); err != nil {
		return "", "", err
	}
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	appID, err = shortcutAppID(link)
	if err != nil {
		return "", "", err
	}
	return syscall.UTF16ToString(buf[:n]), appID, nil
}

// SetShortcutAppID sets the Application User Model ID of the existing shortcut at path to
// appID. The shortcut is loaded and saved, so its target and its other settings are kept.
func SetShortcutAppID(path, appID string) error {
	var live []*uint16
	defer func() { runtime.KeepAlive(live) }()
	link, pf, err := loadShortcut(path, stgmReadWrite, &live)
	if err != nil {
		return err
	}
	defer release(pf)
	defer release(link)
	if err := setStringProperty(link, pkeyAppUserModelID, appID, &live); err != nil {
		return err
	}
	return comCall("Save", pf, pfSave, wide(&live, path), 1)
}

// loadShortcut returns the shell link loaded from the file at path, opened with the mode,
// and its IPersistFile. The caller releases both.
func loadShortcut(path string, mode uintptr, live *[]*uint16) (link, pf uintptr, err error) {
	link, err = createCOM("IShellLinkW", &clsidShellLink, &iidShellLinkW)
	if err != nil {
		return 0, 0, err
	}
	pf, err = queryInterface("IPersistFile", link, &iidPersistFile)
	if err != nil {
		release(link)
		return 0, 0, err
	}
	if err := comCall("Load", pf, pfLoad, wide(live, path), mode); err != nil {
		release(pf)
		release(link)
		return 0, 0, err
	}
	return link, pf, nil
}

// shortcutAppID returns the System.AppUserModel.ID property of the loaded shell link, or
// empty when the link has none.
func shortcutAppID(link uintptr) (string, error) {
	var store uintptr
	if err := comCall("QueryInterface", link, iUnknownQueryInterface,
		uintptr(unsafe.Pointer(&iidPropertyStore)), uintptr(unsafe.Pointer(&store))); err != nil {
		return "", err
	}
	defer release(store)
	var pv propVariant
	if err := comCall("GetValue", store, psGetValue, uintptr(unsafe.Pointer(&pkeyAppUserModelID)), uintptr(unsafe.Pointer(&pv))); err != nil {
		return "", err
	}
	if pv.vt != vtLPWSTR || pv.ptr == 0 {
		return "", nil
	}
	id := wideString(pv.ptr)
	win32.CoTaskMemFree(pv.ptr)
	return id, nil
}

// SetCurrentUserString sets the REG_SZ value name of the key subkey under
// HKEY_CURRENT_USER, which is created when it does not exist. An empty name sets
// the key's default value.
func SetCurrentUserString(subkey, name, value string) error {
	return win32.SetRegistryString(win32.HKEY_CURRENT_USER, subkey, name, value)
}
