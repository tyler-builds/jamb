//go:build windows

package watch

import (
	"sync"
	"time"

	"github.com/tyler-builds/jamb/internal/win32"
)

// Watch calls onChange (on its own goroutine) after files in dir change, until
// stop is called. Changes arriving within debounce of each other produce one
// call. It uses no CPU while nothing changes: the goroutine blocks in the
// kernel on the directory's change notification and on a stop event.
func Watch(dir string, debounce time.Duration, onChange func()) (stop func(), err error) {
	change, err := win32.FindFirstChangeNotification(dir, false,
		win32.FILE_NOTIFY_CHANGE_FILE_NAME|win32.FILE_NOTIFY_CHANGE_SIZE|win32.FILE_NOTIFY_CHANGE_LAST_WRITE)
	if err != nil {
		return nil, err
	}
	quit, err := win32.CreateEvent()
	if err != nil {
		win32.FindCloseChangeNotification(change)
		return nil, err
	}
	handles := []uintptr{change, quit}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer win32.CloseHandle(quit)
		defer win32.FindCloseChangeNotification(change)
		for {
			if win32.WaitForAny(handles, win32.INFINITE) != win32.WAIT_OBJECT_0 {
				return // stopped (or the wait failed)
			}
			win32.FindNextChangeNotification(change)
			// Wait until the directory has been quiet for debounce; each
			// further change re-arms the notification and restarts the wait.
			for quiet := false; !quiet; {
				switch win32.WaitForAny(handles, uint32(debounce.Milliseconds())) {
				case win32.WAIT_OBJECT_0:
					win32.FindNextChangeNotification(change)
				case win32.WAIT_TIMEOUT:
					quiet = true
				default:
					return
				}
			}
			onChange()
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			win32.SetEvent(quit)
			<-done
		})
	}, nil
}
