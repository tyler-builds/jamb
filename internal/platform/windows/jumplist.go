//go:build windows

package windows

import (
	"fmt"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/win32"
)

// The jump list is a custom destination list, built through the COM objects of
// shobjidl.h and propsys.h and called through their vtables, as the file dialogs
// are. Each task is an IShellLinkW, and its title is set through the link's
// IPropertyStore. Vtable indices are counted from the declaration order, with
// IUnknown at 0 to 2.
const (
	iUnknownQueryInterface = 0

	// IObjectArray has GetCount and GetAt, so IObjectCollection's AddObject is 5.
	ocAddObject = 5

	// ICustomDestinationList.
	cdlBeginList           = 4
	cdlAppendKnownCategory = 6
	cdlAddUserTasks        = 7
	cdlCommitList          = 8
	cdlAbortList           = 11

	// KDC_RECENT, the Recent category of KNOWNDESTCATEGORY.
	kdcRecent = 2

	// IShellLinkW.
	slSetDescription      = 7
	slSetArguments        = 11
	slSetIconLocation     = 17
	slSetPath             = 20
	slSetWorkingDirectory = 9

	// IPropertyStore.
	psSetValue = 6
	psCommit   = 7

	// VT_LPWSTR, the PROPVARIANT type of a wide string.
	vtLPWSTR = 31
)

// The class and interface identifiers, from shobjidl_core.h and propsys.h.
var (
	clsidDestinationList            = guid{0x77f10cf0, 0x3db5, 0x4966, [8]byte{0xb5, 0x20, 0xb7, 0xc5, 0x4f, 0xd3, 0x5e, 0xd6}}
	iidCustomDestinationList        = guid{0x6332debf, 0x87b5, 0x4670, [8]byte{0x90, 0xc0, 0x5e, 0x57, 0xb4, 0x08, 0xa4, 0x9e}}
	clsidEnumerableObjectCollection = guid{0x2d3468c1, 0x36a7, 0x43b6, [8]byte{0xac, 0x24, 0xd3, 0xf0, 0x2f, 0xd9, 0x60, 0x7a}}
	iidObjectArray                  = guid{0x92ca9dcd, 0x5622, 0x4bba, [8]byte{0xa8, 0x05, 0x5e, 0x9f, 0x54, 0x1b, 0xd8, 0xc9}}
	iidObjectCollection             = guid{0x5632b1a4, 0xe38a, 0x400a, [8]byte{0x92, 0x8a, 0xd4, 0xcd, 0x63, 0x23, 0x02, 0x95}}
	clsidShellLink                  = guid{0x00021401, 0, 0, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidShellLinkW                   = guid{0x000214f9, 0, 0, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidPropertyStore                = guid{0x886d8eeb, 0x8cf2, 0x4446, [8]byte{0x8d, 0x02, 0xcd, 0xba, 0x1d, 0xbd, 0xcf, 0x99}}
	pkeyTitle                       = propertyKey{guid{0xf29f85e0, 0x4ff9, 0x1068, [8]byte{0xab, 0x91, 0x08, 0x00, 0x2b, 0x27, 0xb3, 0xd9}}, 2}
)

// propertyKey is a PROPERTYKEY: a format identifier and a property identifier.
type propertyKey struct {
	fmtid guid
	pid   uint32
}

// propVariant is a PROPVARIANT, sized for 64-bit Windows, where its union takes
// 16 bytes after the 8-byte header. Only the string member is used.
type propVariant struct {
	vt  uint16
	_   [3]uint16
	ptr uintptr
	_   uintptr
}

// ShellTask is one task of a jump list: the program to start, its arguments, and
// the text and icon the list shows. Path is the program's full path.
type ShellTask struct {
	Title       string
	Path        string
	Args        string
	Description string
	IconPath    string // empty uses the program's own icon
	IconIndex   int
}

// SetJumpList replaces the jump list of this process with the tasks, in order, and
// with the Recent category when recent is set. The list belongs to the process's
// Application User Model ID, so the ID must be set before it is called. A failure
// aborts the change, and the previous list stays.
func SetJumpList(tasks []ShellTask, recent bool) error {
	dl, err := createCOM("ICustomDestinationList", &clsidDestinationList, &iidCustomDestinationList)
	if err != nil {
		return err
	}
	defer release(dl)

	var minSlots uint32
	var removed uintptr
	if err := comCall("BeginList", dl, cdlBeginList, uintptr(unsafe.Pointer(&minSlots)),
		uintptr(unsafe.Pointer(&iidObjectArray)), uintptr(unsafe.Pointer(&removed))); err != nil {
		return err
	}
	// The removed destinations are not used; the array is released.
	release(removed)

	if err := appendTasks(dl, tasks, recent); err != nil {
		comCall("AbortList", dl, cdlAbortList)
		return err
	}
	return comCall("CommitList", dl, cdlCommitList)
}

// appendTasks adds the tasks and, when recent is set, the Recent category to a
// list that has begun.
func appendTasks(dl uintptr, tasks []ShellTask, recent bool) error {
	if len(tasks) > 0 {
		arr, err := newTaskArray(tasks)
		if err != nil {
			return err
		}
		err = comCall("AddUserTasks", dl, cdlAddUserTasks, arr)
		release(arr)
		if err != nil {
			return err
		}
	}
	if recent {
		return comCall("AppendKnownCategory", dl, cdlAppendKnownCategory, kdcRecent)
	}
	return nil
}

// newTaskArray returns an IObjectArray of the tasks' shell links, in order. The
// caller releases it.
func newTaskArray(tasks []ShellTask) (uintptr, error) {
	coll, err := createCOM("IObjectCollection", &clsidEnumerableObjectCollection, &iidObjectCollection)
	if err != nil {
		return 0, err
	}
	defer release(coll)
	for _, t := range tasks {
		link, err := newShellLink(t)
		if err != nil {
			return 0, err
		}
		// AddObject takes its own reference, so the link is released here.
		err = comCall("AddObject", coll, ocAddObject, link)
		release(link)
		if err != nil {
			return 0, err
		}
	}
	var arr uintptr
	if err := comCall("QueryInterface", coll, iUnknownQueryInterface,
		uintptr(unsafe.Pointer(&iidObjectArray)), uintptr(unsafe.Pointer(&arr))); err != nil {
		return 0, err
	}
	return arr, nil
}

// newShellLink returns an IShellLinkW that starts the task. The caller releases it.
func newShellLink(t ShellTask) (uintptr, error) {
	link, err := createCOM("IShellLinkW", &clsidShellLink, &iidShellLinkW)
	if err != nil {
		return 0, err
	}
	// live holds the wide strings the calls read, until the link is built.
	var live []*uint16
	defer func() { runtime.KeepAlive(live) }()
	fail := func(err error) (uintptr, error) {
		release(link)
		return 0, err
	}
	if err := comCall("SetPath", link, slSetPath, wide(&live, t.Path)); err != nil {
		return fail(err)
	}
	if err := comCall("SetWorkingDirectory", link, slSetWorkingDirectory, wide(&live, filepath.Dir(t.Path))); err != nil {
		return fail(err)
	}
	if t.Args != "" {
		if err := comCall("SetArguments", link, slSetArguments, wide(&live, t.Args)); err != nil {
			return fail(err)
		}
	}
	if t.Description != "" {
		if err := comCall("SetDescription", link, slSetDescription, wide(&live, t.Description)); err != nil {
			return fail(err)
		}
	}
	if t.IconPath != "" {
		if err := comCall("SetIconLocation", link, slSetIconLocation, wide(&live, t.IconPath), uintptr(t.IconIndex)); err != nil {
			return fail(err)
		}
	}
	if err := setTitle(link, t.Title, &live); err != nil {
		return fail(err)
	}
	return link, nil
}

// setTitle sets the title of a shell link through its property store, which is
// how the jump list shows it.
func setTitle(link uintptr, title string, live *[]*uint16) error {
	var store uintptr
	if err := comCall("QueryInterface", link, iUnknownQueryInterface,
		uintptr(unsafe.Pointer(&iidPropertyStore)), uintptr(unsafe.Pointer(&store))); err != nil {
		return err
	}
	defer release(store)
	pv := propVariant{vt: vtLPWSTR, ptr: wide(live, title)}
	if err := comCall("SetValue", store, psSetValue, uintptr(unsafe.Pointer(&pkeyTitle)), uintptr(unsafe.Pointer(&pv))); err != nil {
		return err
	}
	runtime.KeepAlive(&pv)
	return comCall("Commit", store, psCommit)
}

// createCOM creates an in-process object of class clsid, and returns its interface
// iid. The caller releases it.
func createCOM(what string, clsid, iid *guid) (uintptr, error) {
	var obj uintptr
	if hr := hresult(win32.CoCreateInstance(unsafe.Pointer(clsid), unsafe.Pointer(iid), &obj)); hr.failed() {
		return 0, fmt.Errorf("platform/windows: creating %s failed with HRESULT 0x%08X", what, uint32(hr))
	}
	return obj, nil
}

// SetAppUserModelID gives the process its Application User Model ID, which groups
// its windows on the taskbar and keys its jump list and Recent documents. It must
// run before the process creates a window.
func SetAppUserModelID(id string) error { return win32.SetAppUserModelID(id) }

// PackagedProcess reports whether the process has a package identity, as an MSIX
// install gives it. A packaged process has an identity of its own, which the
// explicit ID does not replace.
func PackagedProcess() bool { return win32.PackagedProcess() }

// AddRecentDocument adds the file at the absolute path to the Recent documents of
// the process's Application User Model ID.
func AddRecentDocument(path string) { win32.AddRecentDocument(path) }
