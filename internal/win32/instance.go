//go:build windows

package win32

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// The calls behind single-instance hand-off and the taskbar identity: a named
// mutex and a named pipe that a later launch writes to, the Application User
// Model ID, and the Recent documents of that ID.

var (
	procCreateMutexW                                         = kernel32.NewProc("CreateMutexW")
	procCreateNamedPipeW                                     = kernel32.NewProc("CreateNamedPipeW")
	procConnectNamedPipe                                     = kernel32.NewProc("ConnectNamedPipe")
	procDisconnectNamedPipe                                  = kernel32.NewProc("DisconnectNamedPipe")
	procReadFile                                             = kernel32.NewProc("ReadFile")
	procWriteFile                                            = kernel32.NewProc("WriteFile")
	procCreateFileW                                          = kernel32.NewProc("CreateFileW")
	procWaitNamedPipeW                                       = kernel32.NewProc("WaitNamedPipeW")
	procGetNamedPipeServerProcessId                          = kernel32.NewProc("GetNamedPipeServerProcessId")
	procGetCurrentProcessId                                  = kernel32.NewProc("GetCurrentProcessId")
	procProcessIdToSessionId                                 = kernel32.NewProc("ProcessIdToSessionId")
	procGetCurrentPackageFullName                            = kernel32.NewProc("GetCurrentPackageFullName")
	procLocalFree                                            = kernel32.NewProc("LocalFree")
	procConvertStringSecurityDescriptorToSecurityDescriptorW = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	procAllowSetForegroundWindow                             = user32.NewProc("AllowSetForegroundWindow")
	procSetCurrentProcessExplicitAppUserModelID              = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	procSHAddToRecentDocs                                    = shell32.NewProc("SHAddToRecentDocs")
)

const (
	ERROR_FILE_NOT_FOUND      = 2
	ERROR_BROKEN_PIPE         = 109
	ERROR_INSUFFICIENT_BUFFER = 122
	ERROR_ALREADY_EXISTS      = 183
	ERROR_PIPE_BUSY           = 231
	ERROR_PIPE_CONNECTED      = 535

	PIPE_ACCESS_DUPLEX            = 0x00000003
	FILE_FLAG_FIRST_PIPE_INSTANCE = 0x00080000
	GENERIC_READ_WRITE            = 0xC0000000
	OPEN_EXISTING                 = 3

	// SHARD_PATHW says that the item to add is a wide-character file path.
	SHARD_PATHW = 0x00000003
)

// invalidHandle is INVALID_HANDLE_VALUE, which CreateNamedPipeW and CreateFileW
// return on failure.
const invalidHandle = ^uintptr(0)

// securityAttributes is SECURITY_ATTRIBUTES.
type securityAttributes struct {
	Length             uint32
	SecurityDescriptor uintptr
	InheritHandle      int32
}

// CreateMutex creates the named mutex, or opens it if it exists. existed reports
// whether it already existed, which means another process holds it. The handle
// stays open for the life of the process.
func CreateMutex(name string) (h uintptr, existed bool, err error) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, false, err
	}
	r, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	if r == 0 {
		return 0, false, failure(err)
	}
	return r, err == syscall.Errno(ERROR_ALREADY_EXISTS), nil
}

// CreateNamedPipe creates the server end of a byte-mode named pipe. It is the
// first instance of its name, so no other server can take the name over, and it
// allows access only to the user whose SID is sid. The default security descriptor
// of a named pipe grants read access to Everyone, so the descriptor is given
// explicitly.
func CreateNamedPipe(name, sid string) (uintptr, error) {
	sddl, err := syscall.UTF16PtrFromString("D:P(A;;GA;;;" + sid + ")")
	if err != nil {
		return 0, err
	}
	var sd uintptr
	r, _, err := procConvertStringSecurityDescriptorToSecurityDescriptorW.Call(
		uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&sd)), 0)
	runtime.KeepAlive(sddl)
	if r == 0 {
		return 0, failure(err)
	}
	defer procLocalFree.Call(sd)

	sa := securityAttributes{Length: uint32(unsafe.Sizeof(securityAttributes{})), SecurityDescriptor: sd}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	// Byte mode with waiting, one instance, and buffers of 64 KB.
	h, _, err := procCreateNamedPipeW.Call(uintptr(unsafe.Pointer(p)),
		PIPE_ACCESS_DUPLEX|FILE_FLAG_FIRST_PIPE_INSTANCE, 0, 1, 64<<10, 64<<10, 0,
		uintptr(unsafe.Pointer(&sa)))
	runtime.KeepAlive(p)
	runtime.KeepAlive(&sa)
	if h == invalidHandle {
		return 0, failure(err)
	}
	return h, nil
}

// ConnectNamedPipe waits for a client to connect to the server end h. A client
// that connected before the call counts as connected.
func ConnectNamedPipe(h uintptr) error {
	r, _, err := procConnectNamedPipe.Call(h, 0)
	if r != 0 || err == syscall.Errno(ERROR_PIPE_CONNECTED) {
		return nil
	}
	return failure(err)
}

// DisconnectNamedPipe ends the connection of the server end h, so the next client
// can connect.
func DisconnectNamedPipe(h uintptr) { procDisconnectNamedPipe.Call(h) }

// ReadFile reads from the handle into p. At the end of a pipe it fails with
// ERROR_BROKEN_PIPE.
func ReadFile(h uintptr, p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	var n uint32
	r, _, err := procReadFile.Call(h, uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)), uintptr(unsafe.Pointer(&n)), 0)
	runtime.KeepAlive(p)
	if r == 0 {
		return int(n), failure(err)
	}
	return int(n), nil
}

// WriteFile writes p to the handle.
func WriteFile(h uintptr, p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	var n uint32
	r, _, err := procWriteFile.Call(h, uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)), uintptr(unsafe.Pointer(&n)), 0)
	runtime.KeepAlive(p)
	if r == 0 {
		return int(n), failure(err)
	}
	return int(n), nil
}

// OpenNamedPipe opens the client end of the named pipe for reading and writing.
// It fails with ERROR_PIPE_BUSY when every instance is connected, and with
// ERROR_FILE_NOT_FOUND when no server has created the pipe.
func OpenNamedPipe(name string) (uintptr, error) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	h, _, err := procCreateFileW.Call(uintptr(unsafe.Pointer(p)), GENERIC_READ_WRITE, 0, 0, OPEN_EXISTING, 0, 0)
	runtime.KeepAlive(p)
	if h == invalidHandle {
		return 0, failure(err)
	}
	return h, nil
}

// WaitNamedPipe waits up to ms milliseconds for an instance of the named pipe to
// be free.
func WaitNamedPipe(name string, ms uint32) error {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	r, _, err := procWaitNamedPipeW.Call(uintptr(unsafe.Pointer(p)), uintptr(ms))
	runtime.KeepAlive(p)
	if r == 0 {
		return failure(err)
	}
	return nil
}

// NamedPipeServerProcessID returns the process ID of the server at the client
// end h.
func NamedPipeServerProcessID(h uintptr) (uint32, error) {
	var pid uint32
	r, _, err := procGetNamedPipeServerProcessId.Call(h, uintptr(unsafe.Pointer(&pid)))
	if r == 0 {
		return 0, failure(err)
	}
	return pid, nil
}

// ProcessSessionID returns the Terminal Services session of this process.
func ProcessSessionID() (uint32, error) {
	pid, _, _ := procGetCurrentProcessId.Call()
	var session uint32
	r, _, err := procProcessIdToSessionId.Call(pid, uintptr(unsafe.Pointer(&session)))
	if r == 0 {
		return 0, failure(err)
	}
	return session, nil
}

// PackagedProcess reports whether this process has a package identity, as an MSIX
// install gives it. An unpackaged process gets APPMODEL_ERROR_NO_PACKAGE, and a
// packaged one needs a larger buffer for its full name.
func PackagedProcess() bool {
	var n uint32
	r, _, _ := procGetCurrentPackageFullName.Call(uintptr(unsafe.Pointer(&n)), 0)
	return r == ERROR_INSUFFICIENT_BUFFER
}

// AllowForegroundWindow lets the process pid set the foreground window, which
// Windows otherwise refuses to a process that did not get the input. The caller
// is the process that asks for it.
func AllowForegroundWindow(pid uint32) { procAllowSetForegroundWindow.Call(uintptr(pid)) }

// SetAppUserModelID sets the Application User Model ID of the process. It must be
// called before the process creates a window. It returns the failure of the call.
func SetAppUserModelID(id string) error {
	p, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return err
	}
	hr, _, _ := procSetCurrentProcessExplicitAppUserModelID.Call(uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	if int32(hr) < 0 {
		return fmt.Errorf("SetCurrentProcessExplicitAppUserModelID failed with HRESULT 0x%08X", uint32(hr))
	}
	return nil
}

// AddRecentDocument adds the file at path, which must be absolute, to the Recent
// documents of the process's Application User Model ID.
func AddRecentDocument(path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	procSHAddToRecentDocs.Call(SHARD_PATHW, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
}

// failure returns the error of a call that reported failure. A call can fail
// without setting an error code, so the error is never nil.
func failure(err error) error {
	if err == nil {
		return errNoSyscallError
	}
	return err
}

var errNoSyscallError = errors.New("the call failed without an error code")
