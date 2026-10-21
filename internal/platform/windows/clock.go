//go:build windows

package windows

import "github.com/tyler-builds/jamb/internal/win32"

// Clock returns the time in nanoseconds on the performance counter, for timing frames
// and input latency. Its origin is arbitrary, so only differences mean anything. Unlike
// the clock of time.Now, its steps are far finer than the 0.5 ms of Windows' monotonic
// clock.
func Clock() int64 { return win32.PerformanceNanos() }

// RefreshRate returns the refresh rate, in hertz, of the monitor the window is on, or 0
// when it is not known. It is not part of platform.Window: callers ask for it when the
// adapter has it.
func (w *window) RefreshRate() float64 { return win32.MonitorRefreshRate(w.hwnd) }

// ProcessMemory returns the process's private bytes and working set, in bytes (see
// win32.ProcessMemoryUsage).
func ProcessMemory() (privateBytes, workingSet uint64) { return win32.ProcessMemoryUsage() }
