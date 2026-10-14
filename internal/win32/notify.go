//go:build windows

package win32

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// The calls behind OS notifications: the HSTRING and activation-factory functions
// of combase.dll, which winstring.h and roapi.h declare, and the registry writes
// that register the protocol of a notification's activation.

var (
	combase                       = syscall.NewLazyDLL("combase.dll")
	procWindowsCreateString       = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString       = combase.NewProc("WindowsDeleteString")
	procWindowsGetStringRawBuffer = combase.NewProc("WindowsGetStringRawBuffer")
	procRoGetActivationFactory    = combase.NewProc("RoGetActivationFactory")
	procRegCreateKeyExW           = advapi32.NewProc("RegCreateKeyExW")
	procRegSetValueExW            = advapi32.NewProc("RegSetValueExW")
	procRegCloseKey               = advapi32.NewProc("RegCloseKey")
)

const (
	// REG_SZ is a null-terminated string.
	REG_SZ = 1
	// KEY_WRITE is the access that creates and sets values in a key.
	KEY_WRITE = 0x20006
	// REG_OPTION_NON_VOLATILE keeps the key across restarts.
	REG_OPTION_NON_VOLATILE = 0
)

// HSTRING creates an HSTRING holding s. The caller deletes it with DeleteHString.
func HSTRING(s string) (uintptr, error) {
	p, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	// The length excludes the terminator that UTF16FromString appends.
	var h uintptr
	hr, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)-1), uintptr(unsafe.Pointer(&h)))
	runtime.KeepAlive(p)
	if int32(hr) < 0 {
		return 0, fmt.Errorf("WindowsCreateString failed with HRESULT 0x%08X", uint32(hr))
	}
	return h, nil
}

// DeleteHString frees an HSTRING made by HSTRING or returned by a call. It does
// nothing for 0.
func DeleteHString(h uintptr) {
	if h != 0 {
		procWindowsDeleteString.Call(h)
	}
}

// HSTRINGText returns the text of h, which is not freed.
func HSTRINGText(h uintptr) string {
	var n uint32
	p, _, _ := procWindowsGetStringRawBuffer.Call(h, uintptr(unsafe.Pointer(&n)))
	if p == 0 || n == 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(ptrAt(p)), n))
}

// RoGetActivationFactory returns the factory of the runtime class named class, as
// the interface iid. It returns the HRESULT of the call.
func RoGetActivationFactory(class string, iid unsafe.Pointer, out *uintptr) uintptr {
	h, err := HSTRING(class)
	if err != nil {
		return 0x80004005 // E_FAIL
	}
	defer DeleteHString(h)
	hr, _, _ := procRoGetActivationFactory.Call(h, uintptr(iid), uintptr(unsafe.Pointer(out)))
	return hr
}

// SetRegistryString sets the REG_SZ value name of the key subkey under root, which
// is created when it does not exist. An empty name sets the key's default value.
// It returns the failure of the call.
func SetRegistryString(root uintptr, subkey, name, value string) error {
	k, err := syscall.UTF16PtrFromString(subkey)
	if err != nil {
		return err
	}
	var key uintptr
	r, _, _ := procRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(k)), 0, 0,
		REG_OPTION_NON_VOLATILE, KEY_WRITE, 0, uintptr(unsafe.Pointer(&key)), 0)
	runtime.KeepAlive(k)
	if r != ERROR_SUCCESS {
		return fmt.Errorf("RegCreateKeyExW %q failed with error %d", subkey, r)
	}
	defer procRegCloseKey.Call(key)
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	data, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}
	// The size counts the bytes of the value, terminator included.
	r, _, _ = procRegSetValueExW.Call(key, uintptr(unsafe.Pointer(n)), 0, REG_SZ,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
	runtime.KeepAlive(n)
	runtime.KeepAlive(data)
	if r != ERROR_SUCCESS {
		return fmt.Errorf("RegSetValueExW %q in %q failed with error %d", name, subkey, r)
	}
	return nil
}
