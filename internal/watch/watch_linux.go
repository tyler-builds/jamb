//go:build linux

package watch

import (
	"errors"
	"os"
	"sync"
	"syscall"
	"time"
)

// Watch calls onChange (on its own goroutine) after files in dir change, until
// stop is called. Changes arriving within debounce of each other produce one
// call. It uses no CPU while nothing changes: an inotify descriptor, read
// through the runtime's poller, wakes it on each change.
func Watch(dir string, debounce time.Duration, onChange func()) (stop func(), err error) {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, os.NewSyscallError("inotify_init1", err)
	}
	// The events Windows reports for a directory: names, sizes and writes.
	const mask = syscall.IN_ONLYDIR | syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MOVED_FROM |
		syscall.IN_MOVED_TO | syscall.IN_MODIFY | syscall.IN_CLOSE_WRITE | syscall.IN_ATTRIB
	if _, err := syscall.InotifyAddWatch(fd, dir, mask); err != nil {
		syscall.Close(fd)
		return nil, &os.PathError{Op: "inotify_add_watch", Path: dir, Err: err}
	}
	// A non-blocking descriptor joins the runtime's poller, so closing the file
	// wakes a read blocked on it.
	f := os.NewFile(uintptr(fd), "inotify")

	events := make(chan struct{}, 1)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 64*(syscall.SizeofInotifyEvent+syscall.NAME_MAX+1))
		for {
			n, err := f.Read(buf)
			if err != nil && !errors.Is(err, syscall.EINTR) {
				close(events) // closed by stop, or the descriptor failed
				return
			}
			if n > 0 {
				select {
				case events <- struct{}{}:
				default: // one is already pending
				}
			}
		}
	}()

	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-quit:
				return
			case _, ok := <-events:
				if !ok {
					return
				}
			}
			// Wait until the directory has been quiet for debounce; each further
			// change restarts the wait.
			quiet := time.NewTimer(debounce)
			for waiting := true; waiting; {
				select {
				case <-quit:
					quiet.Stop()
					return
				case _, ok := <-events:
					if !ok {
						quiet.Stop()
						return
					}
					quiet.Reset(debounce)
				case <-quiet.C:
					waiting = false
				}
			}
			onChange()
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(quit)
			f.Close()
			<-readDone
			<-done
		})
	}, nil
}
