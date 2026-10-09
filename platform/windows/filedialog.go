//go:build windows

package windows

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/internal/win32"
)

// The file dialogs are COM objects (IFileOpenDialog and IFileSaveDialog) called
// through their vtables, as the renderer calls Direct3D. Each call names the
// vtable index of its method, counted from the declaration order in shobjidl.h:
// IUnknown has QueryInterface, AddRef and Release at 0 to 2.
const (
	iUnknownRelease = 2

	// IModalWindow and IFileDialog.
	fdShow                = 3
	fdSetFileTypes        = 4
	fdSetFileTypeIndex    = 5
	fdSetOptions          = 9
	fdSetFolder           = 12
	fdSetFileName         = 15
	fdSetTitle            = 17
	fdGetResult           = 20
	fdSetDefaultExtension = 22

	// IFileOpenDialog.
	fodGetResults = 27

	// IShellItem and IShellItemArray.
	siGetDisplayName = 5
	saGetCount       = 7
	saGetItemAt      = 8
)

// guid is a COM class or interface identifier, laid out as the GUID struct.
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	clsidFileOpenDialog = guid{0xdc1c5a9c, 0xe88a, 0x4dde, [8]byte{0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7}}
	clsidFileSaveDialog = guid{0xc0b4e2f3, 0xba21, 0x4773, [8]byte{0x8d, 0xba, 0x33, 0x5e, 0xc9, 0x46, 0xeb, 0x8b}}
	iidFileOpenDialog   = guid{0xd57c7288, 0xd4ad, 0x4768, [8]byte{0xbe, 0x02, 0x9d, 0x96, 0x95, 0x32, 0xd9, 0x60}}
	iidFileSaveDialog   = guid{0x84bccd23, 0x5fde, 0x4cdb, [8]byte{0xae, 0xa4, 0xaf, 0x64, 0xb8, 0x3d, 0x78, 0xab}}
	iidShellItem        = guid{0x43826d1e, 0xe718, 0x42ee, [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}
)

// hresult is a COM result. A failure has the top bit set.
type hresult uint32

func (h hresult) failed() bool { return int32(h) < 0 }

const (
	// hresultCanceled is HRESULT_FROM_WIN32(ERROR_CANCELLED), which Show
	// returns when the user closes the dialog without choosing.
	hresultCanceled = hresult(0x800704C7)
	// sigdnFileSysPath asks for a shell item's file system path.
	sigdnFileSysPath = 0x80058000
)

// File dialog options (FILEOPENDIALOGOPTIONS).
const (
	fosOverwritePrompt  = 0x00000002
	fosStrictFileTypes  = 0x00000004
	fosNoChangeDir      = 0x00000008
	fosPickFolders      = 0x00000020
	fosForceFileSystem  = 0x00000040
	fosAllowMultiSelect = 0x00000200
	fosPathMustExist    = 0x00000800
	fosFileMustExist    = 0x00001000
)

// errCanceled is how runFileDialog reports that the user closed the dialog.
var errCanceled = errors.New("canceled")

// comdlgFilterSpec is a COMDLG_FILTERSPEC: a file type's name and its patterns.
type comdlgFilterSpec struct {
	name, spec *uint16
}

// comMethod returns the function at vtable index i of the COM object obj.
func comMethod(obj uintptr, i int) uintptr {
	vtbl := *(*uintptr)(ptr(obj))
	return *(*uintptr)(ptr(vtbl + uintptr(i)*unsafe.Sizeof(vtbl)))
}

// comCall calls method i of obj with args, and reports a failing HRESULT as an
// error that names what was attempted. Pointer arguments must be kept alive by
// the caller until the call returns.
func comCall(what string, obj uintptr, i int, args ...uintptr) error {
	hr, _, _ := syscall.SyscallN(comMethod(obj, i), append([]uintptr{obj}, args...)...)
	if h := hresult(hr); h.failed() {
		return fmt.Errorf("platform/windows: %s failed with HRESULT 0x%08X", what, uint32(h))
	}
	return nil
}

// release drops a reference to a COM object. It does nothing for 0.
func release(obj uintptr) {
	if obj != 0 {
		syscall.SyscallN(comMethod(obj, iUnknownRelease), obj)
	}
}

// wide returns s as a NUL-terminated UTF-16 string. The string is appended to
// live, which the caller keeps alive until the dialog is done with it.
func wide(live *[]*uint16, s string) uintptr {
	p, _ := syscall.UTF16PtrFromString(s)
	*live = append(*live, p)
	return uintptr(unsafe.Pointer(p))
}

// fileFlags returns the options of a dialog of kind. Paths are file system
// paths, and the working directory is left alone. Each kind adds its own rules.
func fileFlags(kind dialog.FileKind, o dialog.FileOptions) uintptr {
	f := uintptr(fosNoChangeDir | fosForceFileSystem | fosPathMustExist)
	switch kind {
	case dialog.Open:
		f |= fosFileMustExist
		if o.Multiple {
			f |= fosAllowMultiSelect
		}
	case dialog.Save:
		f |= fosOverwritePrompt
		if len(o.Filters) > 0 {
			f |= fosStrictFileTypes
		}
	case dialog.Folder:
		f |= fosPickFolders
	}
	return f
}

// showFileDialog shows a file or folder dialog owned by owner, and returns what
// the user chose. It runs inside modal, on the UI thread.
func showFileDialog(owner uintptr, kind dialog.FileKind, o dialog.FileOptions) dialog.FileResult {
	paths, err := runFileDialog(owner, kind, o)
	if errors.Is(err, errCanceled) {
		return dialog.FileResult{Canceled: true}
	}
	if err != nil {
		return dialog.FileResult{Err: err}
	}
	return dialog.FileResult{Paths: paths}
}

// runFileDialog creates the dialog, configures it from o, shows it and reads
// the result. It returns errCanceled when the user closed the dialog.
func runFileDialog(owner uintptr, kind dialog.FileKind, o dialog.FileOptions) ([]string, error) {
	clsid, iid := &clsidFileOpenDialog, &iidFileOpenDialog
	if kind == dialog.Save {
		clsid, iid = &clsidFileSaveDialog, &iidFileSaveDialog
	}
	var dlg uintptr
	if hr := hresult(win32.CoCreateInstance(unsafe.Pointer(clsid), unsafe.Pointer(iid), &dlg)); hr.failed() {
		return nil, fmt.Errorf("platform/windows: CoCreateInstance failed with HRESULT 0x%08X", uint32(hr))
	}
	defer release(dlg)

	// live holds the wide strings the dialog reads, until it has returned.
	var live []*uint16
	defer func() { runtime.KeepAlive(live) }()
	if err := comCall("SetOptions", dlg, fdSetOptions, fileFlags(kind, o)); err != nil {
		return nil, err
	}
	if o.Title != "" {
		if err := comCall("SetTitle", dlg, fdSetTitle, wide(&live, o.Title)); err != nil {
			return nil, err
		}
	}
	if kind != dialog.Folder && len(o.Filters) > 0 {
		specs := make([]comdlgFilterSpec, len(o.Filters))
		for i, f := range o.Filters {
			name, _ := syscall.UTF16PtrFromString(f.Name)
			spec, _ := syscall.UTF16PtrFromString(strings.Join(f.Patterns, ";"))
			specs[i] = comdlgFilterSpec{name, spec}
		}
		defer runtime.KeepAlive(specs)
		if err := comCall("SetFileTypes", dlg, fdSetFileTypes, uintptr(len(specs)), uintptr(unsafe.Pointer(&specs[0]))); err != nil {
			return nil, err
		}
		if err := comCall("SetFileTypeIndex", dlg, fdSetFileTypeIndex, 1); err != nil {
			return nil, err
		}
	}
	if kind == dialog.Save {
		if o.Name != "" {
			if err := comCall("SetFileName", dlg, fdSetFileName, wide(&live, o.Name)); err != nil {
				return nil, err
			}
		}
		if ext := strings.TrimPrefix(o.Extension, "."); ext != "" {
			if err := comCall("SetDefaultExtension", dlg, fdSetDefaultExtension, wide(&live, ext)); err != nil {
				return nil, err
			}
		}
	}
	if o.Folder != "" {
		item, err := shellItem(o.Folder)
		if err != nil {
			return nil, err
		}
		err = comCall("SetFolder", dlg, fdSetFolder, item)
		release(item)
		if err != nil {
			return nil, err
		}
	}

	hr, _, _ := syscall.SyscallN(comMethod(dlg, fdShow), dlg, owner)
	switch h := hresult(hr); {
	case h == hresultCanceled:
		return nil, errCanceled
	case h.failed():
		return nil, fmt.Errorf("platform/windows: Show failed with HRESULT 0x%08X", uint32(h))
	}

	if kind == dialog.Save {
		var item uintptr
		if err := comCall("GetResult", dlg, fdGetResult, uintptr(unsafe.Pointer(&item))); err != nil {
			return nil, err
		}
		defer release(item)
		path, err := itemPath(item)
		if err != nil {
			return nil, err
		}
		return []string{path}, nil
	}
	return resultPaths(dlg)
}

// resultPaths reads the chosen items of an open or folder dialog.
func resultPaths(dlg uintptr) ([]string, error) {
	var arr uintptr
	if err := comCall("GetResults", dlg, fodGetResults, uintptr(unsafe.Pointer(&arr))); err != nil {
		return nil, err
	}
	defer release(arr)
	var n uint32
	if err := comCall("GetCount", arr, saGetCount, uintptr(unsafe.Pointer(&n))); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errCanceled
	}
	paths := make([]string, 0, n)
	for i := uint32(0); i < n; i++ {
		var item uintptr
		if err := comCall("GetItemAt", arr, saGetItemAt, uintptr(i), uintptr(unsafe.Pointer(&item))); err != nil {
			return nil, err
		}
		path, err := itemPath(item)
		release(item)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// shellItem returns an IShellItem for a file system path. The caller releases it.
func shellItem(path string) (uintptr, error) {
	p, _ := syscall.UTF16PtrFromString(path)
	var item uintptr
	hr := hresult(win32.SHCreateItemFromParsingName(p, unsafe.Pointer(&iidShellItem), &item))
	runtime.KeepAlive(p)
	if hr.failed() {
		return 0, fmt.Errorf("platform/windows: SHCreateItemFromParsingName failed with HRESULT 0x%08X", uint32(hr))
	}
	return item, nil
}

// itemPath returns the file system path of a shell item.
func itemPath(item uintptr) (string, error) {
	var name uintptr
	if err := comCall("GetDisplayName", item, siGetDisplayName, sigdnFileSysPath, uintptr(unsafe.Pointer(&name))); err != nil {
		return "", err
	}
	defer win32.CoTaskMemFree(name)
	return wstring(name), nil
}

// wstring reads the NUL-terminated UTF-16 string at p.
func wstring(p uintptr) string {
	var units []uint16
	for i := uintptr(0); ; i += 2 {
		u := *(*uint16)(ptr(p + i))
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}
