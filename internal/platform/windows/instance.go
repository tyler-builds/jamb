//go:build windows

package windows

import (
	"errors"
	"io"
	"syscall"
	"time"

	"github.com/tyler-builds/jamb/internal/win32"
)

// Single instance support. The first process creates a named mutex, which is
// local to its session, and serves a named pipe. A later process finds the mutex
// taken and writes its command line to the pipe. The pipe is machine-wide, so its
// name and its security descriptor keep it to one user; see win32.CreateNamedPipe.
//
// The pipe was chosen over WM_COPYDATA to a hidden window. WM_COPYDATA is sent to
// any window the sender can reach, so its only protection is the integrity check
// of User Interface Privilege Isolation, and a receiver that is busy blocks the
// sender unless the sender uses a timeout. A pipe has its own access control list
// that names the user, and a client waits at most the timeouts set in app, so a
// busy or stalled first instance cannot hold a later launch.

// serverRetry is how long the server waits after a failed connect before it
// listens again, so a failing pipe does not spin.
const serverRetry = 50 * time.Millisecond

// dialRetry is how long a client waits between attempts while the server is not
// there yet, or is busy with another client.
const dialRetry = 20 * time.Millisecond

// Pipe is one end of a named pipe: the server's connection to a client, or the
// client's connection to the server. It reads and writes bytes.
type Pipe struct{ h uintptr }

// Read implements io.Reader. The client's end of the pipe is closed by the
// other process, so a read after that returns io.EOF.
func (p *Pipe) Read(b []byte) (int, error) {
	n, err := win32.ReadFile(p.h, b)
	if errno(err) == win32.ERROR_BROKEN_PIPE {
		return 0, io.EOF
	}
	return n, err
}

// Write implements io.Writer.
func (p *Pipe) Write(b []byte) (int, error) { return win32.WriteFile(p.h, b) }

// Close closes this end of the pipe.
func (p *Pipe) Close() error {
	win32.CloseHandle(p.h)
	return nil
}

// errno returns the Windows error code of err, or 0 when err is not one.
func errno(err error) uint32 {
	var e syscall.Errno
	if errors.As(err, &e) {
		return uint32(e)
	}
	return 0
}

// ClaimInstance creates the named mutex, and reports whether this process created
// it. The process that creates it is the app's first instance. The mutex stays
// held until the process exits.
func ClaimInstance(name string) (first bool, err error) {
	_, existed, err := win32.CreateMutex(name)
	if err != nil {
		return false, err
	}
	return !existed, nil
}

// ServePipe creates the named pipe for the user with the SID sid, and calls serve
// with each client that connects, one client at a time, on a goroutine of its
// own. serve owns the connection while it runs. The pipe is disconnected after
// serve returns, so the next client can connect.
func ServePipe(name, sid string, serve func(conn io.ReadWriter)) error {
	h, err := win32.CreateNamedPipe(name, sid)
	if err != nil {
		return err
	}
	go func() {
		for {
			if err := win32.ConnectNamedPipe(h); err != nil {
				// A client that left before it was accepted is not a fault. The
				// instance must be disconnected before it can accept again.
				win32.DisconnectNamedPipe(h)
				time.Sleep(serverRetry)
				continue
			}
			serve(&Pipe{h})
			win32.DisconnectNamedPipe(h)
		}
	}()
	return nil
}

// DialPipe connects to the server of the named pipe, and returns the process ID
// of the server. It waits up to timeout for a server that is starting, or for an
// instance that is busy, and fails when the time is up.
func DialPipe(name string, timeout time.Duration) (*Pipe, uint32, error) {
	deadline := time.Now().Add(timeout)
	for {
		h, err := win32.OpenNamedPipe(name)
		if err == nil {
			pid, err := win32.NamedPipeServerProcessID(h)
			if err != nil {
				win32.CloseHandle(h)
				return nil, 0, err
			}
			return &Pipe{h}, pid, nil
		}
		left := time.Until(deadline)
		if left <= 0 {
			return nil, 0, err
		}
		switch errno(err) {
		case win32.ERROR_PIPE_BUSY:
			ms := max(uint32(left.Milliseconds()), 1)
			win32.WaitNamedPipe(name, ms) // the loop retries whatever it returns
		case win32.ERROR_FILE_NOT_FOUND:
			time.Sleep(min(dialRetry, left))
		default:
			return nil, 0, err
		}
	}
}

// AllowForegroundWindow lets the server process pid bring its windows to the
// front. Windows still decides, so the call may not be enough.
func AllowForegroundWindow(pid uint32) { win32.AllowForegroundWindow(pid) }

// SessionID returns the Terminal Services session of this process.
func SessionID() (uint32, error) { return win32.ProcessSessionID() }
